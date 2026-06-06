# Agent District Court

Agent District Court, or ADC, is an experimental civil-litigation runtime for AI legal agents.  The Go runtime manages intake, prompts, storage, role APIs, reports, and local agent processes.  The Lean engine enforces procedure and state transitions under the Agent Rules for Civil Procedure.

ADC starts from either a situation file, a complaint, or a scenario JSON file.  A situation file can be turned into a complaint with `adc complain`.  A complaint can be turned into a one-claim case packet and then run through pleadings, motions, discovery, trial, verdict, and judgment.

The current external-agent path uses a case-owned HTTP Role API and a Streamable HTTP MCP adapter.  OpenClaw lawyers connect through MCP.  Pi jurors connect through MCP when `adc run` starts a fresh juror agent for an active juror opportunity from a JSONL request-spec pool.  If a deliberating juror agent fails, ADC removes that juror from the effective concurrence count and derives any verdict from the eligible jurors who remain.

Jury size and verdict threshold are case-policy settings.  `adc case`, `adc scenario`, and `adc run` accept `--juror-count`, `--unanimous-required`, and `--minimum-concurring`; the clerk create API accepts `juror_count`, `unanimous_required`, and `minimum_concurring`.  When those values are omitted, ADC uses the scenario policy or the default six-person unanimous jury.

## Manual

Read [`manual.md`](manual.md) first.  It describes the command set, environment, Role API, MCP adapter, local OpenClaw and Pi operation, remote OpenClaw operation, clerk service, output files, and troubleshooting.  The manual is the current operating reference for ADC.

## Requirements

| Requirement | Purpose |
| --- | --- |
| Go `1.25` | Builds the ADC runtime. |
| Lean `4.27.0` and `lake` | Builds the Lean engine and proof tree. |
| `make` | Runs build, test, proof, and example targets. |
| Docker | Runs OpenClaw lawyer containers in `adc run`. |
| Podman | Runs Pi juror containers in `adc run`. |
| `OPENROUTER_API_KEY` | Required for Pi jurors selected from a request-spec pool. |
| Codex `auth.json` or `OPENAI_API_KEY` | Required for OpenClaw lawyers.  Codex auth is the usual path. |

## Build

Build both local binaries from `adc/`:

```bash
make build
```

That writes `.bin/adc` and `.bin/adcengine`.  Run the Go tests with:

```bash
make test
```

Build the Lean proof tree with:

```bash
make prove
```

## Basic Runs

Draft a complaint from example 1:

```bash
.bin/adc complain \
  --situation examples/ex1/situation.md \
  --out examples/ex1/complaint.md
```

Run the complaint with direct internal roles:

```bash
.bin/adc case \
  --complaint examples/ex1/complaint.md \
  --out-dir out/ex1-direct
```

Run the complaint with local OpenClaw lawyers and Pi jurors:

```bash
export OPENROUTER_API_KEY=REPLACE_WITH_KEY
.bin/adc run \
  --complaint examples/ex1/complaint.md \
  --out-dir out/ex1-openclaw-pi \
  --openclaw-auth codex \
  --openclaw-codex-auth PATH/TO/auth.json
```

Run the clerk service:

```bash
.bin/adc service \
  --listen 127.0.0.1:19870 \
  --output-root out/adc-service \
  --adc-bin .bin/adc \
  --engine .bin/adcengine
```

Create a full local-agent case through the clerk service:

```bash
curl -sS -X POST http://127.0.0.1:19870/clerk/v1/cases \
  -H 'content-type: application/json' \
  --data '{
    "mode": "run",
    "case_id": "adc-ex1",
    "complaint_path": "examples/ex1/complaint.md",
    "out_dir": "out/adc-service/adc-ex1",
    "openclaw_auth": "codex",
    "openclaw_codex_auth_path": "PATH/TO/auth.json",
    "juror_personas": "../common/data/personas/pool.jsonl"
  }'
```

## Repository Layout

| Path | Purpose |
| --- | --- |
| `engine/` | Lean rule engine, proofs, and Lake project. |
| `runtime/` | Go CLI, runtime, Role API, MCP adapter, local run code, and clerk service. |
| `agent-instructions/` | Templates passed to OpenClaw lawyers and Pi jurors. |
| `etc/` | Court profile files. |
| `examples/` | Example case source documents. |
| `docs/` | Rule documents and supporting technical notes. |
| `analysis/` | Mermaid diagrams and explanatory notes. |
| `manual.md` | Current operating manual. |

## Output

Run output normally contains `run.json`, `runtime.json`, `events.ndjson`, `run.db`, `transcript.md`, `digest.md`, and `work-notes.ndjson`.  Complaint-driven runs also write `normalized-case.json`, `plaintiff-strategy.md`, `defense-strategy.md`, and `generated-scenario.json`.  `adc run` adds process logs and local-agent metadata under the selected output directory.

## License

The software is released under the MIT License in `LICENSE`.  Trademark and related notice terms are in `NOTICES.md`.
