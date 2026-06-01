# Lawyer HTTP API

## Purpose

The Lawyer API lets an outside process act as a plaintiff lawyer, defendant lawyer, or observer in an arbitration.  AAR owns the case state, turn order, evidence store, filing validation, deadlines, attempts, and event log.  A caller uses HTTP to ask what the role can do, then executes one returned tool at a time.

The first implementation serves one case from one `aar case` process.  Each request includes `case_id` and `role_id`; `case_id` is required and returned unchanged, but the server does not use it to select a case yet.  Front ends such as a CLI, MCP server, ACP service, or agent runner should be separate clients built on this API.

## Endpoints

| Method | Path | Meaning |
| --- | --- | --- |
| `GET` | `/lawyerapi/v1/get?case_id={case_id}&role_id={role_id}` | Return role status, prompt, tools, limits, and the live turn budget. |
| `POST` | `/lawyerapi/v1/do` | Execute one tool call for the role. |

All responses are JSON.  HTTP status codes describe request handling, and the response body uses `ok` to report whether the requested API operation succeeded.  Well-formed tool calls that fail procedural validation usually return HTTP `200` with `ok: false`, because the server understood the request and rejected the attempted action.

## Roles

`plaintiff` and `defendant` are lawyer roles.  When the procedure is waiting for one of those roles, `GET` returns `status: "ready"`, the current prompt, the available tools, and live turn information.  When the procedure is waiting for another role or for the council, `GET` returns `status: "waiting"` with no lawyer tools.

`observer` is read-only.  It can inspect the case, current turn, events, and evidence.  It cannot submit decisions, submit evidence, upload evidence, vote, or alter any deadline.

## Turn Budget

Each lawyer opportunity has one deadline and one attempt budget.  The deadline covers the whole turn, including malformed tool calls and corrected retries.  The attempt budget counts rejected mutating tool calls, including invalid `submit_decision` calls.

Every `GET` and `POST` response includes a `turn` field.  When a lawyer turn is active, `turn` contains the current role, phase, opportunity id, deadline, live `remaining_ms`, `attempts_max`, and `attempts_remaining`.  A lawyer copies `turn.opportunity_id` into each `POST /do` request for that turn.  When no lawyer turn is active, `turn` is `null`.

## GET

`GET /lawyerapi/v1/get` requires both query parameters.  The role id must be `plaintiff`, `defendant`, or `observer`.  The returned tool list and `turn.opportunity_id` are authoritative for the next lawyer `POST /do` call.

```bash
curl -sS "$BASE/get?case_id=arb-1&role_id=plaintiff"
```

A ready lawyer response has this shape:

```json
{
  "ok": true,
  "case_id": "arb-1",
  "role_id": "plaintiff",
  "status": "ready",
  "prompt": "You represent the plaintiff...",
  "turn": {
    "role_id": "plaintiff",
    "phase": "openings",
    "opportunity_id": "openings:plaintiff",
    "turn_number": 1,
    "deadline": "2026-06-01T15:04:05Z",
    "remaining_ms": 845233,
    "attempts_max": 3,
    "attempts_remaining": 3,
    "completed": false
  },
  "tools": [
    {
      "name": "get_case",
      "description": "Return the current visible arbitration record.",
      "input_schema": {
        "type": "object",
        "properties": {},
        "additionalProperties": false
      },
      "read_only": true
    },
    {
      "name": "submit_decision",
      "description": "Submit the legal act for the current opportunity.",
      "input_schema": {
        "type": "object",
        "properties": {
          "kind": { "type": "string", "enum": ["tool", "pass"] },
          "tool_name": { "type": "string" },
          "payload": { "type": "object" }
        },
        "required": ["kind"],
        "additionalProperties": false
      },
      "read_only": false
    }
  ],
  "limits": {
    "text_char_limit": 5000,
    "max_response_bytes": 131072,
    "attempts_max": 3,
    "attempts_remaining": 3
  }
}
```

A waiting lawyer response keeps the same envelope and returns no tools:

```json
{
  "ok": true,
  "case_id": "arb-1",
  "role_id": "defendant",
  "status": "waiting",
  "prompt": "",
  "turn": {
    "role_id": "plaintiff",
    "phase": "openings",
    "opportunity_id": "openings:plaintiff",
    "turn_number": 1,
    "deadline": "2026-06-01T15:04:05Z",
    "remaining_ms": 845233,
    "attempts_max": 3,
    "attempts_remaining": 3,
    "completed": false
  },
  "tools": []
}
```

## POST

`POST /lawyerapi/v1/do` requires a JSON body with `case_id`, `role_id`, `tool`, and `arguments`.  Lawyer roles also require `opportunity_id`, copied from the ready `GET` response's `turn.opportunity_id`; observer calls do not use `opportunity_id`.  `arguments` is an object, and an empty object is valid for tools that take no arguments.  `call_id` is optional; the current implementation does not echo it.

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
        "text": "Opening statement text.",
        "offered_evidence": [],
        "technical_reports": []
      }
    }
  }'
```

A successful tool call returns `ok: true` and a tool-specific `result`.  A successful `submit_decision` completes the lawyer turn.  The response still includes the turn object that was active when the server accepted the filing.

```json
{
  "ok": true,
  "case_id": "arb-1",
  "role_id": "plaintiff",
  "turn": {
    "role_id": "plaintiff",
    "phase": "openings",
    "opportunity_id": "openings:plaintiff",
    "turn_number": 1,
    "deadline": "2026-06-01T15:04:05Z",
    "remaining_ms": 831002,
    "attempts_max": 3,
    "attempts_remaining": 3,
    "completed": true
  },
  "result": {
    "text": "Decision accepted."
  }
}
```

A failed tool call returns `ok: false` and an error object.  Rejected mutating calls decrement `attempts_remaining`.  Missing or stale opportunity ids are request guards and do not decrement attempts.  When attempts reach zero, the turn fails and the case run returns an error.

```json
{
  "ok": false,
  "case_id": "arb-1",
  "role_id": "plaintiff",
  "turn": {
    "role_id": "plaintiff",
    "phase": "openings",
    "opportunity_id": "openings:plaintiff",
    "turn_number": 1,
    "deadline": "2026-06-01T15:04:05Z",
    "remaining_ms": 812477,
    "attempts_max": 3,
    "attempts_remaining": 2,
    "completed": false
  },
  "error": {
    "code": "tool_failed",
    "message": "payload.text is required.\nThis is invalid submission 1 of 3 for this opportunity. You have 2 invalid submissions remaining.\nProvide payload.text and resubmit.\nIf you exhaust the remaining invalid submissions, this opportunity will fail and the run will end with an error."
  }
}
```

## Lawyer Tools

| Tool | Phases | Mutates | Meaning |
| --- | --- | --- | --- |
| `get_case` | all lawyer phases | no | Return the visible case record and limits for the role. |
| `list_evidence` | arguments, rebuttals | no | List visible evidence metadata. |
| `stat_evidence` | arguments, rebuttals | no | Return metadata and remaining read budget for one evidence item. |
| `read_evidence_range` | arguments, rebuttals | no | Return a bounded byte range as base64. |
| `submit_evidence` | arguments, rebuttals | yes | Submit one direct evidence item with provenance. |
| `begin_evidence_upload` | arguments, rebuttals | yes | Start a chunked upload. |
| `write_evidence_chunk` | arguments, rebuttals | yes | Append one base64 chunk to an upload. |
| `commit_evidence_upload` | arguments, rebuttals | yes | Verify and admit a completed upload. |
| `submit_decision` | all lawyer phases | yes | Submit the phase legal act or a permitted pass. |

`submit_decision` wraps the legal act for the current opportunity.  `kind: "tool"` requires `tool_name` to match one of the allowed legal acts in the current turn, such as `record_opening_statement`, `submit_argument`, `submit_rebuttal`, `submit_surrebuttal`, or `deliver_closing_statement`.  `kind: "pass"` is valid only when the current opportunity allows a pass.

## Observer Tools

| Tool | Meaning |
| --- | --- |
| `get_case` | Return the current arbitration record. |
| `get_turn` | Return the active turn role, phase, deadline, remaining time, and attempts. |
| `list_events` | Return recorded case events, with optional `offset` and `limit`. |
| `list_evidence` | List visible evidence metadata. |
| `stat_evidence` | Return metadata for one evidence item. |
| `read_evidence_range` | Return a bounded byte range as base64. |

An observer `get_turn` response returns the same turn object used in the response envelope.  If no lawyer turn is active, the result is `{"turn": null}`.  Observer evidence reads are bounded by the per-read byte limit.

## Error Codes

| Code | Meaning |
| --- | --- |
| `missing_case_id` | `case_id` is missing or empty. |
| `missing_role_id` | `role_id` is missing or empty. |
| `missing_opportunity_id` | A lawyer `POST /do` omitted the active `turn.opportunity_id`. |
| `stale_opportunity` | A lawyer `POST /do` named an opportunity other than the active one. |
| `missing_tool` | `tool` is missing or empty. |
| `bad_json` | The POST body is missing or cannot be decoded as JSON. |
| `method_not_allowed` | The endpoint was called with the wrong HTTP method. |
| `invalid_role` | `role_id` is not valid for the endpoint. |
| `no_active_turn` | A lawyer called `do` while no lawyer turn was active. |
| `not_current_turn` | A lawyer called `do` while another lawyer role had the active turn. |
| `turn_timeout` | The active turn deadline expired before the call completed. |
| `tool_failed` | The tool was unknown, unavailable, malformed, or rejected by procedural validation. |
