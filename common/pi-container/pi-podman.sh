#!/usr/bin/env bash
set -euo pipefail

: "${PI_CONTAINER_HOME_DIR:?PI_CONTAINER_HOME_DIR is required}"

image="${PI_CONTAINER_IMAGE:-agentcourt-pi-sandbox}"

exec podman run --rm -i \
  --network host \
  --user 0:0 \
  -e HOME=/home/user \
  -e TMPDIR=/home/user \
  -e PI_CODING_AGENT_DIR=/home/user/.pi/agent \
  -e PI_XPROXY_API_KEY=xproxy \
  -e PI_ACP_CLIENT_TOOLS_URL \
  -e PI_ACP_CLIENT_TOOLS_TOKEN \
  -e PI_ACP_CLIENT_TOOLS \
  -e NODE_OPTIONS \
  -v "$PI_CONTAINER_HOME_DIR:/home/user" \
  -w /home/user \
  "$image" \
  "$@"
