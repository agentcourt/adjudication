# Development Notes

## 2026-04-03: `adc acp` wrapper staging

### References

- ACP CLI entrypoint: [`runtime/cli/acp.go`](runtime/cli/acp.go)
- PI-home staging path: [`runtime/runner/pi_container_home.go`](runtime/runner/pi_container_home.go)
- ACP role wrapper setup: [`runtime/runner/acp_role.go`](runtime/runner/acp_role.go)
- Podman ACP wrapper: [`../common/pi-container/acp-podman.sh`](../common/pi-container/acp-podman.sh)

### Decisions

- `adc acp` now stages the temporary PI home when the selected command is `acp-podman.sh` or `pi-podman.sh`.
- That staging path already existed for `acp-role`.  The direct `acp` subcommand had been defaulting to the same wrapper without setting `PI_CONTAINER_HOME_DIR`, so the wrapper exited before ACP initialization.
- When the wrapper is in use, `session/new` now uses `/home/user` instead of the host working directory.  That matches the wrapper mount and the existing `acp-role` behavior.

### Plan

- [x] Reuse the runner PI-home staging helper from `adc acp`.
- [x] Switch wrapper-backed sessions to `/home/user`.
- [x] Add a focused CLI test for wrapper staging.

## 2026-04-03: `adc acp` default wrapper path

### References

- ACP CLI entrypoint: [`runtime/cli/acp.go`](runtime/cli/acp.go)
- CLI helper defaults: [`runtime/cli/helpers.go`](runtime/cli/helpers.go)

### Decisions

- `adc acp` now uses the repository-root relative wrapper path `common/pi-container/acp-podman.sh` as its default ACP command.
- The prior code joined the current working directory with a path that had already been resolved, producing duplicated prefixes such as `/repo/repo/common/...`.
- The ACP command default should stay simple.  If the user is not running from the repository root, the command should fail fast and require `--command` explicitly.

### Plan

- [x] Remove the extra path join in `runtime/cli/acp.go`.
- [x] Make `defaultACPServerPath` return the relative wrapper path directly.
- [ ] Add a dedicated test if this area changes again.

## 2026-03-18: `../common/tools/cluster-personas.py`

### References

- Local xproxy model and config parsing: [`runtime/xproxy/config.go`](runtime/xproxy/config.go)
- Local persona record parsing and prompt text: [`runtime/persona/persona.go`](runtime/persona/persona.go)
- Local xproxy startup and default port behavior: [`runtime/cli/xproxy.go`](runtime/cli/xproxy.go)
- OpenAI Python SDK Responses usage: https://github.com/openai/openai-python
- OpenAI embeddings API reference: https://platform.openai.com/docs/api-reference/embeddings/create

### Decisions

- The shared model and persona corpus now lives under `../common/data/personas/`.  The checked-in genes, sampled pools, cluster assignments, and PCA rows belong to the shared corpus, not to `adc/etc/`.
- The shared tools use the current working directory for their default paths.  Run them from the repository root unless you pass explicit paths.
- The Python tool talks to xproxy at `http://127.0.0.1:$PI_CONTAINER_XPROXY_PORT/v1`, with the same default port `18459` used in the Go code.
- The tool does not try to start xproxy.  The repository has no standalone xproxy CLI.  The Go commands start it internally for their own lifetimes.  The Python script instead checks `/healthz` and fails with a precise error if xproxy is absent.
- Persona records use the same `MODEL,FILE` parsing and the same juror persona prompt text as the Go runtime.
- Completions are sampled with repeated Responses API calls.  This is the direct path exposed by the current SDK usage here.  The task's "hopefully as multiple completions for one request" clause remains aspirational.
- Embeddings use the OpenAI Python SDK directly against the embeddings API.  The default embedding model is `text-embedding-3-small`, overridable with `PERSONA_SAMPLE_EMBEDDING_MODEL`.
- Embeddings run one sampled response at a time.  That avoids provider-side max-token failures on large batch requests and keeps one bad embedding response from aborting the whole run.
- PCA runs per gene over the full set of embeddings for that gene, matching the task.  When the requested PCA dimension exceeds what the sample count permits, the reduced vectors are zero-padded to keep the requested output dimension.
- The script writes cluster rows to stdout and writes per-sample PCA rows to `../common/data/personas/personas-pca.csv` by default.  Those rows are `model,persona_file,gene,x1,...,xN,cluster_num`.
- K-means cluster count is chosen per gene by maximizing silhouette score across the fixed range `3..10`.  If scoring is impossible or degenerate, all points fall into cluster `0`.

### Plan

- [x] Record the task and sources.
- [x] Add the standalone `uv` script.
- [x] Verify syntax and basic CLI behavior.

## 2026-03-18: `adc xproxy`

### References

- Root CLI dispatch and help text: [`runtime/cli/root.go`](runtime/cli/root.go)
- Existing xproxy helpers: [`runtime/cli/xproxy.go`](runtime/cli/xproxy.go)
- xproxy server entrypoint: [`runtime/xproxy/xproxy.go`](runtime/xproxy/xproxy.go)

### Decisions

- The new subcommand is `adc xproxy`.
- It resolves config and port the same way the rest of the CLI does: `--config` overrides `PI_CONTAINER_XPROXY_CONFIG` and `etc/xproxy.json`; `--port` overrides `PI_CONTAINER_XPROXY_PORT`.
- It starts xproxy directly through `xproxy.StartXProxyServer`, then waits for `SIGINT` or `SIGTERM` and closes the server cleanly.
- It fails fast if the target port already serves a healthy xproxy instance.

### Plan

- [x] Add root command dispatch and help wiring.
- [x] Add the server command implementation.
- [x] Verify help text and live `/healthz` behavior in tests.

### Results

- Live test: `uv run ../common/tools/cluster-personas.py --personas-file /tmp/persona-sample-test.csv --genes-file /tmp/persona-sample-genes.json --num-samples 3 --gene-dim 3`
- Live output: three `MP,G,C` rows for one persona and one gene through local xproxy plus direct embeddings.
- Follow-up fix: `adc xproxy` initially returned an error on clean shutdown because the listener was already closed.  [`runtime/xproxy/xproxy.go`](runtime/xproxy/xproxy.go) now ignores `net.ErrClosed` in that path, and a live `Ctrl-C` shutdown now exits with status `0`.
