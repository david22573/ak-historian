package datasets

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/david22573/ak-historian/internal/datasets/derivatives"
	"github.com/david22573/ak-historian/internal/datasets/sentiment"
)

func ValidateSentimentRows(rows []sentiment.Row) error {
	if len(rows) == 0 {
		return fmt.Errorf("empty rows")
	}

	seenEvents := make(map[int64]bool)
	var prevEvent int64

	for i, r := range rows {
		if r.Source == "" {
			return fmt.Errorf("row %d: missing source", i)
		}
		if r.Dataset == "" {
			return fmt.Errorf("row %d: missing dataset", i)
		}
		if r.Scope == "" {
			return fmt.Errorf("row %d: missing scope", i)
		}
		if r.Interval == "" {
			return fmt.Errorf("row %d: missing interval", i)
		}
		if r.EventTimeMS <= 0 {
			return fmt.Errorf("row %d: event_time_ms <= 0", i)
		}
		if r.AvailableAtMS <= 0 {
			return fmt.Errorf("row %d: available_at_ms <= 0", i)
		}
		if r.AvailableAtMS < r.EventTimeMS {
			return fmt.Errorf("row %d: available_at_ms < event_time_ms", i)
		}
		if seenEvents[r.EventTimeMS] {
			return fmt.Errorf("row %d: duplicate event_time_ms %d", i, r.EventTimeMS)
		}
		if i > 0 && r.EventTimeMS < prevEvent {
			return fmt.Errorf("row %d: out-of-order rows (event_time_ms %d < %d)", i, r.EventTimeMS, prevEvent)
		}
		if r.Score < 0 || r.Score > 100 {
			return fmt.Errorf("row %d: score outside 0..100 (got %f)", i, r.Score)
		}
		if r.Intensity < 0 || r.Intensity > 1 {
			return fmt.Errorf("row %d: intensity outside 0..1 (got %f)", i, r.Intensity)
		}

		seenEvents[r.EventTimeMS] = true
		prevEvent = r.EventTimeMS
	}

	return nil
}

func ValidateDerivativesRows(rows []derivatives.Row) error {
	if len(rows) == 0 {
		return fmt.Errorf("empty rows")
	}

	seenEvents := make(map[int64]bool)
	var prevEvent int64

	for i, r := range rows {
		if r.Source == "" {
			return fmt.Errorf("row %d: missing source", i)
		}
		if r.Dataset == "" {
			return fmt.Errorf("row %d: missing dataset", i)
		}
		if r.Market == "" {
			return fmt.Errorf("row %d: missing market", i)
		}
		if r.Symbol == "" {
			return fmt.Errorf("row %d: missing symbol", i)
		}
		if r.Interval == "" {
			return fmt.Errorf("row %d: missing interval", i)
		}
		if r.EventTimeMS <= 0 {
			return fmt.Errorf("row %d: event_time_ms <= 0", i)
		}
		if r.AvailableAtMS <= 0 {
			return fmt.Errorf("row %d: available_at_ms <= 0", i)
		}
		if r.AvailableAtMS < r.EventTimeMS {
			return fmt.Errorf("row %d: available_at_ms < event_time_ms", i)
		}
		if r.IngestedAtMS <= 0 {
			return fmt.Errorf("row %d: ingested_at_ms <= 0", i)
		}
		if r.AvailabilityPolicyID != derivatives.AvailabilityPolicyObservedIngestionID || r.AvailabilityPolicyVersion != derivatives.AvailabilityPolicyObservedIngestionVersion {
			return fmt.Errorf("row %d: unsupported availability policy %q version %q", i, r.AvailabilityPolicyID, r.AvailabilityPolicyVersion)
		}
		if r.AvailableAtMS != r.IngestedAtMS {
			return fmt.Errorf("row %d: observed-ingestion availability must equal ingested_at_ms", i)
		}
		if r.SourceVersion == "" {
			return fmt.Errorf("row %d: missing source_version", i)
		}
		if seenEvents[r.EventTimeMS] {
			return fmt.Errorf("row %d: duplicate event_time_ms %d", i, r.EventTimeMS)
		}
		if i > 0 && r.EventTimeMS < prevEvent {
			return fmt.Errorf("row %d: out-of-order rows (event_time_ms %d < %d)", i, r.EventTimeMS, prevEvent)
		}

		seenEvents[r.EventTimeMS] = true
		prevEvent = r.EventTimeMS
	}

	return nil
}

func ValidateDerivativesRowsFor(rows []derivatives.Row, expected derivatives.FetchRequest) error {
	if err := ValidateDerivativesRows(rows); err != nil {
		return err
	}
	for i, row := range rows {
		if row.Source != expected.Source || row.Dataset != expected.Dataset || row.Market != expected.Market || row.Symbol != strings.ToUpper(expected.Symbol) || row.Interval != expected.Interval {
			return fmt.Errorf("row %d: scope mismatch got %s/%s/%s/%s/%s want %s/%s/%s/%s/%s", i, row.Source, row.Dataset, row.Market, row.Symbol, row.Interval, expected.Source, expected.Dataset, expected.Market, strings.ToUpper(expected.Symbol), expected.Interval)
		}
	}
	return nil
}

func ValidateDatasetParquet(ctx context.Context, path string) (RowStats, error) {
	_, err := exec.LookPath("duckdb")
	if err != nil {
		return RowStats{}, fmt.Errorf("parquet validation requires duckdb installed")
	}

	escapedPath := strings.ReplaceAll(path, "'", "''")

	query := fmt.Sprintf(`SELECT source, dataset, market, symbol, interval, event_time_ms, available_at_ms, ingested_at_ms, value, extra_1, extra_2, source_version, availability_policy_id, availability_policy_version FROM read_parquet('%s');`, escapedPath)

	cmd := exec.CommandContext(ctx, "duckdb", "-json", "-c", query)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return RowStats{}, fmt.Errorf("duckdb parquet validation failed: %s: %w", string(output), err)
	}

	var rows []derivatives.Row
	if err := json.Unmarshal(output, &rows); err != nil {
		return RowStats{}, fmt.Errorf("decode derivative parquet rows: %w", err)
	}
	if err := ValidateDerivativesRows(rows); err != nil {
		return RowStats{}, err
	}
	stats := RowStats{RowCount: int64(len(rows))}
	for i, row := range rows {
		if i == 0 || row.EventTimeMS < stats.MinEventTimeMS {
			stats.MinEventTimeMS = row.EventTimeMS
		}
		if i == 0 || row.EventTimeMS > stats.MaxEventTimeMS {
			stats.MaxEventTimeMS = row.EventTimeMS
		}
		if i == 0 || row.AvailableAtMS < stats.MinAvailableAtMS {
			stats.MinAvailableAtMS = row.AvailableAtMS
		}
		if i == 0 || row.AvailableAtMS > stats.MaxAvailableAtMS {
			stats.MaxAvailableAtMS = row.AvailableAtMS
		}
	}
	return stats, nil
}
