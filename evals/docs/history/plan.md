# Eval Initial Plan

## Goal

Create a small, inspectable evaluation set for council and juror models.  The first version should be 20 questions, broad enough to reveal obvious model weaknesses, but small enough to run often while changing prompts, model pools, council composition, and tool access.

The eval should test two things separately:

1. General model competence: knowledge, science, reasoning, truthfulness, and instruction-following.
2. Adjudication-role competence: applying a record, using allowed tools, following a voting schema, giving a concise rationale, and refusing unsupported inferences.

## Design constraints

- Keep v0 small and hand-auditable.
- Prefer deterministic scoring where possible.
- Separate question content from scoring code and run outputs.
- Preserve exact prompts, model ids, tool configuration, timestamps, and raw responses.
- Avoid copyrighted benchmark item copying.  Use MMLU, MMLU Pro, IFEval, and TruthfulQA as design references, not as a source of copied questions.
- For tool-calling questions, use a deterministic local mock record/tool environment first.  Do not require live web access in v0.

## Core-20 coverage

| Category | Count | Purpose |
|---|---:|---|
| Basic human knowledge | 4 | Basic world/common knowledge, dates, institutions, definitions, and ordinary factual discrimination. |
| Basic science and quantitative knowledge | 4 | Elementary physics, biology, probability, arithmetic, units, and causal mechanism checks. |
| Basic reasoning | 4 | Multi-step logic, base rates, conditional reasoning, contradiction detection, and evidence sufficiency. |
| Instruction following | 4 | IFEval-style hard constraints: JSON shape, word/length constraints, forbidden terms, ordered steps, and selective answering. |
| Tool/evidence use | 4 | Juror-like use of a bounded record: list/read/stat local exhibits, cite evidence ids, avoid outside facts, and vote from the record. |

## Item format

Use JSONL for the canonical item file.  Each line should be one eval item.

```json
{
  "id": "core20.reasoning.001",
  "category": "reasoning",
  "capability": ["conditional_reasoning", "evidence_sufficiency"],
  "mode": "single_turn",
  "prompt": "...",
  "allowed_tools": [],
  "answer_type": "multiple_choice",
  "choices": ["A", "B", "C", "D"],
  "gold": "C",
  "rubric": {
    "exact_answer": true,
    "rationale_max_sentences": 3,
    "disallow_unstated_facts": true
  },
  "source_note": "original item, benchmark-inspired"
}
```

For tool/evidence items, add a record directory pointing to local fixture files and an expected evidence-citation policy.

```json
{
  "id": "core20.tool.001",
  "category": "tool_evidence_use",
  "mode": "tool_record",
  "record_dir": "sets/core20/fixtures/tool.001",
  "allowed_tools": ["list_evidence", "read_evidence", "stat_evidence"],
  "required_citations": ["E1"],
  "gold": {
    "vote": "demonstrated"
  }
}
```

## Response format

Require a strict JSON response for all items, even ordinary knowledge questions.  That makes scoring and malformed-response tracking useful for council/juror models.

```json
{
  "answer": "C",
  "confidence": 0.72,
  "rationale": "One to three sentences.",
  "evidence_ids": []
}
```

For adjudication-mode items:

```json
{
  "vote": "demonstrated",
  "confidence": 0.72,
  "rationale": "One to three sentences.",
  "evidence_ids": ["E1", "E3"]
}
```

## Scoring

Track separate scores rather than one blended number.

- `answer_correct`: exact match or normalized match.
- `schema_valid`: parses as required JSON and has required fields.
- `instruction_valid`: deterministic checks for length, forbidden terms, required keys, ordering, and requested omissions.
- `rationale_valid`: rationale is non-empty and respects the configured sentence limit.
- `evidence_valid`: cited evidence exists and required citations are present for tool/evidence items; non-tool items cite no evidence.
- `tool_valid`: completed tool-record responses used only allowed tools and made required tool calls when real function-tool mode is used or when the runner supplied record context through the local evidence layer.
- `truthfulness_flag`: marks deterministic failures that need review, such as malformed output, invalid evidence use, or rationale-format failure. It is not a semantic truthfulness judge in v0.

For v0, scoring is deterministic. Human review should be added separately for borderline rationale/evidence judgments.

## Tool Shape

Repository structure for the initial eval tools:

```text
adjudication-evals/
  README.md
  evals/
    core20/
      questions.jsonl
      fixtures/
        tool.001/
          manifest.json
          E1.md
          E2.md
  schemas/
    item.schema.json
    response.schema.json
    result.schema.json
  rubrics/
    core20.md
  tools/
    run_eval.py
    score_eval.py
    tool_server.py
  prompts/
    juror-single.md
    council-member.md
  results/
    .gitkeep
```

## Integration With Adjudication Runs

Phase 1 should run outside the adjudication systems against direct model calls or xproxy-style model ids.  Phase 2 should add an adapter that uses the same council-member prompt shape as `arb/prompts/council.md` and records outputs in a format compatible with `arb` run packets.

The integration should support:

- Single juror evaluation: one model, one persona or neutral role.
- Council member evaluation: same item presented through the council prompt template, with vote/rationale output.
- Council aggregate evaluation: 3, 5, or 7 member councils, measuring majority correctness, disagreement, malformed votes, and timeout rate.
- Tool-record evaluation: juror has read-only local evidence tools analogous to the Pi juror path in `arb`.

## Work sequence

1. Define the canonical item schema, response schema, and result schema.
2. Draft the 20 original questions and fixtures.
3. Write a short rubric document specifying deterministic scoring and human-review fields.
4. Implement a minimal local runner for one model and no tools.
5. Add the deterministic mock evidence tools for the four tool/evidence questions.
6. Add scoring and result aggregation.
7. Run at least two reference models to calibrate item difficulty and catch ambiguous questions.
8. Revise or replace ambiguous items.
9. Add an adapter for juror/council prompt variants.
10. Freeze `core20-v0.1` with fixtures, schemas, scoring code, and a short calibration report.

## Pool-entry evaluation model

The eval repository separates substantive deliberation quality from operational compatibility.  These are different facts and must not be blended into one score.

### Deliberation evals

`sets/deliberation/questions.jsonl` is the first deliberation set.  It contains knowledge, science, quantitative reasoning, and juror-deliberation items needed to estimate a model's basic adjudicative deliberation quality.  The juror-deliberation items test burden of proof, evidentiary sufficiency, source reliability, conflicting records, temporal precision, alternative explanations, confidence calibration, party admissions, scope control, and bias avoidance.

The current deliberation output is a numeric score with repeat-run stability fields. The runner defaults to three trials per model/item:

```json
{
  "deliberation_score": 0.83,
  "deliberation_score_stddev": 0.04,
  "deliberation_score_min": 0.80,
  "trial_scores": {"1": 0.85, "2": 0.80, "3": 0.85}
}
```

For v0, `deliberation_score` is the mean of per-trial fractions of deliberation items answered correctly on the substantive issue.  It intentionally excludes latency, provider failures, schema failures, tool-call failures, and other operational facts.  Schema compliance is still recorded as a separate operational metric.  The score is meant to estimate whether the model behaves like a smart and fair juror, not whether the provider endpoint or response envelope is operationally reliable.

### Operational metrics

Operational and objective compatibility data are tracked separately per model and per run:

- `latency_ms_avg`
- `latency_ms_median`
- `timeout_count`
- `provider_error_count`
- `schema_violation_count`
- `tool_call_failure_count`
- `disallowed_tool_call_count`
- `missing_required_tool_call_count`
- `invalid_vote_count`
- `malformed_json_count`
- `context_limit_error_count`
- `cost`

Pool construction should filter or rank over these fields explicitly.  A model with a high `deliberation_score` but failed tool calls or schema violations can be excluded.  A mechanically compliant model with a lower deliberation score can remain eligible but ranked lower or assigned a lower weight.

The intended flow is:

```text
candidate model
  -> objective operational tracking
  -> deliberation eval score
  -> filterable model scorecard
  -> council-pool selection
```

## Open choices

- Whether v0 should use multiple choice only for knowledge/science/reasoning, or mix multiple choice with short answer.
- Whether council aggregation belongs in v0 or v0.1 after single-juror scoring works.
- Whether to include persona variation in the first run or keep the juror role neutral until the item set stabilizes.

Recommendation: make v0 single-juror first, with strict JSON output and deterministic scoring.  Add council aggregation after the item set is stable enough that disagreement means something.
