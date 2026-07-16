#!/usr/bin/env bash
set -e

: "${OPENROUTER_API_KEY:?OPENROUTER_API_KEY is required}"

OPENCLAW_CODEX_AUTH="${OPENCLAW_CODEX_AUTH:-$HOME/.codex/auth.json}"

cd "$(dirname "$0")/arb"
make build

mkdir -p work/example-arbitration
cat > work/example-arbitration/complaint.md <<'EOF'
# Proposition

During May 2026 (ET), Iran initiated a major non-weather closure of its airspace.
EOF

.bin/aar run \
  --complaint work/example-arbitration/complaint.md \
  --openclaw-auth codex \
  --openclaw-codex-auth "$OPENCLAW_CODEX_AUTH" \
  --council-pool "$(pwd)/pool.jsonl"
