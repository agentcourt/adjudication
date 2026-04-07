# Agent Arbitration

Agent Arbitration is a distilled dispute-resolution procedure derived from the sibling [adc](../adc) copy.  This procedure removes pretrial motions, voir dire, the judge, and the clerk.  The merits are argued before a council.  The complaint states the proposition.  Policy or case configuration supplies the standard of evidence.

This repository contains the Lean engine, the Go runtime, the `aar` CLI, and a set of example cases.  The runtime writes a complete case packet for each run: complaint, policy, runtime limits, final state, council roster, transcript, digest, and event log.  The `aar case` command also prints a one-line JSON summary to stdout with the result and the final vote count.

## Layout

| Path | Purpose |
|---|---|
| `docs/` | Project rules and notes |
| `engine/` | Lean arbitration engine |
| `runtime/` | Go CLI and runtime bridge |
| `examples/` | Example disputes |

## Build

`make build` builds the Lean engine and the Go CLI into `.bin/`.

`make test` runs the Go tests.

`make prove` builds all of the theorems.

`make demo` drafts the first example complaint and runs one arbitration in `out/ex1-demo/`.

## Run An Arbitration From Scratch

These commands assume the current working directory is `arb/`.  `aar complain` reads a markdown file and extracts the `# Proposition` section into canonical complaint form.  `aar case` initializes the run from that complaint, loads the council pool from `../common/data/personas/pool.csv` by default, writes the run packet to the requested output directory, and prints a JSON summary to stdout.

Create a case directory with a situation file.  In the current implementation, the complaint format contains only the proposition.

```markdown
# Proposition

Whether the published statement defamed the plaintiff.
```

Build the engine and CLI, draft the complaint, and run the case:

```bash
make build
mkdir -p work/defamation
.bin/aar complain \
  --situation work/defamation/situation.md \
  --out work/defamation/complaint.md
.bin/aar case \
  --complaint work/defamation/complaint.md \
  --out-dir out/defamation-demo
```

`aar case` scans the complaint directory for case files when `--file` is absent.  That scan skips the complaint itself, the situation file, `README.md`, signing artifacts, and directories.  It loads `.txt`, `.md`, `.pem`, and `.b64` files as readable case files, and it records other file types as byte-bearing exhibits.

This variant shows the common parameters that change a run:

```bash
.bin/aar case \
  --complaint work/defamation/complaint.md \
  --file 'work/defamation/exhibits/*.txt' \
  --file work/defamation/statement.md \
  --out-dir out/defamation-demo \
  --policy etc/policy.json \
  --council-size 7 \
  --evidence-standard "Clear and convincing evidence." \
  --council-pool ../common/data/personas/pool.csv \
  --attorney-model 'openai://gpt-5?tools=search' \
  --timeout-seconds 120 \
  --acp-timeout-seconds 300 \
  --invalid-attempt-limit 2 \
  --run-id run-defamation-demo
```

The explicit `--file` path can be repeated.  It accepts shell globs, and it rejects `.gitignore`, `.sh`, and `.sig` files.  When you omit `--policy`, `aar case` loads `etc/policy.json` from the current working directory if that file exists.  Otherwise it uses the built-in default policy.

## Case Parameters

`aar help case` prints the full flag list.  These parameters control most runs:

| Flag | Meaning |
|---|---|
| `--complaint` | Complaint markdown file.  Required. |
| `--out-dir` | Output directory for the run packet.  Required. |
| `--file` | Explicit case file path or glob.  Repeating this flag replaces automatic complaint-directory scanning. |
| `--policy` | Policy JSON file.  Defaults to `./etc/policy.json` when present. |
| `--council-size` | Override `policy.council_size`. |
| `--evidence-standard` | Override `policy.evidence_standard`. |
| `--council-pool` | Council model and persona pool.  Defaults to `../common/data/personas/pool.csv` when `arb/` is the working directory. |
| `--attorney-model` | Attorney ACP model id, including any search capability request, such as `openai://gpt-5` or `openai://gpt-5?tools=search`. |
| `--common-root` | Shared `common/` tree used for the pool, xproxy config, and ACP launcher. |
| `--xproxy-config` | xproxy configuration file.  Defaults under `common/`. |
| `--xproxy-port` | xproxy port.  Default: `18459`. |
| `--timeout-seconds` | Council LLM timeout override. |
| `--acp-timeout-seconds` | Attorney ACP timeout override. |
| `--max-response-bytes` | Maximum parsed response size override. |
| `--invalid-attempt-limit` | Maximum invalid-attempt count before a participant is removed. |
| `--run-id` | Explicit run identifier. |
| `--engine` | Lean engine binary.  Defaults to `.bin/aarengine` next to the CLI binary. |

## Outputs

Each run writes a complete packet to `--out-dir`.  The main files are `complaint.md`, `policy.json`, `runtime.json`, `run.json`, `state.json`, `council.json`, `digest.md`, `transcript.md`, and `events.ndjson`.  Attorney work product is also exported into the run directory.

On success, `aar case` prints a JSON object like this:

```json
{"status":"ok","result":"demonstrated","votes_for":3,"votes_against":2,"run_id":"run-123","out_dir":"out/defamation-demo"}
```

On failure, it prints:

```json
{"status":"error","error":"..."}
```

## Examples

The checked-in Makefile targets show the current example configurations.  `make demo`, `make ex2`, and `make ex3` run with `openai://gpt-5` as the attorney model.  `make ex4` runs with `openai://gpt-5?tools=search`.
