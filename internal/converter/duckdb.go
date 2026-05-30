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
        column0 AS open_time_ms,
        column1 AS open,
        column2 AS high,
        column3 AS low,
        column4 AS close,
        column5 AS volume,
        column6 AS close_time_ms,
        column7 AS quote_asset_volume,
        column8 AS number_of_trades,
        column9 AS taker_buy_base_volume,
        column10 AS taker_buy_quote_volume
    FROM read_csv('%s', 
        header=false, 
        columns={
            'column0': 'BIGINT',
            'column1': 'DOUBLE',
            'column2': 'DOUBLE',
            'column3': 'DOUBLE',
            'column4': 'DOUBLE',
            'column5': 'DOUBLE',
            'column6': 'BIGINT',
            'column7': 'DOUBLE',
            'column8': 'BIGINT',
            'column9': 'DOUBLE',
            'column10': 'DOUBLE'
        }
    )
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
