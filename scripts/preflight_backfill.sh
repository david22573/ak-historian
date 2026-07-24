#!/usr/bin/env bash
set -euo pipefail

# Throttled execution environment for safe system resource usage
export GOMAXPROCS=${GOMAXPROCS:-2}
export GOGC=${GOGC:-50}

echo "== AK-Historian Backfill Preflight (Throttled Pace) =="

if [[ ! -f "go.mod" ]]; then
  echo "ERROR: run this from the ak-historian repo root"
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "ERROR: go is not installed or not in PATH"
  exit 1
fi

if ! command -v duckdb >/dev/null 2>&1; then
  echo "ERROR: duckdb is not installed or not in PATH"
  exit 1
fi

if [[ ! -f ".env" ]]; then
  echo "ERROR: .env file not found"
  exit 1
fi

set -a
source .env
set +a

required_vars=(
  R2_ACCOUNT_ID
  R2_ACCESS_KEY_ID
  R2_SECRET_ACCESS_KEY
  R2_BUCKET_NAME
)

for var in "${required_vars[@]}"; do
  if [[ -z "${!var:-}" ]]; then
    echo "ERROR: missing required env var: $var"
    exit 1
  fi
  echo "OK: $var is set"
done

echo
echo "== make fmt =="
make fmt

echo
echo "== make vet =="
make vet

echo
echo "== make test =="
make test

echo
echo "== go test -p 2 -race ./... =="
go test -p 2 -race ./...

echo
echo "== make build =="
make build

echo
echo "== dry-run smoke test =="
./bin/ak-historian fetch \
  --market futures-um \
  --symbols BTCUSDT \
  --interval 1m \
  --period monthly \
  --start 2024-01 \
  --end 2024-01 \
  --dry-run \
  --concurrency 1

echo
echo "Preflight passed."
