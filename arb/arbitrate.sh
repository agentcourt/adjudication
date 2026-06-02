#!/usr/bin/env bash
# Run an Agent Arbitration case using an already built `aar` binary.
#
# Usage:
#
#   ./arbitrate.sh [INPUT_DIR] [OUTPUT_DIR] [LAWYERAPI_ADDR]
#
# Arguments:
#
#   INPUT_DIR       Directory containing the arbitration input files.
#                   Default: examples/ex1
#
#   OUTPUT_DIR      Directory where the run output should be written.
#                   Default: out/ex1-demo
#
#   LAWYERAPI_ADDR  Address for the HTTP Lawyer API listener.
#                   Default: 127.0.0.1:0
#
# If the input directory contains an executable `sign.sh`, this script runs it.
# If the input directory contains `situation.md`, this script generates
# `complaint.md` in the input directory. Otherwise, the input directory must
# already contain `complaint.md`. The script removes the output directory and
# then runs `.bin/aar case` with a fixed invalid attempt limit of 5.
#
# The script assumes `.bin/aar` and `.bin/aarengine` have already been built. It
# does not run the Makefile build target.
#
# If `$HOME/keys.txt` exists, the script sources it. The direct council backend
# requires provider keys for the selected council models.
set -euo pipefail

cd -- "$(dirname "$0")"

export PATH="$HOME/.elan/bin:$PATH"

if [[ -f "$HOME/keys.txt" ]]; then
  # shellcheck source=/dev/null
  source "$HOME/keys.txt"
fi

: "${OPENROUTER_API_KEY:?OPENROUTER_API_KEY is required}"

INPUT_DIR="${1:-examples/ex1}"
OUTPUT_DIR="${2:-out/ex1-demo}"
LAWYERAPI_ADDR="${3:-127.0.0.1:0}"
INVALID_ATTEMPT_LIMIT=5

SIGN_SCRIPT="$INPUT_DIR/sign.sh"
SITUATION_FILE="$INPUT_DIR/situation.md"
COMPLAINT_FILE="$INPUT_DIR/complaint.md"

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
  --lawyerapi-addr "$LAWYERAPI_ADDR" \
  --invalid-attempt-limit "$INVALID_ATTEMPT_LIMIT"
