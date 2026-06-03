# Observations For Ex4

## Run

The OpenClaw-lawyer run completed at `out/ex4-openclaw-lawyers-20260602132824`.  AAR exited with status 0 and closed the case as `demonstrated`.  The council vote was 3-2: C1, C3, and C4 voted `demonstrated`, while C2 and C5 voted `not_demonstrated`.

The stable MCP tool-set experiment worked for this run.  The MCP log contains no `ok=false` lawyer API calls and no 4xx or 5xx forwarded lawyer calls.  Lawyers did not attempt to submit evidence in openings or closings, and the evidence submissions occurred only during argument opportunities.

Plaintiff's OpenClaw container exited with status 1 after its accepted closing.  The failure was an OpenClaw/Codex compaction error, not a rejected AAR filing.  Defendant exited with status 0 but reported a post-closing `wait_failed` because the local Lawyer API had shut down after the case closed.

## Evidence Use

The starting packet was sparse.  The only case-packet evidence was `README.md~`, which contained the market rules and did not prove the historical event.  Both lawyers read that file in opening and treated it as rule evidence rather than merits evidence.

Plaintiff submitted three official UN text extracts during argument.  The strongest was the December 13 UN noon briefing, which reported UNDOF observations of IDF movements within the area of separation, IDF presence in multiple locations, and UNDOF's notice that the actions violated the 1974 Disengagement Agreement.  Plaintiff also submitted December 12 and December 19 UN Secretary-General materials addressing Syria's sovereignty, unauthorized presence, and continued IDF personnel and equipment in the area of separation and limitation.

Defendant submitted an official Israeli letter to the UN Security Council, `S/2024/887`, from the UN Digital Library.  That evidence was useful because it admitted a temporary IDF deployment east of Line A while framing the deployment as limited, defensive, and tied to armed groups entering the area of separation and threatening UNDOF.  Defendant found the PDF, downloaded it, checked the file size and hash, installed `pdf-parse` in a temporary npm workspace, extracted text, and submitted the readable extract as direct evidence.

The final evidence set was narrow but adequate for the dispute.  It contained official UN sources on presence, duration, treaty violation, and sovereignty language, plus an official Israeli source on intent and the defensive explanation.  The main missing evidence was a precise map or coordinate source tying the "few points" east of Line A to territory the market counts as Syria despite the Golan Heights carveout.

## Advocacy

Plaintiff made a coherent Yes case from the official evidence.  It argued that the Israeli letter supplied the location admission, while the UN evidence supplied multiplicity of locations, continued presence, and unauthorized control.  It handled the defensive and temporary language by arguing that those labels explain motive but do not negate an intent to control specific points.

Defendant made a coherent No case from the same record.  It conceded presence and violation, then focused on the proposition's narrower language: a military offensive intended to establish control over a portion of Syria.  It used the Israeli letter to argue that the admitted evidence showed a temporary security deployment rather than a market-resolving offensive.

The council split for the right reason.  The `demonstrated` votes treated deployment east of Line A, multiple-location presence, and obstacle construction as practical control under a preponderance standard.  The `not_demonstrated` votes treated the same facts as insufficient to prove offensive intent because the official Israeli source framed the action as limited, temporary, and defensive.

## Work Notes

The run contains eight `send_work_notes` entries, one for each lawyer turn.  The notes are kept at `out/ex4-openclaw-lawyers-20260602132824/case/work-notes.ndjson`.  They include issue breakdowns, search terms, source URLs, evidence reads, PDF extraction attempts, tool failures, adverse checks, and filing decisions.

The work notes are useful for auditing evidence behavior.  Plaintiff's argument notes show that it searched official UN, UNDOF, Security Council, and Israeli materials, submitted the three UN extracts, and found the Israeli `S/2024/887` PDF as adverse context before defendant admitted it.  Defendant's notes show exact record reads, search paths, PDF-tool failures, the temporary npm extraction path, and a failed attempt to capture an official State Department source.

## Process Notes

The stable MCP tool list removed the earlier failure mode where plaintiff saw evidence submission instructions but failed to call a direct `submit_evidence` tool.  The prompt now tells lawyers that the MCP adapter exposes stable transport tools and that the opportunity prompt controls which court actions may affect the record.  This run supports that design: the lawyers read the opportunity rules and used `submit_evidence` directly when allowed.

The end-of-case behavior still needs attention.  When AAR closes the case and the Lawyer API stops, an OpenClaw lawyer that calls `wait_for_opportunity` again can receive a connection-refused error rather than a clean `done` state.  That did not affect the completed case artifacts, but the remote-lawyer experience should end with an explicit final status instead of a transport failure.

The OpenClaw container environment still lacks useful default PDF tooling.  Defendant worked around missing `pdftotext`, missing Python PDF libraries, and package-install permission problems by installing `pdf-parse` with npm.  That was effective in this run, but source-heavy cases should not depend on a lawyer discovering a one-off extraction path under turn pressure.
