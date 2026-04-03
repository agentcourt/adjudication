#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "$script_dir/.." && pwd)"
image="${PI_CONTAINER_IMAGE:-agentcourt-pi-sandbox}"

podman build \
  --build-context pi_acp="$repo_root/submodules/pi-acp" \
  -t "$image" \
  -f "$script_dir/Dockerfile" \
  "$script_dir"
