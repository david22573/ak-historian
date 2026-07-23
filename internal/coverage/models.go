package coverage

import "time"

type DatasetCoverage struct {
	Status                    string           `json:"status"`
	TotalFiles                int              `json:"total_files"`
	TotalRows                 int64            `json:"total_rows"`
	TotalExpectedRows         int64            `json:"total_expected_rows"`
	TotalMissingRows          int64            `json:"total_missing_rows"`
	TotalDuplicateTimestamps  int64            `json:"total_duplicate_timestamps"`
	TotalOutOfOrderTimestamps int64            `json:"total_out_of_order_timestamps"`
	TotalGapCount             int64            `json:"total_gap_count"`
	LargestGapSeconds         int64            `json:"largest_gap_seconds"`
	Symbols                   []SymbolCoverage `json:"symbols"`
}

type SymbolCoverage struct {
	Symbol                   string    `json:"symbol"`
	Interval                 string    `json:"interval"`
	MinTimestampUTC          time.Time `json:"min_timestamp_utc"`
	MaxTimestampUTC          time.Time `json:"max_timestamp_utc"`
	RowCount                 int64     `json:"row_count"`
	ExpectedRowCount         int64     `json:"expected_row_count"`
	MissingRowCount          int64     `json:"missing_row_count"`
	DuplicateTimestampCount  int64     `json:"duplicate_timestamp_count"`
	OutOfOrderTimestampCount int64     `json:"out_of_order_timestamp_count"`
	GapCount                 int64     `json:"gap_count"`
	LargestGapSeconds        int64     `json:"largest_gap_seconds"`
	CoveragePct              float64   `json:"coverage_pct"`
	Status                   string    `json:"status"`
	Warnings                 []string  `json:"warnings"`
}

const (
	CoverageStatusPass    = "PASS"
	CoverageStatusWarn    = "WARN"
	CoverageStatusFail    = "FAIL"
	CoverageStatusUnknown = "UNKNOWN"
)

const (
	WarnRowCountsUnavailable      = "DATASET_ROW_COUNTS_UNAVAILABLE"
	WarnTimestampRangeUnavailable = "DATASET_TIMESTAMP_RANGE_UNAVAILABLE"
	WarnGapsDetected              = "DATASET_GAPS_DETECTED"
	WarnDuplicateTimestamps       = "DATASET_DUPLICATE_TIMESTAMPS"
	WarnOutOfOrderTimestamps      = "DATASET_OUT_OF_ORDER_TIMESTAMPS"
	WarnCoverageBelowThreshold    = "DATASET_COVERAGE_BELOW_THRESHOLD"
	WarnIntervalInferFailed       = "DATASET_INTERVAL_INFER_FAILED"
	WarnSymbolInferFailed         = "DATASET_SYMBOL_INFER_FAILED"
)
