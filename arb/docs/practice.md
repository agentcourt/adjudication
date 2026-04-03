# Practice Manual

This manual explains how to litigate in Agent Arbitration as it exists in this repository.  The procedure is small on purpose.  There are no pretrial motions, no judge, no clerk, and no voir dire.  The council decides whether the proposition in the complaint is substantially true under the governing standard of evidence.

The governing source is [Agent Rules for Arbitration Procedure](ARAP.md).  This manual describes the working procedure in practical terms: what each phase is for, what the record contains, and what advocates should do to build a persuasive case.  The phases are fixed and short, so sequencing matters more here than in ordinary civil litigation.

## Core orientation

The complaint does one thing.  It states the proposition to be decided.  The standard of evidence comes from policy or case configuration, not from the pleading.  There are no claims, counts, defenses, motions, or discovery requests separate from the merits sequence.  Everything that matters must be presented through the merits phases and the record built there.

The procedure runs in one line: openings, arguments, rebuttals, surrebuttals, closings, and deliberation.  Claimant goes first in openings, arguments, rebuttals, and closings.  Respondent goes second in openings and arguments, may answer with a surrebuttal, and closes last.  Deliberation then proceeds in rounds until one side reaches the configured vote threshold or the final round ends without one.

Because the structure is this narrow, each phase has a clear job.  Openings frame the case.  Arguments add the record material that the council will later read.  Rebuttal and surrebuttal answer the other side’s merits theory.  Closings explain why the full record satisfies or fails the stated standard of evidence.

## Phase map

| Phase | Who acts | What belongs there |
|---|---|---|
| `openings` | claimant, then respondent | theory of the case and how the standard of evidence applies |
| `arguments` | claimant, then respondent | merits argument, exhibits, and technical reports |
| `rebuttals` | claimant | response to the respondent’s argument, with targeted exhibits and technical reports if needed, or a pass |
| `surrebuttals` | respondent | response to the rebuttal, or a pass |
| `closings` | claimant, then respondent | final application of the full record to the proposition |
| `deliberation` | council members | individual votes and rationales |

This table is stricter than it may look.  In the current implementation, `arguments` remains the main record-building phase.  Rebuttal may also add exhibits and technical reports, but only for the claimant and only as a response to the respondent’s argument.  Surrebuttal remains text-only.

## Openings

Openings should be short, clear, and disciplined.  At that point the record contains only the proposition and the governing standard of evidence.  A good opening therefore states the factual theory at a high level, explains what the advocate expects the record to show, and tells the council why those expected facts matter under the burden of proof.

This phase is not the place for technical experiments, exhibit offers, or detailed quotations from unseen files.  A strong opening names the dispute cleanly and gives the council a structure for reading the record that will come later.  If the opening overstates facts that are not yet in the record, the side loses credibility for no gain.

## Arguments and record building

Arguments are the center of the case.  This is the main phase in which a side may offer files as exhibits and submit technical reports.  Counsel should therefore treat the arguments phase as both merits briefing and record assembly.

The best approach is selective, not exhaustive.  Offer only the files that matter to the proposition and the standard of evidence.  Use technical reports when a concrete check will strengthen or weaken a decisive point: signature verification, document integrity, timing, arithmetic, or a similar question that the council should not have to infer from rhetoric alone.

A good argument ties each offered file to a specific inferential step.  One document may establish the statement at issue, another may show reliance or consequences, and a technical report may confirm authenticity or expose a break in the other side’s account.  If a technical check fails, report the failure directly and explain what follows from that failure.  The council is better served by a precise failed check than by a vague claim that the material is suspicious.

The arguments phase still determines most later flexibility.  Rebuttal may add targeted exhibits or reports in answer to the respondent’s case, but that is a narrow second chance, not a second full record-building window.  Surrebuttal cannot add exhibits or reports.  The right working question remains whether the council will need the item to decide the proposition at all.  If the answer is yes, counsel should usually introduce it in arguments.

## Rebuttal, surrebuttal, and closings

Rebuttal and surrebuttal are narrow response phases.  They exist to answer the other side’s theory.  A good rebuttal identifies one or two decisive failures in the respondent’s argument, then shows why those failures matter under the standard of evidence.  If the claimant needs one additional document or one targeted technical report to answer the respondent directly, rebuttal can now carry that material.  Surrebuttal does not have the same record-building authority.

These phases should not repeat the full merits presentation.  Rebuttal may sharpen the record, but it should do so only in direct response to the respondent’s case.  What remains is argument about how the council should read the record, what inferences are justified, and which gaps matter.  If a side has nothing useful to add, it should pass and preserve focus for the closing.

Closings should synthesize, not expand.  By then the council has the full set of filings, exhibits, and technical reports.  The closing should tell the council what proposition the record proves, what proposition it does not prove, and why the burden of proof resolves the dispute one way rather than the other.

## Council deliberation

The council votes after closings.  Each member casts an individual vote with a rationale.  The resolution is `substantially_true` or `not_substantially_true` if either side reaches the configured vote threshold in a round.  If all members vote in a round and neither side reaches that threshold, the matter moves to the next round until the configured limit is reached.

No foreperson consolidates the vote, and no judge supplies a tie-breaking view.  The recorded votes are the deliberation output.  That makes advocate clarity important: the council reads the filings and record directly, then gives reasons in its own words.  Arguments that depend on a missing inferential step or an undocumented factual leap tend to break down here.

## Practical method

A good working method for this forum has three stages.  First, define the exact proposition and identify what facts must be true for the proposition to satisfy the stated standard.  Second, decide which documents and technical checks can prove or disprove those facts.  Third, map those materials into the arguments phase so that rebuttal and closing can stay focused on inference instead of scrambling to supply missing support.

This procedure rewards concentration.  Because there is no separate motion practice, no discovery phase, and no evidentiary hearing outside the merits sequence, every filing should do visible work.  The best cases in this forum are compact, supported, and explicit about the inferential path from record to resolution.
