# AAR MCP Test Plan

## Scope

This plan tests the behavior specified in [AAR MCP Specification](aar-mcp-spec.md).  Tests should exercise the MCP adapter as an HTTP and JSON-RPC process that forwards to AAR service role APIs.  Tests should not depend on OpenClaw, Pi, live model calls, browser tools, or direct Lean calls.

The core boundary is the adapter boundary.  `aar-mcp` owns authentication, session binding, session expiry, stable tool lists, wait normalization, opportunity-id injection, and transport logging.  AAR owns case state, role authorization, phase rules, attempt budgets, evidence validation, and final results.

## Harnesses

Use three harnesses.  The unit harness should instantiate `mcpServer` with `httptest` and fake AAR HTTP servers.  The process harness should run `.bin/aar-mcp` against fake AAR HTTP servers and communicate over real HTTP.  The service harness should run `aar service`, `aar-mcp`, and a small AAR case, then drive assignments through MCP JSON-RPC.

Each test should retain useful logs on failure.  Process tests should keep adapter stdout, adapter stderr, fake AAR request logs, JSON-RPC requests, JSON-RPC responses, and the temporary service output directory.  Service tests should also retain the service registry, child stdout and stderr logs, `run.json`, `events.ndjson`, `work-notes.ndjson`, and MCP logs.

## Test Matrix

| ID | Harness | Case | Expected Result |
| --- | --- | --- | --- |
| MCP-1 | Unit | Initialize sessions | Valid lawyer, observer, and council sessions receive `Mcp-Session-Id`; invalid bindings return JSON-RPC errors. |
| MCP-2 | Unit | Authentication and origins | Bearer-token and origin checks accept configured clients and reject unauthorized clients. |
| MCP-3 | Unit | Session lifecycle | Missing, unknown, expired, and deleted sessions produce the documented HTTP results. |
| MCP-4 | Unit | Tool lists | Lawyer, observer, and council tool sets match the assignment authority. |
| MCP-5 | Unit | Wait normalization | `/wait` responses map to `ready`, `waiting`, `done`, `failed`, and `error` states. |
| MCP-6 | Unit | Opportunity injection | Mutating lawyer and council calls include the current `opportunity_id`; observer calls do not. |
| MCP-7 | Unit | AAR error propagation | AAR `ok: false`, non-2xx, and transport failures become structured MCP tool results. |
| MCP-8 | Process | Startup and health | `.bin/aar-mcp` starts with required bases, serves `/health`, and rejects invalid flags. |
| MCP-9 | Process | Logs | Adapter logs contain session and forwarding metadata without tokens or payloads. |
| MCP-10 | Service | Lawyer assignment | A plaintiff or defendant can wait, inspect evidence, send notes, and submit a legal act through MCP. |
| MCP-11 | Service | Observer assignment | An observer can inspect status, case data, events, evidence, and final results without mutating the case. |
| MCP-12 | Service | Council assignment | A council member can wait, inspect the record, read evidence, and submit one vote through MCP. |
| MCP-13 | Service | Terminal states | Completed and failed cases produce `done` or `failed` wait results without requiring access to private runner ports. |

## Unit Tests

MCP-1 should cover initialization with `case_id` plus each valid principal shape.  It should assert the returned `Mcp-Session-Id`, protocol version, server info, tools capability, and session instructions.  It should reject missing `case_id`, missing principal, mixed `role_id` and `member_id`, and invalid `role_id`.

MCP-2 should configure `--bearer-token` behavior at the handler level.  Requests without the token and requests with the wrong token should return HTTP `401`.  Requests with no `Origin`, localhost origins, and configured `--allow-origin` values should pass; unconfigured non-local origins should return HTTP `403`.

MCP-3 should assert session header behavior.  `tools/list` without `Mcp-Session-Id` should return HTTP `400` with a JSON-RPC error.  Unknown and expired session ids should return HTTP `404`, and DELETE with a known session id should return HTTP `204` and remove the session.

MCP-4 should assert exact tool names by assignment.  Lawyer sessions should include `submit_decision`, `submit_evidence`, upload tools, `send_work_notes`, evidence readers, `case_status`, and `get_case_result`.  Observer sessions should include read-only status, record, result, turn, event, and evidence tools, and should not include evidence submission, work notes, or decision tools.  Council sessions should include `submit_council_vote` and evidence readers, and should not include lawyer filing tools.

MCP-5 should fake AAR `/wait` responses for all status classes.  `ready` should become `state: "ready"` and carry `after_opportunity_id` when a turn exists.  `waiting` should become `state: "waiting"` and carry `after_version` when the AAR wait object has a version.  `done`, `failed`, AAR `ok: false`, and non-2xx responses should map to `done`, `failed`, or `error` as specified.

MCP-6 should fake AAR `/get` and `/do`.  For a plaintiff or defendant call, the forwarded JSON body must contain the bound `case_id`, the bound `role_id`, the called tool, original arguments, and the active `opportunity_id` from `/get`.  For a council call, the body must contain `member_id` and the active council `opportunity_id`.  For observer calls, the body must not contain an injected `opportunity_id`.

MCP-7 should test tool result shape rather than only HTTP status.  AAR `ok: false` responses should become MCP tool results with `structuredContent.ok: false`.  AAR non-2xx JSON responses should include `http_status` in structured content.  A transport failure should produce a structured adapter error and set a state or error field that the client can report to the operator.

## Process Tests

MCP-8 should build `.bin/aar-mcp`, start it with fake Lawyer and Council API base URLs, and poll `/health` until it returns HTTP `204`.  It should assert startup failures for missing required API bases, malformed API base URLs, negative `--session-ttl`, and non-positive cleanup interval when expiry is enabled.  It should assert `GET /mcp` returns HTTP `405` and POST JSON-RPC remains the only request path for active MCP calls.

MCP-9 should start the process with known tokens and issue initialize, tools/list, wait, and tool-call requests.  The captured stderr should contain session creation, wait, forwarded tool names, HTTP status, and AAR `ok` values.  The same log must not contain bearer-token values, full tool arguments, evidence content, filing text, vote rationales, or private work notes.

Process tests should avoid fixed sleeps.  They should start the process, poll `/health`, run JSON-RPC calls, then stop the process through the test context or process signal.  A failed test should retain adapter stdout, adapter stderr, and fake AAR request logs.

## Service Tests

MCP-10 should run `aar service`, `aar-mcp`, and a small case with `council_backend: "councilapi"`.  The test should initialize a plaintiff or defendant MCP session, call `wait_for_opportunity`, inspect the returned prompt and turn, call evidence readers, call `send_work_notes`, and submit a valid `submit_decision`.  It should assert that AAR receives the injected opportunity id and that work notes appear in `work-notes.ndjson` outside the case record.

MCP-11 should initialize an observer session for the same service-managed case.  It should call `case_status`, `get_case`, `get_turn`, `list_events`, evidence readers, and `get_case_result`.  It should attempt a mutating lawyer or council tool and assert that the call is rejected without changing AAR state.

MCP-12 should advance a case to deliberation, initialize a council member session, call `wait_for_opportunity`, inspect record evidence, and submit `submit_council_vote`.  The test should assert that the forwarded vote uses the bound `member_id`, not a caller-supplied member id inside arguments.  It should also assert that a lawyer session cannot receive or use council vote authority.

MCP-13 should cover terminal behavior.  For a successful case, `wait_for_opportunity` should return `state: "done"` and `get_case_result` should include final resolution, vote tally, council votes, rationales, final reason, and deliberation round.  For a lawyer-failed case, a service-managed lawyer or observer session should return `state: "failed"` or terminal result data that matches `run.json`.  For a council-member failure that does not fail the case, the failed council session should report failure while other council sessions can continue if AAR rules allow.

## Assertions And Artifacts

Every MCP test should assert the JSON-RPC envelope, not only HTTP status.  Successful method calls should use `jsonrpc: "2.0"`, preserve the request id, and place tool data under `result`.  JSON-RPC request errors should use the documented error code; AAR role errors should appear in MCP tool structured content.

Every forwarding test should inspect the fake AAR request body.  The adapter must send the bound case and principal, must add `Authorization` only when `--api-bearer-token` is configured, and must inject opportunity ids only where AAR requires them.  The adapter must not trust caller-supplied `case_id`, `role_id`, `member_id`, or `opportunity_id` inside tool arguments.

Every service test should inspect final artifacts when the case reaches a terminal state.  The service registry, child stdout summary, `run.json`, `events.ndjson`, and MCP logs should agree on whether the case completed, failed, or continued after council-member failure.  The test should use AAR artifacts as the source of case truth and MCP logs as transport diagnostics.

## Minimum Passing Set

The first passing set should include MCP-1 through MCP-8.  That set covers session binding, security gates, session lifetime, tool authority, wait normalization, opportunity injection, error propagation, and process startup.  It does not require live agents or model calls.

The next passing set should add MCP-9 through MCP-13.  Those tests cover transport logs, real service routing, lawyer assignments, observer assignments, council assignments, and terminal states.  They should still use test clients that speak MCP directly; OpenClaw and Pi example runs belong after these tests pass because they diagnose agent behavior as well as adapter behavior.
