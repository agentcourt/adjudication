# Agent Arbitration

Agent Arbitration is a stripped-down dispute-resolution procedure.  A complaint states one proposition, lawyers build the record for the plaintiff and defendant, and a council decides whether the proposition has been demonstrated under the configured evidence standard.  The runtime writes a complete packet for each run: complaint, policy, runtime limits, final state, council roster, transcript, digest, evidence manifest, and event log.

## Layout

| Path | Purpose |
| --- | --- |
| `docs/` | Project rules and notes, including evidence custody. |
| `engine/` | Lean arbitration engine. |
| `runtime/` | Go CLI, runner, Lawyer API, and council integrations. |
| `examples/` | Example disputes. |
| `prompts/` | Prompt templates for lawyers and council members. |

## Build

Run these commands from `arb/`.  `make build` builds the Lean engine and the Go CLI into `.bin/`.  `make test` runs the Go tests, and `make prove` builds the Lean proofs.

```bash
make build
make test
make prove
```

## Run A Case

`aar complain` reads a situation file and writes the canonical complaint form.  If the source contains a `Proposition` heading, the command extracts that section.  Otherwise it treats the whole trimmed file as the proposition.

```bash
mkdir -p work/defamation
.bin/aar complain \
  --situation work/defamation/situation.md \
  --out work/defamation/complaint.md
```

`aar case` starts the arbitration, starts the private case API, loads evidence, samples the council, and waits for lawyer actions over HTTP.  The command prints the private case API base URL to stderr in this form: `caseapi listening on http://127.0.0.1:PORT`.  The command prints the final one-line JSON case summary to stdout after the case resolves or fails.

```bash
.bin/aar case \
  --complaint work/defamation/complaint.md \
  --out-dir out/defamation-demo
```

When `--file` is absent, `aar case` scans the complaint directory for initial evidence.  That scan skips the complaint itself, the situation file, `README.md`, signing evidence, and directories.  It loads `.txt`, `.md`, `.pem`, and `.b64` evidence as text-readable evidence, and it records other file types as byte-bearing evidence.

## Lawyer API

The Lawyer API lets a plaintiff, defendant, or observer use plain HTTP.  A lawyer calls `GET /lawyerapi/v1/get` with `case_id` and `role_id` to receive the current prompt, available tools, role status, opportunity id, live deadline, attempts left, and limits.  A remote runner can call `GET /lawyerapi/v1/wait` to block until its role has work or case state changes.  A lawyer copies `turn.opportunity_id` from the ready response into each `POST /lawyerapi/v1/do` request for the turn.  `POST /do` executes one tool call, including `submit_decision` when the role is ready to file the legal act for the turn.

The API roles are `plaintiff`, `defendant`, and `observer`.  The plaintiff and defendant roles can act only during their own turns.  The observer role is read-only and can call `get_turn`, `get_case`, `list_events`, `list_evidence`, `stat_evidence`, and `read_evidence_range`.

```bash
curl -sS "$BASE/get?case_id=arb-1&role_id=plaintiff"

curl -sS "$BASE/wait?case_id=arb-1&role_id=plaintiff&timeout_ms=30000"

curl -sS -X POST "$BASE/do" \
  -H 'content-type: application/json' \
  --data '{
    "case_id": "arb-1",
    "role_id": "plaintiff",
    "opportunity_id": "openings:plaintiff",
    "tool": "submit_decision",
    "arguments": {
      "kind": "tool",
      "tool_name": "record_opening_statement",
      "payload": {
        "text": "Plaintiff opening.",
        "offered_evidence": [],
        "technical_reports": []
      }
    }
  }'
```

## Case Parameters

`aar help case` prints the full flag list.  Policy values affect the legal procedure and usually belong in `--policy`.  Runtime values affect process limits, timeouts, and service addresses.

| Flag | Meaning |
| --- | --- |
| `--complaint` | Complaint markdown file. Required. |
| `--out-dir` | Output directory for the run packet. Required. |
| `--file` | Explicit initial evidence path or glob. Repeating this flag replaces automatic complaint-directory scanning. |
| `--policy` | Policy JSON file. Defaults to `./etc/policy.json` when present. |
| `--council-size` | Override `policy.council_size`. |
| `--evidence-standard` | Override `policy.evidence_standard`. |
| `--council-pool` | Council model and persona pool. Defaults to `../common/data/personas/pool.csv` when `arb/` is the working directory. |
| `--attorney-instructions` | Standing lawyer instructions file. Defaults to `./attorney-instructions/default.md` when present. |
| `--caseapi-addr` | Private case API listen address. Default: `127.0.0.1:0`. |
| `--lawyer-timeout-seconds` | Lawyer turn timeout override. Default: 900 seconds. |
| `--council-backend` | Council backend. `direct` calls the selected model provider from the runner. `councilapi` waits for external council members through the Council API. |
| `--common-root` | Shared `common/` tree used for the council pool and persona files. |
| `--timeout-seconds` | Council LLM timeout override. |
| `--max-response-bytes` | Maximum parsed response size override. |
| `--invalid-attempt-limit` | Maximum invalid attempt count before a turn fails. |
| `--run-id` | Explicit run identifier. |
| `--engine` | Lean engine binary. Defaults to `.bin/aarengine` next to the CLI binary. |

## Outputs

Each run writes a complete packet to `--out-dir`.  The main files are `complaint.md`, `policy.json`, `runtime.json`, `state.json`, `council.json`, `digest.md`, `transcript.md`, `events.ndjson`, and `evidence-manifest.json`.  Exact evidence bytes are stored under `evidence-store/`, and accepted lawyer evidence is also copied under `submitted-evidence/`.

On success, `aar case` prints a JSON object like this:

```json
{"status":"ok","result":"demonstrated","votes_for":3,"votes_against":2,"run_id":"run-123","out_dir":"out/defamation-demo"}
```

On failure, it prints:

```json
{"status":"error","error":"..."}
```
