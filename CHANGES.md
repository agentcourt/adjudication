# Changes

## July 23-24, 2026

### Run Report Web UI

The repository gained a read-only web report over run output directories on disk.  `adjudication-report` scans configured root trees for run directories, so one process reports across several checkouts and data trees at once.  It serves an index of runs with status, resolution, and vote tallies, run pages with facts, council votes, events, and complete file tables, and views of every artifact: markdown rendered by an internal minimal renderer with text and raw toggles, JSON pretty-printed, NDJSON one record per line, and raw bytes with range support.  The server confines request paths to their configured roots, including through symbolic links.

The scanner treats a directory holding a known artifact file as a run, stops descending at run directories, skips hidden directories and Pi agent homes, and reports unreadable or depth-limited directories in a scan problems table instead of hiding them.  Failed attempts that wrote only logs appear with status `incomplete`.  The visual style follows the reconometrics dashboard: server-rendered monospace tables with client-side sorting and no other scripting.

### ARB Management Web UI

`adjudication-manage` starts, monitors, and stops ARB cases through one `aar service`: clerk and attested cases through the Clerk API, direct cases through the direct case API.  The overview, list, and case pages read the service records, show results for terminal cases and attestation events for attested cases, and offer kill and cancel actions only while a case runs.  Each case links to its report run page through configured mappings from case output directories to report roots, so reading stays in the report server.

A single field-descriptor table drives both the grouped start form and the create payload, one entry per documented create field, with a raw JSON page for requests the form cannot express.  The attested form sends only case selectors plus the attestation object because the service rejects runtime overrides in attested mode.  POST routes reject cross-origin browser senders, so a hostile page in an operator's browser cannot start or kill runs.

## July 3-16, 2026

### Judge Voir Dire Evals

The first judge eval suite tests whether the ADC judge screens lawyer voir dire questions before a juror candidate sees them.  It separates proper bias, burden, evidence-category, damages-skepticism, attention, and instruction-following questions from questions that ask a juror to forecast a vote, commit to damages, weigh named evidence, accept assumed facts, hear merits argument, or react to inadmissible material.  The suite uses real ADC state and opportunity generation, fixed fixture sets, deterministic reason tags, eval-local prompt candidates, and analysis files that record prompt-iteration results without changing the production judge prompt.

This eval established the working pattern for judge-prompt improvement.  A production prompt, candidate prompts, baseline fixtures, and hard fixtures can be compared without changing ADC execution semantics.  The hard voir dire rows also made the key boundary concrete: a lawyer may ask whether a juror can follow a rule or evaluate a class of evidence, but may not ask for a case-specific vote forecast, a sufficiency commitment, or comfort with a named damages number.

### Expanded Judge Eval Suite

The judge eval work now covers a broader set of procedural decisions: Rule 56 summary judgment, Rule 12 dismissal and jurisdiction, Rule 51 jury instructions, Rule 47 for-cause challenges, Rule 37 discovery sanctions, Rule 11 sanctions, Rule 52 bench opinions, Rule 58 judgment entry, and Rule 60 relief from judgment.  These suites test whether the judge preserves the legal boundary of each decision: no factfinding at summary judgment, no disbelief of pleaded facts at Rule 12, no argumentative or evidence-contaminated jury instructions, no sanctions for reasonable advocacy, no judgment inconsistent with the verdict, and no post-judgment relief without a recognized ground.  The eval assets now live under `evals/adc/judge/rules/...`, with generated run output kept under ignored `evals/out/adc/judge/`.

The expanded suite makes judge behavior measurable across decisions that can terminate claims, alter the trial record, change jury composition, or disturb a final judgment.  Each suite keeps fixtures, prompt candidates, plans, and analysis together so a prompt change can be assessed against a stable decision set.  The current analyses distinguish prompt candidates that improve measured behavior from candidates that only restate the rule or move failures between categories.

### Replay Certificates And Proofs

Replay certificates now bind terminal output packets to the accepted Lean transition sequence that produced them.  ARB has the reference certificate path, with certificate writing, an explicit verifier, service artifact access, and proof facts for replay, reachability, terminal accounting, decision-rule behavior, due-process filings, and failed-opportunity records.  ADC and AARD adopted the same operator boundary with procedure-specific certificate schemas, verifier commands, service artifact exposure, and proof packages that match their engines.

The proof work now covers more of the executable claims exposed by the runtimes.  ADC certificate facts cover closed outcomes, verdicts, juror-timeout verdicts, juror-timeout hung juries, and judgments.  AARD certificate facts cover closed answer pairs and failed-case failure records, while ARB certificate facts include matched-case closed-resolution agreement after the decision-rule inputs have been matched.

### Service Web Console

The repository gained a server-rendered web console for the ADC, ARB, and AARD service APIs.  The console provides operator pages for case lists, case details, creation forms, artifacts, evidence, results, recent events, raw service responses, event tails, and service logs.  It uses the existing service APIs and artifact routes, including range requests for large event and log files, so live progress can be inspected without adding a new runtime output format.

The console work also clarified several service-facing behaviors.  Large JSON and structured records are rendered with size bounds, missing cases preserve service errors, inactive management actions are hidden, and evidence-use labels appear when a service manifest supplies them.  Event rendering improved enough to expose phases, attorney actions, juror votes, council votes, failures, and recent activity across the three procedures.

### Model-Pool Evals And Shared Pools

The model-pool eval tooling moved into `evals/model-pool/`, with committed inputs separated from generated run output.  The pipeline covers provider-endpoint inventory, endpoint-variant evaluation, behavior-prompt response collection, embedding reduction, cluster aggregation, and tuple-uniform pool sampling.  The shared persona pool data was refreshed so runtime juror and council pools align with the newer eval pipeline.

The eval tree now covers both model-pool experiments and ADC behavior evals.  Model-pool docs, schemas, rubrics, prompts, personas, variants, and tools are grouped under one subtree, while ADC judge evals live under `evals/adc/judge/`.  README indexes now point readers to the right analysis, runbook, fixture, and prompt locations without embedding run-specific data.

### ADC Juror And Voir Dire Exploration

ADC gained an `adc juror` probe command for asking a single pool member a controlled question and, when needed, repeating samples or carrying a transcript forward.  The command supports the voir dire experiment track by making cheap, direct juror interrogation possible before full case runs.  The accompanying journal records what voir dire questions reveal about pool members, how those answers might inform strikes and argument, and which observations need confirmation against vote behavior.

The voir dire exploration complements the judge evals.  The judge evals ask whether a proposed lawyer question is permitted, while the juror probes ask whether an allowed question reveals a useful decision trait.  Together they define the next practical question for ADC jury work: which judge-allowable questions give a lawyer information that survives noisy answers, model drift, and case variation.

### Repository Documentation

The eval documentation was reorganized around the current directory structure, and old one-off plans left the repository root.  Cross-system proof notes moved to `docs/`, the active voir dire experiment plan moved to `adc/docs/`, and stale root notes were deleted after their live references were redirected.  The root now points to systems, evals, shared directories, and cross-system proof notes without carrying dated planning files as top-level entry points.
