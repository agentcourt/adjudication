# OpenClaw OAuth-Derived Codex Auth

## Purpose

OpenClaw lawyer containers can use the Codex credentials stored under `~/.codex` or an `OPENAI_API_KEY`.  The Codex path stages only `~/.codex/auth.json` into a per-container directory, mounts that directory into the OpenClaw container, sets `CODEX_HOME` to the mount point, extracts `tokens.access_token`, and imports that token into OpenClaw's `openai:codex` provider profile.  OpenClaw then uses that provider profile for `gpt-5.5`.

This is useful for `aar run` because OpenClaw lawyers can run from ChatGPT/Codex subscription credentials without charging the OpenAI Platform API key.  `OPENAI_API_KEY` remains available for machines that do not have a Codex login cache or that need Platform API billing.  The Pi council still uses its configured provider path, currently OpenRouter through `OPENROUTER_API_KEY`.  The OpenClaw lawyer auth path and the Pi council auth path are separate.

## Documentation Basis

The Codex manual says Codex supports ChatGPT sign-in for subscription access and API-key sign-in for usage-based access.  It also says Codex caches login details in `~/.codex/auth.json` or an OS credential store.  The local `~/.codex/auth.json` has `auth_mode: "chatgpt"`, no `OPENAI_API_KEY`, and token fields including `access_token`, `refresh_token`, `id_token`, and `account_id`.

The manual also documents `CODEX_ACCESS_TOKEN` and `codex login --with-access-token` for trusted automation.  That path may be useful later for a machine-specific noninteractive setup.  The test here used the existing cached ChatGPT sign-in file, because that is what already exists on this machine.

## Tested Procedure

The test copied only the Codex auth file into a temporary directory.  It did not mount the whole `~/.codex` directory, because that directory can contain logs, history, configuration, and other local state unrelated to authentication.  It mounted the temporary directory into the container as `/aar-codex` and set `CODEX_HOME=/aar-codex`.

The test also removed `OPENAI_API_KEY` from the container environment before calling OpenClaw.  It imported the staged access token into OpenClaw before starting the agent, so the result depended on the staged Codex auth file rather than the Platform API key path.  The temporary directory was deleted after the test.

```bash
AUTH="$HOME/.codex/auth.json"
OUT="$(mktemp -d "${TMPDIR:-/tmp}/openclaw-codex-auth-test.XXXXXX")"
CODEX_DIR="$OUT/codex"
mkdir -p "$CODEX_DIR"
install -m 0600 "$AUTH" "$CODEX_DIR/auth.json"

docker run --rm \
  -v "$CODEX_DIR:/aar-codex:rw" \
  -e CODEX_HOME=/aar-codex \
  ghcr.io/openclaw/openclaw:latest \
  sh -lc 'set -eu
    unset OPENAI_API_KEY
    test -r "$CODEX_HOME/auth.json"
    codex_token="$(node -e "const fs=require(\"fs\"); const home=process.env.CODEX_HOME; const d=JSON.parse(fs.readFileSync(home + \"/auth.json\", \"utf8\")); process.stdout.write(d.tokens.access_token);")"
    printf "%s\n" "$codex_token" | openclaw models auth paste-token --provider openai --profile-id openai:codex >/dev/null
    unset codex_token
    openclaw agent --local --model gpt-5.5 --thinking low --timeout 120 --session-key agent:aar:codex-auth-test --message "Use the OpenAI Codex credentials in CODEX_HOME. Reply with exactly: codex oauth container auth works" --json
  '
```

## Observed Result

The OpenClaw container completed the request without `OPENAI_API_KEY`.  The response text was exactly `codex oauth container auth works`.  The JSON metadata reported provider `openai-codex`, model `gpt-5.5`, and `requestShaping.authMode: "auth-profile"`.

That result establishes that the stock OpenClaw image can read a staged Codex auth directory through `CODEX_HOME`.  It also establishes that `aar run` does not need to pass `OPENAI_API_KEY` to OpenClaw lawyer containers when this auth path is used.  The test does not establish long-term token refresh behavior under concurrent containers, so `aar run` gives each lawyer container its own staged copy of `auth.json`.

## Implementation Notes For `aar run`

`aar run` creates one Codex home directory per OpenClaw lawyer container under the run output directory when Codex auth is selected.  Each directory contains a copied `auth.json` with mode `0600`.  The Docker invocation mounts that directory read-write and sets `CODEX_HOME=/aar-codex`.

`aar run` has three OpenClaw auth modes.  `auto` prefers a readable Codex auth file and falls back to `OPENAI_API_KEY`.  `codex` requires the Codex auth file.  `api-key` requires `OPENAI_API_KEY`.  The Docker arguments in Codex mode omit `-e OPENAI_API_KEY`, unset that variable inside the container command, and run `openclaw models auth paste-token --provider openai --profile-id openai:codex` before OpenClaw starts.

The implementation avoids sharing one mounted Codex home across multiple lawyer containers.  A shared directory can create write races if Codex or OpenClaw refreshes tokens or updates session state.  Per-container copies keep the state isolated and make cleanup explicit.

The implementation also avoids mounting the operator's whole `~/.codex` directory.  The OpenClaw container needs `auth.json` for this path, not the operator's logs, history, database files, or unrelated configuration.  Copying only `auth.json` limits what enters the container and makes the run output reviewable.

## Security Notes

`auth.json` contains bearer credentials and refresh material.  Treat every staged copy as a secret.  Do not write the file contents to logs, event records, work notes, transcripts, or run summaries.

`aar run` removes staged Codex homes when the run finishes.  If a future diagnostic mode retains those directories, its output-directory permissions must reflect that the directory contains authentication material.  The API-key path does not create staged Codex homes.

## Open Questions

Longer AAR runs should confirm whether OpenClaw refreshes ChatGPT/Codex credentials inside `CODEX_HOME` during a lawyer assignment.  If it does, read-write per-container mounts are required.  If it never writes during ordinary agent execution, read-only mounts may work, but that should be tested before changing the mount mode.

`CODEX_ACCESS_TOKEN` may provide a cleaner automation path for some deployments.  That path would avoid copying a cached `auth.json`, but it requires token creation and rotation outside AAR.  The current tested path remains the local path because this machine already has a working Codex login cache.
