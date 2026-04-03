# Verification Notes

## Current state

The current Lean proof library already establishes several global facts about Agent Arbitration.  The published [theorem index](theorems.md) and [proof statistics](proofstats.md) show the current surface in detail.

Ten facts matter most:

1. Every reachable state preserves the merits-phase structure.
2. Every reachable state preserves procedural parity between claimant and respondent.
3. Every reachable state respects the cumulative exhibit and technical-report limits.
4. Every run that begins at successful initialization preserves the case frame: proposition, policy, and council member identities.
5. Every reachable closed outcome is sound with respect to the recorded deliberation state: `demonstrated` and `not_demonstrated` meet the configured threshold, and `no_majority` closes only under the executable threshold-failure conditions.
6. Every reachable non-closed state has a next opportunity.
7. Every reachable state preserves record provenance, and every successful public run changes admitted exhibits and reports only by appending new entries.
8. Every successful public run from initialization is finite, with an explicit upper bound derived from the initialized policy and phase structure.
9. Seated council membership can only shrink along successful public runs, and any newly introduced council vote comes from a member seated in the source state.
10. If one flips every current-round vote between `demonstrated` and `not_demonstrated`, `currentResolution?` flips the same way.

Those theorems are important.  They show that the engine does not drift away from the intended procedural sequence, does not give one side extra merits turns, does not allow the admitted record to grow past the configured caps, does not switch propositions, policies, or council identities mid-run, does not publish a closed result that outruns the stored vote record, does not strand a reachable live case without a next step, does not let admitted materials appear from an unauthorized phase or disappear from the record later, cannot admit an infinite successful run, does not restore a removed council member to `seated` or admit a new vote from a non-seated member, and applies the two substantive outcomes symmetrically once the current round is fixed.

The initial public verification agenda is therefore complete.

## Further candidate theorems

| Theorem | Meaning | Why it matters | Likely proof shape |
|---|---|---|---|
| Deliberation reachability wrappers | Lift the raw neutrality theorem to more specialized operational statements about live deliberation states, remaining eligible members, or completed rounds. | The current theorem already covers reachable public states.  More specialized wrappers may help later quantitative proofs. | Reuse `Proofs/Neutrality.lean` together with the council-integrity and bounded-termination invariants. |

## Next proof work

The current proof library now covers phase fairness, side-level material limits, case immutability, outcome soundness, no-stuck liveness, record provenance, bounded termination, the monotonic council-status rules, and the symmetry of the aggregation rule under a vote flip.  The next work should only begin once there is a precise claim worth publishing.

## What Lean can and cannot prove here

Lean can prove procedural guarantees about the execution engine.  It can prove what follows from the state machine, the public step boundary, and the stored record.

Lean cannot prove that an advocate was honest, that a web search found the right source, or that a council member exercised good judgment.  It cannot prove the truth of the proposition.  It cannot prove that the stated standard of evidence was applied wisely, because the standard is policy text, not a formal semantics.

That limitation does not weaken the verification project.  It defines it.  The right goal is to prove that the engine preserves the intended procedure, that the published record has the integrity the procedure claims, and that the final outcome follows from the recorded votes according to the stated rule.

## Design implications

Further theorems may call for modest changes to the Lean code.

A future deliberation-summary layer could still help.  Vote counts, remaining eligible members, rounds used, and threshold attainability are now proved indirectly through the executable definitions and the reachable-state invariants.  An explicit summary layer would make later quantitative proofs shorter and clearer.
