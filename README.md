# ak-historian

Bulk backfill tool for Binance historical data, coded in Go.

## Overview

`ak-historian` is a CLI tool designed to download historical kline data from Binance Vision, convert the CSV archives into typed Parquet files using DuckDB, and upload the resulting files to Cloudflare R2.

## Features

- Downloads monthly and daily historical kline archives.
- Verifies SHA256 checksums of downloaded archives.
- Safely extracts CSV files from ZIP archives with path traversal protection.
- Converts CSV to Parquet with explicit schema using DuckDB.
- Validates Parquet files (row count and time ranges).
- Uploads Parquet files to Cloudflare R2 (S3-compatible).
- Supports bounded concurrency using a worker pool.
- Resumable: skips items already present in R2 (use `--force` to override).
- Dry-run mode to plan work without execution.

## Prerequisites

- [Go](https://golang.org/doc/install) 1.22+
- [DuckDB CLI](https://duckdb.org/docs/installation/) (must be in PATH)

## Installation

```bash
git clone https://github.com/davidmiguel22573/ak-historian
cd ak-historian
make build
```

The binary will be available at `./bin/ak-historian`.

## Configuration

Create a `.env` file in the project root:

```env
R2_ACCOUNT_ID=your-account-id
R2_ACCESS_KEY_ID=your-access-key
R2_SECRET_ACCESS_KEY=your-secret-key
R2_BUCKET_NAME=your-bucket-name
```

## Usage

### Fetch Monthly Data

```bash
./bin/ak-historian fetch \
  --market futures-um \
  --symbols BTCUSDT,ETHUSDT \
  --interval 1m \
  --period monthly \
  --start 2024-01 \
  --end 2024-03
```

### Fetch Daily Data

```bash
./bin/ak-historian fetch \
  --market futures-um \
  --symbols BTCUSDT \
  --interval 1m \
  --period daily \
  --start 2024-01-01 \
  --end 2024-01-07
```

### Dry Run

```bash
./bin/ak-historian fetch \
  --market futures-um \
  --symbols BTCUSDT \
  --interval 1m \
  --period monthly \
  --start 2024-01 \
  --end 2024-01 \
  --dry-run
```

## Options

```text
Flags:
      --concurrency int   number of concurrent downloads (default 2)
      --dry-run           dry run (only show what would be done)
      --end string        end date (YYYY-MM or YYYY-MM-DD)
      --force             force re-download and re-process
      --interval string   kline interval (default "1m")
      --keep              keep temporary files after processing
      --market string     market type (futures-um, futures-cm, spot) (default "futures-um")
      --period string     data period (monthly, daily) (default "monthly")
      --start string      start date (YYYY-MM or YYYY-MM-DD)
      --symbols string    comma-separated list of symbols (default "BTCUSDT")
      --verify            verify checksums (default true)
      --workdir string    working directory (default ".ak-historian/work")
```

## Technical Details

### Checksum Verification
By default (`--verify=true`), the tool downloads the `.CHECKSUM` file for each archive (Binance standard). 
- If the checksum file is missing (404), a warning is logged and processing continues.
- Any other download error or a SHA256 mismatch results in a failure for that item, and the corrupted ZIP is deleted.

### Retry Logic
The downloader automatically retries up to 3 times for transient errors:
- HTTP 429 (Too Many Requests)
- HTTP 5xx (Server Errors)
- Network-level timeouts or connection resets

It will **not** retry permanent failures like 403 Forbidden or 404 Not Found.

### Parquet Schema
The output Parquet files (ZSTD compressed) use the following schema:

| Column | Type | Description |
|--------|------|-------------|
| market | VARCHAR | e.g., spot, futures-um |
| symbol | VARCHAR | e.g., BTCUSDT |
| interval | VARCHAR | e.g., 1m |
| period | VARCHAR | monthly or daily |
| source_date | VARCHAR | e.g., 2024-01 or 2024-01-01 |
| open_time_ms | BIGINT | Milliseconds since epoch |
| open | DOUBLE | |
| high | DOUBLE | |
| low | DOUBLE | |
| close | DOUBLE | |
| volume | DOUBLE | |
| close_time_ms | BIGINT | |
| quote_asset_volume | DOUBLE | |
| number_of_trades | BIGINT | |
| taker_buy_base_volume | DOUBLE | |
| taker_buy_quote_volume | DOUBLE | |

### Local Workdir Layout
Temporary files are stored in `workdir/`:
`{workdir}/{market}/{interval}/{symbol}/{period}/{date}/`

## Development

### Running Tests
```bash
make test
```

### Race Detector
```bash
go test -race ./...
```

## Object Key Layout in R2

`candles/{market}/{interval}/symbol={SYMBOL}/year={YYYY}/month={MM}/{SYMBOL}-{INTERVAL}-{DATE}.parquet`

Example:
`candles/futures-um/1m/symbol=BTCUSDT/year=2024/month=01/BTCUSDT-1m-2024-01.parquet`

## License

MIT
