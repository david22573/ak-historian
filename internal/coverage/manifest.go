package coverage

import (
	"fmt"
	"regexp"
	"time"
)

type Manifest struct {
	SchemaVersion      int           `json:"schema_version"`
	Market             string        `json:"market"`
	Symbol             string        `json:"symbol"`
	Interval           string        `json:"interval"`
	CoverageStart      time.Time     `json:"coverage_start"`
	CoverageEnd        time.Time     `json:"coverage_end"`
	ExpectedCandles    int64         `json:"expected_candles"`
	ActualCandles      int64         `json:"actual_candles"`
	UniqueOpenTimes    int64         `json:"unique_open_times"`
	DuplicateOpenTimes int64         `json:"duplicate_open_times"`
	MissingCandles     int64         `json:"missing_candles"`
	ObjectCount        int           `json:"object_count"`
	Objects            []ObjectStats `json:"objects"`
	LastVerifiedAt     time.Time     `json:"last_verified_at"`
	Status             string        `json:"status"`
}

type ObjectStats struct {
	Key           string `json:"key"`
	Period        string `json:"period"`
	SourceDate    string `json:"source_date"`
	RowCount      int64  `json:"row_count"`
	MinOpenTimeMS int64  `json:"min_open_time_ms"`
	MaxOpenTimeMS int64  `json:"max_open_time_ms"`
}

var symbolPattern = regexp.MustCompile(`^[A-Z0-9_]+$`)

func ValidateSymbol(symbol string) error {
	if !symbolPattern.MatchString(symbol) {
		return fmt.Errorf("invalid symbol: %s", symbol)
	}
	return nil
}

func ManifestKey(market, interval, symbol string) string {
	return fmt.Sprintf("manifests/%s/%s/symbol=%s/manifest.json", market, interval, symbol)
}
