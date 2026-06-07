# Pi Container Image

`common/pi-container/` builds the local Podman image used for Pi juror and council agents in live runs.  The active `adc run`, `aar run`, and `aard run` commands start Pi containers directly with Podman, mount one per-agent home directory, write Pi configuration into that home, install the MCP adapter, and give the agent instructions for the current case role.  The default image name is `agentcourt-pi-sandbox`, and each runtime can override it with `--pi-image` or `PI_CONTAINER_IMAGE`.

## Files

| Path | Purpose |
| --- | --- |
| `Dockerfile` | Builds a local image with upstream Pi and its runtime dependencies. |
| `build-image.sh` | Runs `podman build` for the local image. |
| `pi-podman.sh` | Direct-Pi wrapper kept beside the image recipe. |
| `acp-podman.sh` | Adapter wrapper kept beside the image recipe. |

Current `adc/`, `arb/`, and `arbd/` live runs use the local-run code paths in each runtime.  Treat the Dockerfile and `build-image.sh` as the current shared pieces.

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
| `adc run` | Jurors | `npm:pi-mcp-adapter` |
| `aar run` | Council members | `npm:pi-mcp-adapter` |
| `aard run` | Council members | `npm:pi-mcp-adapter` |

Each process gets a private `/home/user` mount under the run output directory.  The runtime writes Pi settings, MCP server configuration, model request settings, and role instructions into that directory before starting the container.  Agent access to the case goes through MCP and the case HTTP API, with the private home mount carrying the Pi configuration and agent-local files.

Pi agents require the provider credentials named by the selected pool entries.  The current local pools use OpenRouter, so live runs require `OPENROUTER_API_KEY`.  Pool records supply the model request configuration, including provider, model, quantization, request parameters, and persona.
