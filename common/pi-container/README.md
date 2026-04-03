# pi-container

`pi-container/` runs upstream `pi` and `pi-acp` inside Podman for the ACP attorney path.  `adc` talks to the ACP bridge through the wrapper scripts in this directory.

## Files

| Path | Purpose |
|---|---|
| `Dockerfile` | Minimal image recipe for upstream `pi`, `pi-acp`, and `openssl` |
| `build-image.sh` | Builds the local image used by `pi-podman.sh` and `acp-podman.sh` |
| `pi-podman.sh` | Starts `pi` inside Podman and preserves stdio |
| `acp-podman.sh` | Starts `pi-acp` inside Podman and preserves stdio |
| `../etc/pi-settings.xproxy.json` | `xproxy` defaults copied into each ephemeral ACP home |
| `../etc/pi-models.xproxy.json` | Minimal `xproxy` model catalog copied into each ephemeral ACP home |

## Build the image

From the repository root:

```bash
./pi-container/build-image.sh
```

The default image name is `agentcourt-pi-sandbox`.  Override it with `PI_CONTAINER_IMAGE` if needed.

## Embedded `xproxy`

Before delegated ACP turns begin, `adc` starts `xproxy` in-process.  For each ACP attorney turn, `adc` stages a fresh writable home directory, writes `settings.json`, `models.json`, and `auth.json` under `/home/user/.pi/agent`, and mounts only that directory into the container.

## How the wrapper works

`adc case` uses `acp-podman.sh` directly.  The ACP wrapper starts `pi-acp` in the container and sets `PI_ACP_PI_COMMAND=/usr/local/bin/pi` so `pi-acp` starts the container-local `pi`, not a host binary.

The wrapper:

- runs Podman with `-i` and no TTY so ACP stdio remains intact;
- uses `--network host` so the containerized `pi` can reach the host-side ACP custom-method bridge and host `xproxy`;
- mounts one ephemeral writable directory at `/home/user`;
- uses that mounted directory as home, temp, and working directory;
- passes through ACP bridge environment variables;
- always uses the host `xproxy` path and passes only `PI_XPROXY_API_KEY=xproxy` into the container.

## Run mode

The container no longer receives the repository checkout, the run output directory, or a persistent host `~/.pi` tree.  Attorney case access goes through ACP methods such as `_adc/list_case_files`, `_adc/read_case_text_file`, `_adc/request_case_file`, and `_adc/get_juror_context`.
