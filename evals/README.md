# Evals

`evals/` holds behavior evals for the adjudication systems: fixture sets that put a system actor in a controlled state, prompt candidates that can be compared against the production prompt, and the analysis that records what a run measured.  The runners are Go code inside the system runtimes, so an eval exercises the same state construction, tool schema, model path, and Lean validation as a live case.  [ADC Evals](adc/README.md) is the only actor tree so far, and [Judge Evals](adc/judge/README.md) is the only suite family within it.

Committed eval directories hold fixtures, prompt candidates, plans, and analysis.  Generated run output belongs under `out/`, which is ignored except for `out/.gitkeep`.  Model and provider-endpoint selection is a separate concern and lives in [Model Pool](../model-pool/README.md); it builds the juror and council pools that live runs draw from rather than measuring actor behavior.

## Contents

| Path | Contents |
| --- | --- |
| [adc/](adc/README.md) | ADC behavior evals, grouped by actor. |
| `out/` | Ignored generated output from eval runs. |
