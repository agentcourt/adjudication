# OpenClaw Attorneys

This note describes the `arb` side of OpenClaw lawyer runs.  The shared setup and command details are in [OpenClaw ACP MCP Bridge](../../common/docs/openclaw-acp-mcp-bridge.md) and [Common Tools](../../common/tools/README.md).  AAR talks to each OpenClaw lawyer through a TCP ACP endpoint served by `common/tools/acp-mcp-bridge.mjs`.

## Architecture

`aar case` sends each lawyer opportunity to the role's bridge through ACP.  The ACP prompt contains the attorney instructions and `_meta.clientTools`.  The bridge exposes those tools to OpenClaw through its HTTP MCP endpoint, runs `openclaw agent` in the role's OpenClaw session, and forwards MCP tool calls back to AAR ACP methods.

OpenClaw owns the lawyer model, the OpenClaw agent configuration, and any native OpenClaw tools such as search or browser tools.  AAR owns the case record, evidence access, filing validation, invalid-attempt feedback, transcripts, and state transitions.  A role using `--plaintiff-acp-endpoint` or `--defendant-acp-endpoint` cannot also set the matching role-specific attorney model flag, because endpoint attorneys select their own model outside AAR.

## Closed-Record Run

Start one bridge process for plaintiff and one for defendant as described in [Common Tools](../../common/tools/README.md).  Use two OpenClaw config directories and two MCP URLs, for example:

```text
plaintiff ACP:  tcp://127.0.0.1:19711
plaintiff MCP:  http://127.0.0.1:19712/mcp
defendant ACP:  tcp://127.0.0.1:19713
defendant MCP:  http://127.0.0.1:19714/mcp
```

Build AAR, then run a case against the two ACP endpoints:

```bash
cd "$ADJUDICATION_REPO/arb"
make build

.bin/aar case \
  --complaint examples/ex1/complaint.md \
  --out-dir out/ex1-openclaw-closed \
  --plaintiff-acp-endpoint tcp://127.0.0.1:19711 \
  --defendant-acp-endpoint tcp://127.0.0.1:19713 \
  --acp-timeout-seconds 900 \
  --invalid-attempt-limit 5
```

The completed run should contain `run.json`, `state.json`, `events.ndjson`, `transcript.md`, `digest.md`, `council.json`, `policy.json`, and `evidence-manifest.json`.  Check `digest.md` for the final resolution and `events.ndjson` for accepted attorney actions.

## Open-Record Run

An open-record run uses the same bridge.  Configure the OpenClaw lawyer agents with the native tools they need, and provide extra role instructions through `AGENTCOURT_OPENCLAW_EXTRA_PROMPT` when starting each bridge.  AAR does not grant search capability to an endpoint lawyer; it only supplies the ACP client tools for case access, evidence handling, and filing.

For open-record work, the extra prompt should tell the lawyer when outside investigation is allowed, how to preserve source provenance, and when to submit source material through `agentcourt__aar_submit_evidence`.  AAR will accept source-evidence submissions only in phases where the procedure exposes that ACP client tool.  Closing statements remain record-bound.

## Evidence Tools

During arguments and rebuttals, AAR may expose evidence methods through `_meta.clientTools`.  The bridge publishes those methods as MCP tools with the configured server prefix.  The common examples use the prefix `agentcourt__`, so `_aar/list_evidence` becomes `agentcourt__aar_list_evidence` and `_aar/read_evidence_range` becomes `agentcourt__aar_read_evidence_range`.

The lawyer can submit source evidence through `agentcourt__aar_submit_evidence` when AAR exposes that method.  AAR hashes accepted bytes, stores them under the run's evidence directories, records provenance, registers the item in the Lean state, and returns an `evidence_id` for later filings.  See [Evidence Handling](evidence-handling.md) for evidence limits and custody rules.

## Inspection

After an open-record run, inspect whether attorneys preserved source evidence rather than leaving unsupported factual claims in prose.  Source material submitted through AAR evidence tools appears in `submitted-evidence/`, `evidence-store/`, `evidence-manifest.json`, `state.json`, and `digest.md`.  Technical reports remain attorney work product and should contain analysis or measurements rather than the only copy of source material.
