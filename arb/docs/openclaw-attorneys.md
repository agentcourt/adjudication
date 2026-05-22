# OpenClaw Attorneys

This note describes how to run `arb` with OpenClaw agents as plaintiff and defendant attorneys.

## Architecture

`aar case` talks to attorneys through ACP. The normal local path starts a Pi ACP wrapper. The OpenClaw path replaces that attorney process with an OpenClaw-backed ACP server.

There are two pieces:

1. `.bin/aar-openclaw-attorney` is a stdio ACP adapter. It receives AAR `session/prompt` requests, asks AAR for the visible case through `_aar/get_case`, inspects visible artifacts through the AAR artifact methods, asks an OpenClaw agent for one filing JSON, and submits that filing through `_aar/submit_decision`.
2. `tools/openclaw-acp-tcp-bridge.js` exposes that stdio adapter as a TCP ACP endpoint. `aar case` connects to this endpoint with `--plaintiff-acp-endpoint` and `--defendant-acp-endpoint`.

The bridge starts a new `.bin/aar-openclaw-attorney` process for each ACP connection. One bridge process can serve both sides of a run.

## Capability boundary

AAR does not select the OpenClaw model. A role using `--*-acp-endpoint` cannot also use `--*-attorney-model`. That flag belongs to the local Pi/xproxy attorney path.

For OpenClaw attorneys:

- model selection belongs to the OpenClaw agent configuration or the OpenClaw runtime invoked by `openclaw agent`;
- tool availability belongs to that OpenClaw environment;
- `run.json` records the ACP endpoint, not an AAR-selected attorney model;
- the bridge deliberately removes `AAR_OPENCLAW_AGENT_MODEL` from the adapter environment.

This matters for reproducibility. A closed-record run is reproduced by using an OpenClaw attorney agent that does not search, or by instructing it to stay within the provided record. An open-record run is reproduced by using an OpenClaw attorney agent with search, browser, fetch, or equivalent tools available, and by giving explicit open-record instructions through `AAR_OPENCLAW_AGENT_EXTRA_PROMPT`.

## Prerequisites

From `arb/`:

```bash
make build
node --check tools/openclaw-acp-tcp-bridge.js
```

The run also needs:

- `.bin/aar` and `.bin/aarengine`, built by `make build`;
- `.bin/aar-openclaw-attorney`, built by `make build`;
- an OpenClaw CLI on `PATH`, or `AAR_OPENCLAW_CLI` set to the CLI path;
- a dedicated OpenClaw lawyer agent, set with `AAR_OPENCLAW_AGENT_ID`;
- provider credentials required by the OpenClaw lawyer agent;
- provider credentials required by AAR for council models.

Do not use a personal default OpenClaw agent. Set `AAR_OPENCLAW_AGENT_ID` explicitly so arbitration work runs in the intended lawyer context.

## Closed-record run

This run lets OpenClaw attorneys argue from the packet that AAR provides. It is the reproducible baseline when the case directory is self-contained.

Terminal 1, start the bridge:

```bash
cd arb
AAR_OPENCLAW_AGENT_ID=aar-lawyer \
AAR_OPENCLAW_ATTORNEY_TIMEOUT_SECONDS=900 \
tools/openclaw-acp-tcp-bridge.js --host 127.0.0.1 --port 19701
```

Terminal 2, prepare and run the case:

```bash
cd arb

case_dir=examples/ex1
out_dir=out/ex1-openclaw-closed-$(date +%Y%m%d-%H%M%S)

.bin/aar complain \
  --situation "$case_dir/situation.md" \
  --out "$case_dir/complaint.md"

.bin/aar case \
  --complaint "$case_dir/complaint.md" \
  --out-dir "$out_dir" \
  --plaintiff-acp-endpoint tcp://127.0.0.1:19701 \
  --defendant-acp-endpoint tcp://127.0.0.1:19701 \
  --council-size 3 \
  --acp-timeout-seconds 900 \
  --invalid-attempt-limit 5
```

Expected output is a single JSON object on stdout:

```json
{"status":"ok","result":"...","votes_for":0,"votes_against":3,"run_id":"run-...","out_dir":"out/ex1-openclaw-closed-..."}
```

## Open-record run

An open-record run uses the same ACP bridge, but gives the OpenClaw attorneys an extra instruction to investigate public sources. The OpenClaw agent must have the relevant tools available. AAR itself does not enable those tools.

Terminal 1, start the bridge with an open-record instruction:

```bash
cd arb
export AAR_OPENCLAW_AGENT_ID=aar-lawyer
export AAR_OPENCLAW_ATTORNEY_TIMEOUT_SECONDS=1200
export AAR_OPENCLAW_AGENT_EXTRA_PROMPT='This is an open-record arbitration. Use available public search, web fetch, browser, transcript, or equivalent tools when they can materially improve the filing. Prefer primary sources over commentary. Preserve URLs, direct excerpts, and uncertainty. If external source material matters, submit the source content and provenance through aar_submit_artifact and cite the returned artifact_id in offered_artifacts. Use technical_reports for attorney analysis or synthesized work product.'

tools/openclaw-acp-tcp-bridge.js --host 127.0.0.1 --port 19702
```

Terminal 2, run the same case against that endpoint:

```bash
cd arb

case_dir=examples/ex1
ts=$(date +%Y%m%d-%H%M%S)
out_dir=out/ex1-openclaw-open-$ts
batch_dir=out/_batch-ex1-openclaw-open-$ts
mkdir -p "$batch_dir/logs"

.bin/aar complain \
  --situation "$case_dir/situation.md" \
  --out "$case_dir/complaint.md"

.bin/aar case \
  --complaint "$case_dir/complaint.md" \
  --out-dir "$out_dir" \
  --plaintiff-acp-endpoint tcp://127.0.0.1:19702 \
  --defendant-acp-endpoint tcp://127.0.0.1:19702 \
  --council-size 3 \
  --acp-timeout-seconds 1200 \
  --invalid-attempt-limit 5 \
  2>&1 | tee "$batch_dir/logs/run.log"

cp "$batch_dir/logs/run.log" "$out_dir/run.log"
```

Use `bash` for scripts that inspect pipeline status. Do not use zsh-only or bash-only status variables interchangeably after piping through `tee`.

## Evidence submissions

The OpenClaw adapter accepts either an ordinary `aar_submit_decision` JSON object or a structured bundle:

```json
{
  "evidence_submissions": [
    {
      "title": "Source title",
      "source_url": "https://example.test/source",
      "source_description": "Readable extraction retrieved during open-record investigation.",
      "retrieval_timestamp": "2026-01-01T00:00:00Z",
      "mime_type": "text/markdown",
      "relevance": "Why this source matters.",
      "content": "# Source title\n\nQuoted or extracted source material.",
      "offer_label": "DX-1 Source title"
    }
  ],
  "decision": {
    "kind": "tool",
    "tool_name": "submit_argument",
    "payload": {
      "text": "Argument text that cites DX-1.",
      "offered_artifacts": []
    }
  }
}
```

The adapter submits each evidence item through `_aar/submit_artifact` before the merits filing. If the submission succeeds, AAR stores the bytes under `submitted-evidence/`, hashes them with SHA-256, records provenance metadata, registers an immutable artifact, and returns `artifact_id`. The adapter cites accepted evidence in `offered_artifacts` for `submit_argument` and `submit_rebuttal` filings unless `offer_as_exhibit` is false.

Remote attorneys can also inspect and transfer exact bytes through the artifact API exposed by AAR during arguments and rebuttals:

- `aar_list_artifacts`
- `aar_stat_artifact`
- `aar_read_artifact_range`
- `aar_materialize_artifact`
- `aar_begin_artifact_upload`
- `aar_write_artifact_chunk`
- `aar_commit_artifact_upload`

Use chunked upload when source evidence is too large or inappropriate for single-request `aar_submit_artifact`. Uploads are not evidence until `aar_commit_artifact_upload` verifies size and hash, the Lean engine accepts the `submit_artifact` action, and AAR registers the bytes in the artifact store. See [`evidence-handling.md`](evidence-handling.md) for the artifact model, limits, and custody rules.

Evidence submissions are not allowed in closing statements. Closing statements are record-only and may contain only closing text.

## What to inspect after the run

A completed run should contain:

```text
run.json
state.json
artifact-manifest.json
artifact-store/
digest.md
council.json
events.ndjson
transcript.md
run.log
```

Check these before reporting the result:

```bash
jq '.status? // empty' "$out_dir/run.json" 2>/dev/null || true
jq '.result? // empty' "$out_dir/run.json" 2>/dev/null || true
grep -n "Resolution:" "$out_dir/digest.md"
grep -n "Submitted Evidence\|technical report\|technical_reports\|http" "$out_dir/digest.md" | head -40
```

For an open-record run, inspect whether the attorneys obtained material public evidence, preserved provenance, and distinguished source evidence from attorney work product. Source material submitted with `aar_submit_artifact` or `aar_commit_artifact_upload` is copied into `submitted-evidence/`, recorded in `state.json`, registered in `artifact-manifest.json`, and becomes a visible artifact that later filings can cite in `offered_artifacts`. If attorneys instead rely on unsupported claims in prose or technical reports, backfill the case directory before using later closed-record runs as reproducibility evidence.

## Current limitations

- The adapter asks OpenClaw for one strict JSON filing per AAR opportunity. It does not stream intermediate reasoning back to AAR.
- `technical_reports` remain attorney work product. Use them for analysis, not for preserving exact source material when the source content matters.
- AAR endpoint metadata deliberately does not claim OpenClaw search capability. The operator must configure and document the OpenClaw agent environment used for open-record runs.
