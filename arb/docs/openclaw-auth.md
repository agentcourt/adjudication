# OpenClaw Codex Auth

## Purpose

OpenClaw lawyer containers can authenticate with either a Codex `auth.json` file or `OPENAI_API_KEY`.  The preferred local path is Codex auth, because it lets `aar run` use the same ChatGPT/Codex subscription credentials used by Codex.  Pi council agents use their own provider configuration from the selected council pool, usually OpenRouter through `OPENROUTER_API_KEY`.

## Codex Auth Mode

In Codex auth mode, `aar run` copies one selected `auth.json` file into a private per-lawyer Codex home under the run output directory.  It mounts that directory into the OpenClaw container, sets `CODEX_HOME` to the mount point, unsets `OPENAI_API_KEY`, extracts the staged access token, and imports that token into OpenClaw with `openclaw models auth paste-token --provider openai --profile-id openai:codex`.

`aar run` does not mount the operator's whole Codex home.  The container needs `auth.json`, not local logs, history, configuration, or unrelated state.  Each lawyer container receives its own staged copy so token or session writes cannot race with another lawyer container.

Use Codex auth explicitly:

```bash
.bin/aar run ex01 \
  --openclaw-auth codex \
  --openclaw-codex-auth "$HOME/.codex/auth.json"
```

Use automatic selection when either auth path is acceptable:

```bash
.bin/aar run ex01 --openclaw-auth auto
```

`auto` first looks for a readable Codex auth file.  If none is available, it uses `OPENAI_API_KEY`.  `api-key` mode requires `OPENAI_API_KEY` and does not create staged Codex homes.

## Secret Handling

`auth.json` contains bearer credentials and refresh material.  Treat the source file and every staged copy as secrets.  Do not write the file contents to logs, event records, work notes, transcripts, run summaries, or support tickets.

`aar run` removes staged Codex homes when the run finishes.  An interrupted process can leave staged copies in the output directory, so review run directories for `auth.json` before preserving or sharing artifacts.
