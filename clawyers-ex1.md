# OpenClaw Lawyers For `arb/examples/ex1`

## Current Direct-Service Path

The current OpenClaw lawyer path uses `common/tools/acp-mcp-bridge.mjs` as one direct service per role.  AAR connects to each service over TCP ACP.  OpenClaw connects back to the same service over HTTP MCP using a `streamable-http` MCP server entry in the role's OpenClaw config.

Use separate OpenClaw config directories for plaintiff and defendant.  Each config stores a different `agentcourt` MCP URL and token, so each lawyer role reaches the MCP endpoint for its own bridge process.

## Live Result

I ran the direct-service path on 2026-05-31.  The run used `ghcr.io/openclaw/openclaw:latest`, copied the known-good OpenClaw role settings into fresh plaintiff and defendant config directories, replaced the `agentcourt` MCP entries with HTTP `streamable-http` URLs, and ran AAR against two TCP ACP endpoints.

The case completed successfully:

```json
{"status":"ok","result":"demonstrated","votes_for":4,"votes_against":1,"run_id":"ex1-openclaw-direct-20260531T202853Z","out_dir":"out/ex1-openclaw-direct-20260531T202853Z"}
```

The output packet is under `arb/out/ex1-openclaw-direct-20260531T202853Z`.  `digest.md` records `Resolution: demonstrated`.  `events.ndjson` contains eight `attorney_action` events, five `evidence_read` events, five `council_vote` events, and one `run_initialized` event.

The plaintiff and defendant OpenClaw trajectories each used one session file for four lawyer turns.  Each trajectory recorded four `session.started` entries and four `toolMetas` entries, with the expected MCP tools present: `agentcourt__aar_get_case`, `agentcourt__aar_submit_decision`, and `agentcourt__aar_submit_evidence`.

```bash
export ADJUDICATION_REPO="$(git rev-parse --show-toplevel)"
export OPENCLAW_IMAGE="${OPENCLAW_IMAGE:-ghcr.io/openclaw/openclaw:latest}"

export PLAINTIFF_OPENCLAW_DIR="${PLAINTIFF_OPENCLAW_DIR:-$HOME/.openclaw-agentcourt-plaintiff}"
export DEFENDANT_OPENCLAW_DIR="${DEFENDANT_OPENCLAW_DIR:-$HOME/.openclaw-agentcourt-defendant}"
mkdir -p "$PLAINTIFF_OPENCLAW_DIR" "$DEFENDANT_OPENCLAW_DIR"

export PLAINTIFF_ACP_PORT=19711
export PLAINTIFF_MCP_PORT=19712
export DEFENDANT_ACP_PORT=19713
export DEFENDANT_MCP_PORT=19714
export PLAINTIFF_MCP_TOKEN=agentcourt-plaintiff
export DEFENDANT_MCP_TOKEN=agentcourt-defendant
```

Configure plaintiff OpenClaw MCP:

```bash
export PLAINTIFF_MCP_JSON='{"url":"http://127.0.0.1:19712/mcp","transport":"streamable-http","headers":{"authorization":"Bearer agentcourt-plaintiff"}}'

docker run --rm -i \
  --network host \
  -v "$PLAINTIFF_OPENCLAW_DIR:/openclaw:rw" \
  -e OPENCLAW_HOME=/openclaw \
  -e OPENCLAW_CONFIG_PATH=/openclaw/openclaw.json \
  "$OPENCLAW_IMAGE" \
  openclaw mcp set agentcourt "$PLAINTIFF_MCP_JSON"
```

Configure defendant OpenClaw MCP:

```bash
export DEFENDANT_MCP_JSON='{"url":"http://127.0.0.1:19714/mcp","transport":"streamable-http","headers":{"authorization":"Bearer agentcourt-defendant"}}'

docker run --rm -i \
  --network host \
  -v "$DEFENDANT_OPENCLAW_DIR:/openclaw:rw" \
  -e OPENCLAW_HOME=/openclaw \
  -e OPENCLAW_CONFIG_PATH=/openclaw/openclaw.json \
  "$OPENCLAW_IMAGE" \
  openclaw mcp set agentcourt "$DEFENDANT_MCP_JSON"
```

Start the plaintiff bridge:

```bash
AGENTCOURT_OPENCLAW_COMMAND=docker \
AGENTCOURT_OPENCLAW_BASE_ARGS_JSON="[\"run\",\"--rm\",\"-i\",\"--network\",\"host\",\"-v\",\"$PLAINTIFF_OPENCLAW_DIR:/openclaw:rw\",\"-e\",\"OPENCLAW_HOME=/openclaw\",\"-e\",\"OPENCLAW_CONFIG_PATH=/openclaw/openclaw.json\",\"-e\",\"OPENAI_API_KEY\",\"$OPENCLAW_IMAGE\",\"openclaw\"]" \
AGENTCOURT_OPENCLAW_LOCAL=1 \
AGENTCOURT_OPENCLAW_AGENT_ID=agentcourt-plaintiff \
AGENTCOURT_OPENCLAW_SESSION_ID=agentcourt-plaintiff-ex1 \
AGENTCOURT_MCP_TOKEN="$PLAINTIFF_MCP_TOKEN" \
"$ADJUDICATION_REPO/common/tools/acp-mcp-bridge.mjs" \
  --acp-port "$PLAINTIFF_ACP_PORT" \
  --mcp-port "$PLAINTIFF_MCP_PORT"
```

Start the defendant bridge:

```bash
AGENTCOURT_OPENCLAW_COMMAND=docker \
AGENTCOURT_OPENCLAW_BASE_ARGS_JSON="[\"run\",\"--rm\",\"-i\",\"--network\",\"host\",\"-v\",\"$DEFENDANT_OPENCLAW_DIR:/openclaw:rw\",\"-e\",\"OPENCLAW_HOME=/openclaw\",\"-e\",\"OPENCLAW_CONFIG_PATH=/openclaw/openclaw.json\",\"-e\",\"OPENAI_API_KEY\",\"$OPENCLAW_IMAGE\",\"openclaw\"]" \
AGENTCOURT_OPENCLAW_LOCAL=1 \
AGENTCOURT_OPENCLAW_AGENT_ID=agentcourt-defendant \
AGENTCOURT_OPENCLAW_SESSION_ID=agentcourt-defendant-ex1 \
AGENTCOURT_MCP_TOKEN="$DEFENDANT_MCP_TOKEN" \
"$ADJUDICATION_REPO/common/tools/acp-mcp-bridge.mjs" \
  --acp-port "$DEFENDANT_ACP_PORT" \
  --mcp-port "$DEFENDANT_MCP_PORT"
```

Run `ex1` from `arb`:

```bash
cd "$ADJUDICATION_REPO/arb"
make build

.bin/aar case \
  --complaint examples/ex1/complaint.md \
  --out-dir out/ex1-openclaw-direct \
  --plaintiff-acp-endpoint tcp://127.0.0.1:19711 \
  --defendant-acp-endpoint tcp://127.0.0.1:19713 \
  --acp-timeout-seconds 900 \
  --run-id ex1-openclaw-direct
```

## Prior Result

The earlier successful run used the stock OpenClaw Docker image and completed with `{"status":"ok","result":"demonstrated","votes_for":4,"votes_against":1,"run_id":"ex1-openclaw-stable-session","out_dir":"out/ex1-openclaw-stable-session-20260531T194414Z"}`.  That output remains under `arb/out/ex1-openclaw-stable-session-20260531T194414Z`.

That earlier run used the old two-process MCP bridge and a separate TCP wrapper.  The direct-service bridge replaces that path.  The retained evidence from the prior run is useful as a case-output reference, but the command sequence above is the current path.
