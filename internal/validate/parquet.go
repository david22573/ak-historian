package validate

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/david22573/ak-historian/internal/parquetutil"
)

type ParquetStats struct {
	RowCount      int64 `json:"row_count"`
	MinOpenTimeMS int64 `json:"min_open_time_ms"`
	MaxOpenTimeMS int64 `json:"max_open_time_ms"`
}

func ValidateParquet(_ context.Context, parquetPath string) (ParquetStats, error) {
	if _, err := exec.LookPath("duckdb"); err == nil {
		return validateParquetWithDuckDB(parquetPath)
	}

	stats, err := parquetutil.ReadStats(parquetPath)
	if err != nil {
		return ParquetStats{}, err
	}

	out := ParquetStats{
		RowCount:      stats.RowCount,
		MinOpenTimeMS: stats.MinOpenTimeMS,
		MaxOpenTimeMS: stats.MaxOpenTimeMS,
	}

	if out.RowCount == 0 {
		return out, fmt.Errorf("parquet file is empty")
	}

	if out.MinOpenTimeMS > out.MaxOpenTimeMS {
		return out, fmt.Errorf("invalid time range: min (%d) > max (%d)", out.MinOpenTimeMS, out.MaxOpenTimeMS)
	}

	return out, nil
}

func validateParquetWithDuckDB(parquetPath string) (ParquetStats, error) {
	path := strings.ReplaceAll(parquetPath, "'", "''")
	query := fmt.Sprintf(`
SELECT
    COUNT(*) AS row_count,
    CAST(MIN(open_time_ms) AS BIGINT) AS min_open_time_ms,
    CAST(MAX(open_time_ms) AS BIGINT) AS max_open_time_ms
FROM read_parquet('%s');
`, path)

	cmd := exec.Command("duckdb", "-json", "-c", query)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ParquetStats{}, fmt.Errorf("duckdb validation failed: %w, output: %s", err, string(output))
	}

	var results []ParquetStats
	if err := json.Unmarshal(output, &results); err != nil {
		return ParquetStats{}, fmt.Errorf("failed to parse duckdb output: %w", err)
	}
	if len(results) == 0 {
		return ParquetStats{}, fmt.Errorf("no results from duckdb validation")
	}

	return results[0], nil
}
