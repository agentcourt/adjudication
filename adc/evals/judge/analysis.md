# Judge Voir Dire Eval Analysis

## Scope

This analysis covers the judge eval for `decide_voir_dire_question`.  The baseline fixture set contains 60 voir dire questions across allowed screening questions and prohibited precommitment, sufficiency, assumed-fact, merits-argument, and inadmissible-material questions.  The hard fixture set adds 30 tier-3 boundary questions focused on damages ranges, digital-evidence sufficiency, limiting-instruction phrasing with embedded disputed facts, missing-witness sufficiency, insurance references, and “could you still find” formulations.

The prompt-iteration path uses eval-local Markdown templates under [Judge Prompt Candidates](prompts/) and leaves the production Lean opportunity text unchanged.  Each live run records the prompt source, copied prompt text, results JSONL, and summary JSON under `evals/judge/out/`, which is generated local data and should not be committed.  The scorer uses deterministic reason tags, so explanation scores measure whether the ruling reason names an accepted category rather than whether a separate model agrees with the explanation.

## Results

| Prompt | Fixture Set | Correct | Reason Matches | False Allows | False Disallows | Invalid | Weighted Accuracy |
|---|---|---:|---:|---:|---:|---:|---:|
| production | baseline 60 | 60 | 60 | 0 | 0 | 0 | 1.000 |
| candidate-v1 | baseline 60 | 60 | 55 | 0 | 0 | 0 | 1.000 |
| candidate-v2 | baseline 60 | 59 | 58 | 1 | 0 | 0 | 0.973 |
| candidate-v3 | baseline 60 | 60 | 59 | 0 | 0 | 0 | 1.000 |
| production | hard 30 | 30 | 30 | 0 | 0 | 0 | 1.000 |
| candidate-v1 | hard 30 | 29 | 27 | 1 | 0 | 0 | 0.952 |
| candidate-v2 | hard 30 | 30 | 30 | 0 | 0 | 0 | 1.000 |
| candidate-v3 | hard 30 | 30 | 30 | 0 | 0 | 0 | 1.000 |

## Findings

The production opportunity prompt remains the best measured prompt.  It scored perfectly on both fixture sets after the deterministic scorer accepted the singular phrase “limiting instruction” as instruction-following wording.  That scorer adjustment did not change any ruling outcome; it corrected a vocabulary gap in explanation matching.

The hard fixture set was useful because it separated candidates that the baseline alone could not distinguish.  Candidate v1 preserved the baseline ruling outcomes, but it encouraged generic explanation language such as “class of evidence” and then failed the hard set by allowing a prohibited damages-range comfort question.  The false allow was `jvdh-002`: “If liability is proven, would you be comfortable returning a damages number between $80,000 and $120,000?”

Candidate v2 fixed the hard-set behavior but failed the original baseline on the same legal boundary.  It allowed `jvd-053`, a tier-3 damages-precommitment question asking whether the candidate would be comfortable returning a damages number in the $80,000 to $120,000 range if liability were proven.  That result shows why the original baseline should remain fixed while new hard sets test candidate prompts against additional failure clusters.

Candidate v3 directly addresses the damages-number cluster.  It distinguishes permissible questions about fixed damages bias or proof discipline from prohibited questions asking whether the candidate would be comfortable with, willing to return, able to award, or inclined to reject a named amount, range, minimum, maximum, or nominal-damages result.  Candidate v3 removed the false-allow cluster across both fixture sets, but it still lost one baseline reason match by classifying an attention question as digital or documentary evidence.

## Recommendation

Do not move any candidate prompt into production based on these runs.  Production scored 90 correct rulings and 90 reason matches across the combined measured sets, with no false allows, false disallows, or invalid responses.  Candidate v3 is the strongest eval-local candidate and should remain as a reference prompt for future work on the damages-number boundary.

Future fixture additions should follow the hard-set pattern.  Add boundary pairs that differ by one legal feature, preserve the original baseline for comparison, and run candidates against both sets before considering production changes.  The next useful clusters are questions that mix proper instruction-following language with disputed facts, and questions that turn a proper evidence-category screen into a specific sufficiency commitment.
