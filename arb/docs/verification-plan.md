# Verification Plan

## Purpose

This plan turns the candidate verification goals in [verification.md](verification.md) into a sequence of proof tasks.  The order matters.  Some theorems are public results in their own right.  Some are prerequisite structure for the others.  The plan should record both.

The current proof library already establishes four global invariants:

1. reachable states preserve the merits-phase structure;
2. reachable states preserve procedural parity; and
3. reachable states respect the cumulative exhibit and report limits; and
4. runs that begin at successful initialization preserve the case frame:
   proposition, policy, and council member identities.

The next work should connect those structural facts to public claims about the final outcome, liveness, and record integrity.

## Work plan

| Stage | Theorem family | Status | Why this stage comes here |
|---|---|---|---|
| 1 | Case immutability after initialization | complete | This theorem is now proved.  It fixed the case frame first because later public theorems should be able to refer back to one proposition, one policy, and one council across the whole run. |
| 2 | Outcome soundness | complete | This theorem family is now proved.  It connects reachable closed outcomes to the recorded deliberation state and the executable closure conditions. |
| 3 | No stuck reachable state | complete | This theorem family is now proved.  It shows that every reachable non-closed state has a next opportunity, including live deliberation states. |
| 4 | Council vote integrity | complete | This theorem family is now complete.  It proves both the reachable integrity invariant for stored votes and the public status-monotonicity facts: seated membership can only shrink, and newly introduced votes come from seated members in the source state. |
| 5 | Record provenance and monotonicity | complete | This theorem family is now proved.  It shows both where admitted material may enter the record and that later successful steps do not rewrite prior admitted entries. |
| 6 | Bounded termination | complete | This theorem family is now proved.  It gives an explicit step budget for every initialized run and shows that every successful public step strictly decreases that budget. |
| 7 | Deliberation neutrality | complete | This theorem family is now proved.  The final proof uses the strict-majority validator, the reachable vote-integrity invariants, and the current-round seat bound. |

## Stage notes

### Stage 1: Case immutability after initialization

Target statement: after successful initialization, public `step` transitions never change the proposition, the policy, or the list of council member identities.  Statuses may change.  Votes and filings may accumulate.  The case being adjudicated and the governing policy remain fixed.

Planned proof shape:

1. Define a small predicate for the fixed case frame: proposition, policy, and council member identifiers.
2. Prove that successful initialization establishes that predicate.
3. Prove that every successful public step preserves it.
4. Lift the local theorem to runs that begin at a successful initialization.

Expected value:

This theorem says that the engine adjudicates one case under one policy.  It does not quietly switch issues or governing limits mid-run.  That is a basic trust property, and it will also be useful when later theorems refer back to the initialized case.

### Stage 2: Outcome soundness

Target statement: if a reachable case closes as `demonstrated`, the recorded council votes contain enough `demonstrated` votes to satisfy the configured threshold.  The same should hold for `not_demonstrated`.  If a reachable case closes as `no_majority`, then neither side reached the threshold, and the engine closed because the allowed rounds were exhausted or the threshold became impossible after removals.

Result:

This stage is now complete.  `Proofs/OutcomeSoundness.lean` proves three layers:

1. the direct soundness of the three closing branches of `continueDeliberation`;
2. the corresponding public council-vote and council-removal wrappers; and
3. the reachable-state wrapper, which lifts those local results to every reachable closed state.

The outcome theorem family now says exactly what the public result claims.  If the engine closes a reachable case as `demonstrated` or `not_demonstrated`, the stored vote record meets the configured threshold.  If it closes as `no_majority`, neither side reached the threshold, and closure followed one of the executable `no_majority` conditions.

### Stage 3: No stuck reachable state

Target statement: every reachable non-closed state has a next opportunity.

Result:

This stage is now complete in `Proofs/NoStuck.lean`.

The proof has two parts.

First: it proves that every reachable merits-phase state preserves a pristine deliberation record: no stored council votes, round one, and every council member still seated.  That makes the second closing statement easy to analyze, because deliberation begins from a known clean council state.

Second: it proves that every reachable live deliberation state has a next council voter.  That argument depends on the council-integrity theorems from Stage 4 and the repaired deliberation semantics in `Main.lean`.

The final theorem says exactly what the stage promised: every reachable non-closed state has a next opportunity.  In other words, the Lean engine does not strand a live case.

### Stage 4: Council vote integrity

Target statement: each council member votes at most once per round, only council members vote, non-seated members never vote, and a removed or timed-out member never returns to `seated`.

Result:

This stage is now complete in `Proofs/CouncilIntegrity.lean` and `Proofs/CouncilStatus.lean`.

The proof library now establishes:

1. unique council identifiers in every reachable state;
2. distinct current-round vote identifiers;
3. current-round votes come from seated members;
4. vote rounds are bounded by the stored deliberation round; and
5. successful public steps preserve those facts;
6. seated membership can only shrink along successful public runs; and
7. any newly introduced council vote comes from a member seated in the source state.

This is the full public theorem family the stage called for.  It says not only that the stored deliberation record is well formed, but also that the public transition system never restores a removed or timed-out member to `seated` and never admits a new vote from a non-seated member.

### Stage 5: Record provenance and monotonicity

Target statement: every admitted exhibit and technical report in a reachable state entered through an allowed filing by the same side in `arguments` or `rebuttals`, and admitted materials are append-only.

Result:

This stage is now complete in `Proofs/RecordProvenance.lean`.

The proof establishes two global facts.

First: every admitted exhibit and technical report in a reachable state has an allowed origin.  In practice that means the item was introduced in `arguments` by either side or in `rebuttals` by the plaintiff.

Second: along any successful public run, the admitted-material lists change only by appending suffixes.  A step may add new materials or leave the lists unchanged.  It does not rewrite or delete prior admitted entries.

These theorems strengthen the earlier aggregate-limit result into a real record-integrity statement.

### Stage 6: Bounded termination

Target statement: every successful execution path from initialization is finite, with an upper bound determined by the fixed merits phases, `council_size`, `required_votes_for_decision`, and `max_deliberation_rounds`.

Result:

This stage is now complete in `Proofs/BoundedTermination.lean`.

The proof defines an explicit public measure with three parts:

1. `remainingMeritsSteps`, which counts the fixed merits actions still available from the current phase;
2. `remainingDeliberationSteps`, which counts the remaining deliberation budget under the current policy and council state; and
3. `remainingStepBudget`, which combines those two pieces away from `closed`.

The file then proves the whole bounded-termination chain:

1. initialization establishes a concrete initial budget;
2. every successful public `step` strictly decreases the remaining-step budget; and
3. any successful public step path from initialization has length at most that initial budget.

This is the theorem family that rules out infinite successful runs.  The bound is procedural.  It depends only on the initialized policy and the fixed phase structure, not on the content of the filings.

### Stage 7: Deliberation neutrality

Target statement: if one flips every vote in the current deliberation record from `demonstrated` to `not_demonstrated` and back, the outcome flips the same way, and `no_majority` remains `no_majority`.

Result:

This stage is now complete.  `Proofs/Neutrality.lean` defines the vote-flip operation, proves the count-swap lemmas for the current round, rules out the dual-threshold case from the strict-majority validator, and lifts the result to every reachable state.  The public theorem is `reachable_currentResolution_is_neutral_under_vote_flip`.

## Progress notes

- 2026-04-02: Added the plan.  Stage 1 is the first implementation target.
- 2026-04-02: Proved Stage 1.  Successful initialization now establishes a fixed case frame, and every successful public step preserves it.  The proof lives in `Proofs/CaseFrame.lean`.  Stage 2 is the next implementation target.
- 2026-04-02: Began Stage 2.  `Proofs/OutcomeSoundness.lean` proved the direct soundness of the `continueDeliberation` closing branches, plus the corresponding public council-step wrappers.
- 2026-04-02: Completed Stage 2.  `Proofs/OutcomeSoundness.lean` now also proves the reachable closed-outcome theorems, so outcome soundness is established for every reachable closed state.  Stage 3 is next.
- 2026-04-02: Early Stage 3 work exposed two Lean-engine defects before the no-stuck theorem was finished: initialization did not enforce unique council member identifiers, and deliberation allowed removal of a current-round voter.  The note is in `docs/verification-notes.md`.
- 2026-04-02: Repaired those defects in `Main.lean`, added direct theorems for the repaired behavior, and restored the proof build.  Stage 3 can now proceed on the repaired engine.
- 2026-04-02: Began the Stage 3 and Stage 4 bridge work in `Proofs/CouncilIntegrity.lean` and `Proofs/NoStuck.lean`.  The reachable deliberation-record integrity invariant is now proved.  The no-stuck file contains the first operational lemmas, but not yet the final reachable-state theorem.
- 2026-04-02: Completed Stage 3 in `Proofs/NoStuck.lean`.  The proof now shows that every reachable non-closed state has a next opportunity.  The deliberation branch depends on the council-integrity layer and on the repaired council-removal semantics in `Main.lean`.
- 2026-04-02: Completed Stage 4 in `Proofs/CouncilStatus.lean`.  The proof now states the missing public monotonicity facts: seated membership can only shrink along successful public runs, and any newly introduced council vote comes from a member seated in the source state.
- 2026-04-02: Moved Stage 5 to the front of the queue.  The next proof work should explain the provenance and monotonic growth of admitted exhibits and technical reports.
- 2026-04-02: Completed Stage 5 in `Proofs/RecordProvenance.lean`.  The proof now shows that every admitted exhibit and report in a reachable state has an allowed phase-role origin, and that successful public runs modify the admitted-material lists only by appending suffixes.  Stage 6 is next.
- 2026-04-02: Began Stage 6 in `Proofs/BoundedTermination.lean`.  The file defined an explicit remaining-step budget, proved the council-roster-size facts that budget arithmetic needs, and proved the decrease lemmas for the merits actions.
- 2026-04-02: Completed Stage 6 in `Proofs/BoundedTermination.lean`.  The proof now covers the council-vote and council-removal cases as well, proves that every successful public step strictly decreases the remaining-step budget, and proves an explicit upper bound on the length of any successful public run from initialization.
- 2026-04-02: Chose the strict-majority route for Stage 7.  `validatePolicy` in Lean and Go now requires `2 * required_votes_for_decision > council_size`, so the neutrality theorem can quantify over the full validated policy space instead of carrying that condition as a separate hypothesis.
- 2026-04-02: Completed Stage 7 in `Proofs/Neutrality.lean`.  The proof now shows that `currentResolution?` commutes with the vote-flip operation on every reachable state, using the strict-majority validator together with the reachable current-round integrity and seat-bound invariants.
