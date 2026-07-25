#!/bin/sh
# Runs one full case with real LLM agents as the participants.  Each
# participant is a one-shot `claude -p` run whose only tools come from
# the vmcp server through the stdio adapter.  Participants run in the
# procedure's order; the orchestrator carries the public record
# (proposition and statements) into the juror prompts.
set -e
cd "$(dirname "$0")/.."
ROOT="$(pwd)"
BIN="$ROOT/.lake/build/bin/vmcp"
CONFIG="$ROOT/examples/demo/config.json"
D="$ROOT/work/live"
MODEL="${VMCP_AGENT_MODEL:-haiku}"
rm -rf "$D"
mkdir -p "$D"
mkfifo "$D/control.in"
: > "$D/out.log"

"$BIN" serve --config "$CONFIG" --log "$D/live.log.ndjson" --state "$D/live.state.json" \
  < "$D/control.in" >> "$D/out.log" 2> "$D/server.err" &
SRV=$!
# Hold a writer so the control FIFO never reaches EOF between agents.
exec 8> "$D/control.in"
cleanup() { exec 8>&- 2>/dev/null; kill "$SRV" 2>/dev/null; }
trap cleanup EXIT INT TERM

run_agent() {
  NAME="$1"; TOKEN="$2"; PROMPT="$3"
  cat > "$D/mcp-$NAME.json" <<EOF
{"mcpServers":{"court":{"command":"$ROOT/drive/mcp-adapter.sh","args":["$NAME","$TOKEN","$D"]}}}
EOF
  echo "=== $NAME"
  claude -p "$PROMPT" \
    --mcp-config "$D/mcp-$NAME.json" \
    --strict-mcp-config \
    --model "$MODEL" \
    --dangerously-skip-permissions \
    2>> "$D/agents.err" || echo "(agent $NAME exited nonzero)"
}

PROP=$(jq -r '.init.proposition' "$CONFIG")

run_agent p tok-p "You are the plaintiff in a small arbitration.  The proposition: $PROP  Submit one brief opening statement supporting the proposition using the submit_statement tool.  Then stop."
run_agent d tok-d "You are the defendant in a small arbitration.  The proposition: $PROP  Submit one brief opening statement opposing the proposition using the submit_statement tool.  Then stop."

PSTMT=$(jq -r '.statements[0].text // "(none)"' "$D/live.state.json")
DSTMT=$(jq -r '.statements[1].text // "(none)"' "$D/live.state.json")
JURY_PROMPT="You are a juror in a small arbitration.  The proposition: $PROP  The plaintiff said: $PSTMT  The defendant said: $DSTMT  Decide whether the proposition was demonstrated.  Cast your vote with the submit_vote tool, vote value demonstrated or not_demonstrated, with a one-sentence rationale.  Then stop."

run_agent c1 tok-c1 "$JURY_PROMPT"
run_agent c2 tok-c2 "$JURY_PROMPT"
run_agent c3 tok-c3 "$JURY_PROMPT"

cleanup
trap - EXIT INT TERM
wait "$SRV" 2>/dev/null || true

echo "=== verify"
"$BIN" verify --config "$CONFIG" --log "$D/live.log.ndjson" --state "$D/live.state.json"
echo "=== outcome"
jq '{resolution, phase, votes}' "$D/live.state.json"
