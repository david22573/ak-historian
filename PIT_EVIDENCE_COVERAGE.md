# Point-in-Time Evidence Coverage Auditor

The `pit-evidence-coverage` module assesses the availability of cryptographic evidence for symbols during a given research window.

## Purpose
While lifecycle metadata asserts when a symbol was listed and delisted, PIT evidence coverage verifies whether the metadata was captured with lookahead-free snapshots and verified exchanges. 

## Definitions
- **PIT_ELIGIBLE**: Full coverage via verified exchange snapshots for the entire research window.
- **PIT_PARTIAL**: Some gaps in evidence or missing snapshots for part of the window.
- **PIT_NOT_ELIGIBLE**: Serious gaps such as only local data available or unverified backfill evidence.

## CLI Usage
```bash
ak-historian pit-evidence-coverage \
  --asset-lifecycle-manifest lifecycle.json \
  --universe-manifest universe.json \
  --dataset-manifest dataset.json \
  --research-start "2021-01-01T00:00:00Z" \
  --research-end "2021-12-31T23:59:59Z" \
  --out report.json
```
