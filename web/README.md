# Adjudication Web Console

`web/` contains two separate servers: a service console for the ADC, ARB, and AARD service APIs, and a read-only report over run output directories on disk.

## Run Report

`adjudication-report` scans one or more root directories for run output directories and serves a read-only report: an index of runs across all roots, run pages with facts, council votes, events, and file listings, and views of every artifact.  Markdown artifacts render as HTML through an internal minimal renderer, with text and raw views one link away.  JSON artifacts pretty-print, NDJSON artifacts render one record per line, and every file is also served raw with HTTP range support.

```bash
go run ./web/cmd/adjudication-report \
  --listen 127.0.0.1:19980 \
  --root arbattest=/media/hd2/src/arbattest/adjudication \
  --root recon=/media/hd2/src/reconometrics/var/packets
```

Roots can also come from a JSON config file: `{"listen": "127.0.0.1:19980", "roots": [{"name": "arbattest", "path": "..."}]}` passed as `--config path`.  Command-line roots append to config-file roots.  A directory counts as a run when it holds a known artifact such as `run.json`, `state.json`, `events.ndjson`, or `certificate.json`; the scanner skips `.git` and symbolic links and reports directories it cannot read on the index page.  The server only reads files, and every request path is confined to its configured root.

## Service Console

The console talks to configured service base URLs over HTTP.  The console talks to configured service base URLs over HTTP.  It does not read case output directories, import runtime packages, or fetch artifacts from the filesystem.

Run it from the repository root:

```bash
go run ./web/cmd/adjudication-web \
  --listen 127.0.0.1:19990 \
  --adc-url http://127.0.0.1:19870 \
  --arb-url http://127.0.0.1:19770 \
  --arbd-url http://127.0.0.1:19790
```

Use `--adc-token`, `--arb-token`, and `--arbd-token` when the backing services require bearer tokens.  Use `--bearer-token` to protect the web console itself.  The same settings can come from `ADC_SERVICE_URL`, `ADC_SERVICE_TOKEN`, `ARB_SERVICE_URL`, `ARB_SERVICE_TOKEN`, `ARBD_SERVICE_URL`, `ARBD_SERVICE_TOKEN`, `ADJUDICATION_WEB_LISTEN`, and `ADJUDICATION_WEB_BEARER_TOKEN`.

The create pages send raw JSON to the selected service create endpoint.  That preserves the service API as the only case-creation schema.  Browser upload is outside the current service API boundary because the services accept server-visible paths, examples, and case-file paths rather than multipart file content.
