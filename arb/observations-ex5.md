# Observations For Ex5

## Run

The OpenClaw-lawyer run completed at `out/ex5-openclaw-lawyers-20260602051309`.  AAR exited with status 0 and returned `demonstrated`, with a 4-1 council vote.  Both OpenClaw containers exited with status 0 after completing their assigned filings.

The starting packet contained only `complaint.md~`, and that item was not text-readable through the ordinary manifest path.  The lawyers read the bytes and treated it as a case-definition exhibit rather than event proof.  The final record contained that complaint plus two defendant-submitted evidence extracts.

## Evidence Use

Plaintiff found useful outside source targets during argument, including CRS, CFR, and AP/PBS leads about a January 3, 2026 operation in Venezuela.  It also recorded adverse nuance from those sources, including uncertainty about practical territorial control.  Plaintiff then failed to admit its own sources because it tried to submit evidence by wrapping `submit_evidence` inside `submit_decision`, and AAR rejected the unsupported action type.

Defendant submitted the two source extracts that shaped the record.  The CRS extract reported strikes across Venezuela, the capture and transfer of Maduro and Cilia Flores, prior lethal strikes and vessel seizures, a port-facility drone strike, and President Trump’s statement that the United States would “run” Venezuela until a transition.  The CSIS extract supplied the limiting account: more than 200 U.S. special operations forces in Caracas, battle damage at Fort Tiuna, La Carlota, La Guaira, and El Higuerote, but an operation described as a narrow law-enforcement capture mission rather than a broader campaign against the Venezuelan security apparatus.

The record quality was better than it first appeared because defendant admitted adverse evidence instead of limiting the record to defense-favorable material.  The main weakness is that the record used extracted text rather than full source artifacts, raw imagery, or archived copies.  That weakness did not prevent reasoned council review, but it left the council dependent on lawyer-selected excerpts for the CRS and CSIS source content.

## Advocacy

Plaintiff’s argument was appropriately cautious before defendant submitted evidence.  It told the council that the complaint alone did not prove the event elements and identified outside sources as investigative leads rather than admitted proof.  After defendant admitted CRS and CSIS, plaintiff used those same exhibits well in rebuttal and closing, focusing on the proposition’s “any portion” language, the physical entry into Caracas, the strikes and air-defense suppression, and the announced plan for the United States to run Venezuela during transition.

Defendant made the best available argument from the admitted sources.  It conceded force, entry, capture, and political ambition, then pressed the distinction between a capture raid and an offensive intended to establish territorial control.  Its use of CSIS was serious: the source directly supported the defense theory that the operation was narrow, temporary, and focused on extraction rather than occupation.

The council split on that distinction.  Four members found the combined CRS and CSIS evidence sufficient because the proposition required control over any portion, not permanent occupation or control of the whole country.  One member found the record insufficient because the sources did not show a held, administered, or occupied portion of Venezuelan land.

## Work Notes

The record contains eight `send_work_notes` entries, one for each lawyer turn.  The notes are useful because they record source searches, source-selection decisions, exact evidence reads, adverse checks, failed submission attempts, and the reasoning behind each filing.  The plaintiff notes are also the only place where the unadmitted CRS/CFR/AP/PBS source trail appears in detail.

The notes again show a tool-presentation problem for plaintiff.  In this same run, defendant successfully used `submit_evidence` twice, so the server exposed the capability somewhere in the lawyer path.  Plaintiff nevertheless concluded that no separate upload tool was available and made the recurring invalid wrapper attempt.

## Process Notes

The OpenClaw containers completed the run cleanly.  Both reported a Lawyer API connection refusal only after their final filings had already been accepted.  AAR still closed the case and produced a complete digest, transcript, manifest, and work-notes file.

The evidence-admission failure is now a repeated pattern across source-heavy examples.  It is not a policy issue, since evidence submission is allowed during arguments, rebuttals, and surrebuttals.  The lawyer instructions or MCP tool flow need enough clarity that a lawyer reaches `submit_evidence` directly when the phase permits it.
