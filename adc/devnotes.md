# Development Notes

## 2026-03-18: `tools/cluster-personas.py`

### References

- Local xproxy model and config parsing: [`runtime/xproxy/config.go`](runtime/xproxy/config.go)
- Local persona record parsing and prompt text: [`runtime/persona/persona.go`](runtime/persona/persona.go)
- Local xproxy startup and default port behavior: [`runtime/cli/xproxy.go`](runtime/cli/xproxy.go)
- OpenAI Python SDK Responses usage: https://github.com/openai/openai-python
- OpenAI embeddings API reference: https://platform.openai.com/docs/api-reference/embeddings/create

### Decisions

- The Python tool talks to xproxy at `http://127.0.0.1:$PI_CONTAINER_XPROXY_PORT/v1`, with the same default port `18459` used in the Go code.
- The tool does not try to start xproxy.  The repository has no standalone xproxy CLI.  The Go commands start it internally for their own lifetimes.  The Python script instead checks `/healthz` and fails with a precise error if xproxy is absent.
- Persona records use the same `MODEL,FILE` parsing and the same juror persona prompt text as the Go runtime.
- Completions are sampled with repeated Responses API calls.  This is the direct path exposed by the current SDK usage here.  The task's "hopefully as multiple completions for one request" clause remains aspirational.
- Embeddings use the OpenAI Python SDK directly against the embeddings API.  The default embedding model is `text-embedding-3-small`, overridable with `PERSONA_SAMPLE_EMBEDDING_MODEL`.
- Embeddings run one sampled response at a time.  That avoids provider-side max-token failures on large batch requests and keeps one bad embedding response from aborting the whole run.
- PCA runs per gene over the full set of embeddings for that gene, matching the task.  When the requested PCA dimension exceeds what the sample count permits, the reduced vectors are zero-padded to keep the requested output dimension.
- The script writes cluster rows to stdout and writes per-sample PCA rows to `etc/personas-pca.csv` by default.  Those rows are `model,persona_file,gene,x1,...,xN,cluster_num`.
- K-means cluster count is chosen per gene by maximizing silhouette score across all admissible `k` values from `2` through `points - 1`.  If scoring is impossible or degenerate, all points fall into cluster `0`.

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

- Live test: `uv run tools/cluster-personas.py --personas-file /tmp/persona-sample-test.csv --genes-file /tmp/persona-sample-genes.json --num-samples 3 --gene-dim 3`
- Live output: three `MP,G,C` rows for one persona and one gene through local xproxy plus direct embeddings.
- Follow-up fix: `adc xproxy` initially returned an error on clean shutdown because the listener was already closed.  [`runtime/xproxy/xproxy.go`](runtime/xproxy/xproxy.go) now ignores `net.ErrClosed` in that path, and a live `Ctrl-C` shutdown now exits with status `0`.
