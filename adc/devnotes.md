# Development Notes

## 2026-07-15: Judge Rule 56 eval

### References

- Judge eval plan: [`evals/judge/plan.md`](evals/judge/plan.md)
- Rule 56 plan: [`evals/judge/rules/rule56/plan.md`](evals/judge/rules/rule56/plan.md)
- Rule 56 fixtures: [`evals/judge/rules/rule56/fixtures.jsonl`](evals/judge/rules/rule56/fixtures.jsonl)
- Candidate prompt v1: [`evals/judge/rules/rule56/prompts/candidate-v1.md`](evals/judge/rules/rule56/prompts/candidate-v1.md)
- Candidate prompt v2: [`evals/judge/rules/rule56/prompts/candidate-v2.md`](evals/judge/rules/rule56/prompts/candidate-v2.md)
- Analysis: [`evals/judge/rules/rule56/analysis.md`](evals/judge/rules/rule56/analysis.md)
- Runner: [`runtime/eval/judge_rule56.go`](runtime/eval/judge_rule56.go)
- CLI entry point: [`runtime/cli/eval.go`](runtime/cli/eval.go)

### Decisions

`adc eval judge-rule56` evaluates the judge's `decide_rule56_motion` behavior against fixed JSONL fixtures.  Each fixture builds a pretrial ADC state with a Rule 56 motion, opposition, and optional reply in the docket, obtains the real Lean judge opportunity, asks for a single tool call, applies the decision through Lean, and scores the resulting disposition.  The first fixture set has 30 rows across clean grants, clean denials, partial dispositions, credibility disputes, competing inferences, authentication disputes, unsupported damages theories, movant burden failures, and legal bars.

Production scored 29 correct dispositions, 30 reason matches, one false grant, and no invalid responses on the full live run.  The false grant was `r56-009`: the motion attacked consequential lost profits only, but the model returned `granted` rather than `partial`, overresolving the claim while direct damages and liability remained.  Candidate v1 fixed that row by emphasizing motion scope, but it introduced a false grant on `r56-027` by granting partial summary judgment on an immaterial deposition subpoint despite a genuine dispute on the material reliance issue.

Candidate v2 is the best measured Rule 56 prompt.  It preserves the scope rule for damages-only and issue-only motions, and it blocks partial grants on isolated document sentences, deposition answers, or subfacts when the full record supports a competing inference on the material issue.  Candidate v2 scored 30 correct dispositions, 30 reason matches, no false grants, no false denials, and no invalid responses on the full live run.

### Verification

- [x] `go test ./adc/runtime/eval ./adc/runtime/cli`
- [x] `go run ./adc/runtime/cmd/adc eval judge-rule56 --dry-run --engine adc/.bin/adcengine --out-dir adc/evals/judge/out/rule56-production-dry`
- [x] Short live run: `go run ./adc/runtime/cmd/adc eval judge-rule56 --limit 2 --engine adc/.bin/adcengine --out-dir adc/evals/judge/out/rule56-production-live-short`
- [x] Production live run: `go run ./adc/runtime/cmd/adc eval judge-rule56 --engine adc/.bin/adcengine --out-dir adc/evals/judge/out/rule56-production-live`
- [x] Candidate v1 dry run: `go run ./adc/runtime/cmd/adc eval judge-rule56 --dry-run --engine adc/.bin/adcengine --opportunity-prompt-file adc/evals/judge/rules/rule56/prompts/candidate-v1.md --opportunity-prompt-name candidate-v1 --out-dir adc/evals/judge/out/rule56-candidate-v1-dry`
- [x] Candidate v1 live run: `go run ./adc/runtime/cmd/adc eval judge-rule56 --engine adc/.bin/adcengine --opportunity-prompt-file adc/evals/judge/rules/rule56/prompts/candidate-v1.md --opportunity-prompt-name candidate-v1 --out-dir adc/evals/judge/out/rule56-candidate-v1-live`
- [x] Candidate v2 dry run: `go run ./adc/runtime/cmd/adc eval judge-rule56 --dry-run --engine adc/.bin/adcengine --opportunity-prompt-file adc/evals/judge/rules/rule56/prompts/candidate-v2.md --opportunity-prompt-name candidate-v2 --out-dir adc/evals/judge/out/rule56-candidate-v2-dry`
- [x] Candidate v2 live run: `go run ./adc/runtime/cmd/adc eval judge-rule56 --engine adc/.bin/adcengine --opportunity-prompt-file adc/evals/judge/rules/rule56/prompts/candidate-v2.md --opportunity-prompt-name candidate-v2 --out-dir adc/evals/judge/out/rule56-candidate-v2-live`

## 2026-07-14: Judge voir dire eval

### References

- Eval plan: [`../judge-eval.md`](../judge-eval.md)
- CLI entry point: [`runtime/cli/eval.go`](runtime/cli/eval.go)
- Eval package: [`runtime/eval/judge_voir_dire.go`](runtime/eval/judge_voir_dire.go)
- Fixtures: [`evals/judge/rules/rule47/voir_dire_questions.jsonl`](evals/judge/rules/rule47/voir_dire_questions.jsonl)
- Hard fixtures: [`evals/judge/rules/rule47/voir_dire_questions_hard_v1.jsonl`](evals/judge/rules/rule47/voir_dire_questions_hard_v1.jsonl)
- Candidate prompt v1: [`evals/judge/rules/rule47/prompts/candidate-v1.md`](evals/judge/rules/rule47/prompts/candidate-v1.md)
- Candidate prompt v2: [`evals/judge/rules/rule47/prompts/candidate-v2.md`](evals/judge/rules/rule47/prompts/candidate-v2.md)
- Candidate prompt v3: [`evals/judge/rules/rule47/prompts/candidate-v3.md`](evals/judge/rules/rule47/prompts/candidate-v3.md)
- Analysis: [`evals/judge/rules/rule47/analysis.md`](evals/judge/rules/rule47/analysis.md)

### Decisions

`adc eval judge-voir-dire` evaluates the judge's `decide_voir_dire_question` behavior against fixed JSONL fixtures.  The Go implementation lives under `runtime/eval`, and the fixture and report paths live under `evals/judge`, matching the planned boundary between eval code and eval data.  Rule 47 voir dire materials now live under `evals/judge/rules/rule47`, leaving the top-level judge directory for cross-rule planning and generated reports.  Each fixture builds a minimal ADC `voir_dire` state with one pending `VoirDireExchange`, asks the Lean engine for the current judge opportunity, builds the judge prompt from the real opportunity text and judge runtime brief, and scores the resulting `decide_voir_dire_question` tool call.

The initial fixture set has 60 questions.  The set covers allowed bias, burden, digital-evidence, damages-skepticism, attention, and instruction-following questions, and disallowed liability precommitment, damages precommitment, specific-evidence sufficiency, assumed-disputed-fact, merits-argument, inadmissible-material, and compound-precommitment questions.  Severity weights are higher for disallowed questions because a false allow can expose the juror to a prohibited question; false disallows remain separate summary fields because overblocking weakens jury selection.

The runner writes `results.jsonl` and `summary.json`.  Dry-run mode uses the expected ruling as a synthetic tool call, which validates fixture loading, state construction, Lean opportunity generation, prompt construction, scoring, report writing, and Lean acceptance without an external model request.  Live model runs use `endpoint://model` syntax through the existing OpenAI/OpenRouter client.  The command also supports `--rescore-results` so deterministic scorer changes can be applied to an existing live result file without repeating model calls.

Prompt iteration now has an eval-local path.  `--opportunity-prompt-file` reads a Markdown template under `evals/judge/rules/rule47/prompts`, renders fixture placeholders, and uses the rendered text as the model-facing opportunity objective.  The Lean opportunity still supplies the phase, role, allowed tool, constraints, and transition id, so candidate wording can be evaluated without changing the production opportunity text in `engine/Main.lean`.  Each report records the prompt source and copies the prompt file into the output directory.  Generated report data under `evals/judge/out/` is ignored and should not be committed.

The first two prompt candidates do not justify a production prompt change.  Candidate v1 preserved ruling outcomes on the live run, with 60 correct rulings, no false allows, no false disallows, and no invalid responses, but its explanation matches dropped from 60 to 55 because the prompt encouraged generic category wording such as “class of evidence.”  Candidate v2 required concrete ruling categories and improved explanation matches to 58, but it allowed fixture `jvd-053`, a tier-3 damages-precommitment question asking whether the candidate would be comfortable returning an $80,000 to $120,000 damages range if liability were proven.  That false allow is disqualifying for this prompt candidate.

`voir_dire_questions_hard_v1.jsonl` adds 30 tier-3 boundary rows without changing the original 60-row baseline.  The added rows concentrate on damages-range comfort questions, digital-evidence sufficiency, limiting-instruction phrasing that embeds disputed facts, missing-witness sufficiency, insurance references, and “could you still find” formulations.  Production scored 30 correct rulings, 30 reason matches, no false allows, and no invalid responses after the deterministic scorer accepted the singular phrase “limiting instruction” as instruction-following wording.

Candidate v1 failed the hard set by allowing `jvdh-002`, a prohibited damages-range comfort question.  Candidate v2 scored 30/30 on the hard set but had already failed the original baseline on `jvd-053`, another damages-range comfort question.  Candidate v3 adds a focused damages-number rule: bias or proof-discipline questions remain allowed, but questions asking whether the candidate would be comfortable with, willing to return, able to award, or inclined to reject a named damages amount, range, minimum, maximum, or nominal result are disallowed.  Candidate v3 scored 60 correct rulings, 59 reason matches, no false allows, and no invalid responses on the original set, and 30 correct rulings, 30 reason matches, no false allows, and no invalid responses on the hard set.  Production remains the best measured prompt on these two sets because it scored 60/60 and 30/30 on both ruling outcome and deterministic reason matching.

### Verification

- [x] `go test ./adc/runtime/eval`
- [x] `go test ./adc/runtime/cli`
- [x] `go test ./adc/runtime/eval ./adc/runtime/cli`
- [x] `go run ./adc/runtime/cmd/adc eval judge-voir-dire --dry-run --engine adc/.bin/adcengine --out-dir adc/evals/judge/out/latest`
- [x] `go run ./adc/runtime/cmd/adc eval judge-voir-dire --dry-run --engine adc/.bin/adcengine --opportunity-prompt-file adc/evals/judge/rules/rule47/prompts/candidate-v1.md --opportunity-prompt-name candidate-v1 --out-dir adc/evals/judge/out/candidate-v1-dry`
- [x] `go run ./adc/runtime/cmd/adc eval judge-voir-dire --dry-run --engine adc/.bin/adcengine --opportunity-prompt-file adc/evals/judge/rules/rule47/prompts/candidate-v2.md --opportunity-prompt-name candidate-v2 --out-dir adc/evals/judge/out/candidate-v2-dry`
- [x] `go run ./adc/runtime/cmd/adc eval judge-voir-dire --dry-run --engine adc/.bin/adcengine --opportunity-prompt-file adc/evals/judge/rules/rule47/prompts/candidate-v3.md --opportunity-prompt-name candidate-v3 --out-dir adc/evals/judge/out/candidate-v3-dry`
- [x] `go run ./adc/runtime/cmd/adc eval judge-voir-dire --dry-run --fixtures adc/evals/judge/rules/rule47/voir_dire_questions_hard_v1.jsonl --engine adc/.bin/adcengine --out-dir adc/evals/judge/out/hard-v1-dry`
- [x] `go run ./adc/runtime/cmd/adc eval judge-voir-dire --dry-run --fixtures adc/evals/judge/rules/rule47/voir_dire_questions_hard_v1.jsonl --engine adc/.bin/adcengine --opportunity-prompt-file adc/evals/judge/rules/rule47/prompts/candidate-v3.md --opportunity-prompt-name candidate-v3 --out-dir adc/evals/judge/out/hard-v1-candidate-v3-dry`
- [x] `go test ./adc/runtime/...`
- [x] Short live run: `go run ./adc/runtime/cmd/adc eval judge-voir-dire --limit 2 --engine adc/.bin/adcengine --out-dir adc/evals/judge/out/live-short`
- [x] Full live run: `go run ./adc/runtime/cmd/adc eval judge-voir-dire --engine adc/.bin/adcengine --out-dir adc/evals/judge/out/live-full`
- [x] Rescore completed live results after expanding deterministic reason-tag vocabulary for accepted wording on attention and instruction-following questions.
- [x] Candidate v1 live run: `go run ./adc/runtime/cmd/adc eval judge-voir-dire --engine adc/.bin/adcengine --opportunity-prompt-file adc/evals/judge/rules/rule47/prompts/candidate-v1.md --opportunity-prompt-name candidate-v1 --out-dir adc/evals/judge/out/candidate-v1-live`
- [x] Candidate v2 live run: `go run ./adc/runtime/cmd/adc eval judge-voir-dire --engine adc/.bin/adcengine --opportunity-prompt-file adc/evals/judge/rules/rule47/prompts/candidate-v2.md --opportunity-prompt-name candidate-v2 --out-dir adc/evals/judge/out/candidate-v2-live`
- [x] Candidate v3 live run: `go run ./adc/runtime/cmd/adc eval judge-voir-dire --engine adc/.bin/adcengine --opportunity-prompt-file adc/evals/judge/rules/rule47/prompts/candidate-v3.md --opportunity-prompt-name candidate-v3 --out-dir adc/evals/judge/out/candidate-v3-live`
- [x] Hard production live run: `go run ./adc/runtime/cmd/adc eval judge-voir-dire --fixtures adc/evals/judge/rules/rule47/voir_dire_questions_hard_v1.jsonl --engine adc/.bin/adcengine --out-dir adc/evals/judge/out/hard-v1-production-live`
- [x] Hard candidate v1 live run: `go run ./adc/runtime/cmd/adc eval judge-voir-dire --fixtures adc/evals/judge/rules/rule47/voir_dire_questions_hard_v1.jsonl --engine adc/.bin/adcengine --opportunity-prompt-file adc/evals/judge/rules/rule47/prompts/candidate-v1.md --opportunity-prompt-name candidate-v1 --out-dir adc/evals/judge/out/hard-v1-candidate-v1-live`
- [x] Hard candidate v2 live run: `go run ./adc/runtime/cmd/adc eval judge-voir-dire --fixtures adc/evals/judge/rules/rule47/voir_dire_questions_hard_v1.jsonl --engine adc/.bin/adcengine --opportunity-prompt-file adc/evals/judge/rules/rule47/prompts/candidate-v2.md --opportunity-prompt-name candidate-v2 --out-dir adc/evals/judge/out/hard-v1-candidate-v2-live`
- [x] Hard candidate v3 live run: `go run ./adc/runtime/cmd/adc eval judge-voir-dire --fixtures adc/evals/judge/rules/rule47/voir_dire_questions_hard_v1.jsonl --engine adc/.bin/adcengine --opportunity-prompt-file adc/evals/judge/rules/rule47/prompts/candidate-v3.md --opportunity-prompt-name candidate-v3 --out-dir adc/evals/judge/out/hard-v1-candidate-v3-live`

## 2026-07-14: `adc juror` probe command

### References

- New command: [`runtime/cli/juror.go`](runtime/cli/juror.go)
- Persona record loading: [`../common/persona/persona.go`](../common/persona/persona.go)
- Voir dire experiment plan: [`../experiments-1.md`](../experiments-1.md)

### Decisions

`adc juror` asks one juror pool member one question.  It loads a JSONL
pool (default `common/data/personas/pool.jsonl`), selects a member by
1-based index or unique substring (`--member`, `--list`), builds the
runtime juror system prompt from the member's persona, and sends one
prompt through the member's request spec.  `--repeat K` draws
independent samples (NDJSON output when K > 1), `--vote` reuses the
`adc llm` tool-check path to force a `submit_juror_vote` call, and
`--transcript FILE` gives file-based multi-turn: prior turns in the
NDJSON file are replayed as conversation history and the new exchange
is appended, with a member-mismatch check.  The command exists for the
iterative juror interrogation loop in the experiment plan.

Two defects observed during live testing, both outside the new code.
`--vote` against checked-in pool members fails at OpenRouter with 404
"no endpoints found that can handle the requested parameters": the
derived provider pin (for example `only=["novita/fp8"]` with
`quantizations=["fp8"]`) plus function tools matches no endpoint on
`/responses`.  The same record answers plain prompts.  An unpinned
record (`{"openrouter_model_id":"openai/gpt-4o-mini","persona":...}`)
completes `--vote` correctly, so the tool path itself works.  This
needs a decision, because direct-mode juror votes in `adc case` and
`adc scenario` use the same client with the same tools and may fail
for pinned members.  Separately, `adc llm --model endpoint://model`
without a persona record sends the literal `endpoint://model` string
as the model id and OpenRouter rejects it; the request-spec path
strips the prefix and works.

The `--model` defect is fixed (2026-07-14, approved): `llm.go` and the
plain-record branch of `juror.go` now pass the parsed model id from
`ParseModelRef` to `CreateResponse`, matching `Spec.UpstreamModel()`
behavior on the request-spec path.  Verified live with
`adc llm --model openrouter://openai/gpt-4o-mini`.

### Verification

- [x] `gofmt -l runtime/cli/` (clean)
- [x] `go vet ./runtime/cli/`
- [x] `go test ./runtime/cli/ -run 'TestSelectJurorMember|TestJurorTranscriptRoundTrip'`
- [x] Live: `--list`, single prompt, two-turn `--transcript` continuation against pool member 1 (deepseek-r1/novita)
- [x] Live: `--vote` against an unpinned gpt-4o-mini record
- [x] Live failure reproduced: `--vote` against pinned pool members returns OpenRouter 404

## 2026-07-13: Terminal state artifact

### References

- Runner output writer: [`runtime/runner/io.go`](runtime/runner/io.go)
- Replay certificate implementation: [`runtime/runner/certificate.go`](runtime/runner/certificate.go)
- Certificate verifier command: [`runtime/cli/verify_certificate.go`](runtime/cli/verify_certificate.go)
- Exhibit action boundary: [`engine/Main.lean`](engine/Main.lean)
- Service artifact route: [`runtime/service/service.go`](runtime/service/service.go)
- ADC manual output section: [`manual.md`](manual.md#output-artifacts)
- Certificate plan: [`../plan-2026-07-13-certificates.md`](../plan-2026-07-13-certificates.md)

### Decisions

ADC terminal packets now include `state.json` beside `run.json`.  The runner writes `state.json` from the same `Result.FinalState` value embedded in `run.json`, so certificate verification has a stable file boundary without reconstructing state from the result envelope.  The service artifact API lists and fetches `state.json` through the same allowlist path used for `run.json`, logs, transcripts, and manifests.

`offer_case_file_as_exhibit` now sends `file_id` and `offered_at` into the Lean `offer_exhibit` action.  Lean validates the file id when present and records the corresponding `file_events` entry.  The runner no longer appends that event after the engine returns, so the terminal state is reproducible from accepted engine transitions.

ADC terminal packets now include `certificate.json`.  The certificate stores the initial state, optional seeded complaint initialization, accepted `step` transitions, accepted pass decisions, the claimed final state, and a compact JSON SHA-256 hash of that final state.  `adc verify-certificate` reads `certificate.json` and `state.json`, replays the recorded transitions through the Lean engine, and requires the certificate hash, packet-state hash, and replayed-state hash to match.

The ADC proof layer now has accepted-certificate modules under `engine/Proofs/`.  `Reachability.lean` defines the typed certificate initialization and transition objects, including accepted `step` transitions and accepted `apply_decision` pass transitions.  `Replay.lean` proves exact replay and reachability from the replay start, `CertificateFacts.lean` packages closed-terminal facts, and `CertificateExamples.lean` checks a concrete closed certificate.  ADC `CourtState` contains JSON and `Float`, so the Lean accepted-certificate boundary is the proposition `replayCertificate init transitions = .ok claimed`; the executable equality check remains in `adc verify-certificate`.

`CertificateOutcomeFacts.lean` adds verdict, juror-failure verdict, juror-failure hung-jury, judgment, and outcome accounting predicates for accepted certificates.  `CertificateOutcomeExamples.lean` checks four concrete replayed outcome certificates: a `submit_juror_vote` transition that derives a plaintiff verdict under the configured concurrence threshold, a `process_juror_timeout` transition that removes a deliberating juror and derives a verdict under the effective concurrence rule, a `process_juror_timeout` transition that removes the last sworn juror and records a hung jury, and an `enter_judgment` transition that carries jury-verdict damages into `monetary_judgment` and records the judgment trace.  The `OutcomeCertificateFacts` package gives callers one citation point for closed, verdict, juror-failure verdict, juror-failure hung-jury, and judgment certificate facts without changing the exact replay boundary.

ADC failure accounting has two different boundaries.  Lawyer and nonjuror role failures stop the runner before `writeEvidence`, so the current runtime produces no `state.json` or `certificate.json` for those failures.  Juror failures during voir dire or deliberation enter the Lean state through accepted `process_juror_timeout` transitions, and deliberation failures may also derive a verdict or hung jury.  The certificate proof layer should treat those juror failures as ordinary replayed engine transitions; a failed-certificate Lean package should wait for a state-level ADC failure claim in the runtime.

The deliberating-juror timeout certificate package now records that boundary directly.  `JurorFailureVerdictCertificateFacts` combines exact accepted replay, replay-start reachability, a recorded `process_juror_timeout` transition for the juror, and a final verdict whose `required_votes` equals `effectiveMinimumConcurring` after the juror has been marked `timed_out`.  `JurorFailureHungJuryCertificateFacts` covers the corresponding hung-jury path, requiring the timeout transition, the timed-out juror record, no jury verdict, and a recorded `HungJury` value with a claim id and note.  The concrete samples cover both outcomes: J1 times out while J2 through J6 have plaintiff votes, producing a five-vote verdict; and J1 times out as the last sworn juror, producing a hung-jury record.

Importing the closed-case opportunity theorem into the certificate proof path exposed an obsolete theorem in `OrchestrationCore.lean`: it still claimed `assignOpportunityIds` returned sequential ids.  The engine now assigns deterministic hash ids from opportunity content.  The theorem now states the property the function actually supports at that level, length preservation.

### Verification

- [x] `go test ./adc/runtime/runner ./adc/runtime/service`
- [x] `go test ./adc/runtime/runner ./adc/runtime/service ./adc/runtime/cli`
- [x] `go test ./adc/runtime/...`
- [x] `lake build Proofs.RecentExhibitLimits`
- [x] `lake build Proofs.CertificateOutcomeFacts Proofs.CertificateOutcomeExamples`
- [x] `lake build Proofs adcengine`
- [x] Real no-LLM run with `adc scenario`, followed by `adc verify-certificate`

## 2026-07-10: Service process record reconciliation

### References

- Service process manager: [`runtime/service/service.go`](runtime/service/service.go)
- Service process tests: [`runtime/service/service_test.go`](runtime/service/service_test.go)

### Decisions

The service now gives child processes direct stdout and stderr log file descriptors instead of copying pipe output through the service process.  This lets the child continue writing logs if the service process exits, and it removes pipe-copy goroutines from the lifecycle path.  Completion still reads the stdout log after the child exits to populate the service summary.

Startup record loading now persists repaired case records.  Active or previously detached records are repaired from `run.json` when it appears; otherwise active records become failed with `service restarted and child process is not attached`.  The service does not reattach to a process after restart.

### Service API error cleanup

Artifact reads now distinguish names outside the allowlist from listed artifacts whose files are absent.  The first returns `unknown_artifact`, and the second returns `artifact_missing`.  Missing listed-artifact responses omit host filesystem paths.

### Verification

- [x] `go test ./adc/runtime/service`
- [x] `go test ./arb/runtime/... ./arbd/runtime/... ./adc/runtime/...`

## 2026-07-09: Live evidence manifest for service evidence fetch

### References

- Service evidence route: [`runtime/service/service.go`](runtime/service/service.go)
- Runner evidence output: [`runtime/runner/io.go`](runtime/runner/io.go)
- ADC manual service section: [`manual.md`](manual.md#service-api)

### Decisions

The ADC service evidence route now distinguishes an active missing manifest from a terminal packet that lacks a manifest.  Active cases return HTTP `409` with error code `evidence_manifest_pending`; terminal packets return HTTP `404` with error code `manifest_missing`; unreadable and malformed manifests return separate server errors.  The route accepts both the legacy array manifest and the object manifest with an `evidence` array.

The runner now writes `evidence-manifest.json` when it initializes and after case-file state changes.  The manifest uses an empty array when no files exist, avoiding the `null` shape that made service parsing fail in the ARB live test.  Case files are copied into `submitted-evidence/` with atomic replacement, so the service route can fetch a file by `evidence_id` during a live run after the corresponding manifest update.

### Verification

- [x] `go test ./runtime/runner`
- [x] `go test ./runtime/service`

## 2026-06-17: Manual review

### References

- ADC manual: [`manual.md`](manual.md)
- ADC Docker runbook: [`Dockerfile.md`](Dockerfile.md)
- ADC attested dev-host requirements: [`docs/attested-dev-host.md`](docs/attested-dev-host.md)

### Decisions

The manual distinguishes `adc case`, `adc scenario`, `adc run`, and `adc service` by the boundary under test.  It also records how to treat an output directory as the record for one case, with `events.ndjson`, `run.json`, `digest.md`, and `work-notes.ndjson` serving different review needs.  The command-selection table includes `adc case-packet`, because attested complaint runs depend on that deterministic input format.

The manual documents utility commands for `adc case-packet`, `adc validate`, `adc pool`, `adc pacer`, and `adc llm`.  The troubleshooting section names complaint setup and case-packet failures as input-resolution problems before runtime settings should be changed.  Attested Clerk troubleshooting points to the ADC Docker runbook, which contains the service-run and verification sequence.

### README review

The README uses the same documentation table as AAR, AARD, and evals.  It links to the manual, Docker runbook, dev-host requirements, attested run helper, practice guide, and rules.  The license references point to the repository-level files.

## 2026-06-17: Attested ADC complaint path

### References

- ADC Docker runbook: [`Dockerfile.md`](Dockerfile.md)
- ADC dev-host requirements: [`docs/attested-dev-host.md`](docs/attested-dev-host.md)
- Attested ADC driver: [`tools/run-adc-attested.py`](tools/run-adc-attested.py)
- Exec workload script: [`tools/run-adc.sh`](tools/run-adc.sh)
- Exec container entrypoint: [`attest/exec-container-entrypoint.sh`](attest/exec-container-entrypoint.sh)
- Clerk service attestation support: [`runtime/service/attested.go`](runtime/service/attested.go)
- Complaint packet builder: [`runtime/casepacket/case_packet.go`](runtime/casepacket/case_packet.go)

### Decisions

The first ADC attested path supports only `complaint_path`.  The driver packages the complaint and linked local files into `case.tar.gz` and `case-packet.json`, uploads those objects under `INPUT_PREFIX`, and requires verification before the clerk service can mark the case completed.  Scenario input and local runtime overrides remain rejected until each field has an explicit attested interpretation.

ADC uses its own base image and glue image.  The base image builds `.bin/adc`, embeds a Pi juror root filesystem, and includes the Docker CLI.  The glue image adds AWS CLI, `nitro-tpm-attest`, TSS libraries, and the ADC exec entrypoint.

The clerk service uses `execution.mode = "attested"` with an `execution.attestation` object.  It starts the Python driver, marks the service record running after the driver starts, exposes `/attestation/events`, and reads results, artifacts, and evidence from the extracted `adc-output/` directory after verification.  The service treats a zero driver exit without `verification.log` and readable `adc-output/run.json` as a failed service record.

ADC local OpenClaw Codex auth staging now matches the working AAR and AARD behavior.  The staged Codex home is mode `0777`, and `auth.json` is mode `0666`, because OpenClaw reads the mounted file as a container user that may not match the host user.  The attested run still excludes those staged auth directories from uploaded ADC archives.

ADC now also matches the AAR and AARD OpenClaw networking fix.  In the exec AMI topology, a child OpenClaw container using Docker bridge networking can import Codex auth and still fail a Codex request with `stream disconnected before completion` from `https://chatgpt.com/backend-api/codex/responses`.  The recorded AAR diagnosis showed the same failure on the same Docker-enabled exec AMI and showed that `--network host` fixed it.  ADC therefore supports `--openclaw-network host`, uses `127.0.0.1` as the Docker MCP host in that mode, omits the bridge-only `host.docker.internal` mapping, and passes `--openclaw-network host` from the attested exec entrypoint.

### Clerk test failure

The real Clerk attestation test `case_id=adc-real-ex1-20260617T040521Z` and `run_id=run-adc-real-ex1-20260617T040521Z` used exec instance `i-0966af392feb5b657` and S3 output prefix `s3://agentcourt-data/arbattest/adc-runs/run-adc-real-ex1-20260617T040521Z/`.  The run proved that the ADC auth and networking fixes were present in the running image: both OpenClaw lawyer containers started with host networking, imported Codex auth, filed pleadings, completed discovery, completed voir dire, tried the case, reached jury instructions, and entered deliberation.  The last S3 event record showed the third juror deliberating; the run then failed without `run.log`, `adc-partial.tar.gz`, `manifest.json`, `manifest.sha384`, `attestation.b64`, or `adc-output.tar.gz`.

The failure came from the generic `attest/exec.sh` console-output timeout.  The ADC legal runtime had not reached a terminal error.  `adc/tools/run-adc-attested.py` passed `POLL_ATTEMPTS=1800` to `exec.sh`; `exec.sh` sleeps two seconds per poll, so the generic launcher controls the instance for about 60 minutes before it exits 1 and terminates the instance.  The ADC driver had `--timeout-seconds 10800`, but the shorter generic launcher timeout killed the still-running exec instance before ADC could produce terminal S3 artifacts or a verifiable attestation.

The ADC driver now derives the default `POLL_ATTEMPTS` value from `--timeout-seconds` and `exec.sh`'s two-second poll period, with ten minutes of headroom.  Explicit `--exec-poll-attempts` and `POLL_ATTEMPTS` values still override that default.  The Clerk service default for `--attested-exec-poll-attempts` is now zero, so the service no longer passes the obsolete 1,800-attempt value unless the operator requests that value.  A shared launcher change may still be warranted so S3 terminal artifacts control long attested AAR, AARD, and ADC runs without a separate console-output timeout.

The next Clerk test attempt used `case_id=adc-real-ex1-20260617T133247Z` and fixed the service-level poll-attempt default, but it failed before ADC started because the fresh input prefix contained only `case.tar.gz` and `case-packet.json`.  The exec entrypoint requires `auth.json` and `keys.sh` in the same prefix.  Any fresh ADC attested Clerk test must stage those two secret objects to the selected input prefix before posting the create request.

ADC now has `tools/run-one-attested-adc.sh`, matching the working AAR and AARD one-run helpers at the operator layer.  The helper takes a complaint path, stages `/home/ec2-user/arbattest-secrets/auth.json` and `/home/ec2-user/arbattest-secrets/keys.sh` into the selected `adc-inputs` prefix through `dev`, and then runs `tools/run-adc-attested.py` with verification enabled.  This fixes the repeated operator mistake where ADC used a fresh input prefix without the two secret objects even though the runtime and exec entrypoint already handled OpenClaw Codex auth like AAR and AARD.

The real Clerk-managed attested ADC run `case_id=adc-real-ex1-20260617T133710Z` and `run_id=run-adc-real-ex1-20260617T133710Z` completed successfully.  The exec instance `i-061c6a07dc811676d` terminated after the driver downloaded terminal artifacts, and S3 output prefix `s3://agentcourt-data/arbattest/adc-runs/run-adc-real-ex1-20260617T133710Z/` contains `events.ndjson`, `run.log`, `adc-output.tar.gz`, `manifest.json`, `manifest.sha384`, and `attestation.b64`.  The Clerk service marked the case `completed`, exit code `0`, with attestation status `verified`.

Verification passed for `manifest.sha384`, run id, mode, input mode, input prefix, ADC case id, output prefix, archive key, run-log hash, archive hash, archive size, container image identity, case-packet keys and hashes, attestation signature, attestation user data, PCR 4, PCR 7, and PCR 12.  The final ADC state was `status=judgment_entered`, `phase=post_verdict`, `trial_mode=jury`.  The jury returned a plaintiff verdict with damages `108000`, five votes for the verdict, and required votes `5`; juror `J6` timed out during deliberation, and ADC handled that timeout under the existing effective-threshold rule.

The ADC runbook now documents the full Clerk service path as one sequence: choose fresh ids and prefixes, stage `auth.json` and `keys.sh` into the exact S3 input prefix, start `adc service` with attested defaults, post the create request, poll the case record and `/attestation/events`, inspect terminal artifacts, verify the attestation state, and stop the service.  The runbook also records the attestation troubleshooting map for the failures already diagnosed: missing staged secrets, expired or unreadable Codex auth, missing host networking for OpenClaw, an exec console polling limit shorter than the ADC runtime timeout, recursive S3 uploads, incomplete terminal artifacts, service output-directory validation, and PCR or manifest verification failures.  The README, manual, and dev-host requirements now point operators to that runbook for the Clerk-managed attested ADC process.

### Verification

- [x] `go test ./adc/runtime/casepacket`
- [x] `go test ./adc/runtime/service`
- [x] `go test ./adc/runtime/localrun`
- [x] `go test ./adc/runtime/...`
- [x] `sh -n adc/tools/run-adc.sh adc/tools/run-container-poc.sh adc/attest/exec-container-entrypoint.sh`
- [x] `python3 -m py_compile adc/tools/run-adc-attested.py`

## 2026-06-06: ADC practice guide rewrite

### References

- Practice guide: [`docs/practice.md`](docs/practice.md)
- Operating reference: [`manual.md`](manual.md)
- Lawyer instructions: [`agent-instructions/openclaw-lawyer.md.tmpl`](agent-instructions/openclaw-lawyer.md.tmpl)
- Juror instructions: [`agent-instructions/pi-juror.md.tmpl`](agent-instructions/pi-juror.md.tmpl)

### Decisions

The ADC practice guide now describes advocacy and evidence work rather than command syntax.  The command, API, MCP, service, clerk JSON, and output-artifact details stay in the manual, and the practice guide links to that manual as the operating reference.

The guide emphasizes case-file inspection, exhibit offers, technical reports, work notes, source search, browser work, OCR, local tools, source-chain analysis, and element-by-element arguments.  It also describes the current external-agent path: OpenClaw lawyers act through MCP over the Role API, and Pi jurors receive the trial transcript, instructions, visible case view, and case-file tools at deliberation.  A separate argument-writing section connects search and extraction work to motions, trial theory, exhibit use, closings, and post-judgment motions.

## 2026-06-06: Jury configuration inputs

### References

- Jury policy and deterministic clerk action: [`engine/Main.lean`](engine/Main.lean)
- Scenario policy defaults and runtime overrides: [`runtime/runner/state_init.go`](runtime/runner/state_init.go), [`runtime/runner/runner.go`](runtime/runner/runner.go)
- Direct command flags: [`runtime/cli/case.go`](runtime/cli/case.go), [`runtime/cli/run.go`](runtime/cli/run.go), [`runtime/cli/localrun.go`](runtime/cli/localrun.go)
- Clerk create request: [`runtime/service/service.go`](runtime/service/service.go)

### Decisions

Jury size and verdict threshold are ADC case-policy values.  The scenario policy keys are `jury_juror_count`, `jury_unanimous_required`, and `jury_minimum_concurring`, and the engine uses those values when the clerk sets jury configuration at trial setup.  The default policy remains a six-person unanimous jury with minimum concurring six.

Direct case commands expose the policy through `--juror-count`, `--unanimous-required`, and `--minimum-concurring`.  Complaint-based commands write the selected values into the generated scenario.  Scenario-based commands apply the same values as startup overrides, leaving the scenario file unchanged.

The clerk service exposes the same configuration through `juror_count`, `unanimous_required`, and `minimum_concurring` in the create-request JSON.  Those fields apply to local-agent children and direct children.  The service checks simple numeric ranges before it starts a child process, and the Lean action validation remains the final rule check.

## 2026-06-06: Failed deliberating jurors

### References

- Verdict derivation: [`engine/Main.lean`](engine/Main.lean)
- Lean sample proofs: [`engine/Proofs/RecentVerdictDerivation.lean`](engine/Proofs/RecentVerdictDerivation.lean)
- Juror timeout handling: [`runtime/runner/timeouts.go`](runtime/runner/timeouts.go)
- Timeout tests: [`runtime/runner/timeouts_test.go`](runtime/runner/timeouts_test.go)

### Decisions

ADC now treats failed deliberating jurors as removed from the effective concurrence threshold.  The configured jury size and nominal concurrence rule remain in the case configuration, but verdict derivation caps the required votes at the number of sworn jurors still eligible to vote.  If no sworn jurors remain, the case records a hung jury because no verdict can be formed.

This rule matches the operational goal for agent trials.  A single Pi model failure should not turn five matching votes into a hung jury.  Disagreement among the remaining jurors can still produce additional deliberation rounds or a hung jury under the existing split-vote rules.

### Plan

- [x] Add an effective concurrence helper in the Lean engine.
- [x] Add a Lean sample proof for a five-vote verdict after one failed juror.
- [x] Update the Go timeout test to expect a verdict from the remaining jurors.

## 2026-06-05: Pi juror opportunity lifetime

### References

- Local ADC full-run process management: [`runtime/localrun/localrun.go`](runtime/localrun/localrun.go)
- Pi juror instructions: [`agent-instructions/pi-juror.md.tmpl`](agent-instructions/pi-juror.md.tmpl)
- Juror deliberation prompt: [`runtime/runner/juror_prompt.go`](runtime/runner/juror_prompt.go)

### Decisions

A Pi juror process handles one active juror opportunity and then stops.  ADC starts a fresh Pi process if the same juror later receives another opportunity.  The case process owns waiting, so juror agents do not stay alive through non-juror trial phases.

A fresh deliberation process gets the deliberation prompt, which includes the trial transcript from openings through closings, jury instructions, and guidance to inspect admitted exhibits and visible case files through MCP.  Voir dire memory is not carried into deliberation through the Pi process home.  The case record remains the source of juror status and trial evidence.

### Plan

- [x] Tie Pi juror process names and homes to the active opportunity id.
- [x] Stop juror processes whose opportunity is no longer active.
- [x] Change Pi juror instructions to stop after a successful submission.
- [x] Strengthen the deliberation prompt's trial transcript and evidence review guidance.
- [x] Run focused ADC tests and build.

## 2026-06-04: Role API and MCP path

### References

- ADC update plan: [`../scratch/adc/update-plan.md`](../scratch/adc/update-plan.md)
- Role API: [`runtime/runner/roleapi.go`](runtime/runner/roleapi.go)
- MCP adapter: [`runtime/mcp/mcp.go`](runtime/mcp/mcp.go)
- Clerk service: [`runtime/service/service.go`](runtime/service/service.go)
- Local run command: [`runtime/localrun/localrun.go`](runtime/localrun/localrun.go)

### Decisions

ADC now exposes external plaintiff, defendant, and juror opportunities through a case-owned role API.  The runner remains the case owner because it holds the Lean state, opportunity deadlines, invalid-attempt counts, case-file visibility, and final result generation.  Complaint planning and scenario construction remain internal direct-model work before the live case starts.

The ADC MCP adapter is a thin HTTP adapter.  Each MCP session binds to a case id, role id, and optional juror principal id, then forwards tool calls to the role API.  The MCP tool set is stable; each active opportunity describes which legal tools Lean permits at that point.

ADC removed the active ACP and xproxy paths.  Juror pools now use JSONL request-spec records.  Those records preserve the model request configuration needed by Pi agents, including endpoint and request parameters.

`adc run` is now the operator-level local command.  It accepts either `--complaint` or `--scenario`, starts the case API and MCP in-process, starts OpenClaw lawyers according to `--auto-lawyers`, and starts Pi jurors when a juror first appears.  The former direct scenario command is now `adc scenario`.

Entries below this section are historical development notes.  ACP and xproxy entries record the old path and do not describe the current ADC runtime.

### Plan

- [x] Record the agreed ADC update plan.
- [x] Add the role API at the Lean opportunity boundary.
- [x] Add work notes and remote case-file reads through the role API.
- [x] Add a thin MCP adapter over the role API.
- [x] Add a clerk service that starts case processes and proxies role API calls.
- [x] Remove active ACP and xproxy runtime paths.
- [x] Make `adc run` the local OpenClaw/Pi command.

## 2026-05-20: ACP role portability

### References

- ACP role runtime: `runtime/runner/acp_role.go`
- PI-home staging: `runtime/runner/pi_container_home.go`
- Agent documentation: [`docs/agents.md`](docs/agents.md)
- Porting inventory: [`../scratch/adc/update.md`](../scratch/adc/update.md)

### Decisions

`adc` now accepts remote ACP endpoints for delegated roles.  The `case` and `run` commands use `--acp-endpoint`, while `acp-role` uses `--endpoint`; each path rejects simultaneous command and endpoint configuration.  Endpoint roles keep model selection and native tool availability outside ADC, while ADC still provides `_adc/*` methods and Lean validation.

Local wrapper-backed ACP roles now use typed ADC role configuration.  The staged PI settings default model comes from the effective role model, normalized to an xproxy model identifier, while `ADC_FLASH_XPROXY_MODEL` remains a compatibility override.  The staged PI home also receives role instructions and a private `/home/user/work-product/` directory, and the runner exports that directory beside `run.json` after the run.

The ACP prompt now names current limits instead of relying only on static documentation.  It includes host methods, support-method budget, decision budget, invalid-submission budget, visible file count, and party exhibit and report counts when the role is a party.  ACP decision rejections now keep an ordered history and fail with that history after the configured invalid-attempt limit.

### Plan

- [x] Add shared TCP endpoint configuration to `case`, `run`, and `acp-role`.
- [x] Stage role model and standing instructions into local PI homes.
- [x] Create and export ACP role work product.
- [x] Add dynamic capability and limit text to ACP opportunity prompts.
- [x] Preserve ordered invalid-decision history for ACP roles.
- [x] Stop tracking generated `.sig.b64` example artifacts.

## 2026-04-03: `adc acp` wrapper staging

### References

- ACP CLI entrypoint: `runtime/cli/acp.go`
- PI-home staging path: `runtime/runner/pi_container_home.go`
- ACP role wrapper setup: `runtime/runner/acp_role.go`
- Podman ACP wrapper: [`../common/pi-container/acp-podman.sh`](../common/pi-container/acp-podman.sh)

### Decisions

- `adc acp` now stages the temporary PI home when the selected command is `acp-podman.sh` or `pi-podman.sh`.
- That staging path already existed for `acp-role`.  The direct `acp` subcommand had been defaulting to the same wrapper without setting `PI_CONTAINER_HOME_DIR`, so the wrapper exited before ACP initialization.
- When the wrapper is in use, `session/new` now uses `/home/user` instead of the host working directory.  That matches the wrapper mount and the existing `acp-role` behavior.

### Plan

- [x] Reuse the runner PI-home staging helper from `adc acp`.
- [x] Switch wrapper-backed sessions to `/home/user`.
- [x] Add a focused CLI test for wrapper staging.

## 2026-04-03: `adc acp` default wrapper path

### References

- ACP CLI entrypoint: `runtime/cli/acp.go`
- CLI helper defaults: [`runtime/cli/helpers.go`](runtime/cli/helpers.go)

### Decisions

- `adc acp` now uses the repository-root relative wrapper path `common/pi-container/acp-podman.sh` as its default ACP command.
- The prior code joined the current working directory with a path that had already been resolved, producing duplicated prefixes such as `/repo/repo/common/...`.
- The ACP command default should stay simple.  If the user is not running from the repository root, the command should fail fast and require `--command` explicitly.

### Plan

- [x] Remove the extra path join in `runtime/cli/acp.go`.
- [x] Make `defaultACPServerPath` return the relative wrapper path directly.
- [ ] Add a dedicated test if this area changes again.

## 2026-03-18: `../evals/tools/cluster-personas.py`

### References

- Local xproxy model and config parsing: `runtime/xproxy/config.go`
- Local persona record parsing and prompt text: `runtime/persona/persona.go`
- Local xproxy startup and default port behavior: `runtime/cli/xproxy.go`
- OpenAI Python SDK Responses usage: https://github.com/openai/openai-python
- OpenAI embeddings API reference: https://platform.openai.com/docs/api-reference/embeddings/create

### Decisions

- The shared model and persona corpus now lives under `../common/data/personas/`.  The checked-in genes, sampled pools, cluster assignments, and PCA rows belong to the shared corpus, not to `adc/etc/`.
- The shared tools use the current working directory for their default paths.  Run them from the repository root unless you pass explicit paths.
- The Python tool talks to xproxy at `http://127.0.0.1:$PI_CONTAINER_XPROXY_PORT/v1`, with the same default port `18459` used in the Go code.
- The tool does not try to start xproxy.  The repository has no standalone xproxy CLI.  The Go commands start it internally for their own lifetimes.  The Python script instead checks `/healthz` and fails with a precise error if xproxy is absent.
- Persona records use the same `MODEL,FILE` parsing and the same juror persona prompt text as the Go runtime.
- Completions are sampled with repeated Responses API calls.  This is the direct path exposed by the current SDK usage here.  The task's "hopefully as multiple completions for one request" clause remains aspirational.
- Embeddings use the OpenAI Python SDK directly against the embeddings API.  The default embedding model is `text-embedding-3-small`, overridable with `PERSONA_SAMPLE_EMBEDDING_MODEL`.
- Embeddings run one sampled response at a time.  That avoids provider-side max-token failures on large batch requests and keeps one bad embedding response from aborting the whole run.
- PCA runs per gene over the full set of embeddings for that gene, matching the task.  When the requested PCA dimension exceeds what the sample count permits, the reduced vectors are zero-padded to keep the requested output dimension.
- The script writes cluster rows to stdout and writes per-sample PCA rows to `../common/data/personas/personas-pca.csv` by default.  Those rows are `model,persona_file,gene,x1,...,xN,cluster_num`.
- K-means cluster count is chosen per gene by maximizing silhouette score across the fixed range `3..10`.  If scoring is impossible or degenerate, all points fall into cluster `0`.

### Plan

- [x] Record the task and sources.
- [x] Add the standalone `uv` script.
- [x] Verify syntax and basic CLI behavior.

## 2026-03-18: `adc xproxy`

### References

- Root CLI dispatch and help text: [`runtime/cli/root.go`](runtime/cli/root.go)
- Existing xproxy helpers: `runtime/cli/xproxy.go`
- xproxy server entrypoint: `runtime/xproxy/xproxy.go`

### Decisions

- The new subcommand is `adc xproxy`.
- It resolves config and port the same way the rest of the CLI does: `--config` overrides `PI_CONTAINER_XPROXY_CONFIG` and `etc/xproxy.json`; `--port` overrides `PI_CONTAINER_XPROXY_PORT`.
- It starts xproxy directly through `xproxy.StartXProxyServer`, then waits for `SIGINT` or `SIGTERM` and closes the server cleanly.
- It fails fast if the target port already serves a healthy xproxy instance.

### Plan

- [x] Add root command dispatch and help wiring.
- [x] Add the server command implementation.
- [x] Verify help text and live `/healthz` behavior in tests.

### Results

- Live test: `uv run ../evals/tools/cluster-personas.py --personas-file /tmp/persona-sample-test.csv --genes-file /tmp/persona-sample-genes.json --num-samples 3 --gene-dim 3`
- Live output: three `MP,G,C` rows for one persona and one gene through local xproxy plus direct embeddings.
- Follow-up fix: `adc xproxy` initially returned an error on clean shutdown because the listener was already closed.  `runtime/xproxy/xproxy.go` now ignores `net.ErrClosed` in that path, and a live `Ctrl-C` shutdown now exits with status `0`.
