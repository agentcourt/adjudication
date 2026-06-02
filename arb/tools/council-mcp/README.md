# AAR Council MCP Adapter

`aar-council-mcp` lets a remote agent act as one council member in an AAR case.  The adapter speaks MCP Streamable HTTP to the agent and calls the AAR Council HTTP API exposed by `aar case --council-backend=councilapi`.  AAR owns the case state, turn order, deadlines, attempt counts, evidence custody, and vote validation.

The adapter can serve many MCP sessions.  Each session starts at `/mcp?case_id=...&member_id=...`, and those query parameters bind that session to one council member in one case.  The current AAR runner serves one case per `aar case` process, but the adapter accepts per-case API mappings so it can front several case processes.

## Start AAR

Build the runtime and start a case with the Council API backend:

```sh
make build
.bin/aar case \
  --complaint examples/ex1/complaint.md \
  --out-dir out/ex1-councilapi \
  --council-backend councilapi \
  --councilapi-addr 127.0.0.1:19772
```

`aar case` prints the Council API base URL to stderr.  With the address above, the base URL is `http://127.0.0.1:19772/councilapi/v1`.

## Start The MCP Adapter

Start the adapter with a default Council API base URL:

```sh
.bin/aar-council-mcp \
  --listen 127.0.0.1:19781 \
  --councilapi-base http://127.0.0.1:19772/councilapi/v1 \
  --bearer-token choose-a-token
```

Use `--case` when one adapter fronts several `aar case` processes:

```sh
.bin/aar-council-mcp \
  --listen 0.0.0.0:19781 \
  --case arb-1=http://127.0.0.1:19772/councilapi/v1 \
  --case arb-2=http://127.0.0.1:19773/councilapi/v1 \
  --bearer-token choose-a-token
```

MCP sessions expire after 30 minutes without a request.  Use `--session-ttl` to change that duration, or `--session-ttl 0` to disable expiry.  If a session expires, the agent can initialize again with the same MCP URL and continue from the current Council API status.

## Give The Agent Its URL

Give the agent an MCP URL for the council member it should run:

```text
Use this MCP server to act as council member C1 in case arb-1:
{"url":"http://127.0.0.1:19781/mcp?case_id=arb-1&member_id=C1","transport":"streamable-http","headers":{"Authorization":"Bearer choose-a-token"}}
```

The adapter returns an MCP session id during initialization.  The agent sends that session id on later MCP requests.  If the session disappears or expires, the agent can initialize again with the same URL; the adapter reads the current case-member status from AAR.

## Agent Loop

Every session exposes `wait_for_council_opportunity` and `get_current_council_opportunity`.  `wait_for_council_opportunity` calls `GET /councilapi/v1/wait` for the bound case-member and waits up to 30 seconds.  It returns `state: ready` when that council member should act, `state: waiting` when no opportunity is ready, `state: done` when the case has ended, and `state: error` when the agent needs operator attention.

When `wait_for_council_opportunity` returns `state: waiting`, call it again with the returned `after_version`.  When it returns `state: ready`, use the returned prompt, turn, limits, and tools to complete exactly that opportunity.  After a successful `submit_council_vote`, call `wait_for_council_opportunity` again.  Stop when `state: done` is returned.

`get_current_council_opportunity` calls `GET /councilapi/v1/get` for the bound case-member and returns the prompt, current status, active turn, available tools, limits, remaining time, and attempts left.  Use it for inspection.  Use `wait_for_council_opportunity` for the normal work loop.

The adapter exposes the direct Council API tools that AAR reports for the current turn.  A call to an MCP tool such as `submit_council_vote` forwards the supplied JSON object as the Council API `arguments` value.  Before forwarding the call, the adapter asks AAR for the current `opportunity_id` and includes it in `POST /councilapi/v1/do`.
