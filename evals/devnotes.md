# Development Notes

## 2026-06-17 Manual Split

Moved the detailed evals README content into `manual.md` and kept `README.md` as a short index.  The manual contains purpose, operator guidance, command reference material, endpoint-variant procedures, scoring model, scope, and troubleshooting.  The README links to the manual, sampling runbook, and model-inventory notes, and keeps validation commands, a deterministic local test, credential notes, and the main file map.

## 2026-05-31 End-To-End Sampling Test

Goal: run a small end-to-end sampling test over five OpenRouter root models.  The test shape uses one eval trial, two genes, one sample per gene, the generic persona, PCA with dimensions capped by available rows, per-gene clustering, variant/persona aggregation, and tuple-uniform pool sampling.

Implementation note: generalized `tools/run_gene_pca_clustering.py` so the expected variant count and expected samples per variant are flags rather than historical constants.  Generalized `tools/aggregate_variant_persona_clusters.py` so one-sample test runs can aggregate without the historical three-sample requirement.  Synthetic validation passed with two variants, two genes, one sample per gene, and two PCA dimensions.

Run result: sampled five root models with seed `0`: `anthropic/claude-opus-4.7`, `mistralai/mistral-large-2407`, `openai/gpt-5.1`, `openai/gpt-audio`, and `qwen/qwen3-32b`.  Inventory found 18 endpoint variants and zero endpoint-fetch errors.  The one-trial core eval completed all 18 variant runs with no runner failures; 13 variants had provider errors, and five survived the historical filter `provider_error_count == 0` and `deliberation_score >= 0.90`.

Survivors: Mistral Large 2407 on Mistral, Qwen3 32B on DeepInfra, Qwen3 32B on Nebius, Qwen3 32B on AtlasCloud, and Qwen3 32B on Groq.  The two gene runs used `What do you think about religion?` and `What causes climate change?`; each wrote five records and five embeddings with no completion or embedding errors.  PCA used three dimensions for each gene.  Per-gene clustering selected `k=2` for both genes, aggregation wrote five variant/persona rows, and tuple-uniform sampling emitted five pool rows from two unique cluster tuples.

## 2026-05-31 End-To-End Runner

Added `tools/run_end_to_end.py`, a uv-runnable command for the endpoint-variant pool pipeline.  It calls the existing inventory, eval, filter, gene inference, PCA, clustering, aggregation, and tuple-pool tools, and writes a single run directory with `manifest.json`, `commands.jsonl`, stage subdirectories, and `summary.json`.  The runner supports explicit `--model-id` values, sampled roots, resume, dry-run, stage stops, configurable filter criteria, configurable genes and samples, PCA dimension capping, and pool sampling parameters.

Validation: `uv run --script tools/run_end_to_end.py --help` passed.  Dry-run over `/tmp/adjudication-evals-orchestrator-dry/dry` wrote the manifest and recorded the inventory command without calling OpenRouter.  Resume validation over `/tmp/adjudication-evals-orchestrator-resume-test/run` reused today’s completed inventory, eval, and gene inference artifacts, then ran filter, PCA, clustering, aggregation, and pool sampling to completion without live API calls.

Path-handling fix: generalized `tools/run_embedding_pca.py`, `tools/run_first_gene_inference_embeddings.py`, and `tools/run_variant_batch.py` so output and input paths outside the repository root can be reported without `Path.relative_to(ROOT)` failures.  `tools/run_gene_pca_clustering.py` and `tools/aggregate_variant_persona_clusters.py` already received the same display-path treatment during the small-run generalization.

Variant-timeout fix: `tools/run_variant_batch.py` now redirects each child eval run to `variant-runs/*/run_eval.log`, monitors raw-result and log progress without blocking on child stdout, and terminates a variant that exceeds `--no-progress-timeout` or `--variant-timeout`.  Timed-out variants are recorded in `progress.jsonl` and `variant_summary.csv`, and the batch exits with status 0 when timeout is the only variant failure.  `tools/run_end_to_end.py` passes the no-progress timeout through the eval stage and writes `filtered/removed_variants.jsonl` so timed-out variants are removed before gene inference.
