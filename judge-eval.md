# Judge Voir Dire Eval Plan

## Scope

This eval should measure how a judge handles proposed voir dire questions before a juror candidate sees them.  The first target is the ADC ruling path for `decide_voir_dire_question`.  The eval should present one pending lawyer question and require the judge to return `allowed`, `ruling_reason`, and no other case disposition.

The eval should use the same procedural boundary as ADC.  Counsel proposes a question, the engine records a pending `VoirDireExchange`, the judge rules, and a juror answer becomes available only if the judge allows the question.  That structure tests the behavior that protects the juror from prohibited questions.

## Eval Item Shape

Each item should contain a minimal ADC state in the `voir_dire` phase with one pending `VoirDireExchange` where `judge_allowed = null`.  The item should identify the asking side, target juror candidate, proposed question, expected ruling, and accepted reason tags.  Optional context can include the case theme, prior questionnaire answers, prior voir dire exchanges, or admitted-record limits when those details affect the ruling.

A fixture row should include these fields: `id`, `case_theme`, `asked_by`, `juror_id`, `question`, `expected_allowed`, `expected_reason_tags`, `severity`, and `context_notes`.  The fixture should stay readable, so a helper should construct the full ADC state from the row rather than requiring each row to embed a complete state object.  Rows that need richer context can carry an override block, but the default path should remain compact.

## Dataset

The first dataset should contain 60 to 100 rows.  Allowed questions should test bias, burden-of-proof discipline, attitudes toward documentary or digital evidence, damages skepticism, attention, and ability to follow instructions.  Disallowed questions should include merits argument, assumed disputed facts, precommitment on liability, precommitment on damages, questions about whether specific proof would be enough, inadmissible-material references, and disguised forms of “how would you vote.”

The dataset should have three tiers.  Tier 1 should contain clean rule-application items where the text alone determines the ruling.  Tier 2 should contain contextual items where the same phrasing may be allowed or disallowed depending on the case record or prior answers.  Tier 3 should contain adversarial paraphrases, including hypotheticals, sufficiency probes, damages anchors, and “could you still find for my client if” formulations.

## Runner

Add a command such as `adc eval judge-voir-dire --model ... --fixtures ... --output ...`.  The command should load fixtures, build the pending-ruling ADC state, obtain the judge opportunity, ask the judge model, validate that the model called `decide_voir_dire_question`, and write one JSONL result per item.  It should use the existing ADC model and tool-call code where possible, and it should not add third-party dependencies without a separate decision.

The result record should preserve the fixture id, prompt state, proposed question, model name, raw model response, extracted tool payload, expected ruling, score, and normalized failure reason.  Invalid responses should be first-class results, including missing tool call, wrong tool, wrong exchange id, wrong juror id, malformed `allowed`, and empty or unusable `ruling_reason`.  The command should also write a summary with aggregate scores and per-category slices.

## Scoring

The primary score should be binary ruling accuracy: whether `allowed` matches `expected_allowed`.  Explanation scoring should be tag-based at first.  The scorer should check whether the ruling reason names an accepted category such as `proper_bias_probe`, `proper_burden_probe`, `proper_digital_evidence_probe`, `precommitment_liability`, `precommitment_damages`, `specific_evidence_sufficiency`, `assumed_disputed_fact`, `merits_argument`, or `inadmissible_material`.

False allows should carry higher severity weight than false disallows because they can expose a juror to an improper question.  False disallows should still be reported separately because a judge that blocks legitimate screening questions weakens voir dire.  The summary should show overall accuracy, weighted accuracy, false-allow rate, false-disallow rate, invalid-response rate, and results by reason tag, party, tier, and phrasing family.

## Prompt Improvement Loop

The eval should support a small prompt-tuning process for the judge opportunity text.  Run the fixed fixture set against the current prompt, classify each failure, and group failures by cause before editing the prompt.  Useful failure classes include false allow, false disallow, wrong reason, malformed payload, invented fact, overbroad ruling, and missed context.

Prompt edits should address a failure cluster rather than a single example.  The first likely cluster is the line between proper bias probes and disguised verdict precommitment.  The prompt should tell the judge to allow questions about whether a juror can follow a rule or evaluate a class of evidence, while blocking questions that ask the juror to forecast a vote, assign case weight to named evidence, commit to a damages range, or answer a hypothetical that matches the disputed facts.

After each prompt edit, run the full fixture set again and compare the score movement.  Accept the edit only if it improves the intended cluster without a material decline in other categories.  The report should preserve before-and-after summaries so a prompt change can be reviewed by category, tier, question family, and asking side.

Fixtures should stay fixed during a tuning pass.  New examples can be added after failure analysis, but they should enter as a new eval version rather than changing the measurement set mid-pass.  That separation keeps prompt improvement distinct from dataset expansion.

Prompt iteration should begin with eval-local template files under `adc/evals/judge/prompts/`.  The runner accepts `--opportunity-prompt-file`, renders fixture values such as `{{question}}`, `{{asked_by}}`, `{{juror_id}}`, `{{case_theme}}`, and `{{production_objective}}`, and sends the rendered text as the model-facing opportunity objective.  The Lean opportunity still controls the role, phase, allowed tool, constraints, and state transition, so a candidate prompt can be tested without editing `adc/engine/Main.lean`.

Each candidate run should use a distinct output directory, such as `adc/evals/judge/out/candidate-v1-dry` or `adc/evals/judge/out/candidate-v1-live`.  The report records `prompt_source`, `prompt_name`, and `prompt_path`, and copies the prompt file into the report directory as `opportunity_prompt.md`.  A production prompt change should be a separate step after the eval shows that a candidate improves a failure cluster without degrading other categories.

The hard fixture augmentation lives in `adc/evals/judge/voir_dire_questions_hard_v1.jsonl`.  It is a separate 30-row tier-3 set rather than a replacement for the original 60-row baseline.  The hard set concentrates on boundary pairs: damages-range comfort questions, digital-evidence sufficiency, limiting-instruction phrasing that embeds disputed facts, missing-witness sufficiency, insurance references, and “could you still find” formulations.

## Tests

Unit tests should cover fixture parsing, state construction, pending-opportunity construction, extraction of the `decide_voir_dire_question` tool call, invalid tool-call handling, and scorer behavior.  Golden scorer examples should include a clean allowed bias question, a clean allowed burden question, a prohibited “Would this signed confession be enough?” question, a prohibited damages-anchor question, and a disguised verdict-precommitment question.  Tests should distinguish ordinary verification from regression coverage unless a future bug requires a regression test.

## Documentation

Record the rationale in `adc/devnotes.md` when implementation begins.  The note should explain why fixtures use pending ADC exchanges, why false allows receive higher severity, and why explanation scoring starts with deterministic tags rather than model-graded free text.  Any later move to model-graded explanation scoring should be a separate design decision because it changes the eval’s trust boundary.

## Location

Go implementation code should live under `adc/runtime/eval`.  Fixtures, generated reports, and judge-eval-specific documentation should live under `adc/evals/judge`.  Keeping eval assets outside ordinary case execution prevents test data from becoming part of the court process while still using real ADC state and tool schemas.
