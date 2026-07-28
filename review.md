# Repository Review

Review of the `adjudication` repository for stale information, inconsistencies,
change-history narration in reference documents, and broken links.  Reviewed
2026-07-28 against commit `1f62a56`; the fixes are on branch `tidy`.

## Method

The review covered the 389 tracked Markdown files at the time, the root entry
documents, the per-system READMEs and manuals, and the generated-versus-committed
boundary in each `.gitignore`.  One script resolved every relative Markdown link
against the working tree; a second resolved backtick-quoted repository paths.
Greps located change-narration phrasing, version pins, model identifiers, and
command names, and each hit was checked against the code or the file system.
After the changes, `go build ./...` and `go test` over `common`, `adc`, `arb`,
`arbd`, and `web` pass, `model-pool/tools/audit_eval.py` reports no issues, and
the link checker reports five remaining links, all explained below.

## Structure

`evals/` held two systems that shared a parent directory and nothing else.  The
Python tooling that queries the OpenRouter catalog, evaluates provider endpoints,
clusters response embeddings, and samples juror pools is model-selection
infrastructure; it evaluates no part of the adjudication procedures.  The material
under `evals/adc/judge/` is fixtures and prompts for Go runners under
`adc/runtime/eval`, invoked through `adc eval`.  They shared no tool, data file,
language, working directory, or output location.

`model-pool/` is now a top-level directory and `evals/` holds behavior evals
alone.  Three documents titled "Adjudication Evals" or "Adjudication Evals
Manual" described only the model-pool subtree while `common/tools/README.md` and
`common/docs/README.md` used the same name for the whole tree; the model-pool
documents are now "Model Pool" and "Model Pool Manual", and the four link labels
name what they point at.  The overlapping document tables in `evals/README.md`,
`model-pool/README.md`, and `model-pool/docs/README.md` were merged so a parent
lists its children and a child lists its own contents.

The nine rule-level READMEs under `evals/adc/judge/rules/` opened with the same
fixture-coverage list as their suite README.  Each now states what the ARCP rule
requires and which judge failure has consequences, leaving fixtures, scorers, and
runners to the suite README.  A reader landing in `rules/rule52/` learns what
Rule 52 is without climbing back up the tree.

## Removed

| Path | Reason |
| --- | --- |
| `scratch/` (27 files) | Archived notes, superseded API drafts, and run observations.  Two files carried broken links, and `scratch/arb/lawyerapi.md` was behind the implementation: it lacked `pass_phase_opportunity`, which exists in both the code and `arb/manual.md`.  The Lawyer and Council API sections of `arb/manual.md` cover the current APIs, so the six `arb/devnotes.md` references now point there. |
| `model-pool/docs/history/` (3 files, 2,008 lines) | A second dated journal beside `devnotes.md`, listed inside an index that called itself stable documentation.  `history/sampling.md` carried a sampling procedure superseded by `docs/sampling-runbook.md`. |
| `CHANGES.md` | A root changelog nothing linked to, five commits behind, restating judge-eval scores that each suite's `analysis.md` and the cross-rule `plan.md` already carried.  Each system keeps its own dated `devnotes.md`. |
| `adc/latency.csv`, `adc/openrouter-models.txt` | Unreferenced probe output at the top of a system directory.  The live equivalents the tools read are `common/data/personas/model-latency.csv` and `openrouter-models.json`. |
| `arb/pool0.jsonl`, `common/data/personas/pool0.jsonl` | The superseded 19-row council pools beside the active 25-row files.  Nothing read them. |
| `example.sh` | A second copy of the arbitration example in `README.md`, referenced nowhere. |
| `model-pool/personas/` (53 files) | Byte-identical to `common/etc/personas/`.  See below. |
| `common/data/personas/genes.json` | Byte-identical to `model-pool/genes.json`.  See below. |

## Duplicated data

The 53 persona text files existed byte-identical in `common/etc/personas/` and
`model-pool/personas/`, and both copies were read: the Go runtimes reach the
first, the Python pool tools read the second.  `common/persona/persona.go:78-81`
resolves a pool record's `persona.path` by trying `baseDir/fileRef` and then
`baseDir/../../etc/fileRef`; live records carry `personas/generic.md` with base
`common/data/personas/`, so the first candidate misses and the runtime reads
`common/etc/personas/generic.md`.  `arbd/devnotes.md:69-71` records the two failed
attested runs that established this.  The `model-pool/` copy is gone and the two
tool defaults now point at `../common/etc/personas/generic.md`, so the persona
text the clustering sees is the text a live juror gets.

`genes.json` was the same duplication with the opposite answer.  No runtime reads
it; both copies fed only model-pool's own tools, and the legacy CSV tools defaulted
to the `common/` path because they used to live elsewhere.  The single copy is
`model-pool/genes.json`, and `generate-council.py` and `cluster-personas.py` now
default there.

## Broken links

Eleven link sites were broken and are fixed.  `evals/README.md` and
`model-pool/README.md` both indexed `analysis.md`, which the root `.gitignore`
rule `analysis.md` prevents from ever being committed; those rows are gone.  Two
`arb/examples` READMEs cited `scratch/ronaldo.md`, which was absent before the
`scratch/` removal; both now describe the source draft without a link, and
`ex02/README.md` also dropped the stale command name `agentarbitration`.
`arb/examples/ex11/source-captures/README.md` pointed at `../case-packet/market-page.txt`
for a file at `../market-page.txt`.  `model-pool/docs/sampling-runbook.md` tabled
`removed_variants.jsonl` as part of the committed 2026-05-29 snapshot;
`devnotes.md` line 61 shows why it is absent, since `run_end_to_end.py` writes
`filtered/removed_variants.jsonl` inside a run directory and that feature postdates
the snapshot.

Ten links pointed at `attest/dev-host.md` in a sibling repository.  That file
existed in neither repository: `/media/hd2/src/attest` has `dev.md`, whose
Prerequisites section is three lines, while the adjudication text described a
document covering the base host, Nix daemon setup, AWS CLI, EC2 and EBS-snapshot
permissions, role passing, VPC assumptions, disk, and verification commands.  The
missing document has since been located and added as
[Dev Host Requirements](docs/attest-host.md); it covers every item claimed, and
the ten links now resolve.

Five link sites remain broken and none is a defect.  Four are
`confession.sig.b64` and `samantha_public.pem` in `adc/examples/ex1/situation.md`
and `ex2/situation.md`, produced by the adjacent `sign.sh` and excluded by the
example `.gitignore`; `ex1/README.md` already runs `sign.sh` first and its Inputs
table now marks both files as generated.  `situation.md` is fed to the complaint
drafter, so build notes do not belong in it.  The fifth is
`adc/runtime/casegen/prompts/complaint_draft_system.md:8`, where a
`[label](path)` placeholder appears inside backticks as a format template for
the model.  My checker matched
inside the code span; this is a false positive, and my first report of it as a
defect was wrong.

## Change narration

Reference documents described changes instead of describing the system.  Each
fact was correct; the framing dated the sentence, so a later reader could not tell
whether "now" meant the current state or the state at some past commit.  The
pattern appeared where a capability reached one system before the others.

| File | Was | Is |
| --- | --- | --- |
| `arb/docs/verification.md:39` | "AAR now has a packet-level replay certificate path." | "AAR has a packet-level replay certificate path." |
| `arb/docs/verification.md:43` | "ADC and AARD now follow the same operator boundary" | "ADC and AARD use the same operator boundary" |
| `docs/proof-notes.md:28` | "The certificate plan has been carried across ... ADC and AARD now have runtime certificates" | "All three adjudication procedures carry the certificate path" |
| `docs/proof-notes.md:44` | "AARD now covers ... ADC now covers ... ARB closed certificates now carry" | Each stated in the present tense. |
| `arb/docs/councils.md:56` | "The procedure now waits ..."; "the strict-majority policy check established earlier" | The procedure waits; the strict-majority policy check.  "Established earlier" named development order a reader of the current rules cannot reconstruct. |
| `adc/docs/ARCP-commentary.md:5` | "Each ARCP rule now states operative text rather than incorporation by reference." | "Each ARCP rule states operative text rather than incorporating a federal rule by reference."  The clause described a superseded draft. |
| `adc/analysis/lean-complete-flow.md:11` | "Trial now includes ..." | "Trial includes ..." |
| `adc/docs/limits.md:77` | "Useful timing controls now include response windows for ..." | "The timing controls worth defining are ..."  The document opens by saying it proposes limits, and none of these controls appear in the engine or runtime, so the sentence stated a proposal as fact. |

The two eval `analysis.md` files that say "The scorer now accepts" and "The scorer
now distinguishes" are unchanged.  An analysis document recording a scorer
correction explains why its numbers differ from a naive run, which belongs there.

`adc/docs/voir-dire-journal.md` is unchanged.  I first reported its ten
`scratch/experiment1/` citations as undisclosed dangling references; lines 7-8
already say the raw outputs are not committed.  That disclosure is accurate and
the per-cycle paths label which local run produced each result.  The data is no
longer on disk, so it cannot be committed now.

## Documentation gaps

The `adc` command dispatches fourteen subcommands and `adc/manual.md` documented
twelve.  The two missing were `adc eval`, the entry point named in eleven eval
READMEs, and `adc juror`, the probe command used throughout the voir dire journal.
Both now have manual sections covering their options and purpose.  The `aar` and
`aard` manuals already matched their dispatchers.

`vmcp/` appeared in no table of the root README, and neither did `docs/vmcp.md`.
It is now a row in the Systems table with its design document, and the Build
section states that it builds with `lake build` rather than `make`, being a
standalone Lean package outside the Go module.  The Requirements table gained
`uv`, which every model-pool tool needs and which no requirement listed.

`evals/adc/judge/plan.md` listed `rules/rule55/`, `rules/rule59/`, `rules/rule62/`,
and `rules/rule26/` in the same unmarked table as ten real directories, while its
own status table called Rule 59 deferred and never mentioned the other three.  The
table is now split into directories that exist and areas that do not.  The
"Implementation Sequence" table listed eight steps of which seven were complete;
it is replaced by the work that remains, each with the condition that unblocks it.

`evals/.gitignore` carried an OpenClaw agent-home block ignoring `AGENTS.md`,
`TOOLS.md`, `USER.md`, `SOUL.md`, `IDENTITY.md`, and `HEARTBEAT.md` throughout
`evals/`.  The repository tracks `AGENTS.md` at its root, so any `AGENTS.md`
written under `evals/` would have vanished without a message.  Nothing under
`evals/` uses OpenClaw.  That file is now four lines, and `model-pool/` has its
own covering `results/` and `secrets/`.

`arb/manual.md` and `arbd/manual.md` gave container defaults without naming the
default OpenClaw model, which `adc/manual.md` did.  All three runtimes set
`gpt-5.5` with thinking `low`; all three manuals now say so in the same place.

## Duplicated results

Each judge suite's measured score appeared in four places: its `analysis.md`, the
"Prompt Iteration" section of its `plan.md`, the "Current Status" table in the
cross-rule `evals/adc/judge/plan.md`, and `CHANGES.md`.  Four copies of a number
that changes whenever a prompt or scorer changes give four chances to disagree.

`analysis.md` is now the single record of what a run measured.  Each suite
`plan.md` describes its fixture design and what each candidate prompt says, then
points at its analysis for the outcome.  The cross-rule status table is replaced
by a suite table giving fixture and candidate counts, which are properties of the
committed inputs rather than of a run, with a link to each analysis.  The
paragraph explaining how candidate prompts are rendered was repeated in six plans
and now appears once in `evals/adc/judge/README.md`.

The fixture counts in that table were checked against the files.  My first draft
said Rule 47 voir dire had "24 plus a hard set"; it has 60 baseline rows and 30
hard rows.

## Listen addresses

`adjudication-manage` defaulted to `127.0.0.1:9091` while every other server used
the `19xxx` block: `19770` and `19780` for arb service and MCP, `19790` and
`19800` for arbd, `19870` and `19880` for adc, `19980` for the report server, and
`19990` for the console.  The manage default is now `127.0.0.1:19970`, set in
`web/manage/config.go` and stated in `web/runbook.md`.

## Checks that passed unchanged

Go `1.25` in `README.md` matches `go.mod`.  Lean `v4.27.0` matches all four
`lean-toolchain` files.  The `gpt-5.5` default, the
`ghcr.io/openclaw/openclaw:latest` image, the `agentcourt-pi-sandbox` image, and
the pinned `pi-mcp-adapter@2.11.0` match the Go sources and
`common/pi-container/Dockerfile`.  The `aar` and `aard` subcommand tables match
their dispatchers.  The attested runner and wrapper script names in the three
`attested-dev-host.md` files match the files in `adc/tools/`, `arb/tools/`, and
`arbd/tools/`.  `vmcp/` commits no build output.

## Open

`model-pool/config/` holds two saved run records in a directory the README
documents as committed input.  No tool reads either file.  Both are kept by your
decision, and both README and manual now describe them as retained reference sets
rather than configuration.

`docs/attest-host.md` is a copy of a document belonging to the `attest`
repository.  The two will drift.  The alternative was a dead link, but the
duplication is worth knowing about.

## The second pool pipeline

`model-pool/` carried two pipelines that did the same job by different means.
Both picked a diverse set of model-and-persona pairs by sampling completions on
gene prompts, embedding them, reducing with PCA, and clustering per gene.  They
differed everywhere else: the legacy pipeline selected on the OpenRouter model
ID, screened with a metadata filter and a latency probe, chose by farthest-first
on PCA vectors, ran from the repository root, wrote CSV into
`common/data/personas/`, and produced a `council.csv` that its own document
called "not a current runtime pool, because it omits provider endpoint and
quantization constraints".  The current pipeline selects on the provider endpoint
variant, screens with a scored question set, samples tuple-uniformly over cluster
tuples, runs from `model-pool/`, and writes `pool.jsonl` records the runtimes
consume directly.

The current pipeline tools were last changed 2026-07-09, in the commit that
produced the active `common/data/personas/pool.jsonl`.  The legacy tools were
last touched 2026-06-02, the chart renderer 2026-04-03, and every legacy output
file was last regenerated 2026-05-21.

One dependency had to be settled first.  `adc pool` was a live Go subcommand with
a test, and it sampled `persona-clusters.csv`, a legacy artifact.  Its documented
usage wrote `common/data/personas/pool.jsonl` — the same runtime file the current
pipeline produces — while sampling on model ID and cluster alone, so it could not
carry the provider endpoint or quantization that a request spec needs.  Two
commands writing the same file under different rules was the root problem.  `adc
pool` is removed, along with `pool.go`, `pool_test.go`, and its manual section.

Removed with it: `filter-models.py`, `model-speed.sh`, `cluster-personas.py`,
`generate-council.py`, `select-council.py`, `docs/jury-pool-generation.md`, and
every file under `common/data/personas/` except `pool.jsonl`, plus
`common/etc/personas.csv`.  Three Go files used `personas.csv` as an alternate
marker for locating the common root; each now checks `pool.jsonl` alone, so no
code names a deleted file.

`clusters-graph.py` was the one capability only the legacy side had, and it is
kept.  It read a headerless positional CSV that only the legacy tools produced.
It now reads the headered `clusters.csv` that `run_gene_pca_clustering.py`
already writes, by column name.  Because that file carries `provider_name` and
`openrouter_model_id` as separate columns, the chart faceting changed meaning:
rows were the model author parsed out of the model string, and are now the
serving provider.  For endpoint-variant work that is the more useful reading.  I
verified the rewrite by generating a CSV in the real output format and rendering
it.  The [Sampling Runbook](model-pool/docs/sampling-runbook.md) carries the
command.
