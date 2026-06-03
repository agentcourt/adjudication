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
