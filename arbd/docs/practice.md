# Practice Manual

This manual explains how to litigate in Agent Arbitration Degree as it exists in this repository.  The procedure is small on purpose.  There are no pretrial motions, no judge, no clerk, and no voir dire.  The council answers the complaint question with one integer from `0` through `100` under the governing judgment standard.

The governing source is [Agent Rules for Arbitration Degree Procedure](ARAP.md).  This manual describes the working procedure in practical terms: what each phase is for, what the record contains, and what advocates should do to build a persuasive case.  The phases are fixed and short, so sequencing matters more here than in ordinary civil litigation.

## Core Orientation

The complaint does one thing.  It states the question to be decided.  The judgment standard comes from policy or case configuration, not from the pleading.  There are no counts, defenses, motions, or discovery requests separate from the merits sequence.  Everything that matters must be presented through the merits phases and the record built there.

The procedure runs in one line: openings, arguments, rebuttals, surrebuttals, closings, and deliberation.  Claimant goes first in openings, arguments, rebuttals, and closings.  Respondent goes second in openings and arguments, may answer with a surrebuttal, and closes last.  Deliberation then proceeds through the seated council members, each of whom records one answer and one rationale.

Because the structure is this narrow, each phase has a clear job.  Openings frame the question and explain how the score should be approached.  Arguments add the record material that the council will later read.  Rebuttal and surrebuttal answer the other side's method or score.  Closings explain why the full record supports one advocated number better than nearby alternatives.

## Phase Map

| Phase | Who acts | What belongs there |
|---|---|---|
| `openings` | claimant, then respondent | theory of the case, framing of the numeric question, and explanation of how the judgment standard applies |
| `arguments` | claimant, then respondent | merits argument, exhibits, technical reports, and a concrete advocated score or range |
| `rebuttals` | claimant | response to the respondent's method or score, with targeted exhibits and technical reports if needed, or a pass |
| `surrebuttals` | respondent | response to the rebuttal, or a pass |
| `closings` | claimant, then respondent | final application of the full record to the question, with a concrete advocated answer |
| `deliberation` | council members | individual answers and rationales |

In the current implementation, `arguments` remains the main record-building phase.  Rebuttal may also add exhibits and technical reports, but only for the claimant and only as a response to the respondent's argument.  Surrebuttal remains text-only.

## Openings

Openings should be short, clear, and methodical.  At that point the record contains only the question and the governing judgment standard.  A good opening therefore states what features should move the score up or down, explains which features should be discounted, and gives the council a method for reading later evidence.

Use openings to frame the method and identify the evidence that would justify a low, middle, or high answer.  Reserve detailed scoring tables for record material already introduced.  If the opening overstates facts that are not yet in the record, the side loses credibility for no gain.

## Arguments and Record Building

Arguments are the center of the case.  This is the main phase in which a side may offer files as exhibits and submit technical reports.  Counsel should therefore treat the arguments phase as both merits briefing and record assembly.

Use a selective record-building strategy.  Offer the files that matter to the score and the judgment standard, and use technical reports when a concrete check will move the number in an explainable way.  In a similarity case, that may mean a side-by-side alignment, a chronology check, a structural comparison, or a report that separates distinctive reuse from background conventions.

A good argument ties each offered file to a specific scoring consequence.  One document may establish the source text, another may show the later text, and a report may explain how much of the wording, structure, or development persisted.  If a technical check weakens your side's preferred score, report that weakness directly and explain why the remaining evidence still supports the advocated number.

Arguments in `arbd` should also make their scoring method explicit.  A side should say which factors matter, which factors should be discounted, and why its advocated number fits the record better than numbers ten points lower or higher.  A filing that argues for `92` without explaining why `82` is too low and why `98` is too high leaves the decisive work to the council without guidance.

## Rebuttal, Surrebuttal, and Closings

Rebuttal and surrebuttal are narrow response phases.  They exist to answer the other side's method, weighting, or use of the record.  A good rebuttal identifies one or two decisive failures in the respondent's scoring approach, then shows why those failures matter under the stated judgment standard.

These phases should not repeat the full merits presentation.  Rebuttal may sharpen the record, but it should do so only in direct response to the respondent's case.  What remains is argument about how the council should read the record, which features are distinctive, which similarities should be discounted, and which nearby numbers fail to fit the evidence.

Closings should synthesize, not expand.  By then the council has the full set of filings, exhibits, and technical reports.  The closing should tell the council what number the record supports, why nearby alternatives fit less well, and how the judgment standard bears on disputed inferences.  A strong closing in `arbd` almost always names a concrete answer rather than a vague upper or lower band.

## Council Deliberation

The council answers after closings.  Each member casts one individual answer with a rationale.  Counsel should therefore expect the result to show any real spread across the council.

Advocate clarity matters here.  The council reads the filings and record directly, then gives reasons in its own words.  Arguments that depend on an unstated weighting rule or an undocumented factual leap tend to break down here, because each council member has to reconstruct the missing method independently.

## Practical Method

A good working method for this forum has three stages.  First, define the exact question and identify what facts would justify low, middle, and high answers under the stated standard.  Second, decide which documents and technical checks can prove or disprove those facts.  Third, map those materials into the arguments phase so that rebuttal and closing can stay focused on weighting and inference instead of scrambling to supply missing support.

This procedure rewards concentration.  Because there is no separate motion practice, no discovery phase, and no evidentiary hearing outside the merits sequence, every filing should do visible work.  The best cases in this forum are compact, supported, and explicit about the path from record to advocated number.
