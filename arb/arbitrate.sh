#!/usr/bin/env bash
# Run an Agent Arbitration case using an already built `aar` binary.
#
# Usage:
#
#   ./arbitrate.sh [INPUT_DIR] [OUTPUT_DIR] [ATTORNEY_MODEL]
#
# Arguments:
#
#   INPUT_DIR       Directory containing the arbitration input files.
#                   Default: examples/ex1
#
#   OUTPUT_DIR      Directory where the run output should be written.
#                   Default: out/ex1-demo
#
#   ATTORNEY_MODEL  xproxy model id used for attorney ACP turns.
#                   Default: openai://gpt-5
#                   Use openai://gpt-5?tools=search to enable web search.
#
# If the input directory contains an executable `sign.sh`, this script runs it.
# If the input directory contains `situation.md`, this script generates
# `complaint.md` in the input directory. Otherwise, the input directory must
# already contain `complaint.md`. The script removes the output directory and
# then runs `.bin/aar case` with the selected attorney model and a fixed invalid
# attempt limit of 5.
#
# The script assumes `.bin/aar` and `.bin/aarengine` have already been built. It
# does not run the Makefile build target.
#
# If `$HOME/keys.txt` exists, the script sources it. The run requires
# `OPENAI_API_KEY` for attorney model calls and `OPENROUTER_API_KEY` for council
# model calls.
set -euo pipefail

cd -- "$(dirname "$0")"

export PATH="$HOME/.elan/bin:$PATH"

if [[ -f "$HOME/keys.txt" ]]; then
  # shellcheck source=/dev/null
  source "$HOME/keys.txt"
fi

: "${OPENAI_API_KEY:?OPENAI_API_KEY is required}"
: "${OPENROUTER_API_KEY:?OPENROUTER_API_KEY is required}"

# Containerized Pi reaches host-side xproxy through Podman's host alias on this Mac.
export AGENTCOURT_PI_XPROXY_BASE_URL="http://host.containers.internal:18459/v1"

INPUT_DIR="${1:-examples/ex1}"
OUTPUT_DIR="${2:-out/ex1-demo}"
ATTORNEY_MODEL="${3:-openai://gpt-5}"
INVALID_ATTEMPT_LIMIT=5

SIGN_SCRIPT="$INPUT_DIR/sign.sh"
SITUATION_FILE="$INPUT_DIR/situation.md"
COMPLAINT_FILE="$INPUT_DIR/complaint.md"

if ! podman info >/dev/null 2>&1; then
  podman machine start podman-machine-default
fi

rm -rf "$OUTPUT_DIR"

if [[ -e "$SIGN_SCRIPT" ]]; then
  if [[ ! -x "$SIGN_SCRIPT" ]]; then
    echo "error: sign script exists but is not executable: $SIGN_SCRIPT" >&2
    exit 1
  fi
  "$SIGN_SCRIPT"
fi

if [[ -f "$SITUATION_FILE" ]]; then
  .bin/aar complain --situation "$SITUATION_FILE" --out "$COMPLAINT_FILE"
elif [[ ! -f "$COMPLAINT_FILE" ]]; then
  echo "error: input directory must contain situation.md or complaint.md: $INPUT_DIR" >&2
  exit 1
fi

.bin/aar case \
  --complaint "$COMPLAINT_FILE" \
  --out-dir "$OUTPUT_DIR" \
  --attorney-model "$ATTORNEY_MODEL" \
  --invalid-attempt-limit "$INVALID_ATTEMPT_LIMIT"
