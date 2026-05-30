## Biggest issues to fix

### 1. `--dry-run` incorrectly increments `Uploaded`

In `processItem`, dry-run logs the plan but then increments `summary.Uploaded`. That makes the final summary lie:

```go
if opts.DryRun {
    log.Printf("Plan: %s %s -> %s", symbol, date, objectKey)
    summary.mu.Lock()
    summary.Uploaded++ // count as planned upload in dry run
    summary.mu.Unlock()
    return nil
}
```

This should not count as uploaded. Add a separate `DryRunPlanned` counter or just leave `Uploaded` untouched. 

### 2. `SourcePeriod` is wrong in Parquet

You pass:

```go
SourcePeriod: opts.Period,
```

So Parquet stores `monthly` or `daily`, not the actual source date like `2024-01` or `2024-01-05`. That field should probably be renamed or corrected.

Better:

```go
SourceDate: date
Period: opts.Period
```

Or keep `source_period` but set it to `date`. Right now the output metadata is less useful for partition verification. 

### 3. Checksum failure is downgraded to a warning

The roadmap said: if checksum is unavailable with 404, warn and continue; any other checksum request failure should be an error. Current code logs a warning for **all** checksum download errors:

```go
expectedChecksum, err := binance.DownloadChecksum(...)
if err != nil {
    log.Printf("Warning: checksum failed...")
}
```

That means network errors, bad status codes, or malformed checksum responses can be ignored. For historical trading data, that’s too loose. Only 404 should be allowed as a warning. Everything else should fail the item. 

### 4. Downloader retries the wrong things

The retry loop retries any non-200 status except 404. That means it retries 400/403 too, which are usually permanent. The roadmap wanted retry for `429` and `5xx` only.

Current logic:

```go
if resp.StatusCode != http.StatusOK {
    return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
}
```

Then the generic retry wrapper retries it. Add typed retryable errors or inspect status codes directly. 

### 5. Context cancellation is not respected before starting goroutines

`RunFetch` launches a goroutine for every symbol/date immediately, then limits actual execution with a semaphore. For big backfills, that can create thousands of goroutines waiting on the semaphore.

Current pattern:

```go
for _, symbol := range opts.Symbols {
    for _, date := range dates {
        wg.Add(1)
        go func(...) {
            semaphore <- struct{}{}
```

This is bounded execution, but not bounded goroutine creation. Use a real worker pool/job channel instead. 

### 6. Summary counters are partially unsafe

`summary.Planned++` happens without locking, while other counters use a mutex. Because planned increments happen before goroutines start doing summary mutations, it may be okay in this exact flow, but the struct is already concurrent. Better to centralize all summary mutation through methods:

```go
summary.IncPlanned()
summary.IncUploaded()
summary.IncFailed()
```

This avoids future race bugs. 

### 7. `ObjectExists` error handling is brittle

The R2 `ObjectExists` method string-matches errors:

```go
if fmt.Errorf("%w", err).Error() == "api error NotFound: Not Found"
```

and then uses type-name/string contains fallback. That is fragile. Use AWS SDK v2 error handling with `smithy.APIError` and check error code/status. 

### 8. ZIP path check is almost good, but the prefix check is weak

This check:

```go
if !strings.HasPrefix(destPath, filepath.Clean(destDir)) {
```

can be tricked in some path-prefix cases. Use `filepath.Rel(destDir, destPath)` and reject paths beginning with `..`.

Also, `strings.Contains(targetFile.Name, "..")` is too broad and too narrow at the same time. Use normalized path component validation. 

### 9. `go.mod` says Go 1.24.4, not Go 1.21+

The roadmap said Go 1.21+. The generated module is:

```go
go 1.24.4
```

That may be fine on your machine, but it raises the floor unnecessarily. If you want this portable, set it to a stable available target like `go 1.22` or whatever you actually intend to require. 

## Agent fix roadmap

Give this to the agent next:

````md
# AK-Historian Hardening Roadmap

## Objective

Harden the current implementation so it is safe for real historical ingestion into R2 and reliable enough for AK-Trader data backfills.

Current tests pass, but there are correctness, retry, summary, checksum, and concurrency issues that must be fixed before production use.

---

## Phase 1: Fix Dry-Run Summary Semantics

### Problem

`--dry-run` currently increments `summary.Uploaded`, which makes the final summary inaccurate.

### Tasks

1. Add a `DryRunPlanned int` field to `Summary`.
2. In dry-run mode, increment `DryRunPlanned`, not `Uploaded`.
3. Update summary output to include:

```text
dry_run_planned: N
````

4. Add an orchestration test proving dry-run does not increment uploaded.

### Acceptance Criteria

* Dry-run never reports uploaded files.
* Dry-run still reports planned work clearly.
* Tests cover this behavior.

---

## Phase 2: Fix Parquet Metadata Fields

### Problem

`SourcePeriod` is currently set to `opts.Period`, producing values like `monthly` or `daily` instead of the actual source date.

### Tasks

1. Replace `SourcePeriod` with two explicit fields:

```go
Period     string
SourceDate string
```

2. Update Parquet schema to include:

```text
period       VARCHAR
source_date  VARCHAR
```

3. Set:

```go
Period: opts.Period
SourceDate: date
```

4. Update converter tests to verify both fields exist.

### Acceptance Criteria

* Parquet contains `period = monthly` or `daily`.
* Parquet contains `source_date = 2024-01` or `2024-01-05`.
* Tests verify schema and values.

---

## Phase 3: Make Checksum Handling Strict

### Problem

All checksum download errors are currently warnings. Only checksum 404 should be a warning.

### Tasks

1. Create a sentinel error:

```go
var ErrChecksumNotFound = errors.New("checksum not found")
```

2. Make `DownloadChecksum` return `ErrChecksumNotFound` on 404.
3. In `processItem`:

   * If `ErrChecksumNotFound`, warn and continue.
   * Any other checksum error fails the item.
   * SHA mismatch fails the item and deletes the zip.
4. Add tests for:

   * checksum 404 warning path
   * checksum 500 failure path
   * checksum mismatch failure path

### Acceptance Criteria

* Missing checksum is allowed.
* Broken checksum request fails.
* Mismatched checksum fails.
* Corrupt zip is deleted.

---

## Phase 4: Fix Retry Behavior

### Problem

Downloader retries all non-200 statuses except 404. It should only retry 429 and 5xx.

### Tasks

1. Create a typed error for HTTP status:

```go
type HTTPStatusError struct {
    StatusCode int
}
```

2. Retry only when:

   * status is 429
   * status is 500-599
   * network error is temporary/context-appropriate
3. Do not retry:

   * 400
   * 401
   * 403
   * 404
4. Add downloader tests for:

   * 429 retries then succeeds
   * 500 retries then succeeds
   * 403 does not retry
   * 404 returns `NotFound`
   * context cancellation stops retry sleep

### Acceptance Criteria

* Retry behavior matches status class.
* Permanent failures fail fast.
* Context cancellation interrupts retry delay.

---

## Phase 5: Replace Semaphore Goroutines with Worker Pool

### Problem

The current implementation launches one goroutine per item and only bounds execution with a semaphore. Large backfills can create too many waiting goroutines.

### Tasks

1. Build a list/channel of jobs:

```go
type FetchJob struct {
    Symbol string
    Date   string
}
```

2. Start exactly `opts.Concurrency` workers.
3. Workers read jobs from a channel.
4. Stop queueing new jobs if `ctx.Err() != nil`.
5. Ensure `--concurrency <= 0` returns a validation error.
6. Add tests for concurrency 1 and invalid concurrency.

### Acceptance Criteria

* No unbounded goroutine creation.
* `--concurrency 1` is sequential.
* `--concurrency 4` works.
* Context cancellation stops new work.

---

## Phase 6: Add Summary Methods and Race Safety

### Problem

Summary mutation is partially locked and partially unlocked.

### Tasks

1. Add methods:

```go
func (s *Summary) IncPlanned()
func (s *Summary) IncUploaded()
func (s *Summary) IncSkippedExisting()
func (s *Summary) IncSkippedMissing()
func (s *Summary) IncFailed()
func (s *Summary) Snapshot() SummarySnapshot
```

2. Use methods everywhere.
3. Print from snapshot after workers finish.
4. Run:

```bash
go test -race ./...
```

### Acceptance Criteria

* No direct counter mutation outside methods.
* Race detector passes.

---

## Phase 7: Harden R2 ObjectExists

### Problem

`ObjectExists` string-matches AWS/R2 errors.

### Tasks

1. Use `github.com/aws/smithy-go`.
2. Detect 404 through `smithy.APIError`.
3. Accept error codes:

   * `NotFound`
   * `NoSuchKey`
   * `404`
4. Return `false, nil` only for real not-found responses.
5. Add unit tests around the helper that classifies not-found errors.

### Acceptance Criteria

* No brittle full error string comparisons.
* Real not-found returns false.
* Other errors are returned.

---

## Phase 8: Harden ZIP Extraction Path Checks

### Problem

ZIP extraction uses fragile string prefix checks.

### Tasks

1. Replace prefix check with `filepath.Rel`.
2. Reject if relative path starts with `..`.
3. Reject absolute target paths.
4. Normalize zip entry path.
5. Only extract exact expected CSV filename.
6. Add tests for:

   * `../evil.csv`
   * `/tmp/evil.csv`
   * `nested/../../evil.csv`
   * expected CSV success

### Acceptance Criteria

* Path traversal is robustly blocked.
* Only expected CSV is extracted.
* Tests cover malicious zip names.

---

## Phase 9: Validate Inputs Earlier

### Tasks

Add validation before starting work:

1. Validate market:

   * `futures-um`
   * `futures-cm`
   * `spot`

2. Validate period:

   * `monthly`
   * `daily`

3. Validate interval is non-empty and contains no path separator.

4. Validate symbols:

   * trim spaces
   * uppercase
   * reject empty symbols
   * reject path separators
   * reject symbols with whitespace

5. Validate concurrency:

   * must be >= 1

6. Validate workdir:

   * non-empty

### Acceptance Criteria

* Bad input fails before R2 client creation.
* Tests cover invalid symbols, invalid interval, invalid concurrency.

---

## Phase 10: Improve Converter Tests

### Tasks

1. Add test that reads generated Parquet schema using DuckDB.

2. Verify expected columns exist:

   * market
   * symbol
   * interval
   * period
   * source_date
   * open_time_ms
   * open
   * high
   * low
   * close
   * volume
   * close_time_ms
   * quote_asset_volume
   * number_of_trades
   * taker_buy_base_volume
   * taker_buy_quote_volume

3. Add bad CSV test.

4. Skip DuckDB tests if `duckdb` binary is not available.

### Acceptance Criteria

* Converter tests prove schema correctness.
* Tests do not fail on systems missing DuckDB; they skip clearly.

---

## Phase 11: Add App-Level Tests with Interfaces

### Problem

`RunFetch` directly depends on concrete R2/downloader/converter behavior, making orchestration hard to test.

### Tasks

1. Introduce small interfaces:

```go
type ObjectStore interface {
    ObjectExists(ctx context.Context, key string) (bool, error)
    UploadFile(ctx context.Context, localPath string, objectKey string) error
}

type ArchiveDownloader interface {
    DownloadArchive(ctx context.Context, url string, destPath string, force bool) (binance.DownloadStatus, error)
}
```

2. Refactor orchestration to accept dependencies internally.
3. Keep CLI path simple.
4. Add fake implementations for tests.

Test cases:

* dry-run plans only
* existing object skips
* 404 archive skips
* one failed item does not stop other item
* failed item causes final error

### Acceptance Criteria

* Orchestration has meaningful tests.
* No real network/R2/DuckDB needed for app tests.

---

## Phase 12: README Accuracy Pass

### Tasks

Update README to document:

1. Dry-run summary behavior.
2. `--force` behavior.
3. Checksum behavior:

   * checksum 404 warning
   * checksum mismatch failure
4. Object key layout.
5. Parquet schema.
6. Local workdir layout.
7. Example `go test -race ./...`.
8. Note that DuckDB must be in PATH for conversion.

### Acceptance Criteria

* README matches actual CLI behavior.
* README includes Parquet schema.
* README explains failure behavior.

---

## Final Verification

Run:

```bash
make fmt
make vet
make test
go test -race ./...
make build
```

Then run:

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

Then run one real ingestion:

```bash
./bin/ak-historian fetch \
  --market futures-um \
  --symbols BTCUSDT \
  --interval 1m \
  --period monthly \
  --start 2024-01 \
  --end 2024-01 \
  --concurrency 1
```

Then rerun the same command and confirm it skips existing R2 object unless `--force` is set.

## Definition of Done

* `make fmt` passes.
* `make vet` passes.
* `make test` passes.
* `go test -race ./...` passes.
* Dry-run does not report uploads.
* Checksum handling is strict.
* Retry behavior only retries retryable failures.
* Worker pool prevents unbounded goroutines.
* R2 not-found detection is robust.
* ZIP extraction is path-safe.
* Parquet schema includes useful metadata.
* Re-running ingestion skips existing R2 files.

```

Bottom line: **good build, not done-hardening.** After those fixes, I’d bump it close to **8.7–9/10** for a dedicated historical ingestion tool.
```
