package validate

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type ParquetStats struct {
	RowCount      int64 `json:"row_count"`
	MinOpenTimeMS int64 `json:"min_open_time_ms"`
	MaxOpenTimeMS int64 `json:"max_open_time_ms"`
}

func ValidateParquet(ctx context.Context, parquetPath string) (ParquetStats, error) {
	// Escape path
	path := strings.ReplaceAll(parquetPath, "'", "''")

	query := fmt.Sprintf(`
SELECT
    COUNT(*) AS row_count,
    CAST(MIN(open_time_ms) AS BIGINT) AS min_open_time_ms,
    CAST(MAX(open_time_ms) AS BIGINT) AS max_open_time_ms
FROM read_parquet('%s');
`, path)

	// Get output in JSON format for easier parsing
	cmd := exec.CommandContext(ctx, "duckdb", "-json", "-c", query)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ParquetStats{}, fmt.Errorf("duckdb validation failed: %w, output: %s", err, string(output))
	}

	var results []ParquetStats
	err = json.Unmarshal(output, &results)
	if err != nil {
		return ParquetStats{}, fmt.Errorf("failed to parse duckdb output: %w", err)
	}

	if len(results) == 0 {
		return ParquetStats{}, fmt.Errorf("no results from duckdb validation")
	}

	stats := results[0]

	if stats.RowCount == 0 {
		return stats, fmt.Errorf("parquet file is empty")
	}

	if stats.MinOpenTimeMS > stats.MaxOpenTimeMS {
		return stats, fmt.Errorf("invalid time range: min (%d) > max (%d)", stats.MinOpenTimeMS, stats.MaxOpenTimeMS)
	}

	return stats, nil
}
