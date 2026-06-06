
# Example Batch 20260602T005933Z

Driver: Lawyer API for counsel turns; Council API for council votes.  The driver selected local case-packet evidence during argument turns and did not perform web or external-source collection.

## ex1 Run 1

Output: `out/ex1-councilapi-run1-20260602T005933Z`

Status: ok.  Resolution: demonstrated.  Final phase: closed.  Council backend: councilapi.

Record counts: case files 11; visible evidence 11; selected local evidence 9; offered exhibits 18; submitted evidence 0; technical reports 2; council replacements 0; votes demonstrated=5.

Selected evidence:
- printing-invoice.txt (`ev_54c33ae78e7d_printing-invoice`)
- distribution-work-order.txt (`ev_6488cfe4679b_distribution-work-order`)
- confession.txt (`ev_77ec661ab33b_confession`)
- damages-breakdown.txt (`ev_839024ec3b20_damages-breakdown`)
- time-and-token-log.txt (`ev_9d3b588a9864_time-and-token-log`)
- instructions.txt (`ev_b0c7a5192e1c_instructions`)
- print-approval-note.txt (`ev_cbdda31309ff_print-approval-note`)
- session-summary.txt (`ev_fbc123d72aa5_session-summary`)
- deadline-message-thread.txt (`ev_fca019619f19_deadline-message-thread`)

Evidence quality: strong local packet for the narrow dispute.  The packet contains message, work-order, invoice, approval, confession, and damages materials.  The run did not independently verify signatures or cryptographic material, and the driver deliberately excluded key and signature files from offered exhibits.

Offering quality: acceptable for API testing because the offered exhibits are substantive text records from the case packet.  The filings still analyze them shallowly because the driver is scripted rather than an autonomous lawyer.

Procedural observation: all completed lawyer and council API calls returned `ok: true`; review the saved `do-*.json` files for the exact request results.

## ex1 Run 2

Output: `out/ex1-councilapi-run2-20260602T005933Z`

Status: ok.  Resolution: demonstrated.  Final phase: closed.  Council backend: councilapi.

Record counts: case files 11; visible evidence 11; selected local evidence 9; offered exhibits 18; submitted evidence 0; technical reports 2; council replacements 0; votes demonstrated=5.

Selected evidence:
- printing-invoice.txt (`ev_54c33ae78e7d_printing-invoice`)
- distribution-work-order.txt (`ev_6488cfe4679b_distribution-work-order`)
- confession.txt (`ev_77ec661ab33b_confession`)
- damages-breakdown.txt (`ev_839024ec3b20_damages-breakdown`)
- time-and-token-log.txt (`ev_9d3b588a9864_time-and-token-log`)
- instructions.txt (`ev_b0c7a5192e1c_instructions`)
- print-approval-note.txt (`ev_cbdda31309ff_print-approval-note`)
- session-summary.txt (`ev_fbc123d72aa5_session-summary`)
- deadline-message-thread.txt (`ev_fca019619f19_deadline-message-thread`)

Evidence quality: strong local packet for the narrow dispute.  The packet contains message, work-order, invoice, approval, confession, and damages materials.  The run did not independently verify signatures or cryptographic material, and the driver deliberately excluded key and signature files from offered exhibits.

Offering quality: acceptable for API testing because the offered exhibits are substantive text records from the case packet.  The filings still analyze them shallowly because the driver is scripted rather than an autonomous lawyer.

Procedural observation: all completed lawyer and council API calls returned `ok: true`; review the saved `do-*.json` files for the exact request results.

## ex2 Run 1

Output: `out/ex2-councilapi-run1-20260602T005933Z`

Status: ok.  Resolution: demonstrated.  Final phase: closed.  Council backend: councilapi.

Record counts: case files 3; visible evidence 3; selected local evidence 3; offered exhibits 6; submitted evidence 0; technical reports 2; council replacements 0; votes demonstrated=5.

Selected evidence:
- market-question.txt (`ev_74d969c640f7_market-question`)
- formal-draw-program.txt (`ev_7de1d3f2c404_formal-draw-program`)
- pre-event-remarks.txt (`ev_af2671376951_pre-event-remarks`)

Evidence quality: weak as an archival record.  The folder README states that the files are distilled from a draft complaint and are not downloaded source archives.  The offered evidence captures the asserted remarks, the market question, and the absence point about the formal program, but it does not establish primary-source custody.

Offering quality: useful for procedural testing, but the source packet is too derived for a serious merits run.  The right next test would replace these files with captured source text and metadata.

Procedural observation: all completed lawyer and council API calls returned `ok: true`; review the saved `do-*.json` files for the exact request results.

## ex2 Run 2

Output: `out/ex2-councilapi-run2-20260602T005933Z`

Status: ok.  Resolution: demonstrated.  Final phase: closed.  Council backend: councilapi.

Record counts: case files 3; visible evidence 3; selected local evidence 3; offered exhibits 6; submitted evidence 0; technical reports 2; council replacements 1; votes demonstrated=5.

Selected evidence:
- market-question.txt (`ev_74d969c640f7_market-question`)
- formal-draw-program.txt (`ev_7de1d3f2c404_formal-draw-program`)
- pre-event-remarks.txt (`ev_af2671376951_pre-event-remarks`)

Evidence quality: weak as an archival record.  The folder README states that the files are distilled from a draft complaint and are not downloaded source archives.  The offered evidence captures the asserted remarks, the market question, and the absence point about the formal program, but it does not establish primary-source custody.

Offering quality: useful for procedural testing, but the source packet is too derived for a serious merits run.  The right next test would replace these files with captured source text and metadata.

Procedural observation: all completed lawyer and council API calls returned `ok: true`; review the saved `do-*.json` files for the exact request results.

## ex3 Run 1

Output: `out/ex3-councilapi-run1-20260602T005933Z`

Status: ok.  Resolution: demonstrated.  Final phase: closed.  Council backend: councilapi.

Record counts: case files 4; visible evidence 4; selected local evidence 4; offered exhibits 8; submitted evidence 0; technical reports 2; council replacements 0; votes demonstrated=5.

Selected evidence:
- market-page.txt (`ev_2c7c2c8a2d76_market-page`)
- pre-event-reporter-exchange.txt (`ev_9cd6fa5297f3_pre-event-reporter-exchange`)
- presmention-terms.txt (`ev_a373371a8c9c_presmention-terms`)
- formal-draw-archive-note.txt (`ev_eac0f81b7818_formal-draw-archive-note`)

Evidence quality: materially better than ex2.  The packet includes Kalshi terms, a market-page note, a pre-event reporter exchange, and an archive note explaining the unavailable formal-program transcript.  The formal transcript gap remains important, but the record is honest about the retrieval limit.

Offering quality: good for local-packet testing.  The offered evidence frames both the affirmative source text and the governing market-language question.

Procedural observation: all completed lawyer and council API calls returned `ok: true`; review the saved `do-*.json` files for the exact request results.

## ex3 Run 2

Output: `out/ex3-councilapi-run2-20260602T005933Z`

Status: ok.  Resolution: demonstrated.  Final phase: closed.  Council backend: councilapi.

Record counts: case files 4; visible evidence 4; selected local evidence 4; offered exhibits 8; submitted evidence 0; technical reports 2; council replacements 1; votes demonstrated=5.

Selected evidence:
- market-page.txt (`ev_2c7c2c8a2d76_market-page`)
- pre-event-reporter-exchange.txt (`ev_9cd6fa5297f3_pre-event-reporter-exchange`)
- presmention-terms.txt (`ev_a373371a8c9c_presmention-terms`)
- formal-draw-archive-note.txt (`ev_eac0f81b7818_formal-draw-archive-note`)

Evidence quality: materially better than ex2.  The packet includes Kalshi terms, a market-page note, a pre-event reporter exchange, and an archive note explaining the unavailable formal-program transcript.  The formal transcript gap remains important, but the record is honest about the retrieval limit.

Offering quality: good for local-packet testing.  The offered evidence frames both the affirmative source text and the governing market-language question.

Procedural observation: all completed lawyer and council API calls returned `ok: true`; review the saved `do-*.json` files for the exact request results.

## ex4 Run 1

Output: `out/ex4-councilapi-run1-20260602T005933Z`

Status: ok.  Resolution: demonstrated.  Final phase: closed.  Council backend: councilapi.

Record counts: case files 1; visible evidence 1; selected local evidence 0; offered exhibits 0; submitted evidence 0; technical reports 2; council replacements 1; votes demonstrated=5.

Selected evidence:

Evidence quality: poor for adjudication.  The example supplies market language and a Polymarket rationale in the README, but no separate source packet containing official confirmation or credible reporting.  The driver did not offer README material as evidence because it is setup prose rather than captured source text.

Offering quality: intentionally empty.  The run tests procedure, not evidentiary sufficiency.

Procedural observation: all completed lawyer and council API calls returned `ok: true`; review the saved `do-*.json` files for the exact request results.

## ex4 Run 2

Output: `out/ex4-councilapi-run2-20260602T005933Z`

Status: ok.  Resolution: demonstrated.  Final phase: closed.  Council backend: councilapi.

Record counts: case files 1; visible evidence 1; selected local evidence 0; offered exhibits 0; submitted evidence 0; technical reports 2; council replacements 0; votes demonstrated=5.

Selected evidence:

Evidence quality: poor for adjudication.  The example supplies market language and a Polymarket rationale in the README, but no separate source packet containing official confirmation or credible reporting.  The driver did not offer README material as evidence because it is setup prose rather than captured source text.

Offering quality: intentionally empty.  The run tests procedure, not evidentiary sufficiency.

Procedural observation: all completed lawyer and council API calls returned `ok: true`; review the saved `do-*.json` files for the exact request results.

## ex5 Run 1

Output: `out/ex5-councilapi-run1-20260602T005933Z`

Status: ok.  Resolution: not_demonstrated.  Final phase: closed.  Council backend: councilapi.

Record counts: case files 1; visible evidence 1; selected local evidence 0; offered exhibits 0; submitted evidence 0; technical reports 2; council replacements 1; votes not_demonstrated=5.

Selected evidence:

Evidence quality: poor for adjudication.  The folder has market terms in the README but no source packet addressing whether the event occurred before expiration.  The claim is time-sensitive and source-dependent, so this example needs contemporaneous official or credible-source records.

Offering quality: intentionally empty.  A serious run needs external source collection or a prepared evidence packet.

Procedural observation: all completed lawyer and council API calls returned `ok: true`; review the saved `do-*.json` files for the exact request results.

## ex5 Run 2

Output: `out/ex5-councilapi-run2-20260602T005933Z`

Status: ok.  Resolution: not_demonstrated.  Final phase: closed.  Council backend: councilapi.

Record counts: case files 1; visible evidence 1; selected local evidence 0; offered exhibits 0; submitted evidence 0; technical reports 2; council replacements 1; votes not_demonstrated=5.

Selected evidence:

Evidence quality: poor for adjudication.  The folder has market terms in the README but no source packet addressing whether the event occurred before expiration.  The claim is time-sensitive and source-dependent, so this example needs contemporaneous official or credible-source records.

Offering quality: intentionally empty.  A serious run needs external source collection or a prepared evidence packet.

Procedural observation: all completed lawyer and council API calls returned `ok: true`; review the saved `do-*.json` files for the exact request results.

## ex6 Run 1

Output: `out/ex6-councilapi-run1-20260602T005933Z`

Status: ok.  Resolution: not_demonstrated.  Final phase: closed.  Council backend: councilapi.

Record counts: case files 0; visible evidence 0; selected local evidence 0; offered exhibits 0; submitted evidence 0; technical reports 2; council replacements 1; votes not_demonstrated=5.

Selected evidence:

Evidence quality: poor for adjudication.  The complaint contains the event blueprint, and the README only identifies the blueprint source.  There is no packet containing DoD, White House, UN, or credible-source records for the resolution criteria.

Offering quality: intentionally empty.  The run tests procedure and final-state handling, not evidence collection.

Procedural observation: all completed lawyer and council API calls returned `ok: true`; review the saved `do-*.json` files for the exact request results.

## ex6 Run 2

Output: `out/ex6-councilapi-run2-20260602T005933Z`

Status: ok.  Resolution: not_demonstrated.  Final phase: closed.  Council backend: councilapi.

Record counts: case files 0; visible evidence 0; selected local evidence 0; offered exhibits 0; submitted evidence 0; technical reports 2; council replacements 1; votes not_demonstrated=5.

Selected evidence:

Evidence quality: poor for adjudication.  The complaint contains the event blueprint, and the README only identifies the blueprint source.  There is no packet containing DoD, White House, UN, or credible-source records for the resolution criteria.

Offering quality: intentionally empty.  The run tests procedure and final-state handling, not evidence collection.

Procedural observation: all completed lawyer and council API calls returned `ok: true`; review the saved `do-*.json` files for the exact request results.

## Batch-Level Observations

All twelve runs completed and wrote closed `run.json` files.  No saved `do-*.json` response had `ok: false`.  The new event timestamp format appeared in the generated `events.ndjson` files as UTC millisecond timestamps such as `2026-06-02T00:59:40.171Z`.

The main implementation observation remains council preflight.  Seven of the twelve runs replaced one sampled council model before the HTTP Council API began accepting council votes.  That behavior is procedurally safe because replacements are recorded, but it is still the wrong dependency for a future remote-council design: a remote council member should not require the local runner to preflight unrelated model availability.

The evidence observation is sharper than the procedural result.  `ex1` has a strong local evidence packet for a closed-record test.  `ex3` is the best Ronaldo example because it includes governing market terms, source text, and an honest note about the unavailable formal transcript.  `ex2` is useful only as a normalized draft packet because its README says the files are distilled from a complaint rather than captured source archives.  `ex4`, `ex5`, and `ex6` are not evidence-complete examples; they need source packets before their merits results should be treated as meaningful.

The driver offered the same selected local evidence from both sides during argument turns.  That exercised API acceptance and council visibility, but it is not good advocacy behavior.  A real lawyer client should select and discuss evidence differently for each side, and should submit new evidence or technical reports only when it has actual source material and analysis to add.
