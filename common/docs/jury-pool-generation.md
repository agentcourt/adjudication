# Jury Pool Generation

This manual describes how to build the active jury pool file from scratch.  The active file is [Juries](../etc/personas.csv).  Both `adc` and `arb` consume that file by default, so any rewrite changes the candidate pool for both systems.  The rest of the files in this process either feed that file or record how it was derived.

This process has two stages.  The first stage builds the active pool file directly from a model list and the checked-in persona files.  The second stage, if you choose to use it, measures behavioral variation over a prompt set, records cluster assignments, and samples a smaller pool from those assignments.  The clustered outputs remain separate until you copy the sampled result into [Juries](../etc/personas.csv).

## Files And Working Directories

The commands in this manual assume two working directories.  Run the shared tools from the repository root, `/home/somebody/src/adjudication`.  Run `adc pool` from `/home/somebody/src/adjudication/adc`, because that command reads `../common/data/personas/persona-clusters.csv` relative to the current working directory.  The tools use the working directory for their default paths.

These files matter in the current pipeline:

| File | Role |
|---|---|
| [Juries](../etc/personas.csv) | Active pool consumed by `adc` and `arb` |
| [Persona files](../etc/personas/persons) | Checked-in persona texts used in the pool |
| [Candidate models](../data/personas/models.csv) | One model id per line after filtering |
| [Latency and tool support](../data/personas/model-latency.csv) | `MODEL,ELAPSED_MS,TOOLS_SUPPORTED` records from [`model-speed.sh`](../tools/model-speed.sh) |
| [Gene prompts](../data/personas/genes.json) | Prompt set used for clustering |
| [Cluster assignments](../data/personas/persona-clusters.csv) | `MODEL,PERSONA_FILE,GENE,CLUSTER` rows from the clustering run |
| [PCA rows](../data/personas/personas-pca.csv) | Per-sample projected coordinates written by [`cluster-personas.py`](../tools/cluster-personas.py) |
| [Sampled clustered pool](../data/personas/pool.csv) | Candidate reduced pool generated from cluster assignments |
| [Optional sampled subset](../data/personas/some-personas.csv) | Smaller input file for exploratory clustering runs |

Each row in [Juries](../etc/personas.csv) has two columns: an xproxy model id and a persona path relative to `common/etc`.  A valid row looks like `openrouter://openai/gpt-4o,personas/persons/d715074-0.txt`.  The runtime reads the file as written, so malformed rows break the pool instead of being repaired on load.

## Baseline Generation

The baseline path starts with a candidate model list, filters that list for tool support and response time, and then forms the cross product with the checked-in persona files.

Build `adc` first, because [`model-speed.sh`](../tools/model-speed.sh) calls `adc/.bin/adc llm --tool-check`.  Start local xproxy from the repository root before you run [`model-speed.sh`](../tools/model-speed.sh) or [`cluster-personas.py`](../tools/cluster-personas.py).  Both steps depend on that service path.  From the repository root, run:

```bash
adc/.bin/adc xproxy
```

In another shell, still from the repository root, probe candidate models.  This example uses the checked-in OpenRouter snapshot in [OpenRouter models](../../adc/openrouter-models.txt) as the candidate list and one checked-in persona file as the probe persona.  Provider inventories change, so this file is only one possible input set.  From the repository root, run:

```bash
common/tools/model-speed.sh common/etc/personas/persons/d715074-0.txt \
  < adc/openrouter-models.txt \
  > common/data/personas/model-latency.csv
```

That command writes `MODEL,ELAPSED,TOOLS_SUPPORTED` rows.  The repository currently keeps models with `TOOLS_SUPPORTED=true` and latency at or below `8000` milliseconds.  If you choose a different threshold, record the reason.  From the repository root, run:

```bash
awk -F, '$2 != "timeout" && ($2 + 0) <= 8000 && $3 == "true" { print $1 }' \
  common/data/personas/model-latency.csv \
  | sort -u \
  > common/data/personas/models.csv
```

Once [Candidate models](../data/personas/models.csv) is in place, generate the active pool file as the cross product of those models and the checked-in persona texts under [Persona files](../etc/personas/persons).  The persona paths in the output must stay relative to `common/etc`.  The command below writes the full file in one pass, and it does not depend on any hidden path discovery.  From the repository root, run:

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

At that point `adc` and `arb` will both read [Juries](../etc/personas.csv) by default.  You can stop there, or continue to the clustering stage and derive a smaller pool from the generated file.

## Optional Clustering

Clustering gives you a second selection stage.  It samples completions for model and persona pairs over the prompt set in [Gene prompts](../data/personas/genes.json), embeds those completions, reduces them with PCA, assigns a cluster label within each gene, and writes both the cluster rows and the PCA rows.  The runtime does not read those files.  They exist to support a smaller or more behaviorally varied pool.

You can cluster the full active pool, but that is expensive.  The checked-in workflow uses a smaller input file when exploring or refreshing the clustered sample.  The simplest way to create that subset is to sample rows from [Juries](../etc/personas.csv).  From the repository root, run:

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

Run `adc pool` from `adc/`.  The command expects `../common/data/personas/persona-clusters.csv` to exist relative to that directory.  The checked-in `pool` Makefile target already writes its output to [Sampled clustered pool](../data/personas/pool.csv).  From `adc/`, run:

```bash
.bin/adc pool --size 50 | sort | uniq > ../common/data/personas/pool.csv
wc -l ../common/data/personas/pool.csv
```

The runtime continues to read [Juries](../etc/personas.csv) until you copy [Sampled clustered pool](../data/personas/pool.csv) over the active file.  From the repository root, run:

```bash
cp common/data/personas/pool.csv common/etc/personas.csv
```

## Verification

After any regeneration, inspect the resulting active file directly.  A valid pool file has no blank lines, no comment lines, and no malformed rows.  The fastest useful checks are line count, uniqueness, and a small visual inspection of the first few rows.  Those checks do not prove quality, but they catch broken files immediately.  From the repository root, run:

```bash
wc -l common/etc/personas.csv
sort common/etc/personas.csv | uniq | wc -l
sed -n '1,20p' common/etc/personas.csv
```

If you used clustering, inspect the auxiliary files as well.  [Cluster assignments](../data/personas/persona-clusters.csv) should have four columns.  [PCA rows](../data/personas/personas-pca.csv) should have model, persona file, gene index, projected coordinates, and a final cluster number.  [Sampled clustered pool](../data/personas/pool.csv) should have the same two-column format as the active pool file.
