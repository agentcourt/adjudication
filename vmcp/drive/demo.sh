#!/bin/sh
# Drives one full demo case through the vmcp server over the envelope
# transport, then verifies the resulting log against the final state.
set -e
cd "$(dirname "$0")/.."
BIN=.lake/build/bin/vmcp
CONFIG=examples/demo/config.json
mkdir -p work
rm -f work/demo.log.ndjson work/demo.state.json

{
  # Sessions authenticate with their tokens.
  echo '{"session":"p","payload":{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"token":"tok-p"}}}'
  echo '{"session":"d","payload":{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"token":"tok-d"}}}'
  echo '{"session":"c1","payload":{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"token":"tok-c1"}}}'
  echo '{"session":"c2","payload":{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"token":"tok-c2"}}}'
  # The plaintiff sees its statement tool; the defendant sees none yet.
  echo '{"session":"p","payload":{"jsonrpc":"2.0","id":2,"method":"tools/list"}}'
  echo '{"session":"d","payload":{"jsonrpc":"2.0","id":2,"method":"tools/list"}}'
  # Out of turn: the defendant's attempt is rejected.
  echo '{"session":"d","payload":{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"submit_statement","arguments":{"text":"Defendant tries to file first."}}}}'
  # In turn: plaintiff, then defendant.
  echo '{"session":"p","payload":{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"submit_statement","arguments":{"text":"The proposition holds for the demo reasons."}}}}'
  echo '{"session":"d","payload":{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"submit_statement","arguments":{"text":"The proposition fails for the demo reasons."}}}}'
  # Votes arrive in seating order.  C2 voting early is rejected.
  echo '{"session":"c2","payload":{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"submit_vote","arguments":{"vote":"demonstrated","rationale":"early"}}}}'
  echo '{"session":"c1","payload":{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"submit_vote","arguments":{"vote":"demonstrated","rationale":"persuaded"}}}}'
  echo '{"session":"c2","payload":{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"submit_vote","arguments":{"vote":"demonstrated","rationale":"agree"}}}}'
  # Two of three votes reach the threshold; the case is closed.
  echo '{"session":"p","payload":{"jsonrpc":"2.0","id":4,"method":"tools/list"}}'
} | "$BIN" serve --config "$CONFIG" --log work/demo.log.ndjson --state work/demo.state.json

echo "--- verify"
"$BIN" verify --config "$CONFIG" --log work/demo.log.ndjson --state work/demo.state.json
echo "--- final state"
cat work/demo.state.json
