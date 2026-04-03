# Example 1

This directory contains the main end-to-end example.  Peter alleges a contract dispute against Samantha, pleads diversity jurisdiction, and seeks damages from a paid commercial writing engagement that Peter says Samantha mishandled by failing to read Neal Stephenson's essay before drafting.

The example matters because it is compact, documentary, and federal.  It also exercises the external-attorney path.  In ACP runs, both plaintiff and defense counsel are delegated through ACP, `pi` runs in Podman for those attorneys, and the plaintiff lawyer can verify the detached signature on `confession.txt` with `openssl` inside the container before filing that verification as a technical report.  The attorney container gets only a fresh writable home directory for the turn.  Case access goes through ACP methods, not host mounts.

## Reproduce the ACP run

From the repository root:

```bash
make build
make demo
```

That path signs the example materials, drafts `examples/ex1/complaint.md` from `examples/ex1/situation.md`, and then runs `.bin/adc case` with both attorneys delegated through `pi-container/acp-podman.sh`.

If you want the manual case command after complaint drafting, use:

```bash
examples/ex1/sign.sh
.bin/adc complain --situation examples/ex1/situation.md --out examples/ex1/complaint.md
.bin/adc case \
  --complaint examples/ex1/complaint.md \
  --out-dir out/ex1-demo \
  --acp-role plaintiff \
  --acp-role defendant \
  --acp-command "$PWD/pi-container/acp-podman.sh"
```

## Inputs

| File | Purpose |
|---|---|
| `situation.md` | Narrative source text |
| `instructions.txt` | Assignment record |
| `confession.txt` | Samantha's written admission |
| `confession.sig.b64` | Base64-encoded detached signature over `confession.txt` |
| `samantha_public.pem` | Public key used for verification |
| `printing-invoice.txt` | Printing charges for the 1,000-copy run |
| `distribution-work-order.txt` | Bindery, packaging, and distribution charges |
| `time-and-token-log.txt` | Internal cleanup time and model-usage record |
| `damages-breakdown.txt` | Claimed damages |

## Attorney work products

| File | Purpose |
|---|---|
| `complaint.md` | Generated complaint drafted from `situation.md` and used by `make demo` |
| `session-summary.txt` | Meeting and reliance summary prepared for the case record |
| `plaintiff-strategy.md` | Plaintiff private plan written during the run |
| `defense-strategy.md` | Defense private plan written during the run |

## Outputs

The demo run writes these into the selected output directory:

| File | Meaning |
|---|---|
| `run.json` | Full authoritative run artifact |
| `digest.md` | Case digest |
| `transcript.md` | Trial transcript |
| `normalized-case.json` | Intake packet |
| `generated-scenario.json` | Seeded execution bundle |

The plaintiff technical report records the exact OpenSSL verification flow and states its limit clearly: the signature binds the confession text to the key in `samantha_public.pem`, but key attribution to Samantha still depends on the rest of the record.
