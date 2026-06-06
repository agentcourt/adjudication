# AARD MCP Specification

## Scope

This specification defines the MCP adapter behavior for AARD.  The adapter exposes one Streamable HTTP MCP endpoint for lawyer, observer, and council-member sessions.  It forwards tool calls to the AARD case HTTP APIs described in [AARD Process And HTTP Specification](aard-spec.md).

The case process remains authoritative for phase, role authority, deadlines, attempts, evidence custody, filing validation, council answer validation, final artifacts, and failure handling.  The MCP adapter stores session state and translates MCP tool calls into HTTP requests.  The adapter's degree-specific authority is limited to binding each session to one case and one principal.

## Process Model

`aard mcp` starts one HTTP server.  It requires `--caseapi-base`, which points to a private `aard case` base URL or to an AARD service base URL.  It accepts optional MCP bearer authentication, optional API bearer authentication for calls to the service, idle session expiry, cleanup interval, and allowed origins.

The server prints a startup line with the `/mcp` URL after binding.  Clients and tests should use `GET /health` to verify readiness.  The health endpoint returns HTTP `204` and permits unauthenticated requests.

## Sessions

An MCP client creates a session by sending JSON-RPC `initialize` to `/mcp` with assignment parameters in the URL query.  Each session binds to one case and one principal.  Later JSON-RPC calls for that session must include the `Mcp-Session-Id` response header returned by initialization.

| URL shape | Assignment |
| --- | --- |
| `/mcp?case_id=CASE&role_id=plaintiff` | Plaintiff lawyer for `CASE`. |
| `/mcp?case_id=CASE&role_id=defendant` | Defendant lawyer for `CASE`. |
| `/mcp?case_id=CASE&role_id=observer` | Read-only observer for `CASE`. |
| `/mcp?case_id=CASE&member_id=C1` | Council member `C1` for `CASE`. |

The server rejects initialization when `case_id` is missing, both `role_id` and `member_id` are present, or neither principal field is present.  `role_id` must be `plaintiff`, `defendant`, or `observer`.  `member_id` must be non-empty after trimming whitespace.

Session expiry deletes adapter session state.  The case state remains with AARD.  A client that loses a session can initialize a new session with the same assignment and then call `wait_for_opportunity` or `get_current_opportunity` to recover current state.

## Tools

The MCP tool set is stable for each session type.  AARD determines whether the current phase allows a tool call.  If a caller tries a tool that conflicts with phase, role, opportunity id, deadline, attempt budget, or evidence policy, AARD rejects the call through the forwarded HTTP response.

All sessions expose `wait_for_opportunity` and `get_current_opportunity`.  Lawyer sessions also expose `case_status`, `get_case`, `get_case_result`, `send_work_notes`, evidence readers, evidence submission, evidence upload, and `submit_decision`.  Observer sessions expose read-only status, case, result, turn, event, and evidence tools.  Council sessions expose `get_case`, evidence readers, and `submit_council_answer`.

Evidence-reading tools are available throughout the lawyer sequence.  Evidence-submission tools are available during arguments, rebuttals, and surrebuttals.  Council sessions receive read-only evidence tools and answer submission.

## Wait And Forwarding

`wait_for_opportunity` forwards to the bound role API `/wait` endpoint.  It waits no longer than 30 seconds, then returns a structured state: `ready`, `waiting`, `done`, `failed`, or `error`.  A caller that receives `waiting` should call the tool again with the returned wait version when present.

`get_current_opportunity` forwards to `/get`.  The response includes the current prompt, turn, limits, tools, remaining time, attempts left, and case status for the bound assignment.  Lawyers and council members should finish one ready opportunity before waiting again.

For ordinary tool calls, the adapter fetches current state, injects the active `opportunity_id` for plaintiff, defendant, and council sessions, and forwards the call to `/do`.  Observer sessions forward read-only tools without an injected opportunity id.  The forwarded body always uses the session-bound `case_id` and principal.

## Results And Errors

MCP tool results include structured content containing the forwarded AARD response or adapter error.  Clients should inspect `structuredContent.ok`, `structuredContent.status`, and `structuredContent.state`.  Rejected legal acts and rejected evidence actions are tool results because AARD processed the request.

JSON-RPC errors are reserved for MCP transport and protocol problems: malformed JSON, invalid JSON-RPC, unknown methods, invalid initialization, missing session headers, and unknown sessions.  HTTP errors from AARD become structured MCP tool content when the adapter can decode them.

The adapter must not log bearer tokens, full filings, evidence bytes, council rationales, or work-note payloads.  It may log session creation, case id, principal, tool names, forwarded HTTP status, and `ok` values.  The case packet remains the source for filings, evidence, work notes, answers, events, and final results.
