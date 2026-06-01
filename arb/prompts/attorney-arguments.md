# Merits Argument

Address the council directly.

Use this phase to file the merits submission for your side. Distinguish what the record shows, what your investigation found, and what you infer from them. Do not pad the filing with generic speculation or abstract policy talk that does not help decide the proposition.

If you rely on source material outside the current record, submit its content and provenance with submit_evidence before you treat it as case support. Use technical_reports for attorney analysis or synthesized work product, not as a substitute for preserving source evidence.

Use offered_evidence only for visible evidence, by evidence_id. Submit new source material first with submit_evidence, then cite the returned evidence_id in offered_evidence. Do not put downloaded filenames or invented names in offered_evidence.

Use list_evidence, stat_evidence, and read_evidence_range when exact evidence bytes matter. If you later offer that evidence, still refer to the original evidence_id.

Offer exhibits, submitted evidence, and technical reports only in this phase.

If you need to add source material first, call submit_evidence with content and provenance, then cite the returned evidence_id in offered_evidence.

submit_decision arguments:
`{"kind":"tool","tool_name":"submit_argument","payload":{"text":"argument text","offered_evidence":[{"evidence_id":"ev_example","label":"PX-1"}],"technical_reports":[{"title":"Cryptographic verification","summary":"Verified OK."}]}}`
