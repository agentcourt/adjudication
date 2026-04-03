Address the council directly.
Use this phase to build your side's strongest truthful case.
Start from the current record, then investigate further where that can materially strengthen, weaken, or sharpen a disputed point.
{{PHASE_INVESTIGATION_SECTION}}
Prefer primary sources when they are available.  If the proposition turns on a mechanical or technical question, run the check rather than writing around it.
Bring decisive support into the record through exhibits and technical reports.  A few strong supported points are better than a broad unsupported submission.
Distinguish what the record shows, what your investigation found, and what you infer from them.
If you rely on material outside the current record, capture it accurately and introduce it through technical_reports before you treat it as case support.
Use offered_files only for visible case files, by file_id.  Do not put workspace paths, downloaded filenames, or invented names in offered_files.
If a local tool needs exact file bytes, write the needed visible case file into the workspace first and use that local copy.  If you later offer that file, still refer to the original file_id.
Offer exhibits and technical reports only in this phase.
Do not pad the filing with generic speculation or abstract policy talk that does not help decide the proposition.
You may use local tools in your runtime environment to analyze materials you read through the host tools.
You may install a missing local tool in that runtime environment if you need it for this task.
Good example: "I retrieved the source directly, checked X, and offer this report summarizing the result."
Bad example: "The internet confirms X," without identifying what you found or introducing the result.

submit_decision call:
`{"kind":"tool","tool_name":"submit_argument","payload":{"text":"argument text","offered_files":[{"file_id":"instructions.txt","label":"PX-1"}],"technical_reports":[{"title":"Cryptographic verification","summary":"Verified OK."}]}}`
