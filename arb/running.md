# Running AAR With OpenClaw Lawyers

## Purpose

This runbook describes how to run one AAR case with real OpenClaw containers acting as the plaintiff and defendant lawyers.  Each lawyer gets one OpenClaw container for the whole case.  The container receives one assignment prompt, then works all of that role's turns by calling the AAR MCP tool `wait_for_opportunity`.

The AAR process owns the case, deadlines, evidence store, turn order, and final result.  The Lawyer MCP adapter translates OpenClaw MCP calls into the Lawyer HTTP API exposed by `aar case`.  OpenClaw owns lawyer reasoning, source search, evidence selection, legal filings, and the loop that waits for later opportunities.

## Topology

| Process | Runs where | Role |
| --- | --- | --- |
| `aar case` | host | Runs the arbitration and exposes Lawyer API on `127.0.0.1`. |
| `aar-lawyer-mcp` | host | Exposes Streamable HTTP MCP on a host port reachable from Docker. |
| OpenClaw plaintiff container | Docker | Acts as plaintiff lawyer for all plaintiff turns. |
| OpenClaw defendant container | Docker | Acts as defendant lawyer for all defendant turns. |
| Council backend | host | Votes after lawyer phases.  Use `direct` when testing lawyer behavior. |

The container talks to the host MCP adapter through `host.docker.internal`.  On Linux, Docker needs `--add-host=host.docker.internal:host-gateway`.  The MCP adapter listens on `0.0.0.0` so Docker can reach it, while the Lawyer API can remain on host loopback because only the adapter calls it.

The lawyer containers must be treated as remote lawyers.  Do not mount the source example directory, the run output directory, or any case-packet directory into a lawyer container.  The lawyer's access to case files comes only from AAR tools exposed through MCP.  This matches a real remote clawyer, which receives an MCP endpoint and token but cannot read the operator's filesystem.

## Case File Access

`aar case` imports the complaint directory into the AAR record.  Files in the case packet become immutable `case_packet` evidence with an `evidence_id`, hash, size, MIME type, title, and storage record.  A lawyer sees those files only through the Lawyer API tools that AAR exposes for the current turn.

The file-access tools are:

| Tool | Purpose |
| --- | --- |
| `get_case` | Returns the visible arbitration record. |
| `send_work_notes` | Sends private lawyer work notes to `work-notes.ndjson` for off-record review. |
| `list_evidence` | Lists visible evidence metadata, including case-packet files and accepted submitted evidence. |
| `stat_evidence` | Returns metadata and read limits for one visible `evidence_id`. |
| `read_evidence_range` | Reads a bounded byte range from one visible evidence item as base64. |
| `submit_evidence` | Admits small source material found by a lawyer, with provenance, into the AAR record. |
| `begin_evidence_upload`, `write_evidence_chunk`, `commit_evidence_upload` | Admit larger source material through a chunked upload. |

Every lawyer phase exposes read-only evidence tools.  Argument, rebuttal, and surrebuttal opportunities also expose evidence-submission tools.  The tool list returned by `wait_for_opportunity` or `get_current_opportunity` is authoritative for that turn.

The lawyer should cite only AAR `evidence_id` values in `offered_evidence`.  AAR tools govern court evidence and filings, not the lawyer's full investigation toolbox.  If a lawyer finds a public source with native OpenClaw web, browser, file, shell, OCR, PDF, image, audio, video, metadata, hash, signature, archive, or local analysis tools, the lawyer must submit that source through AAR evidence tools before relying on it as record support.

## Inputs

For `ex1`, the complaint is `examples/ex1/complaint.md`.  The output directory should be a fresh directory under `out/`, for example `out/ex1-openclaw-lawyers-20260602T020000Z`.  The OpenClaw container needs provider keys in its environment; in this workspace those keys usually come from `~/keys.txt`.

The examples below use:

| Name | Example value |
| --- | --- |
| Case id | `arb-1` |
| Lawyer API | `127.0.0.1:19771` |
| Lawyer MCP | `0.0.0.0:19780` on the host, `host.docker.internal:19780` from Docker |
| MCP token | `aar-local-test-token` |
| Model | `gpt-5.5` |

Use different ports if those ports are already bound.  Use a fresh token per run when the adapter listens on a reachable interface.  Keep the token out of filings, logs intended for publication, and notes that will be shared outside the operator environment.

## Build

Run the build from `arb/`:

```bash
make build
```

This builds `.bin/aar`, `.bin/aar-lawyer-mcp`, `.bin/aar-council-mcp`, and the Lean engine used by the runtime.

## Start The Case

Create the output directories for host logs and the AAR record.  Run the host-side commands from one shell so `OUT` and `TOKEN` stay available; if another terminal starts a process, export the same variables there.

```bash
export OUT=out/ex1-openclaw-lawyers-$(date -u +%Y%m%dT%H%M%SZ)
export TOKEN=aar-local-test-token
mkdir -p "$OUT/logs"
```

Start `aar case` with a fixed Lawyer API address.  For a lawyer test, use direct council so the review focuses on the OpenClaw lawyers' filings and evidence work.  Source the provider keys before starting `aar case`; the direct council backend uses the host xproxy process, so the host process needs the council provider keys as well as the OpenClaw containers.

```bash
set +x
source "$HOME/keys.txt"
export OPENAI_API_KEY OPENROUTER_API_KEY ANTHROPIC_API_KEY GEMINI_API_KEY

.bin/aar case \
  --complaint examples/ex1/complaint.md \
  --out-dir "$OUT/case" \
  --lawyerapi-addr 127.0.0.1:19771 \
  --council-backend direct \
  --lawyer-timeout-seconds 900 \
  >"$OUT/logs/aar.stdout" \
  2>"$OUT/logs/aar.stderr" &
echo $! >"$OUT/aar.pid"
```

Wait until the Lawyer API port is listening before starting the MCP adapter.  A direct check is enough:

```bash
curl -fsS 'http://127.0.0.1:19771/lawyerapi/v1/get?case_id=arb-1&role_id=observer' >"$OUT/logs/observer-start.json"
```

## Start The Lawyer MCP Adapter

Start one adapter for both lawyer roles.  The adapter can serve several case-role MCP sessions.  In this single-case run, the default Lawyer API base is enough.

```bash
.bin/aar-lawyer-mcp \
  --listen 0.0.0.0:19780 \
  --lawyerapi-base http://127.0.0.1:19771/lawyerapi/v1 \
  --bearer-token "$TOKEN" \
  --session-ttl 0 \
  >"$OUT/logs/lawyer-mcp.stdout" \
  2>"$OUT/logs/lawyer-mcp.stderr" &
echo $! >"$OUT/lawyer-mcp.pid"
```

Check the adapter through raw MCP only when debugging adapter reachability.  The normal OpenClaw path uses `openclaw mcp set` inside each container and then lets the agent call the MCP tools.

## Prepare The Assignment Text

```bash
read -r -d '' PLAINTIFF_ASSIGNMENT <<'EOF'
You are the plaintiff lawyer for AAR case arb-1. Use MCP server aar-arb-1-plaintiff. Work only through the AAR MCP tools for court filings.

Call wait_for_opportunity first. If it returns state waiting, call wait_for_opportunity again with after_version. If it returns state ready, read the returned prompt, turn, tools, limits, remaining time, attempts remaining, and opportunity id. Complete exactly that opportunity. Use get_case and scan the evidence list for new case-packet files, newly submitted evidence, or changed metadata before filing. Use stat_evidence and read_evidence_range when exact contents matter. Analyze what the relevant evidence proves, what it does not prove, and whether provenance, custody, conflict, or missing links affect weight.

The AAR MCP tool list controls court actions, not your full investigation toolbox. Keep private notes throughout the turn as a working journal: objective, issue breakdown, plan, work log, search log, source URLs or identifiers, tools used, scripts or programs written, packages installed, OCR or extraction steps, browser work, adverse checks, errors, reasoning, draft theory, decisions, and unresolved gaps. Call send_work_notes with the accumulated notes before submit_decision. If the existing record leaves a material gap, use all accessible and available resources that can find or test material evidence: native OpenClaw web, browser, file, shell, OCR, PDF, image, audio, video, metadata, hash, signature, archive, and local analysis tools. If the environment permits it, install useful programs, write and run scripts or small programs, download source artifacts, and use a browser for dynamic pages or visual inspection. Follow search results to source pages or artifacts before relying on them. Check adverse sources, conflicting primary material, later corrections, missing context, and source-chain breaks. Do not use credentials, paid services, private accounts, or privileged sources unless the operator explicitly provides them for this case. When evidence-submission tools are available, submit source material through AAR before relying on it in a filing. Use AAR evidence_id values for offered evidence. Do not cite a URL, filename, or your own notes as admitted evidence unless AAR has accepted the source and returned an evidence_id.

Submit the legal act through submit_decision. If submit_decision succeeds, return to wait_for_opportunity for the next plaintiff opportunity.

If wait_for_opportunity returns state done, report that the case is done and stop. If it returns state error, report the error and stop. Do not ask the user for the next turn. Do not create a cron job. Do not listen for inbound HTTP.
EOF
```

```bash
read -r -d '' DEFENDANT_ASSIGNMENT <<'EOF'
You are the defendant lawyer for AAR case arb-1. Use MCP server aar-arb-1-defendant. Work only through the AAR MCP tools for court filings.

Call wait_for_opportunity first. If it returns state waiting, call wait_for_opportunity again with after_version. If it returns state ready, read the returned prompt, turn, tools, limits, remaining time, attempts remaining, and opportunity id. Complete exactly that opportunity. Use get_case and scan the evidence list for new case-packet files, newly submitted evidence, or changed metadata before filing. Use stat_evidence and read_evidence_range when exact contents matter. Analyze what the relevant evidence proves, what it does not prove, and whether provenance, custody, conflict, or missing links affect weight.

The AAR MCP tool list controls court actions, not your full investigation toolbox. Keep private notes throughout the turn as a working journal: objective, issue breakdown, plan, work log, search log, source URLs or identifiers, tools used, scripts or programs written, packages installed, OCR or extraction steps, browser work, adverse checks, errors, reasoning, draft theory, decisions, and unresolved gaps. Call send_work_notes with the accumulated notes before submit_decision. If the existing record leaves a material gap, use all accessible and available resources that can find or test material evidence: native OpenClaw web, browser, file, shell, OCR, PDF, image, audio, video, metadata, hash, signature, archive, and local analysis tools. If the environment permits it, install useful programs, write and run scripts or small programs, download source artifacts, and use a browser for dynamic pages or visual inspection. Follow search results to source pages or artifacts before relying on them. Check adverse sources, conflicting primary material, later corrections, missing context, and source-chain breaks. Do not use credentials, paid services, private accounts, or privileged sources unless the operator explicitly provides them for this case. When evidence-submission tools are available, submit source material through AAR before relying on it in a filing. Use AAR evidence_id values for offered evidence. Do not cite a URL, filename, or your own notes as admitted evidence unless AAR has accepted the source and returned an evidence_id.

Submit the legal act through submit_decision. If submit_decision succeeds, return to wait_for_opportunity for the next defendant opportunity.

If wait_for_opportunity returns state done, report that the case is done and stop. If it returns state error, report the error and stop. Do not ask the user for the next turn. Do not create a cron job. Do not listen for inbound HTTP.
EOF
```

## Start The Plaintiff Lawyer

Source the provider keys in the shell that starts Docker.  The values must be exported or passed through with `-e`.

```bash
set +x
source "$HOME/keys.txt"
export OPENAI_API_KEY OPENROUTER_API_KEY ANTHROPIC_API_KEY GEMINI_API_KEY
```

Start the plaintiff container.  The command does not mount the case packet, source tree, output directory, prompt file, or OpenClaw home.  The MCP config and assignment enter the container as command data, which stands in for what a remote clawyer would receive from the operator.

```bash
PLAINTIFF_MCP_JSON='{"url":"http://host.docker.internal:19780/mcp?case_id=arb-1&role_id=plaintiff","transport":"streamable-http","headers":{"Authorization":"Bearer '"$TOKEN"'"}}'

docker run --rm \
  --name aar-arb-1-plaintiff \
  --add-host=host.docker.internal:host-gateway \
  -e OPENAI_API_KEY \
  -e OPENROUTER_API_KEY \
  -e ANTHROPIC_API_KEY \
  -e GEMINI_API_KEY \
  -e AAR_MCP_JSON="$PLAINTIFF_MCP_JSON" \
  -e AAR_ASSIGNMENT="$PLAINTIFF_ASSIGNMENT" \
  ghcr.io/openclaw/openclaw:latest \
  sh -lc '
    openclaw mcp set aar-arb-1-plaintiff "$AAR_MCP_JSON"
    openclaw agent --local --model gpt-5.5 --thinking low --timeout 1800 --session-key agent:aar:arb-1:plaintiff --message "$AAR_ASSIGNMENT" --json
  ' >"$OUT/logs/openclaw-plaintiff.stdout" 2>"$OUT/logs/openclaw-plaintiff.stderr" &
echo $! >"$OUT/openclaw-plaintiff.pid"
```

## Start The Defendant Lawyer

Start the defendant container in the same way, with a different MCP server name, role id, and session key.

```bash
DEFENDANT_MCP_JSON='{"url":"http://host.docker.internal:19780/mcp?case_id=arb-1&role_id=defendant","transport":"streamable-http","headers":{"Authorization":"Bearer '"$TOKEN"'"}}'

docker run --rm \
  --name aar-arb-1-defendant \
  --add-host=host.docker.internal:host-gateway \
  -e OPENAI_API_KEY \
  -e OPENROUTER_API_KEY \
  -e ANTHROPIC_API_KEY \
  -e GEMINI_API_KEY \
  -e AAR_MCP_JSON="$DEFENDANT_MCP_JSON" \
  -e AAR_ASSIGNMENT="$DEFENDANT_ASSIGNMENT" \
  ghcr.io/openclaw/openclaw:latest \
  sh -lc '
    openclaw mcp set aar-arb-1-defendant "$AAR_MCP_JSON"
    openclaw agent --local --model gpt-5.5 --thinking low --timeout 1800 --session-key agent:aar:arb-1:defendant --message "$AAR_ASSIGNMENT" --json
  ' >"$OUT/logs/openclaw-defendant.stdout" 2>"$OUT/logs/openclaw-defendant.stderr" &
echo $! >"$OUT/openclaw-defendant.pid"
```

At this point, the operator should not feed turns to the lawyers.  A lawyer container handles every opportunity for its role by repeating `wait_for_opportunity`.  The AAR process advances when each lawyer submits a valid legal act through `submit_decision`.

## Wait For Completion

Watch the case process, the MCP adapter log, and the two OpenClaw logs.  The AAR process exits when the case closes or fails.

```bash
tail -f "$OUT/logs/aar.stderr" "$OUT/logs/lawyer-mcp.stderr" "$OUT/logs/openclaw-plaintiff.stderr" "$OUT/logs/openclaw-defendant.stderr"
```

Check the result after `aar case` exits:

```bash
cat "$OUT/logs/aar.stdout"
sed -n '1,260p' "$OUT/case/digest.md"
```

The expected final artifacts are in `$OUT/case/`.  The files to review first are `digest.md`, `transcript.md`, `events.ndjson`, `work-notes.ndjson`, `evidence-manifest.json`, and `state.json`.  The OpenClaw stdout files contain each agent turn's final response and metadata; the AAR record remains the source for accepted filings and admitted evidence.

## Review Checklist

Review whether the plaintiff and defendant actually acted through OpenClaw by checking `events.ndjson` for lawyer tool calls and MCP logs for forwarded calls.  Review whether the lawyers inspected the record before filing and whether argument phases offer admitted `evidence_id` values.  For examples that require outside research, review whether source material was submitted through AAR evidence tools before the filings relied on it.

For `ex1`, outside web research is usually unnecessary because the case packet contains the relevant proof.  A good run should show lawyers reading and citing the local case-packet evidence rather than inventing facts or submitting irrelevant source material.  The record should distinguish case-packet evidence, attorney analysis, and legal argument.

For open-record examples such as `ex5`, the same procedure should show real lawyer-side searching.  The evidence-manifest should contain submitted source material with title, provenance, retrieval timestamp, source URL when available, relevance, and accepted `evidence_id` values.  A filing that argues from web material without first admitting that material is a failed lawyer performance even if the final vote is plausible.

## Cleanup

If the run completes, the OpenClaw containers should exit after `wait_for_opportunity` returns `state: done`.  Stop leftover processes by using the PIDs written into `$OUT`.

```bash
kill "$(cat "$OUT/aar.pid")" "$(cat "$OUT/lawyer-mcp.pid")" 2>/dev/null || true
docker stop aar-arb-1-plaintiff aar-arb-1-defendant 2>/dev/null || true
```

Do not delete the output directory before review.  It contains the AAR record and logs needed to diagnose lawyer behavior.  It should not contain host-mounted lawyer workspaces because the lawyer containers should not receive host filesystem access.
