# Lawyer HTTP API

## Purpose

The Lawyer API lets an outside process act as a plaintiff lawyer, defendant lawyer, or observer in an arbitration.  AAR owns the case state, turn order, evidence store, filing validation, deadlines, attempts, and event log.  A caller uses HTTP to ask what the role can do, then executes one returned tool at a time.  Lawyers can also send private work notes for operator analysis; those notes are written to `work-notes.ndjson` and do not become part of the case record.

The first implementation serves one case from one `aar case` process.  Each request includes `case_id` and `role_id`; `case_id` is required and returned unchanged, but the server does not use it to select a case yet.  Front ends such as a CLI, MCP server, ACP service, or agent runner should be separate clients built on this API.

## Endpoints

| Method | Path | Meaning |
| --- | --- | --- |
| `GET` | `/lawyerapi/v1/get?case_id={case_id}&role_id={role_id}` | Return role status, prompt, tools, limits, and the live turn budget. |
| `GET` | `/lawyerapi/v1/wait?case_id={case_id}&role_id={role_id}` | Wait for role status or case state to change, then return the same status shape as `GET /get`. |
| `POST` | `/lawyerapi/v1/do` | Execute one tool call for the role. |

All responses are JSON.  HTTP status codes describe request handling, and the response body uses `ok` to report whether the requested API operation succeeded.  Well-formed tool calls that fail procedural validation usually return HTTP `200` with `ok: false`, because the server understood the request and rejected the attempted action.

## Roles

`plaintiff` and `defendant` are lawyer roles.  When the procedure is waiting for one of those roles, `GET` returns `status: "ready"`, the current prompt, the available tools, and live turn information.  When the procedure is waiting for another role or for the council, `GET` returns `status: "waiting"` with no lawyer tools.

`observer` is read-only.  It can inspect the case, current turn, events, and evidence.  It cannot submit decisions, submit evidence, upload evidence, vote, or alter any deadline.

## Turn Budget

Each lawyer opportunity has one deadline and one attempt budget.  The deadline covers the whole turn, including malformed tool calls and corrected retries.  The attempt budget counts rejected mutating tool calls, including invalid `submit_decision` calls.

Every `/get`, `/wait`, and `/do` response includes a `turn` field.  When a lawyer turn is active, `turn` contains the current role, phase, opportunity id, deadline, live `remaining_ms`, `attempts_max`, and `attempts_remaining`.  A lawyer copies `turn.opportunity_id` into each `POST /do` request for that turn.  When no lawyer turn is active, `turn` is `null`.

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
      "name": "send_work_notes",
      "description": "Send private work notes for off-record operator analysis. This does not create evidence, a filing, a technical report, or a case event.",
      "input_schema": {
        "type": "object",
        "properties": {
          "notes": { "type": "string" }
        },
        "required": ["notes"],
        "additionalProperties": false
      },
      "read_only": false
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

## WAIT

`GET /lawyerapi/v1/wait` requires `case_id` and `role_id`.  It accepts optional `after`, `after_version`, and `timeout_ms` query parameters.  `after` is an opportunity id the caller has already seen.  `after_version` is the `wait.version` returned by an earlier wait response.  `timeout_ms` is capped by the server.

```bash
curl -sS "$BASE/wait?case_id=arb-1&role_id=plaintiff&after=openings:plaintiff&after_version=12&timeout_ms=30000"
```

The response uses the same envelope as `GET /get` and adds a `wait` object:

```json
{
  "ok": true,
  "case_id": "arb-1",
  "role_id": "plaintiff",
  "status": "waiting",
  "prompt": "",
  "turn": {
    "role_id": "defendant",
    "phase": "openings",
    "opportunity_id": "openings:defendant",
    "turn_number": 2,
    "deadline": "2026-06-01T15:14:05Z",
    "remaining_ms": 612341,
    "attempts_max": 3,
    "attempts_remaining": 3,
    "completed": false
  },
  "tools": [],
  "wait": {
    "reason": "timeout",
    "version": 14,
    "state_version": 3
  }
}
```

When `wait.reason` is `timeout`, the caller should call `wait` again with the returned `wait.version` as `after_version`.  When the role has an actionable opportunity, the response has `status: "ready"` and includes prompt, tools, limits, and the active `turn.opportunity_id`.  When the case reaches a terminal result while a wait request is active, the response has `status: "done"` and `final_reason`.  A remote runner can use `wait` to block on AAR state without inventing its own sleep interval.

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
| `send_work_notes` | all lawyer phases | off-record log only | Append private work notes to `work-notes.ndjson`. |
| `list_evidence` | all lawyer phases | no | List visible evidence metadata. |
| `stat_evidence` | all lawyer phases | no | Return metadata and remaining read budget for one evidence item. |
| `read_evidence_range` | all lawyer phases | no | Return a bounded byte range as base64. |
| `submit_evidence` | arguments, rebuttals, surrebuttals | yes | Submit one direct evidence item with provenance. |
| `begin_evidence_upload` | arguments, rebuttals, surrebuttals | yes | Start a chunked upload. |
| `write_evidence_chunk` | arguments, rebuttals, surrebuttals | yes | Append one base64 chunk to an upload. |
| `commit_evidence_upload` | arguments, rebuttals, surrebuttals | yes | Verify and admit a completed upload. |
| `submit_decision` | all lawyer phases | yes | Submit the phase legal act or a permitted pass. |

`send_work_notes` takes `{"notes": "..."}`.  The server logs the complete string with timestamp, role, phase, turn number, opportunity id, and optional `call_id`.  Good notes include plans, issue outlines, work logs, search logs, source URLs or identifiers, tools used, scripts or programs written, packages installed, OCR or extraction steps, browser work, adverse checks, errors, reasoning, draft theories, decisions, and unresolved gaps.  The notes do not appear in `events.ndjson`, `state.json`, `transcript.md`, `digest.md`, or the evidence manifest.  The tool does not complete the turn and does not consume invalid-attempt budget.

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
