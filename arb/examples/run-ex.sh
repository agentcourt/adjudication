#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat >&2 <<'EOF'
Usage: examples/run-ex.sh EXAMPLE

Run one arb example through aar service and aar mcp.

The script starts OpenClaw containers for plaintiff and defendant lawyers.
Council members run as Pi agents in Podman and use the same MCP service.
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
OUT="out/${EX}-service-openclaw-pi-${RUN_ID}"
TOKEN="aar-${EX}-${RUN_ID}"
SERVICE_PORT=$((22000 + RANDOM % 10000))
MCP_PORT=$((SERVICE_PORT + 1))
SERVICE_BASE="http://127.0.0.1:${SERVICE_PORT}"
CASE_API="$SERVICE_BASE"
LAWYER_API="${CASE_API}/lawyerapi/v1"
MCP_FROM_DOCKER="http://host.docker.internal:${MCP_PORT}/mcp"
MCP_FROM_PODMAN="http://127.0.0.1:${MCP_PORT}/mcp"
PI_IMAGE="${PI_CONTAINER_IMAGE:-agentcourt-pi-sandbox}"

mkdir -p "$OUT/logs"
printf '%s\n' "$OUT" >"out/latest-${EX}-openclaw-lawyers.txt"

source "$HOME/keys.txt"
: "${OPENAI_API_KEY:?OPENAI_API_KEY is required for OpenClaw lawyers}"
: "${OPENROUTER_API_KEY:?OPENROUTER_API_KEY is required for Pi council}"
export OPENAI_API_KEY OPENROUTER_API_KEY

CONTAINERS=()
PIDS=()
LAWYER_PIDS=()

cleanup() {
  set +e
  if [[ -n "${SERVICE_PID:-}" ]]; then kill "$SERVICE_PID" 2>/dev/null; fi
  if [[ -n "${MCP_PID:-}" ]]; then kill "$MCP_PID" 2>/dev/null; fi
  if [[ -n "${RESULT_PID:-}" ]]; then kill "$RESULT_PID" 2>/dev/null; fi
  for pid in "${PIDS[@]}"; do
    kill "$pid" 2>/dev/null
  done
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

.bin/aar mcp \
  --listen "0.0.0.0:${MCP_PORT}" \
  --caseapi-base "$CASE_API" \
  --bearer-token "$TOKEN" \
  --api-bearer-token "$TOKEN" \
  --session-ttl 0 \
  >"$OUT/logs/mcp.stdout" \
  2>"$OUT/logs/mcp.stderr" &
MCP_PID=$!
echo "$MCP_PID" >"$OUT/mcp.pid"

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
You are council member ${member} for AAR case ${CASE_ID}. Your MCP server is named ${server}. Use the Pi MCP proxy tool to call AAR tools.

First call mcp with {"connect":"${server}"}. Then call mcp with {"tool":"wait_for_opportunity","args":"{}"}. If Pi reports that wait_for_opportunity is not found, call mcp with {"search":"wait_for_opportunity"} and use the exact returned tool name. If wait_for_opportunity returns state waiting, call it again with the returned after_version, using an args string such as {"after_version":3}. If it returns state ready, read the returned prompt, turn, limits, remaining time, attempts remaining, and opportunity id.

Review the visible arbitration record and admitted evidence. Use mcp tool calls for get_case, list_evidence, stat_evidence, and read_evidence_range when exact contents or provenance matter. You may use Pi's ordinary local tools to organize and analyze the record, but your vote must rely on admitted record evidence and the arbitration record.

When ready, call submit_council_vote through mcp with vote demonstrated or not_demonstrated and a concise rationale grounded in the record. Do not write tool-call markup as plain text. Invoke the mcp tool through Pi. If wait_for_opportunity returns state done, report that the case is done and stop. If it returns state error, report the error and stop. Do not ask the user for the next turn. Do not create a cron job. Do not listen for inbound HTTP.
EOF
}

start_openclaw_lawyer() {
  local role="$1"
  local name="aar-${EX}-${RUN_ID}-${role}"
  local server="aar-${CASE_ID}-${role}"
  local mcp_json
  local text
  mcp_json='{"url":"'"${MCP_FROM_DOCKER}"'?case_id='"${CASE_ID}"'&role_id='"${role}"'","transport":"streamable-http","headers":{"Authorization":"Bearer '"${TOKEN}"'"}}'
  text="$(assignment_lawyer "$role")"
  CONTAINERS+=("$name")
  docker run --rm \
    --name "$name" \
    --add-host=host.docker.internal:host-gateway \
    -e OPENAI_API_KEY \
    -e AAR_MCP_NAME="$server" \
    -e AAR_MCP_JSON="$mcp_json" \
    -e AAR_SESSION_KEY="agent:aar:${CASE_ID}:${role}" \
    -e AAR_ASSIGNMENT="$text" \
    -e AAR_PRINCIPAL="$role" \
    ghcr.io/openclaw/openclaw:latest \
    sh -lc '
      set -u
      openclaw mcp set "$AAR_MCP_NAME" "$AAR_MCP_JSON" || exit $?
      exec openclaw agent --local --model gpt-5.5 --thinking low --timeout 3600 --session-key "$AAR_SESSION_KEY" --message "$AAR_ASSIGNMENT" --json
    ' \
    >"$OUT/logs/openclaw-${role}.stdout" \
    2>"$OUT/logs/openclaw-${role}.stderr" &
  local pid=$!
  PIDS+=("$pid")
  LAWYER_PIDS+=("$pid")
  echo "$pid" >"$OUT/openclaw-${role}.pid"
}

wait_for_council_roster() {
  local status_json="$OUT/logs/observer-status.json"
  for _ in $(seq 1 120); do
    curl -fsS \
      -H "Authorization: Bearer ${TOKEN}" \
      "${LAWYER_API}/status?case_id=${CASE_ID}&role_id=observer" \
      >"$status_json.tmp" 2>"$OUT/logs/observer-status.err"
    if [[ -s "$status_json.tmp" ]] && jq -e '.council_roster | length >= 1' "$status_json.tmp" >/dev/null 2>&1; then
      mv "$status_json.tmp" "$status_json"
      echo "$status_json"
      return 0
    fi
    sleep 1
  done
  rm -f "$status_json.tmp"
  echo "Council roster did not become available" >&2
  exit 1
}

write_pi_config() {
  local member="$1"
  local entry_file="$2"
  local home="$3"
  local model
  local endpoint
  local unsupported_request
  local max_tokens
  local routing

  model="$(jq -r '.request_spec.model // (.model | sub("^[^:]+://"; "") | split("?")[0])' "$entry_file")"
  endpoint="$(jq -r '.request_spec.endpoint // (.model | split("://")[0])' "$entry_file")"
  if [[ "$endpoint" != "openrouter" ]]; then
    echo "Pi council requires openrouter endpoint for ${member}; got ${endpoint}" >&2
    exit 1
  fi
  unsupported_request="$(jq -r '(.request_spec.request // .request // {}) | keys[]? | select(. != "max_tokens" and . != "max_output_tokens")' "$entry_file" | tr '\n' ' ')"
  if [[ -n "${unsupported_request// }" ]]; then
    echo "Pi council cannot enforce request fields for ${member}: ${unsupported_request}" >&2
    exit 1
  fi
  max_tokens="$(jq -r '(.request_spec.request.max_output_tokens // .request_spec.request.max_tokens // .request.max_output_tokens // .request.max_tokens // empty)' "$entry_file")"
  if [[ -z "$max_tokens" ]]; then
    max_tokens=null
  fi
  routing="$(jq -c '(.request_spec.provider // .provider // null)' "$entry_file")"

  mkdir -p "$home/.pi/agent"
  jq -n \
    --arg model "$model" \
    '{defaultProvider:"openrouter", defaultModel:$model, quietStartup:true}' \
    >"$home/.pi/agent/settings.json"
  jq -n \
    --arg model "$model" \
    --arg member "$member" \
    --argjson routing "$routing" \
    --argjson maxTokens "$max_tokens" \
    '{
      providers: {
        openrouter: {
          baseUrl: "https://openrouter.ai/api/v1",
          apiKey: "$OPENROUTER_API_KEY",
          api: "openai-completions",
          models: [
            ({
              id: $model,
              name: ("AAR " + $member + " " + $model)
            }
            + (if $maxTokens == null then {} else {maxTokens: $maxTokens} end)
            + (if $routing == null then {} else {compat: {openRouterRouting: $routing}} end))
          ]
        }
      }
    }' >"$home/.pi/agent/models.json"
}

start_pi_council() {
  local member="$1"
  local roster_json="$2"
  local server="aar-${CASE_ID}-${member}"
  local home="$PWD/$OUT/pi-${member}"
  local entry_file="$OUT/logs/council-${member}-roster.json"
  local text
  local model

  jq --arg member "$member" '.council_roster[] | select(.member_id == $member)' "$roster_json" >"$entry_file"
  if [[ ! -s "$entry_file" ]]; then
    echo "Missing council roster entry for ${member}" >&2
    exit 1
  fi
  write_pi_config "$member" "$entry_file" "$home"
  model="$(jq -r '.request_spec.model // (.model | sub("^[^:]+://"; "") | split("?")[0])' "$entry_file")"
  jq -n \
    --arg server "$server" \
    --arg url "${MCP_FROM_PODMAN}?case_id=${CASE_ID}&member_id=${member}" \
    --arg token "$TOKEN" \
    '{
      mcpServers: {
        ($server): {
          url: $url,
          transport: "streamable-http",
          lifecycle: "keep-alive",
          headers: {
            Authorization: ("Bearer " + $token)
          }
        }
      }
    }' >"$home/.mcp.json"
  text="$(assignment_council "$member")"

  podman run --rm \
    --network host \
    --user 0:0 \
    -e HOME=/home/user \
    -e TMPDIR=/home/user \
    -e PI_CODING_AGENT_DIR=/home/user/.pi/agent \
    -e OPENROUTER_API_KEY \
    -e NODE_OPTIONS \
    -v "$home:/home/user" \
    -w /home/user \
    "$PI_IMAGE" \
    --provider openrouter \
    --model "$model" \
    -e npm:pi-mcp-adapter \
    --mode json \
    -p "$text" \
    >"$OUT/logs/pi-${member}.stdout" \
    2>"$OUT/logs/pi-${member}.stderr" &
  local pid=$!
  PIDS+=("$pid")
  echo "$pid" >"$OUT/pi-${member}.pid"
}

start_openclaw_lawyer plaintiff
start_openclaw_lawyer defendant
ROSTER_JSON="$(wait_for_council_roster)"
for member in C1 C2 C3 C4 C5; do
  start_pi_council "$member" "$ROSTER_JSON"
done

RESULT="$OUT/logs/result.json"

wait_for_result() {
  while true; do
    curl -fsS \
      -H "Authorization: Bearer ${TOKEN}" \
      "${SERVICE_BASE}/api/v1/cases/${CASE_ID}/result" \
      >"$RESULT"
    if jq -e '.status == "done" or (.result != null and .result.resolution != null)' "$RESULT" >/dev/null; then
      return 0
    fi
    if jq -e '.status == "failed" or .status == "canceled"' "$RESULT" >/dev/null; then
      return 1
    fi
    sleep 5
  done
}

wait_for_result &
RESULT_PID=$!
wait -n "$RESULT_PID" "${LAWYER_PIDS[@]}"

curl -fsS \
  -H "Authorization: Bearer ${TOKEN}" \
  "${SERVICE_BASE}/api/v1/cases/${CASE_ID}/result" \
  >"$RESULT" \
  2>"$OUT/logs/result.err"

curl -fsS \
  -H "Authorization: Bearer ${TOKEN}" \
  "${SERVICE_BASE}/api/v1/cases/${CASE_ID}" \
  >"$OUT/logs/case-record.json" \
  2>"$OUT/logs/case-record.err"

jq -e '.status == "done" or (.result != null and .result.resolution != null)' "$RESULT" >/dev/null

echo "$OUT"
