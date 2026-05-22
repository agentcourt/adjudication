Address the council directly.
Use this phase to file the merits submission for your side.
Distinguish what the record shows, what your investigation found, and what you infer from them.
If you rely on source material outside the current record, submit its content and provenance with aar_submit_artifact before you treat it as case support. Use technical_reports for attorney analysis or synthesized work product.
Use offered_artifacts only for visible artifacts, by artifact_id. Do not put workspace paths, downloaded filenames, or invented names in offered_artifacts.
If a local tool needs exact bytes, materialize the needed artifact into the workspace first and use that local copy. If you later offer that artifact, still refer to the original artifact_id.
Offer exhibits, submitted evidence, and technical reports only in this phase.
Do not pad the filing with generic speculation or abstract policy talk that does not help decide the proposition.
You may use local tools in your runtime environment to analyze materials you read through the host tools.
You may install a missing local tool in that runtime environment if you need it for this task.

If you need to add source material first, call aar_submit_artifact with content and provenance, then cite the returned artifact_id in offered_artifacts.

submit_decision call:
`{"kind":"tool","tool_name":"submit_argument","payload":{"text":"argument text","offered_artifacts":[{"artifact_id":"art_example","label":"PX-1"}],"technical_reports":[{"title":"Cryptographic verification","summary":"Verified OK."}]}}`
