# Common Tools

This directory contains shared maintenance tools for the `arb` and `arbd` trees.  Lawyer integrations do not live here.  AAR exposes lawyer turns through the HTTP Lawyer API, and front ends such as CLIs, MCP servers, ACP services, or agent runners can be built on top of that API without changing the arbitration runtime.

## Files

| File | Role |
| --- | --- |
| `cluster-personas.py` | Cluster persona text for pool review. |
| `clusters-graph.py` | Render persona-cluster graphs. |
| `filter-models.py` | Filter model lists for pool construction. |
| `gendiagram.sh` | Generate diagrams from source files. |
| `generate-council.py` | Build council persona data. |
| `gentheorems.py` | Generate theorem scaffolding. |
| `llm_graph.py` | Render model and request graphs. |
| `model-speed.sh` | Measure model request timing. |
| `proofstats.sh` | Summarize proof statistics. |
| `select-council.py` | Select council members from a persona pool. |
