# Adjudication Evals

`evals/` contains adjudication eval tools, checked-in eval sets, endpoint-variant sampling tools, and pool samplers.  The [Adjudication Evals Manual](manual.md) documents commands, endpoint-variant inventory work, batch evals, filtered variants, gene clustering, pool sampling, scoring, and troubleshooting.

## Documentation

| Document | Use |
| --- | --- |
| [Adjudication Evals Manual](manual.md) | Commands, endpoint-variant inventory, batch evals, filtered variants, gene clustering, pool sampling, scoring, and troubleshooting. |
| [Sampling Runbook](docs/sampling-runbook.md) | Repeatable procedure from root-model inventory through tuple-uniform pool sampling. |
| [Model Inventory Notes](docs/model-inventory.md) | Notes for OpenRouter endpoint inventory work. |

## Quick Checks

Run these commands from `evals/` before changing items, prompts, schemas, or sampling inputs:

```bash
uv run tools/score_eval.py validate-items --questions sets/core20/questions.jsonl
uv run tools/score_eval.py validate-items --questions sets/deliberation/questions.jsonl
uv run tools/audit_eval.py --json
```

Run a deterministic local test and score it:

```bash
uv run tools/run_eval.py --mock perfect --models mock:perfect --out results/mock-perfect
uv run tools/score_eval.py score --run results/mock-perfect
```

Generated outputs belong under `results/`, which is ignored except for `results/.gitkeep`.  OpenRouter credentials come from `OPENROUTER_API_KEY` or ignored `secrets/openrouter.api.txt`.  The gene-response embedding runner also needs `OPENAI_API_KEY` or ignored `secrets/openai.api.txt`.

## Layout

| Path | Contents |
| --- | --- |
| `sets/core20/questions.jsonl` | Canonical 20-item core eval set. |
| `sets/deliberation/questions.jsonl` | 20-item deliberation eval set. |
| `tools/run_eval.py` | Mock and OpenRouter eval runner. |
| `tools/score_eval.py` | Item validator and deterministic scorer. |
| `tools/audit_eval.py` | Repository consistency audit. |
| `tools/run_end_to_end.py` | Full endpoint-variant pool pipeline runner. |
| `variants/filtered-20260529/` | Current checked-in filtered endpoint-variant snapshot. |
