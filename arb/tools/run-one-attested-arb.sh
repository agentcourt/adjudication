#!/usr/bin/env bash
set -euo pipefail

usage() {
  printf 'Usage: %s examples/NAME\n' "${0##*/}" >&2
}

if [ "$#" -ne 1 ]; then
  usage
  exit 2
fi

script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
arb_dir="$(CDPATH= cd -- "$script_dir/.." && pwd)"
workspace="$(CDPATH= cd -- "$arb_dir/../.." && pwd)"

example_path="${1%/}"
case "$example_path" in
  examples/*)
    example="${example_path#examples/}"
    ;;
  "$arb_dir"/examples/*)
    example="${example_path#"$arb_dir"/examples/}"
    ;;
  "$workspace"/adjudication/arb/examples/*)
    example="${example_path#"$workspace"/adjudication/arb/examples/}"
    ;;
  *)
    printf 'error: expected an example directory such as examples/ex03\n' >&2
    exit 2
    ;;
esac

case "$example" in
  ""|.*|*/*|*..*)
    printf 'error: invalid example name: %s\n' "$example" >&2
    exit 2
    ;;
esac

example_dir="$arb_dir/examples/$example"
if [ ! -d "$example_dir" ]; then
  printf 'error: example directory does not exist: %s\n' "$example_dir" >&2
  exit 2
fi
if [ ! -f "$example_dir/complaint.md" ]; then
  printf 'error: example has no complaint.md: %s\n' "$example_dir" >&2
  exit 2
fi

dev_host="${DEV_HOST:-dev}"
aws_region="${AWS_REGION:-us-east-2}"
exec_ami="${EXEC_AMI:-ami-011f957fe91cf7b81}"
input_root="${INPUT_ROOT:-s3://agentcourt-data/arbattest/aar-inputs}"
output_root="${OUTPUT_ROOT:-s3://agentcourt-data/arbattest/aar-runs}"
out_root="${OUT_ROOT:-$workspace/aar-attested}"
expected_pcr4="${EXPECTED_PCR4:-83AC49DFAA5D76939970E1568472FF463FBE90C4038D000D31F6C0520F583D1DD51CE0C103CEB26E4B773AAD99A4B3B4}"
expected_pcr7="${EXPECTED_PCR7:-98441C7F7625D10058C47683AEC486CE311C633235EB555593A7EE791121E3578AE72D04ECEF661F272D59058B77AF35}"

stamp="$(date -u +%Y%m%dT%H%M%SZ)"
run_id="${RUN_ID:-aar-$example-$stamp}"
input_prefix="${INPUT_PREFIX:-$input_root/$example-$stamp}"
output_prefix="${OUTPUT_PREFIX:-$output_root/$run_id}"
out_dir="${OUT_DIR:-$out_root/$run_id}"

mkdir -p "$out_root"

printf 'EXAMPLE=%s\n' "$example"
printf 'INPUT_PREFIX=%s\n' "$input_prefix"
printf 'RUN_ID=%s\n' "$run_id"
printf 'OUTPUT_PREFIX=%s\n' "$output_prefix"
printf 'OUT_DIR=%s\n' "$out_dir"

ssh "$dev_host" "set -eu
AWS_DEFAULT_REGION='$aws_region' aws s3 cp /home/ec2-user/arbattest-secrets/auth.json '$input_prefix/auth.json' --no-progress
AWS_DEFAULT_REGION='$aws_region' aws s3 cp /home/ec2-user/arbattest-secrets/keys.sh '$input_prefix/keys.sh' --no-progress
AWS_DEFAULT_REGION='$aws_region' aws s3 ls '$input_prefix/'"

uv run "$arb_dir/tools/run-arb-attested.py" \
  --example "$example" \
  --input-prefix "$input_prefix" \
  --exec-ami "$exec_ami" \
  --run-id "$run_id" \
  --output-prefix "$output_prefix" \
  --out-dir "$out_dir" \
  --verify \
  --expected-pcr4 "$expected_pcr4" \
  --expected-pcr7 "$expected_pcr7"
