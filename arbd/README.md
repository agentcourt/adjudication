# Agent Arbitration Degree

Agent Arbitration Degree is a sibling of [Agent Arbitration](../arb).  It keeps the same adversarial merits sequence: openings, arguments, rebuttal, surrebuttal, closings, and council deliberation.  The difference is the result: `arbd/` handles degree questions and asks each seated council member for one integer answer from `0` through `100`.

The procedure records the final answer map keyed by `member_id`, for example `{"C1":72,"C2":45,"C3":88}`.  The case closes when every seated council member has submitted one answer for the round.  If a council member fails during deliberation, the engine records that member as failed and continues with the remaining seated members.

## Layout

| Path | Purpose |
|---|---|
| `agent-instructions/` | Templates used by `aard run` for OpenClaw lawyers, remote OpenClaw lawyers, and Pi council members. |
| `docs/` | Project rules and notes.  See [`docs/evidence-handling.md`](docs/evidence-handling.md) for evidence custody and evidence-transfer semantics. |
| `engine/` | Lean engine and proofs. |
| `runtime/` | Go runtime and `aard` CLI. |
| `prompts/` | Attorney and council prompts. |
| `examples/` | Degree-question example materials. |

## Build

Run these commands from `arbd/`.  `make build` builds the Lean engine and the Go CLI.  `make test` runs the Go tests under `runtime/`, and `make prove` runs the Lean proof batch.

```bash
make build
make test
make prove
```

## Complaint Format

`aard complain` and `aard validate` accept plain text or a markdown file with a `Question` heading.  If the source contains that heading, the commands use that section.  Otherwise they treat the trimmed file as the question and write canonical complaint output with a `# Question` heading.

```markdown
# Question

What percentage of the 2025 work overlaps with the 2024 work?
```

## Commands

| Command | Purpose |
|---|---|
| `aard complain` | Generate a canonical complaint from a situation file. |
| `aard validate` | Validate a complaint and print the parsed question. |
| `aard case` | Start one case process with HTTP Lawyer and Council APIs. |
| `aard mcp` | Serve MCP sessions that proxy to a running case API. |
| `aard service` | Start the Clerk HTTP service for creating, listing, inspecting, killing, and reading cases. |
| `aard run` | Start a local case, MCP server, OpenClaw lawyers, and Pi council agents. |

`aard case` is the low-level case process.  It loads the complaint, policy, case files, prompts, council pool, and Lean engine, then waits for lawyer and council clients.  It does not start OpenClaw or Pi agents; `aard run` starts those agents and points them at the case through MCP.

## Case Files

`aard case` scans the complaint directory for initial case evidence when `--file` is absent.  That scan skips the complaint itself, the situation file, `README.md`, signing evidence, and directories.  It loads `.txt`, `.md`, `.pem`, and `.b64` evidence as text-readable evidence, and it records other file types as byte-bearing evidence with `evidence_id`, SHA-256, byte size, MIME type, and content-addressed storage metadata.

## HTTP And MCP

The case process exposes a private health endpoint plus Lawyer and Council APIs.  Lawyer clients call `/lawyerapi/v1/get`, `/lawyerapi/v1/wait`, `/lawyerapi/v1/status`, `/lawyerapi/v1/result`, and `/lawyerapi/v1/do` with `case_id` and `role_id`.  Council clients call `/councilapi/v1/get`, `/councilapi/v1/wait`, `/councilapi/v1/do`, and `/councilapi/v1/fail` with `case_id` and `member_id`.

`aard mcp --caseapi-base URL` serves MCP over HTTP and translates MCP tool calls into those case APIs.  A lawyer session binds to one `case_id` and one role, while a council session binds to one `case_id` and one council member.  The MCP layer carries no degree policy of its own; the case API supplies the current prompt, allowed tools, remaining time, attempts left, and final result data.

## Local Runs

`aard run` starts a complete local run using OpenClaw lawyer containers and Pi council containers.  OpenClaw can authenticate with either `OPENAI_API_KEY` or a Codex `auth.json` file supplied through `--openclaw-auth codex --openclaw-codex-auth PATH`.  Pi council members are sampled from `--council-pool`, and each selected entry supplies the provider, model, quantization, and persona used for that council member.

```bash
.bin/aard run ex1 \
  --openclaw-auth codex \
  --openclaw-codex-auth ~/src/auth.json \
  --council-pool pool.jsonl
```

`--auto-lawyers both` starts both local OpenClaw lawyers.  `--auto-lawyers plaintiff` or `--auto-lawyers defendant` starts only that side, so the other side can be a remote OpenClaw using the generated remote-lawyer instructions.  Use `--mcp-public-base-url` when a remote OpenClaw needs an externally reachable MCP URL.

## Service

`aard service` starts the Clerk API.  `/api/v1/cases` creates and manages low-level `aard case` processes, while `/clerk/v1/cases` creates and manages full `aard run` processes.  The service stores case records under the configured output root and polls each child case health endpoint until it is ready or the startup timeout expires.

## Outputs

Each run writes a packet to `--out-dir`: `complaint.md`, `policy.json`, `runtime.json`, `run.json`, `state.json`, `council.json`, `evidence-manifest.json`, `evidence-store/`, `digest.md`, `transcript.md`, `events.ndjson`, and work-note logs when lawyers submit them.  The digest and transcript separate source evidence from attorney analysis.  Final-result APIs return the case status while the case is pending and the council answer map after closure.

On success, `aard case` prints one JSON object to stdout:

```json
{"status":"ok","answers":{"C1":72,"C2":45,"C3":88},"run_id":"run-123","out_dir":"out/ex1-demo"}
```

On failure, it prints a JSON object with `status` and `error`:

```json
{"status":"error","error":"..."}
```
