# Web Runbook

Operating notes for the two web servers in `web/`.  The service console is `adjudication-web`, and the run report is `adjudication-report`.  Both are single-binary Go HTTP servers with no state of their own, so a restart is safe at any time and loses nothing but the open pages.

## Service Console

The console talks to the ADC, ARB, and AARD service APIs over HTTP and renders case lists, case detail, artifacts, evidence, events, and attestation records.  It also submits case-creation JSON and management actions to the services, so it needs network reach to each configured base URL.

```sh
go run ./web/cmd/adjudication-web \
  --listen 127.0.0.1:19990 \
  --adc-url http://127.0.0.1:19870 \
  --arb-url http://127.0.0.1:19770 \
  --arbd-url http://127.0.0.1:19790
```

Use `--adc-token`, `--arb-token`, and `--arbd-token` when the services require bearer tokens, and `--bearer-token` to protect the console.  The same settings come from `ADC_SERVICE_URL`, `ADC_SERVICE_TOKEN`, `ARB_SERVICE_URL`, `ARB_SERVICE_TOKEN`, `ARBD_SERVICE_URL`, `ARBD_SERVICE_TOKEN`, `ADJUDICATION_WEB_LISTEN`, and `ADJUDICATION_WEB_BEARER_TOKEN`.  Case state and artifact bytes stay behind the service APIs: the console reads no output directories.

## Run Report

The report scans root directories for run output directories and serves a read-only view of everything it finds.  It reads only the filesystem, serves only GET routes, and has no authentication, so bind it to `127.0.0.1` unless the host network is trusted.

```sh
go run ./web/cmd/adjudication-report \
  --listen :9090 \
  --root arbattest=/media/hd2/src/arbattest/adjudication \
  --root recon=/media/hd2/src/reconometrics/var/packets
```

Stop with SIGINT or SIGTERM.  The report holds no state and writes nothing, so restarts and upgrades are safe at any time.

### Configuration

| Flag | Meaning |
|------|---------|
| `--listen` | Listen address, default `127.0.0.1:19980`. |
| `--root [name=]path` | A tree to scan, repeatable.  Without `name=`, the name is the path base name. |
| `--config path` | JSON config file. |

The config file holds the same settings: `{"listen": "127.0.0.1:19980", "roots": [{"name": "arbattest", "path": "/media/hd2/src/arbattest/adjudication"}]}`.  Command-line roots append after config-file roots, and a `--listen` flag overrides the file.  Root names appear in URLs and must match `[A-Za-z0-9._-]+`.  A repeated name gets a numeric suffix, and a repeated path fails at startup.  Each root must exist as a readable directory when the server starts.

### Scanning

A directory counts as a run when it directly contains one of `run.json`, `state.json`, `local-run.json`, `events.ndjson`, `certificate.json`, `transcript.md`, `digest.md`, `aar-stdout.log`, or `aar-stderr.log`.  The scanner stops at a run directory, so artifacts inside one never register as further runs.  It skips hidden directories, Pi agent home directories such as `pi-C1`, and symbolic links, and it stops sixteen levels deep.  Directories it cannot read, and directories at the depth limit, appear in a scan problems table at the top of the index.  The index rescans all roots on every load; against the current two trees, a cold scan takes about two seconds and a warm one under half a second.

A run with artifacts parses into an index row with case, system, status, phase, resolution, vote tally, and timing.  A run directory holding only logs, such as a failed `aar` attempt, reports status `incomplete`.

### Pages

| Route | Content |
|-------|---------|
| `/` | All runs across roots, sortable, with the scan problems table when problems exist.  `?root=name` filters to one root. |
| `/run/{root}/{path}` | One run: facts, complaint, attorneys, council, votes with rationales, events, and a table of every file in the run directory. |
| `/view/{root}/{path}` | One file or directory.  Markdown renders as HTML with `text` and `raw` links, JSON pretty-prints, NDJSON renders one record per line, directories render as listings, and binary files link to raw. |
| `/raw/{root}/{path}` | Exact bytes with HTTP range support. |

Column headers on sortable tables sort on click and reverse on a second click.  The index refreshes every thirty seconds, and a run page refreshes every fifteen while its recorded status is `running` or `active`.

The markdown renderer is internal and covers headings, paragraphs, lists, blockquotes, fenced code, horizontal rules, inline code, bold, single-star emphasis, and links with `http`, `https`, `mailto`, or relative targets.  Everything else renders as literal text, which is the safe treatment for model-written filings.

### Limits

View pages refuse files above 8 MB and link to the raw route, which always serves byte ranges.  The run page skips parsing an `events.ndjson` above the same limit and says so on the page.  Event payload previews on the run page truncate at 240 characters; the full payload is in the `events.ndjson` view.

### Troubleshooting

A missing run means its directory holds none of the marker files, sits under a hidden or `pi-C<n>` directory, or lies deeper than sixteen levels; the scan problems table reports the unreadable and depth-limited cases.  A 404 on a file that exists on disk means the path resolves outside the configured root, which happens when a symbolic link points out of the tree; the report refuses those.  Check a suspect markdown rendering against the `text` and `raw` views of the same file, which show the source.
