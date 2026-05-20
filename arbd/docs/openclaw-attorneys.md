# OpenClaw Degree Attorneys

This note describes how to run `arbd` with OpenClaw-backed plaintiff and defendant attorneys.  `aard case` talks to attorneys through ACP.  The ordinary local path starts a Pi ACP wrapper, while the OpenClaw path connects each role to a remote ACP endpoint backed by the `aard-openclaw-attorney` adapter.

## Components

`aard-openclaw-attorney` is a stdio ACP adapter.  It receives AARD `session/prompt` requests, asks AARD for the visible case through `_aar/get_case`, reads visible text files through `_aar/list_case_files` and `_aar/read_case_text_file`, obtains one JSON filing from an OpenClaw command or OpenClaw agent, and submits that filing through `_aar/submit_decision`.  If the OpenClaw response contains source-evidence submissions, the adapter submits them through `_aar/submit_evidence` before filing the decision.

`tools/openclaw-acp-tcp-bridge.js` exposes that stdio adapter as a TCP ACP endpoint.  One bridge process starts a fresh adapter for each ACP connection.  `aard case` then connects the plaintiff and defendant roles with `--plaintiff-acp-endpoint` and `--defendant-acp-endpoint`.

## Model Ownership

AARD does not select the OpenClaw model for an endpoint attorney.  Model selection and native tool availability belong to the OpenClaw agent or command behind the endpoint.  For that reason, a role using `--plaintiff-acp-endpoint` or `--defendant-acp-endpoint` cannot also set the matching role-specific attorney model flag.

This rule matters for reproducibility.  A closed-record run should use an OpenClaw agent that stays inside the record provided by AARD.  An open-record run should use an OpenClaw agent with the needed search, browser, fetch, transcript, or equivalent tools, and the run notes should identify that environment.

## Build and Check

Run these commands from `arbd/`:

```bash
make build
node --check tools/openclaw-acp-tcp-bridge.js
```

The build creates `.bin/aard-openclaw-attorney`.  The node check verifies the TCP bridge syntax.  The bridge requires Node.js, an OpenClaw CLI on `PATH` or `AARD_OPENCLAW_CLI`, and a dedicated OpenClaw lawyer agent when `AARD_OPENCLAW_AGENT=1`.

## Environment

| Variable | Meaning |
| --- | --- |
| `AARD_OPENCLAW_AGENT` | Set to `1` to ask the adapter to call `openclaw agent`. |
| `AARD_OPENCLAW_AGENT_ID` | Dedicated OpenClaw lawyer agent id. |
| `AARD_OPENCLAW_CLI` | OpenClaw CLI path.  Defaults to `openclaw`. |
| `AARD_OPENCLAW_AGENT_SESSION_ID` | Optional fixed OpenClaw session id. |
| `AARD_OPENCLAW_AGENT_THINKING` | Optional OpenClaw thinking setting. |
| `AARD_OPENCLAW_AGENT_LOCAL` | Set to `1` to pass `--local` to `openclaw agent`. |
| `AARD_OPENCLAW_AGENT_EXTRA_PROMPT` | Extra instruction text appended to the adapter prompt. |
| `AARD_OPENCLAW_ATTORNEY_COMMAND` | Command that reads the adapter job JSON on stdin and writes one filing JSON on stdout. |
| `AARD_OPENCLAW_ATTORNEY_DECISION_JSON` | Fixed filing JSON for tests or scripted runs. |
| `AARD_OPENCLAW_ATTORNEY_TIMEOUT_SECONDS` | Adapter command timeout. |

Use `AARD_OPENCLAW_AGENT_ID` rather than a personal default agent.  Degree arbitration work should run in the intended lawyer context.  The bridge removes `AARD_OPENCLAW_AGENT_MODEL` from the adapter environment, because endpoint attorneys own model selection outside AARD.

## Closed-Record Run

Start the bridge in one terminal:

```bash
tools/openclaw-acp-tcp-bridge.js --host 127.0.0.1 --port 19801
```

Run a case in another terminal:

```bash
.bin/aard case \
  --complaint examples/ex1/complaint.md \
  --out-dir out/ex1-openclaw-closed \
  --plaintiff-acp-endpoint tcp://127.0.0.1:19801 \
  --defendant-acp-endpoint tcp://127.0.0.1:19801
```

The OpenClaw attorney receives the AARD attorney prompt, the visible case view, and visible text case files.  It must return either an ordinary `aar_submit_decision` object or a structured bundle with `evidence_submissions` and `decision`.  In a closed-record run, the response should normally contain only the decision.

## Open-Record Evidence

An open-record run can submit source material through the structured bundle form:

```json
{
  "evidence_submissions": [
    {
      "title": "Source page",
      "source_url": "https://example.test/source",
      "mime_type": "text/plain",
      "retrieval_timestamp": "2026-05-20T00:00:00Z",
      "relevance": "Shows the source text used for the similarity score.",
      "content": "source text",
      "preferred_filename_ext": "txt",
      "offer_label": "PX-new",
      "offer_as_exhibit": true
    }
  ],
  "decision": {
    "kind": "tool",
    "tool_name": "submit_argument",
    "payload": {
      "text": "Argument text.",
      "technical_reports": []
    }
  }
}
```

The adapter submits each evidence item first.  Accepted items are stored by AARD under `submitted-evidence/`, recorded in state with provenance, added to the visible case-file set, and cited in `offered_files` when `offer_as_exhibit` is true.  Technical reports should contain attorney analysis, measurements, or synthesized work product rather than source content.
