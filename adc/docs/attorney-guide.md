# ADC Attorney Guide

This draft distills the experiment-1 findings (cycles 1 through 9,
2026-07-14, [journal](../../experiment1-journal.md)) into guidance for
lawyers in ADC jury trials.  Scope limits: everything below comes
from single-call probes on the current 25-record pool with the
generic persona, on one fact family (civil misrepresentation with a
disputed electronic document), and is not yet calibrated against full
`adc run` deliberations.  Counts are small; treat every rule as a
hypothesis with stated evidence, not settled fact.

## The Governing Facts

Six of ten juries drawn from this pool will decide a
document-against-denial case for the defense on the record alone: 12
of 21 live members vote defendant on an uncorroborated document
against a sworn denial, 4 vote plaintiff, 5 are unstable.
Deliberation converges to the legally stronger side: across a
completed 2x2 (both majority directions on both a defense-normative
and a plaintiff-normative record), members held against 5-1
majorities only when their position was stronger on the record and
folded when it was weaker.  Majority size itself moved nobody.

Six members ({relace, deepseek-v3.1 at both providers, gpt-4o-mini,
qwen3-30b/bf16, mistral}) carry holdout risk: they keep their round-1
vote against majority and, for some, against the record.  About nine
in ten six-seat juries contain at least one.  Strikes (one for-cause
plus one peremptory per side) cannot clear six risky members.  A
juror frozen against the record hangs the jury, and a hung jury
blocks judgment, so holdout risk favors the defense.

## Voir Dire

Verbal questions about evidence attitudes are worthless on this pool.
Three forms were tested (attitude, experience, verification
requirement): answers did not predict votes, and in several members
the self-report was inverted (jurors who relied on an unverified
screenshot said they require verification).  The one question form
that separates jurors is a concrete evidence-collision hypothetical,
and the judge disallows that form (5 of 5 simulated rulings) as
asking whether specific proof would be enough.

What the voir dire record is good for instead:

1. Identification.  Questionnaire answers matched against offline
   exemplar answers re-identify the member 11 of 21 exactly and 17 of
   21 at model-family level.  Naming the model without exemplars
   fails (2 of 21).  An attorney with a probe-built dossier per pool
   member can read lean and holdout risk out of the match.  Three
   members answer byte-identically across independent samplings,
   which is itself a signature.
2. For-cause material.  A candid persona can state disqualifying
   bias outright; the questionnaire is where that surfaces.
3. Form reading.  Label-reasoning contradictions (a vote word that
   contradicts the sentence beside it) mark the unstable qwen3-30b
   variants.  Prose determinism does not predict vote instability.

## Strikes

Strike arithmetic is dominated by the holdout-risk set.  Plaintiff
counsel should spend the peremptory on an identified holdout-risk
member whenever the record favors the plaintiff, because a frozen
adverse round-1 vote is a hung jury and the plaintiff loses hung
juries.  Defense counsel wants holdout-risk members seated in any
case the record decides against them, and should spend strikes
instead on label-susceptible members (see Argument) when the defense
theory depends on excluding characterizations.  The judge, an
internal model, seats the final six from the survivors, so strike
value is probabilistic.

## Argument

The record beats rhetoric everywhere it was tested, and the specific
levers are ranked:

1. Authentication in the record moots the juror question.  Adding an
   independent forensic examiner to the collision case flipped every
   stable defendant-voter, 15 of 15 samples.  Real corroboration is
   the plaintiff's only move that works on everyone.
2. Counsel's own "authenticated" label moves about half the pool
   (one member absorbed it completely, unstable members shifted,
   provenance-strict members discounted it aloud), and one routine
   instruction that lawyer statements are not evidence cancels the
   entire gain and over-cancels: some jurors then treat the exhibit
   itself as excluded.  Plaintiff counsel should not lean on
   characterization; defense counsel should always request the
   instruction, which costs nothing and can poison the exhibit.
3. Damages follow documents, category by category.  One document per
   category unlocks that category (a customer cancellation email was
   worth exactly its 1,500; stress testimony was worth zero), and
   nothing else moved the number: not demand size (10x anchor, no
   effect), not persona, not closing posture, not concession.
   Plaintiff guidance: document every category, however modestly.
   Defense guidance: contest undocumented categories and nothing
   else; concession of proven liability costs nothing but also buys
   nothing measurable.
4. Round 1 is the verdict.  Anchored jurors freeze on it and
   everyone else converges to the record's stronger side by round 2.
   The closing should be built to win the first ballot: state the
   burden logic the panel will converge on, in the direction the
   record supports.

## What Does Not Work

Sympathy framing moved no votes against weak proof.  Damages anchors
moved no awards.  Verbal voir dire questions predicted nothing.
Concession bought nothing measurable.  Majority pressure by itself
flipped nobody.  Persona effects exist (a burned-by-written-lies
persona flipped deepseek-r1's collision vote 2 of 3) but are
model-dependent and did not touch damages.

## Open Validity Items

Probe votes are single calls without the tool loop a runtime Pi juror
has; calibration against full deliberation packets has not run.
Every finding is from one fact family and the generic persona.  The
pool snapshot is dated 2026-06-18 and four records are already dead,
so dossiers decay and the identification step needs a freshness
check before trial use.
