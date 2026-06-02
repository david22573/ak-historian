#!/usr/bin/env bash
set -euo pipefail

mkdir -p runs/proof

STAMP="$(date +%Y-%m-%d_%H%M%S)"
OUT="runs/proof/backfill_proof_${STAMP}.txt"

{
  echo "# AK-Historian Backfill Proof"
  echo
  echo "Generated: $(date -Is)"
  echo
  echo "== git status =="
  git status --short || true

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
  echo "== dry-run core monthly =="
  ./bin/ak-historian fetch \
    --market futures-um \
    --symbols BTCUSDT,ETHUSDT,SOLUSDT \
    --interval 1m \
    --period monthly \
    --start 2023-01 \
    --end 2023-01 \
    --dry-run \
    --concurrency 1

  echo
  echo "== dry-run core daily =="
  ./bin/ak-historian fetch \
    --market futures-um \
    --symbols BTCUSDT,ETHUSDT,SOLUSDT \
    --interval 1m \
    --period daily \
    --start 2026-05-01 \
    --end 2026-05-01 \
    --dry-run \
    --concurrency 1

  echo
  echo "== recent backfill logs =="
  find runs/backfill -type f -name "*.log" -printf "%TY-%Tm-%Td %TH:%TM %p\n" 2>/dev/null | sort | tail -10 || true
} 2>&1 | tee "$OUT"

echo
echo "Proof written to: $OUT"
