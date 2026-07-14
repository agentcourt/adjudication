# Proof Work Status

This note records the proof agenda after the July 2026 certificate and ARB proof work.  It supersedes the earlier ARB proof review that treated realisability, maximal-run terminal accounting, opportunity agreement, and certificate replay as open items.  The current branch has those results in Lean, so the current theorem surface is the starting point for the next proof pass.

## Current ARB Surface

ARB is the most complete proof target.  Its proof library has 38 proof files, 665 theorem or lemma declarations, and 21,297 lines according to `arb/docs/proofstats.md`.  A targeted scan found no `sorry`, no `axiom` declarations, and no `unsafe` declarations in the ARB, ADC, or AARD engine proof trees; the word `admit` appears only in prose.

| Area | Anchor theorem or file | Status |
| --- | --- | --- |
| Reachability | `Reachable`, `StepReachableFrom` | Executions are modeled as successful initialization followed by successful public steps. |
| Phase order and parity | `reachable_phaseShape`, `reachable_proceduralParity` | Merits filings preserve the required order and side-to-side parity. |
| Case frame | `initialized_run_preserves_caseFrame` | Proposition, policy, and council identity are fixed across a run. |
| Material limits and provenance | `reachable_materialLimitsRespected`, `reachable_recordProvenance`, `stepReachableFrom_materialsExtend` | Admitted materials respect caps, have allowed origins, and grow by suffix. |
| Outcome soundness | `reachable_closed_demonstrated_sound`, `reachable_closed_not_demonstrated_sound`, `reachable_closed_no_majority_sound` | Closed outcomes follow from current-round votes, the threshold, and executable closure conditions. |
| Liveness and realisability | `reachable_nonclosed_has_nextOpportunity`, `reachable_active_has_successful_step` | Reachable live states expose an opportunity and admit at least one successful public action. |
| Maximal paths | `initializedStepPathMaximal_terminal_accounted` | A maximal successful path from initialization ends closed with an enumerated resolution or failed with an accounted party-opportunity failure. |
| Opportunity agreement | `accepted_actor_action_matches_current_opportunity` | Accepted actor-facing actions match the advertised role and allowed-tool list. |
| Replay certificates | `checkReplayCertificate_terminal_facts` | Accepted terminal replay certificates expose exact replay, reachability, bounded length, decision-summary replay, and closed or failed terminal facts. |
| Failure resilience | `step_fail_opportunity_same_round_resilience` | Same-round opportunity failure preserves stored council votes and does not create a substantive result once no substantive outcome remains viable. |
| Decision rule | `DecisionRuleFacts`, `DecisionRuleCharacterization` | The executable threshold rule has permutation invariance, neutrality, quota monotonicity, and a count-level characterization. |
| Due process | `reachable_status_closed_merits_complete`, certificate due-process facts | Closed cases and accepted closed certificates carry the ordered mandatory merits filings and filing-count facts. |

## Certificate Work

The certificate plan has been carried across all three current adjudication procedures.  ARB remains the reference implementation because its proof package is deepest, but ADC and AARD now have runtime certificates, explicit verifier commands, service artifact exposure, and Lean replay facts.  Services list and fetch certificate artifacts; they do not run replay verification during case creation, listing, polling, or artifact reads.

| System | Runtime boundary | Proof boundary |
| --- | --- | --- |
| ARB | Writes `certificate.json`; `aar verify-certificate` replays against `state.json`. | Accepted terminal certificates expose exact replay, reachability, bounded length, closed outcome soundness, due-process facts, decision-summary replay, or failed-opportunity facts. |
| ADC | Writes `state.json` and `certificate.json`; `adc verify-certificate` checks final-state hashes and replays accepted transitions. | Accepted certificates expose exact replay, replay-start reachability, closed-terminal accounting, verdict facts, juror-failure verdict facts, judgment facts, a combined outcome package, and concrete replayed examples. |
| AARD | Writes `state.json` and `certificate.json`; `aard verify-certificate` replays initialization and accepted actions. | Accepted terminal certificates expose exact replay, reachability, terminal accounting, closed answer-pair replay, failed-case failure-record replay, and checked closed and failed examples. |

## Remaining Candidates

The remaining useful proof work supports operational or adjudicative claims that the system already makes.  The next items avoid abstract rule spaces or shared certificate schemas unless the runtime has a concrete output that needs that proof.  The table below orders candidates by current value against expected proof and design cost.

| Priority | Candidate | Reason |
| --- | --- | --- |
| 1 | ADC hung-jury timeout facts | ADC now packages juror-timeout verdict certificates.  A matching hung-jury package is useful if terminal packets report timeout-derived hung juries as a system claim. |
| 2 | ARB decision-rule package at certificate boundary | ARB has decision-rule facts over reachable states.  Packaging the relevant facts for accepted closed certificates would give a checked packet one certificate-facing decision-rule statement. |
| 3 | ARB removal-interleaving vote-order theorem | Current ARB vote-order work covers current-round vote permutation with fixed seating and round.  Removal interleavings are harder because removals change the denominator, so this should wait for a precise operational claim. |

AARD now covers the current certificate report boundary for both terminal shapes.  ADC's remaining certificate work is narrower after the outcome package, and it should follow concrete runtime outputs.  Further ARB work packages existing facts at the certificate boundary before adding new theory.

## Current Limits

The proof surface verifies the executable procedure and stored records.  It does not prove that a lawyer searched well, that a council member reasoned well, or that the underlying proposition is true.  Those limits are appropriate for the current system: Lean verifies the procedure, the certificate binds a packet to the engine transition sequence, and the record remains the source for human review of advocacy and evidence quality.
