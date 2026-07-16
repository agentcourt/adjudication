# Agent-driven adjudication

This repository contains three agent adjudication systems and eval tools for juror and council model-pool work.  The systems share one Go module and use separate Lean engines.  `common/` contains provider clients, request-spec types, shared persona data, Pi container support, and tools used by more than one system.

Live-agent runs use case-owned HTTP APIs and MCP adapters.  OpenClaw lawyers connect through MCP or direct HTTP, depending on the runtime.  Pi jurors and council members connect through MCP and receive model configuration from JSONL request-spec pools.

## Systems

| Path | Command | Manual | Purpose |
| --- | --- | --- | --- |
| [adc/](adc/README.md) | `adc` | [Agent District Court Manual](adc/manual.md) | Civil litigation procedure with pleadings, motions, discovery, trial, jury deliberation, verdict, and judgment. |
| [arb/](arb/README.md) | `aar` | [Agent Arbitration Manual](arb/manual.md) | Arbitration over one proposition, with plaintiff and defendant lawyers and a council vote on demonstrated or not demonstrated. |
| [arbd/](arbd/README.md) | `aard` | [Agent Arbitration Degree Manual](arbd/manual.md) | Degree arbitration over one question, with plaintiff and defendant lawyers and council answers from `0` through `100`. |
| [evals/model-pool/](evals/model-pool/README.md) | `cd evals/model-pool && uv run tools/COMMAND.py` | [Model-Pool Evals Manual](evals/model-pool/manual.md) | Core and deliberation eval sets, endpoint-variant inventory, scoring, clustering, and pool sampling tools. |

The manuals document commands, services, HTTP APIs, MCP adapters, attested execution, outputs, and troubleshooting.  The practice guides describe how lawyers, jurors, and council members examine evidence, create the record, and deliberate within each procedure.

## Shared Directories

| Path | Purpose |
| --- | --- |
| `common/` | Shared Go packages, model-request types, persona data, Pi container support, and common tools. |
| [scratch/](scratch/README.md) | Archived notes, old drafts, run observations, and investigation records. |
| `skills/` | Local analysis notes for proof review. |

## Requirements

| Requirement | Purpose |
| --- | --- |
| Go `1.25` | Builds the Go runtimes. |
| Lean `4.27.0` and `lake` | Build the Lean engines and proof trees. |
| `make` | Runs build, test, proof, and example targets in each system directory. |
| Docker | Runs OpenClaw lawyer containers and builds attested workload images. |
| Podman | Runs Pi juror and council containers for local-agent runs. |
| Model-provider credentials | OpenClaw lawyers use Codex `auth.json` or `OPENAI_API_KEY`; current Pi pools use OpenRouter through `OPENROUTER_API_KEY`. |

## Build

Build one or more systems from the repository root:

```bash
make -C adc build test prove
make -C arb build test prove
make -C arbd build test prove
```

The repository root has no top-level `Makefile`.  Shared packages build through the system commands because the runtimes use the same Go module.  Build the Pi container image from `common/pi-container/` when a local-agent run needs the local Pi image.

## Documentation

| Area | Primary documents |
| --- | --- |
| ADC | [README](adc/README.md), [manual](adc/manual.md), [practice guide](adc/docs/practice.md), [rules](adc/docs/ARCP.md), [attested runbook](adc/Dockerfile.md), [dev-host requirements](adc/docs/attested-dev-host.md). |
| AAR | [README](arb/README.md), [manual](arb/manual.md), [council and juror replay guide](arb/docs/council-replay.md), [practice guide](arb/docs/practice.md), [rules](arb/docs/ARAP.md), [attested runbook](arb/Dockerfile.md), [dev-host requirements](arb/docs/attested-dev-host.md). |
| AARD | [README](arbd/README.md), [manual](arbd/manual.md), [practice guide](arbd/docs/practice.md), [rules](arbd/docs/ARAP.md), [attested runbook](arbd/Dockerfile.md), [dev-host requirements](arbd/docs/attested-dev-host.md). |
| Evals | [README](evals/README.md), [model-pool manual](evals/model-pool/manual.md), [sampling runbook](evals/model-pool/docs/sampling-runbook.md), [model inventory notes](evals/model-pool/docs/model-inventory.md), [judge eval plan](evals/adc/judge/plan.md). |
| Shared model pools | [Jury and council pool generation](evals/model-pool/docs/jury-pool-generation.md). |

## License

The software is released under the MIT License in [LICENSE](LICENSE).  Trademark and related notice terms are in [NOTICES.md](NOTICES.md).
