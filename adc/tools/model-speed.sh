#!/usr/bin/env bash
# Reads xproxy model ids from stdin, one per line, and issues one tool-checking
# request for each model under a single persona file supplied on the command
# line.  The prompt is fixed: "How honest are you".
#
# For each model, the script:
# 1. calls `.bin/adc llm` with `--tool-check` and `--persona MODEL,PERSONA_FILE`
# 2. enforces a 20-second timeout
# 3. prints `MODEL,ELAPSED,TOOLS_SUPPORTED` to stdout for easy capture
# 4. prints status and model output or captured error text to stderr
#
# Usage:
#   tools/model-speed.sh etc/personas/persons/d715074-0.txt < etc/models.csv
#
# Notes:
# - run from the repository root, or let the script change there itself
# - `.bin/adc` must already exist; run `make build` first if needed
# - blank lines and `#` comments in stdin are ignored
# - `TOOLS_SUPPORTED` is `true`, `false`, `timeout`, or `error`
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <persona-file> < models.txt" >&2
  exit 2
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

persona_ref="$1"
if [[ "$persona_ref" = /* ]]; then
  persona_path="$persona_ref"
else
  persona_path="$repo_root/$persona_ref"
fi

if [[ ! -f "$persona_path" ]]; then
  echo "error: persona file not found: $persona_ref" >&2
  exit 2
fi

if [[ ! -x .bin/adc ]]; then
  echo "error: .bin/adc not found or not executable; run make build first" >&2
  exit 2
fi

if ! command -v timeout >/dev/null 2>&1; then
  echo "error: timeout not found in PATH" >&2
  exit 2
fi

trim() {
  local s="$1"
  s="${s#"${s%%[![:space:]]*}"}"
  s="${s%"${s##*[![:space:]]}"}"
  printf '%s' "$s"
}

while IFS= read -r line || [[ -n "$line" ]]; do
  model="$(trim "${line%$'\r'}")"
  if [[ -z "$model" || "${model:0:1}" == "#" ]]; then
    continue
  fi

  out_file="$(mktemp)"
  err_file="$(mktemp)"
  start_ms="$(date +%s%3N)"

  set +e
  timeout 20s .bin/adc llm \
    --timeout-seconds 20 \
    --tool-check \
    --prompt "How honest are you" \
    --persona "${model},${persona_ref}" \
    >"$out_file" 2>"$err_file"
  status=$?
  set -e

  end_ms="$(date +%s%3N)"
  elapsed_ms="$((end_ms - start_ms))"

  if [[ $status -eq 0 ]]; then
    result="ok"
    elapsed_value="$elapsed_ms"
    tools_supported="true"
    payload_file="$out_file"
  elif [[ $status -eq 124 ]]; then
    result="timeout"
    elapsed_value="timeout"
    tools_supported="timeout"
    payload_file="$err_file"
  else
    result="error:$status"
    elapsed_value="$elapsed_ms"
    if grep -qiE 'support tool use|required tool|did not call required tool|tool call' "$err_file"; then
      tools_supported="false"
    else
      tools_supported="error"
    fi
    payload_file="$err_file"
  fi

  printf '%s,%s,%s\n' "$model" "$elapsed_value" "$tools_supported"
  printf 'model=%s\telapsed_ms=%s\ttools_supported=%s\tstatus=%s\tpersona=%s\n' "$model" "$elapsed_value" "$tools_supported" "$result" "$persona_ref" >&2
  cat "$payload_file" >&2
  printf '\n' >&2

  rm -f "$out_file" "$err_file"
done
