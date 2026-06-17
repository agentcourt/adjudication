# ADC Docker Image Runbook

## Scope

This runbook covers the ADC base image from `adc/Dockerfile`, the ADC attested workload image from `adc/Dockerfile.glue`, and the exec run launched by `adc/tools/run-adc.sh`.  The base image contains the compiled ADC runtime, the Lean engine, the `adjudication` source tree, the Docker CLI, and an embedded Pi juror root filesystem.  The attested workload image adds AWS CLI, `nitro-tpm-attest`, TSS runtime libraries, and `adc/attest/exec-container-entrypoint.sh`.

The current ADC attested path supports complaint input only.  The local driver packages a local `complaint_path` and its linked local Markdown files into `case.tar.gz` and `case-packet.json`, uploads those objects under `INPUT_PREFIX`, and passes the extracted complaint path to `adc run`.  Scenario input, examples, and local runtime overrides are local-service features until the attested path has explicit support for them.

## Files

| Item | Location | Role |
| --- | --- | --- |
| ADC base image | `adc/Dockerfile` | Builds ADC, the Lean engine, Docker CLI support, and the embedded Pi root filesystem. |
| Attested workload image | `adc/Dockerfile.glue` | Adds AWS CLI, `nitro-tpm-attest`, TSS libraries, and the S3 artifact flow. |
| Exec container entrypoint | `adc/attest/exec-container-entrypoint.sh` | Runs ADC, uploads live events, archives output, writes the manifest, obtains the TPM attestation, and uploads artifacts to S3. |
| Exec script | `adc/tools/run-adc.sh` | Loads `adc-glue:poc` on the exec AMI and starts the attested workload container. |
| Local driver | `adc/tools/run-adc-attested.py` | Builds the complaint packet, starts `exec.sh` through `dev`, polls S3, downloads artifacts, extracts ADC output, and verifies the attestation. |
| Container proof script | `adc/tools/run-container-poc.sh` | Runs the attested workload image in attestation-only mode. |
| Clerk service | `adc runtime service` | Starts attested ADC through the same `/clerk/v1/cases` API used for local ADC cases. |

The generic exec AMI launcher lives in the `attest` repository.  The launcher directory on `dev` is `/home/ec2-user/attest`, and that directory must contain `exec.sh`, `parse_attestation.py`, and `run-adc.sh`.  The source checkout used to build ADC images must be separate from the launcher directory.

## Dev Host And AWS Requirements

The generic `dev` host requirements for the exec AMI launcher live in [Dev Host Requirements](../../attest/dev-host.md).  Read that document before building or launching through `attest/exec.sh`.  It covers the base x86_64 host, Nix daemon setup, AWS CLI, EC2 permissions, EBS direct snapshot permissions, role passing, default VPC assumptions, disk requirements, and verification commands.

ADC-specific requirements live in [Attested ADC Dev Host Requirements](docs/attested-dev-host.md).  The ADC document adds Docker image build requirements, S3 prefixes, secret file locations, instance profile requirements, expected PCR values, and operational checks.  The verified first path uses `us-east-2`, `m5.4xlarge`, `ec2-nix-builder`, `s3://agentcourt-data/arbattest/images/adc-glue-poc.tar`, `s3://agentcourt-data/arbattest/adc-inputs/`, and `s3://agentcourt-data/arbattest/adc-runs/`.

## Attestation Record

The attestation record lives in S3.  The local driver uses stdout from `exec.sh` only for progress and instance-id discovery, then reads terminal artifacts from the configured S3 output prefix.  A successful run leaves `run.log`, `manifest.json`, `manifest.sha384`, `attestation.b64`, `adc-output.tar.gz`, and a live `events.ndjson` object under `OUTPUT_PREFIX`.

The live `events.ndjson` object supports monitoring while ADC is running.  The verified event log remains the copy inside `adc-output.tar.gz`, because the manifest binds the archive hash.  `manifest.sha384` contains the SHA-384 hash of `manifest.json`, and the exec container passes that file to `nitro-tpm-attest --user-data`, so the attestation user data must equal the manifest hash.

The manifest binds the input mode, case-packet object hashes, input prefix, output prefix, exec AMI, instance ID, workload image ID, workload image tar hash, run log hash, and ADC output archive hash.  If ADC exits nonzero, the container uploads `run.log`, `adc-partial.tar.gz`, and any available live events, then exits with failure.  A failed ADC run does not produce a verified completion record through the current driver path.

## Build Locally

Run the base image build from `/media/hd2/src/arbattest`.  The Dockerfile clones the repository named by `ADJUDICATION_REPO` and checks out `ADJUDICATION_REF`; it does not copy the current local checkout into the image.  Use `--no-cache` after pushing branch changes, because Docker can otherwise keep an older cloned source layer.

```bash
docker build --no-cache \
  -t arbattest-adc:local \
  -f adjudication/adc/Dockerfile \
  adjudication/adc
```

Build the attested workload image from the same directory.  The glue build uses the ADC base image as its parent and adds the attestation entrypoint.  The image name used by the exec script is `adc-glue:poc`.

```bash
docker build --no-cache \
  --build-arg ADC_IMAGE=arbattest-adc:local \
  -t adc-glue:poc \
  -f adjudication/adc/Dockerfile.glue \
  adjudication/adc
```

Validate the base image before a long run.  This command checks that the image can parse an existing complaint and invoke the ADC CLI.  It does not start OpenClaw, Pi, Docker-in-Docker, or the exec AMI.

```bash
docker run --rm \
  arbattest-adc:local \
  validate --scenario examples/ex1/scenario.json
```

## Build And Upload On `dev`

Build on `dev` from the source checkout, not from `/home/ec2-user/attest`.  Save the glue image as a Docker archive and upload it to S3.  Record the SHA-384 hash because the exec entrypoint records that value in each manifest.

```bash
ssh dev 'set -eu
cd /home/ec2-user/adjudication-build-2361886
git pull --ff-only origin arbattest
cd adc
sudo docker build --no-cache \
  -t arbattest-adc:dev \
  -f Dockerfile \
  .
sudo docker build --no-cache \
  --build-arg ADC_IMAGE=arbattest-adc:dev \
  -t adc-glue:poc \
  -f Dockerfile.glue \
  .
sudo docker save adc-glue:poc -o /home/ec2-user/adc-glue-poc.tar
sudo chown ec2-user:ec2-user /home/ec2-user/adc-glue-poc.tar
sha384sum /home/ec2-user/adc-glue-poc.tar
AWS_DEFAULT_REGION=us-east-2 \
  aws s3 cp /home/ec2-user/adc-glue-poc.tar \
  s3://agentcourt-data/arbattest/images/adc-glue-poc.tar
'
```

Install the runner file into the launcher directory.  This directory is the runtime directory used by `exec.sh`, while the checked-in source remains under `adjudication/adc/tools`.  Keep the runtime copy current whenever `run-adc.sh` changes.

```bash
ssh dev 'mkdir -p /home/ec2-user/attest'
scp attest/exec.sh attest/parse_attestation.py adjudication/adc/tools/run-adc.sh dev:/home/ec2-user/attest/
ssh dev 'chmod 755 /home/ec2-user/attest/exec.sh /home/ec2-user/attest/run-adc.sh /home/ec2-user/attest/parse_attestation.py'
```

## Prepare Inputs

`INPUT_PREFIX` must contain `auth.json` and `keys.sh`.  `auth.json` is the Codex auth file used by OpenClaw, and `keys.sh` must assign or export `OPENROUTER_API_KEY` for Pi jurors.  The local driver uploads `case.tar.gz` and `case-packet.json` into the same prefix before launching the exec AMI.

```bash
ssh dev 'set -eu
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
input_prefix="s3://agentcourt-data/arbattest/adc-inputs/adc-$stamp"
AWS_DEFAULT_REGION=us-east-2 aws s3 cp \
  /home/ec2-user/arbattest-secrets/auth.json \
  "$input_prefix/auth.json"
AWS_DEFAULT_REGION=us-east-2 aws s3 cp \
  /home/ec2-user/arbattest-secrets/keys.sh \
  "$input_prefix/keys.sh"
printf "INPUT_PREFIX=%s\n" "$input_prefix"
'
```

The complaint packet is deterministic for the same complaint and linked files.  ADC reads Markdown links with the same loader used by local complaint setup, and every linked local file must live under the complaint directory.  The packet keeps the complaint at `case/<complaint basename>` and linked files under their same relative paths, so existing relative Markdown links continue to work after extraction.

## Run Through The Local Driver

The local driver is the normal operator path.  It builds the complaint packet locally, uploads packet objects through `dev`, starts the exec AMI, polls the output prefix, downloads terminal artifacts, extracts `adc-output.tar.gz`, and verifies the attestation when `--verify` is set.  Use fresh input and output prefixes for each run.

```bash
uv run adjudication/adc/tools/run-adc-attested.py \
  --case-id adc-custom-REPLACE_WITH_STAMP \
  --run-id adc-custom-REPLACE_WITH_STAMP \
  --complaint adjudication/adc/examples/ex1/complaint.md \
  --input-prefix s3://agentcourt-data/arbattest/adc-inputs/adc-REPLACE_WITH_STAMP \
  --exec-ami ami-011f957fe91cf7b81 \
  --out-dir /tmp/adc-custom-REPLACE_WITH_STAMP \
  --verify \
  --expected-pcr4 83AC49DFAA5D76939970E1568472FF463FBE90C4038D000D31F6C0520F583D1DD51CE0C103CEB26E4B773AAD99A4B3B4 \
  --expected-pcr7 98441C7F7625D10058C47683AEC486CE311C633235EB555593A7EE791121E3578AE72D04ECEF661F272D59058B77AF35
```

The output directory receives `run.env`, `progress.log`, `launcher.log`, `case.tar.gz`, `case-packet.json`, downloaded S3 artifacts, `attestation.txt`, `verification.log`, and either `adc-output/` or `adc-partial/`.  A successful verified run has `adc-output/run.json`, `adc-output/events.ndjson`, `adc-output/digest.md`, and any submitted evidence files.  The driver defaults to `DEV_HOST=dev`, `AWS_REGION=us-east-2`, `INSTANCE_TYPE=m5.4xlarge`, `IAM_INSTANCE_PROFILE=ec2-nix-builder`, `IMAGE_TAR_S3=s3://agentcourt-data/arbattest/images/adc-glue-poc.tar`, and `REMOTE_ATTEST_DIR=/home/ec2-user/attest`.

## Clerk Service

The clerk service can start the same attested path with `execution.mode = "attested"`.  Start the service with attested defaults, then each create request supplies a complaint path and any per-run attestation overrides.  Verification is required before the service marks an attested ADC case `completed`.

```bash
.bin/adc service \
  --listen 127.0.0.1:19870 \
  --output-root out/adc-service \
  --adc-bin .bin/adc \
  --engine .bin/adcengine \
  --attested-driver "$(pwd)/tools/run-adc-attested.py" \
  --attested-uv uv \
  --attested-input-prefix s3://agentcourt-data/arbattest/adc-inputs/adc-REPLACE_WITH_STAMP \
  --attested-output-root s3://agentcourt-data/arbattest/adc-runs \
  --attested-exec-ami ami-011f957fe91cf7b81 \
  --attested-expected-pcr4 83AC49DFAA5D76939970E1568472FF463FBE90C4038D000D31F6C0520F583D1DD51CE0C103CEB26E4B773AAD99A4B3B4 \
  --attested-expected-pcr7 98441C7F7625D10058C47683AEC486CE311C633235EB555593A7EE791121E3578AE72D04ECEF661F272D59058B77AF35
```

Create an attested ADC case through the same service API.  The request shape matches the local clerk create API for the case input: `complaint_path` is the complaint path, `case_id` and `run_id` are optional, and `out_dir` must remain under the service output root.  The exec entrypoint starts OpenClaw lawyer containers with `--openclaw-network host`, matching the verified ARB and AARD exec topology.  Attested ADC currently rejects `scenario_path` and local runtime fields such as model overrides, Docker commands, jury overrides, and OpenClaw options.

```bash
curl -sS -X POST http://127.0.0.1:19870/clerk/v1/cases \
  -H 'content-type: application/json' \
  --data '{
    "mode": "run",
    "case_id": "adc-attested-ex1",
    "complaint_path": "examples/ex1/complaint.md",
    "out_dir": "out/adc-service/adc-attested-ex1",
    "execution": {
      "mode": "attested",
      "attestation": {
        "verify": true
      }
    }
  }'
```

Monitor the service record and event stream through HTTP.  While the driver is running, `/attestation/events` reads the live S3 `events.ndjson` object when the local output copy is not present.  After completion, artifact, result, and evidence routes read from the extracted `adc-output/` directory.

```bash
curl -sS http://127.0.0.1:19870/clerk/v1/cases/adc-attested-ex1
curl -sS http://127.0.0.1:19870/clerk/v1/cases/adc-attested-ex1/attestation/events
curl -sS http://127.0.0.1:19870/clerk/v1/cases/adc-attested-ex1/artifacts
curl -sS http://127.0.0.1:19870/clerk/v1/cases/adc-attested-ex1/result
```

## Verification

A verified ADC run checks the manifest hash, the ADC archive hash, `run.log` hash, the attestation signature and certificate chain, Nitro TPM user data, and expected PCR values.  The expected PCR values in this runbook correspond to the current Docker-enabled exec AMI and must change when that AMI changes.  The local driver writes `verification.log`; the service requires that file and a readable extracted `adc-output/run.json` before marking the case completed.
