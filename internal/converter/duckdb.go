package converter

import (
	"context"
	"fmt"

	"github.com/david22573/ak-historian/internal/duckdbquery"
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
	query := fmt.Sprintf(`
COPY (
    SELECT
        %s AS market,
        %s AS symbol,
        %s AS interval,
        %s AS period,
        %s AS source_date,
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
    FROM read_csv_auto(%s, all_varchar=true)
)
TO %s
(FORMAT PARQUET, COMPRESSION ZSTD);
`,
		duckdbquery.QuoteString(opts.Market),
		duckdbquery.QuoteString(opts.Symbol),
		duckdbquery.QuoteString(opts.Interval),
		duckdbquery.QuoteString(opts.Period),
		duckdbquery.QuoteString(opts.SourceDate),
		duckdbquery.QuoteString(opts.CSVPath),
		duckdbquery.QuoteString(opts.ParquetPath),
	)

	_, err := duckdbquery.RunQuery(ctx, query)
	if err != nil {
		return fmt.Errorf("duckdb conversion failed: %w", err)
	}

	return nil
}
