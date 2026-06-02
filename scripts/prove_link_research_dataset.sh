#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export GOCACHE="${GOCACHE:-$ROOT/.cache/go-build}"
export GOMODCACHE="${GOMODCACHE:-$ROOT/.cache/go-mod}"

mkdir -p runs/proof

STAMP="$(date +%Y-%m-%d_%H%M%S)"
OUT="runs/proof/link_research_dataset_${STAMP}.txt"
LATEST_TXT="runs/proof/link_dataset_latest.txt"
LATEST_LINK="runs/proof/latest"
LATEST_PATH_TXT="runs/proof/latest_path.txt"
STATUS="PASS"

write_latest() {
  cp "$OUT" "$LATEST_TXT"
  rm -f "$LATEST_LINK"
  ln -s "$(basename "$OUT")" "$LATEST_LINK" 2>/dev/null || true
  printf '%s\n' "$OUT" >"$LATEST_PATH_TXT"
}

trap 'STATUS="FAIL"; write_latest' ERR

{
  echo "# AK-Historian LINK Research Dataset Proof"
  echo
  echo "Generated: $(date -Is)"
  echo "Git Commit: $(git rev-parse HEAD 2>/dev/null || true)"
  echo
  echo "== git status =="
  git status --short || true

  echo
  echo "== make build =="
  make build

  echo
  echo "== make test =="
  make test

  echo
  echo "== go test -race ./... =="
  go test -race ./...

  echo
  echo "== make vet =="
  make vet

  echo
  echo "== dry-run LINK monthly =="
  ./bin/ak-historian fetch \
    --market futures-um \
    --symbols LINKUSDT \
    --interval 1m \
    --period monthly \
    --start 2023-01 \
    --end 2026-04 \
    --dry-run \
    --concurrency 1

  echo
  echo "== dry-run LINK daily =="
  ./bin/ak-historian fetch \
    --market futures-um \
    --symbols LINKUSDT \
    --interval 1m \
    --period daily \
    --start 2026-05-01 \
    --end 2026-05-29 \
    --dry-run \
    --concurrency 1

  echo
  echo "== verify LINK coverage =="
  ./bin/ak-historian verify-coverage \
    --market futures-um \
    --symbol LINKUSDT \
    --interval 1m \
    --from 2023-01-01 \
    --to 2026-05-29 \
    --source r2

  echo
  echo "== write LINK manifest =="
  ./bin/ak-historian write-manifest \
    --market futures-um \
    --symbol LINKUSDT \
    --interval 1m \
    --from 2023-01-01 \
    --to 2026-05-29 \
    --source r2

  echo
  echo "== final status =="
  echo "$STATUS"
} 2>&1 | tee "$OUT"

write_latest

echo
echo "Proof written to: $OUT"
echo "Latest proof copied to: $LATEST_TXT"
