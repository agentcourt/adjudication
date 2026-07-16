# Evals

`evals/` is the repository root for committed eval assets and local eval output.  `model-pool/` contains the provider-endpoint, question-set, embedding, clustering, and pool-sampling eval system.  `adc/` contains ADC behavior evals organized by actor, with judge evals under `adc/judge/`.

Generated output belongs under `out/` for cross-ADC eval runs and under `model-pool/results/` for model-pool tools that run from `evals/model-pool/`.  Those directories are ignored except for `.gitkeep` files.  Committed eval directories should contain fixtures, prompt candidates, plans, analysis, schemas, rubrics, and source tooling, rather than raw run output.

## Layout

| Path | Contents |
| --- | --- |
| `model-pool/` | Model and provider-endpoint eval tooling, checked-in question sets, schemas, rubrics, prompts, personas, variants, and docs. |
| `adc/judge/` | Judge behavior eval fixtures, prompt candidates, plans, and analysis, grouped by rule and behavior. |
| `out/` | Ignored local output from ADC behavior evals. |
