# Adjudication Evals

`evals/` selects OpenRouter provider endpoints for juror and council model pools.  It evaluates endpoints with JSON-scored question sets, filters endpoints by provider-error count and deliberation score, samples behavior prompts from accepted endpoints, clusters the response embeddings, and samples endpoint/persona records for pool use.  Scripts write generated files under `results/`.  Checked-in inputs are in `sets/`, `schemas/`, `rubrics/`, `prompts/`, `personas/`, `config/`, `genes.json`, and `sampled-genes.json`.  Checked-in accepted endpoint files are in `variants/`.

Use `evals/` as the working directory unless a command says otherwise.  OpenRouter calls require `OPENROUTER_API_KEY` in the environment or an ignored `secrets/openrouter.api.txt` file.  Gene-response embedding calls also require `OPENAI_API_KEY` in the environment or an ignored `secrets/openai.api.txt` file.

## Task Guide

| Task | Use |
| --- | --- |
| Check one model or pinned provider endpoint against a question set. | [Question-Set Evaluation](#question-set-evaluation) |
| Build a pool from OpenRouter model IDs. | [Full Selection Procedure](#full-selection-procedure) |
| Refresh or inspect the accepted provider endpoint set. | [Provider Endpoint Selection](#provider-endpoint-selection) |
| Understand behavior prompts, embeddings, PCA, clusters, and pool sampling. | [Behavior Clustering And Pool Sampling](#behavior-clustering-and-pool-sampling) |
| Find the meaning and purpose of a repository term. | [Glossary](#glossary) |
| Find the file that stores a specific record. | [File Reference](#file-reference) |

## Full Selection Procedure

Input: OpenRouter model IDs.  Output: `pool.jsonl`, a JSONL file of endpoint/persona records.  A provider endpoint is evaluated as its own unit because one OpenRouter model ID can route to multiple provider endpoints with different provider tags, quantization, context limits, supported parameters, pricing, and behavior.

| Step | Input | Script | Output |
| --- | --- | --- | --- |
| Select root models | Explicit OpenRouter model IDs, or `--root-count` plus `--root-seed` | `tools/run_end_to_end.py` or a recorded selection command | Selected OpenRouter model IDs |
| Inventory provider endpoints | Selected OpenRouter model IDs | `tools/model_inventory.py` | Provider endpoint rows and raw OpenRouter catalog files |
| Evaluate provider endpoints | Provider endpoint rows and a question file | `tools/run_variant_batch.py`, which calls `tools/run_eval.py` and `tools/score_eval.py` | Response files, score files, exact request specs, and per-endpoint summary rows |
| Filter endpoints | Provider endpoint rows and evaluation summaries | `tools/run_end_to_end.py` filter stage, or the filter script in `docs/sampling-runbook.md` | Accepted endpoint rows, rejected endpoint records, copied request specs, and filter summary |
| Collect behavior responses | Accepted endpoint rows, sampled genes, persona file, sample count | `tools/run_first_gene_inference_embeddings.py` | Gene completions, OpenRouter metadata, embeddings, and per-gene summary |
| Reduce embeddings | Gene completion records with embeddings | `tools/run_embedding_pca.py` | PCA coordinates and PCA summary |
| Cluster responses | Per-gene PCA records | `tools/run_gene_pca_clustering.py` | Cluster assignments and clustering summary |
| Aggregate cluster labels | Cluster assignments, cluster fit, and accepted endpoint rows | `tools/aggregate_variant_persona_clusters.py` | Endpoint/persona cluster records |
| Sample pool | Endpoint/persona cluster records | `tools/sample-tuple-pool.py` | `pool.jsonl` and sampling diagnostics |

`tools/run_end_to_end.py` executes those stages in one command.  It writes `manifest.json`, `commands.jsonl`, stage directories, and `summary.json` under one output directory.  Use the individual scripts when a stage needs inspection, a failed stage needs to be repeated, or the accepted endpoint snapshot under `variants/` needs to be rebuilt with explicit filter records.

```bash
uv run --script tools/run_end_to_end.py \
  --run-id e2e-YYYYMMDDTHHMMSSZ \
  --root-count 5 \
  --root-seed 0 \
  --eval-trials 1 \
  --gene-count 2 \
  --samples-per-gene 1 \
  --pca-dimensions 3 \
  --min-k 2 \
  --max-k 4 \
  --pool-size 5
```

This example evaluates five sampled root models, uses one trial per question, samples two genes, collects one response per accepted endpoint/gene pair, reduces embeddings to three PCA dimensions, searches K-means values from `2` through `4`, and writes five pool entries.  A production pool should set root model IDs or root sampling parameters, question file, trial count, filter criteria, gene selection, sample count, PCA dimensions, clustering range, pool size, and random seeds explicitly.  Use `--resume` to reuse completed stage files and `--stop-after` to stop after a named stage.

## Glossary

| Term | Meaning | Purpose | Main files |
| --- | --- | --- | --- |
| Question | One ordinary question or record-based adjudication task. | Tests answer quality, JSON compliance, or record use. | `sets/*/questions.jsonl` |
| Evidence record | Local evidence for a record-based adjudication question. | Limits what the model may cite when answering a record-based question. | `sets/core20/fixtures/*` |
| Response | Strict JSON returned by a model for one question. | Gives the scorer fixed fields for answer, confidence, rationale, and evidence citations. | `schemas/response.schema.json` |
| Evaluation output | Raw and parsed responses for a model or provider endpoint over questions and trials. | Preserves model output, tool trace, provider metadata, timing data, and errors before scoring. | `results/*/raw_results.jsonl`, `results/*/run.json` |
| Score | Deterministic checks and aggregate metrics for an evaluation output. | Separates answer quality from formatting failures, provider errors, tool failures, latency, and cost. | `results/*/scores.json`, `results/*/scores.jsonl` |
| Provider endpoint | One OpenRouter provider endpoint for one OpenRouter model ID. | Keeps provider routing, quantization, context limits, pricing, and supported parameters separate during evaluation. | `endpoint_variants.jsonl` |
| Accepted endpoint | A provider endpoint that passed the current operational and deliberation filters. | Supplies eligible endpoints for behavior sampling and pool construction. | `variants/filtered-20260529/*` |
| Gene | A behavior-eliciting prompt used after endpoint filtering. | Produces response variation used to compare accepted endpoints beyond question-set scores. | `genes.json`, `sampled-genes.json` |
| Persona | Role text used while sampling gene responses. | Holds the role constant while comparing endpoint behavior on genes. | `personas/generic.md` |
| Cluster assignment | One sampled completion assigned to a per-gene PCA cluster. | Records the behavior group for one endpoint response to one gene. | `clusters.jsonl` |
| Cluster record | One endpoint/persona row with one cluster label per sampled gene. | Summarizes endpoint/persona behavior for pool sampling. | `variant-persona-clusters.jsonl` |
| Pool entry | One selected endpoint/persona row for a model pool. | Provides endpoint/persona records to the pool code. | `pool.jsonl` |

## Question-Set Evaluation

Question-set evaluation tests whether a model or pinned provider endpoint answers the question set correctly and returns the required JSON.  It also records formatting failures, provider errors, tool failures, latency, and cost as separate fields.  Pool filtering uses those separate fields instead of mixing operational failures with answer quality.

Inputs:

| Input | Meaning |
| --- | --- |
| Question file | `sets/core20/questions.jsonl` or `sets/deliberation/questions.jsonl` |
| Target endpoint | `--models openrouter://...`, `--model-spec ...`, `--model-spec-jsonl ...`, or a mock model |
| Trial count | `--trials`, default `3` |
| Evidence records | `sets/core20/fixtures/*` for record-based adjudication questions |
| Output directory | `--out results/<name>` |

Outputs:

| File | Contents |
| --- | --- |
| `raw_results.jsonl` | One response row per target, trial, and question |
| `run.json` | Response rows, model specs, question IDs, trial count, and timestamps |
| `scores.jsonl` | One scored row per response after `tools/score_eval.py score` |
| `scores.json` | Aggregate score summary by model or provider endpoint |

Ordinary questions must return `answer`, `confidence`, `rationale`, and `evidence_ids`.  The evidence list is empty unless the question requires evidence.  Record-based adjudication questions must return `vote`, `confidence`, `rationale`, and `evidence_ids`.  Cited evidence IDs must come from the evidence record.

```json
{"answer":"A","confidence":0.75,"rationale":"One to three sentences.","evidence_ids":[]}
```

```json
{"vote":"demonstrated","confidence":0.75,"rationale":"One to three sentences.","evidence_ids":["E1"]}
```

`sets/core20/questions.jsonl` contains 20 questions: four human-knowledge questions, four science or quantitative questions, four reasoning questions, four instruction-following questions, and four record-based adjudication questions.  `sets/deliberation/questions.jsonl` contains the first twelve knowledge, science, and reasoning questions from `core20` plus eight juror-deliberation questions.  The juror-deliberation questions test burden of proof, evidentiary sufficiency, source reliability, conflicting records, temporal precision, alternative explanations, confidence calibration, and scope control.

The local baseline needs no API key:

```bash
uv run tools/score_eval.py validate-items --questions sets/core20/questions.jsonl
uv run tools/audit_eval.py --json
uv run tools/run_eval.py --mock perfect --models mock:perfect --out results/mock-perfect
uv run tools/score_eval.py score --run results/mock-perfect
```

A two-question OpenRouter evaluation confirms credentials, request formatting, result writing, and scoring:

```bash
uv run tools/run_eval.py \
  --models openrouter://openai/gpt-4.1-mini \
  --limit 2 \
  --out results/openrouter-test

uv run tools/score_eval.py score --run results/openrouter-test
```

An exact provider-endpoint evaluation starts from a JSON spec.  `tools/run_eval.py` adds the OpenRouter metadata header, requests the pinned provider endpoint, disables fallbacks, and records returned generation metadata when OpenRouter provides it.  The spec files under `variants/filtered-20260529/specs/` show the request body used for the checked-in accepted endpoint set.

```bash
uv run tools/run_eval.py \
  --model-spec variants/filtered-20260529/specs/03-openai-gpt-4o-mini-2024-07-18-openai-openai.json \
  --limit 2 \
  --out results/openrouter-provider-endpoint-test

uv run tools/score_eval.py score --run results/openrouter-provider-endpoint-test
```

`deliberation_score` is the mean, across trials, of the fraction of deliberation questions answered correctly on the substantive issue.  Operational metrics report latency, provider failures, malformed JSON, schema violations, invalid votes, tool-call failures, context-limit errors, and cost.  The checked-in accepted endpoint set uses `provider_error_count == 0` and `deliberation_score >= 0.90`.

## Provider Endpoint Selection

Provider endpoint selection records the provider endpoints available for selected OpenRouter model IDs, evaluates each endpoint separately, and applies explicit filter criteria.  `tools/model_inventory.py` fetches the OpenRouter model catalog and endpoint metadata, then writes one normalized row per provider endpoint.  Each row preserves raw model JSON, raw endpoint JSON, provider name, endpoint tag, quantization, context limits, supported parameters, pricing, and status fields when OpenRouter returns them.

```bash
uv run tools/model_inventory.py \
  --run-id model-roots-10-YYYYMMDDTHHMMSSZ \
  --model-id deepseek/deepseek-v4-flash
```

Inventory outputs:

| File | Contents |
| --- | --- |
| `endpoint_variants.jsonl` | One provider endpoint row per OpenRouter endpoint |
| `endpoint_variants.csv` | Inspection table for the endpoint rows |
| `summary.json` | Counts, selected model IDs, provider counts, and endpoint-fetch errors |
| `raw/models.json` | Raw OpenRouter model catalog response |
| `raw/endpoints/*.json` | Raw OpenRouter endpoint metadata responses |

Provider-endpoint evaluations use exact routing constraints.  For known quantization, the request includes `provider.only`, `allow_fallbacks: false`, `require_parameters: true`, and `provider.quantizations`.  For `quantization: "unknown"`, the request still pins the provider endpoint and omits the quantization list.

```json
{
  "provider": {
    "only": ["deepinfra/fp4"],
    "allow_fallbacks": false,
    "require_parameters": true,
    "quantizations": ["fp4"]
  }
}
```

OpenRouter metadata measures the routed OpenRouter product.  Exact weights, GPU type, serving engine, KV-cache precision, hidden provider prompts, and provider changes after the snapshot require provider attestations or controlled deployments.  Model-quality comparisons should keep provider endpoints separate unless the evaluation defines an explicit aggregation rule.

The checked-in accepted endpoint set is `variants/filtered-20260529/`.  It contains 32 accepted provider endpoints from a 72-endpoint source set.  Its filter criteria are recorded in `summary.json`: `provider_error_count == 0` and `deliberation_score >= 0.90`.

Accepted endpoint files:

| File | Contents |
| --- | --- |
| `endpoint_variants.jsonl` | Full provider endpoint rows for accepted endpoints |
| `endpoint_variants.csv` | Inspection table for accepted endpoints |
| `variant_summary.jsonl` | Evaluation summaries for accepted endpoints |
| `manifest.jsonl` | Accepted endpoint manifest rows |
| `specs/*.json` | Exact request specs copied for accepted endpoints |
| `summary.json` | Filter criteria, source paths, accepted endpoint count, and selected source indexes |

## Behavior Clustering And Pool Sampling

Behavior clustering compares accepted endpoints on behavior-eliciting prompts after question-set filtering.  A gene is one behavior prompt, and the current configuration uses `personas/generic.md` as the persona.  For each `gene + provider endpoint + persona`, `tools/run_first_gene_inference_embeddings.py` collects one or more completions and embeds the response text.

Inputs:

| Input | Meaning |
| --- | --- |
| Accepted endpoints | `variants/filtered-20260529/endpoint_variants.jsonl` or a new filtered endpoint file |
| Genes | `sampled-genes.json` or another sampled gene file |
| Persona | `personas/generic.md` unless the evaluation config names another persona file |
| Samples per gene | `--samples` for gene inference and `--expected-samples-per-gene` for aggregation |

Outputs:

| File | Contents |
| --- | --- |
| `records.jsonl` | Gene completions and embedding vectors |
| `pca-records.jsonl` | PCA coordinates for one gene's embedded responses |
| `clusters.jsonl` | One cluster assignment per sampled completion |
| `variant-persona-clusters.jsonl` | One endpoint/persona record with cluster labels ordered by `gene_index` |
| `pool.jsonl` | Sampled endpoint/persona records selected from cluster-label tuples |

PCA is computed separately for each gene because each gene has its own response distribution.  Per-gene clustering assigns each sampled completion to a K-means cluster within that gene.  Aggregation converts sample-level labels into one cluster record per endpoint/persona row, ordered by ascending `gene_index`.

`tools/sample-tuple-pool.py` samples by cluster tuple.  For each emitted pool entry, it chooses one unique cluster tuple uniformly at random, then chooses one row uniformly from rows with that tuple.  Sampling uses replacement, so repeated rows can appear in `pool.jsonl`.

## File Reference

| Path | Contents |
| --- | --- |
| `sets/core20/questions.jsonl` | 20-question core set with knowledge, science, reasoning, instruction-following, and record-based adjudication questions |
| `sets/deliberation/questions.jsonl` | 20-question deliberation set with eight juror-deliberation questions |
| `sets/core20/fixtures/` | Evidence records for record-based questions |
| `schemas/` | JSON schemas for questions, responses, and evaluation outputs |
| `rubrics/core20.md` | Deterministic checks, deliberation score, and operational metrics |
| `tools/run_eval.py` | Model call script for mock models, OpenRouter model IDs, and exact provider specs |
| `tools/score_eval.py` | Question validation and deterministic scoring |
| `tools/model_inventory.py` | OpenRouter model and provider endpoint inventory |
| `tools/run_variant_batch.py` | Provider-endpoint batch evaluation |
| `tools/run_end_to_end.py` | Full selection procedure from root models to `pool.jsonl` |
| `tools/run_first_gene_inference_embeddings.py` | Gene completion and embedding collection |
| `tools/run_embedding_pca.py` | PCA reduction for embedded gene responses |
| `tools/run_gene_pca_clustering.py` | Per-gene K-means clustering |
| `tools/aggregate_variant_persona_clusters.py` | Aggregation from sample-level clusters to endpoint/persona cluster records |
| `tools/sample-tuple-pool.py` | Tuple-uniform pool sampling |
| `variants/filtered-20260529/` | Checked-in accepted endpoint snapshot |
| `results/` | Generated evaluation and pool-construction files.  Git ignores this directory except for `.gitkeep`. |

## Detailed References

[Sampling Runbook](docs/sampling-runbook.md) contains the staged procedure from OpenRouter root sampling through tuple-uniform pool sampling.  [OpenRouter Inventory Procedure](docs/model-inventory.md) explains provider-endpoint identity, routing constraints, metadata fields, and interpretation limits.  [Core20 Rubric](rubrics/core20.md) defines response schemas, deterministic checks, deliberation scoring, and operational metrics.
