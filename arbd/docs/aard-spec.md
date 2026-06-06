# AARD Process And HTTP Specification

## Scope

This specification defines the external process and HTTP behavior for AARD tests and clients.  It covers `aard case`, the direct case service under `/api/v1`, the Clerk service under `/clerk/v1`, the Lawyer API, and the Council API.  Internal Lean calls, Go package APIs, and agent behavior sit outside this specification.

## Process Model

`aard case` runs one degree-arbitration case from startup to a terminal state.  It accepts command-line flags for the complaint, case id, run id, output directory, private case API address, policy, evidence files, council pool, prompt files, limits, and council backend.  During startup it creates one private case API listener that serves `/health`, `/lawyerapi/v1/...`, and, in Council API mode, `/councilapi/v1/...`.

When the private case API starts, `aard case` writes one diagnostic line to stderr with the exact prefix `caseapi listening on `.  The suffix is the private base URL without a role path.  Direct process tests may read this line to call the private role APIs, while `aard service` supplies the child address before startup and waits on `/health`.

`aard case` writes one JSON summary line to stdout when the case ends.  A normal degree-arbitration result uses `status: "ok"` and includes the answer map keyed by council member id.  A procedural lawyer failure uses `status: "failed"` and exits `0` after the runtime records the failure and writes final artifacts.

Process-level faults use nonzero exit status and a stdout summary with `status: "error"` when the command can produce one.  Examples include invalid flags, unreadable complaint files, failure to start a required HTTP listener, engine execution failure, storage failure, and invalid internal state.  Participant failures are case facts reported through case state, HTTP responses, stdout summaries, events, and artifacts.

## Service APIs

`aard service` starts a long-lived HTTP service.  The direct case API under `/api/v1/cases` starts `aard case` child processes and can proxy role API calls by `case_id`.  The Clerk API under `/clerk/v1/cases` starts full `aard run` child processes with local OpenClaw lawyers, MCP, and Pi council agents.

The direct create request includes `complaint_path` and may include `case_id`, `run_id`, `case_files`, `policy_path`, `out_dir`, `council_backend`, timeouts, attempt limits, response limits, common root, engine path, council pool path, prompt paths, and attorney instruction paths.  The service assigns the child private case API address, starts the child, polls `GET /health`, and records process and output metadata.

The Clerk create request mirrors `aard run` options in structured JSON.  It accepts fields such as `example`, `complaint_path`, `case_files`, `out_dir`, `policy_path`, `council_size`, `judgment_standard`, `council_pool_path`, `openclaw_auth`, `openclaw_codex_auth_path`, `auto_lawyers`, `mcp_public_base_url`, `pi_image`, `pi_mcp_adapter`, and council output limits.  A Clerk-started run stores `clerk.json` in the run output directory.

Both service groups support create, list, inspect, kill or cancel, result, artifact listing, artifact read, and submitted-evidence read endpoints.  Artifact routes serve exact listed artifact names.  Evidence routes use `evidence-manifest.json` and serve accepted submitted evidence by `evidence_id`.

## Common HTTP Rules

JSON responses include `ok` unless the endpoint serves a file.  HTTP status describes request handling; the JSON body describes the AARD operation.  Request-level failures return ordinary HTTP status codes and an `error` object with a machine-readable `code` and a human-readable `message`.

Procedural tool rejection can return HTTP `200` with `ok: false`.  These rejections include stale opportunity ids, wrong active role, missing opportunity id, invalid tool arguments, exhausted attempts, and turn timeout.  Errors that prevent the service from reaching the child process use service-level error codes and HTTP status codes such as `404`, `409`, or `502`.

If `aard service` starts with a bearer token, every service request must include `Authorization: Bearer TOKEN`.  Private role APIs started by `aard case` do not enforce the service bearer token.  MCP may have a separate bearer token, documented in [AARD MCP Specification](aard-mcp-spec.md).

## Lawyer HTTP API

The Lawyer API is available at `/lawyerapi/v1` on a private `aard case` listener and through the service proxy for direct `/api/v1/cases` records.  Lawyer roles are `plaintiff` and `defendant`; `observer` is read-only.  Every request includes `case_id`, and lawyer requests include `role_id`.

`GET /lawyerapi/v1/get` returns role status, prompt, tools, limits, and current turn state.  `GET /lawyerapi/v1/wait` waits for a state change or timeout and returns the same response shape plus a wait object.  `GET /lawyerapi/v1/status` returns compact case status, and `GET /lawyerapi/v1/result` returns the final answer map or pending status.

`POST /lawyerapi/v1/do` executes one tool call.  Mutating lawyer calls include `case_id`, `role_id`, `tool`, `arguments`, and the active `opportunity_id`.  The `opportunity_id` must match the `turn.opportunity_id` returned by the current ready response.

Ready lawyer turns return `status: "ready"`, a prompt, available tools, limits, remaining time, attempts left, and a turn object.  Waiting roles return `status: "waiting"`.  Terminal successful cases return `status: "done"`, and terminal failed cases return `status: "failed"` with the failure object when available.

Lawyer tools include case status, case read, work notes, evidence listing, evidence metadata, bounded evidence reads, direct evidence submission, chunked evidence upload, and `submit_decision`.  Evidence-reading tools are available in every lawyer phase.  Evidence-submission tools are available in arguments, rebuttals, and surrebuttals.

## Council HTTP API

The Council API is available when the case starts with `--council-backend councilapi` or the equivalent service field.  Calls go to `/councilapi/v1` and include `case_id` and `member_id`.  A council member receives one deliberation opportunity, reads the admitted record, and submits one integer answer with a rationale.

`GET /councilapi/v1/get` returns council-member status, prompt, tools, limits, and turn state.  `GET /councilapi/v1/wait` waits for a state change or timeout.  `POST /councilapi/v1/do` executes a council tool call, and `POST /councilapi/v1/fail` records a council-member failure for the active opportunity.

Council tools include `get_case`, `list_evidence`, `stat_evidence`, `read_evidence_range`, and `submit_council_answer`.  A council answer payload includes an integer answer from `0` through `100` and a rationale.  The runtime injects the trusted member id; a caller-provided member id inside arguments has no authority.

Council-member failure dismisses that member and lets the case continue with the remaining seated members.  Failure reasons include deadline expiration, exhausted attempts, request failure, agent exit, and output-limit termination.  The failed member's API returns `status: "failed"` and a structured failure object.

## Results And Failures

A successful result has `status: "done"` in role result responses and `status: "ok"` in the `aard case` stdout summary.  The summary includes `answers`, `run_id`, and `out_dir`.  `run.json` contains the final state, answer map, council roster, council answer records, admitted evidence, final reason, and generated artifacts.

A lawyer failure fails the case.  The external reports use `status: "failed"` and include an error message plus a structured `failure` object.  The failure object identifies the role, phase, opportunity id, reason, and message.

A council-member failure records a failed member.  The final result can still be `status: "ok"` if remaining members complete the council phase.  Final state, `council.json`, and `events.ndjson` preserve the failed member record.
