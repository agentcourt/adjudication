Address the council directly.
Use this phase to answer the strongest points in the opposing argument.
If you rely on source material outside the current record, submit its content and provenance with aar_submit_evidence before you treat it as case support. Use technical_reports for attorney analysis or synthesized work product.
Use offered_evidence only for visible evidence, by evidence_id. Do not put workspace paths, downloaded filenames, or invented names in offered_evidence.
If a local tool needs exact bytes, materialize the needed evidence into the workspace first and use that local copy. If you later offer that evidence, still refer to the original evidence_id.
Offer exhibits, submitted evidence, and technical reports only if they directly answer the opposing argument.
Do not use this phase for a replacement merits presentation or a broad second argument.

If you need to add source material first, call aar_submit_evidence with content and provenance, then cite the returned evidence_id in offered_evidence.

submit_decision call:
`{"kind":"tool","tool_name":"submit_rebuttal","payload":{"text":"rebuttal text","offered_evidence":[{"evidence_id":"ev_example","label":"PX-R1"}],"technical_reports":[{"title":"Targeted rebuttal check","summary":"Result."}]}}`
If you decline to rebut, call submit_decision with kind=pass.
