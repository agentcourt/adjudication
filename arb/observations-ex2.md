# Observations For Ex2

## Run

The OpenClaw-lawyer run completed at `out/ex2-openclaw-lawyers-20260602044001`.  AAR, the plaintiff container, and the defendant container all exited with status 0.  The result was `not_demonstrated`, with a 5-0 council vote.

This run materially improved on the earlier ex2 run because the lawyers searched beyond the initial packet.  The original packet contained three files: `market-question.txt`, `formal-draw-program.txt`, and `pre-event-remarks.txt`.  The final evidence record contained those three files plus two defendant-submitted GovInfo transcript excerpts.

## Evidence Use

Both lawyers read all three packet files during opening.  Plaintiff used the packet to frame the case as an event-boundary dispute and then searched the web during argument.  Its work notes record useful source leads: a TN Argentina report, a Rev formal-program transcript, a CNN transcript, an Oddschecker market article, and a related Polymarket/PAM page.

Plaintiff did not add outside evidence to the record.  The notes say plaintiff tried to submit the TN source by wrapping `submit_evidence` inside `submit_decision`, and AAR rejected that malformed action as an unsupported `submit_decision` action type.  Defendant later used `submit_evidence` correctly and AAR accepted two GovInfo excerpts during defendant argument.

Defendant’s evidence work was strong.  It found official GovInfo Daily Compilation transcripts for the reporter exchange before the draw and the later formal drawing remarks.  The first transcript proved the Ronaldo remarks but classified them as remarks “Prior to” the draw at 11:13 a.m.; the second transcript covered the formal World Cup Drawing at about 12:50 p.m. and included a no-Ronaldo search note.

The submitted GovInfo excerpts changed the case.  Before submission, the packet had only draft-complaint summaries and no upstream source custody.  After submission, the record had official-source evidence proving both the utterance and the timing boundary that favored the defense.

## Work Notes

The record contains nine `send_work_notes` entries because defendant sent a second surrebuttal note after correcting a rejected filing.  The notes are useful and detailed: they include issue breakdowns, search logs, source leads, source-quality analysis, tool use, evidence-submission results, adverse checks, and unresolved gaps.  Plaintiff’s notes are especially useful for evaluating lost opportunities because they show outside sources discovered but not admitted.

The notes show a process weakness in the lawyer-facing tool flow.  Plaintiff believed the prompt allowed evidence submission but attempted it through the wrong tool shape, and the persisted MCP log records only `ok=false` for the failed `submit_decision`.  The work-note text preserves the failure reason here, but post-run audit should not depend on the lawyer voluntarily recording the error.

## Advocacy

Plaintiff’s advocacy was coherent but weaker after the GovInfo evidence entered the record.  It argued that the market named an event rather than a formal transcript and that GovInfo’s “Prior to” title was chronology rather than market-resolution language.  Plaintiff also made fair concessions: formal drawing material contained no Ronaldo hit, the official title was adverse, and the record still lacked exact market rules.

Defendant made the stronger evidentiary argument.  It did not contest that Trump said Ronaldo; it used the official transcript separation to argue that plaintiff had proved Ronaldo-before-draw rather than Ronaldo-during-draw.  The closing kept the burden point clean: the absence of a formal-transcript-only rule was not affirmative proof that the market’s word `during` included the prior reporter exchange.

The council accepted the defense theory unanimously.  Each vote treated the GovInfo separation between prior exchange and formal drawing as the decisive record fact.  The missing market rule or platform guidance hurt plaintiff because the proposition required expanding `during` backward to pre-program remarks.

## Process Notes

The evidence tools worked for reading, offering, and defendant evidence submission.  Argument and rebuttal filings used valid AAR evidence identifiers.  Surrebuttal correctly rejected offered evidence, and defendant corrected the filing.

The post-closure wait behavior remains noisy.  Both OpenClaw final messages reported a Lawyer API connection refusal after all filings were accepted and the case closed.  The process exits were still 0, but a long-running remote lawyer should receive a stable terminal response rather than a refused connection after AAR exits.

## Run 2026-06-02 14:47

The rerun completed at `out/ex2-openclaw-lawyers-20260602144700`.  AAR exited with status 0 and returned `not_demonstrated`, with a 5-0 council vote.  Both OpenClaw containers exited with status 1 after the case closed because OpenClaw reported a CLI compaction error for `openai-codex/gpt-5.5`; the case itself had already reached final judgment and wrote complete results.

This run fixed the main evidence failure from the earlier ex2 run.  Plaintiff submitted three outside evidence items during argument: the Polymarket market-rules capture, a CNN transcript capture for event timing and context, and a Yahoo Sports/Foot Africa capture reporting the Ronaldo remarks and pointing to an RCN red-carpet source chain.  Defendant submitted two outside evidence items during argument: a GovInfo/DCPD formal drawing transcript and a Senate/CQ Factbase red-carpet reporter transcript.

The evidence record was useful but still imperfect.  The strongest plaintiff evidence was the market-rules capture because it supplied the actual resolution language: scheduled event, scheduled appearance, Q&A counting if the event contains one, exclusion of outside comments, and official English-language FIFA stream as the resolution source.  The strongest defense evidence was the Senate/CQ transcript because it both confirmed the Ronaldo utterance and characterized the red-carpet exchange as before the FIFA event, with video and audio pieced together from non-FIFA sources.

Plaintiff's outside search improved.  It found the governing market language, a contemporaneous CNN context transcript, and a secondary report that corroborated the Ronaldo remarks.  Plaintiff also preserved its source-chain limitations in the submitted evidence text, which made the record more honest: no official FIFA stream, no primary RCN clip, and no exact official-stream timestamp.

Defendant's search was stronger on source weight.  It found a government transcript of the formal drawing segment and a direct transcript source for the red-carpet exchange.  The defense did not pretend that Trump never said Ronaldo; it argued that the admitted sources proved Ronaldo in a pre-event red-carpet scrum, not in the market's official English FIFA stream or formal scheduled event.

The filings used the admitted evidence well.  Plaintiff offered six exhibits in argument and four in rebuttal, including the rules capture, the CNN context capture, the Yahoo/Foot Africa capture, the packet files, and defendant's later red-carpet transcript.  Defendant offered five exhibits in argument, including plaintiff's rules capture, the formal-program packet, the GovInfo formal transcript, the Senate/CQ red-carpet transcript, and the pre-event packet.

The work notes were complete and valuable.  The record contains nine work-note entries: one for every lawyer phase, plus a second defendant surrebuttal note after a rejected filing.  The notes include issue breakdowns, exact evidence reads, search queries, source choices, adverse-source checks, unresolved gaps, and filing plans.

One lawyer-action problem remains.  Defendant's first surrebuttal attempt included offered evidence, and AAR rejected it because offered evidence is allowed only in arguments and rebuttals.  Defendant corrected the filing on the next attempt, so the case was not harmed, but the prompt or tool response should make phase limits unmistakable at the moment of filing.

The council decision tracked the record.  All five members voted `not_demonstrated` because the rules named the official English FIFA stream and excluded comments outside the scheduled event, while the Ronaldo mentions came from a red-carpet transcript that identified the exchange as before the FIFA event.  The result was not caused by weak advocacy; both lawyers found and used the evidence needed to put the real issue before the council.

## Clerk API Run 2026-06-03

The Clerk API run completed at `out/clerk-api-ex-runs-20260603202508/arb-20260603202508-0a98d531`.  The service started a full `aar run` child, used Codex `auth.json` for both OpenClaw lawyers, and used the local `arb/pool.jsonl` through an absolute path for Pi council members.  The case closed with status `ok`, resolution `not_demonstrated`, and a 4-1 council vote.

Plaintiff performed real outside-source work during argument.  It submitted a GovInfo Daily Compilation transcript excerpt proving that Trump said `Ronaldo`, a Kalshi PRESMENTION/NEWMENTION terms excerpt, and a market-page capture identifying the Kalshi Ronaldo strike.  The plaintiff work notes recorded the search path, source choices, adverse Polymarket lead, and the remaining gap that no official FIFA stream clip had been captured.

Defendant answered with stronger timing and scope evidence.  It submitted an American Presidency Project transcript page that labeled the Ronaldo exchange as prior to the FIFA World Cup Draw and fixed the time at 11:13 a.m.  It also submitted a FIFA official announcement capture showing the Final Draw scheduled for 12:00 ET at the Kennedy Center, which gave the council a concrete before-versus-during boundary.

The advocacy was sound on both sides.  Plaintiff conceded the adverse timing record and argued that the market used an event/context period rather than a noon-only formal-program boundary.  Defendant conceded occurrence and focused on burden, the `prior to` transcript label, the 47-minute gap before the scheduled draw, the formal-program no-Ronaldo point, and the absence of market-specific inclusion language for pre-event reporter exchanges.

The council decision tracked the admitted record.  Four members voted `not_demonstrated` because the plaintiff proved occurrence but did not prove that pre-event remarks counted as occurring during the draw.  One member voted `demonstrated` because it gave more weight to the same-venue public-event context and the flexible Kalshi event-period language.

The process also exposed a Pi council instruction defect.  Council member C3 voted successfully, then kept polling and later attempted a C5 opportunity, which AAR rejected.  The case outcome was unaffected because C5 voted correctly, but the Pi council instruction template now tells a member to stop after an accepted `submit_council_vote` call.

## Clerk API Rerun 2026-06-03

The rerun completed at `out/clerk-api-ex-runs-20260603204530/arb-20260603204530-119262f1`.  It used the same Clerk path, Codex-auth OpenClaw lawyers, and Pi council pool configuration.  The case closed with status `ok`, resolution `not_demonstrated`, and a unanimous council vote.

The evidence record again showed real lawyer search.  Plaintiff submitted a Polymarket Gamma API record with the governing Ronaldo market rule, a Senate transcript of reporter-facing remarks containing the Ronaldo statements, and a Senate transcript of the formal ceremony that did not contain Ronaldo.  Defendant submitted an official GovInfo/Daily Compilation transcript showing the Ronaldo exchange occurred at 11:13 a.m. and was titled as prior to the FIFA World Cup Draw.

The rerun improved the issue framing.  The admitted Polymarket rule made the scheduled event and official English-language FIFA stream central, so the council did not have to decide from a generic Kalshi event-period record.  Plaintiff still searched for an official FIFA-stream/red-carpet capture during rebuttal and recorded that it did not find one, which made the unresolved source gap explicit.

The council process behaved correctly after the Pi instruction change.  The MCP log contained no rejected council calls, and `events.ndjson` contained no `opportunity_failed` or `council_member_removed` events.  Each Pi model config contained `maxTokens: 4096` and preserved the selected provider and quantization fields from the pool entry.

Two council preflight replacements occurred before the case initialized because initial selected models timed out during availability checks.  The replacements were recorded in the run initialization events and the final roster used concrete provider constraints from `pool.jsonl`.  The final deliberation proceeded without council-agent failure.
