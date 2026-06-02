# AK-Historian Backfill Automation Roadmap

## Objective

Automate the first serious historical data backfill for `ak-historian`.

The goal is to populate Cloudflare R2 with reliable Binance USD-M futures 1m historical candle data for AK-Trader strategy replay/backtesting.

The current `ak-historian` implementation already supports:

* Binance archive download
* checksum verification
* safe ZIP extraction
* DuckDB CSV-to-Parquet conversion
* Parquet validation
* R2 upload
* resumable object skipping
* dry-run mode
* bounded concurrency

Do not redesign the project. Build a safe automation layer around the existing CLI.

---

# Phase 0: Rules and Safety Constraints

## Hard Rules

1. Do not print secrets.
2. Do not modify `.env`.
3. Do not commit `.env`.
4. Do not run uncontrolled parallel shells.
5. Use the tool’s built-in `--concurrency` flag.
6. Default concurrency should be `4`.
7. Do not use `--force` unless explicitly requested.
8. Make the script rerunnable.
9. Every run must produce a timestamped log.
10. Every run must end with a concise summary.
11. If any command fails, stop the script and preserve logs.

## Backfill Strategy

Start with the most useful symbols for scalp-bot research:

```text
BTCUSDT
ETHUSDT
SOLUSDT
```

Then expand to secondary symbols:

```text
BNBUSDT
XRPUSDT
DOGEUSDT
ADAUSDT
LINKUSDT
AVAXUSDT
```

Use Binance USD-M futures:

```text
--market futures-um
--interval 1m
```

For the current first production backfill:

```text
Monthly:
  2023-01 through 2026-04

Daily recent gap:
  2026-05-01 through 2026-05-28
```

Reason:

* Monthly files should cover full historical months.
* May 2026 is not a complete monthly archive yet.
* Daily files fill the current-month gap.

---

# Phase 1: Preflight Checks

Create:

```text
scripts/preflight_backfill.sh
```

The script must check:

1. Running from repo root.
2. `go` exists.
3. `duckdb` exists.
4. `.env` exists.
5. Required env keys are present without printing values:

   * `R2_ACCOUNT_ID`
   * `R2_ACCESS_KEY_ID`
   * `R2_SECRET_ACCESS_KEY`
   * `R2_BUCKET_NAME`
6. Binary builds successfully.
7. Tests pass.
8. Race tests pass.
9. Vet passes.
10. Dry-run works.

## Required Script

```bash
#!/usr/bin/env bash
set -euo pipefail

echo "== AK-Historian Backfill Preflight =="

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
echo "== go test -race ./... =="
go test -race ./...

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
```

Make it executable:

```bash
chmod +x scripts/preflight_backfill.sh
```

## Acceptance Criteria

* Script fails fast if dependencies are missing.
* Script does not print secret values.
* Script confirms dry-run works.
* Script exits nonzero on any failure.

---

# Phase 2: Core Backfill Script

Create:

```text
scripts/backfill_core_futures_1m.sh
```

This script should backfill the core symbols:

```text
BTCUSDT,ETHUSDT,SOLUSDT
```

It must run:

1. Core monthly backfill.
2. Core daily current-month gap.
3. Rerun dry/proof check is optional but useful.
4. Save all logs to `runs/backfill/`.

## Required Script

```bash
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
```

Make it executable:

```bash
chmod +x scripts/backfill_core_futures_1m.sh
```

## Acceptance Criteria

* Script builds binary first.
* Script backfills BTCUSDT, ETHUSDT, SOLUSDT.
* Script uses concurrency from `CONCURRENCY`, defaulting to `4`.
* Script logs all output.
* Script stops on failure.
* Script is rerunnable and should skip existing R2 objects.

---

# Phase 3: Expansion Backfill Script

Create:

```text
scripts/backfill_expansion_futures_1m.sh
```

This script should backfill secondary symbols only after the core backfill succeeds.

Expansion symbols:

```text
BNBUSDT,XRPUSDT,DOGEUSDT,ADAUSDT,LINKUSDT,AVAXUSDT
```

Use a smaller historical range first:

```text
Monthly:
  2024-01 through 2026-04

Daily:
  2026-05-01 through 2026-05-28
```

## Required Script

```bash
#!/usr/bin/env bash
set -euo pipefail

mkdir -p runs/backfill

STAMP="$(date +%Y-%m-%d_%H%M%S)"
LOG="runs/backfill/expansion_futures_1m_${STAMP}.log"

CONCURRENCY="${CONCURRENCY:-4}"

EXPANSION_SYMBOLS="BNBUSDT,XRPUSDT,DOGEUSDT,ADAUSDT,LINKUSDT,AVAXUSDT"

MONTHLY_START="2024-01"
MONTHLY_END="2026-04"

DAILY_START="2026-05-01"
DAILY_END="2026-05-28"

echo "== AK-Historian Expansion Futures 1m Backfill ==" | tee "$LOG"
echo "Started: $(date -Is)" | tee -a "$LOG"
echo "Concurrency: $CONCURRENCY" | tee -a "$LOG"
echo "Expansion symbols: $EXPANSION_SYMBOLS" | tee -a "$LOG"

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

run_step "expansion monthly futures 1m backfill" \
  ./bin/ak-historian fetch \
    --market futures-um \
    --symbols "$EXPANSION_SYMBOLS" \
    --interval 1m \
    --period monthly \
    --start "$MONTHLY_START" \
    --end "$MONTHLY_END" \
    --concurrency "$CONCURRENCY"

run_step "expansion daily futures 1m current-month gap" \
  ./bin/ak-historian fetch \
    --market futures-um \
    --symbols "$EXPANSION_SYMBOLS" \
    --interval 1m \
    --period daily \
    --start "$DAILY_START" \
    --end "$DAILY_END" \
    --concurrency "$CONCURRENCY"

echo | tee -a "$LOG"
echo "Finished: $(date -Is)" | tee -a "$LOG"
echo "Log: $LOG" | tee -a "$LOG"
```

Make it executable:

```bash
chmod +x scripts/backfill_expansion_futures_1m.sh
```

## Acceptance Criteria

* Expansion backfill is separate from core backfill.
* Script is rerunnable.
* Script uses controlled concurrency.
* Script produces timestamped logs.
* Script stops on failure.

---

# Phase 4: Optional Full Backfill Orchestrator

Create:

```text
scripts/backfill_all_futures_1m.sh
```

This script should run:

1. Preflight.
2. Core backfill.
3. Expansion backfill.

## Required Script

```bash
#!/usr/bin/env bash
set -euo pipefail

echo "== Full AK-Historian Futures 1m Backfill =="

./scripts/preflight_backfill.sh
./scripts/backfill_core_futures_1m.sh
./scripts/backfill_expansion_futures_1m.sh

echo
echo "Full backfill completed."
```

Make it executable:

```bash
chmod +x scripts/backfill_all_futures_1m.sh
```

## Acceptance Criteria

* One command can run the entire planned backfill.
* If preflight fails, no backfill starts.
* If core fails, expansion does not start.
* Logs are preserved by each child script.

---

# Phase 5: Add Makefile Targets

Update `Makefile` with:

```makefile
.PHONY: preflight-backfill backfill-core backfill-expansion backfill-all

preflight-backfill:
	./scripts/preflight_backfill.sh

backfill-core:
	./scripts/backfill_core_futures_1m.sh

backfill-expansion:
	./scripts/backfill_expansion_futures_1m.sh

backfill-all:
	./scripts/backfill_all_futures_1m.sh
```

## Acceptance Criteria

These commands work:

```bash
make preflight-backfill
make backfill-core
make backfill-expansion
make backfill-all
```

---

# Phase 6: Add Backfill Documentation

Create:

```text
docs/backfill.md
```

Document:

1. Purpose of the backfill.
2. Why core symbols are first:

   * BTCUSDT
   * ETHUSDT
   * SOLUSDT
3. Why expansion symbols are second:

   * BNBUSDT
   * XRPUSDT
   * DOGEUSDT
   * ADAUSDT
   * LINKUSDT
   * AVAXUSDT
4. Backfill ranges:

   * Monthly `2023-01..2026-04` for core
   * Daily `2026-05-01..2026-05-28` for core
   * Monthly `2024-01..2026-04` for expansion
   * Daily `2026-05-01..2026-05-28` for expansion
5. How to run preflight.
6. How to run core only.
7. How to run expansion only.
8. How to run all.
9. How to change concurrency.
10. Why not to run multiple backfill scripts at once.
11. How reruns work.
12. How to inspect logs.

Include examples:

```bash
make preflight-backfill
```

```bash
make backfill-core
```

```bash
CONCURRENCY=6 make backfill-core
```

```bash
make backfill-expansion
```

```bash
make backfill-all
```

## Acceptance Criteria

* Docs explain safe operation.
* Docs warn not to run multiple scripts in parallel.
* Docs explain `CONCURRENCY`.
* Docs explain rerun/skip behavior.

---

# Phase 7: Create Backfill Proof Capture Script

Create:

```text
scripts/prove_backfill.sh
```

Purpose:

After a backfill run, capture proof that the repo and CLI are healthy.

The script should run:

1. `make test`
2. `go test -race ./...`
3. `make vet`
4. Dry-run core monthly
5. Dry-run core daily
6. Show recent backfill logs

## Required Script

```bash
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
```

Make it executable:

```bash
chmod +x scripts/prove_backfill.sh
```

Add Makefile target:

```makefile
.PHONY: prove-backfill

prove-backfill:
	./scripts/prove_backfill.sh
```

## Acceptance Criteria

* Proof script writes a timestamped proof file.
* Proof script captures tests, race tests, vet, and dry-run.
* Proof script lists recent backfill logs.
* Proof script does not expose secrets.

---

# Phase 8: Improve Date Handling Later

For this first version, fixed dates are acceptable:

```text
monthly end: 2026-04
daily end: 2026-05-28
```

After the first successful backfill, optionally create a dynamic script that computes:

* latest complete month
* yesterday’s date
* current month daily start

Do not do this until the fixed-date version works.

Future script name:

```text
scripts/backfill_recent_gap.sh
```

Future behavior:

1. Determine current date.
2. Determine yesterday.
3. Determine first day of current month.
4. Run daily gap backfill for core and expansion symbols.
5. Use concurrency 4.
6. Be safe to run daily.

---

# Phase 9: Execution Plan

Run in this order:

```bash
make preflight-backfill
```

Then:

```bash
make backfill-core
```

Only after core succeeds:

```bash
make backfill-expansion
```

Then capture proof:

```bash
make prove-backfill
```

Do not run `make backfill-all` until individual scripts have been proven once.

---

# Phase 10: Success Criteria

The backfill automation is successful when:

1. `make preflight-backfill` passes.
2. `make backfill-core` completes.
3. `make backfill-expansion` completes.
4. Logs are written under `runs/backfill/`.
5. No secrets are printed.
6. Rerunning core backfill skips existing R2 objects.
7. Rerunning expansion backfill skips existing R2 objects.
8. `make prove-backfill` passes.
9. Proof file is written under `runs/proof/`.
10. There is enough R2 data to begin building AK-Trader historical replay/backtesting.

---

# Phase 11: Stop Conditions

Stop immediately if any of these occur:

1. Repeated checksum mismatches.
2. Repeated HTTP 429s even at concurrency 4.
3. R2 upload failures.
4. DuckDB conversion failures.
5. Parquet validation failures.
6. Local disk fills up.
7. Logs show many `failed:` entries.
8. `go test -race ./...` fails.
9. Secrets appear in logs.

If stop condition occurs:

1. Do not use `--force`.
2. Preserve logs.
3. Run `make prove-backfill`.
4. Report the failing log section.
5. Wait for operator review.

---

# Phase 12: After Backfill

Once the data is backfilled, do not keep adding random symbols yet.

The next project should be:

```text
AK historical replay/backtester
```

Required capabilities:

1. Read Parquet candle data from R2 or local synced copy.
2. Stream candles in chronological order.
3. Simulate strategy signals.
4. Simulate entries and exits.
5. Include fees.
6. Include slippage.
7. Include stop-loss.
8. Include take-profit.
9. Include timeout exits.
10. Report:

    * win rate
    * average win
    * average loss
    * expectancy
    * profit factor
    * max drawdown
    * trade count
    * losing streak
    * per-symbol performance
    * per-month performance

Backfill is only the data foundation. The next milestone is proving whether a scalp strategy has positive expectancy after fees and slippage.

---

# Final Definition of Done

This roadmap is complete when the repo contains:

```text
scripts/preflight_backfill.sh
scripts/backfill_core_futures_1m.sh
scripts/backfill_expansion_futures_1m.sh
scripts/backfill_all_futures_1m.sh
scripts/prove_backfill.sh
docs/backfill.md
```

And the following commands pass:

```bash
make preflight-backfill
make backfill-core
make backfill-expansion
make prove-backfill
```

Final operator-facing output should include:

```text
Core backfill completed.
Expansion backfill completed.
Proof written to: runs/proof/backfill_proof_<timestamp>.txt
Ready to begin AK historical replay/backtesting.
```
