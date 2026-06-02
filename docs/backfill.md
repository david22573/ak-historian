# AK-Historian Backfill Documentation

## Purpose

The purpose of the backfill is to populate Cloudflare R2 with reliable Binance USD-M futures 1m historical candle data for AK-Trader strategy replay/backtesting.

## Symbols

### Core Symbols (Tier 1)

These are the most liquid and useful symbols for scalp-bot research:

* BTCUSDT
* ETHUSDT
* SOLUSDT

### Expansion Symbols (Tier 2)

Secondary symbols for broader strategy validation:

* BNBUSDT
* XRPUSDT
* DOGEUSDT
* ADAUSDT
* LINKUSDT
* AVAXUSDT

## Backfill Ranges

For the first production backfill:

### Core Symbols
* **Monthly:** `2023-01` through `2026-04`
* **Daily (Gap):** `2026-05-01` through `2026-05-28`

### Expansion Symbols
* **Monthly:** `2024-01` through `2026-04`
* **Daily (Gap):** `2026-05-01` through `2026-05-28`

## How to Run

### Preflight Check

Always run preflight before a backfill to ensure environment and credentials are correct:

```bash
make preflight-backfill
```

### Core Backfill

Runs Tier 1 symbols (BTC, ETH, SOL):

```bash
make backfill-core
```

### Expansion Backfill

Runs Tier 2 symbols:

```bash
make backfill-expansion
```

### Full Backfill

Runs preflight, core, and expansion sequentially:

```bash
make backfill-all
```

## Configuration

### Concurrency

Default concurrency is `4`. You can override it via environment variable:

```bash
CONCURRENCY=6 make backfill-core
```

**Warning:** Do not set concurrency too high (e.g., > 10) to avoid Binance rate limits or local resource exhaustion.

## Safety and Reruns

* **Do not run multiple backfill scripts in parallel.** This can cause local disk congestion or rate limiting.
* **Reruns are safe.** The CLI skips objects that already exist in R2 with the correct size.
* **Failure handling:** If a script fails, it stops immediately. Logs are preserved for inspection.

## Logs and Proof

Logs are written to `runs/backfill/`.

After a successful run, capture proof of health:

```bash
make prove-backfill
```

Proof files are written to `runs/proof/`.
