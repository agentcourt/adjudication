# AAR Service Design

## Purpose

This design describes a service form for Agent Arbitration.  The current runtime can run a case, expose a Lawyer HTTP API, expose a Council HTTP API when selected, and front those APIs through one MCP server.  The target service keeps the existing case runner as the authority for a single case while adding one public HTTP API server and one MCP server for many concurrent cases.

The design preserves the current division of responsibility.  A case runner owns state, turn order, deadlines, attempt budgets, evidence custody, engine calls, and final artifacts.  The new service manager starts and tracks case runner processes, assigns stable case ids, routes HTTP calls by case id, and exposes operator functions such as starting, listing, inspecting, canceling, and reading case outputs.

The design also replaces the two MCP adapter processes with one MCP process.  One MCP session still binds to one assignment, either a lawyer role or a council member.  The server keeps those authorities separate inside one process, so a plaintiff session never receives council vote tools and a council session never receives lawyer filing or evidence-submission tools.

## Current State

The `aar case` command runs one arbitration case in one OS process.  That process creates one `runContext`, loads the complaint and case files, samples or configures the council, starts the Lawyer API, optionally starts the Council API, and loops through engine opportunities until the case terminates.  The process writes the final packet into one output directory and prints one JSON summary to standard output.

The Lawyer API belongs to the case process.  It serves `/lawyerapi/v1/get`, `/lawyerapi/v1/wait`, `/lawyerapi/v1/status`, `/lawyerapi/v1/result`, and `/lawyerapi/v1/do`.  It binds clients by `case_id` and `role_id`, but the process contains only one `runContext`, so the current `case_id` parameter identifies the intended case for callers rather than selecting among multiple case states.

The Council API also belongs to the case process.  It serves `/councilapi/v1/get`, `/councilapi/v1/wait`, and `/councilapi/v1/do` when the case runs with `--council-backend=councilapi`.  It binds clients by `case_id` and `member_id`, but it has the same single-runner shape as the Lawyer API.  It publishes one active council opportunity at a time and leaves vote validation, evidence-read budgets, and member removal with the runner.

One MCP server sits in front of those APIs.  It serves lawyer, observer, and council sessions by forwarding MCP calls to the service role APIs.  The service routes those calls by `case_id` to the runner that owns the case.

## Target Topology

The service topology has three process classes.  One manager process exposes the public HTTP API and owns the case registry.  One case runner subprocess runs each active case.  One MCP process exposes Streamable HTTP MCP and forwards assignment calls to the manager's public Lawyer and Council API routes.

| Process | Count | Responsibility |
|---|---:|---|
| `aar service` | 1 | Starts cases, records case metadata, routes HTTP calls, exposes operator endpoints, and reads case artifacts. |
| `aar case` | One per active case | Runs one arbitration case, owns the `runContext`, exposes private role APIs, and writes the case packet. |
| `aar-mcp` | 1 | Serves MCP sessions for lawyer roles, observers, and council members by forwarding to `aar service`. |

Each case runner listens on private localhost ports assigned by the manager.  A runner started for case `arb-20260602-0001` might expose `http://127.0.0.1:21431/lawyerapi/v1` and `http://127.0.0.1:21432/councilapi/v1`.  The public clients never need those private addresses.  They call the manager, and the manager proxies to the correct runner after resolving the case id.

## Service Manager

The manager is the public HTTP service for AAR.  It accepts case creation requests, allocates a case id, chooses a fresh output directory, starts a runner subprocess, captures the runner's private API base URLs, and records the case in an in-memory registry backed by a small persistent registry file.  The manager also exposes status and artifact endpoints so an operator can inspect active and completed cases without reading private runner logs by hand.

The manager does not implement arbitration rules.  It does not call the Lean engine, validate filings, mutate case state, or decide whether a lawyer or council member may act.  It routes requests to the runner that owns the case, and the runner keeps the current authority boundary for every procedural decision.

The manager should start child runner processes rather than embedding several `runContext` values in one process for the first service version.  A child process gives each case a separate failure boundary, output directory, process id, API listener set, and cancellation target.  That structure also matches the code that already exists, so the first service implementation can concentrate on routing and lifecycle rather than rewriting the runner.

### Case Registry

The manager registry records one row per case.  The registry lives in memory for low-latency routing and persists to disk for restart recovery.  The on-disk file can use JSON lines or one JSON object per case under a registry directory, but each write must be atomic through rename so a manager crash does not leave a partial case record.

| Field | Meaning |
|---|---|
| `case_id` | Stable public case id. |
| `run_id` | Runner run id passed to `aar case`. |
| `pid` | Child process id when the runner is active. |
| `status` | `starting`, `running`, `completed`, `failed`, or `canceled`. |
| `complaint_path` | Complaint file path supplied at creation. |
| `out_dir` | Output directory for the case packet. |
| `lawyerapi_base` | Private Lawyer API base URL for this runner. |
| `councilapi_base` | Private Council API base URL when the runner exposes one. |
| `created_at` | Creation time in UTC. |
| `started_at` | Runner start time in UTC. |
| `finished_at` | Runner finish time in UTC when known. |
| `exit_code` | Runner exit code when known. |
| `summary` | Parsed final summary JSON from runner stdout when available. |
| `error` | Startup, runtime, or exit error text when the case fails. |

The manager must distinguish process state from case result.  A case can have a finished process and an error summary, because `aar case` prints JSON errors to stdout and exits nonzero.  A case can also have a killed process after cancellation, and the registry should record that as `canceled` rather than `failed`.

### Case Creation

The primary creation endpoint accepts the same inputs as `aar case`, expressed as JSON.  The manager validates file existence, policy paths, requested backend values, and output directory policy before it starts a child process.  It then starts `aar case` with `--lawyerapi-addr 127.0.0.1:0`, `--councilapi-addr 127.0.0.1:0` when the Council API backend is selected, a generated `--run-id`, and a runner-visible case id flag once the runner supports one.

```http
POST /api/v1/cases
```

```json
{
  "complaint_path": "examples/ex1/complaint.md",
  "case_files": ["examples/ex1/*.txt"],
  "policy_path": "etc/policy.json",
  "out_dir": "out/service/arb-20260602-0001",
  "council_backend": "councilapi",
  "lawyer_timeout_seconds": 900,
  "council_timeout_seconds": 900,
  "case_id": "arb-20260602-0001"
}
```

The response returns the public case record, not private runner internals beyond fields that clients need for diagnostics.  The manager may accept caller-supplied `case_id` values only when they pass a strict identifier rule and do not already exist.  If the caller omits `case_id`, the manager generates one from UTC time and a monotonic sequence or random suffix.

### Case Listing And Inspection

The listing endpoint returns case records with compact status fields.  It should support filters for status and creation time, plus pagination fields so a long-running service does not return every historical case by default.  The inspection endpoint returns the full registry record for one case and may include the parsed case-result summary once the runner finishes.

```http
GET /api/v1/cases
GET /api/v1/cases/{case_id}
GET /api/v1/cases/{case_id}/result
```

`GET /api/v1/cases/{case_id}/result` can call the runner's Lawyer API result endpoint while the case is active, then read `run.json` after completion if the process has exited.  That endpoint should return the same result shape regardless of source.  The response can include `source: "runner_api"` or `source: "artifact"` for diagnostics, but clients should not need to branch on it.

### Cancellation

Cancellation operates on the child process that runs the case.  `POST /api/v1/cases/{case_id}/cancel` marks the case as canceling, closes new mutating requests for that case, sends a graceful termination signal to the child, waits for a short deadline, and then kills the child if it remains active.  The manager records `status: "canceled"` after the child exits because of cancellation.

The runner should eventually accept a structured cancellation path that lets it set terminal state and write a partial packet.  Until then, the manager can preserve the child logs, registry record, and any artifacts already written under `out_dir`.  The manager must report cancellation truthfully: a canceled case has no final arbitration result unless the runner had already completed before cancellation reached it.

## Public Role APIs

The manager exposes public Lawyer and Council API routes with the same shapes as the current per-case APIs.  This lets the unified MCP process and any direct HTTP clients use one stable base URL for all cases.  The manager resolves `case_id`, checks that the target runner exists, and proxies the request to the private API base for that case.

```http
GET  /lawyerapi/v1/get?case_id=...&role_id=...
GET  /lawyerapi/v1/wait?case_id=...&role_id=...
GET  /lawyerapi/v1/status?case_id=...&role_id=...
GET  /lawyerapi/v1/result?case_id=...&role_id=...
POST /lawyerapi/v1/do

GET  /councilapi/v1/get?case_id=...&member_id=...
GET  /councilapi/v1/wait?case_id=...&member_id=...
POST /councilapi/v1/do
```

The proxy must reject unknown case ids before forwarding.  It must also reject a Council API request when the case did not start with a Council API backend.  For `POST /do`, the manager reads enough JSON to extract `case_id`, then forwards the original request body to the matching private API after enforcing request-size limits.

The manager should preserve response status codes and JSON bodies from the runner.  It can add proxy error responses for unknown cases, missing case ids, inactive runners, private API timeouts, and child process failures.  It should not reinterpret runner tool failures, because the runner already knows whether an attempted legal act consumed an invalid attempt.

## Unified MCP Server

The unified MCP server serves Streamable HTTP MCP at `/mcp`, authenticates requests, creates MCP sessions, expires idle sessions, and forwards tool calls to the manager's public role APIs.  It stores no case state and no turn state beyond the assignment binding for each MCP session.

An MCP session binds to one assignment.  A session is a lawyer assignment when initialization includes `case_id` and `role_id`.  A session is a council assignment when initialization includes `case_id` and `member_id`.  The server rejects initialization when both principal fields appear or when neither appears.

```text
/mcp?case_id=arb-20260602-0001&role_id=plaintiff
/mcp?case_id=arb-20260602-0001&role_id=defendant
/mcp?case_id=arb-20260602-0001&role_id=observer
/mcp?case_id=arb-20260602-0001&member_id=C1
```

### MCP Session Model

The session record stores one principal and one assignment type.  Lawyer sessions store `role_id`, which must be `plaintiff`, `defendant`, or `observer`.  Council sessions store `member_id`, which must be a non-empty identifier without whitespace.  Every session also stores `case_id`, creation time, last-seen time, and the manager API bases used for forwarding.

| Field | Lawyer session | Council session |
|---|---|---|
| `case_id` | Required | Required |
| `assignment_type` | `lawyer` | `council` |
| Principal | `role_id` | `member_id` |
| Wait tool | `wait_for_opportunity` | `wait_for_opportunity` |
| Current tool | `get_current_opportunity` | `get_current_opportunity` |
| Forwarded API | Lawyer API | Council API |

The unified server should use assignment-neutral tool names.  Every assignment exposes `wait_for_opportunity` and `get_current_opportunity`; the session's assignment type determines whether those calls forward to the Lawyer API or the Council API.  Provider-specific tools then follow the assigned authority, so a lawyer session can file lawyer acts and a council session can submit a vote.

### MCP Tool Authority

The MCP server must maintain separate tool providers under the shared transport.  A lawyer provider exposes the stable lawyer tool set, forwards read-only status and result calls to the Lawyer API, fetches the active opportunity before mutating calls, injects `opportunity_id` where required, and returns runner failures as MCP tool errors with structured content.  A council provider fetches the current Council API status, exposes the current council tools, injects the active `opportunity_id`, and forwards the call to the Council API.

This split keeps authorization local and visible in code.  A session created with `role_id=plaintiff` cannot call `submit_council_vote` because the lawyer provider never exposes that tool.  A session created with `member_id=C1` cannot call `submit_decision` or submit evidence because the council provider never exposes lawyer tools.

The unified MCP process should support one manager Lawyer API base and one manager Council API base.  It should not accept per-case API maps in service mode, because case routing belongs to the manager.  Both MCP providers point at the manager:

```bash
.bin/aar-mcp \
  --listen 0.0.0.0:19780 \
  --lawyerapi-base http://127.0.0.1:19770/lawyerapi/v1 \
  --councilapi-base http://127.0.0.1:19770/councilapi/v1 \
  --bearer-token "$AAR_MCP_TOKEN"
```

## Case Id Semantics

The public service must treat `case_id` as a routing key and an integrity guard.  The manager routes every public role API call by `case_id` before it reaches a runner.  The MCP server binds each session to one `case_id` at initialization and includes that value in every forwarded request.

The current runner still initializes Lean state with `case_id: "arb-1"`.  That value should become a runner configuration field so the case process knows its public id.  After that change, the Lawyer and Council APIs inside the runner should reject a request whose `case_id` does not match the runner's configured case id.

The manager can route safely before the runner change because each private API base belongs to one runner.  The deeper runner check removes a class of operator mistakes, especially during manual debugging with private ports.  Both layers should exist because the manager protects public routing and the runner protects its own private API.

## Artifact Access

The manager should expose artifact reads through case-scoped endpoints.  It should read only from the registered `out_dir` for that case and should reject path traversal, symlinks that escape the case directory, and hidden process-control files.  Artifact endpoints are read-only and should never trigger engine calls or runner mutations.

```http
GET /api/v1/cases/{case_id}/artifacts
GET /api/v1/cases/{case_id}/artifacts/run.json
GET /api/v1/cases/{case_id}/artifacts/digest.md
GET /api/v1/cases/{case_id}/artifacts/transcript.md
GET /api/v1/cases/{case_id}/evidence/{evidence_id}
```

The artifact listing should return file names, sizes, MIME types, and hashes when available.  Evidence reads should use the evidence manifest rather than trusting caller-supplied paths.  The manager can serve byte ranges for large files, but it should keep those reads separate from the lawyer and council evidence tools because artifact access is an operator capability, not a case-role capability.

## Logging And Observability

The manager should write structured logs without color.  Each log record should include `case_id` when a request or process event belongs to a case.  Process logs should include child pid, exit code, signal, stdout summary parse status, and stderr log path.

Each child runner should write stdout and stderr to case-specific log files under a manager-owned log directory.  The manager should parse the runner's stderr lines that announce private API bases, but it should still persist the raw logs for review.  If startup fails before the runner prints an API base, the manager should mark the case failed and include the startup error in the registry record.

The MCP server should log session creation, session deletion, assignment type, case id, principal id, forwarded tool names, HTTP status, and runner `ok` value.  It must not log bearer tokens, tool payloads, evidence bytes, filings, or private work notes.  The runner already records case events and work notes in the case packet, so the adapter log should remain a transport log.

## Authentication

The service needs separate authentication policy for operator endpoints and assignment endpoints.  Operator endpoints can start and cancel cases, read artifacts, and inspect the registry, so they should require an operator token or an equivalent local deployment control.  Lawyer and council endpoints should require assignment credentials or remain bound to localhost behind the MCP server during local tests.

The first implementation can use bearer tokens because the existing MCP adapters already support them.  A later version can issue per-assignment tokens through the manager so the MCP URL for a plaintiff role carries only plaintiff authority for one case.  The manager should record token metadata without storing raw token values when that issuance path exists.

Authorization checks should follow the assignment boundary.  A plaintiff token can call Lawyer API routes only for `role_id=plaintiff` and its case id.  A council token can call Council API routes only for its `member_id` and case id.  An operator token can use service endpoints and artifact reads but should not file lawyer decisions or council votes unless the operator deliberately creates an assignment client.

## Failure Handling

The manager should treat child process exit as the source of lifecycle truth.  If a runner exits after printing a success summary, the manager records `completed`.  If a runner exits with a JSON error summary or a nonzero exit code, the manager records `failed`.  If the manager sent cancellation and the runner exited before completion, the manager records `canceled`.

Proxy calls should fail with explicit JSON errors when no active route exists.  Unknown case ids return `404`.  Known cases without an active runner return a case status or result endpoint response when possible, but mutating `POST /do` calls fail because no runner can accept the turn.  Private API timeouts return a proxy error and should not be transformed into runner tool failures.

Manager restart recovery should read the persistent registry and reconcile child processes.  If the manager owns process groups, it can record child pids and inspect whether they remain alive after restart.  Any active case whose child process has disappeared without a final summary becomes `failed` unless the output packet shows a completed `run.json`.

## Greenfield Cutover

The service branch should treat `aar service` and `aar-mcp` as the supported entry points for multi-case operation.  The split MCP adapter binaries should disappear after the unified server passes the lawyer and council tests.  Documentation and examples should describe the service topology only, so a new operator sees one HTTP manager and one MCP endpoint.

The `aar case` command should remain as the child runner used by the manager.  It is still useful as a direct development command, but it should accept the same case identity and terminal-state behavior that the manager expects.  The runner should gain a `--case-id` flag, pass that value into `initialState`, and reject Lawyer or Council API requests whose `case_id` differs from the configured value.

## Implementation Plan

The implementation should proceed in phases that keep the runner as the arbitration authority while adding the service boundary around it.  Each phase should end with tests that exercise the new behavior at the narrowest useful boundary.  Full OpenClaw examples come last because they are expensive and diagnose fewer code paths than focused HTTP and MCP tests.

| Phase | Work | Tests |
|---:|---|---|
| 1 | Add `--case-id` to `aar case`, store it in `runner.Config`, use it in `initialState`, write it into run artifacts, and reject mismatched `case_id` values in the Lawyer and Council APIs. | Runner unit tests for initial state, Lawyer API mismatch rejection, Council API mismatch rejection, and normal `arb-1` default behavior when the flag is omitted. |
| 2 | Add terminal role responses inside the runner so `/get`, `/wait`, `/status`, and `/result` return a stable done state after the case closes while the runner process remains alive. | Lawyer API and Council API tests that close a test case and then call every read endpoint; wait should return `done` rather than block or fail. |
| 3 | Add an internal service manager package that starts one child `aar case`, captures private Lawyer and Council API bases, records stdout summary JSON, stores stderr/stdout log paths, and terminates the child on cancellation. | Manager unit tests with a fake child command for startup success, startup failure, stdout summary parsing, stderr API-base parsing, graceful termination, forced termination, and registry updates. |
| 4 | Add `aar service` with `POST /api/v1/cases`, `GET /api/v1/cases`, `GET /api/v1/cases/{case_id}`, `GET /api/v1/cases/{case_id}/result`, and `POST /api/v1/cases/{case_id}/cancel`. | HTTP tests for case creation validation, generated case ids, duplicate case ids, listing filters, inspection, result before and after completion, and cancellation state. |
| 5 | Add public Lawyer and Council API proxy routes on the manager.  `GET` routes should route by query `case_id`; `POST /do` should read a bounded JSON body, extract `case_id`, and forward the original body to the private runner API. | Proxy tests for unknown case ids, inactive cases, missing case ids, request-size rejection, response status preservation, private API timeout, and read-only completed-case responses from artifacts. |
| 6 | Add registry persistence as one JSON file per case under a registry directory, written through atomic rename.  Manager startup should reconcile registry rows with live child processes and completed output packets. | Persistence tests for atomic writes, reload, completed-case recovery from `run.json`, missing-child failure marking, and canceled-case preservation. |
| 7 | Add artifact endpoints scoped to the registered output directory: listing, `run.json`, `digest.md`, `transcript.md`, `work-notes.ndjson`, and evidence reads by `evidence_id`. | Artifact tests for path traversal rejection, symlink escape rejection, evidence manifest lookup, byte ranges, MIME type reporting, and missing-file errors. |
| 8 | Build `aar-mcp` from shared transport code and separate assignment providers.  One MCP session binds to one `case_id` plus either `role_id` or `member_id`; all assignments use `wait_for_opportunity` and `get_current_opportunity`, and provider-specific tools remain separated by assignment type. | MCP tests for session initialization, invalid mixed principals, bearer-token rejection, lawyer tool authority, council tool authority, opportunity-id injection, runner error propagation, and session expiry. |
| 9 | Remove the split MCP adapter commands and update build targets, runbooks, skills, and example scripts to use `aar service` and `aar-mcp`. | Build tests proving only supported commands are generated, documentation checks for stale adapter names, and example-run tests through the service manager. |
| 10 | Run end-to-end examples through the service topology with OpenClaw lawyers and council API members where applicable. | `examples/ex1` through `examples/ex6` should complete or fail with diagnosable service errors; review should confirm evidence submission, evidence reads, work notes, final results, and clean terminal `done` responses. |

## Open Design Decisions

| Decision | Options | Preferred direction |
|---|---|---|
| Case id generation | Caller-supplied, timestamp sequence, random suffix | Accept caller-supplied ids under strict validation, and generate timestamp plus sequence when omitted. |
| Registry persistence | Single JSON file, JSON lines, one file per case | One file per case under a registry directory, written through atomic rename. |
| Runner API discovery | Parse stderr, write a startup JSON file, pass fixed ports | Add a startup metadata file and keep stderr messages for humans. |
| Assignment credentials | One server token, per-case tokens, per-assignment tokens | Start with server token for local service mode, then add per-assignment tokens through the manager. |
| Completed-case role API | Proxy only while active, read artifacts after exit, keep runner alive briefly | Return stable done/status/result from the manager after completion, using runner data while active and artifacts after exit. |

The unresolved design point is whether the manager should ever embed several runners in one process.  The current answer should be no for the first service version.  The child-process model uses the existing runner correctly, isolates failures, and gives the manager a small routing problem instead of a full case-state refactor.
