# Adjudication Evals

`evals/` contains small adjudication eval tools and the endpoint-variant sampling tools used to build juror and council model pools.  The checked-in eval sets are small enough for manual audit: a 20-item core set and a 20-item deliberation set.  The directory also contains the current checked-in filtered endpoint-variant snapshot, gene-response clustering tools, and pool samplers.

The runner requests strict JSON responses and records raw outputs, parsed responses, tool traces, provider metadata, and timing data.  Generated run artifacts belong under `results/`, which is ignored except for `results/.gitkeep`.  The current checked-in endpoint-variant survivor set lives under `variants/filtered-20260529/`.

## Contents

| Path | Contents |
| --- | --- |
| `sets/core20/questions.jsonl` | The canonical 20-item core eval set. |
| `sets/deliberation/questions.jsonl` | A 20-item deliberation set used for `deliberation_score`. |
| `sets/core20/fixtures/` | Bounded evidence records for tool/evidence items. |
| `schemas/` | JSON Schemas for items, model responses, and run results. |
| `rubrics/core20.md` | Deterministic scoring rules. |
| `prompts/` | Single-juror and council-member prompt wrappers. |
| `personas/generic.md` | The current generic persona for gene-response sampling. |
| `config/` | Model-pool configuration files. |
| `tools/run_eval.py` | The mock and OpenRouter runner, including function-tool mode for bounded-record items. |
| `tools/score_eval.py` | The item validator and deterministic scorer. |
| `tools/audit_eval.py` | The repository consistency audit. |
| `tools/tool_server.py` | Local read-only evidence tools used by the runner. |
| `tools/model_inventory.py` | The OpenRouter model/provider endpoint inventory tool. |
| `tools/run_end_to_end.py` | The end-to-end pipeline runner that calls the inventory, eval, filtering, gene, PCA, clustering, aggregation, and pool tools. |
| `tools/run_variant_batch.py` | The endpoint-variant batch evaluator. |
| `tools/run_first_gene_inference_embeddings.py` | The gene-response and embedding runner. |
| `tools/run_embedding_pca.py` | The embedding PCA reducer. |
| `tools/run_gene_pca_clustering.py` | The per-gene PCA clustering tool. |
| `tools/aggregate_variant_persona_clusters.py` | The cluster-vector aggregator for variant/persona rows. |
| `variants/filtered-20260529/` | A 32-row survivor endpoint-variant snapshot with exact request specs. |
| `genes.json` and `sampled-genes.json` | The source gene list and the sampled gene subset used by the clustering workflow. |
| `tools/sample-pool.py`, `tools/sample-diverse-pool.py`, and `tools/sample-tuple-pool.py` | Pool samplers for variant/persona cluster rows. |
| `docs/model-inventory.md` | Detailed notes for OpenRouter endpoint inventory work. |
| `docs/sampling-runbook.md` | The repeatable sampling workflow from root-model inventory through tuple-uniform pool sampling. |

## Quick Start

Validate the core items:

```bash
uv run tools/score_eval.py validate-items --questions sets/core20/questions.jsonl
```

Validate the deliberation items:

```bash
uv run tools/score_eval.py validate-items --questions sets/deliberation/questions.jsonl
```

Run the internal consistency audit:

```bash
uv run tools/audit_eval.py --json
```

Run and score a deterministic local baseline.  The runner defaults to three repeated trials per model/item, so the 20-item core set produces 60 result rows.

```bash
uv run tools/run_eval.py --mock perfect --models mock:perfect --out results/mock-perfect
uv run tools/score_eval.py score --run results/mock-perfect
```

Run a minimal OpenRouter test:

```bash
uv run tools/run_eval.py --models openrouter://openai/gpt-4.1-mini --limit 2 --out results/openrouter-test
uv run tools/score_eval.py score --run results/openrouter-test
```

Run a single exact OpenRouter endpoint variant from a JSON spec:

```bash
uv run tools/run_eval.py --model-spec variants/filtered-20260529/specs/03-openai-gpt-4o-mini-2024-07-18-openai-openai.json --limit 2 --out results/openrouter-variant-test
uv run tools/score_eval.py score --run results/openrouter-variant-test
```

Run endpoint variants from JSONL inventory/spec rows:

```bash
uv run tools/run_eval.py --model-spec-jsonl variants/filtered-20260529/endpoint_variants.jsonl --limit 2 --out results/openrouter-variant-jsonl-test
uv run tools/score_eval.py score --run results/openrouter-variant-jsonl-test
```

Run the deliberation eval view:

```bash
uv run tools/run_eval.py --questions sets/deliberation/questions.jsonl --models openrouter://openai/gpt-4.1-mini --out results/deliberation-openrouter-test
uv run tools/score_eval.py score --questions sets/deliberation/questions.jsonl --run results/deliberation-openrouter-test
```

Use `--trials 1` for a single-pass test run.  The default of three trials supports stability fields in the scorer.

Run one real function-tool test item:

```bash
uv run tools/run_eval.py --questions sets/core20/questions.jsonl --item-id core20.tool.001 --tool-mode function --models openrouter://openai/gpt-4.1-mini --out results/tool-function-openrouter-test
uv run tools/score_eval.py score --questions sets/core20/questions.jsonl --run results/tool-function-openrouter-test
```

The OpenRouter tools read `OPENROUTER_API_KEY` from the environment first.  If the variable is absent, they check `secrets/openrouter.api.txt`, which is ignored.  The gene-response embedding runner also needs `OPENAI_API_KEY`; if that variable is absent, it checks ignored `secrets/openai.api.txt`.

## End-To-End Runner

`tools/run_end_to_end.py` runs the full endpoint-variant pool pipeline by calling the existing tools.  It writes one top-level run directory with `manifest.json`, `commands.jsonl`, stage subdirectories, and `summary.json`.

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

The default run shape is intentionally small: five root models, one eval trial, two genes, and one sample per gene.  Use `--model-id` to evaluate explicit OpenRouter model IDs instead of sampling roots.  Use `--resume` to reuse completed stage outputs, `--stop-after` to stop at a stage boundary, and `--strict-pca-dimensions` to fail rather than cap PCA dimensions to the available row count.  The eval stage passes `--timeout` as the per-request timeout and uses `--eval-no-progress-timeout` to terminate a variant child process that stops writing output or result rows.

## OpenRouter Endpoint Variants

`tools/model_inventory.py` builds a static OpenRouter endpoint inventory.  A variant row represents one provider endpoint/configuration for one OpenRouter model ID.  The row preserves provider, endpoint tag, quantization, context limits, supported parameters, pricing, status fields when OpenRouter exposes them, raw JSON paths, and raw JSON hashes.

Run a sampled inventory:

```bash
uv run tools/model_inventory.py --sample-models 5
```

Run a specific model:

```bash
uv run tools/model_inventory.py --model-id deepseek/deepseek-v4-flash
```

Outputs are written under `results/<run-id>/`:

```text
endpoint_variants.jsonl
endpoint_variants.csv
summary.json
summary.md
raw/models.json
raw/endpoints/*.json
```

Use these fields from a variant JSON object when constructing an exact OpenRouter request:

| Field | Request use |
| --- | --- |
| `openrouter_model_id` | `model` |
| `endpoint_tag` | `provider.only[0]` when present |
| `provider_name` | Fallback `provider.only` value when `endpoint_tag` is absent |
| `quantization` | `provider.quantizations` when the value is known |
| `supported_parameters` | Runtime parameters safe to send |

For a known-quantization endpoint, the request shape is:

```json
{
  "model": "<openrouter_model_id>",
  "messages": [
    {"role": "user", "content": "..."}
  ],
  "temperature": 0,
  "top_p": 1,
  "max_tokens": 1024,
  "provider": {
    "only": ["<endpoint_tag>"],
    "allow_fallbacks": false,
    "require_parameters": true,
    "quantizations": ["<quantization>"]
  }
}
```

For `quantization: "unknown"`, pin by endpoint tag or provider tag and omit `provider.quantizations`:

```json
{
  "model": "deepseek/deepseek-v4-flash",
  "messages": [
    {"role": "user", "content": "Answer with exactly: OK"}
  ],
  "temperature": 0,
  "top_p": 1,
  "max_tokens": 32,
  "provider": {
    "only": ["alibaba"],
    "allow_fallbacks": false,
    "require_parameters": true
  }
}
```

Exact-variant runs set this HTTP header:

```text
X-OpenRouter-Experimental-Metadata: enabled
```

After each OpenRouter call, verify the routed endpoint from response metadata and `/api/v1/generation?id=<generation_id>`.  Record provider, endpoint, usage, cost, latency, native token counts, and upstream IDs when OpenRouter returns them.  This request pattern measures the routed OpenRouter endpoint product, but it cannot establish exact weights, serving engine, GPU type, KV-cache precision, hidden provider prompt templates, or provider-side changes after the inventory snapshot.

`tools/run_eval.py` supports variant rows through `--model-spec` for one JSON object or `--model-spec-jsonl` for JSONL.  Existing `--models openrouter://...` runs still measure the requested OpenRouter model ID without endpoint pinning.  A variant spec run adds provider constraints to the request body, enables the metadata header, stores requested provider/quantization policy in result metadata, and attempts to attach post-hoc generation metadata for each response.

## Variant Batch Evals

Use `tools/run_variant_batch.py` to evaluate an endpoint inventory one variant at a time.  The batch runner writes one exact spec file per variant, calls `tools/run_eval.py`, scores each completed run with `tools/score_eval.py`, and writes per-variant summary rows.  It resumes variants already present in `progress.jsonl`.

```bash
uv run --script tools/run_variant_batch.py \
  --variants results/model-roots-10-YYYYMMDDTHHMMSSZ/endpoint_variants.jsonl \
  --out results/model-roots-10-eval-YYYYMMDDTHHMMSSZ \
  --questions sets/core20/questions.jsonl \
  --trials 3 \
  --timeout 90
```

The batch output includes these files:

| File | Contents |
| --- | --- |
| `specs/*.json` | Exact OpenRouter variant specs used for requests. |
| `variant-runs/*/run_eval.log` | Child-process output for one variant. |
| `variant-runs/*/raw_results.jsonl` | Raw eval rows for one variant. |
| `variant-runs/*/scores.json` | Scored summary for one variant. |
| `progress.jsonl` | Per-variant completion records. |
| `variant_summary.csv` | Tabular per-variant summary. |
| `summary.json` | Batch status and aggregate counts. |

The batch runner also has `--no-progress-timeout` and `--variant-timeout`.  `--timeout` remains the per-request timeout passed to `tools/run_eval.py`; `--no-progress-timeout` terminates a child process that stops writing output or result rows.  A timed-out variant remains in `progress.jsonl` and `variant_summary.csv`, and downstream filtering excludes it before gene inference.  A child crash or scoring failure still makes the batch fail because it indicates an eval-tool failure rather than an endpoint timeout.

To stop a long run, create the `STOP` file named in the runner's JSON status output.  The runner terminates the active child process and exits without starting another variant.  Use `--resume` after inspecting `progress.jsonl` if the remaining variants should continue.

## Filtered Variants

`variants/filtered-20260529/` contains the active checked-in survivor set.  `variants/filtered-20260529/summary.json` records the filter criteria: `provider_error_count == 0` and `deliberation_score >= 0.90`.

The directory contains the full survivor rows, an inspection CSV, per-variant eval summaries, manifest rows, exact request specs, and a summary file:

```text
variants/filtered-20260529/endpoint_variants.jsonl
variants/filtered-20260529/endpoint_variants.csv
variants/filtered-20260529/variant_summary.jsonl
variants/filtered-20260529/manifest.jsonl
variants/filtered-20260529/specs/*.json
variants/filtered-20260529/summary.json
```

OpenRouter endpoint availability and provider behavior can change after the snapshot.  Claims about current provider reliability or current model behavior require a refreshed inventory and fresh evals.  `docs/sampling-runbook.md` contains the repeatable workflow that produces a new filtered set.

## Gene Clustering And Pool Sampling

The sampling pipeline starts with filtered endpoint variants and ends with a JSONL pool sampled from variant/persona cluster vectors.  The detailed procedure is in `docs/sampling-runbook.md`.  This section gives the command shapes for the current scripts.

Run one sampled gene through the filtered variants and embed the responses:

```bash
uv run --script tools/run_first_gene_inference_embeddings.py \
  --variants variants/filtered-20260529/endpoint_variants.jsonl \
  --genes sampled-genes.json \
  --persona personas/generic.md \
  --samples 3 \
  --gene-index 0 \
  --out results/gene-1-inference-embeddings-YYYYMMDDTHHMMSSZ
```

Run PCA for one gene response set:

```bash
uv run --script tools/run_embedding_pca.py \
  --records results/gene-1-inference-embeddings-YYYYMMDDTHHMMSSZ/records.jsonl \
  --out results/gene-1-pca-3d-YYYYMMDDTHHMMSSZ \
  --dimensions 3
```

Cluster the per-gene PCA outputs:

```bash
uv run --script tools/run_gene_pca_clustering.py \
  --pca-records results/gene-1-pca-3d-YYYYMMDDTHHMMSSZ/pca-records.jsonl \
  --pca-records results/gene-2-pca-3d-YYYYMMDDTHHMMSSZ/pca-records.jsonl \
  --pca-records results/gene-3-pca-3d-YYYYMMDDTHHMMSSZ/pca-records.jsonl \
  --pca-records results/gene-4-pca-3d-YYYYMMDDTHHMMSSZ/pca-records.jsonl \
  --out results/gene-clusters-YYYYMMDDTHHMMSSZ \
  --expected-rows-per-gene 96 \
  --expected-variants-per-gene 32 \
  --expected-samples-per-variant 3
```

Aggregate sample-level clusters into one variant/persona row per endpoint variant:

```bash
uv run --script tools/aggregate_variant_persona_clusters.py \
  --clusters results/gene-clusters-YYYYMMDDTHHMMSSZ/clusters.jsonl \
  --cluster-fit results/gene-clusters-YYYYMMDDTHHMMSSZ/cluster-fit.json \
  --variants variants/filtered-20260529/endpoint_variants.jsonl \
  --out results/variant-persona-clusters-YYYYMMDDTHHMMSSZ \
  --expected-samples-per-gene 3
```

Sample a tuple-uniform pool:

```bash
uv run --script tools/sample-tuple-pool.py \
  results/variant-persona-clusters-YYYYMMDDTHHMMSSZ/variant-persona-clusters.jsonl \
  --out results/sample-tuple-pool-YYYYMMDDTHHMMSSZ/pool.jsonl \
  --diagnostics-out results/sample-tuple-pool-YYYYMMDDTHHMMSSZ/diagnostics.jsonl \
  --pool-size 20 \
  --seed 0
```

The tuple sampler groups rows by exact `clusters` vector, chooses a unique cluster tuple uniformly for each emitted row, and then chooses one row uniformly from that tuple.  Sampling is with replacement.  The diagnostics file records the selected tuple, source row, model ID, provider, endpoint tag, quantization, and cumulative counts.

## Score Model

The scorer separates deliberation quality from operational behavior:

| Field | Meaning |
| --- | --- |
| `deliberation_score` | Mean trial score for substantive knowledge, science, reasoning, and juror-deliberation items. |
| `trial_scores` | Per-trial deliberation scores. |
| `deliberation_score_stddev` | Population standard deviation over trial scores. |
| `deliberation_score_min` and `deliberation_score_max` | Trial-score range. |
| `item_variation_count` | Count of items with different response values across trials. |
| `operational_metrics` | Latency, timeouts, provider errors, malformed JSON, schema violations, invalid votes, tool-call failures, context-limit errors, and cost. |

Pool selection filters and ranks over those fields explicitly.  Operational failures and substantive deliberation failures are different facts.  The score output keeps them separate.

## Scope

The core eval set catches malformed JSON, brittle instruction following, weak record use, unsupported citations, and obvious reasoning failures.  The endpoint-variant tooling evaluates OpenRouter routed products under explicit provider and quantization constraints.  Full adjudication runs live in `adc/`, `arb/`, and `arbd/`.
