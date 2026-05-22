# Agent Arbitration

Agent Arbitration is a distilled dispute-resolution procedure derived from the sibling [adc](../adc) copy.  This procedure removes pretrial motions, voir dire, the judge, and the clerk.  The merits are argued before a council.  The complaint states the proposition.  Policy or case configuration supplies the standard of evidence.

This repository contains the Lean engine, the Go runtime, the `aar` CLI, and a set of example cases.  The runtime writes a complete case packet for each run: complaint, policy, runtime limits, final state, council roster, transcript, digest, and event log.  The `aar case` command also prints a one-line JSON summary to stdout with the result and the final vote count.

## Layout

| Path | Purpose |
|---|---|
| `docs/` | Project rules and notes. See [`docs/openclaw-attorneys.md`](docs/openclaw-attorneys.md) for the OpenClaw-attorney workflow and [`docs/evidence-handling.md`](docs/evidence-handling.md) for artifact custody and evidence-transfer semantics. |
| `engine/` | Lean arbitration engine |
| `runtime/` | Go CLI and runtime bridge |
| `examples/` | Example disputes |

## Build

`make build` builds the Lean engine and the Go CLI into `.bin/`.

`make test` runs the Go tests.

`make prove` builds all of the theorems.

`make demo` drafts the first example complaint and runs one arbitration in `out/ex1-demo/`.

## Run An Arbitration From Scratch

These commands assume the current working directory is `arb/`.  `aar complain` reads a markdown file and writes canonical complaint form.  If the source contains a `Proposition` heading, the command extracts that section.  Otherwise it treats the whole trimmed file as the proposition.  `aar case` initializes the run from that complaint, loads the council pool from `../common/data/personas/pool.csv` by default, writes the run packet to the requested output directory, and prints a JSON summary to stdout.

Create a case directory with a situation file.  The source file may be plain text or a markdown file with a `Proposition` heading.  The canonical complaint format contains only the proposition.

```markdown
Whether the published statement defamed the plaintiff.
```

Build the engine and CLI, draft the complaint, and run the case:

```bash
make build
mkdir -p work/defamation
.bin/aar complain \
  --situation work/defamation/situation.md \
  --out work/defamation/complaint.md
.bin/aar case \
  --complaint work/defamation/complaint.md \
  --out-dir out/defamation-demo
```

`aar case` scans the complaint directory for initial case artifacts when `--file` is absent.  That scan skips the complaint itself, the situation file, `README.md`, signing artifacts, and directories.  It loads `.txt`, `.md`, `.pem`, and `.b64` artifacts as text-readable artifacts, and it records other file types as byte-bearing artifacts.  Each loaded artifact is registered with `artifact_id`, SHA-256, byte size, MIME type, and content-addressed storage metadata.

This variant shows the common parameters that change a run:

```bash
.bin/aar case \
  --complaint work/defamation/complaint.md \
  --file 'work/defamation/exhibits/*.txt' \
  --file work/defamation/statement.md \
  --out-dir out/defamation-demo \
  --policy etc/policy.json \
  --council-size 7 \
  --evidence-standard "Clear and convincing evidence." \
  --council-pool ../common/data/personas/pool.csv \
  --attorney-instructions attorney-instructions/default.md \
  --attorney-model 'openai://gpt-5?tools=search' \
  --timeout-seconds 120 \
  --acp-timeout-seconds 300 \
  --invalid-attempt-limit 2 \
  --run-id run-defamation-demo
```

The explicit `--file` path can be repeated.  It accepts shell globs, and it rejects `.gitignore`, `.sh`, and `.sig` files.  When you omit `--policy`, `aar case` loads `etc/policy.json` from the current working directory if that file exists.  Otherwise it uses the built-in default policy.

## Attorney Configuration

By default, `aar case` runs both attorneys through the local Pi ACP wrapper at `../common/pi-container/acp-podman.sh`.  The global `--attorney-model` flag applies only to local Pi/xproxy attorneys unless a role-specific local model override is present.  Search capability for that path comes from the model id itself.  For example, `openai://gpt-5` runs without native search, while `openai://gpt-5?tools=search` requests native search through xproxy.

Use `--attorney-instructions FILE` to provide the standing attorney-side instructions file for the run.  When that flag is absent, `aar case` falls back to `./attorney-instructions/default.md` from the current working directory if that file exists.  `arb` applies one shared attorney-instructions file to both sides.  A remote ACP endpoint still has to honor its own instructions on the remote side; `arb` does not transmit this file over the TCP ACP transport.

The global `--acp-command` flag sets the local ACP command for both sides.  Each side can override the shared configuration with its own local Pi/xproxy model, local ACP command, remote ACP endpoint, and ACP session working directory.  A role cannot set both `--*-acp-command` and `--*-acp-endpoint` in the same run.  A role using `--*-acp-endpoint` also cannot set `--*-attorney-model`; the remote ACP attorney owns its model selection and tool availability.

This command keeps the defendant on the local ACP wrapper and points the plaintiff at a remote ACP endpoint:

```bash
.bin/aar case \
  --complaint work/defamation/complaint.md \
  --out-dir out/defamation-demo \
  --plaintiff-acp-endpoint 'tcp://agent.example.com:7000' \
  --defendant-attorney-model 'openai://gpt-5' \
  --defendant-acp-command ../common/pi-container/acp-podman.sh
```

The remote endpoint path uses a persistent TCP connection that carries newline-delimited ACP JSON-RPC messages.  `arb` exposes the current `_aar/*` client methods for case access and filing over that session.  A remote ACP server must already know how to use those methods.

For OpenClaw attorneys, prefer `--*-acp-endpoint` and run an OpenClaw ACP attorney server at that endpoint.  AAR will connect over ACP and assume the remote OpenClaw side owns model selection, session policy, and native tool availability.  The full reproduction guide is [`docs/openclaw-attorneys.md`](docs/openclaw-attorneys.md).

The `aar-openclaw-attorney` adapter is a local ACP wrapper for smoke tests and custom integrations.  It preloads the visible AAR record, inspects visible artifacts through the AAR artifact methods, asks OpenClaw for one strict JSON filing, and submits that filing through `_aar/submit_decision`.  It does not accept or forward an AAR model selection.

```bash
AAR_OPENCLAW_AGENT=1 \
AAR_OPENCLAW_AGENT_ID=aar-lawyer \
AAR_OPENCLAW_AGENT_SESSION_ID=aar-plaintiff-demo \
.bin/aar case \
  --complaint work/defamation/complaint.md \
  --out-dir out/defamation-openclaw-demo \
  --plaintiff-acp-command .bin/aar-openclaw-attorney \
  --defendant-acp-command ../common/pi-container/acp-podman.sh
```

Use a dedicated OpenClaw lawyer agent for this path.  The adapter requires `AAR_OPENCLAW_AGENT_ID` so arbitration prompts do not fall through to a default personal agent workspace.

For deterministic local tests, set `AAR_OPENCLAW_ATTORNEY_DECISION_JSON` to the exact filing object instead of `AAR_OPENCLAW_AGENT=1`.  For custom integrations, set `AAR_OPENCLAW_ATTORNEY_COMMAND` to a command that reads the prompt packet from stdin and prints the filing JSON to stdout.

For endpoint smoke tests, `tools/openclaw-acp-tcp-bridge.js` exposes the stdio adapter as a local TCP ACP endpoint.  It listens on `127.0.0.1:19701` by default, spawns `.bin/aar-openclaw-attorney` for each connection, pipes ACP JSON-RPC between the socket and the adapter, and strips `AAR_OPENCLAW_AGENT_MODEL` from the adapter environment so model selection stays on the OpenClaw side.

```bash
make build
AAR_OPENCLAW_AGENT_ID=aar-lawyer \
tools/openclaw-acp-tcp-bridge.js
```

In another shell:

```bash
.bin/aar case \
  --complaint work/defamation/complaint.md \
  --out-dir out/defamation-openclaw-endpoint-demo \
  --plaintiff-acp-endpoint tcp://127.0.0.1:19701 \
  --defendant-acp-endpoint tcp://127.0.0.1:19701
```

The helper also accepts `--host`, `--port`, and `--adapter`.  It is intended for local smoke tests and simple single-host integrations, not as a hardened network service.

This command shows the same pattern with one global ACP command and a role-specific remote override:

```bash
.bin/aar case \
  --complaint work/defamation/complaint.md \
  --out-dir out/defamation-demo \
  --attorney-model 'openai://gpt-5' \
  --acp-command ../common/pi-container/acp-podman.sh \
  --plaintiff-acp-endpoint 'tcp://agent.example.com:7000'
```

## Case Parameters

`aar help case` prints the full flag list.  These parameters control most runs:

| Flag | Meaning |
|---|---|
| `--complaint` | Complaint markdown file.  Required. |
| `--out-dir` | Output directory for the run packet.  Required. |
| `--file` | Explicit initial artifact path or glob.  Repeating this flag replaces automatic complaint-directory scanning. |
| `--policy` | Policy JSON file.  Defaults to `./etc/policy.json` when present. |
| `--council-size` | Override `policy.council_size`. |
| `--evidence-standard` | Override `policy.evidence_standard`. |
| `--council-pool` | Council model and persona pool.  Defaults to `../common/data/personas/pool.csv` when `arb/` is the working directory. |
| `--attorney-instructions` | Standing attorney-side instructions file.  Defaults to `./attorney-instructions/default.md` when present. |
| `--attorney-model` | Local Pi/xproxy attorney model id, including any search capability request, such as `openai://gpt-5` or `openai://gpt-5?tools=search`. This does not apply to roles using `--*-acp-endpoint`. |
| `--acp-command` | Shared local ACP command for both attorneys.  Defaults to `<common-root>/pi-container/acp-podman.sh`. |
| `--plaintiff-attorney-model`, `--defendant-attorney-model` | Role-specific local Pi/xproxy attorney model overrides. Invalid with the same role's `--*-acp-endpoint`. |
| `--plaintiff-acp-command`, `--defendant-acp-command` | Role-specific ACP command overrides. |
| `--plaintiff-acp-endpoint`, `--defendant-acp-endpoint` | Role-specific remote ACP endpoints.  Supported transport: `tcp://host:port`. |
| `--plaintiff-acp-session-cwd`, `--defendant-acp-session-cwd` | Role-specific `session/new` working-directory overrides. |
| `--common-root` | Shared `common/` tree used for the pool, xproxy config, and ACP launcher. |
| `--xproxy-config` | xproxy configuration file.  Defaults under `common/`. |
| `--xproxy-port` | xproxy port.  Default: `18459`. |
| `--timeout-seconds` | Council LLM timeout override. |
| `--acp-timeout-seconds` | Attorney ACP timeout override. |
| `--max-response-bytes` | Maximum parsed response size override. |
| `--invalid-attempt-limit` | Maximum invalid-attempt count before a participant is removed. |
| `--run-id` | Explicit run identifier. |
| `--engine` | Lean engine binary.  Defaults to `.bin/aarengine` next to the CLI binary. |

## Outputs

Each run writes a complete packet to `--out-dir`.  The main files are `complaint.md`, `policy.json`, `runtime.json`, `run.json`, `state.json`, `council.json`, `digest.md`, `transcript.md`, `events.ndjson`, and `artifact-manifest.json`.  `run.json` records the resolved attorney configuration for each side in its `attorneys` field and includes visible artifact metadata.  Exact artifact bytes are stored under `artifact-store/`; accepted attorney evidence is also copied under `submitted-evidence/` and exposed through artifact methods.  Attorney work product is exported into the run directory separately.

On success, `aar case` prints a JSON object like this:

```json
{"status":"ok","result":"demonstrated","votes_for":3,"votes_against":2,"run_id":"run-123","out_dir":"out/defamation-demo"}
```

On failure, it prints:

```json
{"status":"error","error":"..."}
```

## Examples

The checked-in Makefile targets show the current example configurations.  `make demo`, `make ex2`, and `make ex3` run with `openai://gpt-5` as the attorney model.  `make ex4` and `make ex6` run with `openai://gpt-5?tools=search`.
