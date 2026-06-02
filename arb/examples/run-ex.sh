#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat >&2 <<'EOF'
Usage: examples/run-ex.sh EXAMPLE

Run one arb example with two OpenClaw lawyer containers.

Example:
  examples/run-ex.sh ex1
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
OUT="out/${EX}-openclaw-lawyers-${RUN_ID}"
TOKEN="aar-${EX}-${RUN_ID}"
LAWYER_PORT=$((22000 + RANDOM % 10000))
MCP_PORT=$((LAWYER_PORT + 1))
LAWYER_API="http://127.0.0.1:${LAWYER_PORT}/lawyerapi/v1"
MCP_FROM_DOCKER="http://host.docker.internal:${MCP_PORT}/mcp"
PLAINTIFF_NAME="aar-${EX}-${RUN_ID}-plaintiff"
DEFENDANT_NAME="aar-${EX}-${RUN_ID}-defendant"

mkdir -p "$OUT/logs"
printf '%s\n' "$OUT" >"out/latest-${EX}-openclaw-lawyers.txt"

source "$HOME/keys.txt"
export OPENAI_API_KEY OPENROUTER_API_KEY ANTHROPIC_API_KEY GEMINI_API_KEY

cleanup() {
  set +e
  if [[ -n "${AAR_PID:-}" ]]; then kill "$AAR_PID" 2>/dev/null; fi
  if [[ -n "${MCP_PID:-}" ]]; then kill "$MCP_PID" 2>/dev/null; fi
  docker stop "$PLAINTIFF_NAME" "$DEFENDANT_NAME" >/dev/null 2>&1
}
trap cleanup EXIT

.bin/aar case \
  --complaint "$COMPLAINT" \
  --out-dir "$OUT/case" \
  --lawyerapi-addr "127.0.0.1:${LAWYER_PORT}" \
  --council-backend direct \
  --lawyer-timeout-seconds 900 \
  >"$OUT/logs/aar.stdout" \
  2>"$OUT/logs/aar.stderr" &
AAR_PID=$!
echo "$AAR_PID" >"$OUT/aar.pid"

for _ in $(seq 1 90); do
  if curl -fsS "${LAWYER_API}/get?case_id=${CASE_ID}&role_id=observer" >"$OUT/logs/observer-start.json" 2>"$OUT/logs/observer-start.err"; then
    break
  fi
  sleep 1
done
if [[ ! -s "$OUT/logs/observer-start.json" ]]; then
  echo "Lawyer API did not become ready" >&2
  exit 1
fi

.bin/aar-lawyer-mcp \
  --listen "0.0.0.0:${MCP_PORT}" \
  --lawyerapi-base "$LAWYER_API" \
  --bearer-token "$TOKEN" \
  --session-ttl 0 \
  >"$OUT/logs/lawyer-mcp.stdout" \
  2>"$OUT/logs/lawyer-mcp.stderr" &
MCP_PID=$!
echo "$MCP_PID" >"$OUT/lawyer-mcp.pid"

assignment() {
  local role="$1"
  local server="aar-${CASE_ID}-${role}"
  cat <<EOF
You are the ${role} lawyer for AAR case ${CASE_ID}. Use MCP server ${server}. Work only through the AAR MCP tools for court filings.

Call wait_for_opportunity first. If it returns state waiting, call wait_for_opportunity again with after_version. If it returns state ready, read the returned prompt, turn, allowed operations, limits, remaining time, attempts remaining, and opportunity id. Complete exactly that opportunity. Use get_case and scan the evidence list for new case-packet files, newly submitted evidence, or changed metadata before filing. Use stat_evidence and read_evidence_range when exact contents matter. Analyze what the relevant evidence proves, what it does not prove, and whether provenance, custody, conflict, or missing links affect weight.

The current AAR opportunity controls court actions, not your full investigation toolbox. The MCP adapter exposes stable transport tools; the returned prompt states which court actions are allowed now. Keep private notes throughout the turn as a working journal: objective, issue breakdown, plan, work log, search log, source URLs or identifiers, tools used, scripts or programs written, packages installed, OCR or extraction steps, browser work, adverse checks, errors, reasoning, draft theory, decisions, and unresolved gaps. Call send_work_notes with the accumulated notes before submit_decision. If the existing record leaves a material gap, use all accessible and available resources that can find or test material evidence: native OpenClaw web, browser, file, shell, OCR, PDF, image, audio, video, metadata, hash, signature, archive, and local analysis tools. If the environment permits it, install useful programs, write and run scripts or small programs, download source artifacts, and use a browser for dynamic pages or visual inspection. Follow search results to source pages or artifacts before relying on them. Check adverse sources, conflicting primary material, later corrections, missing context, and source-chain breaks. Do not use credentials, paid services, private accounts, or privileged sources unless the operator explicitly provides them for this case. When the current opportunity allows submit_evidence, submit source material through the direct submit_evidence MCP tool before relying on it in a filing. Do not call submit_decision with tool_name set to submit_evidence. Use AAR evidence_id values for offered evidence. Do not cite a URL, filename, or your own notes as admitted evidence unless AAR has accepted the source and returned an evidence_id.

Submit the final legal act for the turn through submit_decision. Evidence admission is separate from the final legal act. If submit_decision succeeds, return to wait_for_opportunity for the next ${role} opportunity.

If wait_for_opportunity returns state done, report that the case is done and stop. If it returns state error, report the error and stop. Do not ask the user for the next turn. Do not create a cron job. Do not listen for inbound HTTP.
EOF
}

start_lawyer() {
  local role="$1"
  local name="$2"
  local server="aar-${CASE_ID}-${role}"
  local mcp_json
  local text
  mcp_json='{"url":"'"${MCP_FROM_DOCKER}"'?case_id='"${CASE_ID}"'&role_id='"${role}"'","transport":"streamable-http","headers":{"Authorization":"Bearer '"${TOKEN}"'"}}'
  text="$(assignment "$role")"
  docker run --rm \
    --name "$name" \
    --add-host=host.docker.internal:host-gateway \
    -e OPENAI_API_KEY \
    -e OPENROUTER_API_KEY \
    -e ANTHROPIC_API_KEY \
    -e GEMINI_API_KEY \
    -e AAR_MCP_NAME="$server" \
    -e AAR_MCP_JSON="$mcp_json" \
    -e AAR_SESSION_KEY="agent:aar:${CASE_ID}:${role}" \
    -e AAR_ASSIGNMENT="$text" \
    ghcr.io/openclaw/openclaw:latest \
    sh -lc 'openclaw mcp set "$AAR_MCP_NAME" "$AAR_MCP_JSON"; openclaw agent --local --model gpt-5.5 --thinking low --timeout 3600 --session-key "$AAR_SESSION_KEY" --message "$AAR_ASSIGNMENT" --json' \
    >"$OUT/logs/openclaw-${role}.stdout" \
    2>"$OUT/logs/openclaw-${role}.stderr" &
  LAWYER_PID=$!
}

start_lawyer plaintiff "$PLAINTIFF_NAME"
PLAINTIFF_PID="$LAWYER_PID"
echo "$PLAINTIFF_PID" >"$OUT/openclaw-plaintiff.pid"
start_lawyer defendant "$DEFENDANT_NAME"
DEFENDANT_PID="$LAWYER_PID"
echo "$DEFENDANT_PID" >"$OUT/openclaw-defendant.pid"

set +e
wait "$AAR_PID"
AAR_EXIT=$?
wait "$PLAINTIFF_PID"
PLAINTIFF_EXIT=$?
wait "$DEFENDANT_PID"
DEFENDANT_EXIT=$?
set -e

echo "$AAR_EXIT" >"$OUT/logs/aar.exit"
echo "$PLAINTIFF_EXIT" >"$OUT/logs/openclaw-plaintiff.exit"
echo "$DEFENDANT_EXIT" >"$OUT/logs/openclaw-defendant.exit"
echo "$OUT"
exit "$AAR_EXIT"
