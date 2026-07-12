package datasets

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
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

func ValidateDatasetParquet(ctx context.Context, path string) (RowStats, error) {
	_, err := exec.LookPath("duckdb")
	if err != nil {
		return RowStats{}, fmt.Errorf("parquet validation requires duckdb installed")
	}

	escapedPath := strings.ReplaceAll(path, "'", "''")

	query := fmt.Sprintf(`SELECT
  COUNT(*) AS row_count,
  CAST(MIN(event_time_ms) AS BIGINT) AS min_event_time_ms,
  CAST(MAX(event_time_ms) AS BIGINT) AS max_event_time_ms,
  CAST(MIN(available_at_ms) AS BIGINT) AS min_available_at_ms,
  CAST(MAX(available_at_ms) AS BIGINT) AS max_available_at_ms
FROM read_parquet('%s');`, escapedPath)

	cmd := exec.CommandContext(ctx, "duckdb", "-csv", "-noheader", "-c", query)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return RowStats{}, fmt.Errorf("duckdb parquet validation failed: %s: %w", string(output), err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 {
		return RowStats{}, fmt.Errorf("no output from duckdb")
	}

	fields := strings.Split(lines[0], ",")
	if len(fields) != 5 {
		return RowStats{}, fmt.Errorf("expected 5 fields, got %d", len(fields))
	}

	stats := RowStats{}
	stats.RowCount, _ = strconv.ParseInt(fields[0], 10, 64)
	stats.MinEventTimeMS, _ = strconv.ParseInt(fields[1], 10, 64)
	stats.MaxEventTimeMS, _ = strconv.ParseInt(fields[2], 10, 64)
	stats.MinAvailableAtMS, _ = strconv.ParseInt(fields[3], 10, 64)
	stats.MaxAvailableAtMS, _ = strconv.ParseInt(fields[4], 10, 64)

	return stats, nil
}
