# Agent District Court

[`agentcourt.ai`](https://agentcourt.ai/) and [`@agentcourt_ai`](https://x.com/agentcourt_ai)


ADC is a system for agent-driven civil litigation.  ADC is implemented in Go and Lean.

This repository contains the Lean rule engine, the Go runtime, the proof tree, the `ex1` example, the `xproxy` package and config, and the ACP container path for external attorney agents.

Lean enforces procedure and state transitions.  Go handles intake, prompt assembly, storage, ACP transport, and reports.  The ACP path delegates live attorney turns to external agents.  In the default `ex1` path, both plaintiff and defense counsel run through ACP, `pi` runs inside Podman, and the host `xproxy` keeps provider keys outside the container.

## Warning

Not even at "alpha" level.


## Requirements

To build from a fresh checkout:

| Requirement               | Notes                                                           |
|---------------------------|-----------------------------------------------------------------|
| `make`                    | Runs the top-level `build`, `test`, `prove`, and `demo` targets |
| Go `1.25`                 | Required by the root `go.mod`                                   |
| Lean `4.27.0` with `lake` | Required by `engine/lean-toolchain`                             |
| Podman                    | Builds and runs the `pi` image                                  |
| Git                       | Populates `../common/submodules/pi-acp` for the image build     |

To run the default `ex1` demo after building:

| Requirement       | Notes                                                                                                                             |
|-------------------|-----------------------------------------------------------------------------------------------------------------------------------|
| Provider API keys | `OPENAI_API_KEY` is required for ACP attorneys through `xproxy`.  `OPENROUTER_API_KEY` is required for the checked-in juror pool. |
| Network access    | Required for upstream model providers                                                                                             |

For the documented `ex1` run, set these variables before you start:

```bash
export OPENAI_API_KEY=...
export OPENROUTER_API_KEY=...
```

## Quick start

From a fresh checkout:

```bash
git submodule update --init --recursive
make build
```

Then set the required API keys:

```bash
export OPENAI_API_KEY=...
export OPENROUTER_API_KEY=...
```

Then start the end-to-end scenario:

```bash
make demo
```

This execution can take more than 30 minutes.  The current full demo path includes judge-screened voir dire, repeated plaintiff and defense evidence phases with explicit `rest_case` decisions, and per-side exhibit caps.  `make build` also rebuilds the Podman image.  That image now contains both `pi` and `pi-acp`.  `make demo` is the canonical scenario entrypoint.

## Build

Build the local binaries:

```bash
make build
```

That writes `.bin/adc` and `.bin/adcengine`.

Build the Lean proofs:

```bash
make prove
```

Run the Go tests:

```bash
make test
```

Test the ACP path directly:

```bash
.bin/adc acp --prompt "Reply with one sentence."
```

Build the `pi` container image:

```bash
../common/pi-container/build-image.sh
```

## Run `ex1`

`ex1` is the main end-to-end example.  It starts from `examples/ex1/situation.md`, regenerates the signature artifacts, drafts `examples/ex1/complaint.md`, delegates both attorneys through ACP, runs `pi` and `pi-acp` in Podman for those attorneys, keeps provider keys on the host with `xproxy`, and proceeds through discovery, motions, trial, verdict, and judgment.  The attorney container receives only a fresh writable home directory for the turn.  It does not receive the repository checkout, the run output directory, or a persistent host `~/.pi` tree.

From the repository root:

```bash
make demo
```

`make demo` does three things before the case run starts:

1. Runs `examples/ex1/sign.sh`.
2. Runs `.bin/adc complain --situation examples/ex1/situation.md`.
3. Runs `.bin/adc case --complaint examples/ex1/complaint.md --out-dir out/ex1-demo ...`.

If you want the manual case command after complaint drafting, use:

```bash
.bin/adc case \
  --complaint examples/ex1/complaint.md \
  --out-dir out/ex1-demo \
  --acp-role plaintiff \
  --acp-role defendant \
  --acp-command "$PWD/pi-container/acp-podman.sh"
```

## Results

The `--out-dir` directory contains the complete record of the run:

| File                      | Meaning                               |
|---------------------------|---------------------------------------|
| `complaint.md`            | Staged complaint text                 |
| `normalized-case.json`    | Structured intake packet              |
| `plaintiff-strategy.md`   | Plaintiff private litigation plan     |
| `defense-strategy.md`     | Defense private litigation plan       |
| `generated-scenario.json` | Seeded case bundle used by the runner |
| `events.ndjson`           | Event log                             |
| `run.db`                  | SQLite run database                   |
| `run.json`                | Full authoritative run artifact       |
| `digest.md`               | Case digest                           |
| `transcript.md`           | Trial transcript                      |

For the default demo, those files are under `out/ex1-demo/`.

Open the three files you will usually care about first:

```bash
sed -n '1,220p' out/ex1-demo/run.json
sed -n '1,220p' out/ex1-demo/digest.md
sed -n '1,220p' out/ex1-demo/transcript.md
```

`digest.md` shows the case at a high level.  `transcript.md` shows the courtroom sequence.  `run.json` is the authoritative machine-readable result.

## License

The software is released under the MIT License in `LICENSE`.  Trademark and related notice terms are in `NOTICES.md`.

## Repository layout

| Path                 | Purpose                                               |
|----------------------|-------------------------------------------------------|
| `engine/`            | Lean rule engine, proofs, and Lake project            |
| `.bin/`              | Local `adc` and `adcengine` binaries                  |
| `runtime/`           | Go CLI, runtime, and embedded `xproxy` package        |
| `etc/`               | Checked-in config files and juror pool                |
| `examples/ex1/`      | Main ACP-heavy example input set                      |
| `tools/`             | Local scripts for diagrams, proofs, models, and plots |
| `../common/pi-container/` | Shared Podman wrapper and image build path for upstream `pi` |
| `../common/submodules/pi-acp/` | Pinned ACP bridge submodule                     |

## Tools

The `tools/` directory contains a small set of local scripts used during development and analysis:

| Tool | Purpose |
|---|---|
| `gendiagram.sh` | Render a Mermaid `.mmd` file to PNG with `mmdc` and a local Chromium binary |
| `gentheorems.py` | Sort `theorems.tsv` and regenerate `theorems.md` from it |
| `llm_graph.py` | Read `llm_csv` lines from an `xproxy` log and render a latency scatter plot |
| `model-speed.sh` | Probe model latency and tool-call support through `adc llm --tool-check` |
| `cluster-personas.py` | Sample model and persona behavior over a gene set and emit clustering data |
| `proofstats.sh` | Summarize Lean proof files in `engine/Proofs` into `docs/proofstats.md` |

## Notes on `pi-acp`

The submodule at `../common/submodules/pi-acp` pins the fork and commit that expose `_adc/*` ACP methods to `pi`.  The Podman image build copies that source tree into the image and installs `pi-acp` there, so the documented demo path no longer depends on host `node` or host `npm`.  The submodule records the branch name `generic-ext-method-tools` in the repository root `.gitmodules` file so `git submodule update --remote` knows which branch to follow.  Git submodules still pin a commit.  They do not automatically advance to new upstream commits.
