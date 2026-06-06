# Council HTTP API

## Purpose

The Council API lets an outside process act as one council member in an arbitration.  AAR owns the case state, turn order, deadlines, attempt counts, admitted evidence, vote validation, and event log.  A caller uses HTTP to learn whether its council member has a deliberation opportunity, inspect the admitted record, read bounded evidence ranges, and submit one vote for that opportunity.

The first implementation serves one case from one `aar case` process.  Each request includes `case_id` and `member_id`; `case_id` is required and returned unchanged, but the server does not use it to select a case yet.  Front ends such as a CLI, MCP adapter, or agent runner should be separate clients built on this API.

## Research Notes

The same HTTP-plus-MCP architecture used for lawyers applies to council members.  The HTTP API remains the enforcement layer.  MCP remains an adapter for agents that know how to call MCP tools, and the adapter does not need arbitration-specific state beyond the bound `case_id`, `member_id`, and Council API base URL.

Pi does not need built-in council support for this design.  The relevant Pi materials show SDK and RPC extension points, plus MCP extension packages, so a Pi or OpenClaw-side integration can call a Streamable HTTP MCP adapter rather than changing the AAR runner.  Relevant references are the Pi home page (`https://pi.dev/`), `pi-mcp-extension` (`https://pi.dev/packages/pi-mcp-extension`), `@cansiny0320/pi-mcp-adapter` (`https://pi.dev/packages/%40cansiny0320/pi-mcp-adapter`), the SDK docs (`https://pi.dev/docs/latest/sdk`), and the RPC docs (`https://pi.dev/docs/latest/rpc`).

The remote council path maps to HTTP tools.  `get_case`, `list_evidence`, `stat_evidence`, `read_evidence_range`, and `submit_council_vote` are the useful remote tools.  `materialize_evidence` belongs only to a local workspace adapter and should not appear in the remote Council API because the HTTP server owns the evidence store.

## Endpoints

| Method | Path | Meaning |
| --- | --- | --- |
| `GET` | `/councilapi/v1/get?case_id={case_id}&member_id={member_id}` | Return member status, prompt, tools, limits, and the live turn budget. |
| `GET` | `/councilapi/v1/wait?case_id={case_id}&member_id={member_id}` | Wait for member status or case state to change, then return the same status shape as `GET /get`. |
| `POST` | `/councilapi/v1/do` | Execute one tool call for the council member. |

All responses are JSON.  HTTP status codes describe request handling, and the response body uses `ok` to report whether the requested API operation succeeded.  Well-formed tool calls that fail procedural validation usually return HTTP `200` with `ok: false`, because the server understood the request and rejected the attempted action.

## Turn Budget

Each council deliberation opportunity has one deadline and one attempt budget.  The deadline covers the whole turn, including malformed tool calls and corrected retries.  The attempt budget counts rejected mutating tool calls, including invalid `submit_council_vote` calls.

Every `/get`, `/wait`, and `/do` response includes a `turn` field.  When a council turn is active, `turn` contains `role_id: "council"`, the member id, phase, opportunity id, deliberation round, deadline, live `remaining_ms`, `attempts_max`, and `attempts_remaining`.  A council client copies `turn.opportunity_id` into each `POST /do` request for that turn.

## GET

`GET /councilapi/v1/get` requires `case_id` and `member_id`.  When the requested member has the active deliberation opportunity, the response has `status: "ready"`, a prompt, the available tools, limits, and the active `turn.opportunity_id`.  When AAR is waiting on lawyers or another council member, the response has `status: "waiting"` and no tools.

```bash
curl -sS "$BASE/get?case_id=arb-1&member_id=C1"
```

## WAIT

`GET /councilapi/v1/wait` requires `case_id` and `member_id`.  It accepts optional `after`, `after_version`, and `timeout_ms` query parameters.  `after` is an opportunity id the caller has already seen, `after_version` is the `wait.version` returned by an earlier wait response, and `timeout_ms` is capped by the server.

```bash
curl -sS "$BASE/wait?case_id=arb-1&member_id=C1&after=deliberation:1:C1&after_version=12&timeout_ms=30000"
```

When `wait.reason` is `timeout`, the caller should call `wait` again with the returned `wait.version` as `after_version`.  When the member has an actionable opportunity, the response has `status: "ready"` and includes prompt, tools, limits, and the active `turn.opportunity_id`.  When the case reaches a terminal result while a wait request is active, the response has `status: "done"` and `final_reason`.

## POST

`POST /councilapi/v1/do` requires a JSON body with `case_id`, `member_id`, `opportunity_id`, `tool`, and `arguments`.  `opportunity_id` must match the active opportunity for that council member.  The server injects the trusted `member_id` into `submit_council_vote`; it ignores any `member_id` supplied inside `arguments`.

```bash
curl -sS -X POST "$BASE/do" \
  -H 'content-type: application/json' \
  --data '{
    "case_id": "arb-1",
    "member_id": "C1",
    "opportunity_id": "deliberation:1:C1",
    "tool": "submit_council_vote",
    "arguments": {
      "vote": "demonstrated",
      "rationale": "The admitted record satisfies the evidence standard."
    }
  }'
```

## MCP Adapter

`aar-council-mcp` is a thin Streamable HTTP MCP adapter over the Council API.  One MCP session binds to one `case_id` plus one `member_id`, using a URL such as `/mcp?case_id=arb-1&member_id=C1`.  If the MCP session expires or the connection fails, the agent can initialize again with the same URL and recover current status from AAR.

The adapter exposes `wait_for_council_opportunity` and `get_current_council_opportunity` for every session.  It also exposes the Council API tools that AAR returns for the current turn.  The agent loop is: wait, act only when `state: ready`, submit exactly one vote when ready, then wait again until the case is done.
