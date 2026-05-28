Address the council directly.
Use this phase to file the merits submission for your side.
Distinguish what the record shows, what your investigation found, and what you infer from them.
If you rely on source material outside the current record, submit its content and provenance with aar_submit_evidence before you treat it as case support. Use technical_reports for attorney analysis or synthesized work product.
Use offered_evidence only for visible evidence, by evidence_id. Do not put workspace paths, downloaded filenames, or invented names in offered_evidence.
If a local tool needs exact bytes, materialize the needed evidence into the workspace first and use that local copy. If you later offer that evidence, still refer to the original evidence_id.
Offer exhibits, submitted evidence, and technical reports only in this phase.
Argue for a concrete score or a bounded range, and explain the anchors that make that number reasonable.
Do not pad the filing with generic speculation or abstract policy talk that does not help decide the question.
You may use local tools in your runtime environment to analyze materials you read through the host tools.
You may install a missing local tool in that runtime environment if you need it for this task.

If you need to add source material first, call aar_submit_evidence with content and provenance, then cite the returned evidence_id in offered_evidence.

submit_decision call:
`{"kind":"tool","tool_name":"submit_argument","payload":{"text":"argument text","offered_evidence":[{"evidence_id":"ev_example","label":"PX-1"}],"technical_reports":[{"title":"Cryptographic verification","summary":"Verified OK."}]}}`
