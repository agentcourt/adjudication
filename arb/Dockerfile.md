# AAR Docker Image Runbook

## Purpose

This runbook describes the `aar` Docker image built from `adjudication/arb/Dockerfile`.  The image exists to run `aar` directly for arbitration cases such as `arb/examples/ex01`, including the OpenClaw lawyer containers and Pi council containers that `aar run` starts during a full run.  The image also embeds the Pi council root filesystem so a host does not need to build the Pi council image before running `aar`.

The image builds from the public `jsmorph/adjudication` repository and the `arbattest` branch by default.  It contains the cloned `adjudication` tree, the compiled `aar` and `aarengine` binaries, a Docker CLI, and an embedded Pi council root filesystem tar.  The image is independent of the `attest` repository; attestation work runs later through the `attest` tools and the exec AMI.

## Image Contents

| Item | Location in image | Source |
| --- | --- | --- |
| `aar` binary | `/opt/adjudication/arb/.bin/aar` | Built by `make build` from `jsmorph/adjudication`, branch `arbattest`. |
| `aarengine` binary | `/opt/adjudication/arb/.bin/aarengine` | Built by `make build` from `jsmorph/adjudication`, branch `arbattest`. |
| `adjudication` source tree | `/opt/adjudication` | Cloned during the image build. |
| Docker CLI | `/usr/local/bin/docker` | Copied from `docker:26-cli`. |
| Pi root filesystem tar | `/opt/images/agentcourt-pi-sandbox-rootfs.tar` | Built from the Pi runtime recipe in the image build. |
| Entrypoint | `/usr/local/bin/aar-entrypoint` | Imports the embedded Pi root filesystem into the host Docker daemon when needed, then execs `aar`. |

The Dockerfile accepts these build arguments:

| Build argument | Default | Meaning |
| --- | --- | --- |
| `ADJUDICATION_REPO` | `https://github.com/jsmorph/adjudication.git` | Git repository cloned during the build. |
| `ADJUDICATION_REF` | `arbattest` | Branch or ref checked out during the build. |
| `PI_CODING_AGENT_PACKAGE` | `@earendil-works/pi-coding-agent@0.78.0` | Pi package installed into the embedded council root filesystem. |

The final image has these runtime environment defaults:

| Environment variable | Default | Meaning |
| --- | --- | --- |
| `AAR_PI_IMAGE` | `agentcourt-pi-sandbox:latest` | Host Docker image tag used for council member containers. |
| `AAR_PI_ROOTFS_TAR` | `/opt/images/agentcourt-pi-sandbox-rootfs.tar` | Root filesystem tar imported into the host Docker daemon. |

## Runtime Model

`aar run` starts child containers.  The `aar` process runs inside the `arbattest-aar` container, and the child OpenClaw and Pi containers run through the host Docker daemon.  The parent container therefore needs the host Docker socket mounted at `/var/run/docker.sock`.

The run command passes `--docker docker` and `--podman docker`.  This makes both child-container roles use Docker through the mounted host socket.  OpenClaw uses `ghcr.io/openclaw/openclaw:latest`; Pi uses `agentcourt-pi-sandbox:latest`, which the entrypoint imports from the embedded root filesystem tar if the host Docker daemon lacks that tag.

Use host networking for the parent container.  The AAR MCP server runs inside that parent container, and the parent must expose its local ports on the host network.  The proved local and direct `dev` commands both used `--network host` on the parent container.

The child OpenClaw containers use Docker bridge networking by default.  The attested exec path is different: the Docker-enabled exec AMI reproduced a one-line embedded Codex stream failure when child OpenClaw used Docker bridge networking, and the same request succeeded when child OpenClaw used Docker host networking.  The glue AAR command therefore passes `--openclaw-network host`; in that mode `aar run` uses `127.0.0.1` as the default Docker MCP host.

The output root must be mounted at the same absolute path inside the parent container that exists on the host.  `aar run` creates absolute paths for staged Codex homes and Pi homes, and the host Docker daemon then mounts those paths into child containers.  If the parent container sees `/out` but the host only has `/home/ec2-user/aar-out`, the child-container mounts will refer to the wrong host path.

## Attested S3 Output

The glue image writes a small S3 prefix for each attested run.  Successful AAR mode uploads `run.log`, `aar-output.tar.gz`, `manifest.json`, `manifest.sha384`, and `attestation.b64`.  The manifest records the AAR archive S3 key, byte count, and SHA-384 hash before the manifest hash is passed to `nitro-tpm-attest --user-data`.

The AAR archive excludes per-agent working homes such as `pi-C*` and staged OpenClaw Codex directories.  It keeps the case packet, logs, evidence store, event log, transcript, digest, work notes, and `local-run.json`.  This keeps S3 object counts small while preserving the artifacts needed to inspect the run.

If `aar run` exits nonzero, the glue image uploads `run.log` and `aar-partial.tar.gz`, then exits with the AAR status.  It does not create a manifest or attestation for a failed AAR run.

## Required Inputs

The image does not contain runtime secrets.  A full `ex01` run needs a Codex auth JSON file for OpenClaw lawyers and an OpenRouter key for the Pi council.

| Input | Local path used so far | Remote path used so far | Use |
| --- | --- | --- | --- |
| Codex auth JSON | `/media/hd2/src/arbattest/tmp/auth.json` | `/home/ec2-user/arbattest-secrets/auth.json` | Mounted read-only as `/run/secrets/codex-auth.json`. |
| OpenRouter key file | `/media/hd2/src/arbattest/tmp/keys.sh` | `/home/ec2-user/arbattest-secrets/keys.sh` | Sourced before `docker run`; provides `OPENROUTER_API_KEY`. |
| Docker socket | `/var/run/docker.sock` | `/var/run/docker.sock` | Lets the parent `aar` container start child containers through the host daemon. |
| Output root | `/media/hd2/src/arbattest/aar-out` | `/home/ec2-user/aar-out` | Stores run artifacts and logs. |

The `keys.sh` file must export or assign `OPENROUTER_API_KEY`.  The run command passes the variable by name with `-e OPENROUTER_API_KEY`, so the key value stays out of the command line.  On a remote host where `docker run` uses `sudo`, use `sudo --preserve-env=OPENROUTER_API_KEY docker run`; otherwise `sudo` can drop the sourced variable before Docker receives it.

## Building Locally

Run the build from `/media/hd2/src/arbattest`.  The build does not copy local source into the image; it clones the public repository named by `ADJUDICATION_REPO` and `ADJUDICATION_REF`.  Use `--no-cache` after pushing branch changes, because a cached `git clone` layer can otherwise preserve an older branch tip.

```bash
docker build --no-cache \
  -t arbattest-aar:local \
  -f adjudication/arb/Dockerfile \
  adjudication/arb
```

The proved local image after the cleanup fix was `sha256:eadf5c4f91c08653e604c9ce8049eaafd43a7098a097a1cc2fa721370fc5451b` on `arm64`.  The build installs Lean through `elan`, compiles `aar`, installs Node and the Pi package into a separate root filesystem stage, exports that root filesystem as a tar, and copies the Docker CLI into the final image.

Validate that the image can run `aar`:

```bash
docker run --rm \
  arbattest-aar:local \
  validate --complaint examples/ex01/complaint.md
```

The expected output is:

```text
ok
```

## Running `ex01` Locally

The workspace script `run-local-ex01.sh` contains the proved local command.  It checks that `tmp/auth.json`, `tmp/keys.sh`, and `/var/run/docker.sock` exist, sources `tmp/keys.sh`, creates a timestamped output directory under `aar-out`, and writes the `aar run` output to a sibling log file.

```bash
./run-local-ex01.sh
```

The script prints the output directory and log path before running.  A completed run prints those paths again.  The first completed local run used `/media/hd2/src/arbattest/aar-out/ex01-local-20260611T134840Z` and produced `status=ok` and `resolution=demonstrated`.

The script expands to this command shape:

```bash
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
  --docker docker \
  --podman docker \
  --pi-image agentcourt-pi-sandbox:latest \
  ex01 \
  >"$log" 2>&1
```

The `--user` and `--group-add` arguments make files in the output directory belong to the host user while preserving access to the mounted Docker socket.  The `--network host` argument applies to the parent `arbattest-aar` container.  The `--podman docker` argument makes the Pi council run through Docker, matching the embedded Pi image import performed by the entrypoint.

## Inspecting Local Results

Check the result file after the command exits:

```bash
sed -n '1,220p' aar-out/ex01-local-TIMESTAMP/local-run.json
```

A successful run has this shape:

```json
{
  "error": "",
  "failure": null,
  "resolution": "demonstrated",
  "status": "ok"
}
```

The run output should contain `complaint.md`, `policy.json`, `runtime.json`, `run.json`, `evidence-manifest.json`, `state.json`, `council.json`, `transcript.md`, `digest.md`, `local-run.json`, `events.ndjson`, `work-notes.ndjson`, `logs/`, `evidence-store/`, OpenClaw PID files, and Pi member home directories.  The exact evidence hashes and model responses vary by run.  The cleanup code should remove runtime credential files from each new run output.

Check for retained runtime credential files:

```bash
find aar-out/ex01-local-TIMESTAMP \
  \( -name .mcp.json -o -path '*/.pi/agent/auth.json' \) \
  -print
```

The command should print no paths for runs made with the cleanup commit.  Earlier completed output directories may still contain those files if they were created before the cleanup fix.  Do not treat older directories as evidence about the current image.

## Building on `dev`

The remote host used so far is `dev`, an Amazon Linux 2023 `amd64` instance.  Docker must be installed and running on that host.  The proved setup used Docker server version `25.0.13`.

Build the image on `dev` from the public branch:

```bash
ssh dev 'set -eu
tmpdir="$(mktemp -d /tmp/arbattest-adjudication.XXXXXX)"
git clone --branch arbattest --depth 1 https://github.com/jsmorph/adjudication.git "$tmpdir/adjudication"
sudo docker build --no-cache \
  -t arbattest-aar:dev \
  -f "$tmpdir/adjudication/arb/Dockerfile" \
  "$tmpdir/adjudication/arb"
sudo docker image inspect arbattest-aar:dev \
  --format "{{.Id}} {{.Architecture}} {{.Size}}"
'
```

The proved `dev` image was `sha256:c09ce383d82624ba40903780e18e82cab788cef4b81fb8e7ad7d797b59dcf9fe`, architecture `amd64`, size `1468421848`.  The first build attempt on `dev` failed because the root volume was full.  Nix garbage collection removed 1,959 unreferenced store paths and freed 84.9 GiB before the successful build.

Validate the remote image:

```bash
ssh dev 'sudo docker run --rm \
  arbattest-aar:dev \
  validate --complaint examples/ex01/complaint.md'
```

The expected output is:

```text
ok
```

## Building the Glue Image on `dev`

The attested exec path uses the glue image rather than the base AAR image by itself.  Run the build from the `arb` directory, or pass the `arb/Dockerfile` path and the `arb` directory as the Docker build context.

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

After upload, `s3://agentcourt-data/arbattest/images/arb-glue-poc.tar` is the image file used by `run-aar.sh` on the exec AMI.  Recompute and record the SHA-384 after every rebuild because the exec launcher uses the tar as the image input.

The rebuild from commit `d338c32` produced `arbattest-aar:dev` as `sha256:72775dddf4cc1b3dcf77970443801d98c2f9740d6576bf655c4fa33cc41c035f` and `arb-glue:poc` as `sha256:07ee87e51928468e382851ac72ec92062ea7794116652a312a5c32bfab26c2a1`.  The uploaded glue tar has SHA-384 `fbfb459dd3b5b2e73763ac98e424342a56b5a82fe3624bc0c940db7d2e3d95f628a7e9d99e212ab28bb680ad9d040133`.

## Preparing Remote Secrets

Copy only the small runtime secret files to `dev`.  The paths used so far are under `/home/ec2-user/arbattest-secrets`.

```bash
ssh dev 'mkdir -p ~/arbattest-secrets && chmod 700 ~/arbattest-secrets'
scp tmp/auth.json tmp/keys.sh dev:~/arbattest-secrets/
ssh dev 'chmod 600 ~/arbattest-secrets/auth.json ~/arbattest-secrets/keys.sh'
```

The remote run command sources `keys.sh` on `dev` and mounts `auth.json` read-only into the parent container.  It does not copy the local `examples/ex01` tree, because the image already contains the `adjudication` checkout from the public branch.  It also does not copy large run data to `dev`.

## Running `ex01` on `dev`

Run the remote command through SSH.  It creates `/home/ec2-user/aar-out`, makes a timestamped run directory, preserves `OPENROUTER_API_KEY` across `sudo`, mounts the host Docker socket, and runs both OpenClaw and Pi child containers through host Docker.

```bash
ssh dev 'set -eu
. "$HOME/arbattest-secrets/keys.sh"
output_root="$HOME/aar-out"
mkdir -p "$output_root"
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
out="$output_root/ex01-dev-$stamp"
log="$output_root/ex01-dev-$stamp.log"
uid="$(id -u)"
gid="$(id -g)"
sock_gid="$(stat -c %g /var/run/docker.sock)"
sudo --preserve-env=OPENROUTER_API_KEY docker run --rm --network host \
  --user "$uid:$gid" \
  --group-add "$sock_gid" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$output_root:$output_root" \
  -v "$HOME/arbattest-secrets/auth.json:/run/secrets/codex-auth.json:ro" \
  -e OPENROUTER_API_KEY \
  arbattest-aar:dev \
  run \
  --out-dir "$out" \
  --openclaw-auth codex \
  --openclaw-codex-auth /run/secrets/codex-auth.json \
  --docker docker \
  --podman docker \
  --pi-image agentcourt-pi-sandbox:latest \
  ex01 \
  >"$log" 2>&1
printf "%s\n" "$out"
printf "%s\n" "$log"
'
```

The proved `dev` run completed under `/home/ec2-user/aar-out/ex01-dev-20260611T144804Z`.  Its `local-run.json` reported `status=ok` and `resolution=demonstrated`.  The run took long enough that the SSH command may produce no output until `aar` exits, because stdout and stderr from `aar` are redirected to the remote log.

## Inspecting Remote Results

Read the recorded result:

```bash
ssh dev 'sed -n "1,220p" /home/ec2-user/aar-out/ex01-dev-TIMESTAMP/local-run.json'
```

Check for retained runtime credential files:

```bash
ssh dev 'find /home/ec2-user/aar-out/ex01-dev-TIMESTAMP \
  \( -name .mcp.json -o -path "*/.pi/agent/auth.json" \) \
  -print'
```

The cleanup check returned no paths for `/home/ec2-user/aar-out/ex01-dev-20260611T144804Z`.  The output directory still keeps the Pi home directories, logs, transcript, digest, evidence store, and run metadata, because those files are run artifacts rather than runtime credential files.  The remote Docker daemon keeps `agentcourt-pi-sandbox:latest` after the first run imports it from the embedded root filesystem tar.

Check the images on `dev`:

```bash
ssh dev 'sudo docker images'
```

The expected images after a remote run are `arbattest-aar:dev` and `agentcourt-pi-sandbox:latest`.  The OpenClaw image may also appear if the host Docker daemon had to pull it during the run.  Docker build cache can consume several GiB after building the image, so record disk state before large follow-up work.

## First-Failure Checks

If `aar run` exits nonzero, read the run log first:

```bash
sed -n '1,220p' aar-out/ex01-local-TIMESTAMP.log
```

For `dev`:

```bash
ssh dev 'sed -n "1,220p" /home/ec2-user/aar-out/ex01-dev-TIMESTAMP.log'
```

A failure before case start may leave no `local-run.json`.  A failure after case start may write `local-run.json` with `status`, `error`, and `failure` fields.  Use the first concrete error in the log or result file as the diagnostic starting point.

These problems have already occurred:

| Symptom | Root cause | Fix |
| --- | --- | --- |
| Docker build failed with `No space left on device` on `dev`. | The root filesystem was nearly full. | Free disk space on `dev`; the completed fix was Nix garbage collection of unreferenced store paths. |
| Remote run failed with `OPENROUTER_API_KEY is required for Pi council`. | `sudo docker run` did not preserve the sourced environment variable. | Use `sudo --preserve-env=OPENROUTER_API_KEY docker run`. |
| New image build used old branch contents. | Docker reused the cached `git clone` build layer. | Build with `--no-cache` after pushing branch changes. |

Do not delete or sanitize completed output directories unless that is the selected task.  For the current branch, the code removes runtime credential files from new run outputs during cleanup.
