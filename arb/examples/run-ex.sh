#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat >&2 <<'EOF'
Usage: examples/run-ex.sh EXAMPLE

Run one arb example through aar service and aar-mcp.

The script starts OpenClaw containers for plaintiff, defendant, and council
members C1 through C5. Lawyers and council members act only through MCP tools
that forward to the public AAR service APIs.
EOF
}

if [[ $# -ne 1 ]]; then
  usage
  exit 2
fi

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ARB_DIR="$(cd -- "${SCRIPT_DIR}/.." && pwd)"
cd "$ARB_DIR"

EX="$1"
if [[ "$EX" == */* || "$EX" == .* || "$EX" == *..* || "$EX" == "" ]]; then
  echo "Invalid example name: $EX" >&2
  exit 2
fi

COMPLAINT="examples/${EX}/complaint.md"
if [[ ! -f "$COMPLAINT" ]]; then
  echo "Missing complaint file: $COMPLAINT" >&2
  exit 2
fi

RUN_ID="$(date -u +%Y%m%d%H%M%S)"
CASE_ID="arb-${EX}-${RUN_ID}"
OUT="out/${EX}-service-openclaw-${RUN_ID}"
TOKEN="aar-${EX}-${RUN_ID}"
SERVICE_PORT=$((22000 + RANDOM % 10000))
MCP_PORT=$((SERVICE_PORT + 1))
SERVICE_BASE="http://127.0.0.1:${SERVICE_PORT}"
LAWYER_API="${SERVICE_BASE}/lawyerapi/v1"
COUNCIL_API="${SERVICE_BASE}/councilapi/v1"
MCP_FROM_DOCKER="http://host.docker.internal:${MCP_PORT}/mcp"

mkdir -p "$OUT/logs"
printf '%s\n' "$OUT" >"out/latest-${EX}-openclaw-lawyers.txt"

source "$HOME/keys.txt"
export OPENAI_API_KEY OPENROUTER_API_KEY ANTHROPIC_API_KEY GEMINI_API_KEY

CONTAINERS=()
PIDS=()
PRINCIPALS=()

cleanup() {
  set +e
  if [[ -n "${SERVICE_PID:-}" ]]; then kill "$SERVICE_PID" 2>/dev/null; fi
  if [[ -n "${MCP_PID:-}" ]]; then kill "$MCP_PID" 2>/dev/null; fi
  if [[ ${#CONTAINERS[@]} -gt 0 ]]; then
    docker stop "${CONTAINERS[@]}" >/dev/null 2>&1
  fi
}
trap cleanup EXIT

.bin/aar service \
  --listen "127.0.0.1:${SERVICE_PORT}" \
  --registry-dir "$OUT/registry" \
  --out-root "$OUT/service-out" \
  --aar-bin "$(pwd)/.bin/aar" \
  --bearer-token "$TOKEN" \
  >"$OUT/logs/service.stdout" \
  2>"$OUT/logs/service.stderr" &
SERVICE_PID=$!
echo "$SERVICE_PID" >"$OUT/service.pid"

for _ in $(seq 1 90); do
  if curl -fsS -H "Authorization: Bearer ${TOKEN}" "${SERVICE_BASE}/api/v1/cases" >"$OUT/logs/service-start.json" 2>"$OUT/logs/service-start.err"; then
    break
  fi
  sleep 1
done
if [[ ! -s "$OUT/logs/service-start.json" ]]; then
  echo "AAR service did not become ready" >&2
  exit 1
fi

.bin/aar-mcp \
  --listen "0.0.0.0:${MCP_PORT}" \
  --lawyerapi-base "$LAWYER_API" \
  --councilapi-base "$COUNCIL_API" \
  --bearer-token "$TOKEN" \
  --api-bearer-token "$TOKEN" \
  --session-ttl 0 \
  >"$OUT/logs/aar-mcp.stdout" \
  2>"$OUT/logs/aar-mcp.stderr" &
MCP_PID=$!
echo "$MCP_PID" >"$OUT/aar-mcp.pid"

CREATE_BODY="$OUT/create-case.json"
jq -n \
  --arg case_id "$CASE_ID" \
  --arg complaint "$COMPLAINT" \
  --arg out_dir "$OUT/case" \
  '{
    case_id: $case_id,
    complaint_path: $complaint,
    out_dir: $out_dir,
    council_backend: "councilapi",
    lawyer_timeout_seconds: 900,
    council_timeout_seconds: 900
  }' >"$CREATE_BODY"

curl -fsS \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d @"$CREATE_BODY" \
  "${SERVICE_BASE}/api/v1/cases" \
  >"$OUT/logs/create-case.json" \
  2>"$OUT/logs/create-case.err"

assignment_lawyer() {
  local role="$1"
  local server="aar-${CASE_ID}-${role}"
  cat <<EOF
You are the ${role} lawyer for AAR case ${CASE_ID}. Use MCP server ${server}. Work only through the AAR MCP tools for court filings.

Call wait_for_opportunity first. If it returns state waiting, call wait_for_opportunity again with after_version. If it returns state ready, read the returned prompt, turn, final_filing_actions, evidence_access, limits, remaining time, attempts remaining, and opportunity id. Complete exactly that opportunity. Use get_case and scan the evidence list for new case-packet files, newly submitted evidence, or changed metadata before filing. Use stat_evidence and read_evidence_range when exact contents matter. Analyze what the relevant evidence proves, what it does not prove, and whether provenance, custody, conflict, or missing links affect weight.

Keep private notes throughout the turn as a working journal: objective, issue breakdown, plan, work log, search log, source URLs or identifiers, tools used, scripts or programs written, packages installed, OCR or extraction steps, browser work, adverse checks, errors, reasoning, draft theory, decisions, and unresolved gaps. Call send_work_notes with the accumulated notes before submit_decision. If the record leaves a material gap, use all accessible and available resources that can find or test material evidence, including web search, browser tools, local programs, OCR, PDF/image/audio/video tools, metadata tools, hash/signature checks, archives, scripts, and installed programs. Follow search results to source pages or artifacts before relying on them. Check adverse sources, conflicting primary material, later corrections, missing context, and source-chain breaks.

When the current opportunity allows submit_evidence, submit source material through submit_evidence before relying on it in a filing. Evidence admission is separate from the final legal act. Use AAR evidence_id values for offered evidence. Do not cite a URL, filename, or your own notes as admitted evidence unless AAR accepted the source and returned an evidence_id.

Submit the final legal act for the turn through submit_decision. If submit_decision succeeds, return to wait_for_opportunity for the next ${role} opportunity. If wait_for_opportunity returns state done, report that the case is done and stop. If it returns state error, report the error and stop. Do not ask the user for the next turn. Do not create a cron job. Do not listen for inbound HTTP.
EOF
}

assignment_council() {
  local member="$1"
  local server="aar-${CASE_ID}-${member}"
  cat <<EOF
You are council member ${member} for AAR case ${CASE_ID}. Use MCP server ${server}. Work only through the AAR MCP tools for council deliberation.

Call wait_for_opportunity first. If it returns state waiting, call wait_for_opportunity again with after_version. If it returns state ready, read the returned prompt, turn, limits, remaining time, attempts remaining, and opportunity id. Review the visible arbitration record and admitted evidence. Use get_case, list_evidence, stat_evidence, and read_evidence_range when exact contents or provenance matter. Do not search the web, add evidence, rely on facts outside the record, or communicate with any lawyer or other council member.

When ready, call submit_council_vote with vote demonstrated or not_demonstrated and a concise rationale grounded in the record. If wait_for_opportunity returns state done, report that the case is done and stop. If it returns state error, report the error and stop. Do not ask the user for the next turn. Do not create a cron job. Do not listen for inbound HTTP.
EOF
}

start_openclaw() {
  local principal="$1"
  local kind="$2"
  local name="aar-${EX}-${RUN_ID}-${principal}"
  local server="aar-${CASE_ID}-${principal}"
  local mcp_json
  local text
  if [[ "$kind" == "lawyer" ]]; then
    mcp_json='{"url":"'"${MCP_FROM_DOCKER}"'?case_id='"${CASE_ID}"'&role_id='"${principal}"'","transport":"streamable-http","headers":{"Authorization":"Bearer '"${TOKEN}"'"}}'
    text="$(assignment_lawyer "$principal")"
  else
    mcp_json='{"url":"'"${MCP_FROM_DOCKER}"'?case_id='"${CASE_ID}"'&member_id='"${principal}"'","transport":"streamable-http","headers":{"Authorization":"Bearer '"${TOKEN}"'"}}'
    text="$(assignment_council "$principal")"
  fi
  CONTAINERS+=("$name")
  PRINCIPALS+=("$principal")
  docker run --rm \
    --name "$name" \
    --add-host=host.docker.internal:host-gateway \
    -e OPENAI_API_KEY \
    -e OPENROUTER_API_KEY \
    -e ANTHROPIC_API_KEY \
    -e GEMINI_API_KEY \
    -e AAR_MCP_NAME="$server" \
    -e AAR_MCP_JSON="$mcp_json" \
    -e AAR_SESSION_KEY="agent:aar:${CASE_ID}:${principal}" \
    -e AAR_ASSIGNMENT="$text" \
    -e AAR_PRINCIPAL="$principal" \
    ghcr.io/openclaw/openclaw:latest \
    sh -lc '
      set -u
      openclaw mcp set "$AAR_MCP_NAME" "$AAR_MCP_JSON" || exit $?
      first=1
      while true; do
        if [ "$first" -eq 1 ]; then
          message="$AAR_ASSIGNMENT"
          first=0
        else
          message="Continue the AAR assignment for ${AAR_PRINCIPAL}. Call wait_for_opportunity first. If an opportunity is ready, complete it using the AAR MCP tools. If the case is done or an error is reported, state that result. If no opportunity is ready, report waiting."
        fi
        openclaw agent --local --model gpt-5.5 --thinking low --timeout 3600 --session-key "$AAR_SESSION_KEY" --message "$message" --json
        status=$?
        if [ "$status" -ne 0 ]; then
          echo "openclaw agent exited with status $status" >&2
          sleep 10
        else
          sleep 5
        fi
      done
    ' \
    >"$OUT/logs/openclaw-${principal}.stdout" \
    2>"$OUT/logs/openclaw-${principal}.stderr" &
  pid=$!
  PIDS+=("$pid")
  echo "$pid" >"$OUT/openclaw-${principal}.pid"
}

start_openclaw plaintiff lawyer
start_openclaw defendant lawyer
for member in C1 C2 C3 C4 C5; do
  start_openclaw "$member" council
done

RESULT="$OUT/logs/result.json"
CASE_DONE=0
for _ in $(seq 1 720); do
  curl -fsS \
    -H "Authorization: Bearer ${TOKEN}" \
    "${SERVICE_BASE}/api/v1/cases/${CASE_ID}/result" \
    >"$RESULT.tmp" 2>"$OUT/logs/result.err" || true
  if [[ -s "$RESULT.tmp" ]]; then
    mv "$RESULT.tmp" "$RESULT"
    if jq -e '.status == "done" or (.result != null and .result.resolution != null)' "$RESULT" >/dev/null 2>&1; then
      CASE_DONE=1
      break
    fi
    if jq -e '.status == "failed" or .status == "canceled"' "$RESULT" >/dev/null 2>&1; then
      break
    fi
  fi
  sleep 5
done
rm -f "$RESULT.tmp"

if [[ ${#CONTAINERS[@]} -gt 0 ]]; then
  docker stop "${CONTAINERS[@]}" >/dev/null 2>&1 || true
fi
if [[ "$CASE_DONE" -ne 1 ]]; then
  echo "Case did not report completion before the polling deadline" >&2
fi

set +e
for i in "${!PIDS[@]}"; do
  wait "${PIDS[$i]}"
  echo "$?" >"$OUT/logs/openclaw-${PRINCIPALS[$i]}.exit"
done
set -e

curl -fsS \
  -H "Authorization: Bearer ${TOKEN}" \
  "${SERVICE_BASE}/api/v1/cases/${CASE_ID}" \
  >"$OUT/logs/case-record.json" \
  2>"$OUT/logs/case-record.err" || true

if [[ "$CASE_DONE" -ne 1 ]]; then
  exit 1
fi

echo "$OUT"
