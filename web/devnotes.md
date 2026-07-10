# Development Notes

## 2026-07-10

### Service API web console

Reference: `cmd/adjudication-web/main.go`, `console/server.go`, `console/client.go`, `README.md`

The web console is a separate Go HTTP server.  It uses only configured ADC, ARB, and AARD service base URLs for case creation, listing, management, result reads, artifact reads, evidence reads, attestation events, and raw service requests.  The implementation deliberately keeps case state and artifact bytes behind the service APIs, so the web process does not read output directories or import the adjudication runtime packages.

The create form accepts raw JSON and posts it unchanged to the selected service create route.  This avoids creating a second request schema in the web layer and keeps support for service-specific fields such as local execution settings and attested execution settings.  Browser file upload is not part of this first web console because the backing service APIs accept server-visible paths rather than uploaded file content.

During review, the evidence page failed to show raw, non-JSON evidence returned by the service API.  The response panels now render parsed JSON when the response contains JSON and otherwise render the response body as text.  The added test covers a text evidence response through the web evidence page.

The console now renders structured `stdout_log` and `stderr_log` record values as links when the path names one of the service's known child log artifacts.  The service APIs expose only those exact log artifact names: Clerk `clerk.stdout` and `clerk.stderr`, ARB direct `service-logs/aar.stdout` and `service-logs/aar.stderr`, AARD direct `service-logs/aard.stdout` and `service-logs/aard.stderr`, and ADC `service-logs/adc.stdout` and `service-logs/adc.stderr`.  The web process still reads logs through service artifact routes rather than reading output directories.

The case detail page now summarizes structured record values such as AARD `summary` fields instead of formatting Go maps into table cells.  Response panels omit oversized formatted bodies and report their size, so large service responses do not dominate the page while the service API continues to provide the full response through the request console.  Running case and result pages refresh every ten seconds with an HTML meta refresh, keeping the implementation server-rendered and avoiding client-side state.

The evidence page now reads `evidence-manifest.json` through the service artifact API and renders evidence IDs as fetch links.  The page still accepts a manual evidence ID because active cases can return a pending or missing manifest.  The ARB clerk create template now uses `../arb/pool.jsonl`, matching the service's common-root-relative pool lookup for real ARB clerk runs.

The case detail page now reads `events.ndjson` through the service artifact API when that artifact exists and renders a failure-event table.  The table reports timestamp, phase, event type, member, process, reason, message, and the log path carried by the event.  Duplicate removal events are collapsed when they repeat the same member and message.  The web console does not read the log path directly; the service artifact API remains the only source of case data.

The manage form appears only while a case is active.  Completed cases still show result, artifact, evidence, and attestation links, but they no longer present a stale kill or cancel action.

The default console exposes ADC only through Clerk.  The ADC service still serves `/api/v1/cases` for API clients that call the service directly.  The web UI case navigation now presents the operational choices: ADC Clerk, ARB Clerk, ARB Direct, AARD Clerk, and AARD Direct.

Structured case-record fields now render as fact lists plus a closed JSON disclosure.  This keeps `summary` cells useful on the case page: scalar state, answers, vote tallies, and collection counts appear immediately, while the full service JSON remains available without leaving the page.

The case detail page now renders a Recent Events table from `events.ndjson`, newest first and capped at eight rows.  Active cases then show visible phase progress without forcing the operator to open the raw NDJSON artifact.  The existing Failure Events table still reports process and provider failures separately.

### Verification

- [x] `go test ./web/...`
- [x] `go test ./adc/runtime/... ./arb/runtime/... ./arbd/runtime/... ./common/... ./web/...`
- [x] Manual curl test against `go run ./web/cmd/adjudication-web`: `/` returned HTML, `/health` returned HTTP `204`, and a disabled service target returned HTTP `502`.
- [x] Manual curl test against a real ARB clerk service and the web console.  Submitted `web-live-ex01-20260710T1312Z` from `arb/examples/ex01` through the web case-create route, listed cases, monitored the record and result, fetched `evidence-manifest.json`, `events.ndjson`, `digest.md`, `run.json`, and fetched evidence through both the rendered evidence page and the raw evidence route.  The ARB case closed at `2026-07-10T13:32:20Z` with final resolution `demonstrated` and vote tally `demonstrated: 5`, `not_demonstrated: 1`.
- [ ] `go test ./...`: package enumeration fails before testing because `arb/out/juror-model-experiments/generic-5model-tolerant-20260706T145403Z/resume-repair/20260706T202709Z/dirs/runs/ex08a/run-02/deepseek-r1/r01/pi-C1/pi-mcp-output-D1UmsM` is unreadable.
