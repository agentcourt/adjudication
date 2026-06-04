# OpenClaw Degree Attorneys

This note describes the `arbd` side of OpenClaw-backed plaintiff and defendant attorneys.  `aard case` talks to attorneys through ACP endpoints.  The OpenClaw path uses `tools/openclaw-acp-tcp-bridge.js` to expose `.bin/aard-openclaw-attorney` as a TCP ACP endpoint, and the adapter source lives under `tools/aard-openclaw-attorney`.

## Architecture

`aard case` sends each lawyer opportunity to the role's TCP ACP endpoint.  The bridge starts one `aard-openclaw-attorney` process for each ACP connection.  The adapter receives the degree-attorney instructions, visible case view, readable case files, and AARD client tools, then asks OpenClaw for one filing JSON and submits that filing through AARD ACP methods.

OpenClaw owns the lawyer model, OpenClaw agent configuration, and any native OpenClaw tools.  AARD owns the degree case record, evidence access, score-filing validation, invalid-attempt feedback, transcripts, and Lean state transitions.  A role using `--plaintiff-acp-endpoint` or `--defendant-acp-endpoint` cannot also set the matching role-specific attorney model flag, because endpoint attorneys select their own models outside AARD.

## Closed-Record Run

Build AARD and the OpenClaw attorney adapter from `arbd/`:

```bash
make build
```

Start a bridge for each role:

```bash
tools/openclaw-acp-tcp-bridge.js --host 127.0.0.1 --port 19711
tools/openclaw-acp-tcp-bridge.js --host 127.0.0.1 --port 19713
```

Run a case against those endpoints:

```bash
.bin/aard case \
  --complaint examples/ex1/complaint.md \
  --out-dir out/ex1-openclaw-closed \
  --plaintiff-acp-endpoint tcp://127.0.0.1:19711 \
  --defendant-acp-endpoint tcp://127.0.0.1:19713 \
  --acp-timeout-seconds 900 \
  --invalid-attempt-limit 5
```

The OpenClaw lawyer receives the AARD attorney prompt, the visible case view, and readable evidence.  It should submit the required degree filing through the adapter.  In a closed-record run, the response should contain only the filing.

## Open-Record Evidence

An open-record degree run uses the same bridge and adapter.  Configure the OpenClaw lawyer agents with the native tools they need, and provide extra role instructions through `AARD_OPENCLAW_AGENT_EXTRA_PROMPT` when starting each bridge.  AARD does not grant search capability to an endpoint lawyer; it supplies case access, evidence handling, and filing tools for the current opportunity.

When AARD exposes source-evidence tools, OpenClaw can submit source material before the merits filing.  Accepted items are stored by AARD, recorded in state with provenance, added to the visible evidence set, and cited in later filings by `evidence_id` through `offered_evidence`.  Technical reports should contain attorney analysis, measurements, or synthesized work product rather than source content.

## Inspection

After an open-record run, inspect whether attorneys preserved source evidence and cited accepted evidence ids.  Source material submitted through AARD evidence tools appears in `submitted-evidence/`, `evidence-store/`, `evidence-manifest.json`, `state.json`, and `digest.md`.  The final degree answers remain in the AARD run summary and final state.
