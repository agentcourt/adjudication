# Running AAR With OpenClaw And Pi

## Purpose

This runbook describes the current way to run one local AAR case with real OpenClaw lawyers and Pi council members.  The command is `aar run`.  It starts the case runtime and MCP server in the same process, then starts one OpenClaw container for each lawyer and one Pi container for each council member.

The case runtime owns arbitration state, turn order, deadlines, attempt budgets, evidence custody, filing validation, council voting, and final artifacts.  The MCP server exposes the case runtime to remote-style agents.  Each MCP session stores only the assignment for that session: `case_id` plus `role_id` for a lawyer or observer, or `case_id` plus `member_id` for a council member.

## Topology

| Process | Runs where | Role |
| --- | --- | --- |
| `aar run` | host | Runs one arbitration, starts the case API and MCP server, starts local agents, and writes artifacts. |
| OpenClaw lawyer containers | Docker or remote host | Act as plaintiff and defendant through MCP. |
| Pi council containers | Podman or remote host | Act as council members through MCP. |

Local Docker containers reach the host MCP server through `host.docker.internal`.  On Linux, Docker needs `--add-host=host.docker.internal:host-gateway`.  Pi containers run with Podman host networking and reach MCP through `127.0.0.1`.

Lawyer containers must be treated like remote lawyers.  Do not mount the repository, the example directory, the output directory, or a case-packet directory into a lawyer container.  A lawyer receives case files only through AAR evidence tools, which matches a remote clawyer that has an MCP URL and token but no access to the operator filesystem.

## Case File Access

`aar run` imports the complaint directory into the AAR record.  Files in the case packet become immutable `case_packet` evidence with an `evidence_id`, hash, size, MIME type, title, and storage record.  A lawyer or council member reads those files through MCP tools that forward to the case API for that case.

Every lawyer phase allows read-only evidence access.  Argument, rebuttal, and surrebuttal phases also allow evidence submission.  The current opportunity returned by `wait_for_opportunity` or `get_current_opportunity` states the court actions allowed for that turn.

Lawyers should cite only AAR `evidence_id` values in `offered_evidence`.  If a lawyer finds a public source with OpenClaw web, browser, file, shell, OCR, PDF, image, audio, video, metadata, hash, signature, archive, or local analysis tools, the lawyer must submit that source through AAR evidence tools before relying on it as record support.  Work notes are private operator diagnostics and are not evidence.

## Build

Run the build from `arb/`:

```bash
make build
```

This builds `.bin/aar` and the Lean engine used by the runtime.  The split MCP adapter commands are no longer part of the supported build.

## Running One Example

Run an example directly:

```bash
.bin/aar run ex01
```

The command starts the case runtime and MCP server, starts OpenClaw containers for plaintiff and defendant, starts Pi council members from the AAR roster, and writes final artifacts under the output directory.  Each OpenClaw container keeps one session key for the whole lawyer assignment.  Each Pi member receives one mounted home directory under the output directory, so its session files, MCP config, settings, and model config remain available for review.

The command supports two OpenClaw lawyer authentication paths.  By default, `--openclaw-auth auto` first looks for a readable Codex auth file at `~/.codex/auth.json`, or at `$CODEX_HOME/auth.json` when `CODEX_HOME` is set.  If that file is available, `aar run` copies it into one temporary Codex home per OpenClaw lawyer container, mounts that directory as `/aar-codex`, and sets `CODEX_HOME=/aar-codex` inside the container.  If no readable Codex auth file is available, automatic mode falls back to `OPENAI_API_KEY`.

Use `--openclaw-auth codex` to require the Codex auth-file path.  Use `--openclaw-codex-auth PATH` to choose a different `auth.json` file.  Use `--openclaw-auth api-key` to require `OPENAI_API_KEY`.  `OPENROUTER_API_KEY` is still required for Pi council members.  The command uses `gpt-5.5` for OpenClaw agents by default.  It does not mount case files or output directories into the lawyer containers.

## Manual Service Commands

The service commands remain available for multi-case service work.  They are separate from `aar run`.  The service starts first:

```bash
OUT=out/ex01-service-openclaw-$(date -u +%Y%m%d%H%M%S)
TOKEN=aar-local-test-token
mkdir -p "$OUT/logs"

.bin/aar service \
  --listen 127.0.0.1:19770 \
  --registry-dir "$OUT/registry" \
  --out-root "$OUT/service-out" \
  --aar-bin "$(pwd)/.bin/aar" \
  --bearer-token "$TOKEN" \
  >"$OUT/logs/service.stdout" \
  2>"$OUT/logs/service.stderr" &
echo $! >"$OUT/service.pid"
```

Then start the MCP server.  During local Docker tests it listens on all host interfaces so `host.docker.internal` can reach it:

```bash
.bin/aar mcp \
  --listen 0.0.0.0:19780 \
  --caseapi-base http://127.0.0.1:19770 \
  --bearer-token "$TOKEN" \
  --api-bearer-token "$TOKEN" \
  --session-ttl 0 \
  >"$OUT/logs/mcp.stdout" \
  2>"$OUT/logs/mcp.stderr" &
echo $! >"$OUT/mcp.pid"
```

Create a case through the service.  The `case_id` becomes the public routing key for every lawyer, observer, and council call:

```bash
curl -fsS \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "case_id": "arb-ex01-local",
    "complaint_path": "examples/ex01/complaint.md",
    "out_dir": "'"$OUT"'/case",
    "council_backend": "councilapi",
    "lawyer_timeout_seconds": 900,
    "council_timeout_seconds": 900
  }' \
  http://127.0.0.1:19770/api/v1/cases
```

An OpenClaw lawyer receives one MCP server definition and one assignment prompt.  The MCP URL binds that session to one role:

```json
{
  "url": "http://host.docker.internal:19780/mcp?case_id=arb-ex01-local&role_id=plaintiff",
  "transport": "streamable-http",
  "headers": {
    "Authorization": "Bearer REPLACE_WITH_TOKEN"
  }
}
```

Council members use the same MCP endpoint with `member_id` instead of `role_id`.  `aar run` writes this URL into the Pi member's `.mcp.json` file:

```json
{
  "url": "http://host.docker.internal:19780/mcp?case_id=arb-ex01-local&member_id=C1",
  "transport": "streamable-http",
  "headers": {
    "Authorization": "Bearer REPLACE_WITH_TOKEN"
  }
}
```

## Assignment Behavior

A lawyer assignment tells OpenClaw to call `wait_for_opportunity` first and to keep calling it while the response is `state: "waiting"`.  When the response is `state: "ready"`, the lawyer reads the prompt, allowed operations, limits, remaining time, attempts remaining, and `opportunity_id`, then completes that opportunity through the AAR tools.  After a successful filing, the lawyer returns to `wait_for_opportunity` for the next turn.

The lawyer should inspect the record at each opportunity.  `get_case`, `list_evidence`, `stat_evidence`, and `read_evidence_range` provide case-packet files and admitted evidence.  `send_work_notes` records private plans, work logs, search logs, source checks, scripts, installed programs, OCR or extraction work, browser work, errors, analysis, decisions, and unresolved gaps for operator review outside the record.

A council assignment follows the same wait loop, but council tools are read-only except for `submit_council_vote`.  Council members review the record and admitted evidence, then vote `demonstrated` or `not_demonstrated` with a rationale grounded in the record.  Pi receives the selected OpenRouter model, max-token limit, and provider routing constraints in `.pi/agent/models.json`; the routing constraints come from the sampled council pool entry.

## Review

The service result endpoint returns pending status while the case runs and final vote details after completion:

```bash
curl -fsS \
  -H "Authorization: Bearer $TOKEN" \
  http://127.0.0.1:19770/api/v1/cases/arb-ex01-local/result
```

The output directory contains the AAR record and process logs.  Review `case/digest.md`, `case/transcript.md`, `case/events.ndjson`, `case/work-notes.ndjson`, `case/evidence-manifest.json`, `case/run.json`, and `logs/openclaw-*.stdout`.  Evidence review should check whether lawyers read case-packet evidence, admitted outside sources before relying on them, used `evidence_id` values in filings, and explained what the evidence proved.

## Cleanup

`aar run` stops the case runtime, MCP server, OpenClaw containers, and Pi council containers when the command exits.  It also removes staged OpenClaw Codex homes because they contain authentication material.  For manual service runs, stop the host processes by using the PID files, then stop any remaining containers by name.  Do not delete the output directory before review because it contains the record, work notes, service logs, MCP logs, OpenClaw logs, Pi logs, and Pi member homes needed to diagnose the run.
