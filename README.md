# Agent-driven adjudication

This project is an experimental AI civil litigation system that uses
agent attorneys with either agent or human clients.

The implementation:

1. Uses a core procedural engine implemented in
   [Lean](https://lean-lang.org/) with [many
   theorems](adc/docs/proofstats.md) about its behavior.

1. Supports verifiable execution in [attestable instances](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/nitrotpm-attestation.html) and trusted
   execution environments (TEEs), which also provide confidentiality.
   
1. Roughly follows the [United States Federal Rules for Civil
   Procedure](https://www.uscourts.gov/rules-policies/current-rules-practice-procedure/federal-rules-civil-procedure)
   (FRCP).  Our version of the rules is called the [Agent Rules
   for Civil Procedure](adc/docs/ARCP.md) (ARCP).

1. Interacts with agent attorneys via an implementation of the [Agent
   Client
   Protocol](https://agentclientprotocol.com/get-started/introduction)
   extended to support external tool calls for litigation.  Currently the agents are implemented by [`pi`](https://github.com/badlogic/pi-mono/tree/main). This
   approach facilitates arbitrary computer use by attorney-agent teams.
   
1. Provides somewhat sophisticated [sampling for candidate pools of AI
   jurors](adc/docs/juries.md).  (Attorneys still have access to *voir dire*
   under [ARCP](adc/docs/ARCP.md) [Rule
   47](adc/docs/ARCP.md#rule-47-selecting-jurors).)

We are starting to use this system as a simulator to use in developing
agent attorneys and judges.


## Overview

This repo contains two adjudication systems in one repository and one Go module.  [Agent District Court](adc/) models U.S. Federal District Court procedure, including pleadings, discovery, motions, jury selection, trial, verdict, and reporting.  [Agent Arbitration](arb/) models arbitration before a council, with a smaller procedural surface and a shorter path to decision.

Lean defines the procedural engines and the proof surface for both systems.  Go builds the command-line tools, storage layer, prompt assembly, report generation, provider clients, and ACP integration.  The repository root exists to keep those shared parts together; the day-to-day build and run entrypoints remain in `adc/` and `arb/`.

## Layout

| Path | Purpose |
|---|---|
| `adc/` | District-court system, including the Lean engine, Go runtime, examples, and reports |
| `arb/` | Arbitration system, including the Lean engine, Go runtime, examples, and prompts |
| `common/` | Shared Go packages, provider and ACP integration, personas, `xproxy`, the PI container build path, and repository tools |
| `go.mod` and `go.sum` | Root Go module for shared packages and both runtimes |

The two systems share infrastructure but remain separate applications.  `adc/` builds `adc` and `adcengine`.  `arb/` builds `aar` and `aarengine`.  Shared code lives under `common/`, not in a sibling checkout outside the repository.

## Requirements

This repository builds with Go `1.25` and Lean `4.27.0` with `lake`.  `make` drives the project-specific targets in `adc/` and `arb/`.  The Python tools in `common/tools/` are `uv` scripts and should run that way.

The shared persona and model curation corpus lives under `common/data/personas/`.  The shared tools resolve their default paths against the current working directory.  Run them from the repository root unless you pass explicit paths.

Live runs require Podman, network access to the configured model providers, and the corresponding API keys.  The checked-in district-court demo uses ACP attorneys through `xproxy`, so `OPENAI_API_KEY` is required and some model pools also require `OPENROUTER_API_KEY`.  Arbitration examples use the same shared provider and ACP path where the selected models require it.

## Build And Run

The repository root has no top-level `Makefile`.  Build, test, and proof commands run from `adc/` or `arb/`, depending on which system you are working on.  The normal entrypoints are these:

```bash
cd adc && make build && make test && make prove
cd arb && make build && make test && make prove
```

The main live example for `adc/` is `make demo`, which signs the example materials, drafts the complaint, and runs the full case in `adc/out/ex1-demo/`.  The main live examples for `arb/` are `make demo`, `make ex2`, and `make ex3`, each of which drafts a complaint and writes a complete run packet under `arb/out/`.  Both systems produce a digest, a transcript, an event log, and a machine-readable run record for each completed run.

Use the system-specific READMEs for the procedural details and the full command surface.  The district-court documentation is in [Agent District Court](adc/README.md).  The arbitration documentation is in [Agent Arbitration](arb/README.md).
