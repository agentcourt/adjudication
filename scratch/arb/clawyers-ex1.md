# Curl Lawyers For `arb/examples/ex01`

## Current Path

`aar case` now starts the HTTP Lawyer API and waits for plaintiff and defendant tool calls.  No lawyer model, MCP bridge, or adapter runs inside the AAR runtime.  For this test, curl acted as both lawyers and as the observer.

The live run on 2026-06-01 used `arb/examples/ex01/complaint.md`, a temporary one-member council policy, and the case API at `http://127.0.0.1:19771`, with lawyer calls under `/lawyerapi/v1`.  The script sourced an operator environment file, started `aar case`, polled `GET /get` for plaintiff and defendant, called observer `get_turn` during each lawyer turn, and submitted every lawyer filing with `POST /do`.  For each lawyer filing, it copied `turn.opportunity_id` from the ready `GET` response into the POST body.  The case completed:

```json
{"status":"ok","result":"demonstrated","votes_for":1,"votes_against":0,"run_id":"run-1780325088602460322","out_dir":"out/ex01-lawyerapi-curl-20260601-094448"}
```

## Commands

The run used this shape.  The policy file only reduced council size so the live test would finish quickly; it did not change lawyer API behavior.  Each lawyer action below is a curl call to `POST /do`.

```bash
cd arb
source PATH/TO/env-file

BASE=http://127.0.0.1:19771/lawyerapi/v1
OUT=out/ex01-lawyerapi-curl-$(date +%Y%m%d-%H%M%S)
mkdir -p "$OUT"

.bin/aar case \
  --complaint examples/ex01/complaint.md \
  --out-dir "$OUT" \
  --policy /tmp/ex01-lawyerapi-policy.json \
  --caseapi-addr 127.0.0.1:19771 \
  --lawyer-timeout-seconds 240 \
  --invalid-attempt-limit 3
```

A lawyer checks whether it is ready:

```bash
curl -sS "$BASE/get?case_id=arb-1&role_id=plaintiff"
```

The observer checks the active turn:

```bash
curl -sS -X POST "$BASE/do" \
  -H 'content-type: application/json' \
  --data '{"case_id":"arb-1","role_id":"observer","tool":"get_turn","arguments":{}}'
```

A lawyer files an opening:

```bash
curl -sS -X POST "$BASE/do" \
  -H 'content-type: application/json' \
  --data '{
    "case_id": "arb-1",
    "role_id": "plaintiff",
    "opportunity_id": "openings:plaintiff",
    "tool": "submit_decision",
    "arguments": {
      "kind": "tool",
      "tool_name": "record_opening_statement",
      "payload": {
        "text": "Plaintiff opening.",
        "offered_evidence": [],
        "technical_reports": []
      }
    }
  }'
```

During arguments and rebuttals, the script also called `get_case` and `list_evidence` before filing.  The accepted lawyer filings produced eight successful `submit_decision` responses.  The observer responses recorded the active role, phase, deadline, and attempts for each lawyer turn.
