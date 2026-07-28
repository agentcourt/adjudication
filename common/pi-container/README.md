# Pi Container Image

`common/pi-container/` builds the local Podman image used for Pi juror and council agents in live runs.  The active `adc run`, `aar run`, and `aard run` commands start Pi containers directly with Podman, mount one per-agent home directory, write Pi configuration into that home, and give the agent instructions for the current case role.  The image includes the pinned Pi MCP adapter at `/opt/pi-extensions/pi-mcp-adapter/node_modules/pi-mcp-adapter`, so normal runs do not install the adapter from npm at agent startup.  The default image name is `agentcourt-pi-sandbox`, and each runtime can override it with `--pi-image` or `PI_CONTAINER_IMAGE`.

## Files

| Path | Purpose |
| --- | --- |
| `Dockerfile` | Builds a local image with upstream Pi, the pinned Pi MCP adapter, and runtime dependencies. |
| `build-image.sh` | Runs `podman build` for the local image. |

Live runs in `adc/`, `arb/`, and `arbd/` start containers from the local-run code path in each runtime, so this directory supplies the image and nothing else.

## Build

Run this command from `common/pi-container/`:

```bash
./build-image.sh
```

Run this command from the repository root if you prefer an explicit path:

```bash
common/pi-container/build-image.sh
```

`PI_CONTAINER_IMAGE` overrides the tag:

```bash
PI_CONTAINER_IMAGE=my-pi-agent common/pi-container/build-image.sh
```

## Runtime Use

The three live-agent commands use the image for Pi jurors or council members:

| Runtime | Agent role | Default adapter |
| --- | --- | --- |
| `adc run` | Jurors | `/opt/pi-extensions/pi-mcp-adapter/node_modules/pi-mcp-adapter` |
| `aar run` | Council members | `/opt/pi-extensions/pi-mcp-adapter/node_modules/pi-mcp-adapter` |
| `aard run` | Council members | `/opt/pi-extensions/pi-mcp-adapter/node_modules/pi-mcp-adapter` |

Each process gets a private `/home/user` mount under the run output directory.  The runtime writes Pi settings, MCP server configuration, model request settings, and role instructions into that directory before starting the container.  Agent access to the case goes through MCP and the case HTTP API, with the private home mount carrying the Pi configuration and agent-local files.  The adapter path lives outside `/home/user` because the runtime mounts a fresh home for each agent.

Pi agents require the provider credentials named by the selected pool entries.  The current local pools use OpenRouter, so live runs require `OPENROUTER_API_KEY`.  Pool records supply the model request configuration, including provider, model, quantization, request parameters, and persona.
