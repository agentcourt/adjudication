# AAR Case-Failure Testing

## Scope

This plan tests the behavior specified in [AAR Process And HTTP Specification](aar-spec.md).  The tests start `aar case` or `aar service`, interact through HTTP, observe stdout and stderr, and check process exit status.  The test code should treat the engine and runner internals as opaque.

The main facts under test are the distinction between participant failure and process failure, the HTTP reporting of active and terminal cases, and the preservation of failure facts in artifacts.  Lawyer failure should fail the case and still let `aar case` exit `0`.  Council-member failure should fail that member, preserve the failure in state and events, and let the case continue when the council rules allow it.

## Test Setups

Use two test setups.  A direct-case test starts `.bin/aar case`, reads stderr until it finds `caseapi listening on ...`, then appends `/lawyerapi/v1` or `/councilapi/v1` for private role API calls while the child process is active.  A service-backed test starts `.bin/aar service`, creates a case with `POST /api/v1/cases`, then calls the public service routes and proxied role API routes.

Both setups should write process stdout, stderr, request JSON, response JSON, and output directory paths into a temporary test directory.  Each test should use a unique case id, run id, output directory, and service registry directory.  A failed test should retain that directory and print its path, so the external interaction can be inspected without rerunning the case.

The fixtures should stay small.  Use one simple complaint, a small policy with short timeouts and a one-attempt budget for failure tests, and a small council pool used only to populate council seats for `councilapi` cases.  The council API tests do not need live model calls because the test client acts as each council member through HTTP.

## Test Matrix

| ID | Setup | Case | Expected Result |
| --- | --- | --- | --- |
| LF-1 | `aar case` | Lawyer exhausts attempts | Child exits `0`; stdout summary has `status: "failed"` and a matching failure object. |
| LF-2 | `aar service` | Lawyer exhausts attempts | Service case becomes `failed`; service result and role reads report the same failure. |
| LF-3 | `aar case` plus `aar service` | Lawyer deadline expires | Direct child exits `0`, and stdout and `run.json` report `deadline_expired`; service-managed completed role reads report the same failure. |
| CF-1 | `aar service` | Council member exhausts attempts | Member reports `status: "failed"`; the case continues or closes under council rules; child does not fail as a process. |
| CF-2 | `aar service` | Council member deadline expires | Member reports `status: "failed"` with `deadline_expired`; other members can still act if the case remains active. |
| RF-1 | `aar case` | Invalid startup input | Child exits nonzero and writes a stdout summary with `status: "error"`; no role API or case artifacts are expected. |

## Lawyer Failure Tests

LF-1 starts `aar case` with a one-attempt budget, a normal lawyer timeout, and a deterministic complaint.  The test waits for the plaintiff turn through `GET /lawyerapi/v1/wait`, reads `turn.opportunity_id`, and submits one invalid mutating call, such as `submit_decision` with the active opportunity id and an opening-statement payload missing required text.  The expected HTTP response is `200` with `ok: false`, and the subsequent terminal responses should report `status: "failed"`.

After LF-1 triggers the failure, the test waits for the child process to exit.  The child must exit `0`.  The last stdout JSON object must contain `status: "failed"`, `error`, `failure.role: "plaintiff"`, and `failure.reason: "attempts_exhausted"`.  `run.json` must contain a final state whose case status is failed.

LF-2 repeats the same failure through `aar service`.  The test creates the case through `POST /api/v1/cases`, uses the service `/lawyerapi/v1/wait` and `/lawyerapi/v1/do` routes, and then reads `/api/v1/cases/{case_id}/result`.  The service case record should become `failed`, the service result should report `status: "failed"`, and completed role reads through the service should return the same failure object.

LF-3 covers deadline expiry with the direct-case test setup.  Use a short lawyer timeout, wait for an active plaintiff turn, and let the deadline pass without submitting a valid decision.  The direct test should wait for child exit `0`, then inspect the stdout summary, `run.json`, and events for `failure.reason: "deadline_expired"`.  A service-managed deadline test should assert the same failure through `/api/v1/cases/{case_id}/result` and completed role reads, because the private `aar case` role API exits with the child process.

## Council-Member Failure Tests

CF-1 starts a service-managed case with `council_backend: "councilapi"` and a one-attempt budget.  The test client advances the lawyer phases through the Lawyer API with minimal valid filings until the case reaches deliberation.  It then waits for the active council member through the Council API and submits one invalid `submit_council_vote`, such as missing `vote` or `rationale`.

After the invalid vote, the failed council member should receive `status: "failed"`, no tools, and a failure object with role `council`, the member id, the active opportunity id, and reason `attempts_exhausted`.  Observer status should show the failed council member in the roster or current case view.  The event log should contain `opportunity_failed` and `council_member_removed` with `status: "failed"`.

The case should then continue if the council rules allow it.  The test should use the Council API to submit valid votes for remaining members until the case reaches a terminal result, or it should accept a rule-governed terminal result such as no majority if that follows from the configured council policy.  The child process should exit `0`, and the stdout summary should report the final arbitration result rather than a process error caused by the failed member.

CF-2 uses the same service-managed setup but lets one active council member deadline expire.  The failed member should report `status: "failed"` and `failure.reason: "deadline_expired"`.  Other council members should still be able to wait and act when the case remains active under the council rules.

## Runtime-Failure Test

RF-1 starts `aar case` with invalid startup input, such as a missing complaint path.  The process should exit nonzero and write a compact stdout summary with `status: "error"`.  The test should not expect a role API URL, `run.json`, or participant failure object.

This test protects the boundary between process failure and participant failure.  Lawyer missed deadlines, malformed lawyer calls, council missed deadlines, and malformed council calls should use HTTP responses and terminal case artifacts.  Broken startup configuration should use the process exit code.

## Artifacts And Observability

Every participant-failure test should inspect the output directory after the case exits.  `run.json` should contain the same status and failure facts reported through HTTP and stdout.  `events.ndjson` should contain the failed opportunity event; council-member failure should also include the failed member event.

The service tests should also inspect the service record and child logs.  The case record should contain the child exit code, parsed stdout summary, stdout log path, stderr log path, and final service status.  The service result endpoint should agree with the stored child summary and `run.json`.

## Minimum Passing Set

The minimum passing set includes LF-1, LF-2, CF-1, and RF-1.  Those tests cover the main distinction: lawyer failure fails the case, council-member failure fails the member, service reporting preserves the distinction, and runtime faults still use nonzero process exit.  LF-3 and CF-2 cover deadline behavior and should remain bounded by observed active turns and returned deadlines.

The test code should prefer invalid-attempt failures when a failure mode does not require timing, because those cases are deterministic and fast.  Deadline tests should use short but realistic limits and should assert failure only after observing the active turn and waiting past its returned deadline.  Network access and live agents are outside this plan.
