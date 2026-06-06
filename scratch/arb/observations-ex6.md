# Observations For Ex6

## Clerk API Run, 2026-06-04

The Clerk API run completed at `arb/out/clerk-api-ex-runs-20260604042542/arb-20260604042542-970d2ea4`.  AAR closed the case as `not_demonstrated` after a 5-0 council vote.  The selected Pi seats used `pool.jsonl` request specs with provider restrictions, quantization where specified, and persona files preserved in the run artifacts.

The case started with no usable automatic case files, so the lawyers had to build the record from outside sources.  This run differs from the older run below because plaintiff successfully submitted five evidence items during arguments, and defendant submitted two.  The source-heavy path therefore worked for both roles.

Plaintiff found the right source classes but did not always preserve the strongest form of the evidence.  The White House Rubio release and the OMB statement of administration policy were strong official sources for physical entry, timing, combat capacity, and official characterization.  The GlobalSecurity Pentagon News mirror carried the strongest transition and oversight facts, while the official Defense report entered the record mainly as an endnote and source-chain extract rather than full body text.

Defendant submitted stronger official-source extracts from the same White House and EOP materials.  Those extracts emphasized the two-hour ground presence, capture, arrest, prosecution framing, denial of invasion or extended operation, and the statement that Maduro was not the head of state.  The defense evidence narrowed the dispute to objective rather than physical entry or military capacity.

The advocacy was focused.  Plaintiff argued that the record proved physical entry in combat capacity and that the operation removed the sitting government or produced U.S.-directed transition oversight.  Defendant argued that the proposition's edge case controlled because the official sources described a capture and arrest operation without territorial occupation or city control.

The process behavior was mostly clean.  There was one rejected plaintiff `submit_decision` call during rebuttal in `logs/mcp.stderr`, after which plaintiff submitted the rebuttal successfully.  Both lawyers submitted work notes for all turns, and those notes show source searches, official-source prioritization, adverse checks, evidence reads, and filing decisions.

Council review was uneven.  C2 and C4 read admitted evidence during deliberation, while C1, C3, and C5 had no recorded evidence-read events and appear to have relied on the prompt's case record and lawyer filings.  The final result is supported by the record, but direct evidence reads should be more consistent across council members.

The event timestamps show large wall-clock gaps, including gaps that at first made some accepted turns look as though they exceeded the 900-second turn timeout.  I checked `runtime.json`, the Lawyer API timeout code, and MCP responses with `remaining_ms`; this run does not establish an AAR timeout failure.  The evidence points to host wall-clock jumps during the live command, so these event timestamps should be treated as wall-time labels rather than proof of turn-budget behavior.

## Run

The OpenClaw-lawyer run completed at `out/ex6-openclaw-lawyers-20260602052518`.  AAR exited with status 0 and returned `not_demonstrated`, with a 4-1 council vote.  Both OpenClaw containers exited with status 0 after completing all assigned filings.

The initial AAR evidence list was empty.  The complaint text existed as the case proposition, but there were no case-packet evidence items, no hashes, and no source files for the lawyers to read at opening.  The final record contained two defendant-submitted official-source extracts.

## Evidence Use

Plaintiff searched the right source class during opening and argument: White House pages, DoD or Executive Office materials, and UN Security Council leads.  In argument, it found White House and OMB source leads that matched the proposition’s official-source requirement.  It failed to admit those sources because it again tried to reach `submit_evidence` through `submit_decision`, and AAR rejected that call as an unsupported action type.

Defendant submitted the decisive sources.  The first exhibit was a White House release preserving Rubio statements that U.S. forces were on the ground in Venezuela for about two hours, that the operation was not an invasion or extended military operation, and that the event was a raid, capture, and arrest.  The second exhibit was an EOP/OMB statement on S.J. Res. 98 stating that U.S. Armed Forces conducted targeted and limited military strikes within Venezuela in furtherance of apprehending and transporting Maduro and Cilia Flores for federal criminal prosecution.

The evidence quality was good enough for the blueprint question because both exhibits came from official White House or Executive Office sources, were text-readable, had source URLs, retrieval timestamps, and hashes, and addressed the disputed elements directly.  The main evidentiary limitation is that both submissions were extracted text rather than full source artifacts.  No UN Security Council record or DoD source entered the record, so the fallback and conflict provisions never became active.

## Advocacy

Plaintiff handled the empty early record correctly by separating admitted evidence from source leads.  After defendant admitted the official materials, plaintiff used them well for the physical-entry and combat-capacity elements: ground presence in Venezuela, armed resistance expected, U.S. Armed Forces used because ordinary law enforcement could not reach the targets, and military strikes within Venezuela.  Its weakest move was the argument that Maduro’s capture qualified as removing the sitting government despite the express edge case for capture of a head of state without territorial occupation.

Defendant made a strong text-bound argument from the admitted sources.  It conceded physical entry and force, then focused on objective: two-hour ground presence, targeted and limited strikes, apprehension and transport for prosecution, no extended operation, and no source showing seizure, occupation, city control, or installation of a replacement government.  It also used the White House statement that Maduro was not the head of state to attack plaintiff’s “sitting government” theory.

The council majority accepted the defense framing.  Four members treated the case as a brief capture operation excluded by the proposition’s edge case.  One member voted `demonstrated`, reasoning that physical entry, use of force, and removal of Maduro satisfied the event definition even without extended occupation.

## Work Notes

The record contains eight `send_work_notes` entries.  The notes are useful because they show source searches, official-source selection, exact source URLs, admitted-evidence reads, adverse checks, and failed submission attempts.  Plaintiff’s argument notes are especially useful because they preserve its unadmitted White House and OMB leads and explain why no plaintiff exhibits entered the record.

The notes also show two distinct lawyer-tool issues.  Plaintiff could not discover or invoke `submit_evidence` directly during arguments and made the recurring invalid wrapper attempt.  Defendant made a different invalid attempt in surrebuttal by offering evidence where only argument text was allowed, then corrected it and resubmitted text only.

## Process Notes

The evidence-submission path worked for defendant, including two official-source submissions, later reads by both sides, and exhibit use in argument and rebuttal.  The path did not work for plaintiff in the same case, which points away from a server-side ban on evidence submission and toward tool discovery, session behavior, or role-specific interaction with the MCP tool list.  The recurring plaintiff failure is now the clearest process defect in these source-heavy examples.

The post-closure wait problem remains.  After accepted closings, both OpenClaw containers reported a local Lawyer API connection refusal while polling final status.  That did not corrupt the record: AAR closed the case, wrote the digest, manifest, transcript, events, and work notes, and all three exit files contain status 0.
