# Observations For Ex3

## Run

The OpenClaw-lawyer run completed at `out/ex3-openclaw-lawyers-20260602045023`.  AAR exited with status 0 and returned `demonstrated`, with a 3-2 council vote.  Defendant exited with status 0, while plaintiff exited with status 1 after all plaintiff filings were accepted because OpenClaw reported a compaction failure.

This run differs from ex2 because the starting packet was stronger.  It included the market page, the pre-event reporter-exchange transcript, the governing `PRESMENTION` / `NEWMENTION` terms, and the formal-draw archive note.  The final evidence manifest remained limited to those four case-packet files.

## Evidence Use

Both lawyers read all four packet files through AAR evidence tools during opening.  Plaintiff read all four again during argument before offering them as PX-1 through PX-4.  Defendant had already read the full packet during opening, used metadata checks during argument and later phases, and offered the same four packet items as DX-1 through DX-4.

No lawyer submitted outside evidence.  Plaintiff searched for the exact Kalshi market rules, the transcript source, an Oddschecker market article, and a different Kalshi Trump mention market.  Defendant searched the exact Kalshi URL and ticker, probed Kalshi public API endpoints with `curl`, and checked analogous market and odds pages.  Neither side captured an authoritative exact-market source that changed the record.

Both sides filed technical reports that were source-search ledgers rather than technical analyses.  Those reports were useful because they recorded search scope, failed Kalshi captures, and the decision not to rely on non-admitted material.  They also show a limit in the record: no admitted source hierarchy, start-time rule, formal transcript text, or official resolution note was found.

## Advocacy

Plaintiff made the stronger affirmative case than in ex2.  It used the terms well: event-based time periods may include event/context, `Ronaldo` satisfies the distinct standalone-word rule, and the listed exclusions do not fit a public reporter exchange at the venue.  Plaintiff also used the transcript’s context effectively, pointing to the same venue, Infantino’s presence, the World Cup Draw subject matter, and the immediate relation to the formal program.

Defendant made a serious boundary argument.  It emphasized that the transcript itself labels the reporter exchange as `Prior to` the draw and that the market asks what Trump said `during` the draw.  Defendant’s best point was that absence of a formal-only limitation does not itself prove inclusion, especially when the record lacks the exact market source hierarchy and the formal program transcript.

The council split on how to weigh ambiguity.  The three `demonstrated` votes treated the admitted terms and event-linked context as enough because no admitted rule excluded the remarks.  The two `not_demonstrated` votes treated the `Prior to` label and missing formal-source materials as too important to bridge by inference.

## Work Notes

The record contains eight `send_work_notes` entries, one per lawyer turn.  The notes are detailed and useful: they include full evidence-read logs, source-search logs, tool use, adverse checks, decisions not to submit outside material, and unresolved evidentiary gaps.  They make clear that the lawyers did search; the absence of submitted evidence was a reasoned choice, not a failure to investigate.

The work notes also show improved evidence discipline.  Lawyers did not cite outside URLs as admitted proof after deciding not to submit them.  The argument and closing filings stayed inside admitted evidence, with the search results framed only through technical search-ledger reports.

## Process Notes

The evidence tools worked for reading, metadata checks, offered evidence, and work notes.  I found no rejected legal filings in this run.  The record contains no submitted-evidence entries because neither lawyer found new source material that justified admission beyond the already strong packet.

The post-closure wait problem remains.  Defendant reported a Lawyer API connection refusal after its closing was accepted, and plaintiff exited nonzero from an OpenClaw compaction failure.  AAR closed cleanly and produced the digest, but remote lawyers still do not reliably observe a clean terminal state.

## Clerk API Run 2026-06-04

The Clerk API run completed at `out/clerk-api-ex-runs-20260604032626/arb-20260604032626-48e89f15`.  The run used Codex-auth OpenClaw lawyers and Pi council members from `arb/pool.jsonl`.  It closed with status `ok`, resolution `not_demonstrated`, and a 3-2 council vote.

The starting packet was already substantial: market page, pre-event reporter-exchange transcript, PRESMENTION terms, and the formal-draw archive note.  Both lawyers read the packet during opening.  Plaintiff also submitted two outside evidence items during argument: an American Presidency Project extract for the reporter exchange and a FIFA media-release extract confirming the date and Kennedy Center venue.  Defendant submitted three outside evidence items: a Senate formal-draw transcript extract with no Ronaldo match in accessible text, a Senate reporter-exchange extract describing the red-carpet setting before the FIFA event, and a Roll Call / Factba.se formal-drawing extract.

The evidence work was strong.  Plaintiff strengthened occurrence, venue, and event context rather than relying only on the packet.  Defendant filled the formal-draw archive gap with accessible transcript material and added source-chain detail showing the Ronaldo utterance came from a separate pre-event red-carpet exchange.  That turned the case into a clean boundary dispute rather than a source-custody dispute.

The advocacy reflected that record.  Plaintiff argued that the governing terms did not create a formal-program-only rule and that public red-carpet event coverage was materially different from rehearsals, sound checks, or backstage material.  Defendant argued that plaintiff bore the burden to prove inclusion, and that the admitted sources repeatedly described the Ronaldo exchange as prior to or before the FIFA event while the formal drawing extracts lacked the strike.

The council split tracked the evidence.  Three members voted `not_demonstrated`, treating the `Prior to` and red-carpet-before-event sources as dispositive under the preponderance burden.  Two members voted `demonstrated`, giving more weight to the public same-venue event context and the absence of a clear market exclusion.

The process was clean.  The run recorded five submitted-evidence events, thirteen evidence-read events, eight attorney actions, and five council votes.  `logs/mcp.stderr` showed no rejected MCP calls, and the case recorded no `opportunity_failed` or `council_member_removed` events.  Work notes were present for every lawyer phase.  Each Pi model config preserved the selected provider constraints and wrote `maxTokens: 4096`.
