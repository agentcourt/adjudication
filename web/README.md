# Adjudication Web Console and Report

`web/` contains two separate servers.  `adjudication-web` is a server-rendered console for the ADC, ARB, and AARD service APIs: it creates cases, lists them, manages active runs, and reads results, artifacts, evidence, and attestation events through configured service base URLs.  `adjudication-report` is a read-only report over run output directories on disk: it scans configured root trees for runs and serves an index, run pages with facts, votes, and events, and views of every artifact.

The console keeps case state and artifact bytes behind the service APIs and reads no output directories.  The report reads only the filesystem and serves only GET routes.  [The web runbook](runbook.md) covers commands, configuration, page structure, limits, and troubleshooting for both.
