# Parameter Plan

This plan separates three classes of input that the current code mixes together: procedural policy, complaint content, and runtime limits.  Procedural policy determines what the arbitration procedure allows and when it closes.  Complaint content states the proposition for one case.  Runtime limits constrain the infrastructure that runs the case, not the legal procedure itself.

The immediate goal is to remove duplicated hard-coded values and make the configurable surface explicit.  Today, text limits and deliberation-round limits are hard-coded in both [the Lean engine](../engine/Main.lean) and [the Go runner](../runtime/runner/run.go).  Timeouts live only in [the case CLI](../runtime/cli/case.go).  The required vote threshold is not configurable at all.  The plan below gives each class of parameter one home and one enforcement path.

## Current status

The first implementation slice is complete.  [The case CLI](../runtime/cli/case.go) now loads a policy file, defaulting to [`etc/policy.json`](../etc/policy.json) in the current working directory when that file exists.  That policy now includes `evidence_standard` as well as the structural limits.  [The Go runner](../runtime/runner/policy.go) validates both procedural policy and runtime limits before case initialization.  [The Lean engine](../engine/Main.lean) now carries `required_votes_for_decision`, `max_surrebuttal_chars`, and cumulative per-side exhibit and report caps.  [The runner](../runtime/runner/acp.go) and [the council path](../runtime/runner/council.go) enforce runtime invalid-attempt and response-size limits.  [The output artifacts](../runtime/runner/render.go) now include `policy.json` and `runtime.json`.

One part remains open.  The proof-friendly policy shape is now in place, but the Lean theorems over it are not.  The shared per-side fields make those theorems straightforward to state.  They have not yet been written.

## Parameter groups

| Group | Parameter | Purpose | Home | Primary enforcement |
|---|---|---|---|---|
| Procedure | `council_size` | Number of council members seated for the case | policy | Go at startup, Lean in state |
| Procedure | `evidence_standard` | The burden the council applies in this case | policy | Go at startup, Lean in state, prompts |
| Procedure | `required_votes_for_decision` | Votes needed to resolve the case in one direction | policy | Lean |
| Procedure | `max_deliberation_rounds` | Upper bound on deliberation rounds before `no_majority` | policy | Lean |
| Procedure | `max_opening_chars` | Opening text limit | policy | Lean |
| Procedure | `max_argument_chars` | Argument text limit | policy | Lean |
| Procedure | `max_rebuttal_chars` | Rebuttal text limit | policy | Lean |
| Procedure | `max_surrebuttal_chars` | Surrebuttal text limit | policy | Lean |
| Procedure | `max_closing_chars` | Closing text limit | policy | Lean |
| Procedure | `max_exhibits_per_filing` | Maximum offered files in one filing | policy | Lean and Go |
| Procedure | `max_exhibits_per_side` | Maximum offered files by one side across the whole case | policy | Lean |
| Procedure | `max_exhibit_bytes` | Maximum bytes for one offered file | policy | Go before submission |
| Procedure | `max_reports_per_filing` | Maximum technical reports in one filing | policy | Lean and Go |
| Procedure | `max_reports_per_side` | Maximum technical reports by one side across the whole case | policy | Lean |
| Procedure | `max_report_title_bytes` | Maximum title size for one report | policy | Lean and Go |
| Procedure | `max_report_summary_bytes` | Maximum summary size for one report | policy | Lean and Go |
| Complaint | `proposition` | The disputed proposition | complaint | complaint parser |
| Runtime | `council_llm_timeout_seconds` | Timeout for council turns | runner config | Go |
| Runtime | `attorney_acp_timeout_seconds` | Timeout for attorney ACP turns | runner config | Go |
| Runtime | `max_response_bytes` | Maximum raw model response size accepted from a turn | runner config | Go |
| Runtime | `invalid_attempt_limit` | Maximum invalid attempts before a turn fails | runner config | Go |

`evidence_standard` does not belong in the complaint.  It is a case parameter no different in kind from `council_size` or `required_votes_for_decision`.  The complaint should state only the disputed proposition.  The policy or case configuration should supply the burden the council applies.

## Majority rule

The vote threshold should be explicit.  The policy should therefore carry both `council_size` and `required_votes_for_decision`.  That removes the current hard-coded simple-majority rule and replaces it with a direct threshold that can express `3 of 5`, `4 of 5`, `5 of 7`, or unanimity without changing the engine.

Startup validation should enforce the basic arithmetic bounds: `council_size > 0`, `required_votes_for_decision > 0`, `required_votes_for_decision <= council_size`, and `2 * required_votes_for_decision > council_size`.  The last condition rules out dual-threshold states in which both substantive outcomes simultaneously satisfy the configured threshold.  The default should remain the current simple-majority behavior.  For a five-member council, that means `required_votes_for_decision = 3`.

## Proof-friendly policy shape

Where the procedure intends symmetry, the policy should express that symmetry directly.  For example, if each side should have the same cumulative exhibit cap, the policy should contain one field, `max_exhibits_per_side`, not separate plaintiff and defendant fields.  The same rule applies to technical reports.  One shared parameter gives Lean a cleaner object to reason about and removes accidental inequality from the configuration shape.

The same principle applies to vote thresholds.  A single `required_votes_for_decision` field is better than a computed majority rule hidden in code, but it is also better than side-specific decision thresholds that the procedure does not need.  The policy should state the common limit once.  Theorems can then quantify over one field and prove the same bound for both sides.

This matters for cumulative limits.  Per-filing limits and per-side limits solve different problems.  `max_exhibits_per_filing` prevents one oversized submission.  `max_exhibits_per_side` constrains the whole case record attributable to one side.  If later work needs proofs such as “each side gets the same maximum number of exhibits” or “no side can exceed the configured report cap,” those proofs are simpler if the state counts filings by side and compares them to one shared policy field.

## Enforcement split

Lean should continue to enforce procedural rules that affect the legal state: filing phase, text limits, vote thresholds, round limits, and counts of exhibits or reports.  Go should enforce byte-based limits and transport limits before material reaches the engine.  A file-size limit is about what the runner will carry and persist.  A phase rule is about what the procedure allows.  They are different constraints and should stay in different layers.

This split also determines persistence.  Policy values that affect the legal case should be written into the arbitration state and therefore into artifacts such as `run.json`, `state.json`, and the event log.  Runtime limits should stay in runner config and, if we want them recorded, they should appear in run metadata rather than in the legal state.

## Configuration surface

The main configuration surface should be one policy file, not a long list of unrelated CLI flags.  A single `--policy FILE` argument is enough for procedural policy.  The existing CLI can keep a small number of operational flags such as timeout values and output paths, plus narrow policy overrides such as `--council-size` and `--evidence-standard` when they are useful for testing.  That keeps the procedure readable, reduces duplicated defaults, and gives each run one concrete policy artifact that can be inspected later.

The policy file should contain only procedural parameters.  Complaint content stays in the complaint markdown.  Runtime limits stay in Go config.  That separation matters because the same complaint should be able to run under different procedural policies without rewriting the complaint, and the same policy should be able to run under different timeout settings without changing the legal state.

## Implementation order

1. Add a Go `Policy` type that mirrors the Lean `ArbitrationPolicy` and populate initial state from that type instead of duplicating literals in the runner.
2. Expand `ArbitrationPolicy` in Lean to include `required_votes_for_decision`, `max_surrebuttal_chars`, exhibit-count limits, and report-size limits.
3. Add Lean-side counting helpers for exhibits and reports by side, then enforce `max_exhibits_per_side` and `max_reports_per_side` in the submission path.
4. Change Lean deliberation resolution to use `required_votes_for_decision` instead of a computed simple majority.
5. Add Go-side startup validation for the policy file and reject impossible values before case initialization.
6. Add Go-side byte checks for exhibits and technical reports before attorney submissions reach the engine.
7. Add runtime-config fields for `council_llm_timeout_seconds`, `attorney_acp_timeout_seconds`, `max_response_bytes`, and `invalid_attempt_limit`, then thread them through the runner without storing them in legal state.
8. Add boundary tests for every new limit and every rejection path, then add basic Lean theorems over the symmetric caps.

## Defaults

The initial defaults should preserve current behavior unless a parameter needs to be split.  That means a five-member council, `Preponderance of the evidence.` as the default burden, three votes required for decision, three deliberation rounds, and the current text limits for openings, arguments, rebuttals, and closings.  `max_surrebuttal_chars` should start with the same value as `max_rebuttal_chars`, even though it becomes a distinct parameter.  The file and report limits should start conservative enough to prevent pathological payloads, but they should be chosen from observed example sizes rather than guessed.  The cumulative caps should also start symmetric: one exhibit cap per side and one report cap per side.
