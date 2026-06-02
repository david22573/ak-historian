#!/usr/bin/env bash
set -euo pipefail

echo "== Full AK-Historian Futures 1m Backfill =="

./scripts/preflight_backfill.sh
./scripts/backfill_core_futures_1m.sh
./scripts/backfill_expansion_futures_1m.sh

echo
echo "Full backfill completed."
