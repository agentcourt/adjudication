# Development Notes

## 2026-06-04

### Service and agent runtime migration

Reference: [AARD service and agent update plan](update-plan.md)

`arbd` now has the same current runtime shape as `arb`: a direct `aard` command with `case`, `mcp`, `service`, and `run` subcommands; a private case HTTP API; role-bound Lawyer API and Council API endpoints; an MCP adapter over those APIs; a Clerk service; and a local run path that starts OpenClaw lawyers and Pi council agents.  The implementation keeps AARD degree semantics.  Lawyers argue for numeric answers or answer ranges, council members call `submit_council_answer` with an integer from `0` through `100`, and final result data exposes the answer map rather than a binary resolution.

The Lean engine now represents opportunity failure directly.  A lawyer failure marks the case status as `failed` with a stored reason, while a council member failure marks that member as failed and continues deliberation with the remaining seated members.  The process-level API test started `aard case`, drove both lawyer roles through HTTP, submitted three council answers through the Council API, and produced a successful answer map.

- [x] Add `runtime/proceeding` with private Case API, Lawyer API, Council API, result reporting, work notes, and degree-specific council answers.
- [x] Add `runtime/mcp`, `runtime/service`, `runtime/localrun`, and `agent-instructions`.
- [x] Replace the old `runtime/cmd/aard` dispatcher with direct subcommands.
- [x] Update `Makefile` and `README.md` for the supported API and local-run path.
- [x] Run `go test ./...`, `lake build Proofs`, `lake build aardengine`, a direct case API process test, and a service API process test.

## 2026-05-02

### Initial fork from `arb`

`arbd/` began as a bounded fork of `arb/`, but it does not preserve the binary decision model.  The fork keeps the same high-level merits sequence and the same council-member machinery.  It changes the complaint from `Proposition` to `Question`, changes the policy field from standard-of-evidence framing to `judgment_standard`, and changes the deliberation act from a binary vote to one integer answer in `[0,100]`.

The final result is the answer set itself.  The runtime exports that result as a Go map keyed by `member_id`.  The engine does not compute a threshold outcome, an aggregate, or a `no_majority` result.  Closure now follows one condition: every seated council member has answered once in the round.

### Proof scope

The copied `arb` proof surface was wrong for `arbd`, because those files proved properties of threshold closure, substantive-outcome viability, and binary neutrality.  The new proof set is narrow and direct.  It proves initialization rules, ordered merits flow, bounded council answers, answer completeness on closure, and the current removal guards.

### Documentation boundary

This first version keeps the new tree small on purpose.  It has one example, one proof batch, and one Makefile demo path.  It avoids importing `arb`'s binary-vote notes and examples under misleading names.

### Example 1 refresh

The first example now uses two sonnets rather than the earlier placeholder novelty prompt.  The record states that one sonnet was written in 2024 and that a very similar but not identical sonnet was written in 2025.  That makes the example fit the degree model directly, because the council has to answer how much of the later text was really the earlier text.

### Council answer transport

The first live council runs exposed a transport mismatch with the shared council pool, not a Lean defect.  `arb` asks council models for a string-valued tool argument, while the first `arbd` draft asked the same mixed pool for a JSON integer.  That difference was enough to trigger repeated invalid council submissions under the normal `make demo` path.

The current Council API accepts a numeric `answer` for `submit_council_answer` and stores the value as an integer before it calls the Lean engine.  `aard run` now gives Pi council agents full request-spec model configs and a degree-specific MCP instruction.  The engine and final run evidence still store numeric answers.

### Documentation set

`arbd/docs/` now mirrors the core non-proof `arb/docs/` set with procedure-specific replacements: `ARAP.md`, `councils.md`, `goals.md`, `params.md`, and `practice.md`.  The text stays close to the working implementation rather than speculating about later aggregation or convergence designs.

The proof-oriented `arb/docs/` files were omitted on purpose.  `arbd` has a smaller proof surface, and the user asked for the procedural and practical documents first.  The documentation review pass focused on direct statement, explicit procedure description, and removal of binary-outcome phrasing that did not fit the degree model.

### Example 2

`arbd/examples/ex2/` now follows the same narrow pattern as `ex1`, but with two short stories instead of two sonnets.  The 2024 story, `first-story.md`, describes a near-future city whose civic AI assigns small mercies.  The 2025 story, `second-story.md`, tracks the same plot, scene order, and motifs with paraphrastic substitutions and relabeled set-pieces.

`arbd/Makefile` now has an `ex2` target that prints the supported local command path.  The live path is `aard run ex2`, because `aard case` waits for Lawyer API and Council API clients.  Earlier live runs on this example produced a high answer spread consistent with its close textual overlap.

### Example 3

`arbd/examples/ex3/` reuses the same 2024 base story as `ex2`, but pairs it with a 2025 story that is only loosely related.  The second story keeps some of the same named places and people, along with the same near-future municipal-AI setting, but changes the conflict, the central mechanism, the scene sequence, and most of the phrasing.  The point was to give `arbd` a case where common world-building and cast should not by themselves force a high score.

`arbd/Makefile` now has an `ex3` target as well.  The live path is `aard run ex3`, because the direct case process waits for external clients.  Earlier live runs on this example produced lower answers than `ex2`, which fits the intended design of the example.

### Evidence fidelity and explicit file filtering

The first review pass found two runtime issues worth fixing.  First, the exported `council.json`, `run.json`, digest, and transcript used the initially sampled council list even after the Lean state had marked a member `timed_out` or otherwise removed.  That made the packet misleading in exactly the cases where a reader most needs the status history.

`arbd` now derives the exported council list from the final Lean state.  That keeps the packet aligned with the source of truth and carries each member's final `status` into JSON and the rendered markdown reports.  The runtime still keeps the sampled council list in memory for persona text during live execution.

The same review found that `--file .gitignore` slipped past `validateExplicitCaseFilePath`, because `filepath.Ext(".gitignore")` is empty.  The validator now rejects `.gitignore` by basename before it checks ordinary extensions.  That change affects only the explicit `--file` path, which is where the gap existed.

## 2026-05-04

### Flexible complaint input

Reference: [Complaint parser](runtime/spec/complaint.go)

The degree runtime needs one question string.  The source file no longer has to
carry a literal `# Question` heading for the parser to produce that value.  When
a `Question` section exists, the parser uses that section.  When no such section
exists, the parser treats the whole trimmed file as the question.

The canonical writer still emits a `# Question` heading.  That keeps generated
complaint packets stable and readable.  Empty input fails, and an explicit empty
`Question` section fails, because either case lacks a question.

- [x] Preserve canonical complaint output.
- [x] Accept plain text as complaint input.
- [x] Reject blank complaints and blank explicit sections.
- [x] Cover parser behavior in tests.
