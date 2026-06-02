# AAR Process And HTTP Specification

## Scope

This specification defines the external behavior used to test AAR.  Tests start AAR processes, observe their exit status and standard streams, and call HTTP endpoints.  Tests do not call Lean directly, import Go packages, inspect in-memory state, or rely on agent behavior.

The specification covers two entry points.  `aar case` runs one arbitration case and exposes role HTTP APIs during the run.  `aar service` runs a multi-case HTTP service, starts `aar case` child processes, and proxies role API calls to the active child.

## Process Model

`aar case` runs one case from start to terminal state.  It accepts command-line flags that identify the complaint, case id, run id, output directory, lawyer API address, council backend, and optional council API address.  During startup it writes the private role API base URL lines to standard error.

When the lawyer API starts, `aar case` writes a line with this exact prefix: `lawyerapi listening on `.  The suffix is the base URL for the private Lawyer API, including `/lawyerapi/v1`.  When the case starts with `--council-backend councilapi`, `aar case` also writes a line with this exact prefix: `councilapi listening on `, followed by the base URL for the private Council API, including `/councilapi/v1`.

`aar case` writes a JSON summary line to standard output when the case reaches a terminal state.  A normal arbitration result uses `status: "ok"`.  A procedural case failure, such as a lawyer missed deadline or exhausted attempt budget, uses `status: "failed"` and still exits `0`.

A nonzero `aar case` exit status reports process or runtime failure.  Examples include invalid command-line input, unreadable complaint files, failure to start a required HTTP listener, engine execution failure, storage failure, or invalid internal state.  Participant failure should not produce a nonzero process exit.

`aar service` runs until interrupted or until its parent process stops it.  It writes `aar service listening on http://{addr}` to standard error after successful startup.  It requires a registry directory, an output root, and the path to the `aar` binary used for child cases.

`aar service` starts one `aar case` child for each created case.  The service records the child process id, private role API base URLs, stdout and stderr log paths, child exit code, parsed stdout summary, and service-level status.  The service status reflects the child result: active children are `starting` or `running`, normal terminal results are `completed`, procedural case failures are `failed`, and canceled cases are `canceled`.

## Common HTTP Rules

All service and role API responses are JSON unless the endpoint serves an artifact file.  JSON responses use an `ok` field.  HTTP status describes request handling, while `ok` describes the AAR operation in the body.

All error responses include an `error` object.  That object has a machine-readable `code` and a human-readable `message`.  Request-level errors use ordinary HTTP status codes such as `400`, `401`, `404`, `405`, `410`, `413`, or `502`.

Procedural tool rejection usually returns HTTP `200` with `ok: false`.  The server understood the request and rejected the attempted role action.  These errors include stale opportunity ids, wrong active role, missing opportunity id, invalid tool arguments, exhausted attempts, and turn timeout.

If `aar service` starts with a bearer token, every HTTP request to the service must include `Authorization: Bearer {token}`.  Missing or wrong tokens return HTTP `401` with `ok: false`.  Private role APIs started by `aar case` do not enforce the service bearer token.

## Service Case Endpoints

`POST /api/v1/cases` starts a new case.  The JSON request includes `complaint_path` and may include `case_id`, `run_id`, `case_files`, `policy_path`, `out_dir`, `council_backend`, timeouts, attempt limits, response limits, common root, engine path, council pool path, prompt paths, and attorney instruction paths.  On success, the service returns HTTP `202` with `ok: true` and a `case` record.

The returned case record contains `case_id`, `run_id`, `status`, `complaint_path`, `out_dir`, `council_backend`, timestamps, and service log paths.  While the child starts, `status` is `starting`.  After the child prints the required role API base URLs, the service marks the case `running`.

`GET /api/v1/cases` lists known cases.  It accepts an optional `status` query parameter.  The response body has `ok: true` and a `cases` array sorted by creation time.

`GET /api/v1/cases/{case_id}` returns one public case record.  Unknown case ids return HTTP `404` with `code: "unknown_case"`.  The returned record reflects the latest registry state known to the service.

`GET /api/v1/cases/{case_id}/result` returns the current or final result.  For an active case with a live Lawyer API, the service proxies to the child observer result endpoint.  For a completed child, the service reads `run.json` from the case output directory and returns the final result shape.

If a child has no final artifact yet, the result endpoint returns `ok: true`, the service status, and a message that the case is pending or has no final result.  If the child exited `0` with `status: "failed"` in its stdout summary, the service result endpoint returns `status: "failed"` and the failure message.  If `run.json` exists, the response also includes the structured `failure` object.

`POST /api/v1/cases/{case_id}/cancel` cancels an active case.  The service marks the record `canceling`, signals the child, kills it if needed, and later records `canceled` after the child exits.  The endpoint returns `ok: true` with the current public case record.

`GET /api/v1/cases/{case_id}/artifacts` lists files under the case output directory.  `GET /api/v1/cases/{case_id}/artifacts/{name}` serves one artifact file if the normalized path stays inside the output directory.  Artifact path errors return JSON; successful file reads use normal file serving.

`GET /api/v1/cases/{case_id}/evidence/{evidence_id}` serves a submitted evidence file from the final evidence manifest.  Unknown cases, missing manifests, bad manifests, missing paths, and unknown evidence ids return JSON errors.  Successful reads serve the stored file bytes.

## Lawyer HTTP API

The Lawyer API is available at `/lawyerapi/v1` on either the private `aar case` listener or the public service proxy.  The lawyer roles are `plaintiff` and `defendant`.  The `observer` role is read-only.

`GET /lawyerapi/v1/get?case_id={case_id}&role_id={role_id}` returns role status, prompt, tools, limits, and turn state.  `GET /lawyerapi/v1/wait?case_id={case_id}&role_id={role_id}` waits for a state change or timeout and returns the same response shape plus a `wait` object.  `GET /lawyerapi/v1/status?case_id={case_id}&role_id={role_id}` returns compact case status.  `GET /lawyerapi/v1/result?case_id={case_id}&role_id={role_id}` returns the final result or pending status.

`POST /lawyerapi/v1/do` executes one tool call.  The JSON body must include `case_id`, `role_id`, `tool`, and `arguments`.  Mutating lawyer calls must also include the active `opportunity_id`; the value must match `turn.opportunity_id` from the current ready response.

A ready lawyer turn returns `status: "ready"`, a non-empty `prompt`, a non-null `turn`, `limits`, and the tools available to that role.  A waiting lawyer returns `status: "waiting"` and no mutating tools.  A terminal successful case returns `status: "done"`, and a terminal failed case returns `status: "failed"`.

The active `turn` object contains the current role, phase, opportunity id, turn number, deadline, live remaining milliseconds, maximum attempts, remaining attempts, and completion flag.  Every ready-turn response must include the current attempt count and deadline state.  Tests should treat `turn.opportunity_id` as the only valid id for mutating calls.

Invalid mutating lawyer calls that count against the attempt budget reduce `attempts_remaining`.  Before exhaustion, the response has HTTP `200`, `ok: false`, an error object, and updated turn state.  When attempts reach zero, AAR records a procedural opportunity failure and the case becomes `status: "failed"`.

If a lawyer turn deadline expires, AAR records a procedural opportunity failure.  The case becomes `status: "failed"`, and all lawyer roles plus observer reads report the same failure.  `aar case` exits `0` after writing a stdout summary with `status: "failed"`.

The observer can call read-only tools through `POST /do`, including `case_status`.  The observer must not mutate case state.  The observer result endpoint is the service's source for active-case result reads.

## Council HTTP API

The Council API is available only when a case starts with `council_backend: "councilapi"` or `--council-backend councilapi`.  The API is available at `/councilapi/v1` on either the private `aar case` listener or the public service proxy.  Calls require `case_id` and `member_id`.

`GET /councilapi/v1/get?case_id={case_id}&member_id={member_id}` returns council-member status, prompt, tools, limits, and turn state.  `GET /councilapi/v1/wait?case_id={case_id}&member_id={member_id}` waits for a state change or timeout and returns the same response shape plus a `wait` object.  `POST /councilapi/v1/do` executes one council-member tool call.

`POST /councilapi/v1/do` must include `case_id`, `member_id`, `opportunity_id`, `tool`, and `arguments`.  The `opportunity_id` must match the active council opportunity for that member.  The server supplies the trusted member id to vote submission; a caller-provided member id inside `arguments` has no authority.

A ready council turn returns `status: "ready"`, a non-empty `prompt`, a non-null `turn`, `limits`, and tools.  A waiting member returns `status: "waiting"` and no tools.  A member who failed an opportunity returns `status: "failed"`, no tools, and a structured failure object.

The active council `turn` object contains `role_id: "council"`, member id, phase, opportunity id, turn number, deliberation round, deadline, live remaining milliseconds, maximum attempts, remaining attempts, and completion flag.  Tests should use the returned `turn.opportunity_id` for mutating calls.  Tests should not infer the active member from roster order alone.

Invalid mutating council calls that count against the attempt budget reduce `attempts_remaining`.  Before exhaustion, the response has HTTP `200`, `ok: false`, an error object, and updated turn state.  When attempts reach zero, AAR records a procedural opportunity failure for that member.

Council-member failure does not by itself fail the case.  AAR marks that member `status: "failed"` with failure reason, opportunity id, and message.  The case continues if the council rules allow another member or round to proceed; if the rules produce a terminal result, the final result reflects those rules.

If a council member misses a deadline, AAR records the same kind of member failure.  The failed member's API reports `status: "failed"` and no tools.  Other members can still wait and act if the case remains active.

## Terminal Results

A successful terminal case result has `status: "done"` in role result responses and `status: "ok"` in the `aar case` stdout summary.  It includes the final phase, case status, resolution, council votes, vote tally, final reason, run id, and output directory.  The output directory contains `run.json` with the final state and generated artifacts.

A procedural lawyer failure has `status: "failed"` in role responses, service result responses, and the `aar case` stdout summary.  The summary includes an `error` string and a structured `failure` object.  The final state contains `case.status: "failed"`.

A council-member failure appears inside case state and events.  If the case later closes with a normal arbitration result, role result responses and the `aar case` stdout summary report the final arbitration result, while final state and events preserve the failed member.  If existing council rules produce a terminal non-merits result, the final result reports that rule-governed outcome rather than a process error.

## Failure Object

The structured failure object for participant failure has type `opportunity_failed`.  It identifies the role, phase, opportunity id, reason, and message.  Council-member failure also identifies the member id and may include model or invalid-reason details.

Lawyer failure reasons include `deadline_expired` and `attempts_exhausted`.  Council-member failure reasons include `deadline_expired`, `attempts_exhausted`, and request-failure reasons produced by the council backend.  The `message` field should name the role or member, opportunity, and reason in plain text.

The same failure fact should be visible through every applicable external surface.  For a lawyer failure, `aar case` stdout, `run.json`, service result, lawyer role status, observer status, and lawyer result should agree.  For a council-member failure, the failed member API, observer status, final state, and event log should agree.

## Test Obligations

Tests for this specification should start real `aar` processes and communicate over HTTP.  A direct `aar case` test should read standard error until it finds the role API base URL lines, then use those URLs for role API calls.  A service test should start `aar service`, call `POST /api/v1/cases`, and use the service's public endpoints.

Tests should use short deadlines and small attempt budgets.  Attempt-exhaustion tests should prefer invalid tool calls over sleeps because they produce faster and more deterministic failures.  Deadline tests should use bounded waits and assert the terminal status after the deadline has actually expired.

Tests should assert exact external facts.  For a lawyer failure, assert process exit `0`, stdout summary `status: "failed"`, service result `status: "failed"`, role responses `status: "failed"`, and a matching structured failure object.  For a council-member failure, assert failed member status, continued or rule-governed case progression, event presence, and absence of process failure.

Tests should reserve nonzero process exit assertions for runtime faults.  A participant missing a deadline, submitting malformed calls, exhausting attempts, or failing a council vote request is part of case state.  Those facts belong in HTTP responses, stdout summaries, events, and artifacts.
