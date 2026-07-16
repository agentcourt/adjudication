# Evals

`evals/` is the repository root for committed eval assets and local eval output.  [Model-Pool Evals](model-pool/README.md) contains the provider-endpoint, question-set, embedding, clustering, and pool-sampling eval system.  [ADC Evals](adc/README.md) contains ADC behavior evals organized by actor, with [Judge Evals](adc/judge/README.md) under `adc/judge/`.

Generated output belongs under `out/` for ADC behavior eval runs and under `model-pool/results/` for model-pool tools that run from `evals/model-pool/`.  Those directories are ignored except for `.gitkeep` files.  Committed eval directories contain fixtures, prompt candidates, plans, analysis, schemas, rubrics, and source tooling.

## Documentation

| Document | Use |
| --- | --- |
| [Model-Pool Evals](model-pool/README.md) | Model and provider-endpoint eval tooling, checked-in question sets, schemas, rubrics, prompts, personas, variants, and docs. |
| [Model-Pool Analysis](model-pool/analysis.md) | Human analysis tied to model-pool eval results or pool-construction work. |
| [ADC Evals](adc/README.md) | ADC behavior eval organization and output conventions. |
| [Judge Evals](adc/judge/README.md) | Judge eval suites, runner locations, and cross-rule planning. |
| [Judge Rule Index](adc/judge/rules/README.md) | Rule-grouped judge eval suites. |

## Layout

| Path | Contents |
| --- | --- |
| `model-pool/` | Model-pool eval tooling, inputs, prompt files, schemas, rubrics, personas, variants, and documentation. |
| `adc/judge/` | Judge behavior eval fixtures, prompt candidates, plans, and analysis, grouped by rule and behavior. |
| `out/` | Ignored local output from ADC behavior evals. |
