package converter

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type ConvertOptions struct {
	CSVPath     string
	ParquetPath string
	Market      string
	Symbol      string
	Interval    string
	Period      string
	SourceDate  string
}

func ConvertKlinesCSVToParquet(ctx context.Context, opts ConvertOptions) error {
	// Simple SQL escaping by replacing single quotes with double single quotes
	market := strings.ReplaceAll(opts.Market, "'", "''")
	symbol := strings.ReplaceAll(opts.Symbol, "'", "''")
	interval := strings.ReplaceAll(opts.Interval, "'", "''")
	period := strings.ReplaceAll(opts.Period, "'", "''")
	sourceDate := strings.ReplaceAll(opts.SourceDate, "'", "''")
	csvPath := strings.ReplaceAll(opts.CSVPath, "'", "''")
	parquetPath := strings.ReplaceAll(opts.ParquetPath, "'", "''")

	query := fmt.Sprintf(`
COPY (
    SELECT
        '%s' AS market,
        '%s' AS symbol,
        '%s' AS interval,
        '%s' AS period,
        '%s' AS source_date,
        CAST(#1 AS BIGINT) AS open_time_ms,
        CAST(#2 AS DOUBLE) AS open,
        CAST(#3 AS DOUBLE) AS high,
        CAST(#4 AS DOUBLE) AS low,
        CAST(#5 AS DOUBLE) AS close,
        CAST(#6 AS DOUBLE) AS volume,
        CAST(#7 AS BIGINT) AS close_time_ms,
        CAST(#8 AS DOUBLE) AS quote_asset_volume,
        CAST(#9 AS BIGINT) AS number_of_trades,
        CAST(#10 AS DOUBLE) AS taker_buy_base_volume,
        CAST(#11 AS DOUBLE) AS taker_buy_quote_volume
    FROM read_csv_auto('%s', all_varchar=true)
)
TO '%s'
(FORMAT PARQUET, COMPRESSION ZSTD);
`, market, symbol, interval, period, sourceDate, csvPath, parquetPath)

	cmd := exec.CommandContext(ctx, "duckdb", "-c", query)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("duckdb conversion failed: %w, output: %s", err, string(output))
	}

	return nil
}
