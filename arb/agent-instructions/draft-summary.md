# Drafting Arbitration Summaries

Use these instructions to draft a `summary.md` for one example directory in a completed AAR batch, such as `out/local-direct-three-per-ex-only-20260629/ex13/summary.md`.  The audience is interested in the dispute and the arbitration result.  Write for readers who may not know AAR, models, MCP, or the local run machinery.

## Scope

Use the run artifacts as the record unless the user asks for independent source verification.  Do not search outside the record, correct the record from memory, or state external facts as though the arbitration proved them.  If the record contains source URLs, describe how the parties and council used those sources, and say when the summary has not independently verified them.

Summarize every run in the selected example directory.  If the directory contains `run-01`, `run-02`, and `run-03`, the summary covers all three.  Do not collapse materially different runs into one narrative without first reporting their separate outcomes, evidence sets, council composition, vote counts, and failures.

Prefer the structured artifacts over prose digests when extracting facts.  Use `run.json` and `state.json` for case IDs, timestamps, status, resolution, admitted evidence, filings, technical reports, council members, council votes, and failures.  Use `digest.md`, `transcript.md`, submitted evidence files, and `evidence-manifest.json` to check wording, source titles, exhibit labels, and readable context.  Use the batch `ledger.csv` to cross-check run status, resolution, timestamps, and cleanup notes.

## Output Structure

Start with the dispute, then the result.  A reader should learn the proposition, the governing clarification, and the resolution before reading procedure or implementation details.

Use this section order unless the record calls for a narrow adjustment:

1. `# exNN Summary`
2. `## Proposition and Clarifying Rules`
3. `## Resolution Summary`
4. `## Procedure`
5. `## Results`
6. `## Evidence and Arguments`
7. `## Deliberation Results`
8. `## Juror Explanation Summary`

In the proposition section, put the proposition in a Markdown block quote.  State the evidence standard and link to every case-packet clarification document that controls the dispute.  Summarize the clarification in plain terms: the definition of the proposition, what proof can establish it, what proof is excluded, and any source hierarchy.

The resolution summary should be short.  State the outcome across the runs, the vote pattern, and the main evidentiary reason for the outcome.  Do not repeat the full results table in prose.

The procedure section should explain AAR in neutral terms for an unfamiliar reader.  A concise paragraph is enough: the parties make filings, submit source captures and reports, the admitted record closes, and a council votes under the stated evidence standard.  Then identify the record artifacts used for the draft with a real Markdown list, grouped by artifact type.

The results section should elaborate in a table.  Include each run, case ID, start time, finish time, resolution, vote tally, and any non-voting or failed council member.  Link run labels to the corresponding `run.json` files.

The evidence and arguments section should summarize the admitted evidence set and the parties' use of it.  Use a table for evidence sources because the columns are parallel: source, runs and submitting side, and use in the record.  Link internal artifacts with relative Markdown links.  Link external source URLs only when they appear in the record, and describe them as record sources rather than as independently verified facts.

The deliberation section should list every voting or failed council member.  Include submitted votes, failed members, deliberation rounds when relevant, and a concise explanation summary for each submitted rationale.  The table should preserve disagreements, abstentions, no-majority outcomes, and process failures.

The juror explanation summary should synthesize the council's reasoning.  Group recurring reasons, source-weight judgments, missing links, and material disagreements.  Do not create a new merits decision.  The summary explains the jurors' stated reasons.

## Extraction Checklist

Before drafting, collect these facts:

- Proposition, evidence standard, and case-packet clarification files.
- Run IDs, case IDs, start and finish timestamps, process status, phase, and resolution.
- Submitted evidence titles, source URLs, submitting side, phase, evidence IDs, hashes, and relevance descriptions.
- Offered exhibit labels, when they help connect filings to submitted evidence.
- Technical reports and source-search summaries.
- Openings, arguments, rebuttals, surrebuttals, and closings.
- Council members, model identifiers when relevant to process reporting, member status, failure reasons, submitted votes, rounds, and rationales.
- Batch ledger status and notes.

Cross-check the same fact in more than one artifact when possible.  For example, compare `ledger.csv` against each `run.json`, and compare council votes in `run.json` against the digest.  If artifacts conflict, report the conflict instead of resolving it silently.

## Writing Rules

Use careful, record-based language.  Write "the record contains no admitted signed agreement" when that is what the run record supports.  Do not write "no signed agreement existed" unless independent verification has been requested and performed.

Distinguish official sources, party arguments, and council reasoning.  A source's title and relevance description are part of the record, but the summary should not treat a party's relevance statement as the summary writer's finding unless the council adopted that reasoning or the admitted source text independently supports it.

Keep the technology in the background.  Mention process failures, council-member failures, and multiple runs because they affect the arbitration record.  Do not foreground model names, provider routing, MCP tools, logs, or local runtime details unless they bear on the result or the user asks for a technical summary.

Use real enumerations for natural lists.  Long inline lists of artifacts, runs, or evidence types should become short Markdown lists or tables.  Use paragraphs for explanation and tables for parallel data.

Use internal Markdown links for local artifacts.  Link `rules.txt`, `run.json`, `digest.md`, `evidence-manifest.json`, transcripts, and submitted evidence files when the summary names them.  Prefer clear titles as link text, except when the filename is the meaningful reference, such as `rules.txt`.

Avoid overclaiming consensus.  If all runs agree, say so and give the count.  If evidence or arguments differ by run, report the variation before summarizing the common pattern.

Do not hide process defects.  Failed council members, missing votes, no-majority outcomes, failed runs, partial artifacts, or cleanup notes belong in the summary when they affect how a reader should understand the result.

End with the council reasoning rather than a new editorial conclusion.  The document should read as a fair arbitration-record summary, not an appellate opinion or a technology report.
