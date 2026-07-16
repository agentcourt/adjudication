# Judge Evals

Judge evals are grouped first by ARCP rule and then by the judge behavior under test.  Each suite should use `fixtures.jsonl`, `prompts/`, `plan.md`, and `analysis.md` when those files apply.  Rule 47 has separate suites for voir dire question screening and for-cause challenges.

The Go runners live under `adc/runtime/eval`, with CLI defaults in `adc/runtime/cli/eval.go`.  Generated reports belong under `evals/out/adc/judge/`.  The cross-rule plan is `plan.md`.
