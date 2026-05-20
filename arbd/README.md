# Agent Arbitration Degree

Agent Arbitration Degree is a sibling of [Agent Arbitration](../arb).  It keeps the same stripped-down merits procedure: openings, arguments, rebuttal, surrebuttal, closings, and council deliberation.  The difference is the question type.  `arbd/` handles questions of degree and returns one bounded integer answer from each council member in the range `[0,100]`.

The procedure does not aggregate those answers.  It records the final answer map keyed by `member_id`, for example `{"C1":72,"C2":45,"C3":88}`.  The result closes when every seated council member has submitted one answer for the round.  If removals or timeouts reduce the seated set during deliberation, closure follows the remaining seated members.

## Layout

| Path | Purpose |
|---|---|
| `engine/` | Lean engine and proofs |
| `runtime/` | Go runtime and `aard` CLI |
| `prompts/` | Attorney and council prompts |
| `examples/` | Degree-question example materials |

## Build

Run these commands from `arbd/`:

```bash
make build-image
make build
make test
make prove
```

`make build-image` builds the shared Podman image used by the ACP attorney backend.  `make build` then builds the Lean engine and the Go CLI.  `make demo` drafts `examples/ex1/complaint.md` from `examples/ex1/situation.md` and runs one complete case in `out/ex1-demo/`.

## Complaint Format

`aard complain` and `aard validate` accept plain text or a markdown file with a `Question` heading.  If the source contains that heading, the commands use the section.  Otherwise they treat the whole trimmed file as the question.  The canonical complaint output keeps the heading:

```markdown
What percentage of artwork Y is novel in view of artwork X?
```

## Run A Case

```bash
make build-image
make build
.bin/aard complain \
  --situation examples/ex1/situation.md \
  --out examples/ex1/complaint.md
.bin/aard case \
  --complaint examples/ex1/complaint.md \
  --out-dir out/ex1-demo
```

`aard case` scans the complaint directory for case files when `--file` is absent.  It skips the complaint, the situation file, `README.md`, signing artifacts, and directories.  It loads `.txt`, `.md`, `.pem`, and `.b64` files as readable case files and records other file types as byte-bearing exhibits.

During arguments and claimant rebuttal, attorneys may submit source material discovered during the run through `aar_submit_evidence`.  The runner stores accepted bytes under `submitted-evidence/`, records source URL or source description, MIME type, retrieval timestamp, relevance, SHA-256, and byte count, and adds the new file to the visible case-file set.  A party that wants the material admitted as an exhibit must cite the returned `file_id` in `offered_files`; attorney analysis belongs in `technical_reports`.

The default attorney path uses `../common/pi-container/acp-podman.sh`.  That path also requires a provider API key in the environment for the selected model endpoint, such as `OPENAI_API_KEY` for `openai://...` models or `OPENROUTER_API_KEY` for `openrouter://...` models.

## Key Flags

| Flag | Meaning |
|---|---|
| `--complaint` | Complaint markdown file.  Required. |
| `--out-dir` | Output directory for the run packet.  Required. |
| `--file` | Explicit case file path or glob.  Repeating this flag replaces automatic complaint-directory scanning. |
| `--policy` | Policy JSON file.  Defaults to `./etc/policy.json` when present. |
| `--council-size` | Override `policy.council_size`. |
| `--judgment-standard` | Override `policy.judgment_standard`. |
| `--council-pool` | Council model and persona pool.  Defaults under `common/`. |
| `--attorney-model` | Attorney ACP model id. |
| `--plaintiff-acp-endpoint` | Remote ACP endpoint for the plaintiff attorney. |
| `--defendant-acp-endpoint` | Remote ACP endpoint for the defendant attorney. |
| `--attorney-instructions` | Standing attorney instructions file. |
| `--engine` | Lean engine binary.  Defaults to `.bin/aardengine` next to the CLI binary. |

A role using a remote ACP endpoint owns its own model selection and native tool availability.  Do not combine `--plaintiff-attorney-model` with `--plaintiff-acp-endpoint`, or `--defendant-attorney-model` with `--defendant-acp-endpoint`.  The local model flags apply to the Pi/xproxy attorney path only.

OpenClaw-backed endpoint runs are described in [OpenClaw Degree Attorneys](docs/openclaw-attorneys.md).  The build creates `.bin/aard-openclaw-attorney`, and `tools/openclaw-acp-tcp-bridge.js` exposes that stdio adapter as a TCP ACP endpoint.  Use `make openclaw-acp-bridge` to build the tools and start the bridge with default settings.

## Outputs

Each run writes a full packet to `--out-dir`: `complaint.md`, `policy.json`, `runtime.json`, `run.json`, `state.json`, `council.json`, `digest.md`, `transcript.md`, and `events.ndjson`.  When attorneys submit source evidence, the packet also includes a `submitted-evidence/` directory.  The digest and transcript distinguish submitted source evidence from exhibits and technical reports.

On success, `aard case` prints one JSON object to stdout:

```json
{"status":"ok","answers":{"C1":72,"C2":45,"C3":88},"run_id":"run-123","out_dir":"out/ex1-demo"}
```

On failure, it prints:

```json
{"status":"error","error":"..."}
```
