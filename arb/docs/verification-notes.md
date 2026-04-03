# Verification Notes

## Purpose

This note records implementation and verification findings exposed by the Lean proof work.  The first two findings were concrete engine defects found while trying to prove that every reachable non-closed state has a next opportunity.  The third finding was different.  It was not an implementation bug in the narrow sense.  It was a mismatch between a planned theorem and the validated policy space.

The proof library already established reachable phase order, procedural parity, aggregate material limits, case-frame immutability, and outcome soundness.  The next theorem, no stuck reachable state, required a more exact account of what a live deliberation state means.  That is where the defects appeared.

## Status

As of 2026-04-02, all three findings described here are fixed or resolved in [`engine/Main.lean`](../engine/Main.lean).

Initialization now rejects duplicate council identifiers.  Deliberation now rejects removal of a member who already voted in the current round.  The repair is covered directly in [`engine/Proofs/InitializeCase.lean`](../engine/Proofs/InitializeCase.lean) and [`engine/Proofs/Deliberation.lean`](../engine/Proofs/Deliberation.lean), and the full proof and engine builds pass again.

The third finding is resolved by design choice and then discharged in the proof library.  Policy validation now requires `2 * required_votes_for_decision > council_size`, so the deliberation-neutrality theorem ranges over a policy space in which both substantive outcomes cannot simultaneously satisfy the threshold.  `Proofs/Neutrality.lean` now proves the corresponding reachable-state theorem.

## Findings

| Area | Defect | Why it matters |
|---|---|---|
| Initialization | `initializeCase` does not enforce unique council member identifiers. | Deliberation uses `member_id` as the identity key for voting, removal, and next-opportunity selection.  Without uniqueness, several seat-level rules collapse. |
| Deliberation | `removeCouncilMember` allows removal of a member who already voted in the current round. | The round-completion rule counts stored votes and seated members.  Removing a voter changes that comparison and can advance the round before every remaining seated member has voted. |
| Aggregation policy | The planned deliberation-neutrality theorem is false under the current policy space. | `currentResolution?` checks `demonstrated` before `not_demonstrated`.  When both thresholds can be met at once, flipping all votes does not necessarily flip the outcome. |

## Finding 1: Council member identifiers are not validated for uniqueness

The current initialization path in [`engine/Main.lean`](../engine/Main.lean) checks the number of supplied council members against `policy.council_size`.  It does not check whether `member_id` values are unique.

The deliberation logic treats `member_id` as the identifier for a council seat in three separate places:

1. `recordCouncilVote` checks whether a member is known and whether that member already voted in the current round.
2. `removeCouncilMember` finds the member to remove by `member_id`.
3. `nextCouncilMember?` decides who should receive the next vote opportunity by comparing the current round's vote list against `member_id`.

If two seated members share the same `member_id`, those rules stop describing a coherent seat model.  A single vote can block both seats for the round because the duplicate-check is keyed by `member_id`, not by seat position.  A single removal can update both seats because the removal map rewrites every matching `member_id`.  The engine still counts both seats in `council_size` and in the seated-member count, but the vote and removal rules no longer distinguish them.

This defect became visible during the proof work because the next theorem family needs a clear statement of council identity.  The already-proved case-frame theorem in [`engine/Proofs/CaseFrame.lean`](../engine/Proofs/CaseFrame.lean) says that council identities remain fixed after initialization.  That statement assumes the engine has a meaningful notion of identity to preserve.  The planned council-vote-integrity theorem makes that assumption even more directly.  It cannot say "each council member votes at most once per round" if two seats can share the same identifier.

The right repair is to reject duplicate `member_id` values during initialization.  The engine already treats `member_id` as the identity key.  Initialization should enforce that choice.

## Finding 2: Removing a current-round voter changes the meaning of round completion

The second defect is more subtle.  In [`engine/Main.lean`](../engine/Main.lean), `removeCouncilMember` allows removal of any seated member during deliberation.  It does not reject a member who already voted in the current round.

That interacts badly with `continueDeliberation`.  The current code treats a round as complete when

`(currentRoundVotes c).length = seatedCouncilMemberCount c`.

That equation is sound only if the current round's vote list refers to the same set of seats that the seated-member count refers to.  The engine violates that assumption when it removes a member who already voted in the current round.  The vote remains in `currentRoundVotes`.  The seated-member count decreases.  The equality can therefore become true even though one or more currently seated members have not yet voted.

### Concrete execution trace

The defect is visible through the public Lean engine executable, not just by inspection of the source.

One direct example uses:

- `council_size = 5`
- `required_votes_for_decision = 4`
- `max_deliberation_rounds = 3`

Starting from a normal initialized case and moving through the merits phases into deliberation, the following round-one actions are all accepted:

| Step | Effect |
|---|---|
| `C1` votes | Round 1 has one stored vote. |
| Remove `C1` as `timed_out` | `C1` is no longer seated, but the vote remains in the round-one vote list. |
| `C2` votes | Round 1 now has two stored votes. |
| `C3` votes | Round 1 now has three stored votes. |
| `C4` votes | Round 1 now has four stored votes. |

At that point, the seated members are `C2`, `C3`, `C4`, and `C5`.  `C5` has not voted in round one.  Even so, the engine advances to round two, because:

- `currentRoundVotes.length = 4`; and
- `seatedCouncilMemberCount = 4`.

The round advances not because every remaining seated member voted, but because the stored vote count happens to match the reduced seated count after the earlier removal.

The next opportunity becomes a round-two vote opportunity instead of a round-one vote opportunity for `C5`.  That is a procedural defect, not merely an unusual internal representation.

## Why the proof work exposed these defects

Happy-path runs do not put pressure on these corners of the state machine.  The existing proof work already succeeded on several meaningful theorems:

- reachable phase order;
- procedural parity;
- aggregate material limits;
- case-frame immutability; and
- reachable outcome soundness.

Those theorems did not require a fully explicit account of council identity or of what it means for a deliberation round to be complete.  The no-stuck theorem does.

To prove that every reachable non-closed state has a next opportunity, one has to say what the deliberation phase means when no decision has yet been reached.  Two questions:

First: what exactly is a council member?  The engine answers that with `member_id`, but initialization did not enforce the uniqueness that answer requires.

Second: when exactly is a round complete?  The natural procedural statement is "every currently seated member has voted in the current round."  The current implementation does not enforce that statement after removal of a current-round voter.

The proof forced the engine's implicit assumptions into explicit propositions.  Two of those assumptions turned out to be false in the current implementation.

## Repair

The repair stayed direct.

1. `initializeCase` now rejects duplicate `member_id` values.
2. `removeCouncilMember` now rejects removal of a member who already voted in the current round.

The second fix is the better repair.  An alternative would have been to keep allowing the removal and redesign round completion around a more complicated notion of effective current-round voters.  That would have changed the deliberation model itself.  The present engine does not need that complexity.  It only needs to preserve the simple procedural rule that a round completes after every currently seated member has had a chance to vote.

The proof work can now continue on a better foundation.  The no-stuck theorem will describe the actual public procedure instead of a procedure with hidden exceptions.

## Finding 3: Deliberation neutrality failed under the earlier policy space

The next planned theorem in [`docs/verification.md`](verification.md) says that if one flips every vote in the current deliberation record from `demonstrated` to `not_demonstrated` and back, the result should flip the same way, with `no_majority` unchanged.

That statement was false for the earlier engine and policy validation rules.

The reason is in [`currentResolution?`](../engine/Main.lean).  The function checks the two thresholds in a fixed order:

1. if `demonstrated` votes reach the threshold, return `some "demonstrated"`;
2. else if `not_demonstrated` votes reach the threshold, return `some "not_demonstrated"`; and
3. otherwise return `none`.

That rule is neutral only when both thresholds cannot be satisfied at the same time.  The earlier policy validator did not enforce that condition.  It required only:

- `required_votes_for_decision > 0`; and
- `required_votes_for_decision ≤ council_size`.

So the engine allows policies like:

- `council_size = 2`
- `required_votes_for_decision = 1`

Under that policy, one `demonstrated` vote and one `not_demonstrated` vote satisfy both thresholds at once.

### Concrete counterexample

I checked the executable Lean code directly with a two-member deliberation state whose current-round votes are:

- `C1: demonstrated`
- `C2: not_demonstrated`

With `required_votes_for_decision = 1`, the current engine returns:

`currentResolution? c 1 = some "demonstrated"`

If one flips both stored votes and asks again, the engine still returns:

`currentResolution? flipped 1 = some "demonstrated"`

The result does not flip.  The planned neutrality theorem therefore fails.

This is not a proof artifact.  It was the actual behavior of the aggregation rule under the earlier validator.

## Meaning

The issue was not that the engine behaved inconsistently.  It behaved exactly as written.  The issue was that the neutrality theorem required an assumption the validator did not impose.

The missing assumption is that the decision threshold must be large enough that both outcomes cannot simultaneously reach it.  For a fixed council size, that means a strict-majority condition:

`2 * required_votes_for_decision > council_size`

Without that condition, the engine has a built-in tie-breaking bias toward `demonstrated`, because it checks that branch first.

## Resolution

The engine now takes the first of the three clean options that this finding exposed.  Policy validation requires `2 * required_votes_for_decision > council_size`.

That choice keeps the present aggregation rule and removes the dual-threshold states that made neutrality false.  The proof work did its job here.  It exposed an assumption that had not yet been made explicit, and the validator now states it directly.
