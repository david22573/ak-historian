#!/usr/bin/env bash
set -euo pipefail

mkdir -p runs/backfill

STAMP="$(date +%Y-%m-%d_%H%M%S)"
LOG="runs/backfill/core_futures_1m_${STAMP}.log"

CONCURRENCY="${CONCURRENCY:-4}"

CORE_SYMBOLS="BTCUSDT,ETHUSDT,SOLUSDT"

MONTHLY_START="2023-01"
MONTHLY_END="2026-04"

DAILY_START="2026-05-01"
DAILY_END="2026-05-28"

echo "== AK-Historian Core Futures 1m Backfill ==" | tee "$LOG"
echo "Started: $(date -Is)" | tee -a "$LOG"
echo "Concurrency: $CONCURRENCY" | tee -a "$LOG"
echo "Core symbols: $CORE_SYMBOLS" | tee -a "$LOG"

run_step() {
  local name="$1"
  shift

  echo | tee -a "$LOG"
  echo "== $name ==" | tee -a "$LOG"
  echo "$*" | tee -a "$LOG"

  "$@" 2>&1 | tee -a "$LOG"
}

run_step "build binary" \
  make build

run_step "core monthly futures 1m backfill" \
  ./bin/ak-historian fetch \
    --market futures-um \
    --symbols "$CORE_SYMBOLS" \
    --interval 1m \
    --period monthly \
    --start "$MONTHLY_START" \
    --end "$MONTHLY_END" \
    --concurrency "$CONCURRENCY"

run_step "core daily futures 1m current-month gap" \
  ./bin/ak-historian fetch \
    --market futures-um \
    --symbols "$CORE_SYMBOLS" \
    --interval 1m \
    --period daily \
    --start "$DAILY_START" \
    --end "$DAILY_END" \
    --concurrency "$CONCURRENCY"

echo | tee -a "$LOG"
echo "Finished: $(date -Is)" | tee -a "$LOG"
echo "Log: $LOG" | tee -a "$LOG"
