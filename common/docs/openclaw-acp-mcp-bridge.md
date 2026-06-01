# OpenClaw ACP MCP Bridge

The operational README is [Common Tools](../tools/README.md).  This note records the design boundary.

The bridge lets a stock OpenClaw agent act as an AAR or AARD lawyer.  AAR and AARD speak ACP to the bridge over TCP.  OpenClaw speaks MCP to the bridge over HTTP using a `streamable-http` MCP server entry in the OpenClaw config.

## Components

`common/tools/acp-mcp-bridge.mjs` runs one service for one lawyer role.  The ACP side listens on `tcp://host:port` for AAR or AARD.  The MCP side listens on `http://host:port/mcp` for OpenClaw.

During each ACP `session/prompt`, AAR or AARD includes the current lawyer tools in `_meta.clientTools`.  The bridge converts that list into MCP `tools/list` output for the active OpenClaw turn.  When OpenClaw sends `tools/call`, the bridge sends the original ACP client method back to AAR or AARD over the same TCP connection.

Each bridge process keeps one OpenClaw session id.  Run separate bridge processes and separate OpenClaw config directories for plaintiff and defendant roles, because each OpenClaw config contains one concrete MCP URL and token.

## OpenClaw Configuration

OpenClaw needs one MCP server entry named `agentcourt` unless the bridge runs with another `--mcp-server-name`.  For a plaintiff bridge listening on `http://127.0.0.1:19712/mcp`, configure OpenClaw with:

```json
{
  "url": "http://127.0.0.1:19712/mcp",
  "transport": "streamable-http",
  "headers": {
    "authorization": "Bearer agentcourt-plaintiff"
  }
}
```

Save that JSON with `openclaw mcp set agentcourt "$JSON"`.  Repeat the command for the defendant OpenClaw config with the defendant bridge's MCP port and token.

The OpenClaw tool prefix comes from the MCP server name.  With the default server name, an ACP tool named `aar_get_case` appears to the OpenClaw lawyer as `agentcourt__aar_get_case`.  The bridge adds the prefixed tool names to the OpenClaw prompt so the lawyer can select the right MCP tool.

## Responsibility Boundary

The bridge maps protocol messages.  AAR and AARD still decide which tools exist for the opportunity, which evidence can be read, whether a filing is valid, how many invalid attempts remain, and how the case state changes.  OpenClaw owns the lawyer model and its native tools.

The bridge maps:

| Direction | Mapping |
| --- | --- |
| ACP prompt metadata | `_meta.clientTools` becomes MCP `tools/list`. |
| MCP tool call | `tools/call` becomes an ACP client method request. |
| ACP tool result | The ACP result becomes the MCP tool result. |

## Tests

`common/tools/acp-mcp-bridge-test.mjs` starts the direct bridge, drives two ACP prompts, calls the HTTP MCP endpoint during each prompt, and checks that both turns use the same OpenClaw session id.  With `--docker-image ghcr.io/openclaw/openclaw:latest`, the fake OpenClaw command runs inside the stock OpenClaw image and uses that image's MCP SDK to call the bridge.
