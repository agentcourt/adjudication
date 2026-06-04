# Jury Pool Generation

This document describes the older CSV pipeline that produced selected council files from OpenRouter model metadata, live tool-probe results, checked-in persona texts, clustering output, and PCA coordinates. Runtime council pools now use JSONL request-spec records, because provider endpoint and quantization constraints must remain attached to each sampled council member.

The runtime no longer uses the CSV files described here as its default council pool. A current pool record must carry the OpenRouter model id, provider endpoint tag, quantization when known, and persona path in JSONL form.

Run the commands below from the repository root unless the command says otherwise. Use `uv run --script` for Python scripts with PEP 723 metadata.

## Files

| File | Role |
|---|---|
| `common/data/personas/openrouter-models.json` | Raw OpenRouter model metadata used by the metadata filter and `select-council.py` |
| `common/tools/filter-models.py` | Conservative metadata prefilter for OpenRouter models |
| `common/data/personas/models-prefiltered.csv` | Intermediate model list after metadata prefilter |
| `common/data/personas/model-filter-decisions.csv` | Audit file explaining prefilter decisions |
| `common/tools/model-speed.sh` | Live probe for latency and `submit_juror_vote` tool support |
| `common/data/personas/model-latency.csv` | `MODEL,ELAPSED_MS,TOOLS_SUPPORTED` records from the live probe |
| `common/data/personas/models.csv` | Model ids retained after latency and tool-support filtering |
| `common/data/personas/cluster-input.csv` | Deterministic sampled model/persona input for clustering when using `generate-council.py` |
| `common/etc/personas/persons/` | Checked-in persona text files |
| `common/etc/personas.csv` | Cross product of retained models and persona files |
| `common/data/personas/genes.json` | Gene prompts used for clustering |
| `common/tools/cluster-personas.py` | Samples completions, embeds them, clusters per gene, and writes PCA rows |
| `common/data/personas/clusters.csv` | `MODEL,PERSONA_FILE,GENE,CLUSTER` rows from `cluster-personas.py` |
| `common/data/personas/pca-cluster.csv` | `MODEL,PERSONA_FILE,GENE,PC1,PC2,PC3,CLUSTER` rows from `cluster-personas.py` |
| `common/data/personas/model-operational-failures.csv` | Manual exclusion ledger for known model failures |
| `common/tools/select-council.py` | Selects a behaviorally diverse council from cluster/PCA data |
| `common/data/personas/council.csv` | Historical selected council rows, written as `MODEL,personas/persons/....txt` |
| `common/data/personas/council-report.md` | Default selection report from `generate-council.py` |
| `common/data/personas/pool.csv` | Historical CSV pool file; not a current runtime council pool |

Rows in `common/etc/personas.csv`, `common/data/personas/council.csv`, and `common/data/personas/pool.csv` have two columns:

```text
MODEL,PERSONA_PATH
```

`PERSONA_PATH` must be relative to `common/etc`, for example:

```text
openrouter://openai/gpt-4.1-mini,personas/persons/d715074-6.txt
```

Do not check in absolute local home-directory paths in council or pool files.

## End-To-End Driver

`common/tools/generate-council.py` runs the full pipeline described below:

```bash
uv run --script common/tools/generate-council.py
```

A full run fetches OpenRouter metadata, filters models, probes tool support and latency, rebuilds `models.csv`, rebuilds `common/etc/personas.csv`, samples a clustering input, runs `cluster-personas.py`, and runs `select-council.py`.

The full run requires:

- `OPENROUTER_API_KEY` for OpenRouter model probing and clustering.
- `OPENAI_API_KEY` for embeddings during clustering.
- enough time for live model probing and clustering.

For a selection-only verification using existing metadata, latency, clusters, and PCA rows, run:

```bash
uv run --script common/tools/generate-council.py \
  --use-existing-metadata \
  --use-existing-latency \
  --use-existing-clusters
```

The driver defaults to a deterministic 512-row clustering input sample (`--sample-size 512 --sample-seed 0`) and writes `common/data/personas/council.csv`. Its default selection report is `common/data/personas/council-report.md`, beside the other council-generation evidence. It does not modify `pool.csv`.

By default, the driver reuses intermediate files younger than seven days. Control that cache window with `--expires DAYS`:

```bash
uv run --script common/tools/generate-council.py --expires 7
```

Use `--expires 0` to regenerate all intermediates. The cache applies to intermediate outputs such as metadata, metadata-filter outputs, `model-latency.csv`, `models.csv`, `common/etc/personas.csv`, `cluster-input.csv`, `clusters.csv`, and `pca-cluster.csv`. The final `council.csv` and `council-report.md` are regenerated from the selected inputs. For paired outputs, such as `clusters.csv` and `pca-cluster.csv`, the driver refuses to mix a fresh file with a stale or missing paired file.

## Stage 1: Fetch OpenRouter Metadata

Fetch the current OpenRouter model metadata and save it as `openrouter-models.json`:

```bash
curl -fsSL https://openrouter.ai/api/v1/models \
  > common/data/personas/openrouter-models.json
```

`select-council.py` accepts either the raw OpenRouter object with a `data` array or a raw list of model records.

## Stage 2: Metadata Prefilter

The metadata prefilter removes models only when OpenRouter metadata proves the model cannot satisfy the juror path. It excludes models without text input, without text output, or without advertised function-tool support. Unknowns remain eligible for the live probe.

```bash
uv run --script common/tools/filter-models.py \
  --metadata common/data/personas/openrouter-models.json \
  --out common/data/personas/models-prefiltered.csv \
  --decisions common/data/personas/model-filter-decisions.csv
```

`models-prefiltered.csv` is an intermediate input to the live probe. `model-filter-decisions.csv` is an audit file.

## Stage 3: Live Tool And Latency Probe

Run the live probe against the prefiltered model list:

```bash
common/tools/model-speed.sh common/etc/personas/persons/d715074-0.txt \
  < common/data/personas/models-prefiltered.csv \
  > common/data/personas/model-latency.csv
```

The output format is:

```text
MODEL,ELAPSED_MS,TOOLS_SUPPORTED
```

`TOOLS_SUPPORTED=true` means the model successfully called the `submit_council_vote` tool in a direct OpenRouter request. `ELAPSED_MS` is used later to exclude slow models. The default council-selection threshold is 8000 milliseconds unless `--max-elapsed-ms` changes it.

A stale or partial `model-latency.csv` changes the eligible set. For example, a short 35-row latency file will exclude most clustered candidates as `latency_missing`. The 490/198/20 pipeline described below requires the full latency file used for the clustering run.

## Stage 4: Build The Retained Model List

Derive `models.csv` from the latency probe:

```bash
awk -F, '$2 != "timeout" && ($2 + 0) <= 8000 && $3 == "true" { print $1 }' \
  common/data/personas/model-latency.csv \
  | sort -u \
  > common/data/personas/models.csv
```

Record any threshold change. Changing this filter changes the model/persona candidate corpus.

## Stage 5: Build The Model/Persona Cross Product

Create `common/etc/personas.csv` from the retained model list and the checked-in persona files:

```bash
while IFS= read -r model; do
  [ -n "$model" ] || continue
  case "$model" in
    \#*) continue ;;
  esac
  rg --files common/etc/personas/persons | sort | while IFS= read -r persona; do
    printf '%s,%s\n' "$model" "${persona#common/etc/}"
  done
done < common/data/personas/models.csv > common/etc/personas.csv
```

This file is the broad source pool. It is larger than `council.csv` and `pool.csv`.

## Stage 6: Choose The Clustering Input

Clustering the full cross product can be expensive. Use a deliberate sampled input when refreshing the clustered candidate universe. The current clustering data was produced from sampled model/persona inputs, not from the 20-row council file.

A simple sampling pattern is:

```bash
shuf -n 100 common/etc/personas.csv > common/data/personas/some-personas.csv
```

`generate-council.py` writes the sampled clustering input to `common/data/personas/cluster-input.csv`. Because `cluster-personas.py` resolves persona paths relative to the input CSV, persona references in `cluster-input.csv` are written relative to `common/data/personas`, for example `../../etc/personas/persons/d715074-6.txt`.

## Stage 7: Generate Clusters And PCA Rows

Run `cluster-personas.py` over the sampled model/persona input and gene prompts:

```bash
uv run --script common/tools/cluster-personas.py \
  --personas-file common/data/personas/cluster-input.csv \
  --genes-file common/data/personas/genes.json \
  --pca-out common/data/personas/pca-cluster.csv \
  --num-personas all \
  --num-samples 3 \
  --num-genes 3 \
  > common/data/personas/clusters.csv
```

`clusters.csv` is headerless and has four columns:

```text
MODEL,PERSONA_FILE,GENE,CLUSTER
```

`pca-cluster.csv` is headerless and has seven columns:

```text
MODEL,PERSONA_FILE,GENE,PC1,PC2,PC3,CLUSTER
```

`cluster-personas.py` samples completions for each selected model/persona pair over the selected gene prompts, embeds those completions, reduces each gene's embedding set with PCA, assigns a cluster label, writes cluster assignments to stdout, and writes PCA rows to `--pca-out`.

The current `clusters.csv` contains 3921 rows representing 490 unique `(MODEL, PERSONA_FILE)` pairs. `select-council.py` treats those 490 unique pairs as the candidate universe.

## Stage 8: Select `council.csv`

Run `select-council.py` with the cluster rows, PCA rows, OpenRouter metadata, latency data, and operational-failure ledger:

```bash
uv run --script common/tools/select-council.py \
  --clusters common/data/personas/clusters.csv \
  --pca common/data/personas/pca-cluster.csv \
  --metadata common/data/personas/openrouter-models.json \
  --latency-csv common/data/personas/model-latency.csv \
  --failures common/data/personas/model-operational-failures.csv \
  --min-context 200000 \
  --size 20 \
  --out common/data/personas/council.csv \
  --report common/data/personas/council-report.md
```

Selection works in four steps:

1. Group `clusters.csv` by `(MODEL, PERSONA_FILE)`.
2. Exclude candidates that lack full coverage, fail metadata requirements, fall below the context threshold, fail latency/tool-support requirements, or appear in the operational-failure ledger.
3. Build a diversity vector for each eligible candidate. With `--pca`, this is the mean PCA vector per gene. Without `--pca`, the script falls back to cluster-frequency signatures.
4. Select `--size` rows by deterministic farthest-first selection with provider caps.

For the current cluster data and the restored full latency file, the expected result is:

```text
candidates=490 eligible=198 selected=20 out=common/data/personas/council.csv
```

`council.csv` is a historical selected council candidate set. It is not a current runtime pool, because it omits provider endpoint and quantization constraints.

## Stage 9: Runtime Pool Decision

`aar`/`arb` read JSONL request-spec pools. When `--council-pool` is not supplied, the runtime checks `./pool.jsonl`, then `<common-root>/data/personas/pool.jsonl`. The runtime does not read `clusters.csv`, `pca-cluster.csv`, `model-latency.csv`, `pool.csv`, or the council-selection report.

Do not promote a CSV file into a runtime pool. A runtime pool entry must preserve the request-spec fields needed for provider routing.

## Verification

After regenerating `council.csv`, check line count, path form, and uniqueness:

```bash
wc -l common/data/personas/council.csv
sort common/data/personas/council.csv | uniq | wc -l
awk -F, '{ if ($2 ~ /^\//) abs++; else rel++; if ($2 !~ /^personas\/persons\//) bad++; } END { printf "absolute=%d relative=%d bad_relative=%d total=%d\n", abs+0, rel+0, bad+0, NR }' common/data/personas/council.csv
sed -n '1,20p' common/data/personas/council.csv
```

Expected path-form check for a repository-clean `council.csv`:

```text
absolute=0 relative=20 bad_relative=0 total=20
```

Validate the selector and inspect the council-selection report if needed:

```bash
python3 -m py_compile common/tools/select-council.py
sed -n '1,120p' common/data/personas/council-report.md
```

Use the current JSONL pool workflow for runtime council pools.
