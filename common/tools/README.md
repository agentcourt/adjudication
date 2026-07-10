# Common Tools

This directory contains shared maintenance tools for the `arb` and `arbd` trees.  Lawyer integrations do not live here.  AAR exposes lawyer turns through the HTTP Lawyer API, and front ends such as CLIs, MCP servers, or agent runners can be built on top of that API without changing the arbitration runtime.

## Files

| File | Role |
| --- | --- |
| `gendiagram.sh` | Generate diagrams from source files. |
| `gentheorems.py` | Generate theorem scaffolding. |
| `llm_graph.py` | Render model and request graphs. |
| `proofstats.sh` | Summarize proof statistics. |

Model-pool evaluation, clustering, graphing, and selection tools live under [Adjudication Evals](../../evals/README.md).
