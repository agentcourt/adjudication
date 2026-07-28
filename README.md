# Agent-driven adjudication

This repo provides agent-driven adjudication that uses agents in legal procedures controlled by explicit rules.  In these systems, lawyers, jurors, judges (if applicable) and council members operate together to perform an adjudication in some form.  A rigorous engine, implemented in Lean, controls phases, opportunities, accepted actions, and final outcomes.  Each run produces a record that can be evaluated and, in some modes, verified via an attestation.

## Approach

Each system separates procedure from advocacy and fact evaluation.  The procedural engine (Lean) controls the current phase, required opportunities, accepted actions, and ending conditions.  The (Go) runtime exposes the engine through commands, HTTP APIs, MCP adapters, services, and artifacts.  Lawyers, jurors, and council members create filings, votes, and answers that the engine accepts or rejects.

The systems use different procedures for different goals.  [Agent District Court](adc/README.md) models civil litigation with pleadings, motions, discovery, voir dire, trial, jury deliberation, verdict, and judgment.  [Arbitration](arb/README.md) models binary arbitration over whether a proposition has been demonstrated.  [Arbitration of Degree](arbd/README.md) models degree arbitration over a numerical answer.

## Core Capabilities

| Capability | Description |
| --- | --- |
| Lean engines | The three systems use separate Lean engines for phases, opportunities, accepted actions, and final states. |
| Engine proofs | The proof work checks engine behavior and certificate replay.  Lawyer search quality and juror reasoning are judged from records and evals, not Lean proofs. |
| Run records | Completed runs write state, event, transcript, and certificate artifacts for later inspection and replay verification. |
| Pool sampling | The model-pool pipeline compares candidate provider endpoints, groups them by behavior, and samples juror or council panels for live runs. |
| External lawyers | Lawyer agents can participate through assigned role APIs and MCP adapters. |
| OpenClaw support | A run can start OpenClaw lawyers for one or both sides as one way to run lawyer agents.  For example, `adc run --auto-lawyers defendant` starts the defendant lawyer locally and writes plaintiff instructions for an independently running OpenClaw session. |
| Evals | Behavior evals put a system actor in a controlled state and score the decision.  The Agent District Court judge suites cover voir dire question rulings, summary judgment, dismissal, jury instructions, sanctions, bench opinions, judgment entry, and post-judgment relief, and they test candidate prompts before use in live runs. |
| Attested execution | Attested runs package case inputs, run the procedure on an attested host, and link uploaded artifacts to attestation records and manifest hashes. |
| Service operation | Long-lived services can create, track, inspect, and stop case runs through service APIs. |

## Example Usage

Run a direct [Arbitration](arb/README.md) case when you want one local arbitration without the service API.  This builds the `aar` command, creates a small complaint, and starts a complete local proceeding.  With the default OpenClaw settings, the plaintiff and defendant litigators are OpenClaw agents running in Docker containers, using OpenAI `gpt-5.5` with `low` thinking.  The council agents come from a sampled Pi council pool and decide whether the proposition in the complaint has been demonstrated.

```bash
cd arb
make build

mkdir -p work/example-arbitration
cat > work/example-arbitration/complaint.md <<'EOF'
# Proposition

During May 2026 (ET), Iran initiated a major non-weather closure of its airspace.
EOF

export OPENROUTER_API_KEY=REPLACE_WITH_OPENROUTER_KEY

.bin/aar run \
  --complaint work/example-arbitration/complaint.md \
  --openclaw-auth codex \
  --openclaw-codex-auth "$HOME/.codex/auth.json" \
  --council-pool "$(pwd)/pool.jsonl"
```

Use `--openclaw-auth api-key` with `OPENAI_API_KEY` for API-key lawyer auth instead of Codex auth.  The completed run writes the transcript, event log, state, certificate, and summary files under its output directory.

## Systems

| Path | Command | Manual | Purpose |
| --- | --- | --- | --- |
| [adc/](adc/README.md) | `adc` | [Agent District Court Manual](adc/manual.md) | Civil litigation procedure with pleadings, motions, discovery, trial, jury deliberation, verdict, and judgment. |
| [arb/](arb/README.md) | `aar` | [Agent Arbitration Manual](arb/manual.md) | Arbitration over one proposition, with plaintiff and defendant lawyers and a council vote on demonstrated or not demonstrated. |
| [arbd/](arbd/README.md) | `aard` | [Agent Arbitration Degree Manual](arbd/manual.md) | Degree arbitration over one question, with plaintiff and defendant lawyers and council answers from `0` through `100`. |
| [vmcp/](vmcp/README.md) | `vmcp` | [VMCP Design](docs/vmcp.md) | A persistent MCP server written in Lean that holds a simplified arbitration, admits each tool call by rule, and advertises to each connection only the tools its role currently holds. |

The manuals document commands, services, HTTP APIs, MCP adapters, attested execution, outputs, and troubleshooting.  The practice guides describe how lawyers, jurors, and council members examine evidence, create the record, and deliberate within each procedure.  The three Go systems share one module and one runtime shape; `vmcp/` is standalone and imports nothing from the rest of the repository.

## Supporting Directories

| Path | Purpose |
| --- | --- |
| `common/` | Shared Go packages, model-request types, persona data, Pi container support, and common tools. |
| [docs/](docs/README.md) | Cross-system proof and repository notes. |
| [evals/](evals/README.md) | Behavior evals for the adjudication actors, run through the system commands. |
| [model-pool/](model-pool/README.md) | Model and provider-endpoint evaluation and the sampling pipeline that builds juror and council pools.  Run with `cd model-pool && uv run tools/COMMAND.py`. |
| `skills/` | Local analysis notes for proof review. |
| [web/](web/README.md) | Service console, run report, and ARB management web servers. |

## Requirements

| Requirement | Purpose |
| --- | --- |
| Go `1.25` | Builds the Go runtimes. |
| Lean `4.27.0` and `lake` | Build the Lean engines and proof trees. |
| `make` | Runs build, test, proof, and example targets in each system directory. |
| `uv` | Runs the model-pool Python tools, which carry inline script metadata. |
| Docker | Runs the included OpenClaw lawyer containers and builds attested workload images. |
| Podman | Runs Pi juror and council containers for local-agent runs. |
| Model-provider credentials | The included OpenClaw support uses Codex `auth.json` or `OPENAI_API_KEY`.  Current Pi pools use OpenRouter through `OPENROUTER_API_KEY`. |

## Build

Build one or more systems from the repository root:

```bash
make -C adc build test prove
make -C arb build test prove
make -C arbd build test prove
```

The repository root has no top-level `Makefile`.  Shared packages build through the system commands because the runtimes use the same Go module.  Build the Pi container image from `common/pi-container/` when a local-agent run needs the local Pi image.

`vmcp/` has no `Makefile` and builds with `lake build` from its own directory, since it is a standalone Lean package rather than part of the Go module.

## Documentation

| Area | Primary documents |
| --- | --- |
| Agent District Court | [README](adc/README.md), [manual](adc/manual.md), [practice guide](adc/docs/practice.md), [rules](adc/docs/ARCP.md), [attested runbook](adc/Dockerfile.md), [dev-host requirements](adc/docs/attested-dev-host.md). |
| Arbitration | [README](arb/README.md), [manual](arb/manual.md), [council and juror replay guide](arb/docs/council-replay.md), [practice guide](arb/docs/practice.md), [rules](arb/docs/ARAP.md), [attested runbook](arb/Dockerfile.md), [dev-host requirements](arb/docs/attested-dev-host.md). |
| Arbitration of Degree | [README](arbd/README.md), [manual](arbd/manual.md), [practice guide](arbd/docs/practice.md), [rules](arbd/docs/ARAP.md), [attested runbook](arbd/Dockerfile.md), [dev-host requirements](arbd/docs/attested-dev-host.md). |
| Evals | [README](evals/README.md), [judge evals](evals/adc/judge/README.md), [judge eval plan](evals/adc/judge/plan.md). |
| Model pools | [README](model-pool/README.md), [manual](model-pool/manual.md), [sampling runbook](model-pool/docs/sampling-runbook.md), [model inventory notes](model-pool/docs/model-inventory.md), [legacy CSV pipeline](model-pool/docs/jury-pool-generation.md). |
| Proofs | [Proof work status](docs/proof-notes.md), [VMCP design](docs/vmcp.md). |
| Web | [Web servers overview](web/README.md), [web runbook](web/runbook.md). |

## License

The software is released under the MIT License in [LICENSE](LICENSE).  Trademark and related notice terms are in [NOTICES.md](NOTICES.md).
