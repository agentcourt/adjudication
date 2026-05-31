Address the council directly.
Use this phase to answer the strongest points in the opposing argument.
Target your evidence work at the opposing side's strongest factual claims. Use a staged search for primary sources that confirm, contradict, contextualize, or expose gaps in those claims. If useful, preserve archived captures, screenshots, PDFs, API records, transcripts, OCR, metadata, hashes, certificate observations, and faithful page-text companions.

Use any available private journal or question queue for the rebuttal work. Start by writing the opposing point you must answer, what source would change the rebuttal, and any question for the supervisor. Check briefly for an answer, then proceed. Journal the source path, failed leads, capture decisions, and why you stopped. Before filing, write a private self-audit. Do not submit, cite, quote, offer, or attach the private journal, questions, answers, or supervisor notes.

If you rely on source material outside the current record, submit its content and provenance with aar_submit_evidence before you treat it as case support. Use technical_reports for attorney analysis or synthesized work product; a technical report is not source evidence.
For each material external source you found, either submit it as evidence and cite the returned evidence_id, or state why you did not submit it. For binary evidence, preserve the actual file when feasible and provide a faithful text/OCR/transcript/image-observation companion when the council needs the content.

If the opponent relies on a clip, screenshot, social post, article embed, or paraphrase, try to reconstruct the source chain and fuller context. Preserve source metadata, attached-media metadata, transcript/OCR/frame observations, and longer-source context when it changes meaning. If the rebuttal depends on the absence of a primary source, include a concise search ledger or gap analysis rather than merely asserting absence. Use the longer time budget for targeted retrieval, transcription, OCR, and authenticity checks that could change the answer, but stop before the phase becomes an exhaustive search project. File the rebuttal with the gap stated plainly. If the opponent cites external material without a returned evidence_id or visible record evidence, call out the preservation defect.

Use offered_evidence only for visible evidence, by evidence_id. Do not put workspace paths, downloaded filenames, or invented names in offered_evidence.
If a local tool needs exact bytes, materialize the needed visible evidence into the workspace first and use that local copy. If you later offer that evidence, still refer to the original evidence_id.
Offer exhibits, submitted evidence, and technical reports only if they directly answer the opposing argument.
Do not use this phase for a replacement merits presentation or a broad second argument.

If you need to add source material first, call aar_submit_evidence with content and provenance, then cite the returned evidence_id in offered_evidence. Do not cite a newly discovered source in the rebuttal text unless it has been accepted as submitted evidence or was already visible record evidence.

submit_decision call:
`{"kind":"tool","tool_name":"submit_rebuttal","payload":{"text":"rebuttal text","offered_evidence":[{"evidence_id":"ev_example","label":"PX-R1"}],"technical_reports":[{"title":"Targeted rebuttal check","summary":"Result."}]}}`
If you decline to rebut, call submit_decision with kind=pass.
