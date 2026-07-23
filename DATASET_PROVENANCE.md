# Dataset Provenance in AK Universe

This document describes how exact dataset provenance is managed in `ak-historian` and how it is integrated into the RIF (Research Interchange Format) via dataset manifests.

## What is a Dataset Manifest?
A dataset manifest (`dataset_manifest.json`) is a deterministic representation of a local dataset (such as parquet files). It includes:
- Information about the dataset (ID, Role, Git SHA)
- Deterministic hashing of the dataset files
- Validation status and missing field warnings
- Dataset coverage metadata when requested
- Universe policy, universe hashes, and survivorship bias risk assessment when a `universe_manifest.json` is supplied
- Asset lifecycle hashes, evidence summaries, listing/delisting evidence status, and lifecycle warnings when the universe manifest was lifecycle-backed
- Exchange metadata snapshot hashes, coverage window, current-only status, and point-in-time coverage status when lifecycle evidence came from snapshots

## Hashes
### dataset_hash
The `dataset_hash` is computed by finding all `.parquet` files within the `data_root`, hashing each file with SHA-256, sorting the file paths, and then hashing the concatenated string of `relative_path:sha256\n`.

### manifest_hash
The `manifest_hash` is a deterministic hash of the manifest file itself.
1. The JSON representation of the manifest is normalized.
2. Unstable properties like `generated_at_utc`, `data_root`, and `manifest_hash` are excluded.
3. The remaining structured data is marshaled into JSON and hashed with SHA-256.

## Generating a Manifest
You can generate a dataset manifest using the `ak-historian dataset-manifest` command:

```bash
ak-historian dataset-manifest \
  --data-root /path/to/dataset \
  --out /path/to/dataset_manifest.json \
  --dataset-id my-dataset \
  --dataset-role candles \
  --source-type local-parquet
```

To link the dataset to an explicit universe policy:

```bash
ak-historian dataset-manifest \
  --data-root /path/to/dataset \
  --out /path/to/dataset_manifest.json \
  --dataset-id my-dataset \
  --dataset-role research \
  --source-type local_parquet \
  --include-coverage \
  --coverage-mode strict \
  --universe-manifest /path/to/universe_manifest.json
```

When `--universe-manifest` is present, the dataset manifest embeds:

- `universe_id`
- `universe_hash`
- `universe_manifest_hash`
- `universe_policy`
- `includes_delisted_assets`
- `survivorship_bias_risk`
- `lifecycle_id`
- `lifecycle_hash`
- `lifecycle_manifest_hash`
- `lifecycle_evidence_level_summary`
- `lifecycle_warnings`
- `listing_evidence_status`
- `delisting_evidence_status`
- `survivorship_support_status`
- `exchange_metadata_snapshot_hash`
- `exchange_metadata_snapshot_manifest_hash`
- `exchange_metadata_snapshot_archive_hash`
- `exchange_metadata_snapshot_coverage_start_utc`
- `exchange_metadata_snapshot_coverage_end_utc`
- `exchange_metadata_snapshot_evidence_level`
- `exchange_metadata_snapshot_current_only`
- `point_in_time_coverage_status`
- universe warning codes

## How `ak-engine` Consumes Manifests
`ak-engine` commands that run research simulations (like `evaluate-candidate-deep` and `evaluate-research-leads-deep`) can take a `--dataset-manifest` flag or auto-discover a `dataset_manifest.json` file inside the local parquet directory.

The manifest's fields are parsed and injected into:
1. `research.lock`: Includes `dataset_hash`, `dataset_manifest_hash`, `source_git_sha`, `universe_hash`, `universe_manifest_hash`, `universe_policy`, `survivorship_bias_risk`, `lifecycle_hash`, lifecycle evidence summary, and exchange metadata snapshot summary fields when present.
2. `research_audit.json`: Includes `dataset_provenance` with file count, coverage status, universe policy, survivorship risk, universe warnings, lifecycle warnings, lifecycle evidence status, exchange snapshot coverage status, and structured RIF warning objects.

## Survivorship Warnings
Every dataset manifest includes a survivorship risk assessment. If a dataset relies on an explicit list of symbols rather than a point-in-time snapshot, the universe manifest carries warnings such as `UNIVERSE_EXPLICIT_SYMBOL_LIST_SURVIVORSHIP_RISK` and a non-LOW `survivorship_bias_risk`.

Dataset/universe consistency warnings include:

- `DATASET_SYMBOL_NOT_IN_UNIVERSE`
- `UNIVERSE_SYMBOL_MISSING_DATA`
- `DATASET_RANGE_OUTSIDE_UNIVERSE_WINDOW`
- `UNIVERSE_DATASET_MARKET_TYPE_MISMATCH`
- `UNIVERSE_DATASET_QUOTE_ASSET_MISMATCH`

These flow into RIF as structured warnings such as `RIF_UNIVERSE_DATASET_MISMATCH`, `RIF_UNIVERSE_NOT_POINT_IN_TIME`, `RIF_SURVIVORSHIP_BIAS_RISK`, and `RIF_UNIVERSE_LOW_RISK_UNPROVEN`.

Lifecycle warnings flow into RIF as structured warnings such as:

- `RIF_LIFECYCLE_MANIFEST_MISSING`
- `RIF_LIFECYCLE_EVIDENCE_WEAK`
- `RIF_LIFECYCLE_LISTING_EVIDENCE_MISSING`
- `RIF_LIFECYCLE_DELISTING_EVIDENCE_MISSING`
- `RIF_SURVIVORSHIP_NOT_SOLVED`
- `RIF_LOW_SURVIVORSHIP_RISK_UNPROVEN`
- `RIF_EXCHANGE_SNAPSHOT_CURRENT_ONLY`
- `RIF_SNAPSHOT_ARCHIVE_DOES_NOT_COVER_RESEARCH_WINDOW`
- `RIF_POINT_IN_TIME_EVIDENCE_PARTIAL`
- `RIF_DELISTING_NOT_PROVEN`

## Promotion Blocking
If a dataset manifest, complete universe metadata, or sufficient lifecycle metadata is missing for a promotion-grade output, `ak-engine` rejects promotion with structured RIF warnings. Exploratory research flows still emit warnings but do not crash.

## Dataset Coverage Auditor

You can use the built-in auditor to inspect local parquet files:

```bash
ak-historian dataset-coverage --data-root path/to/candles
```

### Fast vs Strict Coverage Mode

- **Fast mode** (`--mode fast`): Tries to use DuckDB (if installed) or parquet metadata to swiftly gather min/max timestamps and row counts. It is efficient but may not strictly validate internal file row ordering or individual gaps if duckdb is used.
- **Strict mode** (`--mode strict`): Scans every single row across all files sequentially, exactly as the engine would read them. It computes precise gap counts, missing row metrics, and out-of-order timestamp errors.

### Expected Candle Counts

The coverage module calculates the theoretical number of candles by comparing the `min_timestamp_utc` and `max_timestamp_utc` for each symbol/interval group. If `max - min / interval_ms + 1` does not match the deduplicated row count, missing rows are reported.

### Coverage Warnings

- `DATASET_ROW_COUNTS_UNAVAILABLE`: Row counts could not be read.
- `DATASET_TIMESTAMP_RANGE_UNAVAILABLE`: Timestamps could not be extracted.
- `DATASET_GAPS_DETECTED`: Physical time gaps are present.
- `DATASET_DUPLICATE_TIMESTAMPS`: A timestamp appeared more than once.
- `DATASET_OUT_OF_ORDER_TIMESTAMPS`: A timestamp decreased compared to the previous row (requires strict mode).
- `DATASET_COVERAGE_BELOW_THRESHOLD`: Coverage % is below the defined threshold.

### Generating Manifest with Coverage

```bash
ak-historian dataset-manifest \
  --data-root path/to/dataset \
  --out dataset_manifest.json \
  --dataset-id prod_dataset \
  --dataset-role research \
  --source-type local_parquet \
  --include-coverage \
  --coverage-mode strict
```
