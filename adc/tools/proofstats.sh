#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROOFS_DIR="${REPO_ROOT}/engine/Proofs"
OUT_PATH="${1:-${REPO_ROOT}/docs/proofstats.md}"

cd "${REPO_ROOT}"

shopt -s nullglob
files=("${PROOFS_DIR}"/*.lean)
if [[ ${#files[@]} -eq 0 ]]; then
  echo "no proof files found under ${PROOFS_DIR}" >&2
  exit 1
fi

tmp_rows="$(mktemp)"
trap 'rm -f "${tmp_rows}" "${tmp_summary}" "${tmp_categories}" "${tmp_files}"' EXIT
tmp_summary="$(mktemp)"
tmp_categories="$(mktemp)"
tmp_files="$(mktemp)"

category_for() {
  local base="$1"
  case "$base" in
    Contempt.lean|Jury.lean|Party.lean)
      printf 'Domain'
      ;;
    StatusBasics.lean|OverrideBasics.lean|LimitBasics.lean|EffectiveLimitBasics.lean|TimeBasics.lean)
      printf 'Basics'
      ;;
    Validate*.lean)
      printf 'Validation'
      ;;
    *)
      printf 'Flow/Invariants'
      ;;
  esac
}

for path in "${files[@]}"; do
  base="$(basename "$path")"
  category="$(category_for "$base")"
  proofs="$(awk '/^[[:space:]]*(theorem|lemma)[[:space:]]+/{c++} END{print c+0}' "$path")"
  lines="$(wc -l < "$path" | tr -d ' ')"
  bytes="$(wc -c < "$path" | tr -d ' ')"
  printf '%s\t%s\t%s\t%s\t%s\n' "$category" "$base" "$proofs" "$lines" "$bytes" >> "$tmp_rows"
done

awk -F'\t' '
  { files += 1; proofs += $3; lines += $4; bytes += $5 }
  END {
    printf "Proof files\t%d\n", files
    printf "Total proofs\t%d\n", proofs
    printf "Total lines\t%d\n", lines
    printf "Total bytes\t%d\n", bytes
  }
' "$tmp_rows" > "$tmp_summary"

awk -F'\t' '
  {
    cat = $1
    files[cat] += 1
    proofs[cat] += $3
    lines[cat] += $4
    bytes[cat] += $5
  }
  END {
    for (cat in files) {
      printf "%s\t%d\t%d\t%d\t%d\n", cat, files[cat], proofs[cat], lines[cat], bytes[cat]
    }
  }
' "$tmp_rows" | sort -t$'\t' -k1,1 > "$tmp_categories"

sort -t$'\t' -k1,1 -k2,2 "$tmp_rows" > "$tmp_files"

{
  printf '# Proof Stats\n\n'
  printf 'Generated from `engine/Proofs/*.lean` using declarations matching `^(theorem|lemma)`.\n\n'

  printf '## Summary\n\n'
  printf '| Metric | Value |\n'
  printf '|---|---:|\n'
  while IFS=$'\t' read -r metric value; do
    printf '| %s | %s |\n' "$metric" "$value"
  done < "$tmp_summary"

  printf '\n## By Category\n\n'
  printf '| Category | Files | Proofs | Lines | Bytes |\n'
  printf '|---|---:|---:|---:|---:|\n'
  while IFS=$'\t' read -r category files proofs lines bytes; do
    printf '| %s | %s | %s | %s | %s |\n' "$category" "$files" "$proofs" "$lines" "$bytes"
  done < "$tmp_categories"

  printf '\n## By File\n\n'
  printf '| Category | File | Proofs | Lines | Bytes |\n'
  printf '|---|---|---:|---:|---:|\n'
  while IFS=$'\t' read -r category file proofs lines bytes; do
    printf '| %s | %s | %s | %s | %s |\n' "$category" "$file" "$proofs" "$lines" "$bytes"
  done < "$tmp_files"
} > "$OUT_PATH"

echo "wrote ${OUT_PATH}"
