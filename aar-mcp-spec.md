# AAR MCP Specification

## Scope

This specification defines the external behavior of `aar-mcp`.  The process exposes one Model Context Protocol endpoint for lawyer, observer, and council assignments, and forwards tool calls to the AAR service Lawyer and Council HTTP APIs.  The AAR service and the per-case runner remain the authorities for case state, turn order, deadlines, attempt budgets, evidence custody, filing validation, council voting, and terminal artifacts.

This specification assumes the service topology described in [AAR Process And HTTP Specification](aar-spec.md).  `aar service` runs the public HTTP manager, starts `aar case` child processes, and routes role API calls by `case_id`.  `aar-mcp` stores only MCP session state: the session id, `case_id`, assignment type, principal id, creation time, and last-seen time.

## Process Model

`aar-mcp` starts one long-running HTTP server.  It requires a Lawyer API base URL and a Council API base URL, normally the public service routes `http://HOST/lawyerapi/v1` and `http://HOST/councilapi/v1`.  It accepts optional bearer authentication for MCP clients and optional bearer authentication for the AAR service APIs it calls.

The process supports these command-line options:

| Option | Meaning |
| --- | --- |
| `--listen` | HTTP listen address.  Default: `127.0.0.1:19780`. |
| `--lawyerapi-base` | Required public Lawyer API base URL ending in `/lawyerapi/v1`. |
| `--councilapi-base` | Required public Council API base URL ending in `/councilapi/v1`. |
| `--bearer-token` | Optional bearer token required from MCP clients. |
| `--api-bearer-token` | Optional bearer token sent to AAR service API requests. |
| `--session-ttl` | Idle MCP session lifetime.  Default: `30m`; `0` disables expiry. |
| `--session-cleanup-interval` | Interval for background deletion of expired sessions.  Default: `1m`. |
| `--allow-origin` | Additional allowed HTTP `Origin` value.  May be repeated. |

The process writes a startup line with the `/mcp` URL to stderr.  Clients and tests should use `GET /health` to verify readiness because the log line is process output, not a protocol response.  `GET /health` returns HTTP `204` and does not require bearer authentication.

## HTTP Transport

The MCP endpoint is `/mcp`.  The implementation uses Streamable HTTP-style JSON-RPC over POST, but it does not provide a server-sent event stream.  `GET /mcp` returns HTTP `405`; clients must use POST for JSON-RPC requests and DELETE for explicit session deletion.

Requests to `/mcp` require `Authorization: Bearer {token}` when `--bearer-token` is configured.  Requests without an `Origin` header are allowed.  Requests with an `Origin` header are allowed when the origin is localhost, `127.0.0.1`, `::1`, or one of the configured `--allow-origin` values.

POST bodies are JSON-RPC 2.0 objects.  Parse failures return JSON-RPC error `-32700`.  Invalid JSON-RPC requests return `-32600`, unknown methods return `-32601`, and invalid method parameters return `-32602`.  JSON-RPC notifications, meaning requests without an `id`, return HTTP `202` and do not execute a method.

## Session Model

An MCP client creates a session by sending `initialize` to `/mcp` with assignment parameters in the URL query.  The URL must include `case_id` and exactly one principal field.  Lawyer and observer sessions use `role_id`; council sessions use `member_id`.

| URL shape | Assignment |
| --- | --- |
| `/mcp?case_id=CASE&role_id=plaintiff` | Plaintiff lawyer for `CASE`. |
| `/mcp?case_id=CASE&role_id=defendant` | Defendant lawyer for `CASE`. |
| `/mcp?case_id=CASE&role_id=observer` | Read-only observer for `CASE`. |
| `/mcp?case_id=CASE&member_id=C1` | Council member `C1` for `CASE`. |

The server rejects initialization when `case_id` is missing, when both `role_id` and `member_id` are present, or when neither is present.  `role_id` must be `plaintiff`, `defendant`, or `observer`.  `member_id` must be non-empty after trimming whitespace.

A successful `initialize` response includes the `Mcp-Session-Id` response header.  Every later `tools/list`, `tools/call`, `ping`, or DELETE request for that session must send `Mcp-Session-Id` with the returned value.  Missing session ids return HTTP `400`; unknown or expired session ids return HTTP `404`.

Session expiry deletes adapter session state only.  It does not change the AAR case, does not close an opportunity, and does not consume an attempt.  A client that loses a session may initialize a new session with the same `case_id` and principal, then recover the current assignment state through `wait_for_opportunity` or `get_current_opportunity`.

DELETE `/mcp` with `Mcp-Session-Id` deletes the session when it exists and returns HTTP `204`.  Deleting an unknown session also returns HTTP `204`.  Deletion affects the adapter session only; it does not cancel the case or submit anything to AAR.

## Tool Sets

The adapter exposes a stable tool set for each assignment type.  The tool list does not change when the AAR phase changes.  The current opportunity returned by AAR determines which tools can affect the record at that moment; AAR rejects calls that conflict with the current role, phase, opportunity id, deadline, attempt budget, or tool policy.

All sessions expose these tools:

| Tool | Purpose |
| --- | --- |
| `wait_for_opportunity` | Wait up to 30 seconds for a ready opportunity or case-status change. |
| `get_current_opportunity` | Return current prompt, turn, tools, limits, remaining time, and attempts for this assignment. |

Plaintiff and defendant lawyer sessions also expose:

| Tool | Purpose |
| --- | --- |
| `case_status` | Return the current case phase, active turn, role status, and counts. |
| `get_case` | Return the current visible arbitration record. |
| `get_case_result` | Return final case results, or pending status while the case remains open. |
| `send_work_notes` | Send private work notes for operator analysis outside the record. |
| `list_evidence` | List visible immutable record evidence. |
| `stat_evidence` | Return metadata and read limits for one visible evidence item. |
| `read_evidence_range` | Read a bounded byte range from visible evidence as base64. |
| `begin_evidence_upload` | Begin a chunked evidence upload. |
| `write_evidence_chunk` | Write one base64 chunk into an upload session. |
| `commit_evidence_upload` | Verify and admit a completed evidence upload. |
| `submit_evidence` | Submit source evidence with provenance. |
| `submit_decision` | Submit the final legal act for the current opportunity. |

Observer sessions also expose:

| Tool | Purpose |
| --- | --- |
| `case_status` | Return current case status and counts. |
| `get_case` | Return the current arbitration record. |
| `get_case_result` | Return final case results, or pending status while the case remains open. |
| `get_turn` | Return the current turn role, phase, deadline, and attempts. |
| `list_events` | List recorded case events. |
| `list_evidence` | List visible immutable record evidence. |
| `stat_evidence` | Return metadata for one visible evidence item. |
| `read_evidence_range` | Read a bounded byte range from visible evidence as base64. |

Council sessions also expose:

| Tool | Purpose |
| --- | --- |
| `get_case` | Return the current visible arbitration record for this council member. |
| `list_evidence` | List visible immutable record evidence. |
| `stat_evidence` | Return metadata and read limits for one visible evidence item. |
| `read_evidence_range` | Read a bounded byte range from visible evidence as base64. |
| `submit_council_vote` | Submit one vote for the current deliberation opportunity. |

## Wait Semantics

`wait_for_opportunity` forwards to the service role API `/wait` endpoint for the session assignment.  It accepts `timeout_ms`, `after_version`, and `after_opportunity_id`.  `timeout_ms` defaults to 30 seconds and is capped at 30 seconds; callers should repeat the tool call while the returned state is `waiting`.

The returned structured content includes the AAR response plus adapter fields.  `state` is one of `ready`, `waiting`, `done`, `failed`, or `error`.  When AAR returns a wait version, the adapter copies it to `after_version`; when AAR returns a current opportunity id, the adapter copies it to `after_opportunity_id`.

The client loop is deterministic.  If `state` is `waiting`, call `wait_for_opportunity` again with the returned `after_version` when present.  If `state` is `ready`, read the returned prompt, turn, limits, tools, remaining time, attempts, and opportunity id, then complete the opportunity.  If `state` is `done` or `failed`, stop acting on the assignment; if `state` is `error`, report the error to the operator.

## Tool Calls And Forwarding

For `get_current_opportunity`, the adapter sends `GET /get` to the session's AAR role API.  For lawyer and observer `case_status`, it sends `GET /status`; for lawyer and observer `get_case_result`, it sends `GET /result`.  All other tool calls use the service role API `POST /do`.

Before forwarding an ordinary tool call, the adapter fetches the current assignment state through `GET /get`.  It then builds the AAR `POST /do` body with the bound `case_id`, the bound `role_id` or `member_id`, the requested tool name, and the tool arguments.  For plaintiff, defendant, and council member sessions, the adapter injects the current `turn.opportunity_id`; observer sessions do not receive an injected opportunity id.

The adapter does not decide whether a legal act is timely, complete, or allowed in the current phase.  AAR makes that decision and returns the result.  The MCP tool result always includes `structuredContent`; clients should inspect `structuredContent.ok`, `structuredContent.status`, and `structuredContent.state`, rather than relying only on the MCP `isError` flag.

## Error And Result Shape

A successful `tools/call` JSON-RPC response contains an MCP tool result.  The result has text content, `structuredContent` containing the AAR or adapter JSON object, and `isError` indicating whether the adapter considered the tool call unsuccessful.  The text content begins with a compact summary of keys such as `ok`, `status`, `state`, `message`, `after_version`, `after_opportunity_id`, `role_id`, `member_id`, and `case_id`, then includes the full JSON object so MCP clients that expose only text content still receive evidence lists, case records, read bytes, prompts, and tool results.

HTTP errors from the AAR service become structured tool content where possible.  Non-2xx AAR responses are decoded as JSON when the service returns JSON, tagged with `ok: false`, and include `http_status`.  Transport failures, malformed JSON, and wait-tool failures produce `state: "error"` or an adapter error object.

JSON-RPC errors are reserved for MCP request failures: malformed JSON-RPC, missing session headers, unknown methods, invalid initialization bindings, and invalid `tools/call` parameters.  A rejected filing, stale opportunity id, bad evidence request, missing case, inactive runner, or exhausted attempt budget belongs in the MCP tool result because AAR processed the tool request.

## Authentication And Origins

`--bearer-token` protects the MCP endpoint.  It is the token remote agents present in the MCP server definition.  The health endpoint remains unauthenticated so process supervisors and local tests can verify readiness without an assignment token.

`--api-bearer-token` protects calls from `aar-mcp` to `aar service`.  The adapter adds that token as `Authorization: Bearer {token}` on every forwarded AAR HTTP request.  The adapter must not log either bearer token.

Origin checks apply when a client sends an `Origin` header.  Localhost origins are allowed by default.  Browser clients from other origins require explicit `--allow-origin` values.

## Logging And Observability

The adapter writes transport logs to stderr.  It logs session creation, session deletion, assignment type, case id, principal id, wait results, forwarded tool names, HTTP status, and AAR `ok` values.  It must not log bearer tokens, full tool payloads, evidence bytes, lawyer filings, council rationales, or private work notes.

The AAR case packet remains the source of record evidence, filings, events, work notes, votes, and final results.  MCP logs explain transport behavior only.  When a run fails, operators should read the MCP log with the service registry, child stdout and stderr logs, `run.json`, `events.ndjson`, `work-notes.ndjson`, and the evidence manifest.
