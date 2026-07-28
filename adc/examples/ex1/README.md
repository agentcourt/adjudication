# Example 1

This directory contains a compact documentary civil case.  Peter alleges a contract dispute against Samantha, pleads diversity jurisdiction, and seeks damages from a paid commercial writing engagement.  The central allegation is that Samantha said it had read Neal Stephenson's essay before drafting the work, then later admitted that it had not.

The example exercises complaint drafting, case-file staging, documentary evidence, technical verification, trial presentation, and judgment.  The plaintiff can use `confession.txt`, the detached signature, the public key, invoices, work orders, and time records.  The case is small enough for repeated local runs while still forcing lawyers to read and analyze evidence.

## Inputs

| File | Purpose |
| --- | --- |
| `situation.md` | Narrative source text for complaint drafting. |
| `instructions.txt` | Assignment record. |
| `confession.txt` | Samantha's written admission. |
| `confession.sig.b64` | Base64-encoded detached signature over `confession.txt`.  Produced by `sign.sh`; not committed. |
| `samantha_public.pem` | Public key used for signature verification.  Produced by `sign.sh`; not committed. |
| `printing-invoice.txt` | Printing charges for the 1,000-copy run. |
| `distribution-work-order.txt` | Bindery, packaging, and distribution charges. |
| `time-and-token-log.txt` | Internal cleanup time and model-usage record. |
| `damages-breakdown.txt` | Claimed damages. |

## Prepare The Complaint

Run these commands from `adc/`:

```bash
make build
examples/ex1/sign.sh
.bin/adc complain \
  --situation examples/ex1/situation.md \
  --out examples/ex1/complaint.md
```

`sign.sh` regenerates the detached signature material.  `adc complain` reads the situation file and linked source documents, resolves the court profile, and writes `complaint.md`.  The later run stages the complaint and attachments into the selected output directory.

## Direct Case Run

Use `adc case` for a direct internal-role run.  This path runs the case process without starting local OpenClaw lawyer containers or Pi juror containers.  It is useful when the goal is to test intake, Lean procedure, storage, and report generation.

```bash
.bin/adc case \
  --complaint examples/ex1/complaint.md \
  --out-dir out/ex1-direct
```

## Full Local-Agent Run

Use `adc run` for the current full local-agent path.  Plaintiff and defendant lawyers are OpenClaw containers by default.  Jurors are Pi processes started when they first receive an opportunity.

```bash
export OPENROUTER_API_KEY=REPLACE_WITH_KEY
.bin/adc run \
  --complaint examples/ex1/complaint.md \
  --out-dir out/ex1-openclaw-pi \
  --openclaw-auth codex \
  --openclaw-codex-auth PATH/TO/auth.json \
  --juror-personas ../common/data/personas/pool.jsonl
```

The lawyers do not need host filesystem access to the output directory.  They receive case access through MCP tools backed by the Role API.  Work notes sent through `send_work_notes` are written to `work-notes.ndjson`.

## Clerk Service Run

The clerk service can create the same full local-agent case.  Start the service, then post a create request with `mode: "run"` or omit `mode`, since `run` is the default.  The service stores one `service-case.json` in the output directory and proxies Role API requests by `case_id`.

```bash
.bin/adc service \
  --listen 127.0.0.1:19870 \
  --output-root out/adc-service \
  --adc-bin .bin/adc \
  --engine .bin/adcengine
```

```bash
curl -sS -X POST http://127.0.0.1:19870/clerk/v1/cases \
  -H 'content-type: application/json' \
  --data '{
    "case_id": "adc-ex1",
    "complaint_path": "examples/ex1/complaint.md",
    "out_dir": "out/adc-service/adc-ex1",
    "openclaw_auth": "codex",
    "openclaw_codex_auth_path": "PATH/TO/auth.json",
    "juror_personas": "../common/data/personas/pool.jsonl"
  }'
```

## Outputs

The selected output directory contains the case record and generated setup files.  `run.json` is the machine-readable result.  `digest.md` and `transcript.md` provide written summaries, while `events.ndjson`, `run.db`, `work-notes.ndjson`, and process logs support detailed review.

The plaintiff technical evidence should verify the signature flow and state the limit of that verification.  The signature binds `confession.txt` to the key in `samantha_public.pem`.  Attribution of that key to Samantha depends on the rest of the record.
