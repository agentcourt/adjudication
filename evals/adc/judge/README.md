# Judge Evals

Judge evals are grouped first by ARCP rule and then by the judge behavior under test.  Each behavior suite keeps stable inputs beside its prompt candidates, implementation plan, and analysis.  Rule 47 has separate suites for voir dire question screening and for-cause challenges because those decisions use different ADC tools and different scoring boundaries.

The Go runners live under `adc/runtime/eval`, with CLI defaults in `adc/runtime/cli/eval.go`.  Generated reports belong under `evals/out/adc/judge/`, which is ignored except for `.gitkeep` files.  The cross-rule plan is [Judge Eval Plan](plan.md), and the rule-grouped suite index is [Judge Rule Index](rules/README.md).

## Suites

| Rule | Suite | Runner | Analysis |
| --- | --- | --- | --- |
| Rule 11 | [Sanctions](rules/rule11/sanctions/README.md) | `judge-rule11` | [Rule 11 Sanctions Analysis](rules/rule11/sanctions/analysis.md) |
| Rule 12 | [Dismissal and Jurisdiction](rules/rule12/dismissal-jurisdiction/README.md) | `judge-rule12` | [Rule 12 Analysis](rules/rule12/dismissal-jurisdiction/analysis.md) |
| Rule 37 | [Discovery Sanctions](rules/rule37/discovery-sanctions/README.md) | `judge-rule37` | [Rule 37 Analysis](rules/rule37/discovery-sanctions/analysis.md) |
| Rule 47 | [Voir Dire Question Screening](rules/rule47/voir-dire-question/README.md) | `judge-voir-dire` | [Rule 47 Voir Dire Analysis](rules/rule47/voir-dire-question/analysis.md) |
| Rule 47 | [For-Cause Challenges](rules/rule47/for-cause-challenge/README.md) | `judge-for-cause` | [Rule 47 For-Cause Analysis](rules/rule47/for-cause-challenge/analysis.md) |
| Rule 51 | [Jury Instructions](rules/rule51/jury-instructions/README.md) | `judge-rule51` | [Rule 51 Analysis](rules/rule51/jury-instructions/analysis.md) |
| Rule 52 | [Bench Opinion](rules/rule52/bench-opinion/README.md) | `judge-rule52` | [Rule 52 Analysis](rules/rule52/bench-opinion/analysis.md) |
| Rule 56 | [Summary Judgment](rules/rule56/summary-judgment/README.md) | `judge-rule56` | [Rule 56 Analysis](rules/rule56/summary-judgment/analysis.md) |
| Rule 58 | [Judgment Entry](rules/rule58/judgment-entry/README.md) | `judge-rule58` | [Rule 58 Analysis](rules/rule58/judgment-entry/analysis.md) |
| Rule 60 | [Relief From Judgment](rules/rule60/relief-from-judgment/README.md) | `judge-rule60` | [Rule 60 Analysis](rules/rule60/relief-from-judgment/analysis.md) |
