# OpenClaw Degree Attorneys

This note describes the `arbd` side of OpenClaw lawyer runs.  The shared setup and command details are in [OpenClaw ACP MCP Bridge](../../common/docs/openclaw-acp-mcp-bridge.md) and [Common Tools](../../common/tools/README.md).  AARD talks to each OpenClaw lawyer through a TCP ACP endpoint served by `common/tools/acp-mcp-bridge.mjs`.

## Architecture

`aard case` sends each lawyer opportunity to the role's bridge through ACP.  The ACP prompt contains the degree-attorney instructions and `_meta.clientTools`.  The bridge exposes those tools to OpenClaw through its HTTP MCP endpoint, runs `openclaw agent` in the role's OpenClaw session, and forwards MCP tool calls back to AARD ACP methods.

OpenClaw owns the lawyer model, the OpenClaw agent configuration, and any native OpenClaw tools.  AARD owns the degree case record, evidence access, score filing validation, invalid-attempt feedback, transcripts, and Lean state transitions.  A role using `--plaintiff-acp-endpoint` or `--defendant-acp-endpoint` cannot also set the matching role-specific attorney model flag, because endpoint attorneys select their own model outside AARD.

## Closed-Record Run

Start one bridge process for plaintiff and one for defendant as described in [Common Tools](../../common/tools/README.md).  Use two OpenClaw config directories and two MCP URLs, for example:

```text
plaintiff ACP:  tcp://127.0.0.1:19711
plaintiff MCP:  http://127.0.0.1:19712/mcp
defendant ACP:  tcp://127.0.0.1:19713
defendant MCP:  http://127.0.0.1:19714/mcp
```

Build AARD, then run a case against the two ACP endpoints:

```bash
cd "$ADJUDICATION_REPO/arbd"
make build

.bin/aard case \
  --complaint examples/ex1/complaint.md \
  --out-dir out/ex1-openclaw-closed \
  --plaintiff-acp-endpoint tcp://127.0.0.1:19711 \
  --defendant-acp-endpoint tcp://127.0.0.1:19713 \
  --acp-timeout-seconds 900 \
  --invalid-attempt-limit 5
```

The OpenClaw lawyer receives the AARD attorney prompt, the visible case view, and the client tools for the current phase.  The lawyer should submit the required degree filing through the MCP tool that maps to AARD's `_aar/submit_decision` ACP method.

## Open-Record Evidence

An open-record degree run uses the same bridge.  Configure the OpenClaw lawyer agents with the native tools they need, and provide extra role instructions through `AGENTCOURT_OPENCLAW_EXTRA_PROMPT` when starting each bridge.  AARD does not grant search capability to an endpoint lawyer; it supplies only the ACP client tools for case access, evidence handling, and filing.

When AARD exposes source-evidence tools, OpenClaw can submit source material before the merits filing.  Accepted items are stored by AARD, recorded in state with provenance, added to the visible evidence set, and cited in later filings when the lawyer offers them.  Technical reports should contain attorney analysis, measurements, or synthesized work product rather than source content.

## Inspection

After an open-record run, inspect whether attorneys preserved source evidence and cited accepted evidence ids.  Source material submitted through AARD evidence tools appears in `submitted-evidence/`, `evidence-store/`, `evidence-manifest.json`, `state.json`, and `digest.md`.  The final degree answers remain in the AARD run summary and final state.
