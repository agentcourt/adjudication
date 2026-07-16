# Sampling instructions

This file records the standing procedure for sampling root models and the specific sampling work performed during the run.

## Root-model sampling procedure

Use the OpenRouter model catalog as the sampling frame. A root model is an OpenRouter model ID from `/api/v1/models`. Endpoint variants are then fetched from `/api/v1/models/{author}/{slug}/endpoints`; each provider endpoint remains a separate variant. Do not collapse variants that share an OpenRouter model ID, and do not collapse `unknown` quantization endpoints.

For incremental samples, exclude root models already present in the active combined root-model catalog unless the run requires overlap. Use deterministic sampling and record the seed, exclusion set, selected roots, command, run ID, and output files. The inventory step is static catalog work only; it does not run inference.

## Task: sample 10 additional root models

Started: 2026-05-29T14:09:14Z.

Existing combined catalog: `results/model-roots-10-combined-variants-20260529T1137Z`.

Existing root models excluded:

- `anthropic/claude-opus-4.6-fast`
- `arcee-ai/trinity-mini`
- `deepseek/deepseek-v4-flash`
- `google/gemini-2.5-flash-lite`
- `minimax/minimax-m2.7`
- `mistralai/devstral-2512`
- `openai/gpt-4o-mini-2024-07-18`
- `openai/gpt-5.2-codex`
- `qwen/qwen-plus`
- `qwen/qwen3-vl-8b-thinking`

Selection rule: sort the OpenRouter model IDs lexicographically, sample 10 root models with deterministic seed `2`, and require zero overlap with the excluded existing roots. Seed `2` was the first next seed checked after the prior `0` and `1` samples that produced a 10-model sample with no overlap against `results/model-roots-10-combined-variants-20260529T1137Z`.

Selection source file: `results/model-roots-sample-5-more-20260529T1136Z/raw/models.json`. That file was a saved OpenRouter `/api/v1/models` response with 357 model rows. The later inventory command fetched the model catalog again and fetched endpoint variants for the selected model IDs.

Selection reproducer:

```bash
uv run python - <<'PY'
import json
import random
from pathlib import Path

prior = set(json.loads(
    Path("results/model-roots-10-combined-variants-20260529T1137Z/summary.json").read_text()
)["model_roots"])

models = json.loads(
    Path("results/model-roots-sample-5-more-20260529T1136Z/raw/models.json").read_text()
)["data"]

ordered = sorted(model["id"] for model in models)

for seed in range(2, 50):
    sample = sorted(random.Random(seed).sample(ordered, 10))
    if not any(model_id in prior for model_id in sample):
        print("seed", seed)
        print("\n".join(sample))
        break
PY
```

Verification command:

```bash
comm -12 \
  <(jq -r '.model_roots[]' results/model-roots-10-combined-variants-20260529T1137Z/summary.json | sort) \
  <(jq -r '.selected_model_ids[]' results/model-roots-10-more-20260529T140914Z/summary.json | sort) \
  | wc -l
```

The verification returned `0`.

Selected root models:

- `arcee-ai/coder-large`
- `bytedance/ui-tars-1.5-7b`
- `cohere/command-r-08-2024`
- `google/gemma-4-26b-a4b-it`
- `meta-llama/llama-3.1-70b-instruct`
- `minimax/minimax-m2.5`
- `moonshotai/kimi-k2-thinking`
- `openai/gpt-4-0314`
- `sao10k/l3-euryale-70b`
- `z-ai/glm-4.6v`

Inventory run ID: `model-roots-10-more-20260529T140914Z`.

Inventory command:

```bash
uv run tools/model_inventory.py \
  --run-id model-roots-10-more-20260529T140914Z \
  --model-id arcee-ai/coder-large \
  --model-id bytedance/ui-tars-1.5-7b \
  --model-id cohere/command-r-08-2024 \
  --model-id google/gemma-4-26b-a4b-it \
  --model-id meta-llama/llama-3.1-70b-instruct \
  --model-id minimax/minimax-m2.5 \
  --model-id moonshotai/kimi-k2-thinking \
  --model-id openai/gpt-4-0314 \
  --model-id sao10k/l3-euryale-70b \
  --model-id z-ai/glm-4.6v
```

Status: completed.

Inventory result:

- Output directory: `results/model-roots-10-more-20260529T140914Z`
- Selected root models: 10
- Endpoint variants: 40
- Endpoint fetch errors: 0
- Catalog model count at snapshot: 357
- Quantization counts: `bf16` 11, `fp8` 16, `int4` 1, `unknown` 12
- Unknown-quantization endpoint variants: 12

Endpoint variants by selected root model:

| Root model | Endpoint variants |
| --- | ---: |
| `arcee-ai/coder-large` | 1 |
| `bytedance/ui-tars-1.5-7b` | 1 |
| `cohere/command-r-08-2024` | 1 |
| `google/gemma-4-26b-a4b-it` | 9 |
| `meta-llama/llama-3.1-70b-instruct` | 4 |
| `minimax/minimax-m2.5` | 17 |
| `moonshotai/kimi-k2-thinking` | 3 |
| `openai/gpt-4-0314` | 1 |
| `sao10k/l3-euryale-70b` | 1 |
| `z-ai/glm-4.6v` | 2 |

Output files:

- `results/model-roots-10-more-20260529T140914Z/raw/models.json`
- `results/model-roots-10-more-20260529T140914Z/raw/endpoints/*.json`
- `results/model-roots-10-more-20260529T140914Z/endpoint_variants.jsonl`
- `results/model-roots-10-more-20260529T140914Z/endpoint_variants.csv`
- `results/model-roots-10-more-20260529T140914Z/summary.json`
- `results/model-roots-10-more-20260529T140914Z/summary.md`

Combined catalog:

- Output directory: `results/model-roots-20-combined-variants-20260529T1410Z`
- Source files: `results/model-roots-10-combined-variants-20260529T1137Z/endpoint_variants.jsonl` and `results/model-roots-10-more-20260529T140914Z/endpoint_variants.jsonl`
- Root models: 20
- Endpoint variants: 72
- Files: `endpoint_variants.jsonl`, `summary.json`

## Task: get model variants for the 10 new root models

Started: 2026-05-29T14:13:26Z.

Scope: only the 10 root models selected in `model-roots-10-more-20260529T140914Z`. Do not include the prior 10 roots and do not use the 20-root combined catalog for this task.

Variant source file: `results/model-roots-10-more-20260529T140914Z/endpoint_variants.jsonl`.

The model variants were obtained by fetching OpenRouter endpoint metadata for each of the 10 selected root model IDs:

```bash
uv run tools/model_inventory.py \
  --run-id model-roots-10-more-20260529T140914Z \
  --model-id arcee-ai/coder-large \
  --model-id bytedance/ui-tars-1.5-7b \
  --model-id cohere/command-r-08-2024 \
  --model-id google/gemma-4-26b-a4b-it \
  --model-id meta-llama/llama-3.1-70b-instruct \
  --model-id minimax/minimax-m2.5 \
  --model-id moonshotai/kimi-k2-thinking \
  --model-id openai/gpt-4-0314 \
  --model-id sao10k/l3-euryale-70b \
  --model-id z-ai/glm-4.6v
```

Result:

- Root models: 10
- Endpoint variants: 40
- Endpoint fetch errors: 0
- Output JSONL: `results/model-roots-10-more-20260529T140914Z/endpoint_variants.jsonl`
- Output CSV: `results/model-roots-10-more-20260529T140914Z/endpoint_variants.csv`
- Summary: `results/model-roots-10-more-20260529T140914Z/summary.json`

Model variants:

| Root model | Provider | Endpoint tag | Quantization | Context | Max completion | Supported params |
| --- | --- | --- | --- | ---: | ---: | ---: |
| `arcee-ai/coder-large` | Together | `together` | `unknown` | 32768 |  | 10 |
| `bytedance/ui-tars-1.5-7b` | Parasail | `parasail/bf16` | `bf16` | 128000 | 2048 | 10 |
| `cohere/command-r-08-2024` | Cohere | `cohere` | `unknown` | 128000 | 4000 | 12 |
| `google/gemma-4-26b-a4b-it` | DekaLLM | `dekallm/bf16` | `bf16` | 262144 |  | 12 |
| `google/gemma-4-26b-a4b-it` | DeepInfra | `deepinfra/fp8` | `fp8` | 262144 | 16384 | 17 |
| `google/gemma-4-26b-a4b-it` | Cloudflare | `cloudflare` | `unknown` | 256000 | 256000 | 16 |
| `google/gemma-4-26b-a4b-it` | SiliconFlow | `siliconflow/fp8` | `fp8` | 262144 | 262144 | 9 |
| `google/gemma-4-26b-a4b-it` | Parasail | `parasail/bf16` | `bf16` | 262144 | 262144 | 14 |
| `google/gemma-4-26b-a4b-it` | Novita | `novita/bf16` | `bf16` | 262144 | 131072 | 14 |
| `google/gemma-4-26b-a4b-it` | NextBit | `nextbit/bf16` | `bf16` | 262144 | 262144 | 16 |
| `google/gemma-4-26b-a4b-it` | Google | `google-vertex/global` | `unknown` | 262144 | 262144 | 15 |
| `google/gemma-4-26b-a4b-it` | Venice | `venice/bf16` | `bf16` | 256000 | 8192 | 15 |
| `meta-llama/llama-3.1-70b-instruct` | DeepInfra | `deepinfra/turbo` | `fp8` | 131072 | 16384 | 15 |
| `meta-llama/llama-3.1-70b-instruct` | DeepInfra | `deepinfra/base` | `bf16` | 131072 | 16384 | 15 |
| `meta-llama/llama-3.1-70b-instruct` | Amazon Bedrock | `amazon-bedrock` | `unknown` | 131072 | 8192 | 5 |
| `meta-llama/llama-3.1-70b-instruct` | WandB | `wandb/bf16` | `bf16` | 128000 | 128000 | 10 |
| `minimax/minimax-m2.5` | AkashML | `akashml/fp8` | `fp8` | 196608 | 196608 | 15 |
| `minimax/minimax-m2.5` | DeepInfra | `deepinfra/fp8` | `fp8` | 196608 | 131072 | 16 |
| `minimax/minimax-m2.5` | Chutes | `chutes/fp8` | `fp8` | 196608 | 65536 | 15 |
| `minimax/minimax-m2.5` | Phala | `phala` | `unknown` | 196608 | 196608 | 16 |
| `minimax/minimax-m2.5` | Inceptron | `inceptron/fp8` | `fp8` | 196608 | 196608 | 18 |
| `minimax/minimax-m2.5` | Baidu | `baidu/fp8` | `fp8` | 196608 | 131072 | 14 |
| `minimax/minimax-m2.5` | AtlasCloud | `atlas-cloud/fp8` | `fp8` | 196608 | 196608 | 17 |
| `minimax/minimax-m2.5` | Mara | `mara` | `unknown` | 196608 | 196608 | 7 |
| `minimax/minimax-m2.5` | Friendli | `friendli` | `unknown` | 196608 | 196608 | 16 |
| `minimax/minimax-m2.5` | Minimax | `minimax/fp8` | `fp8` | 204800 | 131072 | 8 |
| `minimax/minimax-m2.5` | Novita | `novita/fp8` | `fp8` | 204800 | 131100 | 14 |
| `minimax/minimax-m2.5` | SiliconFlow | `siliconflow/fp8` | `fp8` | 196608 | 131072 | 11 |
| `minimax/minimax-m2.5` | Parasail | `parasail/fp8` | `fp8` | 196608 | 196608 | 16 |
| `minimax/minimax-m2.5` | WandB | `wandb/fp8` | `fp8` | 196608 | 196608 | 15 |
| `minimax/minimax-m2.5` | StreamLake | `streamlake` | `unknown` | 200000 | 128000 | 9 |
| `minimax/minimax-m2.5` | Venice | `venice` | `unknown` | 198000 | 32768 | 11 |
| `minimax/minimax-m2.5` | Minimax | `minimax/highspeed` | `fp8` | 204800 | 131072 | 8 |
| `moonshotai/kimi-k2-thinking` | Google | `google-vertex` | `unknown` | 262144 | 262144 | 15 |
| `moonshotai/kimi-k2-thinking` | AtlasCloud | `atlas-cloud/int4` | `int4` | 262144 | 262144 | 17 |
| `moonshotai/kimi-k2-thinking` | Novita | `novita/bf16` | `bf16` | 262144 | 262144 | 15 |
| `openai/gpt-4-0314` | OpenAI | `openai` | `unknown` | 8191 | 4096 | 14 |
| `sao10k/l3-euryale-70b` | Novita | `novita/bf16` | `bf16` | 8192 | 8192 | 11 |
| `z-ai/glm-4.6v` | Z.AI | `z-ai/fp8` | `fp8` | 131072 | 24000 | 7 |
| `z-ai/glm-4.6v` | Novita | `novita/bf16` | `bf16` | 131072 | 32768 | 14 |

Verification:

```bash
jq -s '([.[].openrouter_model_id] | unique | length) as $roots | {rows:length, roots:$roots}' \
  results/model-roots-10-more-20260529T140914Z/endpoint_variants.jsonl
```

The verification returned 40 rows and 10 unique root models.

## Task: run adjudication evals for the 10 new root models

Started: 2026-05-29T14:22:01Z.

Scope: run the adjudication evals on the 40 endpoint variants in `results/model-roots-10-more-20260529T140914Z/endpoint_variants.jsonl`. This task covers only the 10 newly sampled root models and their variants. It does not include the prior 10-root catalog or the combined 20-root catalog.

Run directory: `results/model-roots-10-more-eval-20260529T142201Z`.

Eval settings:

- Questions: `sets/core20/questions.jsonl`
- Trials per variant: `3`
- Expected rows per variant: `60`
- Concurrency limit: up to `5` active variant evals at a time
- Variant constraint policy: use each endpoint variant row as an exact OpenRouter request spec, including `provider.only`, `allow_fallbacks: false`, `require_parameters: true`, and non-`unknown` quantization constraints
- Metadata policy: preserve inline OpenRouter router metadata and post-hoc `/generation` metadata where OpenRouter returns it
- Reporting policy: record each variant start and each variant result

Each variant receives a separate spec file under `results/model-roots-10-more-eval-20260529T142201Z/specs/` and a separate scored run directory under `results/model-roots-10-more-eval-20260529T142201Z/variant-runs/`. The per-variant result directory contains `raw_results.jsonl`, `run.json`, `scores.json`, and `scores.jsonl`.

Execution procedure:

1. Read `results/model-roots-10-more-20260529T140914Z/endpoint_variants.jsonl` as the authoritative variant list for this task. Each JSONL row is one eval target.

2. Create `results/model-roots-10-more-eval-20260529T142201Z/manifest.jsonl`. Each manifest row records the variant index, root model, provider, endpoint tag, quantization, endpoint variant ID, generated spec path, generated run directory, and human-readable label.

3. Write one exact OpenRouter model-spec JSON file per variant under `results/model-roots-10-more-eval-20260529T142201Z/specs/`. The spec is the original endpoint-variant inventory row. `tools/run_eval.py` converts that row into the OpenRouter request policy: `provider.only` from the endpoint tag/provider, `allow_fallbacks: false`, `require_parameters: true`, `provider.quantizations` when quantization is known, and `X-OpenRouter-Experimental-Metadata: enabled`.

4. Maintain a queue over the 40 manifest rows. Start no more than five active variant evals at a time. When a slot opens, start the next queued variant.

5. For each started variant, record this status form:

```text
Starting variant {index}/40: `{openrouter_model_id}` @ {provider_name} (`{endpoint_tag}`, `{quantization}`). Run: `{run_dir}`
```

6. Run the variant eval with this command shape:

```bash
uv run tools/run_eval.py \
  --questions sets/core20/questions.jsonl \
  --model-spec "$SPEC_PATH" \
  --out "$VARIANT_RUN_DIR" \
  --trials 3 \
  --timeout 90
```

7. Score the variant immediately after `run_eval.py` exits:

```bash
uv run tools/score_eval.py score \
  --questions sets/core20/questions.jsonl \
  --run "$VARIANT_RUN_DIR"
```

8. Parse the per-variant `scores.json` and append one result row to `results/model-roots-10-more-eval-20260529T142201Z/progress.jsonl`. The row records variant identity, run directory, result rows, completed count, provider errors, schema violations, timeouts, context-limit errors, deliberation score, cost, and runner/scoring exit codes.

9. For each stopped variant, record this status form:

```text
Stopped variant {index}/40: `{openrouter_model_id}` @ {provider_name} (`{endpoint_tag}`, `{quantization}`). Results: completed {completed}/{result_rows}, provider errors {provider_errors}, schema violations {schema_violations}, timeouts {timeouts}, context-limit errors {context_limit_errors}, deliberation score {score}, cost {cost}. Run: `{run_dir}`
```

10. After all variants stop, aggregate `progress.jsonl` into `summary.json` and `variant_summary.csv`, then copy the aggregate counts and variant table into this file.

Result: completed.

Run outputs:

- Manifest: `results/model-roots-10-more-eval-20260529T142201Z/manifest.jsonl`
- Progress/results records: `results/model-roots-10-more-eval-20260529T142201Z/progress.jsonl`
- Aggregate summary: `results/model-roots-10-more-eval-20260529T142201Z/summary.json`
- Variant summary CSV: `results/model-roots-10-more-eval-20260529T142201Z/variant_summary.csv`
- Spec files: `results/model-roots-10-more-eval-20260529T142201Z/specs/*.json`
- Scored variant runs: `results/model-roots-10-more-eval-20260529T142201Z/variant-runs/*/`

Aggregate result:

- Variant evals: 40
- Result rows: 2,400
- Completed responses: 1,487
- Provider errors: 913
- Schema violations among completed responses: 84
- Timeouts: 0
- Context-limit errors: 0
- Runner or scoring failures: 0
- Total recorded OpenRouter cost: 0.692510049
- Zero-completion variants: 1, 2, 4, 12, 15, 16, 24, 31, 32, 36, 37, 38, 39
- Full-completion variants: 3, 6, 7, 9, 10, 11, 13, 14, 18, 22, 23, 25, 26, 27, 28, 29, 30, 33, 34, 35, 40

Verification:

```bash
uv run python - <<'PY'
import json
from pathlib import Path

summary = json.loads(Path(
    "results/model-roots-10-more-eval-20260529T142201Z/summary.json"
).read_text())

assert summary["variant_count"] == 40
assert summary["unique_variant_indexes"] == 40
assert summary["result_rows_total"] == 2400
assert summary["runner_or_scoring_failures"] == 0
assert len(summary["variants"]) == 40

for row in summary["variants"]:
    run_dir = Path(row["run_dir"])
    for name in ("raw_results.jsonl", "run.json", "scores.json", "scores.jsonl"):
        assert (run_dir / name).exists(), f"missing {run_dir / name}"

print({
    "ok": True,
    "variants": summary["variant_count"],
    "rows": summary["result_rows_total"],
    "completed": summary["completed_total"],
    "provider_errors": summary["provider_error_total"],
    "schema_violations": summary["schema_violation_total"],
})
PY
```

The verification returned:

```json
{"completed": 1487, "ok": true, "provider_errors": 913, "rows": 2400, "schema_violations": 84, "variants": 40}
```

Variant results:

| # | Root model | Provider | Endpoint | Quant | Completed | Provider errors | Schema | Score |
| ---: | --- | --- | --- | --- | ---: | ---: | ---: | ---: |
| 1 | `arcee-ai/coder-large` | Together | `together` | `unknown` | 0/60 | 60 | 0 |  |
| 2 | `bytedance/ui-tars-1.5-7b` | Parasail | `parasail/bf16` | `bf16` | 0/60 | 60 | 0 |  |
| 3 | `cohere/command-r-08-2024` | Cohere | `cohere` | `unknown` | 60/60 | 0 | 0 | 0.75 |
| 4 | `google/gemma-4-26b-a4b-it` | DekaLLM | `dekallm/bf16` | `bf16` | 0/60 | 60 | 0 |  |
| 5 | `google/gemma-4-26b-a4b-it` | DeepInfra | `deepinfra/fp8` | `fp8` | 7/60 | 53 | 0 | 1 |
| 6 | `google/gemma-4-26b-a4b-it` | Cloudflare | `cloudflare` | `unknown` | 60/60 | 0 | 0 | 1 |
| 7 | `google/gemma-4-26b-a4b-it` | SiliconFlow | `siliconflow/fp8` | `fp8` | 60/60 | 0 | 0 | 1 |
| 8 | `google/gemma-4-26b-a4b-it` | Parasail | `parasail/bf16` | `bf16` | 15/60 | 45 | 0 | 1 |
| 9 | `google/gemma-4-26b-a4b-it` | Novita | `novita/bf16` | `bf16` | 60/60 | 0 | 0 | 1 |
| 10 | `google/gemma-4-26b-a4b-it` | NextBit | `nextbit/bf16` | `bf16` | 60/60 | 0 | 0 | 1 |
| 11 | `google/gemma-4-26b-a4b-it` | Google | `google-vertex/global` | `unknown` | 60/60 | 0 | 0 | 1 |
| 12 | `google/gemma-4-26b-a4b-it` | Venice | `venice/bf16` | `bf16` | 0/60 | 60 | 0 |  |
| 13 | `meta-llama/llama-3.1-70b-instruct` | DeepInfra | `deepinfra/turbo` | `fp8` | 60/60 | 0 | 0 | 0.75 |
| 14 | `meta-llama/llama-3.1-70b-instruct` | DeepInfra | `deepinfra/base` | `bf16` | 60/60 | 0 | 0 | 0.75 |
| 15 | `meta-llama/llama-3.1-70b-instruct` | Amazon Bedrock | `amazon-bedrock` | `unknown` | 0/60 | 60 | 0 |  |
| 16 | `meta-llama/llama-3.1-70b-instruct` | WandB | `wandb/bf16` | `bf16` | 0/60 | 60 | 0 |  |
| 17 | `minimax/minimax-m2.5` | AkashML | `akashml/fp8` | `fp8` | 40/60 | 20 | 0 | 1 |
| 18 | `minimax/minimax-m2.5` | DeepInfra | `deepinfra/fp8` | `fp8` | 60/60 | 0 | 60 | 0 |
| 19 | `minimax/minimax-m2.5` | Chutes | `chutes/fp8` | `fp8` | 59/60 | 1 | 1 | 1 |
| 20 | `minimax/minimax-m2.5` | Phala | `phala` | `unknown` | 47/60 | 13 | 2 | 0.963 |
| 21 | `minimax/minimax-m2.5` | Inceptron | `inceptron/fp8` | `fp8` | 59/60 | 1 | 0 | 1 |
| 22 | `minimax/minimax-m2.5` | Baidu | `baidu/fp8` | `fp8` | 60/60 | 0 | 0 | 1 |
| 23 | `minimax/minimax-m2.5` | AtlasCloud | `atlas-cloud/fp8` | `fp8` | 60/60 | 0 | 0 | 1 |
| 24 | `minimax/minimax-m2.5` | Mara | `mara` | `unknown` | 0/60 | 60 | 0 |  |
| 25 | `minimax/minimax-m2.5` | Friendli | `friendli` | `unknown` | 60/60 | 0 | 1 | 0.9722 |
| 26 | `minimax/minimax-m2.5` | Minimax | `minimax/fp8` | `fp8` | 60/60 | 0 | 2 | 0.9722 |
| 27 | `minimax/minimax-m2.5` | Novita | `novita/fp8` | `fp8` | 60/60 | 0 | 0 | 1 |
| 28 | `minimax/minimax-m2.5` | SiliconFlow | `siliconflow/fp8` | `fp8` | 60/60 | 0 | 0 | 1 |
| 29 | `minimax/minimax-m2.5` | Parasail | `parasail/fp8` | `fp8` | 60/60 | 0 | 0 | 1 |
| 30 | `minimax/minimax-m2.5` | WandB | `wandb/fp8` | `fp8` | 60/60 | 0 | 0 | 1 |
| 31 | `minimax/minimax-m2.5` | StreamLake | `streamlake` | `unknown` | 0/60 | 60 | 0 |  |
| 32 | `minimax/minimax-m2.5` | Venice | `venice` | `unknown` | 0/60 | 60 | 0 |  |
| 33 | `minimax/minimax-m2.5` | Minimax | `minimax/highspeed` | `fp8` | 60/60 | 0 | 0 | 1 |
| 34 | `moonshotai/kimi-k2-thinking` | Google | `google-vertex` | `unknown` | 60/60 | 0 | 17 | 0.7222 |
| 35 | `moonshotai/kimi-k2-thinking` | AtlasCloud | `atlas-cloud/int4` | `int4` | 60/60 | 0 | 0 | 1 |
| 36 | `moonshotai/kimi-k2-thinking` | Novita | `novita/bf16` | `bf16` | 0/60 | 60 | 0 |  |
| 37 | `openai/gpt-4-0314` | OpenAI | `openai` | `unknown` | 0/60 | 60 | 0 |  |
| 38 | `sao10k/l3-euryale-70b` | Novita | `novita/bf16` | `bf16` | 0/60 | 60 | 0 |  |
| 39 | `z-ai/glm-4.6v` | Z.AI | `z-ai/fp8` | `fp8` | 0/60 | 60 | 0 |  |
| 40 | `z-ai/glm-4.6v` | Novita | `novita/bf16` | `bf16` | 60/60 | 0 | 1 | 1 |

## Task: copy the 10+10 root-model variants and evals

Started: 2026-05-29T15:18Z.

Scope: copy all variant catalogs and all per-variant eval artifacts for the 20 sampled roots into one flat combined directory. The destination must not segregate the original 10-root sample and the additional 10-root sample. The source directories remain unchanged.

Destination directory: `results/model-roots-20-combined-variants-and-evals-20260529T1518Z`.

Source directories:

- First 10 variant catalog: `results/model-roots-10-combined-variants-20260529T1137Z`
- First 10 evals: `results/model-roots-10-combined-eval-20260529T114235Z`
- Second 10 variant catalog: `results/model-roots-10-more-20260529T140914Z`
- Second 10 evals: `results/model-roots-10-more-eval-20260529T142201Z`
- Existing 20-root combined variant catalog: `results/model-roots-20-combined-variants-20260529T1410Z`

Destination layout:

- `variants/`: combined 20-root variant catalog files from `results/model-roots-20-combined-variants-20260529T1410Z`
- `evals/specs/`: one flat directory of copied model-spec files, renumbered to match the combined 1-72 eval order
- `evals/variant-runs/`: one flat directory of copied per-variant eval runs, renumbered 1-72
- `evals/manifest.jsonl`: combined eval manifest with source pointers, destination run directory, model, provider, endpoint, quantization, and endpoint variant ID
- `evals/variant_summary.jsonl`: combined per-variant eval summary rows
- `summary.json`: aggregate copy manifest and eval-count summary for the destination directory

Verification result:

- Combined 20 variant rows: 72
- Total copied eval variant directories: 72
- Total copied eval result rows: 4,320
- Total copied completed responses: 2,805
- Total copied provider errors: 1,515
- Total copied schema violations: 177
- Runner or scoring failures in copied evals: 0
- No `first-10` or `second-10` directories remain in the destination tree

## Task: create the generic juror/council persona

Started: 2026-05-29T15:34Z.

Scope: create one persona file for the current eval work. The persona directory is `personas/`, and it contains a single file: `generic.md`.

Purpose: use one generic persona for a good, faithful, honest, and intelligent juror or council member. The persona should tell the model to decide from the evidence, the stated rules, and the burden of proof; avoid speculation; weigh reliable evidence; discount unsupported claims; and acknowledge uncertainty when the record does not support a single answer.

Persona path: `personas/generic.md`.

Draft:

```text
You are a careful juror and council member.

Evaluate the record faithfully. Decide only from the evidence, the stated rules, and the applicable burden of proof. Do not favor either side, fill gaps with speculation, or ignore uncertainty.

Give weight to reliable sources, direct evidence, consistent reasoning, and clear admissions. Discount unsupported claims, rhetorical force, irrelevant facts, and conclusions that exceed the record.

When the evidence supports one answer, state it plainly. When the evidence is incomplete or balanced, say so. Be concise, honest, and exact.
```

Verification:

```bash
find personas -maxdepth 1 -type f -print | sort
```

The verification returned:

```text
personas/generic.md
```

## Task: plan gene-based clustering for endpoint variants

Started: 2026-05-29T15:48Z.

Scope: document the corrected plan for gene-based clustering over the combined endpoint variants. In this plan, a gene is a prompt used to elicit behavior, as in the prior `genes.json` clustering work. A gene is not a feature extracted from scored eval results.

Input rows: build one row per endpoint variant and persona. For the current work, the variants come from the combined 72-variant catalog and the persona is `personas/generic.md`.

Execution plan:

1. Select the gene prompts from the existing gene-prompt set.
2. For each `variant + persona` row and each selected gene prompt, run the requested number of completions against the specific OpenRouter endpoint variant.
3. Preserve the exact variant request policy for every completion: `provider.only`, `allow_fallbacks: false`, `require_parameters: true`, known quantization constraint, and `X-OpenRouter-Experimental-Metadata: enabled`.
4. Verify provenance for each completion from the request metadata, inline `openrouter_metadata`, and post-hoc `/generation` metadata. A missing or mismatched provider/endpoint observation is a provenance flag for that completion.
5. Embed each gene response.
6. Run PCA separately per gene.
7. Cluster the PCA vectors separately per gene.
8. Emit cluster rows.

The process stops after emitting cluster rows. It does not proceed to selection, interpretation, or downstream council construction in this task.

Provisional cluster-row shape:

```text
variant_id, model_id, provider, endpoint, quantization, persona, gene_index, cluster
```

This row shape is adequate for the immediate 72-variant experiment. The full variant record and its raw endpoint metadata remain the durable identifier, because a variant can include more routing and endpoint state than the broken-out columns capture.

## Task: sample genes for clustering

Started: 2026-05-29T15:56Z.

Scope: pick `n = 4` random genes from `genes.json` without replacement and write the sampled prompts to `sampled-genes.json`.

Source file: `genes.json`.

Source count:

- Total genes: 14
- Distinct genes: 14

Sampling rule: use a random sample without replacement, producing four distinct gene prompts. The output file is a JSON array of prompt strings, matching the source file shape.

Output file: `sampled-genes.json`.

Selected genes:

```json
[
  "What do you think about religion?",
  "What causes climate change?",
  "Briefly discuss a social issue in a European country.",
  "Write a short metaphor about time."
]
```

Verification:

```bash
uv run - <<'PY'
import json
from pathlib import Path

all_genes = json.loads(Path("genes.json").read_text())
sampled = json.loads(Path("sampled-genes.json").read_text())

assert len(all_genes) == 14
assert len(set(all_genes)) == 14
assert len(sampled) == 4
assert len(set(sampled)) == 4
assert all(gene in all_genes for gene in sampled)

print({
    "sampled_count": len(sampled),
    "sampled_distinct": len(set(sampled)),
    "all_in_source": all(gene in all_genes for gene in sampled),
})
PY
```

The verification returned:

```text
{'sampled_count': 4, 'sampled_distinct': 4, 'all_in_source': True}
```

## Task: choose samples per gene and variant

Started: 2026-05-29T16:00Z.

Scope: choose the number of completions to collect for each sampled gene and each endpoint variant during the gene-based clustering run.

Decision: use `3` samples per `sampled gene + endpoint variant + persona` row.

Current run shape:

- Endpoint variants: 72
- Persona files: 1 (`personas/generic.md`)
- Sampled genes: 4 (`sampled-genes.json`)
- Samples per sampled gene and endpoint variant: 3
- Planned completion requests: 72 * 1 * 4 * 3 = 864

Each completion must use the specific endpoint variant request policy, not just the root OpenRouter model ID.

## Task: explore eval-based variant filtering

Started: 2026-05-29T16:02Z.

Scope: begin filtering the combined endpoint variants based on their existing adjudication eval results. The first exploratory question is how many variants had zero provider errors.

Source file: `results/model-roots-20-combined-variants-and-evals-20260529T1518Z/evals/variant_summary.jsonl`.

Result: 40 of 72 variants had zero provider errors.

Combined indexes with zero provider errors:

```text
1, 2, 3, 6, 7, 9, 12, 13, 16, 18, 21, 22, 23, 24, 26, 27, 28, 30, 32, 35, 38, 39, 41, 42, 43, 45, 46, 50, 54, 55, 57, 58, 59, 60, 61, 62, 65, 66, 67, 72
```

Verification:

```bash
uv run - <<'PY'
import json
from pathlib import Path

p = Path("results/model-roots-20-combined-variants-and-evals-20260529T1518Z/evals/variant_summary.jsonl")
rows = [json.loads(line) for line in p.read_text().splitlines() if line.strip()]
zero = [row for row in rows if int(row["provider_error_count"]) == 0]

print({
    "total_variants": len(rows),
    "zero_provider_error_variants": len(zero),
})
PY
```

The verification returned:

```text
{'total_variants': 72, 'zero_provider_error_variants': 40}
```

## Task: filter variants by eval results

Started: 2026-05-29T16:05Z.

Scope: apply the agreed eval-based filter to the combined 72 endpoint variants and write the survivors under `variants/filtered-20260529/`.

Filter criteria:

- `provider_error_count == 0`
- `deliberation_score >= 0.90`

Source files:

- Variant records: `results/model-roots-20-combined-variants-and-evals-20260529T1518Z/variants/endpoint_variants.jsonl`
- Eval summaries: `results/model-roots-20-combined-variants-and-evals-20260529T1518Z/evals/variant_summary.jsonl`
- Eval manifest: `results/model-roots-20-combined-variants-and-evals-20260529T1518Z/evals/manifest.jsonl`
- Exact variant specs: `results/model-roots-20-combined-variants-and-evals-20260529T1518Z/evals/specs/`

Output directory: `variants/filtered-20260529/`.

Output files:

- `variants/filtered-20260529/endpoint_variants.jsonl`: full survivor variant records, with `combined_index`, `filter_provider_error_count`, and `filter_deliberation_score` added.
- `variants/filtered-20260529/endpoint_variants.csv`: tabular survivor view with identity and eval-filter fields.
- `variants/filtered-20260529/variant_summary.jsonl`: survivor eval summary rows.
- `variants/filtered-20260529/manifest.jsonl`: survivor manifest rows.
- `variants/filtered-20260529/specs/*.json`: exact OpenRouter variant request specs for survivor variants.
- `variants/filtered-20260529/summary.json`: filter criteria, input paths, output list, survivor count, and survivor combined indexes.

Result: 32 of 72 variants survived.

Survivor combined indexes:

```text
1, 2, 3, 6, 7, 9, 12, 13, 16, 18, 22, 26, 27, 28, 30, 32, 38, 39, 41, 42, 43, 54, 55, 57, 58, 59, 60, 61, 62, 65, 67, 72
```

Verification:

```bash
uv run - <<'PY'
import csv
import json
from pathlib import Path

out = Path("variants/filtered-20260529")
summary = json.loads((out / "summary.json").read_text())
variant_rows = [json.loads(line) for line in (out / "endpoint_variants.jsonl").read_text().splitlines() if line.strip()]
summary_rows = [json.loads(line) for line in (out / "variant_summary.jsonl").read_text().splitlines() if line.strip()]
manifest_rows = [json.loads(line) for line in (out / "manifest.jsonl").read_text().splitlines() if line.strip()]
with (out / "endpoint_variants.csv").open(newline="", encoding="utf-8") as f:
    csv_rows = list(csv.DictReader(f))
specs = list((out / "specs").glob("*.json"))

assert summary["survivor_count"] == 32
assert len(variant_rows) == len(summary_rows) == len(manifest_rows) == len(csv_rows) == len(specs) == 32
assert all(int(row["provider_error_count"]) == 0 for row in summary_rows)
assert all(float(row["deliberation_score"]) >= 0.90 for row in summary_rows)
assert [row["combined_index"] for row in summary_rows] == summary["survivor_combined_indexes"]

print({
    "survivor_count": len(summary_rows),
    "spec_files": len(specs),
    "criteria_ok": True,
})
PY
```

The verification returned:

```text
{'survivor_count': 32, 'spec_files': 32, 'criteria_ok': True}
```

## Task: run first-gene inference and embeddings

Started: 2026-05-29T16:45:01Z. Finished: 2026-05-29T16:56:48Z.

Scope: run only the first sampled gene through the filtered endpoint variants and stop. The concurrent four-gene execution plan was cancelled, and the run processed the first gene only. This task stops after inference and embedding capture. It does not run PCA, clustering, or the remaining three sampled genes.

Inputs:

- Filtered endpoint variants: `variants/filtered-20260529/endpoint_variants.jsonl`
- Exact variant specs: `variants/filtered-20260529/specs/*.json`
- First sampled gene from `sampled-genes.json`: `What do you think about religion?`
- Persona: `personas/generic.md`
- Samples per `gene + endpoint variant + persona`: 3

Run shape:

- Filtered endpoint variants: 32
- Sampled genes processed: 1
- Personas: 1
- Samples per combination: 3
- Expected completion responses: 32 * 1 * 1 * 3 = 96
- Expected embeddings: 96

Execution rule: run sequentially for the first gene only. Do not start workers for genes 2, 3, or 4.

Variant-inference rule: each completion must use the exact OpenRouter endpoint variant, not the root model ID. The request must preserve `provider.only`, `allow_fallbacks: false`, `require_parameters: true`, known quantization constraint, and `X-OpenRouter-Experimental-Metadata: enabled`.

Output requirement: store raw response text, embedding vector, request policy, variant identity, gene identity, persona identity, sample index, timestamps, provider/endpoint provenance metadata, and any error record. Missing or mismatched OpenRouter provider/endpoint metadata is a provenance flag.

Request parameters:

- `temperature`: 0.7
- `top_p`: 1.0
- `max_tokens`: 512
- Embedding model: `text-embedding-3-small`

Output directory: `results/first-gene-inference-embeddings-20260529T164401Z-foreground-check`.

Output files:

- `manifest.json`
- `records.jsonl`
- `summary.json`

Result:

- Records written: 96
- Completion responses: 96
- Embeddings: 96
- Status counts: `ok` 96
- Completion errors: 0
- Embedding errors: 0

Verification:

```bash
wc -l results/first-gene-inference-embeddings-20260529T164401Z-foreground-check/records.jsonl
jq -r '.status' results/first-gene-inference-embeddings-20260529T164401Z-foreground-check/records.jsonl | sort | uniq -c
jq '.records_written, .embedding_count, .completion_error_count, .embedding_error_count' \
  results/first-gene-inference-embeddings-20260529T164401Z-foreground-check/summary.json
```

The verification returned 96 `records.jsonl` rows, 96 `ok` statuses, 96 embeddings, zero completion errors, and zero embedding errors.

## Task: run PCA on first-gene embeddings

Started: 2026-05-29T17:34Z. Finished: 2026-05-29T17:34Z.

Scope: reduce the first-gene embedding vectors to 3 PCA dimensions. This task uses only the completed first-gene embedding set and does not process the remaining sampled genes.

Input records: `results/first-gene-inference-embeddings-20260529T164401Z-foreground-check/records.jsonl`.

PCA parameter:

- Output dimensions: 3

Input requirements:

- Use records with status `ok`.
- Require a present embedding vector on every included record.
- Center the embedding matrix before PCA.
- Preserve record identity fields in the projected output.

Output requirement: write one projected row per input record, with the original run identity, gene, variant identity, sample index, and `pca` vector of length 3. Also write PCA components, mean vector, explained variance, explained variance ratio, singular values, and a summary file.

Execution command:

```bash
uv run python tools/run_embedding_pca.py \
  --records results/first-gene-inference-embeddings-20260529T164401Z-foreground-check/records.jsonl \
  --out results/first-gene-pca-3d-20260529T1733Z \
  --dimensions 3
```

Output directory: `results/first-gene-pca-3d-20260529T1733Z`.

Output files:

- `pca-records.jsonl`: 96 projected records, one per input response/embedding.
- `pca-fit.json`: PCA mean vector, components, explained variance, explained variance ratio, and singular values.
- `summary.json`: source counts and PCA summary.

Result:

- Source records: 96
- Included records: 96
- Unique endpoint variants: 32
- Original embedding dimensions: 1536
- PCA output dimensions: 3
- Rows with `pca` vector length 3: 96
- Explained variance ratio: `0.30085312060861064`, `0.12260190202298203`, `0.06761911139811984`
- Total explained variance ratio for the 3 dimensions: `0.4910741340297125`

Verification:

```bash
wc -l results/first-gene-pca-3d-20260529T1733Z/pca-records.jsonl
jq '{included_records, source_records, embedding_dimension, pca_dimensions, explained_variance_ratio, explained_variance_ratio_sum}' \
  results/first-gene-pca-3d-20260529T1733Z/summary.json
uv run python - <<'PY'
import json
from pathlib import Path

rows = [
    json.loads(line)
    for line in Path("results/first-gene-pca-3d-20260529T1733Z/pca-records.jsonl").read_text().splitlines()
    if line.strip()
]
print({
    "rows": len(rows),
    "all_pca_len_3": all(isinstance(row.get("pca"), list) and len(row["pca"]) == 3 for row in rows),
    "unique_combined_indexes": len({row["combined_index"] for row in rows}),
})
PY
```

The verification returned 96 projected rows, all with a 3-value `pca` vector, covering 32 unique endpoint variants.

## Task: run gene-2 inference, embeddings, and PCA

Started: 2026-05-29T17:41:57Z. Finished: 2026-05-29T17:54:29Z.

Scope: repeat the completed first-gene procedure for the second sampled gene only, then stop. This task will not process genes 3 or 4.

Gene 2 from `sampled-genes.json`: `What causes climate change?`

Inputs:

- Filtered endpoint variants: `variants/filtered-20260529/endpoint_variants.jsonl`
- Exact variant specs: `variants/filtered-20260529/specs/*.json`
- Persona: `personas/generic.md`
- Samples per `gene + endpoint variant + persona`: 3

Run shape:

- Filtered endpoint variants: 32
- Sampled genes processed: 1
- Personas: 1
- Samples per combination: 3
- Expected completion responses: 32 * 1 * 1 * 3 = 96
- Expected embeddings: 96

Execution rule: run sequentially for gene 2 only. Do not start concurrent workers.

Variant-inference rule: each completion must use the exact OpenRouter endpoint variant, not the root model ID. The request must preserve `provider.only`, `allow_fallbacks: false`, `require_parameters: true`, known quantization constraint, and `X-OpenRouter-Experimental-Metadata: enabled`.

Request parameters:

- `temperature`: 0.7
- `top_p`: 1.0
- `max_tokens`: 512
- Embedding model: `text-embedding-3-small`

PCA parameter:

- Output dimensions: 3

Planned outputs:

- Gene-2 inference and embedding directory under `results/`
- Gene-2 PCA directory under `results/`
- One inference record per response/embedding pair
- One PCA row per `ok` inference record
- Summary files for both stages

Verification plan:

- Confirm 96 inference rows.
- Confirm 96 embeddings.
- Confirm zero completion and embedding errors, or record any errors exactly.
- Confirm 96 PCA rows.
- Confirm every PCA row has a 3-value `pca` vector.
- Confirm PCA covers 32 unique endpoint variants.

Execution commands:

```bash
uv run python tools/run_first_gene_inference_embeddings.py \
  --gene-index 1 \
  --out results/gene-2-inference-embeddings-20260529T174153Z

uv run python tools/run_embedding_pca.py \
  --records results/gene-2-inference-embeddings-20260529T174153Z/records.jsonl \
  --out results/gene-2-pca-3d-20260529T1754Z \
  --dimensions 3
```

Output directories:

- Inference and embeddings: `results/gene-2-inference-embeddings-20260529T174153Z`
- PCA: `results/gene-2-pca-3d-20260529T1754Z`

Result:

- Inference records: 96
- Embeddings: 96
- Status counts: `ok` 96
- Completion errors: 0
- Embedding errors: 0
- PCA rows: 96
- PCA dimensions: 3
- Unique endpoint variants in PCA output: 32
- Original embedding dimensions: 1536
- Explained variance ratio: `0.3412173669077`, `0.10384594075852867`, `0.06889756651144217`
- Total explained variance ratio for the 3 dimensions: `0.5139608741776708`

Verification:

```bash
wc -l \
  results/gene-2-inference-embeddings-20260529T174153Z/records.jsonl \
  results/gene-2-pca-3d-20260529T1754Z/pca-records.jsonl

jq '{records_written, embedding_count, completion_error_count, embedding_error_count, status_counts, gene}' \
  results/gene-2-inference-embeddings-20260529T174153Z/summary.json

jq '{included_records, source_records, embedding_dimension, pca_dimensions, explained_variance_ratio, explained_variance_ratio_sum}' \
  results/gene-2-pca-3d-20260529T1754Z/summary.json

uv run python - <<'PY'
import json
from pathlib import Path

rows = [
    json.loads(line)
    for line in Path("results/gene-2-pca-3d-20260529T1754Z/pca-records.jsonl").read_text().splitlines()
    if line.strip()
]
print({
    "rows": len(rows),
    "all_pca_len_3": all(isinstance(row.get("pca"), list) and len(row["pca"]) == 3 for row in rows),
    "unique_combined_indexes": len({row["combined_index"] for row in rows}),
    "gene_values": sorted({row["gene"] for row in rows}),
})
PY
```

The verification returned 96 inference rows, 96 PCA rows, 96 `ok` statuses, 96 embeddings, zero completion errors, zero embedding errors, 32 unique endpoint variants, and only the gene value `What causes climate change?`.

## Task: run gene-3 inference, embeddings, and PCA

Started: 2026-05-29T17:57:02Z. Finished: 2026-05-29T18:07:49Z.

Scope: repeat the completed gene-2 procedure for the third sampled gene only, then stop. This task will not process gene 4.

Gene 3 from `sampled-genes.json`: `Briefly discuss a social issue in a European country.`

Inputs:

- Filtered endpoint variants: `variants/filtered-20260529/endpoint_variants.jsonl`
- Exact variant specs: `variants/filtered-20260529/specs/*.json`
- Persona: `personas/generic.md`
- Samples per `gene + endpoint variant + persona`: 3

Run shape:

- Filtered endpoint variants: 32
- Sampled genes processed: 1
- Personas: 1
- Samples per combination: 3
- Expected completion responses: 32 * 1 * 1 * 3 = 96
- Expected embeddings: 96

Execution rule: run sequentially for gene 3 only. Do not start concurrent workers.

Variant-inference rule: each completion must use the exact OpenRouter endpoint variant, not the root model ID. The request must preserve `provider.only`, `allow_fallbacks: false`, `require_parameters: true`, known quantization constraint, and `X-OpenRouter-Experimental-Metadata: enabled`.

Request parameters:

- `temperature`: 0.7
- `top_p`: 1.0
- `max_tokens`: 512
- Embedding model: `text-embedding-3-small`

PCA parameter:

- Output dimensions: 3

Planned outputs:

- Gene-3 inference and embedding directory under `results/`
- Gene-3 PCA directory under `results/`
- One inference record per response/embedding pair
- One PCA row per `ok` inference record
- Summary files for both stages

Verification plan:

- Confirm 96 inference rows.
- Confirm 96 embeddings.
- Confirm zero completion and embedding errors, or record any errors exactly.
- Confirm 96 PCA rows.
- Confirm every PCA row has a 3-value `pca` vector.
- Confirm PCA covers 32 unique endpoint variants.

Execution commands:

```bash
uv run python tools/run_first_gene_inference_embeddings.py \
  --gene-index 2 \
  --out results/gene-3-inference-embeddings-20260529T175658Z

uv run python tools/run_embedding_pca.py \
  --records results/gene-3-inference-embeddings-20260529T175658Z/records.jsonl \
  --out results/gene-3-pca-3d-20260529T1807Z \
  --dimensions 3
```

Output directories:

- Inference and embeddings: `results/gene-3-inference-embeddings-20260529T175658Z`
- PCA: `results/gene-3-pca-3d-20260529T1807Z`

Result:

- Inference records: 96
- Embeddings: 96
- Status counts: `ok` 96
- Completion errors: 0
- Embedding errors: 0
- PCA rows: 96
- PCA dimensions: 3
- Unique endpoint variants in PCA output: 32
- Original embedding dimensions: 1536
- Explained variance ratio: `0.24420601697302108`, `0.09079505495633328`, `0.08490374741861263`
- Total explained variance ratio for the 3 dimensions: `0.419904819347967`

Verification:

```bash
wc -l \
  results/gene-3-inference-embeddings-20260529T175658Z/records.jsonl \
  results/gene-3-pca-3d-20260529T1807Z/pca-records.jsonl

jq '{records_written, embedding_count, completion_error_count, embedding_error_count, status_counts, gene}' \
  results/gene-3-inference-embeddings-20260529T175658Z/summary.json

jq '{included_records, source_records, embedding_dimension, pca_dimensions, explained_variance_ratio, explained_variance_ratio_sum}' \
  results/gene-3-pca-3d-20260529T1807Z/summary.json

uv run python - <<'PY'
import json
from pathlib import Path

rows = [
    json.loads(line)
    for line in Path("results/gene-3-pca-3d-20260529T1807Z/pca-records.jsonl").read_text().splitlines()
    if line.strip()
]
print({
    "rows": len(rows),
    "all_pca_len_3": all(isinstance(row.get("pca"), list) and len(row["pca"]) == 3 for row in rows),
    "unique_combined_indexes": len({row["combined_index"] for row in rows}),
    "gene_values": sorted({row["gene"] for row in rows}),
})
PY
```

The verification returned 96 inference rows, 96 PCA rows, 96 `ok` statuses, 96 embeddings, zero completion errors, zero embedding errors, 32 unique endpoint variants, and only the gene value `Briefly discuss a social issue in a European country.`

## Task: run gene-4 inference, embeddings, and PCA

Status: completed.

Scope: repeat the completed gene-3 procedure for the fourth sampled gene only, then stop.

Gene 4 from `sampled-genes.json`: `Write a short metaphor about time.`

Inputs:

- Filtered endpoint variants: `variants/filtered-20260529/endpoint_variants.jsonl`
- Exact variant specs: `variants/filtered-20260529/specs/*.json`
- Persona: `personas/generic.md`
- Samples per `gene + endpoint variant + persona`: 3

Run shape:

- Filtered endpoint variants: 32
- Sampled genes processed: 1
- Personas: 1
- Samples per combination: 3
- Expected completion responses: 32 * 1 * 1 * 3 = 96
- Expected embeddings: 96

Execution rule: run sequentially for gene 4 only. Do not start concurrent workers.

Variant-inference rule: each completion must use the exact OpenRouter endpoint variant, not the root model ID. The request must preserve `provider.only`, `allow_fallbacks: false`, `require_parameters: true`, known quantization constraint, and `X-OpenRouter-Experimental-Metadata: enabled`.

Request parameters:

- `temperature`: 0.7
- `top_p`: 1.0
- `max_tokens`: 512
- Embedding model: `text-embedding-3-small`

PCA parameter:

- Output dimensions: 3

Planned outputs:

- Gene-4 inference and embedding directory under `results/`
- Gene-4 PCA directory under `results/`
- One inference record per response/embedding pair
- One PCA row per `ok` inference record
- Summary files for both stages

Execution:

```bash
uv run python tools/run_first_gene_inference_embeddings.py \
  --gene-index 3 \
  --out results/gene-4-inference-embeddings-20260529T181038Z
```

```bash
uv run python tools/run_embedding_pca.py \
  --records results/gene-4-inference-embeddings-20260529T181038Z/records.jsonl \
  --out results/gene-4-pca-3d-20260529T1816Z \
  --dimensions 3
```

Results:

- Inference and embedding output: `results/gene-4-inference-embeddings-20260529T181038Z`
- PCA output: `results/gene-4-pca-3d-20260529T1816Z`
- Started: 2026-05-29T18:10:44Z
- Finished: 2026-05-29T18:16:20Z
- Records written: 96
- Embeddings written: 96
- Completion errors: 0
- Embedding errors: 0
- PCA rows: 96
- PCA dimensions: 3
- Original embedding dimensions: 1536
- Total explained variance ratio: `0.37638343731465035`

Verification plan:

- Confirm 96 inference rows.
- Confirm 96 embeddings.
- Confirm zero completion and embedding errors, or record any errors exactly.
- Confirm 96 PCA rows.
- Confirm every PCA row has a 3-value `pca` vector.
- Confirm PCA covers 32 unique endpoint variants.

Verification:

```bash
wc -l \
  results/gene-4-inference-embeddings-20260529T181038Z/records.jsonl \
  results/gene-4-pca-3d-20260529T1816Z/pca-records.jsonl
```

```bash
jq '{records_written, embedding_count, completion_error_count, embedding_error_count, status_counts, gene, gene_index}' \
  results/gene-4-inference-embeddings-20260529T181038Z/summary.json
```

```bash
jq '{included_records, source_records, embedding_dimension, pca_dimensions, explained_variance_ratio, explained_variance_ratio_sum}' \
  results/gene-4-pca-3d-20260529T1816Z/summary.json
```

```bash
uv run python - <<'PY'
import json
from pathlib import Path

rows = [
    json.loads(line)
    for line in Path("results/gene-4-pca-3d-20260529T1816Z/pca-records.jsonl").read_text().splitlines()
    if line.strip()
]
print({
    "rows": len(rows),
    "all_pca_len_3": all(isinstance(row.get("pca"), list) and len(row["pca"]) == 3 for row in rows),
    "unique_combined_indexes": len({row["combined_index"] for row in rows}),
    "gene_values": sorted({row["gene"] for row in rows}),
})
PY
```

The verification returned 96 inference rows, 96 PCA rows, 96 `ok` statuses, 96 embeddings, zero completion errors, zero embedding errors, 32 unique endpoint variants, and only the gene value `Write a short metaphor about time.`

## Task: cluster completed per-gene PCA outputs

Status: completed.

Scope: assign cluster labels to the completed per-gene PCA outputs. This task does not call models, create embeddings, or recompute PCA.

Clustering boundary: one gene. PCA was fit independently for each gene, so coordinates from different genes are not in a shared coordinate system. Run clustering separately for each gene and treat cluster labels as local to `gene_index`.

Inputs:

- Gene 1 PCA rows: `results/first-gene-pca-3d-20260529T1733Z/pca-records.jsonl`
- Gene 2 PCA rows: `results/gene-2-pca-3d-20260529T1754Z/pca-records.jsonl`
- Gene 3 PCA rows: `results/gene-3-pca-3d-20260529T1807Z/pca-records.jsonl`
- Gene 4 PCA rows: `results/gene-4-pca-3d-20260529T1816Z/pca-records.jsonl`

Per-gene method:

1. Read one PCA JSONL file.
2. Validate exactly 96 rows.
3. Validate every row has a 3-number `pca` vector.
4. Validate 32 unique endpoint variants and 3 rows per endpoint variant.
5. Build a 96 by 3 matrix from that gene's `pca` vectors.
6. Run K-means for `k = 3..10`, with `random_state = 0` and `n_init = 10`.
7. Score each valid `k` with silhouette score.
8. Choose the `k` with the highest silhouette score.
9. If silhouette scoring is impossible because the data are degenerate, assign cluster `0` for that gene and record the reason.
10. Write one output row per sampled completion.

Planned output directory: `results/gene-clusters-20260529T1828Z`.

Planned output files:

- `clusters.jsonl`: one cluster row per sampled completion.
- `clusters.csv`: compact inspection table.
- `summary.json`: counts, chosen `k`, silhouette scores, cluster sizes, and validation results per gene.
- `cluster-fit.json`: per-gene K-means centers and candidate scores.

Cluster row fields:

```text
run_id, gene_index, gene, gene_sha256, persona_id, persona_path, variant_order, combined_index, endpoint_variant_id, openrouter_model_id, provider_name, endpoint_tag, quantization, sample_index, source_pca_path, pca, cluster
```

Verification plan:

- Confirm 384 total cluster rows.
- Confirm 96 cluster rows per gene.
- Confirm 32 unique endpoint variants per gene.
- Confirm 3 samples per endpoint variant per gene.
- Confirm every row has exactly one cluster label.
- Confirm no missing PCA vectors.
- Confirm every cluster label is local to its own `gene_index`.

Execution command:

```bash
uv run --script tools/run_gene_pca_clustering.py \
  --pca-records results/first-gene-pca-3d-20260529T1733Z/pca-records.jsonl \
  --pca-records results/gene-2-pca-3d-20260529T1754Z/pca-records.jsonl \
  --pca-records results/gene-3-pca-3d-20260529T1807Z/pca-records.jsonl \
  --pca-records results/gene-4-pca-3d-20260529T1816Z/pca-records.jsonl \
  --out results/gene-clusters-20260529T1828Z
```

Output:

- Output directory: `results/gene-clusters-20260529T1828Z`
- Cluster JSONL: `results/gene-clusters-20260529T1828Z/clusters.jsonl`
- Cluster CSV: `results/gene-clusters-20260529T1828Z/clusters.csv`
- Summary: `results/gene-clusters-20260529T1828Z/summary.json`
- Fit details: `results/gene-clusters-20260529T1828Z/cluster-fit.json`
- Rows written: 384

Chosen clusters:

- Gene 0, `What do you think about religion?`: `k = 3`, silhouette `0.6232454403943477`, cluster sizes `44, 49, 3`.
- Gene 1, `What causes climate change?`: `k = 4`, silhouette `0.4534466243285013`, cluster sizes `5, 25, 33, 33`.
- Gene 2, `Briefly discuss a social issue in a European country.`: `k = 10`, silhouette `0.5303757155843943`, cluster sizes `7, 26, 5, 4, 12, 7, 13, 8, 6, 8`.
- Gene 3, `Write a short metaphor about time.`: `k = 4`, silhouette `0.6521519328630767`, cluster sizes `57, 4, 10, 25`.

Verification:

```bash
wc -l \
  results/gene-clusters-20260529T1828Z/clusters.jsonl \
  results/gene-clusters-20260529T1828Z/clusters.csv
```

```bash
jq '{rows_written, gene_count, genes: [.genes[] | {gene_index, status, chosen_k, chosen_silhouette_score, cluster_counts, row_count, unique_endpoint_variants, samples_per_endpoint_variant}]}' \
  results/gene-clusters-20260529T1828Z/summary.json
```

```bash
uv run python - <<'PY'
import json
from collections import Counter, defaultdict
from pathlib import Path

rows = [
    json.loads(line)
    for line in Path("results/gene-clusters-20260529T1828Z/clusters.jsonl").read_text().splitlines()
    if line.strip()
]
by_gene = defaultdict(list)
for row in rows:
    by_gene[row["gene_index"]].append(row)

checks = {
    "total_rows": len(rows),
    "gene_count": len(by_gene),
    "per_gene_rows": {str(k): len(v) for k, v in sorted(by_gene.items())},
    "per_gene_unique_variants": {str(k): len({r["endpoint_variant_id"] for r in v}) for k, v in sorted(by_gene.items())},
    "per_gene_sample_counts": {},
    "all_pca_len_3": all(isinstance(r.get("pca"), list) and len(r["pca"]) == 3 for r in rows),
    "all_have_cluster": all(isinstance(r.get("cluster"), int) for r in rows),
}
for gene, gene_rows in sorted(by_gene.items()):
    counts = Counter(r["endpoint_variant_id"] for r in gene_rows)
    checks["per_gene_sample_counts"][str(gene)] = sorted(set(counts.values()))
print(json.dumps(checks, sort_keys=True))
PY
```

The verification returned 384 cluster rows, 96 rows per gene, 32 unique endpoint variants per gene, 3 samples per endpoint variant per gene, no missing 3-value PCA vectors, and one integer cluster label on every row.

## Task: aggregate sample-level clusters into variant-persona cluster vectors

Status: completed.

Scope: create one row per endpoint variant and persona, with a `clusters` array containing one integer cluster label per sampled gene. This task consumes the completed sample-level cluster output and does not rerun clustering.

Input files:

- Sample-level clusters: `results/gene-clusters-20260529T1828Z/clusters.jsonl`
- Per-gene cluster fit details: `results/gene-clusters-20260529T1828Z/cluster-fit.json`
- Full variant records: `variants/filtered-20260529/endpoint_variants.jsonl`

Output directory: `results/variant-persona-clusters-20260529T1837Z`.

Output files:

- `variant-persona-clusters.jsonl`: one JSON object per endpoint variant/persona pair.
- `variant-persona-clusters.json`: JSON array version of the same rows.
- `summary.json`: row counts, gene order, and aggregation-method counts.

Row shape:

```text
endpoint_variant_id, combined_index, openrouter_model_id, provider_name, endpoint_tag, quantization, persona, clusters, cluster_details, variant
```

The `variant` field is the full endpoint-variant record from `variants/filtered-20260529/endpoint_variants.jsonl`. The `persona` field contains the persona id and persona path. The `clusters` array is ordered by ascending `gene_index`, so index 0 is gene 0, index 1 is gene 1, index 2 is gene 2, and index 3 is gene 3.

Aggregation rule:

- For each endpoint variant, persona, and gene, read the three sample-level cluster labels.
- If all three samples have the same cluster, use that cluster.
- If two samples share a cluster and one differs, use the majority cluster.
- If all three samples differ, choose the cluster for the sample whose PCA vector is nearest to that sample's assigned K-means center for that gene. Break any exact distance tie by lower `sample_index`, then lower cluster id.
- Preserve the three sample labels and aggregation method in `cluster_details`.

Planned verification:

- Confirm 32 output rows, because there are 32 endpoint variants and one persona.
- Confirm every row has a `clusters` array of length 4.
- Confirm every `clusters` value is an integer.
- Confirm every row contains the full variant record and persona object.
- Confirm all 384 sample-level cluster rows are represented through the 32 output rows and four genes.

Execution command:

```bash
uv run --script tools/aggregate_variant_persona_clusters.py \
  --clusters results/gene-clusters-20260529T1828Z/clusters.jsonl \
  --cluster-fit results/gene-clusters-20260529T1828Z/cluster-fit.json \
  --variants variants/filtered-20260529/endpoint_variants.jsonl \
  --out results/variant-persona-clusters-20260529T1837Z
```

Output:

- Output directory: `results/variant-persona-clusters-20260529T1837Z`
- JSONL: `results/variant-persona-clusters-20260529T1837Z/variant-persona-clusters.jsonl`
- JSON array: `results/variant-persona-clusters-20260529T1837Z/variant-persona-clusters.json`
- Summary: `results/variant-persona-clusters-20260529T1837Z/summary.json`
- Rows: 32
- Clusters per row: 4
- Input sample-level cluster rows represented: 384
- Aggregation methods: 80 unanimous, 42 majority, 6 center-distance tie breaks

Verification:

```bash
wc -l results/variant-persona-clusters-20260529T1837Z/variant-persona-clusters.jsonl
```

```bash
jq '{variant_persona_rows, clusters_per_row, input_cluster_rows, aggregation_method_counts}' \
  results/variant-persona-clusters-20260529T1837Z/summary.json
```

```bash
uv run python - <<'PY'
import json
from pathlib import Path

rows = [
    json.loads(line)
    for line in Path("results/variant-persona-clusters-20260529T1837Z/variant-persona-clusters.jsonl").read_text().splitlines()
    if line.strip()
]
print(json.dumps({
    "rows": len(rows),
    "all_clusters_len_4": all(isinstance(r.get("clusters"), list) and len(r["clusters"]) == 4 for r in rows),
    "all_clusters_int": all(all(isinstance(v, int) for v in r["clusters"]) for r in rows),
    "all_have_variant": all(isinstance(r.get("variant"), dict) and r["variant"].get("endpoint_variant_id") == r.get("endpoint_variant_id") for r in rows),
    "all_have_persona": all(isinstance(r.get("persona"), dict) and r["persona"].get("id") for r in rows),
    "cluster_detail_count": sum(len(r.get("cluster_details", [])) for r in rows),
    "unique_variant_personas": len({(r["endpoint_variant_id"], r["persona"]["id"]) for r in rows}),
}, sort_keys=True))
PY
```

The verification returned 32 variant/persona rows, 32 unique variant/persona keys, a 4-integer `clusters` array on every row, full variant records on every row, persona objects on every row, and 128 gene-level cluster-detail records.

## Task: write `tools/sample-pool.py` for clustered variant-persona sampling

Status: completed.

Scope: create a uv-runnable Python program named `tools/sample-pool.py` that reads a `variant-persona-clusters.jsonl`-style file and writes a sampled JSONL pool. It does not change the input file and does not convert rows into the `arb` council-pool format.

Input file:

- JSONL rows with a `clusters` property.
- `clusters` must be a non-empty array of integers.
- All rows in one input file must have the same `clusters` length.

Command-line interface:

```bash
uv run --script tools/sample-pool.py INPUT.jsonl --out OUTPUT.jsonl [--pool-size N] [--seed N]
```

Arguments:

- `INPUT.jsonl`: source variant/persona cluster file.
- `--out OUTPUT.jsonl`: sampled JSONL output file.
- `--pool-size N`: number of rows to emit; default `20`.
- `--seed N`: optional deterministic random seed.

Sampling rule for each emitted row:

1. Start with all input rows as survivors.
2. Pick a random gene index from the unvisited gene indexes.
3. Pick a random cluster label from the cluster labels present for that gene among the current survivors.
4. Filter survivors to rows whose `clusters[gene_index]` equals that chosen cluster label.
5. Record a log segment in the form `CLUSTER: COUNT`, where `COUNT` is the survivor count after that filter.
6. Repeat until every gene index has been visited once.
7. Pick one random row from the final survivors and write it to the output.
8. Sampling is with replacement, so the same input row may appear more than once in the output.

Log output: print one log line to stdout per emitted row. The line is a comma-separated list of `CLUSTER: COUNT` segments, in the random gene order used for that emitted row. The log line does not include the chosen output row; the output row is written only to `--out`.

Planned verification:

- Run the script on `results/variant-persona-clusters-20260529T1837Z/variant-persona-clusters.jsonl`.
- Confirm the output file has 20 JSONL rows by default.
- Confirm every output row is one of the input rows.
- Confirm every output row has a 4-integer `clusters` array.
- Confirm stdout has exactly one log line per emitted row.

Implementation: `tools/sample-pool.py`.

Verification command:

```bash
mkdir -p results/sample-pool-20260529T1850Z
uv run --script tools/sample-pool.py \
  results/variant-persona-clusters-20260529T1837Z/variant-persona-clusters.jsonl \
  --out results/sample-pool-20260529T1850Z/pool.jsonl \
  --seed 0 \
  | tee results/sample-pool-20260529T1850Z/sample.log
```

Verification output:

- Sampled pool: `results/sample-pool-20260529T1850Z/pool.jsonl`
- Trace log: `results/sample-pool-20260529T1850Z/sample.log`
- Output rows: 20
- Trace log lines: 20
- Every output row came from the input JSONL.
- Every output row has a 4-integer `clusters` array.
- Unique output rows: 9, confirming replacement sampling occurred in this deterministic run.

Validation commands:

```bash
wc -l \
  results/sample-pool-20260529T1850Z/pool.jsonl \
  results/sample-pool-20260529T1850Z/sample.log
```

```bash
uv run python - <<'PY'
import json
from pathlib import Path

source = [
    line
    for line in Path("results/variant-persona-clusters-20260529T1837Z/variant-persona-clusters.jsonl").read_text().splitlines()
    if line.strip()
]
out = [
    line
    for line in Path("results/sample-pool-20260529T1850Z/pool.jsonl").read_text().splitlines()
    if line.strip()
]
rows = [json.loads(line) for line in out]
print(json.dumps({
    "output_rows": len(rows),
    "log_lines": len([line for line in Path("results/sample-pool-20260529T1850Z/sample.log").read_text().splitlines() if line.strip()]),
    "all_rows_from_input": all(line in set(source) for line in out),
    "all_clusters_len_4": all(isinstance(row.get("clusters"), list) and len(row["clusters"]) == 4 for row in rows),
    "all_clusters_int": all(all(isinstance(value, int) for value in row["clusters"]) for row in rows),
    "unique_output_rows": len(set(out)),
}, sort_keys=True))
PY
```

Additional checks passed:

```bash
uv run --script tools/sample-pool.py --help
uv run python -m py_compile tools/sample-pool.py
git diff --check -- tools/sample-pool.py docs/history/sampling.md docs/history/eval-analysis.md
```

Emitted rows in the deterministic verification run:

```text
1. deepseek/deepseek-v4-flash | Morph | morph | unknown
2. google/gemini-2.5-flash-lite | Google | google-vertex | unknown
3. z-ai/glm-4.6v | Novita | novita/bf16 | bf16
4. deepseek/deepseek-v4-flash | Morph | morph | unknown
5. arcee-ai/trinity-mini | Clarifai | clarifai/bf16 | bf16
6. minimax/minimax-m2.7 | Minimax | minimax/fp8 | fp8
7. z-ai/glm-4.6v | Novita | novita/bf16 | bf16
8. google/gemma-4-26b-a4b-it | Google | google-vertex/global | unknown
9. z-ai/glm-4.6v | Novita | novita/bf16 | bf16
10. minimax/minimax-m2.7 | Mara | mara | unknown
11. deepseek/deepseek-v4-flash | Morph | morph | unknown
12. z-ai/glm-4.6v | Novita | novita/bf16 | bf16
13. google/gemini-2.5-flash-lite | Google | google-vertex | unknown
14. minimax/minimax-m2.5 | Friendli | friendli | unknown
15. z-ai/glm-4.6v | Novita | novita/bf16 | bf16
16. z-ai/glm-4.6v | Novita | novita/bf16 | bf16
17. z-ai/glm-4.6v | Novita | novita/bf16 | bf16
18. z-ai/glm-4.6v | Novita | novita/bf16 | bf16
19. arcee-ai/trinity-mini | Clarifai | clarifai/bf16 | bf16
20. minimax/minimax-m2.7 | Minimax | minimax/highspeed | fp8
```

## Task: write `tools/sample-diverse-pool.py` for coverage-based cluster sampling

Status: completed.

Scope: create a uv-runnable Python program named `tools/sample-diverse-pool.py` that samples a variant/persona cluster JSONL file by maximizing cluster coverage. This sampler keeps the random-walk sampler unchanged.

Command-line interface:

```bash
uv run --script tools/sample-diverse-pool.py INPUT.jsonl \
  --out OUTPUT.jsonl \
  [--diagnostics-out DIAGNOSTICS.jsonl] \
  [--pool-size N] \
  [--seed N]
```

Default scoring uses single-gene cluster coverage and penalties for repeated model ids, providers, endpoint variants, and exact duplicate rows. Pairwise gene-cluster coverage is available through `--pair-weight`, but the default is `0.0` so the first implementation optimizes the simpler coverage target.

For each step, the sampler scores candidates with:

```text
score =
  sum(1 / (1 + prior_count[gene_cluster])) +
  pair_weight * sum(1 / (1 + prior_count[gene_cluster_pair])) -
  model_penalty * prior_model_count -
  provider_penalty * prior_provider_count -
  endpoint_penalty * prior_endpoint_count -
  duplicate_penalty * prior_exact_row_count
```

The sampler avoids exact duplicate input rows while unused rows remain. If `pool-size` exceeds the number of input rows, it allows replacement after all rows have been selected once, with the duplicate penalty applied.

Diagnostic logging:

- Stdout prints one line per emitted row with source row, total score, coverage components, penalty, newly covered features, remaining uncovered single-gene features, model id, provider, endpoint, quantization, and cluster vector.
- Stdout also prints a final summary line.
- `--diagnostics-out` writes one JSON object per emitted row with the same diagnostic fields plus candidate count and source row.

Test command:

```bash
mkdir -p results/sample-diverse-pool-20260529T1909Z
uv run --script tools/sample-diverse-pool.py \
  results/variant-persona-clusters-20260529T1837Z/variant-persona-clusters.jsonl \
  --out results/sample-diverse-pool-20260529T1909Z/pool.jsonl \
  --diagnostics-out results/sample-diverse-pool-20260529T1909Z/diagnostics.jsonl \
  --seed 0 \
  | tee results/sample-diverse-pool-20260529T1909Z/sample.log
```

Test output:

- Sampled pool: `results/sample-diverse-pool-20260529T1909Z/pool.jsonl`
- Diagnostics: `results/sample-diverse-pool-20260529T1909Z/diagnostics.jsonl`
- Log: `results/sample-diverse-pool-20260529T1909Z/sample.log`
- Output rows: 20
- Diagnostic rows: 20
- Log lines: 21, consisting of 20 emitted-row lines plus one summary line
- Unique output rows: 20
- All output rows came from the input JSONL.
- Every output row has a 4-integer `clusters` array.
- Covered single-gene cluster features: 21 of 21
- Unique model ids: 13
- Unique providers: 15
- Unique endpoint variants: 20

Validation commands:

```bash
uv run --script tools/sample-diverse-pool.py --help
uv run python -m py_compile tools/sample-diverse-pool.py
git diff --check -- tools/sample-diverse-pool.py
```

## Task: write `tools/sample-tuple-pool.py` for tuple-uniform sampling

Status: completed.

Scope: create a uv-runnable Python program named `tools/sample-tuple-pool.py` that samples by exact cluster assignment tuple. This sampler treats each distinct `clusters` vector as a sampling unit.

Command-line interface:

```bash
uv run --script tools/sample-tuple-pool.py INPUT.jsonl \
  --out OUTPUT.jsonl \
  [--diagnostics-out DIAGNOSTICS.jsonl] \
  [--pool-size N] \
  [--seed N]
```

Sampling rule:

1. Read all input rows.
2. Validate that every row has a same-length integer `clusters` array.
3. Group rows by exact `clusters` tuple.
4. For each emitted row, choose one unique tuple uniformly at random.
5. Choose one row uniformly at random from the rows with that tuple.
6. Write the selected row to the output JSONL.
7. Repeat until `pool-size` rows have been written.

The tuple choice is with replacement, matching the stated design.

Diagnostic logging:

- Stdout prints one line per emitted row with the selected tuple, tuple size, cumulative tuple count, source row, cumulative source-row count, model id, provider, endpoint, and quantization.
- Stdout also prints a final summary line.
- `--diagnostics-out` writes one JSON object per emitted row.

Test command:

```bash
mkdir -p results/sample-tuple-pool-20260529T1913Z
uv run --script tools/sample-tuple-pool.py \
  results/variant-persona-clusters-20260529T1837Z/variant-persona-clusters.jsonl \
  --out results/sample-tuple-pool-20260529T1913Z/pool.jsonl \
  --diagnostics-out results/sample-tuple-pool-20260529T1913Z/diagnostics.jsonl \
  --seed 0 \
  | tee results/sample-tuple-pool-20260529T1913Z/sample.log
```

Test output:

- Sampled pool: `results/sample-tuple-pool-20260529T1913Z/pool.jsonl`
- Diagnostics: `results/sample-tuple-pool-20260529T1913Z/diagnostics.jsonl`
- Log: `results/sample-tuple-pool-20260529T1913Z/sample.log`
- Input rows: 32
- Input unique tuples: 19
- Output rows: 20
- Diagnostic rows: 20
- Log lines: 21, consisting of 20 emitted-row lines plus one summary line
- Output unique tuples: 12
- Output unique rows: 14
- Unique model ids: 8
- Unique providers: 11
- Unique endpoint variants: 14
- Every output row came from the input JSONL.
- Every output row has a 4-integer `clusters` array.
- Every output tuple was present in the input.

Validation commands:

```bash
uv run --script tools/sample-tuple-pool.py --help
uv run python -m py_compile tools/sample-tuple-pool.py
git diff --check -- tools/sample-tuple-pool.py
```
