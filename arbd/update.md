# ARBD Porting Inventory

This inventory records which recent `arb` improvements should move into `arbd`.  `arbd` is closer to `arb` than `adc` is, so the answer is narrower than the ADC inventory.  ACP staging, attorney instructions, endpoint flags, work-product export, invalid-attempt feedback, target filing lengths, plain complaint input, and explicit-file filtering already exist in `arbd`, but several details still lag behind the current arbitration runtime.

## Implementation Status

The 2026-05-20 update completed the immediate runtime and documentation ports for remote endpoint model ownership, submitted source evidence, and OpenClaw degree attorneys.  `arbd` now rejects role-specific model overrides for remote ACP endpoints, stores submitted evidence with provenance, exposes `aar_submit_evidence` during arguments and claimant rebuttal, renders submitted evidence in run reports, and documents the behavior.  It also includes `aard-openclaw-attorney` and a TCP bridge for OpenClaw-backed endpoint runs.  Degree merits graphs and the larger proof architecture remain future work.

## Inventory Basis

The review compared recent `arb` changes with the `arbd` runtime, engine, prompts, proofs, examples, and documentation.  The relevant `arb` improvements include remote ACP endpoint semantics, submitted source evidence, OpenClaw attorney support, merits graphs, expanded examples, improved runtime tests, submitted-evidence rendering, and the larger Lean proof program.  The comparison treated `arbd` as a degree-question procedure, not as a binary-outcome arbitration with different labels.

`arbd` already has a working degree model.  It keeps the `arb` merits sequence, but the complaint states a question, the policy supplies a judgment standard, and the council returns one integer answer from `0` through `100` for each seated member.  The final result is the answer map, so binary-vote thresholds, substantive resolution labels, and no-majority behavior should not be copied into `arbd`.

## Already Present

| Area | Current `arbd` status | Notes |
| --- | --- | --- |
| ACP role execution | Present in `arbd/runtime/runner/acp.go`. | Attorneys use persistent ACP sessions, visible case-file tools, decision submission, transcript events, byte limits, and host feedback. |
| PI wrapper staging | Present in `arbd/runtime/runner/pi_container_home.go`. | The staging code matches `arb`, including model catalog edits, instructions staging, auth stub creation, and `AGENTCOURT_PI_XPROXY_BASE_URL`. |
| Attorney instructions | Present in `arbd/attorney-instructions/default.md` and CLI resolution. | The default instructions mention `/home/user/work-product/case-notes.md`. |
| Work-product export | Present in `arbd/runtime/runner/render.go`. | The runner exports per-role work-product directories into the output packet. |
| Invalid-attempt feedback | Present in `arbd/runtime/runner/invalid_attempts.go`. | The formatter preserves attempt history and gives corrective feedback for length, overflow, bad file ids, missing fields, and forbidden tools. |
| Plain complaint input | Present in `arbd/runtime/spec/complaint.go`. | `aard complain` and `aard validate` accept either a `Question` section or a whole-file question. |
| Degree examples | Present in `arbd/examples/ex1` through `ex3`. | The examples fit the degree model and should remain separate from `arb` examples. |

## Immediate Ports

| Item | Source in `arb` | Current `arbd` position | Recommendation |
| --- | --- | --- | --- |
| Remote endpoint model ownership | `arb/runtime/runner/attorneys.go` and `arb/runtime/cli/case.go` reject role-specific model flags when an ACP endpoint is set. | `arbd` accepts `--plaintiff-attorney-model` with `--plaintiff-acp-endpoint`, parses the local model anyway, and records local search metadata for a remote endpoint. | Port the `arb` endpoint semantics.  A remote ACP endpoint owns model selection and tool availability, so `arbd` should reject role-specific model overrides with endpoints and avoid claiming local search capability for remote attorneys. |
| Submitted source evidence | `arb/engine/Main.lean`, `arb/runtime/runner/acp.go`, `arb/runtime/runner/policy.go`, and `arb/runtime/runner/render.go` define `submitted_evidence`. | `arbd` now has the same source-evidence category, engine action, provenance metadata, and stored bytes. | Keep source material in submitted evidence and attorney analysis in technical reports. |
| Submitted-evidence policy limits | `arb/etc/policy.json` and `arb/runtime/runner/policy.go` add `max_submitted_evidence_per_side` and `max_submitted_evidence_bytes`. | `arbd/etc/policy.json` and `arbd/runtime/runner/policy.go` do not carry those limits. | Add the two policy fields, Go validation, Lean validation, and state-map persistence.  Use the same defaults unless a degree-specific reason requires different limits. |
| ACP evidence tool | `arb` exposes `_aar/submit_evidence` as `aar_submit_evidence` during arguments and rebuttals. | `arbd` now exposes the same source-evidence submission path alongside evidence listing, bounded range reads, materialization, and chunked upload. | Keep the `_aar/` method prefix until the whole `arbd` ACP namespace is renamed.  Accepted bytes belong in `evidence-store/`, and the accepted item returns an `evidence_id` for later `offered_evidence` citations. |
| Prompt and limit text for evidence | `arb` prompts distinguish source evidence, exhibits, and technical reports. | `arbd` now gives the same guidance in the common, argument, and rebuttal prompts. | Attorneys should submit source content and provenance first, then cite the returned `evidence_id` in `offered_evidence` when the material should be offered as an exhibit. |
| Rendering and council view | `arb` renders submitted evidence in transcript, digest, and council record views. | `arbd` renders exhibits and technical reports only. | Add submitted-evidence sections to the transcript, digest, attorney view, and council prompt.  The council should be able to distinguish source material from attorney analysis. |
| Runtime tests | `arb/runtime/runner`, `arb/runtime/cli`, and `arb/runtime/openclawattorney` have broader tests for model parsing, endpoint conflicts, PI staging, prompt content, evidence submission, work-product export, and invalid feedback. | `arbd` has a small runtime test set focused on policy, council answer normalization, render summaries, case summary JSON, file filtering, and complaint parsing. | Port the applicable tests before or alongside behavior changes.  The immediate targets are endpoint/model conflict tests, PI staging tests, prompt capability tests, prompt evidence-limit tests, work-product export tests, invalid-feedback tests, and submitted-evidence tests. |
| Documentation | `arb/docs/openclaw-attorneys.md`, `arb/docs/merits-graphs.md`, `arb/docs/params.md`, and `arb/docs/verification.md` document new behavior. | `arbd/docs` mirrors the older non-proof set and does not mention endpoints, submitted evidence, or OpenClaw. | Update `arbd/README.md`, `arbd/docs/ARAP.md`, `arbd/docs/params.md`, and `arbd/docs/practice.md` after implementation.  Add a separate OpenClaw note only after an `arbd` adapter exists. |

## Design Candidates

### OpenClaw Degree Attorney Adapter

`arb` added a stdio OpenClaw attorney adapter and a TCP bridge.  `arbd` can reuse the transport pattern, but the adapter must speak the degree procedure.  The prompt should present the question and judgment standard, not a proposition and evidence standard, and the filing guidance should ask for a concrete score or range when that helps the party's theory.

The adapter should wait until submitted evidence exists in `arbd`.  Open-record degree cases need the same distinction that `arb` now enforces: source material goes through `aar_submit_evidence`, and attorney analysis goes through `technical_reports`.  After that, an `aard-openclaw-attorney` command can adapt the current `arb` adapter with degree-specific case text, schema examples, and documentation.

### Degree Merits Graphs

`arb` added a merits graph workflow for theories, contentions, record support, and resolution.  `arbd` should not copy that graph unchanged, because a degree case does not end with `demonstrated`, `not_demonstrated`, or `no_majority`.  The graph should represent a quantitative argument: advocated score, anchors, factors that push the score up or down, exhibits, technical reports, submitted evidence, council answers, and rationale clusters.

This should remain a schema design until the core evidence work is complete.  A useful degree graph would help explain why the record supports `75` rather than `55` or `90`.  It should derive from the run packet, not from extra narrative input.

### Proof Architecture

`arbd` has a small proof set: initialization, merits flow, deliberation closure, answer bounds, answer completeness, and removal guards.  `arb` now has a larger global proof program covering reachability, frame preservation, material limits, record provenance, council integrity, no-stuck live states, bounded termination, progress, viable outcomes, and neutrality.  The degree fork should import the architecture, not the binary-outcome theorems.

The first proof ports should follow the runtime evidence work.  Add append-only and provenance lemmas for submitted evidence, offered files, technical reports, filings, and council answers.  Then add reachability and progress results for the fixed merits sequence and the degree deliberation rule: every live state either has a next opportunity or has closed after every seated member answered.

Outcome-soundness and neutrality need degree-specific statements.  The target is answer-map soundness, bounded answer values, and preservation of each council member's recorded rationale.  Strict-majority and no-majority proofs belong to `arb`, because `arbd` intentionally omits threshold closure.

### Open-Record Degree Examples

`arb` added examples four through six, including search-oriented cases.  Those examples should not be copied into `arbd`, because their propositions and remedies are binary arbitration cases.  `arbd` should add its own open-record degree examples after submitted evidence works.

Good candidates would ask for a percentage, share, similarity score, confidence level, or other bounded quantity where public source material can matter.  The examples should demonstrate source-evidence submission, technical reports, and final answer spread.  They should avoid turning the degree forum back into a yes-or-no liability decision.

## Keep in `arb`

Binary-vote policy fields should stay in `arb`: `required_votes_for_decision`, `max_deliberation_rounds`, binary `resolution`, `no_majority`, and strict-majority threshold checks.  `arbd` closes on completed degree answers and returns the answer map.  Copying threshold closure would change the purpose of the fork.

Binary outcome proofs should also stay in `arb`.  `ViableOutcomes`, `OutcomeSoundness`, and `Neutrality` can inspire degree-specific proof names, but their statements depend on binary council votes and outcome labels.  `arbd` needs proofs about answer validity, answer completeness, removal guards, and record preservation.

The `arb` examples, `arbitrate.sh`, and OpenClaw documentation should not be copied as files.  `arbd` needs degree-specific examples, a degree-specific convenience script if one becomes useful, and adapter documentation that describes `aard` behavior.  The current `arb` files would describe the wrong procedure.

## Proposed Order

1. Fix remote ACP endpoint model ownership and capability reporting.
2. Port the missing runtime tests for endpoint conflicts, PI staging, prompt capability, work-product export, and invalid-attempt feedback.
3. Add submitted-evidence policy fields, Lean state, Lean action validation, Go metadata types, byte storage, visible case-file conversion, and event recording.
4. Update ACP tools, prompts, transcript, digest, attorney view, and council view for submitted evidence.
5. Update `arbd/README.md`, `arbd/docs/ARAP.md`, `arbd/docs/params.md`, and `arbd/docs/practice.md`.
6. Add OpenClaw degree attorney support after submitted evidence is stable.
7. Design degree merits graphs from score arguments, evidence, reports, and council answers.
8. Expand the proof program in stages: append-only and provenance lemmas, reachability, no-stuck live states, progress, bounded termination, and answer-map soundness.
