# Remote OpenClaw Lawyer Runbook

## Purpose

This runbook explains how to run an AAR case when one lawyer is an independently running OpenClaw on another machine.  The local AAR process owns the case, starts the MCP service, starts the local lawyer container for the other side, starts Pi council members, and writes a role-specific instruction file for the remote OpenClaw.  The remote OpenClaw joins the case through MCP, waits for its turns, reads evidence through AAR tools, sends private work notes, submits evidence when the phase allows it, and files each legal act.

The current `aar run` interface supports one remote lawyer at a time.  Use `--auto-lawyers defendant` when the remote OpenClaw will act as plaintiff; AAR starts the defendant lawyer locally.  Use `--auto-lawyers plaintiff` when the remote OpenClaw will act as defendant; AAR starts the plaintiff lawyer locally.

## Prerequisites

Run these steps from `arb/`.  Build the CLI before starting the case:

```bash
go build -o .bin/aar ./runtime/cmd/aar
```

The local machine needs Docker for the local OpenClaw lawyer container, Podman for Pi council agents, a valid council pool at `pool.jsonl`, and persona files under `personas/`.  It also needs `OPENROUTER_API_KEY` in the environment because Pi council members use OpenRouter from the pool entries.  The local OpenClaw lawyer can use either `OPENAI_API_KEY` or a Codex auth file; the preferred path is `--openclaw-auth codex --openclaw-codex-auth "$CODEX_AUTH_JSON"`.

The remote OpenClaw must be able to reach the AAR MCP service over HTTP.  The URL in the generated skill must be reachable from the remote OpenClaw process, not just from the operator's terminal.  If the remote OpenClaw is restricted to localhost, run a TCP forward on that remote machine and set `--mcp-public-base-url` to the forwarded localhost URL.

The generated skill contains a bearer token for the case role.  Give it only to the OpenClaw that should act as that lawyer.  Stopping the run ends that token's usefulness because the case MCP service exits with the run.

## Network Setup

Choose one TCP port for the AAR MCP service.  The examples use `8001` on the AAR machine and `9001` as an optional localhost forward on the remote machine.  The MCP listener should bind to all interfaces when a remote machine will connect:

```bash
--mcp-listen 0.0.0.0:8001
```

If the remote OpenClaw can reach the AAR host directly, use the AAR host address as the public base URL:

```bash
AAR_HOST=aar-host.example
--mcp-public-base-url "http://${AAR_HOST}:8001"
```

If AAR runs inside a Linux guest and the remote computer reaches the Mac host instead, forward the host port to the guest.  On the host, set `GUEST_HOST` to the Linux guest address:

```bash
GUEST_HOST=aar-guest.example
socat TCP-LISTEN:8001,bind=0.0.0.0,fork,reuseaddr "TCP:${GUEST_HOST}:8001"
```

If the remote OpenClaw can use only localhost, forward a local port on the remote computer to the reachable host.  On the remote computer, set `FORWARD_HOST` to the host that forwards to AAR:

```bash
FORWARD_HOST=aar-forward.example
socat TCP-LISTEN:9001,bind=127.0.0.1,fork,reuseaddr "TCP:${FORWARD_HOST}:8001"
```

With that remote-localhost bridge, start `aar run` with:

```bash
--mcp-public-base-url http://127.0.0.1:9001
```

After `aar run` starts, test the public MCP base from the remote computer.  A healthy MCP service returns HTTP `204` for `/health`:

```bash
curl -i http://127.0.0.1:9001/health
```

For a direct connection, replace `127.0.0.1:9001` with the direct AAR host and port.  If this check fails, fix the network path before asking the remote OpenClaw to join the case.  The remote OpenClaw cannot act unless it can reach the MCP service URL embedded in the generated skill.

## Start The Case

This example runs `ex01` with a remote plaintiff and a local defendant.  It starts MCP on port `8001`, tells the generated plaintiff skill to use `http://127.0.0.1:9001`, uses Codex auth for the local OpenClaw lawyer, and uses `pool.jsonl` for the Pi council:

```bash
KEYS_FILE=keys.txt
CODEX_AUTH_JSON=auth.json

set -a
. "$KEYS_FILE"
set +a

go build -o .bin/aar ./runtime/cmd/aar

out="out/ex01-remote-plaintiff-$(date -u +%Y%m%d%H%M%S)"
mkdir -p "$out"

.bin/aar run \
  --out-dir "$out" \
  --auto-lawyers defendant \
  --mcp-listen 0.0.0.0:8001 \
  --mcp-public-base-url http://127.0.0.1:9001 \
  --openclaw-auth codex \
  --openclaw-codex-auth "$CODEX_AUTH_JSON" \
  --council-pool pool.jsonl \
  ex01 \
  >"$out/aar-run.log" 2>&1
```

This command runs until the case finishes or fails.  Leave it running.  Use another terminal to inspect `"$out/aar-run.log"` and copy the generated skill file.

For a remote defendant, change `--auto-lawyers defendant` to `--auto-lawyers plaintiff`.  The local AAR process then starts the plaintiff lawyer and writes a defendant skill file.  Use an output directory name that records the example, role, and time, because the run packet and generated skill live there.

## Give The Skill To The Remote OpenClaw

When `aar run` starts in remote-plaintiff mode, it writes:

```text
$out/openclaw-plaintiff-lawyer-skill.md
```

When `aar run` starts in remote-defendant mode, it writes:

```text
$out/openclaw-defendant-lawyer-skill.md
```

Give the whole file to the remote OpenClaw after `aar-run.log` reports that the remote lawyer skill was written.  A direct instruction is enough:

```text
Use the attached AAR remote lawyer instructions to act as plaintiff lawyer in this case. Follow the instructions exactly. Configure the MCP server, call wait_for_opportunity, and keep working until wait_for_opportunity reports done, failed, or error.
```

Use `defendant` instead of `plaintiff` for a remote defendant.  The generated skill already contains the case id, role id, MCP URL, bearer token, server name, and the exact `openclaw mcp set` command.  Do not edit those values by hand unless the public URL is wrong.

The remote OpenClaw should configure the MCP server and then enter the work loop.  It should call `wait_for_opportunity`, read the prompt and evidence, send `send_work_notes` before each filing, and submit the legal action through `submit_decision`.  It should not create a cron job, listen for inbound HTTP, or ask the operator for each turn.

## Monitor The Run

The main progress log is:

```bash
tail -f "$out/logs/mcp.stderr"
```

This log records MCP sessions and AAR tool calls.  A remote lawyer connection appears as `mcp_session_created` with `assignment_type=lawyer` and the expected `principal`, such as `principal=plaintiff`.  Successful lawyer actions appear as `lawyerapi_do ... http_status=200 ok=true`.

The case event log is:

```bash
tail -f "$out/events.ndjson"
```

This log records accepted evidence reads, lawyer filings, council evidence reads, and council votes.  The work-product notes sent by the lawyers are in:

```bash
jq -r '[.timestamp,.role,.phase,(.notes|length)] | @tsv' "$out/work-notes.ndjson"
```

After completion, read the final result:

```bash
jq '{status, phase, resolution, final_reason, started_at, finished_at, council_votes: .final_state.case.council_votes}' "$out/run.json"
```

The final packet also includes `transcript.md`, `digest.md`, `state.json`, `council.json`, `evidence-manifest.json`, and exact evidence bytes under `evidence-store/`.

## Expected Behavior

The remote OpenClaw may open more than one MCP session for the same case and role.  That is acceptable because MCP sessions do not own case state; the case state belongs to AAR.  A lost MCP session can be replaced by reconnecting with the same generated server configuration.

The local `aar run` process starts one local OpenClaw lawyer container for the non-remote side.  It starts Pi council agents when the case reaches deliberation.  The process stops those child processes when the case finishes or fails.

The remote lawyer receives all case files through AAR evidence tools.  It does not need local filesystem mounts.  It should scan the evidence list at each opportunity because new evidence and filings can appear as the case progresses.

When `aar run` reaches a final result, it writes the final run packet and exits.  The MCP service exits with it.  A remote OpenClaw that polls after that point may report that it cannot reach the MCP or health endpoint and may show the last status it saw before shutdown.  That message means the remote OpenClaw lost the case service after completion; it does not by itself mean that the case failed.  Read `run.json` in the run output directory for the final result.

## Troubleshooting

If the remote OpenClaw cannot connect, test `/health` from the same machine and network context that OpenClaw uses.  Terminal curl success does not prove that the OpenClaw process has the same network access.  Use a localhost forward on the remote machine and set `--mcp-public-base-url` to that local forwarded URL when OpenClaw can reach localhost but cannot reach the LAN address.

If `aar run` reports that the manual lawyer address is invalid, check that `--mcp-public-base-url` is set when `--mcp-listen` uses `0.0.0.0`.  The listen address is where AAR accepts traffic; the public base URL is what the remote OpenClaw uses.  They are often different when NAT, a VM, or a local forward is involved.

If the remote lawyer is slow, check the active opportunity deadline in `logs/mcp.stderr` or through the MCP `wait_for_opportunity` response.  A slow turn is acceptable if it stays inside the deadline and continues making valid tool calls.  If the lawyer turn times out, AAR treats lawyer failure as case failure.

If the council phase appears slow, check for `assignment_type=council` sessions and `councilapi_do` calls in `logs/mcp.stderr`.  Pi council members are separate from the remote lawyer, and the case can continue after all lawyer filings have completed.  The final `run.json` is written only after the case reaches a terminal state.

If the remote OpenClaw says the last known status was deliberation active and the MCP health endpoint no longer responds, check whether `aar run` has already exited and written `run.json`.  In the successful `ex01` reference run, the remote lawyer last saw the case during deliberation; after the council completed, `aar run` shut down the MCP service, so the remote OpenClaw could not retrieve the final result through MCP.  The final result was still available in the local run packet.

If a local child process remains after completion, inspect the PID files and container names:

```bash
for f in "$out"/openclaw-*.pid "$out"/pi-C*.pid; do
  [ -e "$f" ] || continue
  pid="$(cat "$f")"
  printf '%s ' "$f"
  ps -p "$pid" -o pid=,stat=,cmd= || true
done

docker ps --format '{{.Names}}' | grep "aar-" || true
podman ps --format '{{.Names}}' | grep "pi-" || true
```

If an old process is still running, diagnose why before stopping it.  A completed successful run should leave final output files and no live child process for that case.

## Successful ex01 Reference

The tested `ex01` remote-plaintiff run completed from `2026-06-03T14:58:08Z` to `2026-06-03T15:15:09Z`.  The remote plaintiff connected through MCP, read all 11 case-packet evidence items during opening, sent work notes for every plaintiff turn, verified the confession signature, filed opening, argument, rebuttal, and closing, and stayed inside the lawyer deadlines.  The local defendant completed all opposing lawyer turns.

The Pi council voted 5-0 for `demonstrated`.  The final result was `status: ok`, `phase: closed`, `resolution: demonstrated`, and `final_reason: demonstrated`.  The narrow failure check found no failed API calls, rejected submissions, exhausted attempts, or process-failure messages.
