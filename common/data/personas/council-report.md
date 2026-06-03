# Council Selection Report

## Inputs

- Clusters: `common/data/personas/clusters.csv`
- PCA: `common/data/personas/pca-cluster.csv`
- Metadata: `common/data/personas/openrouter-models.json`
- Latency CSV: `common/data/personas/model-latency.csv`
- Failures CSV: `common/data/personas/model-operational-failures.csv`
- Signature source: PCA mean vectors

## Eligibility

- Candidate model/persona pairs: 490
- Eligible candidate pairs: 190
- Selected rows: 20
- Expected coverage: 3 genes x 3 samples
- Minimum context: 200000
- Required parameters: tools,tool_choice
- Maximum elapsed ms: 8000

## Exclusions

- context_below_min: 199
- coverage_gene_count: 114
- coverage_row_count: 123
- coverage_samples_per_gene: 14
- operational_failure:context_limit: 8
- operational_failure:council_vote_tool_noncompliance: 16
- operational_failure:pi_mcp_tool_noncompliance: 8
- required_parameters_missing:tool_choice: 8

## Selected Provider Counts

- anthropic: 2
- arcee-ai: 2
- bytedance-seed: 2
- google: 2
- inclusionai: 2
- kwaipilot: 2
- minimax: 2
- mistralai: 2
- openai: 2
- qwen: 2

## Context Stats

- Selected effective context: min=256000 median=262144 max=1048576

## Selected Rows

| # | Model | Persona | Provider | Top context | Provider context | Effective context | Provider max completion | Latency ms | Signature |
|---:|---|---|---|---:|---:|---:|---:|---:|---|
| 1 | openrouter://inclusionai/ling-2.6-flash | personas/persons/d715074-5.txt | inclusionai | 262144 | 262144 | 262144 | 32768 | 1235 | dim=9 norm=0.897 [-0.06232, -0.6165, -0.4251, 0.01506, -0.3467, -0.1703, ...] |
| 2 | openrouter://qwen/qwen3-max-thinking | personas/persons/c4e1a2b-0.txt | qwen | 262144 | 262144 | 262144 | 32768 | 3743 | dim=9 norm=0.7579 [0.09673, 0.3261, -0.2296, 0.3005, 0.4006, 0.09151, ...] |
| 3 | openrouter://openai/gpt-5.1-codex-max | personas/persons/d715074-1.txt | openai | 400000 | 400000 | 400000 | 128000 | 2518 | dim=9 norm=0.7791 [-0.4868, -0.1407, 0.3271, -0.428, -0.1183, 0.01383, ...] |
| 4 | openrouter://mistralai/mistral-large-2512 | personas/persons/e91db77-0.txt | mistralai | 262144 | 262144 | 262144 |  | 1971 | dim=9 norm=0.7791 [-0.05175, 0.2396, -0.2309, 0.07642, -0.3523, -0.373, ...] |
| 5 | openrouter://google/gemini-2.0-flash-001 | personas/persons/d715074-1.txt | google | 1048576 | 1048576 | 1048576 | 8192 | 1113 | dim=9 norm=0.5296 [0.3193, -0.03433, 0.2174, -0.1733, 0.1719, -0.1884, ...] |
| 6 | openrouter://anthropic/claude-opus-4.7-fast | personas/persons/d715074-3.txt | anthropic | 1000000 | 1000000 | 1000000 | 128000 | 1904 | dim=9 norm=0.535 [-0.2957, -0.1191, -0.0204, 0.3884, -0.1166, 0.09573, ...] |
| 7 | openrouter://bytedance-seed/seed-1.6-flash | personas/persons/d715074-5.txt | bytedance-seed | 262144 | 262144 | 262144 | 32768 | 4504 | dim=9 norm=0.7682 [-0.03585, -0.5015, -0.4562, 0.08995, 0.2209, -0.1029, ...] |
| 8 | openrouter://minimax/minimax-m1 | personas/persons/e91db77-0.txt | minimax | 1000000 | 1000000 | 1000000 | 40000 | 3657 | dim=9 norm=0.5713 [-0.1123, 0.1732, -0.1104, -0.316, 0.0001183, 0.08264, ...] |
| 9 | openrouter://kwaipilot/kat-coder-pro-v2 | personas/persons/e91db77-0.txt | kwaipilot | 256000 | 256000 | 256000 | 80000 | 7643 | dim=9 norm=0.6672 [-0.2223, 0.2406, -0.1219, 0.2831, 0.1953, -0.2957, ...] |
| 10 | openrouter://arcee-ai/trinity-large-thinking | personas/persons/d715074-3.txt | arcee-ai | 262144 | 262144 | 262144 | 262144 | 3099 | dim=9 norm=0.5448 [-0.188, 0.1689, 0.09061, -0.3156, -0.2836, 0.1204, ...] |
| 11 | openrouter://openai/gpt-4.1-mini | personas/persons/d715074-6.txt | openai | 1047576 | 1047576 | 1047576 | 32768 | 1881 | dim=9 norm=0.67 [0.271, 0.1253, 0.1371, 0.3434, -0.4035, 0.09143, ...] |
| 12 | openrouter://mistralai/ministral-14b-2512 | personas/persons/d715074-1.txt | mistralai | 262144 | 262144 | 262144 |  | 1256 | dim=9 norm=0.6682 [0.09417, 0.2482, -0.07762, -0.1665, 0.2014, -0.3597, ...] |
| 13 | openrouter://google/gemini-2.0-flash-lite-001 | personas/persons/d715074-8.txt | google | 1048576 | 1048576 | 1048576 | 8192 | 907 | dim=9 norm=0.5264 [0.2829, -0.09668, 0.1003, 0.3076, -0.06739, -0.1015, ...] |
| 14 | openrouter://anthropic/claude-sonnet-4.6 | personas/persons/d715074-5.txt | anthropic | 1000000 | 1000000 | 1000000 | 128000 | 2747 | dim=9 norm=0.4964 [-0.3431, -0.01488, -0.2067, 0.01418, 0.186, -0.05364, ...] |
| 15 | openrouter://kwaipilot/kat-coder-pro-v2 | personas/persons/d715074-3.txt | kwaipilot | 256000 | 256000 | 256000 | 80000 | 7643 | dim=9 norm=0.3835 [0.1043, 0.1784, 0.06131, 0.01229, 0.1406, 0.2494, ...] |
| 16 | openrouter://qwen/qwen3-max-thinking | personas/persons/e91db77-0.txt | qwen | 262144 | 262144 | 262144 | 32768 | 3743 | dim=9 norm=0.828 [-0.08898, 0.3665, -0.331, 0.4783, 0.01373, 0.01291, ...] |
| 17 | openrouter://arcee-ai/trinity-large-thinking | personas/persons/c4e1a2b-0.txt | arcee-ai | 262144 | 262144 | 262144 | 262144 | 3099 | dim=9 norm=0.6754 [-0.4386, 0.1623, -0.07927, 0.1879, 0.1862, 0.002992, ...] |
| 18 | openrouter://inclusionai/ling-2.6-flash | personas/persons/e50e538-0.txt | inclusionai | 262144 | 262144 | 262144 | 32768 | 1235 | dim=9 norm=0.5639 [0.3061, -0.2584, 0.07799, -0.291, -0.1795, 0.04719, ...] |
| 19 | openrouter://bytedance-seed/seed-1.6-flash | personas/persons/e50e538-0.txt | bytedance-seed | 262144 | 262144 | 262144 | 32768 | 4504 | dim=9 norm=0.4945 [0.2578, -0.1044, -0.03909, -0.05426, 0.3077, -0.02294, ...] |
| 20 | openrouter://minimax/minimax-m1 | personas/persons/c4e1a2b-0.txt | minimax | 1000000 | 1000000 | 1000000 | 40000 | 3657 | dim=9 norm=0.5734 [0.04555, 0.1172, -0.2257, -0.2705, 0.1031, 0.2718, ...] |

