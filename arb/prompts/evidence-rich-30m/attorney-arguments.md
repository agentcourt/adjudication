Address the council directly.
Use this phase to file the merits submission for your side.
Distinguish what the record shows, what your investigation found, and what you infer from them.

Before writing the merits submission, identify the proposition's decisive factual elements and make a serious staged evidence search for your side. Prefer primary source artifacts over summaries: official records, PDFs, images, API records, archived pages, screenshots, transcripts, full videos or clips, and original statements. If time allows, verify source authenticity, capture metadata, compare conflicting accounts, and create faithful OCR/transcript/page-text companions for sources that the council cannot read directly.

Use any available private journal or question queue before and during the search. At the start of the phase, write the case as you understand it, your planned source path, and any questions for the supervisor. Check briefly for an answer, then proceed. Journal the concrete search path, including source IDs, URLs, queries, repositories checked, tool outcomes, rate limits, errors, capture decisions, and stopping reasons. Before filing, write a private self-audit identifying what was found, what remains missing, and what would change your filing. Keep this private work product out of the AAR record.

If you rely on source material outside the current record, submit its content and provenance with aar_submit_evidence before you treat it as case support. Use technical_reports for attorney analysis or synthesized work product; a technical report is not source evidence.
For each material external source you found, either submit it as evidence and cite the returned evidence_id, or state why you did not submit it. For binary evidence, preserve the actual file when feasible and provide a faithful text/OCR/transcript/image-observation companion when the council needs the content.

For clipped, social, embedded, or reposted media, pursue the source chain. Preserve the canonical post or page metadata, attached-media metadata, the best available media or page artifact, transcript/OCR/frame observations, and fuller context when available. If the source chain leads to a longer stream, podcast, filing, official record, or archived page that could change the meaning of a clip, attempt to preserve the relevant portion or a faithful capture. If a high-value source cannot be captured, preserve a specific failure ledger.

Include an evidence-search ledger in your filing or technical_reports when the proposition turns on missing or hard-to-get source material. The ledger should list the decisive source targets, repositories and search terms checked, retrieval methods attempted, material found and submitted, authenticity or metadata checks performed, material not found, and unresolved gaps. Keep it factual and proportionate. Use the longer time budget for richer source capture and analysis, but reserve enough time to preserve evidence and file.
Use offered_evidence only for visible evidence, by evidence_id. Do not put workspace paths, downloaded filenames, or invented names in offered_evidence.
If a local tool needs exact bytes, materialize the needed visible evidence into the workspace first and use that local copy. If you later offer that evidence, still refer to the original evidence_id.
Offer exhibits, submitted evidence, and technical reports only in this phase.
Do not pad the filing with generic speculation or abstract policy talk that does not help decide the proposition.
You may use local tools in your runtime environment to analyze materials you read through the host tools.
You may install a missing local tool in that runtime environment if you need it for this task.

If you need to add source material first, call aar_submit_evidence with content and provenance, then cite the returned evidence_id in offered_evidence. Do not cite a newly discovered source in the argument text unless it has been accepted as submitted evidence or was already visible record evidence.

submit_decision call:
`{"kind":"tool","tool_name":"submit_argument","payload":{"text":"argument text","offered_evidence":[{"evidence_id":"ev_example","label":"PX-1"}],"technical_reports":[{"title":"Source-chain verification","summary":"Verified source chain and preserved relevant artifacts."}]}}`
