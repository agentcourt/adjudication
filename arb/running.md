# Running AAR With OpenClaw Through The Service APIs

## Purpose

This runbook describes the current way to run an AAR case with real OpenClaw containers.  One host process runs `aar service`, one host process runs `aar-mcp`, and one `aar case` child process runs each case.  OpenClaw containers act as lawyers and council members by using MCP tools that forward to the public service APIs.

The service owns case creation, case ids, process lifecycle, routing, logs, result reads, and artifact reads.  The child runner owns arbitration state, turn order, deadlines, attempt budgets, evidence custody, filing validation, council voting, and final artifacts.  The MCP process stores only the assignment for one MCP session: `case_id` plus `role_id` for a lawyer or observer, or `case_id` plus `member_id` for a council member.

## Topology

| Process | Runs where | Role |
| --- | --- | --- |
| `aar service` | host | Starts cases, records case metadata, and routes public HTTP calls by `case_id`. |
| `aar case` | host child process | Runs one arbitration and exposes private Lawyer and Council APIs on localhost. |
| `aar-mcp` | host | Exposes Streamable HTTP MCP and forwards tool calls to `aar service`. |
| OpenClaw lawyer containers | Docker or remote host | Act as plaintiff and defendant through MCP. |
| OpenClaw council containers | Docker or remote host | Act as council members through MCP. |

Local Docker containers reach the host MCP server through `host.docker.internal`.  On Linux, Docker needs `--add-host=host.docker.internal:host-gateway`.  The MCP server listens on a host address reachable from the containers, while `aar service` can stay on `127.0.0.1` because only the host MCP process and operator tools call it during local tests.

Lawyer containers must be treated like remote lawyers.  Do not mount the repository, the example directory, the output directory, or a case-packet directory into a lawyer container.  A lawyer receives case files only through AAR evidence tools, which matches a remote clawyer that has an MCP URL and token but no access to the operator filesystem.

## Case File Access

`aar case` imports the complaint directory into the AAR record.  Files in the case packet become immutable `case_packet` evidence with an `evidence_id`, hash, size, MIME type, title, and storage record.  A lawyer or council member reads those files through MCP tools that forward to the service, then to the private runner API for that case.

Every lawyer phase allows read-only evidence access.  Argument, rebuttal, and surrebuttal phases also allow evidence submission.  The current opportunity returned by `wait_for_opportunity` or `get_current_opportunity` states the court actions allowed for that turn.

Lawyers should cite only AAR `evidence_id` values in `offered_evidence`.  If a lawyer finds a public source with OpenClaw web, browser, file, shell, OCR, PDF, image, audio, video, metadata, hash, signature, archive, or local analysis tools, the lawyer must submit that source through AAR evidence tools before relying on it as record support.  Work notes are private operator diagnostics and are not evidence.

## Build

Run the build from `arb/`:

```bash
make build
```

This builds `.bin/aar`, `.bin/aar-mcp`, and the Lean engine used by the runtime.  The split MCP adapter commands are no longer part of the supported build.

## Running One Example

Use the example runner for local end-to-end tests:

```bash
examples/run-ex.sh ex1
```

The script starts `aar service`, starts `aar-mcp`, creates one case through `POST /api/v1/cases`, starts OpenClaw containers for plaintiff, defendant, and council members `C1` through `C5`, waits for the service result endpoint, and writes the output directory path to standard output.  Each container keeps one OpenClaw session key for the whole assignment.  The container reruns `openclaw agent` in that same session when an invocation ends after a bounded wait, because an OpenClaw agent invocation may finish even though the case has no ready opportunity.  The latest output path for an example is also written to `out/latest-exN-openclaw-lawyers.txt`.

The script sources provider keys from `~/keys.txt` and passes exported provider environment variables into OpenClaw containers.  It uses `gpt-5.5` for OpenClaw agents.  It does not mount case files or output directories into the containers.

## Manual Service Commands

The example runner uses the same commands an operator would run by hand.  The service starts first:

```bash
OUT=out/ex1-service-openclaw-$(date -u +%Y%m%d%H%M%S)
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
.bin/aar-mcp \
  --listen 0.0.0.0:19780 \
  --lawyerapi-base http://127.0.0.1:19770/lawyerapi/v1 \
  --councilapi-base http://127.0.0.1:19770/councilapi/v1 \
  --bearer-token "$TOKEN" \
  --api-bearer-token "$TOKEN" \
  --session-ttl 0 \
  >"$OUT/logs/aar-mcp.stdout" \
  2>"$OUT/logs/aar-mcp.stderr" &
echo $! >"$OUT/aar-mcp.pid"
```

Create a case through the service.  The `case_id` becomes the public routing key for every lawyer, observer, and council call:

```bash
curl -fsS \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "case_id": "arb-ex1-local",
    "complaint_path": "examples/ex1/complaint.md",
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
  "url": "http://host.docker.internal:19780/mcp?case_id=arb-ex1-local&role_id=plaintiff",
  "transport": "streamable-http",
  "headers": {
    "Authorization": "Bearer aar-local-test-token"
  }
}
```

Council members use the same MCP endpoint with `member_id` instead of `role_id`:

```json
{
  "url": "http://host.docker.internal:19780/mcp?case_id=arb-ex1-local&member_id=C1",
  "transport": "streamable-http",
  "headers": {
    "Authorization": "Bearer aar-local-test-token"
  }
}
```

## Assignment Behavior

A lawyer assignment tells OpenClaw to call `wait_for_opportunity` first and to keep calling it while the response is `state: "waiting"`.  When the response is `state: "ready"`, the lawyer reads the prompt, allowed operations, limits, remaining time, attempts remaining, and `opportunity_id`, then completes that opportunity through the AAR tools.  After a successful filing, the lawyer returns to `wait_for_opportunity` for the next turn.

The lawyer should inspect the record at each opportunity.  `get_case`, `list_evidence`, `stat_evidence`, and `read_evidence_range` provide case-packet files and admitted evidence.  `send_work_notes` records private plans, work logs, search logs, source checks, scripts, installed programs, OCR or extraction work, browser work, errors, analysis, decisions, and unresolved gaps for operator review outside the record.

A council assignment follows the same wait loop, but council tools are read-only except for `submit_council_vote`.  Council members review the record and admitted evidence, then vote `demonstrated` or `not_demonstrated` with a rationale grounded in the record.  Council members should not search the web or add evidence.

## Review

The service result endpoint returns pending status while the case runs and final vote details after completion:

```bash
curl -fsS \
  -H "Authorization: Bearer $TOKEN" \
  http://127.0.0.1:19770/api/v1/cases/arb-ex1-local/result
```

The output directory contains the AAR record and process logs.  Review `case/digest.md`, `case/transcript.md`, `case/events.ndjson`, `case/work-notes.ndjson`, `case/evidence-manifest.json`, `case/run.json`, and `logs/openclaw-*.stdout`.  Evidence review should check whether lawyers read case-packet evidence, admitted outside sources before relying on them, used `evidence_id` values in filings, and explained what the evidence proved.

## Cleanup

The example runner stops service, MCP, and leftover OpenClaw containers when the script exits.  For manual runs, stop the host processes by using the PID files, then stop any remaining containers by name.  Do not delete the output directory before review because it contains the record, work notes, service logs, MCP logs, and OpenClaw logs needed to diagnose the run.
