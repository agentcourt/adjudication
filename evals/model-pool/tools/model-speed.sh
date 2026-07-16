#!/usr/bin/env bash
# Reads runtime model ids from stdin, one per line, and sends one council-style
# tool-check request for each model under the persona file supplied on the
# command line.
#
# For each model, the script:
# 1. calls the provider directly;
# 2. enforces a per-request timeout;
# 3. prints MODEL,ELAPSED_MS,TOOLS_SUPPORTED to stdout;
# 4. prints status and model output or captured error text to stderr.
#
# Usage:
#   evals/model-pool/tools/model-speed.sh common/etc/personas/persons/d715074-0.txt < common/data/personas/models.csv
#
# Notes:
# - run from the repository root;
# - only openrouter:// models are supported by this probe;
# - OPENROUTER_API_KEY must be set;
# - blank lines and # comments in stdin are ignored;
# - TOOLS_SUPPORTED is true, false, timeout, or error.
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <persona-file> < models.txt" >&2
  exit 2
fi

tool_timeout_seconds="${MODEL_SPEED_TIMEOUT_SECONDS:-20}"

tool_check_prompt='Case:
The plaintiff claims the defendant breached a promise. The defendant admits the breach. The parties stipulate that damages are 1.

You are council member C1. Submit your council vote now using the submit_council_vote tool. Vote demonstrated and give a brief rationale. Do not answer in prose.'

persona_ref="$1"
persona_path="$persona_ref"

if [[ ! -f "$persona_path" ]]; then
  echo "error: persona file not found: $persona_ref (run from the repository root or pass an absolute path)" >&2
  exit 2
fi

if [[ -z "${OPENROUTER_API_KEY:-}" ]]; then
  echo "error: OPENROUTER_API_KEY is required" >&2
  exit 2
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "error: python3 not found in PATH" >&2
  exit 2
fi

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
  MODEL_SPEED_MODEL="$model" \
  MODEL_SPEED_PERSONA_PATH="$persona_path" \
  MODEL_SPEED_PROMPT="$tool_check_prompt" \
  MODEL_SPEED_TIMEOUT_SECONDS="$tool_timeout_seconds" \
  python3 - <<'PY' >"$out_file" 2>"$err_file"
from __future__ import annotations

import json
import os
import socket
import sys
import urllib.error
import urllib.request


def fail(code: int, message: str) -> None:
    print(message, file=sys.stderr)
    raise SystemExit(code)


def parse_model(raw: str) -> tuple[str, str]:
    if "://" not in raw:
        fail(3, f"invalid model {raw!r}: expected ENDPOINT://MODEL")
    endpoint, rest = raw.split("://", 1)
    endpoint = endpoint.strip().lower()
    model_id = rest.split("?", 1)[0].strip()
    if endpoint != "openrouter":
        fail(3, f"unsupported model endpoint {endpoint!r}; this probe supports openrouter")
    if not model_id:
        fail(3, f"invalid model {raw!r}: empty model id")
    return endpoint, model_id


def main() -> int:
    runtime_model = os.environ["MODEL_SPEED_MODEL"]
    _endpoint, model_id = parse_model(runtime_model)
    api_key = os.environ.get("OPENROUTER_API_KEY", "").strip()
    if not api_key:
        fail(3, "OPENROUTER_API_KEY is required")
    try:
        timeout = float(os.environ.get("MODEL_SPEED_TIMEOUT_SECONDS", "20"))
    except ValueError:
        fail(3, "MODEL_SPEED_TIMEOUT_SECONDS must be a number")
    if timeout <= 0:
        fail(3, "MODEL_SPEED_TIMEOUT_SECONDS must be positive")

    persona_path = os.environ["MODEL_SPEED_PERSONA_PATH"]
    try:
        persona_text = open(persona_path, "r", encoding="utf-8").read().strip()
    except OSError as exc:
        fail(3, f"read persona file: {exc}")
    if not persona_text:
        fail(3, "persona text is empty")

    payload = {
        "model": model_id,
        "messages": [
            {
                "role": "system",
                "content": (
                    "This council-member identity is yours for this prompt. "
                    "Treat it as true of yourself, including any bias, skepticism, "
                    "hardship, or limits it implies:\n"
                    + persona_text
                ),
            },
            {"role": "user", "content": os.environ["MODEL_SPEED_PROMPT"]},
        ],
        "tools": [
            {
                "type": "function",
                "function": {
                    "name": "submit_council_vote",
                    "description": "Submit one council vote for the current deliberation opportunity.",
                    "parameters": {
                        "type": "object",
                        "properties": {
                            "member_id": {"type": "string"},
                            "vote": {
                                "type": "string",
                                "enum": ["demonstrated", "not_demonstrated"],
                            },
                            "rationale": {"type": "string"},
                        },
                        "required": ["member_id", "vote", "rationale"],
                        "additionalProperties": False,
                    },
                },
            }
        ],
        "tool_choice": {
            "type": "function",
            "function": {"name": "submit_council_vote"},
        },
        "temperature": 0,
        "max_tokens": 256,
    }
    body = json.dumps(payload).encode("utf-8")
    request = urllib.request.Request(
        "https://openrouter.ai/api/v1/chat/completions",
        data=body,
        headers={
            "Authorization": f"Bearer {api_key}",
            "Content-Type": "application/json",
            "User-Agent": "agentcourt-council-probe/1.0",
        },
        method="POST",
    )
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            raw = response.read()
    except (TimeoutError, socket.timeout):
        return 124
    except urllib.error.HTTPError as exc:
        error_body = exc.read().decode("utf-8", errors="replace")
        print(error_body, file=sys.stderr)
        return 3
    except urllib.error.URLError as exc:
        if isinstance(exc.reason, socket.timeout):
            return 124
        print(str(exc), file=sys.stderr)
        return 3

    try:
        data = json.loads(raw)
    except json.JSONDecodeError as exc:
        print(raw.decode("utf-8", errors="replace"), file=sys.stderr)
        fail(3, f"invalid provider JSON: {exc}")

    choices = data.get("choices")
    if not isinstance(choices, list) or not choices:
        print(json.dumps(data, ensure_ascii=False), file=sys.stderr)
        return 4
    message = choices[0].get("message") if isinstance(choices[0], dict) else None
    if not isinstance(message, dict):
        print(json.dumps(data, ensure_ascii=False), file=sys.stderr)
        return 4
    tool_calls = message.get("tool_calls")
    if not isinstance(tool_calls, list) or not tool_calls:
        print(json.dumps(data, ensure_ascii=False), file=sys.stderr)
        return 4
    call = tool_calls[0]
    function = call.get("function") if isinstance(call, dict) else None
    if not isinstance(function, dict) or function.get("name") != "submit_council_vote":
        print(json.dumps(data, ensure_ascii=False), file=sys.stderr)
        return 4
    arguments_text = function.get("arguments")
    try:
        arguments = json.loads(arguments_text) if isinstance(arguments_text, str) else arguments_text
    except json.JSONDecodeError:
        print(json.dumps(data, ensure_ascii=False), file=sys.stderr)
        return 4
    if not isinstance(arguments, dict):
        print(json.dumps(data, ensure_ascii=False), file=sys.stderr)
        return 4
    missing = [key for key in ("member_id", "vote", "rationale") if not str(arguments.get(key, "")).strip()]
    if missing:
        print(json.dumps(data, ensure_ascii=False), file=sys.stderr)
        return 4
    print(
        json.dumps(
            {
                "model": runtime_model,
                "upstream_model": model_id,
                "tool": "submit_council_vote",
                "arguments": arguments,
            },
            ensure_ascii=False,
        )
    )
    return 0


raise SystemExit(main())
PY
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
  elif [[ $status -eq 4 ]]; then
    result="no-tool"
    elapsed_value="$elapsed_ms"
    tools_supported="false"
    payload_file="$err_file"
  else
    result="error:$status"
    elapsed_value="$elapsed_ms"
    tools_supported="error"
    payload_file="$err_file"
  fi

  printf '%s,%s,%s\n' "$model" "$elapsed_value" "$tools_supported"
  printf 'model=%s\telapsed_ms=%s\ttools_supported=%s\tstatus=%s\tpersona=%s\n' "$model" "$elapsed_value" "$tools_supported" "$result" "$persona_ref" >&2
  cat "$payload_file" >&2
  printf '\n' >&2

  rm -f "$out_file" "$err_file"
done
