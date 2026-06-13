# AAR Docker Image Runbook

## Scope

This runbook covers the AAR base image from `arb/Dockerfile`, the AAR glue image from `arb/Dockerfile.glue`, and the attested exec run launched by `tools/run-aar.sh`.  The base image contains the compiled `aar` and `aarengine` binaries, the `adjudication` source tree, the Docker CLI, and an embedded Pi council root filesystem.  The glue image adds AWS CLI, `nitro-tpm-attest`, the TSS runtime libraries, and `glue/arb-glue.sh`.

The current generic path runs any checked-in example under `arb/examples/<name>`.  Select the example with `AAR_EXAMPLE`; when the variable is absent, both `run-aar.sh` and the glue script use `ex01`.  To run a new arb this way, add the case as a new checked-in example, push the `arbattest` branch, rebuild and upload the glue image tar, run the exec AMI with `AAR_EXAMPLE=<name>`, and verify the S3 artifacts.

The current glue input schema contains only runtime secrets and the example name.  It reads `auth.json` and `keys.sh` from `INPUT_PREFIX` and passes one example argument to `aar run`.  External case packets that live outside the image need an agreed S3 input schema for complaint, files, policy, and optional run flags before implementation.

## Files And Branches

| Item | Location | Role |
| --- | --- | --- |
| AAR base image | `adjudication/arb/Dockerfile` | Builds `aar`, `aarengine`, Docker CLI support, and the embedded Pi council root filesystem. |
| Glue image | `adjudication/arb/Dockerfile.glue` | Adds AWS CLI, `nitro-tpm-attest`, TSS libraries, and the S3 artifact flow. |
| Glue script | `adjudication/arb/glue/arb-glue.sh` | Runs the selected AAR example, archives output, writes the manifest, obtains the TPM attestation, and uploads artifacts to S3. |
| Exec launcher | `attest/exec.sh` | Starts the Docker-enabled exec AMI with user-data from a script. |
| AAR exec script | `adjudication/arb/tools/run-aar.sh` | Downloads the glue image tar on the exec AMI, loads it into Docker, and starts the glue container. |
| Local AAR driver | `adjudication/arb/tools/run-arb-attested.py` | Starts `exec.sh` through `dev`, polls S3, downloads artifacts, extracts the AAR archive, and can verify the result. |
| Container proof script | `adjudication/arb/tools/run-container-poc.sh` | Runs the glue image in `attest-only` mode for the container attestation proof. |
| Attestation parser | `attest/parse_attestation.py` | Verifies the attestation signature and certificate chain and prints user data and PCR values. |
| Dev source checkout | `/home/ec2-user/adjudication-build-2361886` on `dev` | Source tree used for Docker builds on `dev`. |
| Dev launcher directory | `/home/ec2-user/attest` on `dev` | Runtime directory for `exec.sh`, `run-aar.sh`, and helper scripts.  This directory is not the source-control checkout. |

Use the `arbattest` branch in both `jsmorph/adjudication` and `jsmorph/attest`.  The current Docker-enabled exec AMI is `ami-011f957fe91cf7b81` in `us-east-2`.  Its expected PCR values are listed in the verification section and must be replaced when the exec AMI is rebuilt.

## Attestation Record

The attestation record lives in S3, not stdout.  Stdout from `exec.sh` is useful for launch progress and the instance ID, but verification reads the S3 prefix.  A completed AAR run writes exactly these objects under `OUTPUT_PREFIX`: `run.log`, `aar-output.tar.gz`, `manifest.json`, `manifest.sha384`, and `attestation.b64`.

`manifest.sha384` contains the SHA-384 hash of `manifest.json`.  The glue script passes that file to `nitro-tpm-attest --user-data`, so the attestation `User Data` field must equal the manifest hash.  The manifest binds the selected example, input prefix, output prefix, exec AMI, instance ID, glue image ID, glue image tar hash, run log hash, and AAR archive hash.

If `aar run` exits nonzero, the glue image uploads `run.log` and `aar-partial.tar.gz`, then exits with the AAR status.  It does not create `manifest.json`, `manifest.sha384`, or `attestation.b64` for a failed AAR run.  A prefix with only the failure artifacts is a failed AAR run, and no attestation verification exists for that run.

## Runtime Topology

The exec AMI runs Docker on the host.  `run-aar.sh` downloads `s3://agentcourt-data/arbattest/images/arb-glue-poc.tar`, computes its SHA-384 hash, loads `arb-glue:poc`, records the Docker image ID, and starts the glue container.  The container receives `/var/run/docker.sock`, `/dev/tpm0`, `/dev/tpmrm0` when present, `INPUT_PREFIX`, `OUTPUT_PREFIX`, `RUN_ID`, `AAR_EXAMPLE`, and image identity fields.

The glue container starts the parent `aar` process through `/usr/local/bin/aar-entrypoint`.  That parent process starts OpenClaw lawyer containers and Pi council containers through the host Docker daemon.  The parent container uses host networking, and the glue AAR command passes `--openclaw-network host` so OpenClaw uses `127.0.0.1` for the AAR MCP server.

The parent and child containers share paths through the host Docker daemon, so AAR output must live under a path that the host Docker daemon can mount into child containers.  The exec path uses `ARB_GLUE_WORK_ROOT=/var/lib/arbattest-aar`, mounted into the glue container at the same absolute path.  The local direct-run command below follows the same rule by mounting the output root at the identical path inside the parent container.

## Required Inputs

The glue image reads secrets from S3 so the attested instance does not depend on SSH file transfer at run time.  `INPUT_PREFIX` must contain `auth.json` and `keys.sh`.  `auth.json` is the Codex auth file used by OpenClaw, and `keys.sh` must assign or export `OPENROUTER_API_KEY` for the Pi council.

The verified instance profile for the first version is the same profile used on `dev`, passed to `exec.sh` as `IAM_INSTANCE_PROFILE=ec2-nix-builder`.  The verified instance type is `m5.4xlarge`, because the exec AMI root filesystem is RAM-backed and Docker extracts image layers into that RAM-backed filesystem.  The verified region is `us-east-2`, and the verified S3 bucket prefix is `s3://agentcourt-data/arbattest/`.

Valid `AAR_EXAMPLE` values are checked-in example directory names accepted by `aar run`: nonempty, no slash, no dot prefix, and no `..`.  Current examples include `ex01` through `ex12` and the long-form condition examples under `arb/examples`.  The glue script records the chosen example in `manifest.json` as `aar_example`.

## Build The Base Image Locally

Run the base image build from `/media/hd2/src/arbattest`.  The Dockerfile clones the public repository named by `ADJUDICATION_REPO` and checks out `ADJUDICATION_REF`; it does not copy this local checkout into the image.  Use `--no-cache` after pushing branch changes, because a cached clone layer can retain an older branch tip.

```bash
docker build --no-cache \
  -t arbattest-aar:local \
  -f adjudication/arb/Dockerfile \
  adjudication/arb
```

Validate any checked-in example complaint with the selected image.  This command tests that the image contains the example and that the complaint parses.  Replace `ex01` with any checked-in example name.

```bash
AAR_EXAMPLE=ex01
docker run --rm \
  arbattest-aar:local \
  validate --complaint "examples/$AAR_EXAMPLE/complaint.md"
```

The expected output is:

```text
ok
```

## Run An Example Locally Without Attestation

The local direct run exercises the AAR image, OpenClaw lawyers, and Pi council containers without the exec AMI.  It needs a readable Codex auth file at `tmp/auth.json`, a key file at `tmp/keys.sh`, and the host Docker socket.  It writes a timestamped output directory under `aar-out` and a sibling log file.

```bash
set -eu
AAR_EXAMPLE="${AAR_EXAMPLE:-ex01}"
. "$PWD/tmp/keys.sh"
output_root="$PWD/aar-out"
mkdir -p "$output_root"
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
out="$output_root/$AAR_EXAMPLE-local-$stamp"
log="$output_root/$AAR_EXAMPLE-local-$stamp.log"
docker run --rm --network host \
  --user "$(id -u):$(id -g)" \
  --group-add "$(stat -c '%g' /var/run/docker.sock)" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$output_root:$output_root" \
  -v "$PWD/tmp/auth.json:/run/secrets/codex-auth.json:ro" \
  -e OPENROUTER_API_KEY \
  arbattest-aar:local \
  run \
  --out-dir "$out" \
  --openclaw-auth codex \
  --openclaw-codex-auth /run/secrets/codex-auth.json \
  --openclaw-network host \
  --docker docker \
  --podman docker \
  --pi-image agentcourt-pi-sandbox:latest \
  "$AAR_EXAMPLE" \
  >"$log" 2>&1
printf '%s\n' "$out"
printf '%s\n' "$log"
```

Read the local result after the command exits.  A completed run writes `local-run.json` with `status` and `resolution`.  The first completed local `ex01` run produced `status=ok` and `resolution=demonstrated`.

```bash
python3 -m json.tool "$out/local-run.json"
```

Check that the cleanup code removed runtime credential files from a new output directory.  The command should print no paths for current runs.  Older completed directories can contain files created before the cleanup fix.

```bash
find "$out" \
  \( -name .mcp.json -o -path '*/.pi/agent/auth.json' \) \
  -print
```

## Build And Upload The Glue Image On `dev`

Build the base and glue images on `dev` from the `arbattest` branch, then upload the Docker archive used by `run-aar.sh`.  Use the source checkout at `/home/ec2-user/adjudication-build-2361886`, not the launcher directory.  Record the printed SHA-384 hash because the glue manifest records the image tar hash for each run.

```bash
ssh dev 'set -eu
cd /home/ec2-user/adjudication-build-2361886
git pull --ff-only origin arbattest
cd arb
sudo docker build --no-cache \
  -t arbattest-aar:dev \
  -f Dockerfile \
  .
sudo docker build --no-cache \
  --build-arg AAR_IMAGE=arbattest-aar:dev \
  -t arb-glue:poc \
  -f Dockerfile.glue \
  .
sudo docker save arb-glue:poc -o /home/ec2-user/arb-glue-poc.tar
sudo chown ec2-user:ec2-user /home/ec2-user/arb-glue-poc.tar
sha384sum /home/ec2-user/arb-glue-poc.tar
AWS_DEFAULT_REGION=us-east-2 \
  aws s3 cp /home/ec2-user/arb-glue-poc.tar \
  s3://agentcourt-data/arbattest/images/arb-glue-poc.tar
'
```

Validate the base image on `dev` after the build.  This command checks the selected example in the image that the glue image was based on.  It does not require runtime secrets.

```bash
ssh dev 'set -eu
AAR_EXAMPLE="${AAR_EXAMPLE:-ex01}"
sudo docker run --rm \
  arbattest-aar:dev \
  validate --complaint "examples/$AAR_EXAMPLE/complaint.md"
'
```

## Install The Exec Runner On `dev`

`/home/ec2-user/attest` on `dev` is the runtime launcher directory used by the AMI runner.  It is not the `attest` source checkout.  Copy generic exec files from `attest` and AAR-specific files from `adjudication/arb/tools`.  The current `run-aar.sh` accepts `AAR_EXAMPLE`, defaults to `ex01`, passes it to the glue container, and names default runs as `aar-$AAR_EXAMPLE-$STAMP`.

```bash
ssh dev 'mkdir -p /home/ec2-user/attest'
scp attest/exec.sh attest/parse_attestation.py adjudication/arb/tools/run-aar.sh dev:/home/ec2-user/attest/
ssh dev 'chmod 755 /home/ec2-user/attest/exec.sh /home/ec2-user/attest/run-aar.sh /home/ec2-user/attest/parse_attestation.py'
```

Keep the source branch checked in as well.  The runtime copy on `dev` is for execution, while `adjudication/arb/tools/run-aar.sh` records the AAR-specific script.  Commit and push `adjudication/arb` when AAR launcher behavior changes.

## Prepare The S3 Input Prefix

Stage only the small runtime secret files in S3.  The input prefix is separate from the output prefix so a verifier can see exactly which S3 input location the manifest names.  The example name is part of the prefix for readability, but the manifest uses the explicit `AAR_EXAMPLE` field as the selected case.

```bash
ssh dev 'set -eu
AAR_EXAMPLE="${AAR_EXAMPLE:-ex01}"
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
input_prefix="s3://agentcourt-data/arbattest/aar-inputs/$AAR_EXAMPLE-$stamp"
AWS_DEFAULT_REGION=us-east-2 aws s3 cp \
  /home/ec2-user/arbattest-secrets/auth.json \
  "$input_prefix/auth.json"
AWS_DEFAULT_REGION=us-east-2 aws s3 cp \
  /home/ec2-user/arbattest-secrets/keys.sh \
  "$input_prefix/keys.sh"
printf "INPUT_PREFIX=%s\n" "$input_prefix"
AWS_DEFAULT_REGION=us-east-2 aws s3 ls "$input_prefix/"
'
```

The `keys.sh` file must define `OPENROUTER_API_KEY`.  The glue script sources that file inside the container and exits before AAR starts if the variable is absent.  Do not place large case data under this prefix until the external case-packet input schema exists.

## Run The Attested AAR

The preferred command for a normal checked-in example is `adjudication/arb/tools/run-one-attested-arb.sh`.  It takes one argument, an example directory such as `examples/ex03`.  It validates that the directory exists and contains `complaint.md`, stages `auth.json` and `keys.sh` into a fresh S3 input prefix, chooses timestamped input and output prefixes, starts the exec AMI through the local driver, downloads the S3 artifacts, extracts the AAR archive, and verifies the attestation.

```bash
adjudication/arb/tools/run-one-attested-arb.sh examples/ex03
```

The lower-level local driver is `adjudication/arb/tools/run-arb-attested.py`.  It starts the exec AMI through `dev`, polls the S3 output prefix, writes progress and launcher logs under the local output directory, downloads all S3 artifacts into that directory, extracts the AAR archive, and can run verification.  If a terminal S3 artifact set appears while `exec.sh` is still polling, the driver terminates only the EC2 instance ID launched for that run and stops the remote launcher.

```bash
uv run adjudication/arb/tools/run-arb-attested.py \
  --example ex01 \
  --input-prefix s3://agentcourt-data/arbattest/aar-inputs/ex01-REPLACE_WITH_STAMP \
  --exec-ami ami-011f957fe91cf7b81 \
  --out-dir /tmp/aar-ex01-REPLACE_WITH_STAMP \
  --verify \
  --expected-pcr4 83AC49DFAA5D76939970E1568472FF463FBE90C4038D000D31F6C0520F583D1DD51CE0C103CEB26E4B773AAD99A4B3B4 \
  --expected-pcr7 98441C7F7625D10058C47683AEC486CE311C633235EB555593A7EE791121E3578AE72D04ECEF661F272D59058B77AF35
```

The output directory receives `run.env`, `progress.log`, `launcher.log`, the downloaded S3 artifacts, `attestation.txt` when verification runs, `verification.log` when verification runs, and either `aar-output/` or `aar-partial/` extracted from the archive.  The driver defaults to `DEV_HOST=dev`, `AWS_REGION=us-east-2`, `INSTANCE_TYPE=m5.4xlarge`, `IAM_INSTANCE_PROFILE=ec2-nix-builder`, `IMAGE_TAR_S3=s3://agentcourt-data/arbattest/images/arb-glue-poc.tar`, and `REMOTE_ATTEST_DIR=/home/ec2-user/attest`.

The manual command below is the same execution path without the local driver.  Run the exec AMI from `/home/ec2-user/attest` on `dev`.  Pass `RUN_ID` and `OUTPUT_PREFIX` explicitly so the verifier does not need to recover them from console output.  Set `AAR_EXAMPLE` to any checked-in example name.

```bash
ssh dev 'set -eu
cd /home/ec2-user/attest
AAR_EXAMPLE=ex01
INPUT_PREFIX=s3://agentcourt-data/arbattest/aar-inputs/ex01-REPLACE_WITH_STAMP
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
RUN_ID="${RUN_ID:-aar-$AAR_EXAMPLE-$stamp}"
OUTPUT_PREFIX="${OUTPUT_PREFIX:-s3://agentcourt-data/arbattest/aar-runs/$RUN_ID}"
env \
  AWS_DEFAULT_REGION=us-east-2 \
  INSTANCE_TYPE=m5.4xlarge \
  IAM_INSTANCE_PROFILE=ec2-nix-builder \
  POLL_ATTEMPTS=1800 \
  EXEC_ENV_VARS=INPUT_PREFIX,IMAGE_TAR_S3,AAR_EXAMPLE,RUN_ID,OUTPUT_PREFIX \
  INPUT_PREFIX="$INPUT_PREFIX" \
  IMAGE_TAR_S3=s3://agentcourt-data/arbattest/images/arb-glue-poc.tar \
  AAR_EXAMPLE="$AAR_EXAMPLE" \
  RUN_ID="$RUN_ID" \
  OUTPUT_PREFIX="$OUTPUT_PREFIX" \
  ./exec.sh ami-011f957fe91cf7b81 /home/ec2-user/attest/run-aar.sh
printf "RUN_ID=%s\n" "$RUN_ID"
printf "OUTPUT_PREFIX=%s\n" "$OUTPUT_PREFIX"
'
```

`exec.sh` prints the EC2 instance ID after launch and terminates the instance on normal exit.  The current launcher still depends on EC2 console output to notice `ATTESTATION END`, while the attestation record lives in S3.  If S3 contains a complete verified result and the launcher keeps polling, use the printed instance ID to inspect or terminate that instance.

## Download The Result For Verification

Use local AWS credentials when available.  This path keeps verification in the local workspace where `uv` is available for the parser.  The same commands work for any `RUN_ID` and `OUTPUT_PREFIX`.

```bash
RUN_ID=aar-ex01-REPLACE_WITH_STAMP
OUTPUT_PREFIX="s3://agentcourt-data/arbattest/aar-runs/$RUN_ID"
LOCAL="/tmp/$RUN_ID"
mkdir -p "$LOCAL"
aws s3 cp "$OUTPUT_PREFIX/" "$LOCAL/" --recursive
find "$LOCAL" -maxdepth 1 -type f -printf '%f\n' | sort
```

If only `dev` has S3 access, download there and copy the small artifact set back.  The successful archive path has five S3 objects, so this transfer should remain small.  A large object count means the archive path regressed and needs diagnosis before more runs.

```bash
RUN_ID=aar-ex01-REPLACE_WITH_STAMP
OUTPUT_PREFIX="s3://agentcourt-data/arbattest/aar-runs/$RUN_ID"
ssh dev "set -eu
LOCAL=/tmp/$RUN_ID
mkdir -p \"\$LOCAL\"
AWS_DEFAULT_REGION=us-east-2 aws s3 cp '$OUTPUT_PREFIX/' \"\$LOCAL/\" --recursive
find \"\$LOCAL\" -maxdepth 1 -type f -printf '%f\n' | sort
"
scp -r "dev:/tmp/$RUN_ID" /tmp/
```

The expected successful object list is:

```text
aar-output.tar.gz
attestation.b64
manifest.json
manifest.sha384
run.log
```

## Verify The Manifest And Archive

Run these checks from the local workspace root.  Set `AAR_EXAMPLE`, `OUTPUT_PREFIX`, and `LOCAL` to the run under review.  The script checks the manifest hash, the selected example, the output prefix, the run log hash, the archive hash, and the archive byte count.

```bash
set -eu
AAR_EXAMPLE=ex01
RUN_ID=aar-ex01-REPLACE_WITH_STAMP
OUTPUT_PREFIX="s3://agentcourt-data/arbattest/aar-runs/$RUN_ID"
LOCAL="/tmp/$RUN_ID"
cd "$LOCAL"
python3 - "$AAR_EXAMPLE" "$OUTPUT_PREFIX" <<'PY'
import hashlib
import json
import sys
from pathlib import Path

expected_example, expected_output = sys.argv[1:3]

def sha384(path: str) -> str:
    h = hashlib.sha384()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()

manifest = json.loads(Path("manifest.json").read_text())
checks = [
    ("manifest.sha384", sha384("manifest.json") == Path("manifest.sha384").read_text().strip()),
    ("mode", manifest.get("mode") == "aar"),
    ("aar_example", manifest.get("aar_example") == expected_example),
    ("output_prefix", manifest.get("output_prefix") == expected_output),
    ("archive_key", manifest.get("aar_archive_key") == expected_output.rstrip("/") + "/aar-output.tar.gz"),
    ("run.log sha384", sha384("run.log") == manifest.get("log_sha384")),
    ("archive sha384", sha384("aar-output.tar.gz") == manifest.get("aar_archive_sha384")),
    ("archive bytes", str(Path("aar-output.tar.gz").stat().st_size) == manifest.get("aar_archive_bytes")),
    ("container image id present", bool(manifest.get("container_image_id"))),
    ("container tar hash present", bool(manifest.get("container_image_tar_sha384"))),
]
failed = [name for name, ok in checks if not ok]
if failed:
    for name in failed:
        print(f"failed: {name}")
    sys.exit(1)
print("manifest and archive checks passed")
PY
```

Inspect the AAR result inside the archive.  A completed run should report `status=ok`; the resolution depends on the case.  The verified `ex01` run reported `resolution=demonstrated`.

```bash
tar -xOf aar-output.tar.gz ./local-run.json | python3 -m json.tool
```

Confirm that the archive excludes the large per-agent homes and staged OpenClaw Codex directories.  This check should print only `archive exclusion check passed`.  Any printed path means the archive contains data that should have stayed out of S3.

```bash
if tar -tzf aar-output.tar.gz | grep -E '^\./(pi-|openclaw-[^/]+-codex)(/|$)'; then
  echo "error: archive contains excluded runtime directory" >&2
  exit 1
fi
echo "archive exclusion check passed"
```

## Verify The Attestation

Run the attestation parser from the local workspace root.  The parser verifies the COSE signature and certificate chain, then prints the `User Data` field and all NitroTPM PCR values.  The current parser uses `uv`; `uv` is available locally at `/home/somebody/.local/bin/uv` in the verified environment and is absent on `dev`.

```bash
set -eu
RUN_ID=aar-ex01-REPLACE_WITH_STAMP
LOCAL="/tmp/$RUN_ID"
UV="${UV:-/home/somebody/.local/bin/uv}"
cd /media/hd2/src/arbattest
"$UV" run attest/parse_attestation.py "$LOCAL/attestation.b64" > "$LOCAL/attestation.txt"
sed -n '1,40p' "$LOCAL/attestation.txt"
```

Compare the attestation output against the manifest hash and the expected exec AMI PCR values.  These values apply to `ami-011f957fe91cf7b81`; replace them after rebuilding the exec AMI.  PCR12 is all zeros for the current verified run.

```bash
set -eu
RUN_ID=aar-ex01-REPLACE_WITH_STAMP
LOCAL="/tmp/$RUN_ID"
EXPECTED_PCR4=83AC49DFAA5D76939970E1568472FF463FBE90C4038D000D31F6C0520F583D1DD51CE0C103CEB26E4B773AAD99A4B3B4
EXPECTED_PCR7=98441C7F7625D10058C47683AEC486CE311C633235EB555593A7EE791121E3578AE72D04ECEF661F272D59058B77AF35
EXPECTED_PCR12=000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000
MANIFEST_SHA384="$(cat "$LOCAL/manifest.sha384")"
grep -q '^Signature: VALID' "$LOCAL/attestation.txt"
grep -q "^User Data: $MANIFEST_SHA384$" "$LOCAL/attestation.txt"
grep -q "^PCR  4: $EXPECTED_PCR4$" "$LOCAL/attestation.txt"
grep -q "^PCR  7: $EXPECTED_PCR7$" "$LOCAL/attestation.txt"
grep -q "^PCR 12: $EXPECTED_PCR12$" "$LOCAL/attestation.txt"
echo "attestation checks passed"
```

The reference `ex01` run `aar-ex01-20260612T001855Z` verified with manifest SHA-384 `ae52d9b5acccd76a45ce0e6c8f3cabf8e775ddb20e0761702fa1d73e15dffdcab080a0be859556170aaa3a23e9971f41` and archive SHA-384 `ce42ae939df866a2919f20ff8ccd5ffc86df0ffc0f7376b84811f9ae0a44dac8b664b4aaf0a7913b25677a2a7fc75bb0`.  Its attestation `User Data` matched the manifest hash.  That run predates the `aar_example` manifest field; use the manifest script above for runs made with the current glue image.

## Run Any Checked-In Arb

Add or select an example directory under `arb/examples/<name>` with a valid `complaint.md` and any case files needed by that complaint.  Push the `adjudication` `arbattest` branch so the Docker build can clone it, then rebuild and upload the glue image tar from `dev`.  Install the current `adjudication/arb/tools/run-aar.sh` in `/home/ec2-user/attest`, stage `auth.json` and `keys.sh` under a new S3 input prefix, run the exec AMI with `AAR_EXAMPLE=<name>`, and run the verification commands above against the resulting output prefix.

Use a fresh `RUN_ID` and `OUTPUT_PREFIX` for every run.  The recommended naming form is `aar-$AAR_EXAMPLE-$STAMP`, with `STAMP` from `date -u +%Y%m%dT%H%M%SZ`.  Timestamped prefixes keep failed, partial, and verified runs separate and make S3 cleanup decisions explicit.

The manifest is the boundary for later verification.  It names the selected example, input prefix, output prefix, image identity, image tar hash, log hash, and archive hash.  Verification should treat the manifest hash in the attestation `User Data` field, plus matching PCR values, as the link between the attested exec AMI and the exact S3 artifacts.

## First-Failure Checks

Read `run.log` first.  For a successful run, read it from the downloaded artifact directory.  For a failed AAR run, download `run.log` and `aar-partial.tar.gz` from the output prefix and inspect `local-run.json` inside the partial archive if it exists.

Use the first concrete failing line as the diagnostic start.  An output prefix without `manifest.json`, `manifest.sha384`, and `attestation.b64` has no verified attestation.  Do not infer success from console output when S3 artifacts disagree.

| Symptom | Cause already diagnosed | Fix already used |
| --- | --- | --- |
| Docker layer extraction fails with `no space left on device` on the exec AMI. | The exec AMI root filesystem is RAM-backed, and Docker writes into that RAM-backed filesystem. | Use `m5.4xlarge` for the verified path. |
| `OPENROUTER_API_KEY is required`. | `keys.sh` was absent from `INPUT_PREFIX`, unreadable, or did not define the variable. | Upload `keys.sh` to the input prefix and verify it defines `OPENROUTER_API_KEY`. |
| OpenClaw cannot read `/aar-codex/auth.json`. | The child container runs as user `node` and needs world-readable staged Codex auth in this private AMI flow. | Current AAR code stages the Codex home with mode `0777` and `auth.json` with mode `0666`. |
| OpenClaw reports a stream disconnect on the exec AMI while the same request works on `dev`. | The diagnosed exec path failed when child OpenClaw used Docker bridge networking and passed when it used host networking. | Current glue passes `--openclaw-network host`. |
| S3 prefix contains tens of thousands of AAR objects. | The old glue success path recursively uploaded the AAR output tree, including Pi package trees. | Current glue uploads one `aar-output.tar.gz` or one `aar-partial.tar.gz`. |
| `exec.sh` keeps polling after S3 has complete artifacts. | EC2 console output did not show the final marker even though S3 had the verified record. | Verify the S3 artifacts, then use the printed instance ID to inspect or terminate the instance. |

## Cleanup

Keep completed output prefixes that have been cited in notes or commits.  Delete failed experimental prefixes only after recording the cause and confirming that no later diagnosis depends on them.  The old recursive upload prefix was deleted after it was identified as an obsolete failure mode and the archive upload path replaced it.

Docker build cache and old image tars on `dev` can consume the root volume used by builds.  Check disk usage before a rebuild, especially after repeated `--no-cache` builds.  Remove obsolete rebuild artifacts only after confirming the current uploaded glue tar hash and the current source commit.
