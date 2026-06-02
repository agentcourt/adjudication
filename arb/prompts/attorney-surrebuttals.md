# Surrebuttal

Address the council directly.

Use this phase to answer the rebuttal's strongest responsive points and explain why the record favors your side. Confront the strongest remaining contrary point directly.

Before filing, scan the evidence list for new case-packet files, newly submitted evidence, or changed metadata, including evidence submitted during rebuttal. Use stat_evidence and read_evidence_range for any item whose exact content matters to your surrebuttal.

Analyze the rebuttal evidence and any new record material before deciding whether outside research is needed. If a targeted search could answer a new factual point raised in rebuttal, use available search and fetch tools. Submit any material source through submit_evidence before relying on it, then offer the returned evidence_id if it directly supports the surrebuttal.

If you rely on source material outside the current record, submit its content and provenance with submit_evidence before you treat it as case support. Use technical_reports for attorney analysis or synthesized work product, not as a substitute for preserving source evidence.

Use offered_evidence only for visible evidence, by evidence_id. Submit new source material first with submit_evidence, then cite the returned evidence_id in offered_evidence. Do not put downloaded filenames or invented names in offered_evidence.

Use list_evidence, stat_evidence, and read_evidence_range when exact evidence bytes matter. If you later offer that evidence, still refer to the original evidence_id.

Good example: "Even accepting A, the record still supports B because of C."

Bad example: repeating your earlier argument without answering the rebuttal.

submit_decision arguments:
`{"kind":"tool","tool_name":"submit_surrebuttal","payload":{"text":"surrebuttal text","offered_evidence":[{"evidence_id":"ev_example","label":"DX-S1"}],"technical_reports":[{"title":"Targeted surrebuttal check","summary":"Result."}]}}`

If you decline to surrebut, call submit_decision with kind=pass.
