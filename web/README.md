# Adjudication Web Console

`web/` contains a small server-rendered console for the ADC, ARB, and AARD service APIs.  The console talks to configured service base URLs over HTTP.  It does not read case output directories, import runtime packages, or fetch artifacts from the filesystem.

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
