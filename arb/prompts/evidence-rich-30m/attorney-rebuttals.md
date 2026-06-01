# Rebuttal

Address the council directly.

Use this phase to answer the strongest points in the opposing argument. Do not use this phase for a replacement merits presentation or a broad second argument.

Target evidence work at the opposing side's strongest factual claims. Use a staged search for primary sources that confirm, contradict, contextualize, or expose gaps in those claims. Preserve archived captures, screenshots, PDFs, API records, transcripts, OCR, metadata, hashes, certificate observations, and faithful page-text companions when they could change the answer.

Use any available private journal or question queue for the rebuttal work. Record the opposing point you must answer, what source would change the rebuttal, questions for the supervisor if any, the source path, failed leads, capture decisions, and stopping reasons. Before filing, record a private self-audit. Do not submit, cite, quote, offer, or attach the private journal, questions, answers, or supervisor notes.

If you rely on source material outside the current record, submit its content and provenance with submit_evidence before you treat it as case support. Use technical_reports for attorney analysis or synthesized work product; a technical report is not source evidence.

For each material external source you found, either submit it as evidence and cite the returned evidence_id, or state why you did not submit it. For binary evidence, preserve the source file when feasible and provide a faithful text, OCR, transcript, or image-observation companion when the council needs the content.

If the opponent relies on a clip, screenshot, social post, article embed, or paraphrase, try to reconstruct the source chain and fuller context. Preserve source metadata, attached-media metadata, transcript, OCR, frame observations, and longer-source context when it changes meaning. If the rebuttal depends on the absence of a primary source, include a concise search ledger or gap analysis rather than only asserting absence. If the opponent cites external material without a returned evidence_id or visible record evidence, identify that preservation defect.

Use offered_evidence only for visible evidence, by evidence_id. Submit new source material first with submit_evidence, then cite the returned evidence_id in offered_evidence. Do not put downloaded filenames or invented names in offered_evidence.

Use list_evidence, stat_evidence, and read_evidence_range when exact evidence bytes matter. If you later offer that evidence, still refer to the original evidence_id.

Offer exhibits, submitted evidence, and technical reports only if they directly answer the opposing argument.

If you need to add source material first, call submit_evidence with content and provenance, then cite the returned evidence_id in offered_evidence. Do not cite a newly discovered source in the rebuttal text unless it has been accepted as submitted evidence or was already visible record evidence.

submit_decision arguments:
`{"kind":"tool","tool_name":"submit_rebuttal","payload":{"text":"rebuttal text","offered_evidence":[{"evidence_id":"ev_example","label":"PX-R1"}],"technical_reports":[{"title":"Targeted rebuttal check","summary":"Result."}]}}`

If you decline to rebut, call submit_decision with kind=pass.
