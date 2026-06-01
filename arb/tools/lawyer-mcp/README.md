# AAR Lawyer MCP Adapter

`aar-lawyer-mcp` lets OpenClaw act as a plaintiff, defendant, or observer in an AAR case.  It speaks MCP Streamable HTTP to OpenClaw and calls the AAR Lawyer HTTP API exposed by `aar case`.  The adapter does not own case state; `aar case` owns turn order, deadlines, attempts, evidence custody, and filing validation.

The adapter process can serve many case-role sessions.  Each MCP session starts at `/mcp?case_id=...&role_id=...`, and the query parameters bind that session to one AAR case role.  The role must be `plaintiff`, `defendant`, or `observer`.

## Build

Build from `arb/`:

```bash
make build
```

The build writes the adapter to `.bin/aar-lawyer-mcp`.  The same target also builds `.bin/aar` and the Lean engine.  `make test` runs the runtime tests and the adapter tests.

## Start AAR

Start `aar case` with a fixed Lawyer API address so the adapter has a stable target:

```bash
.bin/aar case \
  --complaint work/case/complaint.md \
  --out-dir out/case \
  --lawyerapi-addr 127.0.0.1:19771
```

`aar case` prints the Lawyer API base URL to stderr.  With the address above, the base URL is `http://127.0.0.1:19771/lawyerapi/v1`.  The case id is still required by the API, but the current `aar case` process serves one case.

## Start the Adapter

For one local case, pass the Lawyer API base URL as the default target:

```bash
export AAR_MCP_TOKEN='choose-a-token'

.bin/aar-lawyer-mcp \
  --listen 127.0.0.1:19780 \
  --lawyerapi-base http://127.0.0.1:19771/lawyerapi/v1 \
  --bearer-token "$AAR_MCP_TOKEN"
```

For several AAR case processes, map each case id to its Lawyer API base URL:

```bash
.bin/aar-lawyer-mcp \
  --listen 127.0.0.1:19780 \
  --case arb-1=http://127.0.0.1:19771/lawyerapi/v1 \
  --case arb-2=http://127.0.0.1:19772/lawyerapi/v1 \
  --bearer-token "$AAR_MCP_TOKEN"
```

The adapter accepts local HTTP origins by default and can accept more origins through repeated `--allow-origin` flags.  Use `--bearer-token` unless the adapter only listens on an isolated local test interface.  The adapter logs MCP session creation, MCP session deletion, and forwarded Lawyer API tool calls without logging request payloads or tokens.

MCP sessions expire after 30 minutes without a valid request.  Use `--session-ttl` to change that duration, or `--session-ttl 0` to disable expiry.  Use `--session-cleanup-interval` to change how often the adapter deletes idle sessions.  If a session expires, OpenClaw can initialize again with the same MCP URL and continue from the current case-role status.

## Configure OpenClaw

Give OpenClaw the MCP URL for the role it should play:

```bash
openclaw mcp set aar \
  '{"url":"http://127.0.0.1:19780/mcp?case_id=arb-1&role_id=plaintiff","transport":"streamable-http","headers":{"Authorization":"Bearer choose-a-token"}}'
```

Use a different OpenClaw profile or home for the other side:

```bash
openclaw mcp set aar \
  '{"url":"http://127.0.0.1:19780/mcp?case_id=arb-1&role_id=defendant","transport":"streamable-http","headers":{"Authorization":"Bearer choose-a-token"}}'
```

The adapter returns an MCP session id during initialization.  OpenClaw sends that session id on later MCP requests.  If the session disappears or expires, OpenClaw can initialize again with the same URL, and the adapter will recover the current case-role status from `aar case`.

## OpenClaw Skill

This directory includes an OpenClaw workspace skill at `skills/arb/SKILL.md`.  Install or copy that directory into the clawyer's OpenClaw workspace so the agent knows how to accept an arbitration assignment, configure this MCP server, and work the assignment through `wait_for_opportunity`.

For a workspace install, the resulting path should look like this:

```text
<openclaw-workspace>/skills/arb/SKILL.md
```

Joe's intended flow is then:

```text
Act as plaintiff lawyer in case arb-1.  The AAR MCP endpoint is https://court.example/mcp.  Use this bearer token: ...
```

The skill tells the claw to save an MCP server named `aar-<case_id>-<role_id>`, verify the assignment with `wait_for_opportunity`, and then keep working through that tool.  The AAR court and MCP adapter remain outside Joe's OpenClaw process.  The claw does not need an inbound HTTP listener or a cron job to decide when its role has work.

The same setup can be sent as one complete prompt without installing a skill.  The prompt must give the exact `openclaw mcp set <name> <json>` command form, the MCP endpoint, the bearer token, and the operating loop.  If the prompt verifies MCP with curl in the same turn, it must initialize MCP first, read the `Mcp-Session-Id` response header, send `notifications/initialized`, and then call `tools/call` with that session id.

The operating loop is short.  Call `wait_for_opportunity`.  If it returns `state: waiting`, call `wait_for_opportunity` again with the returned `after_version`.  If it returns `state: ready`, use the returned prompt, turn, limits, and available tools to complete exactly that opportunity.  After a successful `submit_decision`, return to `wait_for_opportunity`.  If it returns `state: done`, stop.  If it returns `state: error`, report the error and stop.

## Tool Behavior

Every session exposes `wait_for_opportunity` and `get_current_opportunity`.  `wait_for_opportunity` calls `GET /lawyerapi/v1/wait` for the bound case-role and waits up to 30 seconds.  It returns `state: ready` when the role should act, `state: waiting` when no opportunity is ready, `state: done` when the case has ended, and `state: error` when the clawyer needs operator help.

`get_current_opportunity` calls `GET /lawyerapi/v1/get` for the bound case-role and returns the prompt, role status, active turn, available tools, limits, remaining time, and attempts left.  A waiting role receives waiting status and no lawyer filing tools.  Use `get_current_opportunity` for inspection; use `wait_for_opportunity` for the main clawyer loop.

The adapter exposes the direct Lawyer API tools that are available for the current role and phase.  A call to an MCP tool such as `submit_decision` forwards the supplied JSON object as the Lawyer API `arguments` value.  Before forwarding a mutating lawyer tool, the adapter asks AAR for the current `opportunity_id` and includes it in `POST /lawyerapi/v1/do`.

The adapter implements request/response Streamable HTTP over MCP POST requests.  It returns 405 for MCP GET streams and advertises `listChanged: false`, so clients refresh tools by calling `tools/list` or `get_current_opportunity`.  A stale or unavailable tool call returns an MCP tool result with `isError: true` and the AAR error details in `structuredContent`.
