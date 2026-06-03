# Development Notes

## 2026-06-02

### OpenClaw lawyer authentication

Reference: [OpenClaw OAuth-Derived Codex Auth](openclaw-oauth.md)

`aar run` now supports both OpenClaw lawyer authentication paths.  Automatic mode prefers a readable Codex `auth.json`, stages one copied Codex home per lawyer container, mounts it as `/aar-codex`, and sets `CODEX_HOME=/aar-codex`.  If no readable Codex auth file exists, automatic mode uses `OPENAI_API_KEY`.  Explicit `codex` and `api-key` modes force either path.

The staged Codex homes are deleted when the run exits because `auth.json` contains bearer and refresh credentials.  The implementation does not mount the operator's whole `~/.codex` directory into OpenClaw containers.  The API-key path still passes only the `OPENAI_API_KEY` environment variable into the OpenClaw container.

`aar run` now patches the in-container OpenClaw config before starting each lawyer.  The patch sets `plugins.entries.codex.config.appServer.turnCompletionIdleTimeoutMs` and `postToolRawAssistantCompletionIdleTimeoutMs` to the effective AAR lawyer turn timeout.  The ex1 OpenClaw/Pi run on 2026-06-03 failed before this change because the embedded Codex app server abandoned a provider turn after about 120 seconds during plaintiff opening.  The rerun passed that point, completed all lawyer filings, and closed the case.

### AAR opportunity failure

Reference: [AAR Case Failures](../case-failures.md)

AAR now treats participant failure as case state.  The Lean engine exposes one procedural action, `fail_opportunity`, which validates the active opportunity before changing state.  Plaintiff or defendant failure sets `case.status` to `failed` and records a typed failure object.  Council-member failure sets that member's status to `failed`, records the reason fields on the member, and lets deliberation continue under the existing council rules.

The Go role APIs detect deadlines and invalid-attempt exhaustion, then call `fail_opportunity`.  Lawyer failure now produces a terminal `Result` with `status: "failed"` and process exit `0`; service/runtime faults still use process errors.  The Lawyer API, Council API, service result endpoint, and MCP `wait_for_opportunity` now report failed case or failed-member states directly.

Verification status: `make build` passes, and focused Go tests for runner, service, CLI, and MCP pass.  `lake build Proofs` still fails in `Proofs.StepPreservation` on existing surrebuttal evidence proof obligations: the proof expects old text-only surrebuttal behavior, while the executable now allows surrebuttal evidence.  That proof repair is separate from the `fail_opportunity` runtime path.

The process and HTTP black-box tests now cover the external AAR failure boundary.  They start `aar case` and `aar service`, drive lawyer and council roles over HTTP, and assert process exit status, stdout summaries, service case records, result endpoints, `run.json`, and event logs for attempt exhaustion and deadline expiration.  The service startup path now binds its listener before printing the readiness line, and the service waits for stdout capture to finish before classifying a child process from its final JSON summary.

The failure specification now distinguishes direct `aar case` terminal artifacts from completed service-backed role reads.  The black-box tests retain per-test process logs and HTTP exchange logs on failure, and service-managed cases assert child exit code, parsed stdout summary, stdout log path, stderr log path, and final service status.

### AAR MCP specification

Reference: [AAR MCP Specification](../aar-mcp-spec.md), [AAR MCP Test Plan](../aar-mcp-test.md)

The MCP behavior now has separate root-level specification and test-plan documents.  The spec treats `aar mcp` as a transport adapter that binds each MCP session to one case-role or case-member assignment, exposes stable assignment tool sets, normalizes wait responses, injects the active opportunity id, and forwards calls to the service role APIs.  AAR remains the authority for case state, role validation, member validation, deadlines, attempts, and terminal case status.

The test plan separates unit, process, and service tests.  It covers session binding, authentication, origin checks, tool lists, wait normalization, opportunity-id injection, forwarding, error propagation, process health, logs, and service-backed lawyer, observer, and council assignments.  OpenClaw and Pi runs remain outside the minimum passing set for this adapter boundary.

The first executable pass now starts `aar mcp` as a subprocess, drives `/mcp` with JSON-RPC over HTTP, and uses fake Lawyer and Council role APIs behind the adapter.  The tests cover invalid startup, health readiness, bearer authentication, origin checks, missing and deleted sessions, lawyer, observer, and council tool sets, wait-state normalization, opportunity-id injection, AAR `ok:false` and non-2xx propagation, outbound service authorization, and log redaction.  Idle-session expiry remains a direct unit test because testing it through the process would depend on wall-clock timing rather than the expiry rule.

### Provider and transport cleanup

Reference: [Council API](../councilapi.md), [OpenClaw service runbook](running.md), [Pi container README](../common/pi-container/README.md)

AAR council calls now use direct provider clients for the `direct` backend.  Council seats carry JSON request specs with endpoint, model, provider, quantization, request parameters, and persona information.  The case runner no longer starts a local provider proxy, and the CLI no longer accepts provider-proxy or removed council-agent flags.

Local service examples now run OpenClaw containers for lawyers and Pi containers for council members.  Council members receive their model and routing configuration through the mounted Pi home and reach the case through the Council API-backed MCP service.  Shared persona-generation tools now probe OpenRouter directly and keep OpenAI embeddings direct.

### Lawyer case results

Reference: [Lawyer HTTP API](../lawyerapi.md), [OpenClaw service runbook](running.md)

The Lawyer API now exposes `GET /lawyerapi/v1/result`.  The request uses the same `case_id` and `role_id` shape as the rest of the API.  While the case remains open, the response reports `status: "pending"` and returns the live turn envelope.  After the case closes, it returns the resolution, final reason when known, deliberation round, every stored council vote with rationale, and vote counts by round.

The unified MCP server exposes the same data through the read-only `get_case_result` tool.  This keeps final-result inspection available to lawyers and observers without adding another polling loop or reading output files from the operator's filesystem.  The MCP server does not interpret the vote data; it forwards the case-result JSON returned by AAR.

### Lawyer case status

Reference: [Lawyer HTTP API](../lawyerapi.md), [OpenClaw service runbook](running.md)

The Lawyer API now exposes `GET /lawyerapi/v1/status` and the read-only `case_status` tool.  The response reports the role's current status, case phase, case status, active turn, current opportunity details, state version, and compact counts for evidence, filings, events, and council votes.  The unified MCP server exposes `case_status` through the stable tool set and calls the status endpoint directly, so a waiting lawyer can inspect case status without an active `opportunity_id`.

### Lawyer Evidence Tools

Reference: [Evidence Handling](docs/evidence-handling.md), [OpenClaw lawyer runbook](running.md)

The Lawyer API now separates read access from evidence submission.  Read-only evidence tools are available in every active lawyer phase, so a remote lawyer can inspect case-packet files before an opening or closing.  Evidence-submission tools remain limited to arguments, rebuttals, and surrebuttals.

Surrebuttals now use the same exhibit and technical-report validation path as arguments and rebuttals.  This keeps surrebuttal narrow as a response phase while allowing the defendant to preserve and cite targeted source material when the plaintiff's rebuttal makes that necessary.  Openings and closings still file text-only legal acts through `submit_decision`.

Lawyer prompts now tell counsel to inspect the current record, scan the evidence list at each opportunity, analyze relevant evidence before advocating from it, and use targeted search when the record leaves a material gap.  They distinguish AAR court tools from native OpenClaw investigation tools, so a clawyer should use web, browser, file, shell, OCR, PDF, image, audio, video, metadata, hash, signature, archive, and local analysis tools when those tools can find or test material sources.  They require source-page retrieval after search results, adverse-source checks, and a search ledger when material evidence cannot be found or captured.  Counsel must submit material outside sources through AAR evidence tools before relying on them when evidence submission is available.  Remote clawyers receive case-packet files and later submissions through AAR evidence tools rather than local filesystem access.

`buildAttorneyPrompt` now adds the evidence-read reminder in every lawyer phase.  A test renders openings, arguments, rebuttals, surrebuttals, and closings through the single prompt directory, and checks that each generated prompt includes instructions for work notes, evidence scans, evidence analysis, native tools, browser work, local programs, and evidence-reading tools.

The Lawyer API now exposes `send_work_notes` in every active lawyer turn.  It writes the complete notes string to `work-notes.ndjson` with role, phase, turn, opportunity id, timestamp, and optional call id.  The prompts now describe those notes as a working journal: plans, issue outlines, work logs, sources checked, scripts or programs written, packages installed, browser work, OCR and extraction work, adverse checks, errors, analysis, decisions, and unresolved gaps.  The notes log is outside the record: it does not enter Lean state, `events.ndjson`, transcript output, digest output, evidence manifests, or observer event tools.  The MCP adapter exposes the tool as part of the stable lawyer transport tool set.

The removed OpenClaw attorney adapter no longer belongs to the runtime.  The supported OpenClaw path is now `aar service` plus `aar mcp`, with lawyers and council members acting through service-backed MCP tools.

Repeated OpenClaw runs showed plaintiff finding useful sources but attempting to submit them by calling `submit_decision` with `tool_name: submit_evidence`.  Defendant could submit evidence directly in the same service, so the failure was prompt and schema ambiguity rather than a server-wide submission failure.  The lawyer prompts and runbook assignment text now say that evidence admission uses the direct `submit_evidence` tool, or the direct chunked-upload tools, before the final filing.  They also state that `submit_decision` is only for the final legal act and must not wrap `submit_evidence`.  The `submit_decision` schema now filters the engine action list to final filing actions, so `submit_evidence` is no longer advertised as a valid `submit_decision.tool_name`.

## 2026-06-01

### Council API and MCP adapter

Reference: [Council HTTP API](../councilapi.md)

The Council API follows the Lawyer API architecture but binds each active client to `case_id` and `member_id`.  The HTTP server exposes `get`, `wait`, and `do`, and the MCP adapter only brokers those calls over Streamable HTTP.  The API keeps vote validation, deadlines, attempts, and evidence read budgets in AAR rather than moving that state into an agent adapter.

The adapter uses one MCP session per case-member.  A failed or expired MCP session can be re-created with the same URL because AAR remains the source of the active opportunity and turn budget.  The council tool set is small enough to expose dynamically from the current Council API status without adding adapter-side arbitration rules.

## 2026-06-01

### Lawyer API

Reference: [Lawyer HTTP API](../lawyerapi.md)

The lawyer side now uses one HTTP API owned by `aar case`.  The runner starts `/lawyerapi/v1`, publishes one active turn at a time, and blocks until the active lawyer submits a valid `submit_decision` call, exhausts attempts, or reaches the turn deadline.  Plaintiff and defendant integrations now sit outside the runtime and can use curl, a CLI, an MCP server, or another client that speaks this API.

The old local lawyer agent path has been removed from the AAR runtime.  Council support now uses direct provider calls or the Council API.  Shared evidence validation, filing validation, and prompt construction remain in the runner and are called by the HTTP API.

The Lawyer API now treats `opportunity_id` as a per-turn guard on plaintiff and defendant `POST /do` calls.  A lawyer receives the current value from `GET /get` in `turn.opportunity_id` and must send it back with every lawyer tool call for that turn.  Missing or stale values fail before tool execution and do not consume the turn's invalid-attempt budget.

The lawyer prompt templates now match that API.  They distinguish HTTP tools from legal acts submitted through `submit_decision`, state the current opportunity id, and remove old local-agent wording.  The single prompt set now contains the evidence-focused source retrieval, preservation, and work-note guidance that previously lived in a separate prompt override directory.

The handbook now gives remote clawyers one procedural and technical reference.  It treats the Lawyer HTTP API as the governing interface and describes MCP as one shared service process with one MCP session per case-role.  The handbook covers phase order, filing rules, evidence custody, turn budgets, observer use, MCP tool mapping, reconnection, and error handling.

The unified MCP server implements the MCP path described in the handbook.  It serves Streamable HTTP at `/mcp`, binds each MCP session from `case_id` and either `role_id` or `member_id` query parameters, exposes assignment-specific tools, and forwards `tools/call` requests to the service role APIs.  It fetches the live opportunity before every forwarded mutating tool call, injects the active `opportunity_id`, and returns AAR failures as MCP tool results with structured content.  The runner remains the phase authority.

OpenClaw onboarding now uses assignment text plus an MCP server definition.  The remote-user flow is the same for lawyers and council members: the operator gives OpenClaw the case id, assignment id, MCP URL, and token; the claw records the MCP server definition, verifies `wait_for_opportunity`, and enters the wait-tool operating loop.  The claw does not need a scheduled Gateway job to discover turns.

The Lawyer HTTP API now has `/lawyerapi/v1/wait`.  It returns the same status shape as `/get`, but it blocks until a role has work, case state changes, or the request timeout expires.  The response includes `wait.version`, so a runner can call the endpoint again with `after_version` and avoid choosing its own sleep interval.

The unified MCP server exposes `wait_for_opportunity` as an always-available read-only tool.  The server maps that tool to `/wait`, caps each call at 30 seconds, and normalizes the result to `state: ready`, `state: waiting`, `state: done`, or `state: error`.  The OpenClaw-facing instructions tell a clawyer or council member to call `wait_for_opportunity` repeatedly until it receives work, completion, or an error.

`aar mcp` runs as a shared service for many case-role and case-member sessions.  Each MCP session stores the binding for `case_id` plus one principal id; it does not own case state.  Idle-session expiry can delete stale MCP session records without changing an arb.  A clawyer or council member that loses a session can initialize a new MCP session with the same URL and recover current status from the service role APIs.  The server has a default 30-minute idle TTL, a configurable cleanup interval, and `--session-ttl 0` for deployments that want to disable expiry.

- [x] Add the HTTP Lawyer API server to `aar case`.
- [x] Replace local lawyer execution with turn blocking on HTTP tool calls.
- [x] Remove lawyer model, lawyer agent command, lawyer endpoint, and bridge CLI flags.
- [x] Delete the old OpenClaw lawyer adapter and bridge files.
- [x] Update prompt text to use HTTP tool names.
- [x] Require active opportunity ids on lawyer tool calls.
- [x] Clean up default and evidence-rich lawyer prompt templates.
- [x] Draft the arbitration handbook for remote clawyers.
- [x] Add the shared MCP adapter for OpenClaw lawyer sessions.
- [x] Draft the OpenClaw `arb` skill for self-service clawyer assignment.
- [x] Add `/wait` and MCP `wait_for_opportunity` for bounded turn waits.
- [x] Expire idle MCP sessions in the shared adapter.

## 2026-04-01

### Literate Lean proof pass

Reference: [Literate Lean notes](docs/literate-lean.md)

The first proof batch does not try to prove the whole procedure at once.  It
states a few properties that the present engine already claims to implement and
that are useful enough to stabilize early.

The current proof files are:

| File | Purpose |
|---|---|
| `engine/Proofs/InitializeCase.lean` | Policy and initialization postconditions |
| `engine/Proofs/MeritsFlow.lean` | Ordered phase progression through the merits sequence |
| `engine/Proofs/Deliberation.lean` | Vote threshold, no-majority closure, round advance, and member selection |

The shared sample file, `engine/Proofs/Samples.lean`, exists only to keep the
later files readable.  It collects the small example states and the narrow
field-extraction helpers that the theorems need.

### Why these proofs first

Initialization, phase order, and deliberation are the parts of the engine that
give the procedure its meaning.  The proofs are still sample-based, but they
are not arbitrary tests.  Each theorem states a procedural fact that should
remain true if the engine changes later.

### Initial proof targets

- Prove the symmetric policy facts that motivated shared per-side limits.
- Prove more about opportunity selection in rebuttal, surrebuttal, and
  deliberation.
- Prove cumulative material limits on exhibits and technical reports.
- Consider whether the engine should expose cleaner helper definitions for more
  general theorems about deliberation and closure.

### Reachable-state invariants

The proof set no longer stops at representative examples.  The current files
now prove two global invariants over every Lean state reachable through
successful initialization and successful public `step` transitions.

| File | Purpose |
|---|---|
| `engine/Proofs/ReachableInvariants.lean` | Every reachable state preserves the merits-sequence invariant, and therefore procedural parity |
| `engine/Proofs/ReachableMaterialLimits.lean` | Every reachable state respects the cumulative exhibit and report caps |
| `engine/Proofs/StepPreservation.lean` | Public `step` preservation for openings, arguments, rebuttals, surrebuttals, closings, optional passes, council votes, and council-member removal |

This changed the proof burden.  The hard part is no longer to state the global
theorems.  It is to keep the step-preservation layer readable while it mirrors
the executable branching structure in `Main.lean`.

### Next proof targets

- Prove stronger global facts about council composition and vote thresholds.
- Prove more about opportunity selection from reachable states, not only about
  state preservation after a successful step.
- Simplify some proof surfaces in `StepPreservation.lean` so the executable
  branches and the proof branches line up more directly.

## 2026-04-02

### Deliberation-neutrality policy decision

Reference: [Verification](docs/verification.md)

The proof work exposed a policy-space problem rather than a coding defect.
`currentResolution?` checks `demonstrated` before `not_demonstrated`.  That is
acceptable only if both outcomes cannot simultaneously satisfy the configured
threshold.  The validator previously allowed that overlap.

The engine now resolves that at the policy boundary.  `validatePolicy` in Lean
and Go requires `2 * required_votes_for_decision > council_size`.  That keeps
the current aggregation rule, removes the dual-threshold cases, and makes the
planned deliberation-neutrality theorem a theorem about the whole validated
policy space rather than a theorem with an extra side condition.

### Deliberation-neutrality proof

Reference: [Verification](docs/verification.md)

Stage 7 is now complete in `engine/Proofs/Neutrality.lean`.  The proof does
not quantify over arbitrary malformed cases.  It proves neutrality over
reachable states, where the existing integrity layer already guarantees that
current-round votes come from distinct seated members and cannot outgrow the
configured council size.

The key proof shape is simple.  First, define a vote-flip map on council
votes and show that flipping the current round swaps the two substantive vote
counts.  Then combine that with the strict-majority validator and the
reachable seat bound to exclude dual-threshold states.  That is enough to show
that `currentResolution?` commutes with the vote flip on every reachable
state.

## 2026-04-03

### Explicit case-file selection for `aar case`

`aar case` still defaults to loading case files from the complaint directory.
That behavior is convenient for the examples, but it depends on a directory
scan and a skip list.  The CLI now also accepts repeated `--file` arguments,
including glob patterns, and passes the resolved file list into the runner.

The explicit list replaces the directory scan entirely.  That keeps the old
default while giving the caller a precise file boundary for one run.  The CLI
expands globs, rejects unmatched glob patterns, and rejects prohibited
extensions: `.gitignore`, `.sh`, and `.sig`.  The runner then loads exactly
those files and fails on duplicate basenames, because the case record keys
files by visible filename.

### `aar case` summary JSON

`aar case` now writes one JSON object to standard output for execution
results.  On success, the object reports the resolution and the final-round
counts for votes for and against the proposition.  On failure, the object
reports the error string.

The command still exits nonzero on failure.  The CLI wraps those failures in a
reported-error type so the JSON object remains the only case-result payload on
standard output and the binary does not add a second plain-text error line for
that path.

### Attorney web search in removed local-agent runs

The attorney prompts already instructed the model to use native web search when
public investigation mattered, but the old local-agent path did not stage a
search-enabled model into the temporary Pi home.  The attorneys were told to do
work that the runtime had not enabled.

That path has since been deleted.  The current lawyer design puts model
selection outside AAR and keeps AAR responsible for case access, evidence
validation, filing validation, and turn budgets.  The lawyer prompt still
requires source retrieval, evidence preservation, analysis, and a work log.

The old attorney timeout also became too short once public-source
investigation was enabled.  In `ex4`, the plaintiff arguments turn used enough
public-source investigation to exceed 480 seconds before filing.  The default
attorney timeout is now 900 seconds.

### Attorney filing limits in prompts

`ex4` exposed a second prompt defect after web search was enabled.  The
attorneys could now gather the needed material, but the prompt still left key
filing constraints implicit.  The plaintiff rebuttal then burned its retries on
 three avoidable mistakes: a rebuttal that exceeded the text limit, too many
technical reports for the side-wide cap, and earlier attempts to place
workspace filenames in `offered_files`.

The prompt and attorney view now state the hard limits for the current
opportunity.  That includes the text limit for the current filing, the per-file
and per-side exhibit and technical-report caps, the amount already used by the
current side, and the remaining capacity.  The prompt now also states the real
record rule: `offered_files` may name only visible case files by `file_id`;
outside material enters through `technical_reports`.

Attorney validation errors now carry the attempted count and the remaining
side capacity.  That keeps the model close to the actual engine rule and avoids
wasting retries on blind correction attempts.

### Retired lawyer model configuration

Older revisions allowed `aar case` to configure lawyer models and local or
remote lawyer agent commands.  The `lawyerapi` branch removed that path and
left lawyer model selection to clients outside the runtime.

The removed plaintiff demo staged a backend Pi home through the same code path
that ordinary attorney runs used.  `aar` exposed two helper commands for that
purpose: one staged the Pi home into a supplied directory, and one printed the
current lawyer tool catalog as JSON.  The demo script used those helpers
instead of carrying its own copies of `settings.json`, `models.json`, and the
tool schema.

## 2026-04-30

### Ignore regenerated signing evidence in `ex1`

Reference: [Example signer](examples/ex1/sign.sh)

`examples/ex1` regenerates `samantha_public.pem` and `confession.sig.b64` from
the ignored source inputs `samantha_private.pem` and `confession.sig`.  Keeping
the derived files tracked leaves the worktree dirty after an ordinary example
run.

The local `.gitignore` in `examples/ex1` now ignores those derived outputs as
well.  The repository index must also stop tracking them, because ignore rules
do not apply to files that Git already tracks.

### Invalid-attempt limit errors now preserve reasons

Reference: [Attorney tool helpers](runtime/runner/attorney_tools.go), [Council runner](runtime/runner/council.go)

The attorney and council runners previously replaced the decisive validation
message with a generic invalid-attempt ceiling error on the final failed
submission.  That made the failure hard to diagnose, because the run-level
error lost the exact reason that had already been returned to the agent during
the correction loop.

The runner now carries the invalid reasons forward and includes them in the
final limit error in attempt order.  That keeps the stop condition the same,
but it makes the terminal error match the actual rejection path instead of
hiding it behind a generic summary.

### Invalid submission feedback now explains the next step

Reference: [Attorney tool helpers](runtime/runner/attorney_tools.go)

The attorney tool path previously returned only the bare validation error on
each rejected submission.  That told the model what failed, but it did not say
how many invalid submissions remained or what another miss would do to the
run.  The handler now returns structured rejection text with the current
invalid-submission count, the remaining budget for the opportunity, and one
corrective instruction.

Length failures now report submitted and allowed characters, direct the agent
to count characters rather than tokens, and give a resubmission target below
the hard cap.  Final exhausted attempts switch to terminal language and state
that the opportunity has failed and the run is ending with an error.  The
terminal message still includes the ordered invalid-submission history.

That change fixed a real mismatch.  The earlier script omitted the write-file
tool and hand-built the Pi configuration.  After the change, the external
plaintiff opening matched the ordinary local path closely enough to complete:
note file write, opening submission, accepted filing.

It did not fix the plaintiff arguments failure in `ex6`.  The plaintiff still
stalled in the arguments phase.  The failure mode changed, which narrows the
cause.  The old run spent its time rewriting notes around citation formatting
and source packaging.  The new run used the full tool surface and reached the
substance faster.  It still kept rewriting `case-notes.md`, but the content now
tracked the adverse merits directly: the notes concluded that the official
record supports ground entry but likely not the territorial-objective element,
and that the plaintiff's best colorable `YES` theory runs into the explicit
edge-case carveout.  That points to a prompt or role-interface problem about
how plaintiff advocacy should proceed when truthful investigation turns the case
against the assigned side.  It does not point to agent transport or Pi-home
staging any longer.

## 2026-04-08

### Verification document consolidation

Reference: [Verification](docs/verification.md)

The verification material had split into a status note, a stage plan, and a
findings note.  That separation made the current state harder to read, because
a reader had to reconstruct one story from three files.  The documentation now
uses `docs/verification.md` as the canonical record for established results,
the finished stage structure, proof-driven findings, and the limits of what the
Lean engine can prove.

### Abstract verification structures

Reference: [More verification notes](docs/more-verification-notes.md)

The next proof work now has a separate note about abstractions that the current
engine already suggests.  The strongest candidates are a progress preorder over
fixed-frame runs, a compact deliberation summary, a viable-outcomes notion for
threshold reachability, the existing vote-flip involution, a lexicographic
termination potential, and a trace semantics for successful runs.  The
recommended first extension is a deliberation-summary layer that isolates
counts, remaining eligible voters, round budget, and outcome attainability from
the full case record.

### Deliberation summary proof layer

Reference: [More verification notes](docs/more-verification-notes.md)

The first implementation step now spans
`engine/Proofs/DeliberationSummaryCore.lean` and
`engine/Proofs/DeliberationSummary.lean`.  The core file now carries the
compact proof-side `DeliberationSummary` record, the direct case-level
correspondence with `currentResolution?`, and the lower council arithmetic that
the summary layer needs.  The wrapper file keeps the reachable vote-count,
seated-count, and positive-threshold bounds that rely on later proof layers.

### Summary-core dependency split

Reference: [Verification](docs/verification.md)

The import graph had blocked the next summary-based compression.  `OutcomeSoundness.lean`
and `NoStuck.lean` sat below `DeliberationSummary.lean`, because that file had
been importing `BoundedTermination.lean` for a few local arithmetic lemmas and
for the reachable wrappers.  The summary layer now splits at that boundary:
`DeliberationSummaryCore.lean` sits below `OutcomeSoundness.lean`, while
`DeliberationSummary.lean` keeps only the reachable wrappers above `NoStuck`.

That change pulled the direct `currentResolution?` soundness facts into the
summary core and let `OutcomeSoundness.lean` consume them directly.  The lower
termination file now imports the core arithmetic instead of defining the same
council-length and current-round-capacity lemmas itself.  The remaining import
pressure is now on the liveness side rather than on outcome soundness.

### Summary-form liveness bridge

Reference: [Verification](docs/verification.md)

The next split now reaches one theorem in `NoStuck.lean`.  The selector fact
that `nextCouncilMember?` returns a seated member who has not yet voted moved
into `DeliberationSummaryCore.lean`, together with the summary-capacity lemma
that turns that fact into `current_round_vote_count < seated_count`.  `NoStuck.lean`
now uses those lower results to prove the summary-form round-capacity theorem
for every reachable live deliberation state.

This matters because it moves one real liveness theorem below
`ViableOutcomes.lean` instead of leaving the whole summary bridge above the
existing Stage 3 file.  The remaining pressure is now narrower: the viability
and closure facts still sit above `NoStuck.lean`, but the basic summary view
of live deliberation no longer does.

### Viability-core dependency split

Reference: [Verification](docs/verification.md)

The same import pressure then showed up inside the viability layer.  The
summary-level viability definitions and lemmas had been sitting in
`ViableOutcomes.lean` above the executable update correspondences, even though
most of them did not depend on removal arithmetic or on later proof layers.
The viability layer now splits the same way the summary layer did:
`ViableOutcomesCore.lean` carries the pure viability language and the
summary-only theorems, while `ViableOutcomes.lean` keeps the direct vote and
removal update correspondences.

This matters on the closure side.  `OutcomeSoundness.lean` now imports
`ViableOutcomesCore.lean` and proves the `no_majority` branch through summary
non-viability instead of reopening the threshold arithmetic directly from
`currentResolution? = none`.  The core file now also carries a summary closure
predicate for `no_majority`, so the lower layer can package the executable
closure reasons with the below-threshold conclusion before `OutcomeSoundness.lean`
translates the result back to the state-level statement.  `OutcomeSoundness.lean`
now also proves the direct bridge in both directions: summary `no_majority`
closure is sufficient for `continueDeliberation` to close that way, and an
executable `no_majority` closure from deliberation implies the same summary
predicate on the source state.  That leaves the higher file responsible only
for the executable update correspondence lemmas that still depend on the later
termination layer.

### Viable outcomes proof layer

Reference: [More verification notes](docs/more-verification-notes.md)

The second implementation step now spans `engine/Proofs/ViableOutcomesCore.lean`
and `engine/Proofs/ViableOutcomes.lean`.  The core file defines summary-level
viability for the two substantive outcomes, proves the first shrinkage facts,
and packages the pure summary-side closure lemmas.  A vote for one side
preserves that side's viability and can only shrink the other side's viability.
Removing one seated member can only shrink viability for both sides.  The
higher file then proves that these summary updates match the intermediate
deliberation states produced by direct vote and removal updates before
`continueDeliberation` runs.

### Summary-based public wrappers

Reference: [More verification notes](docs/more-verification-notes.md)

The first bridge theorems for the third stage now split across
`engine/Proofs/OutcomeSoundness.lean`, `engine/Proofs/NoStuck.lean`,
`engine/Proofs/ViableOutcomesCore.lean`, and `engine/Proofs/ViableOutcomes.lean`.
The liveness side now proves the summary-form current-round capacity bound in
`NoStuck.lean`.  The closure side now proves the `no_majority` arithmetic
through summary non-viability in `OutcomeSoundness.lean`.  The core viability
file handles the summary-side facts: executable `currentResolution?` implies
the corresponding summary-viability fact, summary-level exhaustion implies
executable non-resolution, and the summary-level count flip swaps the two
substantive outcomes.  The higher viability file then handles the executable
vote and removal update correspondences.  `engine/Proofs/Neutrality.lean` now
uses that lower summary form directly, so the reachable vote-flip theorem is
stated over the same public result but proved through `DeliberationSummary`
instead of through another round of raw vote-count case analysis on the case
record.

### Closed-resolution bridge

Reference: [Verification](docs/verification.md)

The next compression step turned the summary closure language into one uniform
bridge for closed deliberation results.  `ViableOutcomesCore.lean` now defines
the proof-side `DeliberationSummary.closedResolution?` function and proves the
summary equalities that correspond to substantive threshold closure and to
`no_majority` closure.  `OutcomeSoundness.lean` now proves the executable
bridge in both directions: if the source summary reports a closed resolution,
`continueDeliberation` returns exactly that closed result, and if
`continueDeliberation` closes a deliberation-phase case, the source summary
reports the same result.

This matters because the summary layer no longer describes only the
`no_majority` branch.  It now packages the whole closed-output boundary of
`continueDeliberation`, which is the right granularity for later monotonicity
or inevitability theorems.  The remaining higher work is correspondingly
narrower: the executable vote and removal update correspondences still sit
above this layer, but the closure logic itself now has one proof-side shape.

### Executable viability transport

Reference: [More verification notes](docs/more-verification-notes.md)

The next step converted those remaining executable update correspondences into
real viability statements.  `ViableOutcomes.lean` still uses the summary equalities
for the intermediate vote and removal cases before `continueDeliberation`, but
it now proves what those equalities mean for the engine state.  A vote for
`demonstrated` preserves demonstrated viability and preserves impossibility of
`not_demonstrated`.  A vote for `not_demonstrated` preserves not-demonstrated
viability and preserves impossibility of `demonstrated`.  A seated-member
removal preserves impossibility for both substantive outcomes.

This matters because the higher viability file is no longer only a transport
layer.  It now carries executable impossibility facts that the later public
step theorems can consume without reopening the arithmetic in the summary core.

### Same-round final-state bridge

Reference: [Verification](docs/verification.md)

The next step pushed that transport across the `continueDeliberation` boundary
when the round does not advance.  `ViableOutcomes.lean` now proves a compact
congruence fact for `DeliberationSummary`: if `continueDeliberation` keeps the
same deliberation round, then the final state has the same summary as the
intermediate `stateWithCase s c`.  That is the right bridge because the
function may still close the case in place, but closure changes none of the
summary fields.

That bridge supports two new public same-round results.  First, a successful
council-vote step now yields an existential `sameRoundVoteTransport` theorem:
for the submitted vote label, the final state preserves viability of the voted
side and preserves impossibility of the opposite side.  Second, a successful
council-member removal step now preserves demonstrated impossibility,
not-demonstrated impossibility, and therefore total substantive non-viability
when the round stays fixed.

### Progress-viability bridge

Reference: [More verification notes](docs/more-verification-notes.md)

The next step connected those same-round deliberation facts to the structural
progress layer without overstating what `fixedFrameProgress` can prove by
itself.  `ProgressViability.lean` imports both `Progress.lean` and
`ViableOutcomes.lean` and proves two public bridge theorems.  A successful
same-round council-vote step now yields both `fixedFrameProgress s t` and an
existential `sameRoundVoteTransport` witness.  A successful same-round
council-member removal step now yields `fixedFrameProgress s t` together with
an implication from source total substantive non-viability to target total
substantive non-viability.

This matters because it marks the boundary of the current abstraction honestly.
The present preorder tracks case frame, materials, seats, phase rank, and
round.  It does not track current-round votes.  The new bridge therefore pairs
progress with viability transport on the concrete same-round deliberation steps
where the vote update is known, instead of claiming a false global monotonicity
theorem for `fixedFrameProgress` alone.

### Same-round deliberation progress

Reference: [Verification](docs/verification.md)

The next step turned that bridge into a proof-side relation.  `ProgressViability.lean`
now defines `viableOutcomesShrink`, which says that target viability for either
substantive outcome implies source viability for that same outcome.  It then
defines `sameRoundDeliberationProgress`, which combines `fixedFrameProgress`,
same-round equality, and that shrink relation.  Both new relations are
reflexive and transitive.

The public step theorems now establish that same-round relation for successful
council-vote and council-removal steps.  The vote side uses a new lower wrapper
in `StepPreservation.lean` that exposes the already-forced vote-label
disjunction from `recordCouncilVote`.  The removal-side non-viability
preservation theorem now follows from `viableOutcomesShrink` instead of sitting
as a separate ad hoc implication.  This is the first abstract relation in the
library that tracks both structural progress and substantive viability
shrinkage without pretending that the global preorder already contains current-round
vote data.

### Same-round closure inevitability

Reference: [More verification notes](docs/more-verification-notes.md)

The next step completed that same-round line.  `ProgressViability.lean` now
proves that `sameRoundDeliberationProgress` preserves `no_majority` closure
reasons in the only form that matters for later closure: the target state has
completed the round.  The key structural lemma here is seat-count monotonicity
under `fixedFrameProgress` plus source council-id uniqueness.  That suffices to
carry the "too few seats" closure reason forward, while same-round equality and
fixed policy carry the last-round reason.

The file then packages the main theorem: if the source summary already has no
viable substantive outcome and already has one `no_majority` closure reason,
then any later same-round progress state that completes the round is forced to
summary `no_majority` closure.  The executable corollary is direct through
`OutcomeSoundness.lean`: `continueDeliberation` on that target state must close
as `no_majority`.  The public council-vote and council-removal theorems now
inherit that result.  This finishes the summary, viability, and same-round
progress agenda as a coherent proof line.

### Fixed-frame progress preorder

Reference: [More verification notes](docs/more-verification-notes.md)

The next implementation step now lives in `engine/Proofs/Progress.lean`.  The
file defines `fixedFrameProgress`, a state relation anchored to the source
frame and paired with the monotone coordinates that the library had been
proving separately: append-only admitted materials, shrinking seated-member
identifiers, nondecreasing phase rank, and nondecreasing deliberation round.
The first theorem batch proves reflexivity and transitivity, shows that every
successful public step establishes that relation, and packages the initialized
run form as the conjunction of the initialization frame and source-anchored
progress from the initialized state.

### Attorney tool-error handling

The attorney guidance now states that tool errors are authoritative host
feedback and that counsel must change the request before retrying the same
tool.  I added that rule to both the standing attorney instructions and the
always-sent attorney court prompt.  The duplication is deliberate because the
standing file does not travel over every remote client path, while the common
court prompt always does.

### Opening cap and target margin

The next policy change raises `max_opening_chars` from `4000` to `5000` in both
the built-in default policy and the checked-in `etc/policy.json` that `make`
targets load by default.  The target-length guidance now uses 75% of the hard
cap again for both the first-submission prompt target and the retry hint.  That
gives openings a `3750` target under a `5000` cap, while leaving the hard cap
itself configurable through policy JSON.

## 2026-05-04

### Flexible complaint input

Reference: [Complaint parser](runtime/spec/complaint.go)

The arbitration runtime needs one proposition string.  The source file format
no longer has to carry a literal `# Proposition` heading for the parser to
produce that value.  When a `Proposition` section exists, the parser uses that
section.  When no such section exists, the parser treats the whole trimmed file
as the proposition.

The canonical writer still emits a `# Proposition` heading.  That keeps
generated complaint packets stable and readable.  Empty input fails, and an
explicit empty `Proposition` section fails, because either case lacks a
proposition.

- [x] Preserve canonical complaint output.
- [x] Accept plain text as complaint input.
- [x] Reject blank complaints and blank explicit sections.
- [x] Cover parser behavior in tests.

## 2026-06-02

### Public service startup

Reference: [AAR service](runtime/service/service.go)

The first `ex1` service run failed during case creation because the public
service waited only thirty seconds for the child runner to announce private
lawyer and council APIs.  The child runner starts those private APIs after
council preflight, and council preflight can spend more than thirty seconds on
external model availability checks.  The public service now returns an accepted
case once the child process starts, keeps the case in `starting`, and lets
public role `wait` calls block within the API wait limit until the private role
API appears.

The corrected path was tested with `ex1` and `ex4` through the public service,
the AAR MCP adapter, OpenClaw lawyer containers, and council members using the
council API.  `ex1` closed as demonstrated with a 4-1 council vote, and `ex4`
closed as demonstrated with a 5-0 council vote.  The searched MCP logs for both
runs showed no HTTP 4xx or 5xx tool calls and no MCP error states.

### Agent lifecycle

The `ex1` OpenClaw/Pi run showed repeated C4 MCP sessions because the example
runner restarted agents and used them to check for work.  That lifecycle was
wrong.  The example runner now starts each lawyer or council agent once and
lets `set -e` fail the run when a command fails.

### Private case API startup

Reference: [Service runner](runtime/service/service.go)

The public service no longer reads child API URLs from child stderr.  It chooses
one local private address before it starts `aar case`, passes that address as
`--caseapi-addr`, records `caseapi_base`, and polls `GET /health` on that base
until startup succeeds or the configured startup timeout expires.  The child
case API serves `/health`, `/lawyerapi/v1/...`, and `/councilapi/v1/...` on the
same private listener when the Council API backend is active.

The subprocess tests also exposed invalid stdout-pipe ordering.  Both the
service child watcher and the black-box process test code now wait for stdout
capture to finish before calling `cmd.Wait()`, matching Go's `StdoutPipe`
requirements and preserving the final JSON summary for service status.

### Service-backed MCP process test

Reference: [MCP process test](runtime/cmd/aar/mcp_blackbox_test.go)

The external MCP test now starts `aar service`, starts `aar mcp`, creates a
real service-managed case with the Council API backend, and drives plaintiff,
defendant, observer, and council assignments through MCP JSON-RPC.  The test
checks tool lists, observer rejection of mutating tools, work-note recording,
evidence reading, lawyer filings, council votes, service final result data, and
the case artifacts written under the output directory.
