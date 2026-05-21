# Jury Pool Generation

This manual describes how to build the files that feed jury and council sampling.  The runtimes consume [the active runtime pool](../data/personas/pool.csv) by default.  The broader candidate corpus lives in [the source pool](../etc/personas.csv), together with the persona texts and the intermediate clustering data.

This process has two stages.  The first stage builds [the source pool](../etc/personas.csv) directly from a model list and the checked-in persona files.  The second stage, if you choose to use it, measures behavioral variation over a prompt set, records cluster assignments, and samples [the active runtime pool](../data/personas/pool.csv) from those assignments.

## Files And Working Directories

The commands in this manual assume two working directories.  Run the shared tools from the repository root, `/home/somebody/src/adjudication`.  Run `adc pool` from `/home/somebody/src/adjudication/adc`, because that command reads `../common/data/personas/persona-clusters.csv` relative to the current working directory.  The tools use the working directory for their default paths.

These files matter in the current pipeline:

| File | Role |
|---|---|
| [Source pool](../etc/personas.csv) | Cross-product candidate pool used as clustering input |
| [Persona files](../etc/personas/persons) | Checked-in persona texts used in the pool |
| [Candidate models](../data/personas/models.csv) | One model id per line after filtering |
| [Latency and tool support](../data/personas/model-latency.csv) | `MODEL,ELAPSED_MS,TOOLS_SUPPORTED` records from [`model-speed.sh`](../tools/model-speed.sh) |
| [Gene prompts](../data/personas/genes.json) | Prompt set used for clustering |
| [Cluster assignments](../data/personas/persona-clusters.csv) | `MODEL,PERSONA_FILE,GENE,CLUSTER` rows from the clustering run |
| [PCA rows](../data/personas/personas-pca.csv) | Per-sample projected coordinates written by [`cluster-personas.py`](../tools/cluster-personas.py) |
| [Active runtime pool](../data/personas/pool.csv) | Sampled pool consumed by `adc` and `arb` |
| [Optional sampled subset](../data/personas/some-personas.csv) | Smaller input file for exploratory clustering runs |

Each row in [the source pool](../etc/personas.csv) and [the active runtime pool](../data/personas/pool.csv) has two columns: an xproxy model id and a persona path relative to `common/etc`.  A valid row looks like `openrouter://openai/gpt-4o,personas/persons/d715074-0.txt`.  The runtime reads the file as written, so malformed rows break the pool instead of being repaired on load.

## Baseline Generation

The baseline path starts with a candidate model list, filters that list for tool support and response time, and then forms the cross product with the checked-in persona files.

Build `adc` first, because [`model-speed.sh`](../tools/model-speed.sh) calls `adc/.bin/adc llm --tool-check`.  Start local xproxy from the repository root before you run [`model-speed.sh`](../tools/model-speed.sh) or [`cluster-personas.py`](../tools/cluster-personas.py).  Both steps depend on that service path.  From the repository root, run:

```bash
adc/.bin/adc xproxy
```

Before the live probe, apply the conservative metadata prefilter.  The prefilter skips only models whose provider metadata proves that the model cannot satisfy the juror path: no text input, no text output, or no advertised function-tool support.  Unknowns remain in the probe set.  Fetch the current OpenRouter metadata into [OpenRouter model metadata](../data/personas/openrouter-models.json), then run:

```bash
common/tools/filter-models.py \
  --metadata common/data/personas/openrouter-models.json \
  --out common/data/personas/models-prefiltered.csv \
  --decisions common/data/personas/model-filter-decisions.csv
```

In another shell, still from the repository root, probe candidate models.  This example uses the prefiltered OpenRouter list and one checked-in persona file as the probe persona.  Provider inventories change, so this file is only one possible input set.  From the repository root, run:

```bash
common/tools/model-speed.sh common/etc/personas/persons/d715074-0.txt \
  < common/data/personas/models-prefiltered.csv \
  > common/data/personas/model-latency.csv
```

That command writes `MODEL,ELAPSED,TOOLS_SUPPORTED` rows.  The repository currently keeps models with `TOOLS_SUPPORTED=true` and latency at or below `8000` milliseconds.  If you choose a different threshold, record the reason.  From the repository root, run:

```bash
awk -F, '$2 != "timeout" && ($2 + 0) <= 8000 && $3 == "true" { print $1 }' \
  common/data/personas/model-latency.csv \
  | sort -u \
  > common/data/personas/models.csv
```

Once [Candidate models](../data/personas/models.csv) is in place, generate [the source pool](../etc/personas.csv) as the cross product of those models and the checked-in persona texts under [Persona files](../etc/personas/persons).  The persona paths in the output must stay relative to `common/etc`.  The command below writes the full file in one pass, and it does not depend on any hidden path discovery.  From the repository root, run:

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

At that point the candidate corpus exists in [the source pool](../etc/personas.csv).  You can stop there if you only need the full cross product, or continue to the clustering stage and derive [the active runtime pool](../data/personas/pool.csv).

## Optional Clustering

Clustering gives you a second selection stage.  It samples completions for model and persona pairs over the prompt set in [Gene prompts](../data/personas/genes.json), embeds those completions, reduces them with PCA, assigns a cluster label within each gene, and writes both the cluster rows and the PCA rows.  The runtime does not read those files.  They exist to support a smaller or more behaviorally varied pool.

You can cluster the full source pool, but that is expensive.  The checked-in workflow uses a smaller input file when exploring or refreshing the clustered sample.  The simplest way to create that subset is to sample rows from [the source pool](../etc/personas.csv).  From the repository root, run:

```bash
shuf -n 100 common/etc/personas.csv > common/data/personas/some-personas.csv
```

Now run [`cluster-personas.py`](../tools/cluster-personas.py).  This example uses the sampled subset, writes PCA rows to [PCA rows](../data/personas/personas-pca.csv), and writes cluster assignments to [Cluster assignments](../data/personas/persona-clusters.csv).  The script must run from the repository root unless you pass every path explicitly.  From the repository root, run:

```bash
uv run --script common/tools/cluster-personas.py \
  --personas-file common/data/personas/some-personas.csv \
  --genes-file common/data/personas/genes.json \
  --pca-out common/data/personas/personas-pca.csv \
  --num-personas all \
  --num-samples 3 \
  --num-genes 3 \
  > common/data/personas/persona-clusters.csv
```

If you want to inspect the projected data before sampling a reduced pool, render the facet graph with [`clusters-graph.py`](../tools/clusters-graph.py) from the repository root.  [`cluster-personas.py`](../tools/cluster-personas.py) already wrote the PCA rows.  The runtime does not read the graph.  From the repository root, run:

```bash
uv run --script common/tools/clusters-graph.py \
  --pca common/data/personas/personas-pca.csv \
  --out clusters.png
```

## Sampling A Reduced Pool

Once [Cluster assignments](../data/personas/persona-clusters.csv) exists, `adc pool` can sample a smaller set of model and persona pairs from it.  The command collapses repeated `MODEL,PERSONA_FILE,GENE,CLUSTER` rows into per-pair cluster membership, applies a random sequence of gene and cluster filters, and then chooses a surviving pair.  The sampler uses replacement, so deduplicate the output if you want a unique file.

Run `adc pool` from `adc/`.  The command expects `../common/data/personas/persona-clusters.csv` to exist relative to that directory.  The checked-in `pool` Makefile target already writes its output to [the active runtime pool](../data/personas/pool.csv).  From `adc/`, run:

```bash
.bin/adc pool --size 50 | sort | uniq > ../common/data/personas/pool.csv
wc -l ../common/data/personas/pool.csv
```

`adc` and `arb` both read [the active runtime pool](../data/personas/pool.csv) by default.  Regenerating that file changes the default sampled pool for both systems.

## Verification

After any regeneration, inspect [the active runtime pool](../data/personas/pool.csv) directly.  A valid pool file has no blank lines, no comment lines, and no malformed rows.  The fastest useful checks are line count, uniqueness, and a small visual inspection of the first few rows.  Those checks do not prove quality, but they catch broken files immediately.  From the repository root, run:

```bash
wc -l common/data/personas/pool.csv
sort common/data/personas/pool.csv | uniq | wc -l
sed -n '1,20p' common/data/personas/pool.csv
```

If you used clustering, inspect the auxiliary files as well.  [Cluster assignments](../data/personas/persona-clusters.csv) should have four columns.  [PCA rows](../data/personas/personas-pca.csv) should have model, persona file, gene index, projected coordinates, and a final cluster number.  [The active runtime pool](../data/personas/pool.csv) should have the same two-column format as [the source pool](../etc/personas.csv).
