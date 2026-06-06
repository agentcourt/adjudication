# Agent Arbitration

Agent Arbitration, or AAR, decides one proposition through an adversarial record and a council vote.  A complaint states the proposition, plaintiff and defendant lawyers build the record, and council members vote `demonstrated` or `not_demonstrated` under the configured evidence standard.  The runtime stores filings, admitted evidence, work notes, council votes, transcripts, event logs, and final output in one run packet.

## Manual

Read [Agent Arbitration Manual](manual.md) for the command surface and operating details.  It covers `aar case`, `aar run`, `aar service`, `aar mcp`, the Lawyer API, the Council API, Clerk routes, OpenClaw auth, remote OpenClaw lawyers, Pi council agents, output files, failure behavior, and troubleshooting.  The manual is the active operator reference for AAR.

Read [Practice Manual](docs/practice.md) for lawyer and council practice.  It explains phase work, evidence search, source preservation, technical reports, work notes, and council deliberation.  The governing rules are [Agent Rules for Arbitration Procedure](docs/ARAP.md).

## Requirements

| Requirement | Purpose |
| --- | --- |
| Go `1.25` | Builds the AAR runtime. |
| Lean `4.27.0` and `lake` | Builds the Lean engine and proof tree. |
| Docker | Runs OpenClaw lawyer containers in `aar run`. |
| Podman | Runs Pi council containers in `aar run`. |
| Codex `auth.json` or `OPENAI_API_KEY` | Authenticates OpenClaw lawyers. |
| `OPENROUTER_API_KEY` | Authenticates current local Pi council pool entries that use OpenRouter. |

## Build

Build from `arb/`:

```bash
make build
make test
make prove
```

`make build` writes `.bin/aar` and `.bin/aarengine`.  `make test` runs the Go tests for the runtime.  `make prove` builds the Lean proof tree.

## First Run

Run an example with OpenClaw lawyers using Codex auth and Pi council agents sampled from `pool.jsonl`:

```bash
export OPENROUTER_API_KEY=REPLACE_WITH_KEY

.bin/aar run \
  --openclaw-auth codex \
  --openclaw-codex-auth PATH/TO/auth.json \
  --council-pool "$(pwd)/pool.jsonl" \
  ex01
```

Start the Clerk service when cases should be created and managed through HTTP:

```bash
.bin/aar service \
  --listen 127.0.0.1:19770 \
  --out-root out/service \
  --aar-bin .bin/aar
```

## Layout

| Path | Purpose |
| --- | --- |
| `manual.md` | Full operating manual. |
| `docs/` | Rules, practice guide, API/process specs, evidence handling, policy notes, and proof references. |
| `engine/` | Lean arbitration engine and proofs. |
| `runtime/` | Go CLI, case runtime, HTTP APIs, MCP adapter, local run code, and service. |
| `agent-instructions/` | Templates for OpenClaw lawyers, remote OpenClaw lawyers, and Pi council agents. |
| `examples/` | Example complaints and case packets. |
| `prompts/` | Prompt templates used by the case runtime. |
| `pool.jsonl` | Local council request-spec pool when present. |
