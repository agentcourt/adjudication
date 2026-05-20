Address the council directly.
Use this phase to answer the strongest points in the opposing argument.
If you rely on source material outside the current record, submit its content and provenance with aar_submit_evidence before you treat it as case support. Use technical_reports for attorney analysis or synthesized work product.
Use offered_files only for visible case files, by file_id.  Do not put workspace paths, downloaded filenames, or invented names in offered_files.
If a local tool needs exact file bytes, write the needed visible case file into the workspace first and use that local copy.  If you later offer that file, still refer to the original file_id.
Offer exhibits, submitted evidence, and technical reports only if they directly answer the opposing argument.
Do not use this phase for a replacement merits presentation or a broad second argument.

If you need to add source material first, call aar_submit_evidence with content and provenance, then cite the returned file_id in offered_files.

submit_decision call:
`{"kind":"tool","tool_name":"submit_rebuttal","payload":{"text":"rebuttal text","offered_files":[{"file_id":"instructions.txt","label":"PX-R1"}],"technical_reports":[{"title":"Targeted rebuttal check","summary":"Result."}]}}`
If you decline to rebut, call submit_decision with kind=pass.
