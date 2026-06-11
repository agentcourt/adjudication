#!/bin/sh
set -eu

mode="${ARB_GLUE_MODE:-attest-only}"
work_root="${ARB_GLUE_WORK_ROOT:-/var/lib/arb-glue}"
run_id="${RUN_ID:-run-$(date -u +%Y%m%dT%H%M%SZ)}"
output_prefix="${OUTPUT_PREFIX:?OUTPUT_PREFIX is required}"
input_prefix="${INPUT_PREFIX:-}"

export TPM2TOOLS_TCTI="${TPM2TOOLS_TCTI:-device:/dev/tpmrm0}"
export TSS2_TCTI="${TSS2_TCTI:-device:/dev/tpmrm0}"
export TPM_DEVICE="${TPM_DEVICE:-/dev/tpm0}"

case "$output_prefix" in
    s3://*) ;;
    *) echo "error: OUTPUT_PREFIX must start with s3://" >&2; exit 1 ;;
esac

output_prefix="${output_prefix%/}"
run_dir="$work_root/$run_id"
mkdir -p "$run_dir"

log="$run_dir/run.log"
manifest="$run_dir/manifest.json"
manifest_hash_file="$run_dir/manifest.sha384"
attestation="$run_dir/attestation.b64"

imds_token="$(curl -fsS -X PUT \
    -H 'X-aws-ec2-metadata-token-ttl-seconds: 21600' \
    http://169.254.169.254/latest/api/token 2>/dev/null || true)"

imds_get() {
    path="$1"
    if [ -n "$imds_token" ]; then
        curl -fsS -H "X-aws-ec2-metadata-token: $imds_token" "http://169.254.169.254/$path" 2>/dev/null || true
    else
        curl -fsS "http://169.254.169.254/$path" 2>/dev/null || true
    fi
}

json_string() {
    printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

start_time="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
instance_id="$(imds_get latest/meta-data/instance-id)"
ami_id="$(imds_get latest/meta-data/ami-id)"

case "$mode" in
    attest-only)
        {
            printf 'mode=%s\n' "$mode"
            printf 'run_id=%s\n' "$run_id"
            printf 'instance_id=%s\n' "$instance_id"
            printf 'ami_id=%s\n' "$ami_id"
        } > "$log"
        ;;
    aar)
        : "${INPUT_PREFIX:?INPUT_PREFIX is required for ARB_GLUE_MODE=aar}"
        secrets_dir="$run_dir/secrets"
        aar_out="$run_dir/aar"
        mkdir -p "$secrets_dir" "$aar_out"
        aws s3 cp "$input_prefix/auth.json" "$secrets_dir/auth.json" --no-progress
        aws s3 cp "$input_prefix/keys.sh" "$secrets_dir/keys.sh" --no-progress
        . "$secrets_dir/keys.sh"
        : "${OPENROUTER_API_KEY:?OPENROUTER_API_KEY is required}"
        export OPENROUTER_API_KEY
        set +e
        /usr/local/bin/aar-entrypoint \
            run \
            --out-dir "$aar_out" \
            --openclaw-auth codex \
            --openclaw-codex-auth "$secrets_dir/auth.json" \
            --docker docker \
            --podman docker \
            --pi-image agentcourt-pi-sandbox:latest \
            ex01 \
            > "$log" 2>&1
        aar_status=$?
        set -e
        if [ "$aar_status" -ne 0 ]; then
            aws s3 cp "$log" "$output_prefix/run.log" --no-progress
            partial_upload_status=0
            aws s3 cp --recursive "$aar_out" "$output_prefix/aar-partial/" --no-progress || partial_upload_status=$?
            if [ "$partial_upload_status" -ne 0 ]; then
                echo "error: failed to upload partial AAR output after aar exit status $aar_status" >&2
                exit "$partial_upload_status"
            fi
            echo "error: aar failed with exit status $aar_status" >&2
            exit "$aar_status"
        fi
        if ! aws s3 cp --recursive "$aar_out" "$output_prefix/aar/" --no-progress; then
            aws s3 cp "$log" "$output_prefix/run.log" --no-progress
            echo "error: failed to upload AAR output" >&2
            exit 1
        fi
        ;;
    *)
        echo "error: unsupported ARB_GLUE_MODE: $mode" >&2
        exit 1
        ;;
esac

end_time="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
set -- $(sha384sum "$log")
log_sha384="$1"

cat > "$manifest" <<EOF
{
  "run_id": "$(json_string "$run_id")",
  "mode": "$(json_string "$mode")",
  "started_at": "$(json_string "$start_time")",
  "finished_at": "$(json_string "$end_time")",
  "instance_id": "$(json_string "$instance_id")",
  "ami_id": "$(json_string "$ami_id")",
  "input_prefix": "$(json_string "$input_prefix")",
  "output_prefix": "$(json_string "$output_prefix")",
  "container_image_id": "$(json_string "${ARB_GLUE_IMAGE_ID:-}")",
  "container_image_tar_sha384": "$(json_string "${ARB_GLUE_IMAGE_TAR_SHA384:-}")",
  "log_sha384": "$(json_string "$log_sha384")"
}
EOF

set -- $(sha384sum "$manifest")
manifest_sha384="$1"
printf '%s\n' "$manifest_sha384" > "$manifest_hash_file"

attestation_raw="$run_dir/attestation.bin"
nitro-tpm-attest --user-data "$manifest_hash_file" > "$attestation_raw"
base64 "$attestation_raw" | tr -d '\n' > "$attestation"
printf '\n' >> "$attestation"

aws s3 cp "$log" "$output_prefix/run.log"
aws s3 cp "$manifest" "$output_prefix/manifest.json"
aws s3 cp "$manifest_hash_file" "$output_prefix/manifest.sha384"
aws s3 cp "$attestation" "$output_prefix/attestation.b64"

printf 'OUTPUT_PREFIX=%s\n' "$output_prefix"
printf 'MANIFEST_SHA384=%s\n' "$manifest_sha384"
