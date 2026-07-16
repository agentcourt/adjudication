# Adjudication Evals

`evals/model-pool/` contains source tooling and checked-in inputs for adjudication model-pool evaluation.  The tools evaluate models and pinned OpenRouter provider endpoints against JSON-scored question sets, build endpoint inventories, collect behavior-prompt responses, cluster embeddings, and sample endpoint/persona pools.  Generated run files belong under `results/`; versioned inputs belong under `sets/`, `schemas/`, `rubrics/`, `prompts/`, `personas/`, `config/`, `genes.json`, `sampled-genes.json`, and `variants/`.

Use `evals/model-pool/` as the working directory unless a command says otherwise.  OpenRouter calls require `OPENROUTER_API_KEY` or ignored `secrets/openrouter.api.txt`.  Embedding runs require `OPENAI_API_KEY` or ignored `secrets/openai.api.txt`.

## Documentation

| Document | Use |
| --- | --- |
| [Adjudication Evals Manual](manual.md) | Detailed command reference, terminology, scoring model, endpoint-variant procedures, pool construction, and troubleshooting. |
| [Model-Pool Analysis](analysis.md) | Current human analysis tied to eval results or pool construction work. |
| [Model-Pool Documentation Index](docs/README.md) | Stable model-pool design notes, runbooks, and historical planning documents. |
| [Sampling Runbook](docs/sampling-runbook.md) | Staged procedure from root-model inventory through tuple-uniform pool sampling. |
| [Model Inventory Notes](docs/model-inventory.md) | OpenRouter provider-endpoint identity, routing constraints, metadata fields, and interpretation limits. |
| [Jury Pool Generation](docs/jury-pool-generation.md) | Legacy CSV pipeline for persona clustering, clustering charts, and council CSV selection. |
| [Core20 Rubric](rubrics/core20.md) | Response schemas, deterministic checks, deliberation score, and operational metrics. |
| [Development Notes](devnotes.md) | Development history, rationale, and follow-up notes for eval tooling. |

## Quick Checks

Run these commands from `evals/model-pool/` before changing items, prompts, schemas, configs, or sampling inputs.  The validation commands check question records and fixture references.  The audit checks repository consistency for the eval inputs and tool references.

```bash
uv run tools/score_eval.py validate-items --questions sets/core20/questions.jsonl
uv run tools/score_eval.py validate-items --questions sets/deliberation/questions.jsonl
uv run tools/audit_eval.py --json
```

Use the mock runner for a deterministic local test that does not call OpenRouter.  The first command writes a local run under `results/`.  The second command scores that run with the deterministic scorer.

```bash
uv run tools/run_eval.py --mock perfect --models mock:perfect --out results/mock-perfect
uv run tools/score_eval.py score --run results/mock-perfect
```

## Run Data

Run outputs, model responses, score files, manifests, provider inventories, sampled pools, and run-specific summaries belong under `results/`, or under an intentional snapshot directory in `variants/` when the repository needs a checked-in survivor set.  `results/` is ignored except for `results/.gitkeep`, and credentials are ignored under `secrets/`.  README content stays limited to stable workflow, file locations, and entry points; run IDs, endpoint counts, pass rates, accepted endpoint lists, and dated filter details belong in run artifacts, snapshot summaries, analysis notes, or the manual.

## Layout

| Path | Contents |
| --- | --- |
| `sets/` | Checked-in question sets and fixtures. |
| `schemas/` | JSON schemas for items, responses, and result records. |
| `rubrics/` | Deterministic scoring rules and metric definitions. |
| `prompts/` | Prompt text used by eval or pool-construction tools. |
| `personas/` | Persona files for gene runs, replay work, and pool construction. |
| `config/` | Model and pool configuration files. |
| `genes.json`, `sampled-genes.json` | Behavior-prompt source data and sampled prompt sets. |
| `variants/` | Checked-in provider-endpoint snapshot files. |
| `tools/run_eval.py`, `tools/score_eval.py`, `tools/audit_eval.py` | Question-set execution, validation, scoring, and repository checks. |
| `tools/model_inventory.py`, `tools/run_variant_batch.py`, `tools/run_end_to_end.py` | Provider inventory, endpoint-variant evaluation, and full pool-pipeline execution. |
| `tools/run_first_gene_inference_embeddings.py`, `tools/run_embedding_pca.py`, `tools/run_gene_pca_clustering.py` | Gene-response collection, embedding reduction, and clustering. |
| `tools/aggregate_variant_persona_clusters.py`, `tools/sample-tuple-pool.py` | Cluster aggregation and tuple-uniform pool sampling. |
| `tools/cluster-personas.py`, `tools/clusters-graph.py`, `tools/generate-council.py`, `tools/select-council.py` | Legacy CSV persona-clustering and council-selection tools. |
| `results/` | Generated eval and pool-construction files. |
