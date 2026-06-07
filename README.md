# Agent-driven adjudication

This repository contains three agent adjudication systems built on one Go module and separate Lean engines, plus eval tools for juror and council model-pool work.  Each adjudication system owns its own command-line tool, rules, examples, prompts, manuals, and proofs.  The shared code under `common/` contains provider clients, model-pool tooling, shared personas, and container support used by more than one system.

The current live-agent path uses case-owned HTTP APIs and MCP adapters.  OpenClaw lawyers connect through MCP or direct HTTP, depending on the tool that drives them.  Pi jurors and council members connect through MCP and receive model configuration from JSONL request-spec pools, including provider, model, quantization, request parameters, and persona.

## Systems

| Path | System | Command | Description |
| --- | --- | --- | --- |
| `adc/` | Agent District Court | `adc` | Civil litigation procedure with pleadings, motions, discovery, trial, jury deliberation, verdict, and judgment. |
| `arb/` | Agent Arbitration | `aar` | Arbitration over one proposition, with plaintiff and defendant lawyers and a council vote on demonstrated or not demonstrated. |
| `arbd/` | Agent Arbitration Degree | `aard` | Degree arbitration over one question, with plaintiff and defendant lawyers and council answers from `0` through `100`. |
| `evals/` | Eval tools | `uv run tools/...` | Core and deliberation eval sets, endpoint-variant inventory, scoring, clustering, and pool sampling tools. |

Each system has a short `README.md`, a full `manual.md`, and a practice guide under `docs/practice.md`.  Manuals cover commands, HTTP APIs, MCP, services, local-agent runs, remote OpenClaw use, outputs, and troubleshooting.  Practice guides cover how lawyers, jurors, or council members should build the record, examine evidence, and argue or deliberate within the relevant procedure.

## Requirements

This repository builds with Go `1.25`, Lean `4.27.0`, and `lake`.  `make` drives the local build, test, and proof targets in each system directory.  Docker runs OpenClaw lawyer containers; Podman runs Pi juror and council containers.

Live runs require model-provider credentials.  OpenClaw lawyers can use a Codex `auth.json` file or `OPENAI_API_KEY`.  Pi jurors and council members require the provider credentials named by the selected pool records, with current local pools using OpenRouter through `OPENROUTER_API_KEY`.

## Build

Build from the system directory you are working on:

```bash
cd adc && make build && make test && make prove
cd arb && make build && make test && make prove
cd arbd && make build && make test && make prove
```

The repository root has no top-level `Makefile`.  Shared packages build through the system commands because the three runtimes use the same Go module.  The Pi container image can be built from `common/pi-container/` when a live run needs the local Pi image.

## Documentation

Start with the system manual for the procedure you intend to run.  [Agent District Court](adc/manual.md) covers `adc case`, `adc run`, the Role API, MCP, juror pools, and the Clerk service.  [Agent Arbitration](arb/manual.md) covers `aar case`, `aar run`, Lawyer and Council APIs, MCP, OpenClaw auth, remote OpenClaw lawyers, and the Clerk service.  [Agent Arbitration Degree](arbd/manual.md) covers the same operating surface for degree questions and council answer maps.

The governing rule documents live in each system's `docs/` directory.  ADC uses [ARCP](adc/docs/ARCP.md).  AAR and AARD use their respective [AAR rules](arb/docs/ARAP.md) and [AARD rules](arbd/docs/ARAP.md).  Shared model-pool documentation lives in [jury and council pool generation](common/docs/jury-pool-generation.md).  The eval tools are documented in [the eval README](evals/README.md).
