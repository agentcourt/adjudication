# Common Tools

This directory contains shared tools for `arb` and `arbd`.  The OpenClaw bridge lets a stock OpenClaw agent act as a plaintiff or defendant lawyer in an AAR or AARD case.  AAR and AARD keep control of the case record, lawyer opportunities, evidence limits, filing validation, transcripts, and state transitions.

## OpenClaw ACP-MCP Bridge

The bridge is one long-lived process for one lawyer role.  It listens for ACP over TCP from AAR or AARD, and it listens for MCP over HTTP from OpenClaw.  During an ACP `session/prompt`, AAR or AARD sends the current lawyer instructions and the current client tools in `_meta.clientTools`; the bridge exposes that same tool list through its HTTP MCP endpoint for the OpenClaw turn.

Run one bridge process for plaintiff and one bridge process for defendant.  Each process keeps one OpenClaw session id for its lifetime, so the plaintiff lawyer has one continuing OpenClaw session and the defendant lawyer has another.  Each role should also use its own OpenClaw config directory or agent configuration, because OpenClaw stores the MCP server URL in its own config.

The bridge does not parse filings or decide case policy.  When OpenClaw calls an MCP tool such as `agentcourt__aar_get_case` or `agentcourt__aar_submit_decision`, the bridge sends the matching ACP client method back to AAR or AARD over the active TCP connection.  AAR or AARD validates the result and records the event.

## Files

| File | Role |
| --- | --- |
| `acp-mcp-bridge.mjs` | Runs the direct TCP ACP and HTTP MCP bridge for one OpenClaw lawyer role. |
| `acp-mcp-bridge-test.mjs` | Tests the direct bridge with a fake OpenClaw command, and optionally with the MCP SDK inside the stock OpenClaw container. |

## Set Up

Run these commands from an adjudication checkout:

```bash
export ADJUDICATION_REPO="$(git rev-parse --show-toplevel)"
export OPENCLAW_IMAGE="${OPENCLAW_IMAGE:-ghcr.io/openclaw/openclaw:latest}"

export PLAINTIFF_ACP_PORT=19711
export PLAINTIFF_MCP_PORT=19712
export DEFENDANT_ACP_PORT=19713
export DEFENDANT_MCP_PORT=19714

export PLAINTIFF_MCP_TOKEN=agentcourt-plaintiff
export DEFENDANT_MCP_TOKEN=agentcourt-defendant
```

Choose persistent OpenClaw config directories for the two lawyer roles:

```bash
export PLAINTIFF_OPENCLAW_DIR="${PLAINTIFF_OPENCLAW_DIR:-$HOME/.openclaw-agentcourt-plaintiff}"
export DEFENDANT_OPENCLAW_DIR="${DEFENDANT_OPENCLAW_DIR:-$HOME/.openclaw-agentcourt-defendant}"
mkdir -p "$PLAINTIFF_OPENCLAW_DIR" "$DEFENDANT_OPENCLAW_DIR"
```

The role-specific MCP JSON tells OpenClaw where the bridge's HTTP MCP endpoint will listen.  The JSON is OpenClaw MCP configuration; pass it to `openclaw mcp set`.  Do not paste it into an AAR or AARD command.

```bash
export PLAINTIFF_MCP_JSON='{"url":"http://127.0.0.1:19712/mcp","transport":"streamable-http","headers":{"authorization":"Bearer agentcourt-plaintiff"}}'
export DEFENDANT_MCP_JSON='{"url":"http://127.0.0.1:19714/mcp","transport":"streamable-http","headers":{"authorization":"Bearer agentcourt-defendant"}}'
```

If OpenClaw runs on the host, save each role's MCP server entry in that role's OpenClaw config file:

```bash
OPENCLAW_CONFIG_PATH="$PLAINTIFF_OPENCLAW_DIR/openclaw.json" \
  openclaw mcp set agentcourt "$PLAINTIFF_MCP_JSON"

OPENCLAW_CONFIG_PATH="$DEFENDANT_OPENCLAW_DIR/openclaw.json" \
  openclaw mcp set agentcourt "$DEFENDANT_MCP_JSON"
```

If OpenClaw runs through the stock Docker image, run the same config command inside the image with the role config directory mounted at `/openclaw`:

```bash
docker run --rm -i \
  --network host \
  -v "$PLAINTIFF_OPENCLAW_DIR:/openclaw:rw" \
  -e OPENCLAW_HOME=/openclaw \
  -e OPENCLAW_CONFIG_PATH=/openclaw/openclaw.json \
  "$OPENCLAW_IMAGE" \
  openclaw mcp set agentcourt "$PLAINTIFF_MCP_JSON"

docker run --rm -i \
  --network host \
  -v "$DEFENDANT_OPENCLAW_DIR:/openclaw:rw" \
  -e OPENCLAW_HOME=/openclaw \
  -e OPENCLAW_CONFIG_PATH=/openclaw/openclaw.json \
  "$OPENCLAW_IMAGE" \
  openclaw mcp set agentcourt "$DEFENDANT_MCP_JSON"
```

Validate both OpenClaw configs before a case run:

```bash
OPENCLAW_CONFIG_PATH="$PLAINTIFF_OPENCLAW_DIR/openclaw.json" openclaw config validate
OPENCLAW_CONFIG_PATH="$DEFENDANT_OPENCLAW_DIR/openclaw.json" openclaw config validate
```

For Docker-backed OpenClaw configs, validate through Docker:

```bash
docker run --rm -i \
  --network host \
  -v "$PLAINTIFF_OPENCLAW_DIR:/openclaw:rw" \
  -e OPENCLAW_HOME=/openclaw \
  -e OPENCLAW_CONFIG_PATH=/openclaw/openclaw.json \
  "$OPENCLAW_IMAGE" \
  openclaw config validate

docker run --rm -i \
  --network host \
  -v "$DEFENDANT_OPENCLAW_DIR:/openclaw:rw" \
  -e OPENCLAW_HOME=/openclaw \
  -e OPENCLAW_CONFIG_PATH=/openclaw/openclaw.json \
  "$OPENCLAW_IMAGE" \
  openclaw config validate
```

Configure each OpenClaw role with the model, credentials, sandbox policy, and native tools you want the lawyer to have.  For the stock image, pass provider keys from the host environment with Docker `-e` entries such as `-e OPENAI_API_KEY`.  If the OpenClaw agent uses sandboxed tools, allow the OpenClaw MCP bundle tool in that OpenClaw config.

## Start The Bridge Services

For a host OpenClaw CLI, start the plaintiff bridge:

```bash
OPENCLAW_CONFIG_PATH="$PLAINTIFF_OPENCLAW_DIR/openclaw.json" \
AGENTCOURT_OPENCLAW_AGENT_ID=agentcourt-plaintiff \
AGENTCOURT_OPENCLAW_SESSION_ID=agentcourt-plaintiff \
AGENTCOURT_MCP_TOKEN="$PLAINTIFF_MCP_TOKEN" \
"$ADJUDICATION_REPO/common/tools/acp-mcp-bridge.mjs" \
  --acp-port "$PLAINTIFF_ACP_PORT" \
  --mcp-port "$PLAINTIFF_MCP_PORT"
```

Start the defendant bridge in another terminal:

```bash
OPENCLAW_CONFIG_PATH="$DEFENDANT_OPENCLAW_DIR/openclaw.json" \
AGENTCOURT_OPENCLAW_AGENT_ID=agentcourt-defendant \
AGENTCOURT_OPENCLAW_SESSION_ID=agentcourt-defendant \
AGENTCOURT_MCP_TOKEN="$DEFENDANT_MCP_TOKEN" \
"$ADJUDICATION_REPO/common/tools/acp-mcp-bridge.mjs" \
  --acp-port "$DEFENDANT_ACP_PORT" \
  --mcp-port "$DEFENDANT_MCP_PORT"
```

For Docker-backed OpenClaw, the bridge still runs on the host, and the bridge starts `docker run ... openclaw agent` for each lawyer turn.  Start the plaintiff bridge with:

```bash
AGENTCOURT_OPENCLAW_COMMAND=docker \
AGENTCOURT_OPENCLAW_BASE_ARGS_JSON="[\"run\",\"--rm\",\"-i\",\"--network\",\"host\",\"-v\",\"$PLAINTIFF_OPENCLAW_DIR:/openclaw:rw\",\"-e\",\"OPENCLAW_HOME=/openclaw\",\"-e\",\"OPENCLAW_CONFIG_PATH=/openclaw/openclaw.json\",\"-e\",\"OPENAI_API_KEY\",\"$OPENCLAW_IMAGE\",\"openclaw\"]" \
AGENTCOURT_OPENCLAW_LOCAL=1 \
AGENTCOURT_OPENCLAW_AGENT_ID=agentcourt-plaintiff \
AGENTCOURT_OPENCLAW_SESSION_ID=agentcourt-plaintiff \
AGENTCOURT_MCP_TOKEN="$PLAINTIFF_MCP_TOKEN" \
"$ADJUDICATION_REPO/common/tools/acp-mcp-bridge.mjs" \
  --acp-port "$PLAINTIFF_ACP_PORT" \
  --mcp-port "$PLAINTIFF_MCP_PORT"
```

Start the defendant bridge the same way, changing the config directory, agent id, session id, token, and ports:

```bash
AGENTCOURT_OPENCLAW_COMMAND=docker \
AGENTCOURT_OPENCLAW_BASE_ARGS_JSON="[\"run\",\"--rm\",\"-i\",\"--network\",\"host\",\"-v\",\"$DEFENDANT_OPENCLAW_DIR:/openclaw:rw\",\"-e\",\"OPENCLAW_HOME=/openclaw\",\"-e\",\"OPENCLAW_CONFIG_PATH=/openclaw/openclaw.json\",\"-e\",\"OPENAI_API_KEY\",\"$OPENCLAW_IMAGE\",\"openclaw\"]" \
AGENTCOURT_OPENCLAW_LOCAL=1 \
AGENTCOURT_OPENCLAW_AGENT_ID=agentcourt-defendant \
AGENTCOURT_OPENCLAW_SESSION_ID=agentcourt-defendant \
AGENTCOURT_MCP_TOKEN="$DEFENDANT_MCP_TOKEN" \
"$ADJUDICATION_REPO/common/tools/acp-mcp-bridge.mjs" \
  --acp-port "$DEFENDANT_ACP_PORT" \
  --mcp-port "$DEFENDANT_MCP_PORT"
```

Each bridge prints the ACP and MCP listener addresses on stderr:

```text
acp listening on tcp://127.0.0.1:19711
mcp listening on http://127.0.0.1:19712/mcp
```

## Run AAR

Build AAR before using the bridge.  The current binary must send `_meta.clientTools` in ACP `session/prompt` requests.

```bash
cd "$ADJUDICATION_REPO/arb"
make build
```

Run a case with the two TCP ACP endpoints:

```bash
.bin/aar case \
  --complaint examples/ex1/complaint.md \
  --out-dir out/openclaw-ex1 \
  --plaintiff-acp-endpoint "tcp://127.0.0.1:$PLAINTIFF_ACP_PORT" \
  --defendant-acp-endpoint "tcp://127.0.0.1:$DEFENDANT_ACP_PORT" \
  --acp-timeout-seconds 900
```

## Run AARD

Build AARD before using the bridge:

```bash
cd "$ADJUDICATION_REPO/arbd"
make build
```

Run a degree case with the same two TCP ACP endpoints:

```bash
.bin/aard case \
  --complaint examples/ex1/complaint.md \
  --out-dir out/openclaw-ex1 \
  --plaintiff-acp-endpoint "tcp://127.0.0.1:$PLAINTIFF_ACP_PORT" \
  --defendant-acp-endpoint "tcp://127.0.0.1:$DEFENDANT_ACP_PORT" \
  --acp-timeout-seconds 900
```

## Turn Sequence

When AAR or AARD reaches a lawyer opportunity, it sends an ACP `session/prompt` request to the role's bridge.  That prompt contains lawyer instructions plus `_meta.clientTools`, whose entries include ACP method names, MCP tool names, descriptions, and JSON schemas.  The bridge records that list for the active prompt and starts `openclaw agent` with the bridge process's OpenClaw session id.

OpenClaw reads its `agentcourt` MCP server config and connects to the bridge's HTTP MCP endpoint.  The bridge answers `tools/list` with the active prompt's tool list.  With the default MCP server name, `_aar/get_case` appears to OpenClaw as `agentcourt__aar_get_case`, and `_aar/submit_decision` appears as `agentcourt__aar_submit_decision`.

When OpenClaw calls an MCP tool, the bridge sends the matching ACP client method back over the same TCP connection that currently has the prompt pending.  AAR or AARD executes the method, validates the filing or evidence operation, and returns the result.  The bridge returns that result as the MCP tool result and waits for `openclaw agent` to finish the turn.

## Environment

| Variable | Meaning |
| --- | --- |
| `AGENTCOURT_ACP_HOST` | ACP TCP bind host.  Defaults to `127.0.0.1`. |
| `AGENTCOURT_ACP_PORT` | ACP TCP bind port.  Defaults to `19701`. |
| `AGENTCOURT_MCP_HOST` | MCP HTTP bind host.  Defaults to `127.0.0.1`. |
| `AGENTCOURT_MCP_PORT` | MCP HTTP bind port.  Defaults to `19702`. |
| `AGENTCOURT_MCP_PATH` | MCP HTTP path.  Defaults to `/mcp`. |
| `AGENTCOURT_MCP_TOKEN` | Bearer token required for MCP requests.  If omitted, the bridge generates one for the process. |
| `AGENTCOURT_OPENCLAW_COMMAND` | Command used to start OpenClaw.  Defaults to `openclaw`. |
| `AGENTCOURT_OPENCLAW_BASE_ARGS_JSON` | JSON string array inserted before the generated `agent` arguments.  Use this for Docker's `run ... openclaw` prefix. |
| `AGENTCOURT_OPENCLAW_EXTRA_ARGS_JSON` | JSON string array appended after the generated OpenClaw arguments. |
| `AGENTCOURT_OPENCLAW_AGENT_ID` | OpenClaw lawyer agent id passed as `--agent`. |
| `AGENTCOURT_OPENCLAW_SESSION_ID` | OpenClaw session id.  If omitted, the bridge generates one id and reuses it for its lifetime. |
| `AGENTCOURT_OPENCLAW_LOCAL` | Set to `1`, `true`, or `yes` to add `--local` to `openclaw agent`. |
| `AGENTCOURT_OPENCLAW_THINKING` | Optional OpenClaw `--thinking` value. |
| `AGENTCOURT_OPENCLAW_CWD` | Working directory for the OpenClaw command. |
| `AGENTCOURT_OPENCLAW_TIMEOUT_SECONDS` | OpenClaw command timeout.  Defaults to `900`. |
| `AGENTCOURT_OPENCLAW_MCP_SERVER_NAME` | MCP server name used for prompt instructions.  Defaults to `agentcourt`. |
| `AGENTCOURT_OPENCLAW_EXTRA_PROMPT` | Extra instruction text inserted before the AAR or AARD prompt. |
| `AGENTCOURT_ACP_TOOL_TIMEOUT_SECONDS` | Timeout for ACP client method calls.  Defaults to `120`. |

The bridge also passes `AGENTCOURT_OPENCLAW_MCP_URL` and `AGENTCOURT_OPENCLAW_MCP_TOKEN` to the OpenClaw child process.  The direct OpenClaw config should already contain the concrete HTTP MCP URL and token, but those variables are useful for tests and custom OpenClaw wrappers.

## Tests

Run these from the adjudication checkout:

```bash
node --check common/tools/acp-mcp-bridge.mjs
node --check common/tools/acp-mcp-bridge-test.mjs
node common/tools/acp-mcp-bridge-test.mjs
node common/tools/acp-mcp-bridge-test.mjs --docker-image ghcr.io/openclaw/openclaw:latest
go test ./common/acp
```

The host test starts the bridge, connects to its TCP ACP endpoint, sends two prompts with different `_meta.clientTools`, and uses a fake OpenClaw command to call the bridge's HTTP MCP endpoint during each prompt.  The Docker test runs the fake OpenClaw command inside the stock OpenClaw image and uses the MCP SDK from `/app/node_modules` to call the bridge's streamable HTTP MCP endpoint.
