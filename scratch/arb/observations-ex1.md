# Observations For Ex1

## Clerk API Run, 2026-06-04

The Clerk API run completed at `arb/out/clerk-api-ex-runs-20260604064719/arb-20260604064719-c49a36fd`.  AAR closed the case as `demonstrated` after four accepted council votes, all in favor.  The selected Pi seats used `pool.jsonl` request specs with provider restrictions, quantization where specified, and persona files preserved in the run artifacts.

The record contained eleven initial case-packet files and no submitted outside evidence.  That was the right shape for this example because the dispute turns on private packet materials: the deadline thread, assignment instructions, confession, signature, public key, printer records, distribution work order, session summary, damages breakdown, and time log.  Both lawyers read all eleven packet files during openings.

Plaintiff made the strongest technical use of the record.  It offered nine exhibits and a technical report explaining that it decoded the confession signature, verified `confession.txt` against `samantha_public.pem` with OpenSSL, and matched the local hashes to the packet identities.  The report handled the attribution limit correctly by distinguishing signature integrity against the packet public key from independent proof that the public key belonged to Samantha.

Defendant made a serious burden argument from the same packet.  It offered six exhibits and a technical report identifying missing private-case material: complete communications, draft or workspace timestamps, final handoff evidence, printer cancellation or payment terms, distribution deposit terms, and mitigation records.  That was the right defense frame because the admitted confession proves a reading-related deception more directly than it proves completion-date falsity or objective reliance.

The advocacy stayed close to the proposition.  Plaintiff treated Samantha's deadline assurance as a composite readiness representation tied to the false claim that she had read Stephenson.  Defendant tried to keep reading deception separate from a material misrepresentation about when the essay would be complete and pressed the commercial-reliance gaps.

The council review was strong through C4.  C1 through C4 all read admitted evidence and voted `demonstrated`, reasoning that the signed confession made the deadline assurance materially misleading because the assignment required reading before drafting and Peter asked direct go/no-go questions before preserving the rush print plan.  The result followed the majority reasoning in the older ex1 runs but with cleaner post-case behavior for the lawyers.

C5 failed before voting because the Pi process exceeded the council output limit.  The failure event reports `143846986` stdout-plus-stderr bytes written against a configured `134217728` byte limit, after which AAR killed `pi-C5`, recorded `agent_output_limit_exceeded`, marked C5 `failed`, and continued under the council rules.  This did not affect the result because the four earlier votes had already made the final resolution clear under the current deliberation path.

The process behavior was otherwise clean.  The MCP log has no `ok=false` lawyer or council tool calls, both lawyers submitted work notes for every turn, and the final run exited with code 0.  The live polling again showed large wall-clock jumps during a 30-second shell wait, consistent with the timestamp artifact noted in `devnotes.md`.

## Run

The OpenClaw-lawyer run completed at `out/ex1-openclaw-lawyers-20260602042739`.  AAR exited with status 0 and returned `demonstrated`, with a 4-1 council vote.  The defendant container exited with status 0, while the plaintiff container exited with status 1 after all plaintiff filings were accepted because OpenClaw reported a compaction failure.

The defendant container filed all defendant turns, then reported `wait_failed` after the Lawyer API refused a later wait request.  The case had already closed and AAR had already produced the final digest.  This repeats the post-closure API-lifetime problem seen in earlier runs: a remote lawyer can finish the case but fail to observe a clean terminal `done` state after AAR exits.

## Evidence Use

The evidence record contained eleven case-packet files: the deadline thread, print-approval note, instructions, session summary, confession text, confession signature, Samantha public key, printing invoice, distribution work order, damages breakdown, and time-and-token log.  Plaintiff read all eleven files during opening through AAR evidence tools, and defendant also read all eleven files during opening.  Later turns used `get_case`, `list_evidence`, `stat_evidence`, and selected `read_evidence_range` calls to re-check decisive items.

Plaintiff offered nine exhibits in argument: the deadline thread, print-approval note, instructions, session summary, confession text, confession signature, public key, printing invoice, and distribution work order.  Defendant offered five exhibits: the deadline thread, confession text, print-approval note, distribution work order, and session summary.  No lawyer submitted outside source evidence, which was appropriate for this closed packet case because the decisive facts were private communications, vendor records, and signature material already in the record.

The best evidence work was the signature analysis.  Plaintiff reconstructed the confession text, decoded the base64 signature, and verified it against the packet public key with OpenSSL before filing a technical report.  Defendant did not contest the technical match, but correctly argued that the verification proves integrity relative to the packet key rather than independent real-world custody of the key.

## Work Notes

The record contains eight `send_work_notes` entries, one for each lawyer turn.  The notes are useful: they include issue breakdowns, evidence read logs, adverse checks, local tool work, and unresolved gaps.  They also make it possible to distinguish the lawyers' private analysis from admitted evidence.

The notes show that both lawyers understood the phase limits.  Opening notes recognized that openings could read evidence but could not file exhibits or technical reports.  Argument notes show that both lawyers selected admitted `evidence_id` values before relying on those materials in filings.

## Advocacy

Plaintiff made a strong evidence-specific case.  The argument tied the March 10-11 deadline thread to Peter's reliance, used the print-approval note as contemporaneous corroboration, and connected the confession to materiality through the assignment requirement that Samantha read Stephenson before drafting conclusions.  The plaintiff also acknowledged weak points: no full raw thread, no independent vendor refund terms, no final handoff record, and no independent public-key custody proof.

Defendant made the best available burden argument.  The defense conceded the damaging assurance and confession, then focused on the proposition's exact terms: completion timing and reasonable reliance rather than general misconduct.  The strongest defense points were that the thread was excerpted, Peter approved the invoice while the work was still in drafting and revision, the confession concerned source reading rather than final delivery, and the March 12 distribution work order raised sequence and mitigation questions.

The council majority accepted plaintiff's inference chain under the preponderance standard.  The dissent treated the missing metadata, final handoff evidence, and vendor terms as too important to bridge by inference.  That split tracks the advocacy: both sides identified the same central gaps, and the council divided over their weight rather than over tool or record confusion.

## Process Notes

The evidence-read tools worked in every phase where needed, including openings and later argument phases.  Evidence offering worked through valid AAR evidence identifiers.  The record contains no submitted-evidence entries because neither side found or needed external material.

The post-closure lawyer behavior still needs attention.  AAR closed cleanly, but at least one lawyer kept waiting after the Lawyer API was gone and received a connection error instead of a stable terminal state.  For long-running remote lawyers, the service should preserve a case-complete response long enough for all clients to observe it.

## Run 2026-06-02

The OpenClaw-lawyer run completed at `out/ex1-openclaw-lawyers-20260602141331`.  AAR exited with status 0 and returned `demonstrated`, with a 3-2 council vote.  Both OpenClaw containers exited with status 1 after the case had closed because the model client hit a `gpt-5.5` Codex subscription usage limit while the lawyers were still waiting for later opportunities.

The case record contains forty-three events: twenty-seven evidence reads, eight attorney actions, five council votes, two council-member replacements, and one run-initialized event.  The evidence manifest contains eleven packet files and no submitted evidence.  That was acceptable in this case because the decisive materials were all private case files: the deadline thread, print-approval note, assignment instructions, session summary, confession text and signature, public key, invoice, work order, damages breakdown, and time-and-token log.

Plaintiff read all eleven packet files during opening, then re-read five items during argument.  Defendant read ten files during opening and one during argument.  Plaintiff's argument offered nine evidence files and one technical report, including signature verification of the confession using the packet public key.  Defendant offered six evidence files and argued from the same record rather than inventing missing facts.

Both lawyers submitted work notes in every phase.  The notes include evidence-read logs, planned arguments, adverse-point checks, and turn-specific strategy.  The notes remain useful for evaluating lawyer behavior because they show why each lawyer selected particular exhibits and where each lawyer saw evidentiary gaps.

The advocacy was sound.  Plaintiff treated the false source-readiness representation as the material fact and connected it to timing, reliance, print approval, and damages.  Defendant focused on burden, incomplete chronology, the distinction between source reading and final completion, and missing independent custody proof for the public key.  The council split over whether the inference from false readiness to timing/reliance was strong enough, rather than over confusion about the record.

The remaining process issue is post-case termination for remote lawyers.  AAR closed successfully, but the OpenClaw containers kept asking for work until the model layer failed on usage limits.  The long-running Lawyer API and MCP service should give a stable case-complete state after final results so a remote lawyer can stop cleanly.
