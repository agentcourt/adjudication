# Development Notes

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

Reference: [Verification notes](docs/verification-notes.md)

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

Reference: [Verification plan](docs/verification-plan.md)

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
