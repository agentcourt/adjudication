# ARB Clerk API Notes

Notes from exercising the ARB service Clerk API directly at
`http://127.0.0.1:19770`.  The [Agent Arbitration Manual](arb/manual.md)
sections `aar service` and Clerk API are the reference.  The service was
already running with `--out-root /tmp/adjudication-web-arb-20260710T0925Z`.

## Create

`POST /clerk/v1/cases` with `example: ex02`, an explicit `case_id` and
`out_dir` (`arb-api-ex02-20260711T144110Z`), `openclaw_auth: codex`,
`openclaw_codex_auth_path`, and `council_pool_path: ../arb/pool.jsonl`
returned HTTP `202 Accepted` with the full Clerk record:

```json
{"case": {"case_id": "arb-api-ex02-20260711T144110Z",
          "run_id": "run-arb-api-ex02-20260711T144110Z",
          "example": "ex02", "pid": 748250, "status": "running",
          "out_dir": "...", "stdout_log": "...", "stderr_log": "...",
          "created_at": "...", "started_at": "..."}, "ok": true}
```

The record echoes server-side absolute paths for `out_dir` and the two clerk
logs.  `202` fits the semantics: the child `aar run` process starts and the
case completes later.

## Reads During The Run

| Route | Status | Body shape |
| --- | --- | --- |
| `GET /clerk/v1/cases` | 200 | `{ok, cases: [record...]}`.  Five records before the new case; list scans the out-root. |
| `GET /clerk/v1/cases?status=running` | 200 | Filtered list; contained only the new case. |
| `GET /clerk/v1/cases/{id}` | 200 | `{ok, case: record}`; `status: running`, `pid` present. |
| `GET .../result` | 200 | `{ok, status: "running", message: "The case is still pending or has no final result."}` |
| `GET .../artifacts` | 200 | `{ok, case_id, artifacts: [{name, size_bytes}...]}`.  Early in the run: `events.ndjson`, `evidence-manifest.json`, and the two clerk logs. |
| `GET .../artifacts/events.ndjson` | 200 | Live NDJSON; last line readable while the run appends. |
| Same with `Range: bytes=-2048` | 206 | Byte range honored on the live file. |

A pending result returning HTTP `200` with `status: running` means a
monitoring client keys on the JSON `status` field, not the HTTP code.

## Error Paths

| Request | Status | Body |
| --- | --- | --- |
| `GET /clerk/v1/cases/does-not-exist` | 404 | `{ok: false, error: {code: "unknown_case", message: "unknown case_id"}}` |
| `GET .../evidence/nope` (running case) | 404 | error body with evidence code |
| `GET .../attestation/events` (local case) | 404 | `{ok: false, error: {code: "attestation_events_unavailable", message: "case is not an attested execution"}}` |

Error bodies carry stable `error.code` values, matching the manual's
description of service-level errors.

## Create Validation

| Request | Status | Body |
| --- | --- | --- |
| Duplicate `case_id` (nonempty `out_dir`) | 400 | `{error: {code: "start_case_failed", message: "case_id already exists"}}` |
| Body `not json` | 400 | `{error: {code: "bad_json", message: "invalid character ..."}}` |
| Empty object `{}` | 400 | `{error: {code: "start_case_failed", message: "complaint_path is required unless example is set"}}` |
| `{"example": "no-such-example"}` | 202 | Accepted; child started. |

The unknown-example request is a validation gap: the service accepted it,
started a child `aar run`, and the child exited with code 1 and stderr
`error: read complaint: open examples/no-such-example/complaint.md: no such
file or directory`.  The Clerk record then reads `status: failed`,
`exit_code: 1`, `error: "child exited with code 1"`, and the operator has to
open `clerk.stderr` to learn the cause.  Checking `examples/EXAMPLE/complaint.md`
at create time would turn this into an immediate 400.  The failed record
`arb-20260711144313-d5816955` remains under the service out-root as a side
effect of this test.

## Artifact And Evidence Routes Mid-Run

`GET .../artifacts/run.json` before completion returns HTTP `400` with
`error.code: "bad_artifact_path"` and a message that embeds the server
filesystem path from the failed `lstat`.  The code choice reads odd twice
over: the artifact name is valid for a completed case (404 would fit a
not-yet-existing file better than 400), and the message leaks the absolute
output path, though the record already exposes `out_dir`, so nothing new is
revealed on this service.

A traversal attempt `artifacts/..%2Fclerk.json` returns HTTP `404` with
`unknown_case`, so the path segment is rejected by routing before any file
access.  `POST .../kill` on an unknown case returns `404 unknown_case`.

Evidence works mid-run once the manifest exists:
`GET .../evidence/ev_74d969c640f7_market-question` returned HTTP `200` with
the stored text bytes.  Evidence IDs come from
`artifacts/evidence-manifest.json`, whose records carry `evidence_id`,
`title`, `mime_type`, `size_bytes`, `sha256`, `admissibility_status`,
`record_visibility`, and provenance fields.

## Monitoring The Run

Two poll loops against the API tracked the case: one read
`GET .../result` for a terminal `status`, and one read the
`events.ndjson` artifact and reported new `attorney_action`,
`council_vote`, and failure events.  Both worked without gaps; every filing
appeared in the event log within the 30-second poll interval.

Event timeline for `arb-api-ex02-20260711T144110Z` (created 14:41:10Z):

| Time (Z) | Phase | Event |
| --- | --- | --- |
| 14:41:21 | openings | two `council_member_replaced` at startup (pool endpoint errors) |
| 14:45:38 | openings | plaintiff `record_opening_statement` |
| 14:47:42 | openings | defendant `record_opening_statement` |
| 14:52:35 | arguments | plaintiff `submit_argument` |
| 14:56:17 | arguments | defendant `submit_argument` |
| 14:57:42 | rebuttals | plaintiff `submit_rebuttal` |
| 14:58:56 | surrebuttals | defendant `submit_surrebuttal` |
| 15:00:04 | closings | plaintiff `deliver_closing_statement` |
| 15:01:04 | closings | defendant `deliver_closing_statement` |
| 15:01:30 | deliberation | C1 votes `demonstrated` |
| 15:02:56 | deliberation | C2 votes `not_demonstrated` |
| 15:02:58 | deliberation | C3 removed (`agent_exited`, OpenRouter tool-use 404) |
| 15:03:08 | deliberation | C4 votes `not_demonstrated` |
| 15:03:17 | deliberation | C5 votes `not_demonstrated`; case closes |

The run took 22 minutes and closed `not_demonstrated`, three votes to one
with one member removed.  The council rejected the proposition that
pre-program remarks count as occurring during the FIFA draw.

Event payload shapes differ by type.  An `attorney_action` payload holds
`action_type` and the filing under `payload`.  A `council_vote` event nests
the vote one level deeper: `.payload.payload.vote` and
`.payload.payload.rationale`, beside `.payload.member_id`, `.payload.model`,
and `.payload.backend`.  A `council_member_removed` payload has `member_id`,
`status`, `failure_reason`, and `cause`.  A consumer has to learn these
shapes from samples; the manuals document the routes and file names but not
the per-type event payload fields.

## Completion Reads

`GET .../result` after completion returns `status: "done"` with
`case_status`, `final_reason`, and the full `result` object, including
`council_votes` with rationales and `failure: null`.  The Clerk record
gains `exit_code: 0`, `finished_at`, and a `summary` mirroring `run.json`
top-level keys.  The artifact list grew to include `run.json`, `digest.md`,
and `transcript.md`, all readable through the artifact route.  The digest
lists the resolution, the proposition, the evidence standard, and the final
council roster with models.

## Candidates

The ARB Clerk API exercise exposed small service API fixes that also apply to
ADC and AARD.  The direction is to fix observed behavior without adding a new
schema layer, process manager, or client abstraction.  ARB and AARD share
example-created Clerk cases, similar service records, and similar artifact,
evidence, result, kill, and attestation-event routes.  ADC creates cases from
complaint and scenario paths, but its service API exposes the same route
families.

| Candidate | Systems | Direction |
| --- | --- | --- |
| Check named examples before child start | ARB, AARD | Validate that `examples/EXAMPLE/complaint.md` exists before reserving a case id or starting a child process.  Return HTTP `400` with a stable error code when the example is missing. |
| Clarify missing artifact semantics | ARB, ADC, AARD | Keep one distinction: unknown artifact name versus listed artifact that has no file yet.  Return stable error codes and omit server filesystem paths from those errors. |
| Document result polling rules | ARB, ADC, AARD | State that a pending result returns HTTP `200` with a nonterminal JSON status.  Document the terminal and nonterminal status values in the manuals. |
| Document byte-range behavior | ARB, ADC, AARD | State that artifact reads support HTTP `Range` when the route serves a file.  Test the shared behavior where practical, without a separate test matrix for identical routing code. |
| Describe detached process handles | ARB, ADC, AARD | Document that case listing reads records from disk, while kill/cancel requires an attached child process.  After a service restart, a record can exist without a live process handle. |
| Defer event payload schemas | ARB, ADC, AARD | Start with the stable event envelope and the event types consumed by the web console.  Do not create a broad schema project until a client needs it. |
