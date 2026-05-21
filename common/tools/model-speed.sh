#!/usr/bin/env bash
# Reads xproxy model ids from stdin, one per line, and issues one juror-style
# tool-checking request for each model under a single persona file supplied on
# the command line.  The prompt gives a trivial text case and asks the model to
# submit a juror vote with the same tool name real jurors use.
#
# For each model, the script:
# 1. calls `adc/.bin/adc llm` with `--tool-check` and `--persona MODEL,PERSONA_FILE`
# 2. enforces a 20-second timeout without requiring GNU timeout
# 3. prints `MODEL,ELAPSED,TOOLS_SUPPORTED` to stdout for easy capture
# 4. prints status and model output or captured error text to stderr
#
# Usage:
#   common/tools/model-speed.sh common/etc/personas/persons/d715074-0.txt < common/data/personas/models.csv
#
# Notes:
# - run from the repository root
# - `adc/.bin/adc` must already exist; run `make build` in `adc/` first if needed
# - blank lines and `#` comments in stdin are ignored
# - `TOOLS_SUPPORTED` is `true`, `false`, `timeout`, or `error`
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <persona-file> < models.txt" >&2
  exit 2
fi

adc_bin="${ADC_BIN:-adc/.bin/adc}"
tool_timeout_seconds="${MODEL_SPEED_TIMEOUT_SECONDS:-20}"

tool_check_prompt='Case:
The plaintiff claims the defendant breached a promise.  The defendant admits the breach.  The parties stipulate that damages are 1.

You are juror J1.  Submit your juror vote now using the submit_juror_vote tool.  Vote for plaintiff, set damages to 1, use high confidence, and give a brief explanation.  Do not answer in prose.'

persona_ref="$1"
persona_path="$persona_ref"

if [[ ! -f "$persona_path" ]]; then
  echo "error: persona file not found: $persona_ref (run from the repository root or pass an absolute path)" >&2
  exit 2
fi

if [[ ! -x "$adc_bin" ]]; then
  echo "error: $adc_bin not found or not executable; run from the repository root and build adc first" >&2
  exit 2
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "error: python3 not found in PATH" >&2
  exit 2
fi

run_with_timeout() {
  local seconds="$1"
  local out_file="$2"
  local err_file="$3"
  shift 3
  python3 - "$seconds" "$out_file" "$err_file" "$@" <<'PY'
import subprocess
import sys

seconds = float(sys.argv[1])
out_path = sys.argv[2]
err_path = sys.argv[3]
cmd = sys.argv[4:]

with open(out_path, "wb") as out_file, open(err_path, "wb") as err_file:
    try:
        completed = subprocess.run(cmd, stdout=out_file, stderr=err_file, timeout=seconds, check=False)
    except subprocess.TimeoutExpired:
        raise SystemExit(124)
    raise SystemExit(completed.returncode)
PY
}

now_ms() {
  python3 - <<'PY'
import time
print(int(time.time() * 1000))
PY
}

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
  start_ms="$(now_ms)"

  set +e
  run_with_timeout "$tool_timeout_seconds" "$out_file" "$err_file" \
    "$adc_bin" llm \
    --timeout-seconds "$tool_timeout_seconds" \
    --tool-check \
    --prompt "$tool_check_prompt" \
    --persona "${model},${persona_ref}"
  status=$?
  set -e

  end_ms="$(now_ms)"
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
    if grep -qiE 'support tool use|required tool|did not call required tool|tool call|tool_choice|malformed arguments|missing (juror_id|vote|confidence|explanation|damages)|model called' "$err_file"; then
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
