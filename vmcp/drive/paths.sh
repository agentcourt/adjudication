#!/bin/sh
# Exercises the paths the demo does not: unknown token, duplicate token
# binding, member failure ending in no_majority, tampered-log
# verification failure, and restart recovery.
set -e
cd "$(dirname "$0")/.."
BIN=.lake/build/bin/vmcp
CONFIG=examples/demo/config.json
LOG=work/paths.log.ndjson
STATE=work/paths.state.json
OUT=work/paths.out.ndjson
mkdir -p work
rm -f "$LOG" "$STATE" "$OUT"

fail() { echo "FAIL: $1"; exit 1; }

{
  echo '{"session":"p","payload":{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"token":"tok-p"}}}'
  echo '{"session":"d","payload":{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"token":"tok-d"}}}'
  echo '{"session":"sys","payload":{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"token":"tok-sys"}}}'
  echo '{"session":"c1","payload":{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"token":"tok-c1"}}}'
  echo '{"session":"c2","payload":{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"token":"tok-c2"}}}'
  echo '{"session":"x1","payload":{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"token":"bogus"}}}'
  echo '{"session":"x2","payload":{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"token":"tok-p"}}}'
  echo '{"session":"p","payload":{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"submit_statement","arguments":{"text":"For."}}}}'
  echo '{"session":"d","payload":{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"submit_statement","arguments":{"text":"Against."}}}}'
  echo '{"session":"sys","payload":{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"fail_member","arguments":{"member_id":"C3","reason":"unresponsive"}}}}'
  echo '{"session":"c1","payload":{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"submit_vote","arguments":{"vote":"demonstrated","rationale":"for"}}}}'
  echo '{"session":"c2","payload":{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"submit_vote","arguments":{"vote":"not_demonstrated","rationale":"against"}}}}'
} | "$BIN" serve --config "$CONFIG" --log "$LOG" --state "$STATE" > "$OUT" 2> work/paths.err

grep '"session":"x1"' "$OUT" | grep -q "unknown token" || fail "unknown token not rejected"
grep '"session":"x2"' "$OUT" | grep -q "bound to another live session" || fail "duplicate binding not rejected"
grep -q '"no_majority"' "$STATE" || fail "no_majority not reached"
grep -q '"seated": false' "$STATE" || fail "member failure not recorded"

"$BIN" verify --config "$CONFIG" --log "$LOG" --state "$STATE" | grep -q '^ok$' || fail "verification of the real log failed"

# A tampered log must fail verification: repeat the last accepted action.
cp "$LOG" work/paths.tampered.ndjson
tail -n 1 "$LOG" >> work/paths.tampered.ndjson
if "$BIN" verify --config "$CONFIG" --log work/paths.tampered.ndjson --state "$STATE" | grep -q '^ok$'; then
  fail "tampered log verified"
fi

# Restart recovery: a fresh process resumes at the logged version.
echo '{"session":"p","payload":{"jsonrpc":"2.0","id":9,"method":"initialize","params":{"token":"tok-p"}}}' \
  | "$BIN" serve --config "$CONFIG" --log "$LOG" --state "$STATE" 2>&1 >/dev/null \
  | grep -q "state_version 5" || fail "restart did not recover state_version 5"

echo "PASS: unknown token, duplicate binding, no_majority, tamper detection, recovery"
