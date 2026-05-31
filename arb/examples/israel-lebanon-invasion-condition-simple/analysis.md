# Israel-Lebanon invasion condition case preparation journal

## 2026-05-01 — Initial orientation

Observe:

- Read the local persona and user guidance, recent memory notes, and the prediction-market-arbitration skill.
- Confirmed the requested task is case-packet preparation only. No arbitration runs and no git commits are authorized.
- Reviewed the existing Zelenskyy suit case packet as the closest format precedent.
- Reviewed the existing `pmdisputes` Israel-Lebanon dispute note. It preserves the core Polymarket rule text and the observed sequence: proposed Yes, disputed, proposed No, disputed, final No.

Think:

- The correct proposition is the direct factual condition supplied by the requester, not market-resolution correctness.
- The evidence must distinguish two issues: whether Israeli forces entered Lebanon or commenced ground operations by the deadline, and whether those operations were intended to establish control over any portion of Lebanon.
- Source hierarchy matters because the market rule allowed official confirmation by Lebanon, Israel, the United Nations, or any permanent UNSC member, plus consensus credible reporting.

Do the right thing:

- Created the target case directory and `source-captures/`.
- Started a local OTRVD journal before evidence collection.

Verify:

- Target directory exists at `adjudication-dev/arb/examples/israel-lebanon-invasion-condition-simple/`.

Document:

- This journal records the preparation process and limitations.

## 2026-05-01 — Packet completion pass

Observe:

- The delegated evidence worker stopped after a generic UN URL returned a 404. The actual case directory already contained useful market, State Department, UNIFIL, and news captures.
- Replaced the failed generic UN source with working UNIFIL source pages from 1 October, 6 October, and 13 October 2024.
- Confirmed the Polymarket page/API rule text, volume, identifiers, dispute sequence, and final state from local captures.
- Confirmed the State Department transcript said on September 30 that Israel had told the United States the operations were limited and focused on Hizballah infrastructure near the border.
- Confirmed UNIFIL's October 1 statement said the IDF notified UNIFIL the prior day of an intention to undertake limited ground incursions into Lebanon.

Think:

- The case is materially stronger on ground operations/incursions than on the narrower `intended to establish control` element.
- The packet should preserve both sides of the inference: pushing Hezbollah away from border villages may imply tactical control over some portion of Lebanon, but the official descriptions emphasize limited raids, infrastructure targeting, and no occupation.
- Later UNIFIL records help characterize the operation but cannot prove the September 30 deadline element by themselves.

Do the right thing:

- Created `situation.md`, `complaint.md`, `README.md`, `market-page.txt`, `official-source-record.txt`, `primary-evidence.txt`, `secondary-reporting.txt`, and `unresolved-record-gaps.txt`.
- Saved an extracted Reuters-syndicated record in `source-captures/web_fetch-usnews-reuters-syndication-20260501.txt` because direct Reuters extraction and direct U.S. News raw capture were blocked or timed out.
- Did not run arbitration and did not commit changes.

Verify:

- Checked source captures and excerpts for Polymarket, State Department, UNIFIL, NPR, BBC, CBS, NBC, Al Jazeera, and Reuters-syndicated reporting.
- Verified the proposition avoids Polymarket/UMA correctness language and asks only the direct factual condition.

Document:

- Remaining gaps are recorded in `unresolved-record-gaps.txt`, especially the lack of clean original X capture, UMA records, official Lebanese documents, and a precise crossing timeline.

## 2026-05-01 — Market capture and validation

Observe:

- Captured the Polymarket canonical event page and nested market URL.
- Captured the Gamma API event response for slug `will-israel-invade-lebanon-in-september`.
- Captured the CLOB condition API by condition id `0xab66a7d5745fc4da8a30a67665ee1de894b740644e643a6c6ec39b17212e4df5` after discovering that a slug query returned a broad listing rather than the target market.
- Moved the erroneous broad CLOB listing out of the packet to Trash rather than leaving misleading material in `source-captures/`.

Think:

- The exact rule text is available and sufficient. The dispute can be framed without asking whether the final Polymarket outcome was correct.
- The API `endDate` does not reproduce the rule-text deadline. The rule text must govern the proposition.

Do the right thing:

- Preserved exact market rule text, final/dispute-state text, identifiers, outcome prices, token winners, UMA status fields, and capture paths in `market-page.txt`.
- Kept the proposition in `situation.md` exactly as assigned.

Verify:

- `polymarket-gamma-event.json` contains event id `12681`, market id `507417`, and the exact rule text.
- `polymarket-clob-condition.json` contains token `Yes` as `winner: false` and token `No` as `winner: true`.
- Extracted Polymarket page text includes `Outcome proposed: Yes`, `Disputed`, `Outcome proposed: No`, `Disputed`, and `Final outcome: No`.

Document:

- Market capture details and remaining UMA gaps are documented in `market-page.txt` and `unresolved-record-gaps.txt`.

## 2026-05-01 — Official and primary-adjacent evidence collection

Observe:

- Captured the September 30, 2024 State Department briefing from the archived `2021-2025.state.gov` site.
- Captured UNIFIL statements from October 1, October 6, and October 13, 2024.
- Attempted to capture the IDF X post. The raw X page was saved, but extracted text did not expose the tweet body.
- Decoded the IDF tweet id timestamp as `2024-09-30T23:02:45.809Z`, inside the market window.
- Searched for an IDF website page carrying the same announcement but did not recover one.

Think:

- The State Department briefing is the cleanest pre-deadline official source. It supports limited operations focused on Hizballah infrastructure near the border and mentions targeting terrorist infrastructure inside Lebanon.
- UNIFIL October 1 is strong official UN evidence that the IDF notified UNIFIL on September 30 of intended limited ground incursions into Lebanon.
- The IDF wording is central, but because X did not yield a readable primary capture, the packet should not overstate the IDF capture quality.
- UNIFIL October 6 and October 13 are official and probative of later IDF presence inside Lebanon, but they are outside the proposition window.

Do the right thing:

- Separated official-source evidence from secondary reporting.
- Treated IDF statement quotations in CBS/AP and NBC as primary-adjacent, not as a fully preserved primary IDF capture.
- Recorded the X and Reuters access limitations rather than concealing them.

Verify:

- The State Department extracted file contains: `They have at this time told us that those are limited operations focused on Hizballah infrastructure near the border`.
- UNIFIL October 1 extracted file contains: `Yesterday, the IDF notified UNIFIL of their intention to undertake limited ground incursions into Lebanon`.
- UNIFIL October 6 extracted file contains: `inside Lebanese territory`.
- UNIFIL October 13 extracted file contains: `observed three platoons of IDF soldiers crossing the Blue Line into Lebanon`.

Document:

- Official and primary-adjacent evidence is documented in `official-source-record.txt` and `primary-evidence.txt`.

## 2026-05-01 — Secondary reporting collection

Observe:

- Captured or fetched Reuters/U.S. News, NPR, CBS/AP, NBC, BBC, and Al Jazeera reporting.
- The direct Reuters page was blocked or minimally rendered locally, but the same Reuters text was accessible through U.S. News by web fetch.
- CBS/AP was timestamped September 30, 2024, 9:45 PM EDT, inside the market window.
- Reporting consistently described the operation as limited, localized, and targeted; several reports also used invasion or ground offensive language.

Think:

- CBS/AP provides the best captured secondary preservation of the IDF announcement within the window.
- NBC is important for the intent question because it says the operation would aim to push Hezbollah forces farther away from the Israeli border.
- NPR October 1 and BBC provide the Lebanese-side and broader invasion framing, but they were published after the deadline.
- Al Jazeera preserves both Israeli announcement language and Hezbollah's denial of entry.

Do the right thing:

- Quoted reporting in context and preserved the publication/timing caveats.
- Avoided treating post-deadline reporting as if it were pre-deadline occurrence evidence unless it reported pre-deadline events.

Verify:

- CBS/AP extracted text contains the IDF quote: `A few hours ago, the IDF began limited, localized, and targeted ground raids`.
- NBC extracted text contains: `The operation would aim to push Hezbollah forces farther away from the Israeli border`.
- NPR September 30 extracted text contains: `brief intelligence-gathering raids inside southern Lebanon`.
- BBC extracted text contains both `ground invasion` framing and the `very close walking distance` limitation.
- Al Jazeera extracted text contains both the Israeli announcement and Hezbollah denial.

Document:

- Secondary reporting excerpts and assessments are documented in `secondary-reporting.txt`.

## 2026-05-01 — Packet synthesis

Observe:

- The record strongly supports Israeli ground operations or intended ground incursions inside Lebanon by the deadline.
- The record is materially weaker on explicit intent to establish control over Lebanese territory.
- The phrase `control over any portion of Lebanon` is the decisive ambiguity.

Think:

- A pro-proposition argument can reasonably read `control` broadly to include temporary tactical control over a raid objective, village position, route, or border-adjacent area. On that view, the IDF's raids in villages, air/artillery support, political approval, and aim of pushing Hezbollah away from the border support the proposition.
- An anti-proposition argument can reasonably read `control` as territorial control or occupation-style control. On that view, the official language of limited, localized, targeted infrastructure raids does not prove the required intent.
- This case is suitable for AgentCourt because both sides can argue from the preserved record, and the dispute turns on applying the rule phrase to a concrete fact pattern.

Do the right thing:

- Wrote a self-contained case packet with the required files:
  - `situation.md`
  - `market-page.txt`
  - `official-source-record.txt`
  - `primary-evidence.txt`
  - `secondary-reporting.txt`
  - `unresolved-record-gaps.txt`
  - `README.md`
  - `analysis.md`
  - `source-captures/`
- Added `source-captures/README.md` for local capture navigation.
- Did not create `complaint.md`; the skill says `arbitrate.sh` can generate it from `situation.md` when a run begins.
- Did not run arbitration.
- Did not commit.

Verify:

- Verified the required files exist.
- Verified core captured terms with targeted searches over extracted text.
- Preserved access failures and limitations as explicit gaps.

Document:

- This analysis file is the OTRVD journal for the preparation pass.

## Readiness assessment

The packet is ready for a first open-record arbitration run as an evidence-discovery audit. The packet is not ideal for a closed-record run because the official IDF tweet body and Lebanese official-source record are incomplete. If a closed-record run is desired, first improve the packet by recovering an official or archived IDF statement page, any Lebanese official statements from September 30 or October 1, and the UMA proposer/disputer rationales if available.


## 2026-05-01 — Argument analysis observations from 10,000-character policy run

Observed after the successful open-record/search-enabled run using the one-off 10,000-character policy:

- The arguments converged on the correct hinge. Timing, actor, and location mostly dropped out. Both sides effectively accepted that Israeli ground activity inside southern Lebanon began on September 30 before the ET deadline. The live issue became whether those raids or offensive actions implied an intent to establish control over any Lebanese ground.
- Plaintiff's strongest move was the low-threshold reading of `any portion`. Plaintiff framed control as temporary, local tactical control: clearing or denying villages, routes, or approaches near the border long enough to push Hezbollah back. That made the proposition easier to satisfy. The majority accepted this theory: ground troops entering Lebanese villages with fires support and a push-back objective implied some temporary control.
- Defendant's strongest move was the contemporaneous-intent objection. The official in-window language emphasized `limited`, `localized`, `targeted ground raids`, `infrastructure`, and `limited incursions`. None of those sources expressly said hold, occupy, administer, regulate movement, or create a buffer. Defendant also correctly emphasized the absence of in-window objective indicia such as checkpoints, berms, posts, sustained positions, logistics posture, or clear holding orders.
- Plaintiff's weakest inference was that `push Hezbollah back` necessarily means control. That inference is plausible, but not necessary. An actor can push an adversary back through raids, fires, disruption, and repeated incursions without intending to hold terrain in the stronger sense. Plaintiff prevailed because the proposition used `any portion` and did not define control tightly.
- Defendant's weakest inference was demanding too much proof at commencement. The proposition asks about intent to establish control, not completed control infrastructure by hour zero. Requiring checkpoints or fixed positions inside the first evening is probably too strict. Intent can be inferred from operational design, especially if the objective is to clear border villages.
- The council split was substantive rather than a vote-label failure. The three `demonstrated` votes accepted temporary tactical control as enough. The two `not_demonstrated` votes required clearer evidence of a holding or territorial-control mission. The rationales aligned with their votes. C1 and C2 were too conclusory, but C3, C4, and C5 exposed the real dispute cleanly.
- The case is highly sensitive to the definition of `control`. If control means temporary tactical exclusion or denial of ground, plaintiff probably wins. If control means a holding mission, territorial authority, or sustained control measures, defendant probably wins. The market text does not resolve that boundary.
- The run is scientifically useful. It did not produce a random council artifact. The split maps onto a genuine ambiguity in the predicate and shows AgentCourt surfacing the controlling interpretive variable: temporary tactical control versus sustained territorial control.

Operational note: the 10,000-character policy improved process quality for this case. It removed the earlier opening-statement failure without producing invalid submissions, and it let both sides state the real hinge rather than fight the limit.


## 2026-05-01 — Four-run sequential 10,000-character policy batch

The operator requested four additional sequential arbitration runs with the same settings as the successful one-off policy test: Israel-Lebanon case, open-record/search-enabled `openai://gpt-5?tools=search`, invalid-attempt limit 5, and a separate policy JSON setting all merits filing limits to 10,000 characters.

Batch directory:

- `out/_batch-israel-lebanon-invasion-condition-simple-gpt5-search-policy10000-more4-20260501-113242`

Summary:

- `out/_batch-israel-lebanon-invasion-condition-simple-gpt5-search-policy10000-more4-20260501-113242/summary.md`

Results:

- Run 01: operational failure before merits result. Error: `acp attorney did not submit a decision`. The run produced `events.ndjson` and `run.log`, but no `run.json`. Last events show the defendant in the argument phase attempted to access `/home/user/official-source-record.txt`, received `ENOENT`, and did not submit the required decision. There was also one recoverable plaintiff opening over-limit attempt at 10,636 characters against the 10,000-character policy limit.
- Run 02: completed, `demonstrated`, 3 demonstrated / 2 not_demonstrated. Run id `run-1777653494900883000`.
- Run 03: completed, `not_demonstrated`, 0 demonstrated / 5 not_demonstrated. Run id `run-1777654015359955000`. There was one recoverable defendant opening over-limit attempt at 10,019 characters against the 10,000-character policy limit.
- Run 04: completed, `demonstrated`, 4 demonstrated / 1 not_demonstrated. Run id `run-1777654590699490000`.

Aggregate for this batch:

- Requested runs: 4.
- Completed merits runs: 3.
- Operational failures: 1.
- Completed-run resolutions: 2 demonstrated / 1 not_demonstrated.
- Completed-run council votes: 7 demonstrated / 8 not_demonstrated.

Aggregate including the earlier successful 10,000-character run:

- Completed 10,000-character merits runs: 4.
- Resolutions: 3 demonstrated / 1 not_demonstrated.
- Council votes: 10 demonstrated / 10 not_demonstrated.

Observation:

The completed runs continue to isolate the same interpretive variable. Councils voting `demonstrated` treat temporary tactical clearing, denial, or local control of Lebanese villages/routes as enough to satisfy `control over any portion of Lebanon`. Councils voting `not_demonstrated` require clearer evidence of an intent to hold or administer terrain, and treat `limited`, `localized`, and `targeted` raid language as insufficient. The batch is therefore useful as a stability test: result labels lean demonstrated by run count, while individual council votes are exactly balanced across the completed 10,000-character runs.


## 2026-05-01 — Five-run sequential 10,000-character policy batch

The operator requested five more sequential arbitration runs with the same settings: Israel-Lebanon case, open-record/search-enabled `openai://gpt-5?tools=search`, invalid-attempt limit 5, and the separate policy JSON setting all merits filing limits to 10,000 characters.

Batch directory:

- `out/_batch-israel-lebanon-invasion-condition-simple-gpt5-search-policy10000-more5-20260501-123102`

Summary:

- `out/_batch-israel-lebanon-invasion-condition-simple-gpt5-search-policy10000-more5-20260501-123102/summary.md`

Results:

- Run 01: completed, `not_demonstrated`, 2 demonstrated / 3 not_demonstrated. Run id `run-1777656662484524000`.
- Run 02: completed, `demonstrated`, 3 demonstrated / 2 not_demonstrated. Run id `run-1777657323931777000`.
- Run 03: completed, `demonstrated`, 4 demonstrated / 1 not_demonstrated. Run id `run-1777657993633222000`.
- Run 04: completed, `demonstrated`, 3 demonstrated / 2 not_demonstrated. Run id `run-1777658531644729000`.
- Run 05: completed, `demonstrated`, 4 demonstrated / 1 not_demonstrated. Run id `run-1777659140113700000`.

Aggregate for this batch:

- Requested runs: 5.
- Completed merits runs: 5.
- Operational failures: 0.
- Completed-run resolutions: 4 demonstrated / 1 not_demonstrated.
- Completed-run council votes: 16 demonstrated / 9 not_demonstrated.

Aggregate across completed 10,000-character policy runs so far:

- Completed merits runs: 9.
- Operational failures in attempted 10,000-character runs: 1.
- Resolutions: 7 demonstrated / 2 not_demonstrated.
- Council votes: 26 demonstrated / 19 not_demonstrated.

Operational notes:

Every run in this batch produced `run.json`, `council.json`, `digest.md`, and `events.ndjson`. All five runs had recoverable over-limit opening attempts under the 10,000-character policy. The over-limit attempts ranged from 10,049 to 10,730 characters and were retried successfully.

Observation:

The result distribution now leans toward `demonstrated` by both run count and council-vote count, but the underlying split remains the same. The positive votes read `control over any portion` to include temporary tactical clearing or denial of Lebanese villages/routes. The negative votes require stronger evidence of a holding, occupation, administration, or sustained territorial-control mission and treat official `limited`, `localized`, and `targeted raids` language as insufficient.


## 2026-05-01 — Observations across ten attempted 10,000-character policy runs

Scope:

- Count: 10 attempted 10,000-character policy runs for `israel-lebanon-invasion-condition-simple`.
- Completed merits runs: 9.
- Operational failures: 1.
- Completed-run resolutions: 7 demonstrated / 2 not_demonstrated.
- Completed-run council votes: 26 demonstrated / 19 not_demonstrated.

Observations:

- The completed-result distribution leans `demonstrated`: 7 of 9 completed merits runs. The aggregate council vote is 26 demonstrated / 19 not_demonstrated. This is a real lean, but not a stable consensus.
- The factual record is no longer the main source of variance. Councils generally agree that Israeli ground activity inside Lebanon began on September 30 before the deadline. The dispute has collapsed onto one interpretive variable: what `establish control over any portion of Lebanon` means.
- The `demonstrated` theory is that `any portion` plus `commenced` sets a low threshold. Ground raids into Lebanese villages, combined-arms support, clearing Hezbollah infrastructure, and pushing Hezbollah away from the border imply temporary tactical control or denial over some Lebanese ground.
- The `not_demonstrated` theory is that `control` means more than raid, disruption, or temporary presence. Negative votes treat the official language — `limited`, `localized`, and `targeted ground raids` against infrastructure — as evidence against a holding, occupation, administration, or sustained territorial-control objective.
- The split is coherent rather than a vote-label artifact. The rationales mostly match the votes. AgentCourt is exposing a genuine ambiguity in the predicate rather than producing random output.
- The `demonstrated` wins are often close or moderate: 3-2, 3-2, 4-1, 3-2, 4-1, etc. The strongest negative result was a 0-5 `not_demonstrated` run. That matters because a council adopting the stricter definition of control can reject the proposition cleanly.
- The market wording is under-specified. If `control` includes tactical clearing, local denial, or temporary control during raids, the condition is probably demonstrated. If `control` requires a holding mission or sustained territorial authority, the condition is probably not demonstrated. The market text does not resolve that boundary.
- Operationally, the 10,000-character policy made the runs workable, but it did not remove filing-pressure problems. One attempted run failed before merits because an attorney did not submit a decision after file-access errors. The later five-run batch completed cleanly, but every run still had recoverable over-limit opening attempts.

Bottom line:

This is no longer primarily an evidence-sufficiency problem. It is a definitional-control problem. The experimental signal is that `demonstrated` is the more frequent outcome under open-record AgentCourt, but the predicate remains scientifically unstable unless `control` is defined.


## 2026-05-01 — Ten additional 10,000-character policy runs

Batch: `out/_batch-israel-lebanon-invasion-condition-simple-gpt5-search-policy10000-more10-20260501-143152`.

Results:

- Run 01: `demonstrated`, 4-1, `run-1777663912784496000`.
- Run 02: `demonstrated`, 3-2, `run-1777664459733122000`.
- Run 03: `not_demonstrated`, 1-4, `run-1777665078745760000`.
- Run 04: `demonstrated`, 4-1, `run-1777665691912179000`.
- Run 05: `demonstrated`, 3-2, `run-1777666333771699000`.
- Run 06: `demonstrated`, 3-2, `run-1777666880804991000`.
- Run 07: `demonstrated`, 3-2, `run-1777667388786878000`.
- Run 08: `demonstrated`, 5-0, `run-1777668086260537000`.
- Run 09: `demonstrated`, 4-1, `run-1777668801431749000`.
- Run 10: `demonstrated`, 4-1, `run-1777669392280714000`.

Aggregate for this batch: 9 `demonstrated` / 1 `not_demonstrated` by resolution, 34 `demonstrated` / 16 `not_demonstrated` by council vote. No operational failures.

Cumulative 10,000-character policy aggregate: 20 attempted runs, 19 completed merits runs, 1 operational failure. Completed-run resolutions: 16 `demonstrated` / 3 `not_demonstrated`. Council votes: 60 `demonstrated` / 35 `not_demonstrated`.

Summary file: `out/_batch-israel-lebanon-invasion-condition-simple-gpt5-search-policy10000-more10-20260501-143152/summary.md`.
Machine-readable run results: `out/_batch-israel-lebanon-invasion-condition-simple-gpt5-search-policy10000-more10-20260501-143152/run-results.json`.
Machine-readable model vote stats: `out/_batch-israel-lebanon-invasion-condition-simple-gpt5-search-policy10000-more10-20260501-143152/model-vote-stats.json`.

## 2026-05-01 — Council-model metadata and interpretive-threshold note

Follow-up model check:

The public metadata did not provide a clean release-date or training-regime explanation for the council split. The strongest signal remains behavioral rather than documentary: the models applied different thresholds for the word `control` in the proposition.

Public metadata checked through OpenRouter listed the main council models as follows:

- `openrouter://google/gemini-3-flash-preview`: Dec. 2025 listing, long-context thinking/agentic model. In the cumulative 10,000-character-policy sample it voted 16 `demonstrated` / 0 `not_demonstrated`.
- `openrouter://google/gemini-2.5-flash`: Jun. 2025 listing, reasoning/workhorse model. It voted 8 / 4 and was therefore mixed.
- `openrouter://anthropic/claude-opus-4.6`: Feb. 2026 listing, long-running professional/agentic model. It voted 8 / 3.
- `openrouter://meta-llama/llama-4-scout`: Apr. 2025 listing, MoE model, 17B active / 109B total by OpenRouter metadata. It voted 7 / 1.
- `openrouter://x-ai/grok-3` and `openrouter://x-ai/grok-4-fast`: smaller sample, but leaned `demonstrated`.
- `openrouter://openai/gpt-5.4`, `openrouter://openai/gpt-5.2-chat`, and `openrouter://amazon/nova-premier-v1`: strongly leaned `not_demonstrated`; observed cumulative counts were GPT-5.4 0 / 6, GPT-5.2 Chat 0 / 5, and Nova Premier 0 / 4.

What did not explain the split:

- Recency did not explain it. GPT-5.4 and Claude Opus 4.6 were both listed as 2026-era frontier models, but they split in opposite directions.
- Context length did not explain it. Long-context models appeared on both sides.
- Generic `reasoning` or `agentic` positioning did not explain it. Both sides included models marketed for reasoning or agentic work.
- Public training descriptions were too thin to support a causal training-regime claim. The provider-facing material mostly exposed capability framing, safety/evaluation summaries, and high-level post-training language, not enough to infer why a model used one legal threshold rather than another.

Observed behavioral split:

- The `demonstrated`-leaning models treated documented ground raids, village clearing, Hezbollah-infrastructure destruction, and IDF/U.S. statements as sufficient to infer temporary tactical control over some Lebanese territory.
- The `not_demonstrated`-leaning models required a more explicit record of control: occupation, holding territory, administration, sustained presence, or an intent to control territory rather than conduct raids.

Practical conclusion:

Council composition is a material experimental variable in this scenario. Model family and model version should be recorded as covariates in stability analysis. The models should not be treated as interchangeable jurors when the dispositive issue is an under-specified legal or quasi-legal threshold such as `control`.
