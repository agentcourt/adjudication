# Agent Arbitration Degree

Agent Arbitration Degree, or AARD, decides one degree question through an adversarial record and a council answer map.  A complaint states the question, plaintiff and defendant lawyers build and argue the record, and each council member submits one integer answer from `0` through `100` with a rationale.  The runtime stores filings, admitted evidence, work notes, council answers, transcripts, event logs, and final output in one run packet.

## Manual

Read [Agent Degree Arbitration Manual](manual.md) for the command surface and operating details.  It covers `aard case`, `aard run`, `aard service`, `aard mcp`, the Lawyer API, the Council API, Clerk routes, OpenClaw auth, remote OpenClaw lawyers, Pi council agents, output files, failure behavior, and troubleshooting.  The manual is the active operator reference for AARD.

Read [Practice Manual](docs/practice.md) for lawyer and council practice.  It explains degree-question framing, evidence search, source preservation, technical reports, work notes, score advocacy, and council answer rationales.  The governing rules are [Agent Rules for Arbitration Degree Procedure](docs/ARAP.md).

## Requirements

| Requirement | Purpose |
| --- | --- |
| Go `1.25` | Builds the AARD runtime. |
| Lean `4.27.0` and `lake` | Builds the Lean engine and proof tree. |
| Docker | Runs OpenClaw lawyer containers in `aard run`. |
| Podman | Runs Pi council containers in `aard run`. |
| Codex `auth.json` or `OPENAI_API_KEY` | Authenticates OpenClaw lawyers. |
| `OPENROUTER_API_KEY` | Authenticates current local Pi council pool entries that use OpenRouter. |

## Build

Build from `arbd/`:

```bash
make build
make test
make prove
```

`make build` writes `.bin/aard` and `.bin/aardengine`.  `make test` runs the Go tests for the runtime.  `make prove` builds the Lean proof tree.

## First Run

Run an example with OpenClaw lawyers using Codex auth and Pi council agents sampled from `pool.jsonl`:

```bash
export OPENROUTER_API_KEY=REPLACE_WITH_KEY

.bin/aard run \
  --openclaw-auth codex \
  --openclaw-codex-auth PATH/TO/auth.json \
  --council-pool "$(pwd)/pool.jsonl" \
  ex1
```

Start the Clerk service when cases should be created and managed through HTTP:

```bash
.bin/aard service \
  --listen 127.0.0.1:19771 \
  --out-root out/service \
  --aard-bin .bin/aard
```

## Layout

| Path | Purpose |
| --- | --- |
| `manual.md` | Full operating manual. |
| `docs/` | Rules, practice guide, evidence handling, policy notes, and council references. |
| `engine/` | Lean degree-arbitration engine and proofs. |
| `runtime/` | Go CLI, case runtime, HTTP APIs, MCP adapter, local run code, and service. |
| `agent-instructions/` | Templates for OpenClaw lawyers, remote OpenClaw lawyers, and Pi council agents. |
| `examples/` | Example complaints and case packets. |
| `prompts/` | Prompt templates used by the case runtime. |
| `pool.jsonl` | Local council request-spec pool when present. |
