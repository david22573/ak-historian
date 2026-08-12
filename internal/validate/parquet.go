package validate

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"strconv"

	"github.com/david22573/ak-historian/internal/duckdbquery"
	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/reader"
)

type ParquetStats struct {
	RowCount      int64 `json:"row_count"`
	MinOpenTimeMS int64 `json:"min_open_time_ms"`
	MaxOpenTimeMS int64 `json:"max_open_time_ms"`
}

type CandleExpectations struct {
	Market   string
	Symbol   string
	Interval string
}

type candleRow struct {
	Market              string  `json:"market" parquet:"name=market, type=BYTE_ARRAY, convertedtype=UTF8"`
	Symbol              string  `json:"symbol" parquet:"name=symbol, type=BYTE_ARRAY, convertedtype=UTF8"`
	Interval            string  `json:"interval" parquet:"name=interval, type=BYTE_ARRAY, convertedtype=UTF8"`
	OpenTimeMS          int64   `json:"open_time_ms" parquet:"name=open_time_ms, type=INT64"`
	Open                float64 `json:"open" parquet:"name=open, type=DOUBLE"`
	High                float64 `json:"high" parquet:"name=high, type=DOUBLE"`
	Low                 float64 `json:"low" parquet:"name=low, type=DOUBLE"`
	Close               float64 `json:"close" parquet:"name=close, type=DOUBLE"`
	Volume              float64 `json:"volume" parquet:"name=volume, type=DOUBLE"`
	CloseTimeMS         int64   `json:"close_time_ms" parquet:"name=close_time_ms, type=INT64"`
	QuoteAssetVolume    float64 `json:"quote_asset_volume" parquet:"name=quote_asset_volume, type=DOUBLE"`
	NumberOfTrades      int64   `json:"number_of_trades" parquet:"name=number_of_trades, type=INT64"`
	TakerBuyBaseVolume  float64 `json:"taker_buy_base_volume" parquet:"name=taker_buy_base_volume, type=DOUBLE"`
	TakerBuyQuoteVolume float64 `json:"taker_buy_quote_volume" parquet:"name=taker_buy_quote_volume, type=DOUBLE"`
}

func ValidateParquet(ctx context.Context, parquetPath string) (ParquetStats, error) {
	return validateParquet(ctx, parquetPath, CandleExpectations{}, readCandleRows)
}

func ValidateParquetFor(ctx context.Context, parquetPath string, expected CandleExpectations) (ParquetStats, error) {
	return validateParquet(ctx, parquetPath, expected, readCandleRows)
}

type candleRowReader func(context.Context, string) ([]candleRow, error)

func validateParquet(ctx context.Context, parquetPath string, expected CandleExpectations, read candleRowReader) (ParquetStats, error) {
	rows, err := read(ctx, parquetPath)
	if err != nil {
		return ParquetStats{}, err
	}
	return validateCandleRows(rows, expected)
}

func readCandleRows(ctx context.Context, path string) ([]candleRow, error) {
	if _, err := exec.LookPath("duckdb"); err == nil {
		return readCandleRowsDuckDB(ctx, path)
	}
	return readCandleRowsFallback(ctx, path)
}

func readCandleRowsDuckDB(ctx context.Context, path string) ([]candleRow, error) {
	query := fmt.Sprintf(`SELECT market, symbol, interval, open_time_ms, open, high, low, close, volume, close_time_ms, quote_asset_volume, number_of_trades, taker_buy_base_volume, taker_buy_quote_volume FROM read_parquet(%s);`, duckdbquery.QuoteString(path))
	output, err := duckdbquery.RunQuery(ctx, query, "-json")
	if err != nil {
		return nil, fmt.Errorf("duckdb validation read failed: %w", err)
	}
	var rows []candleRow
	if err := json.Unmarshal(output, &rows); err != nil {
		return nil, fmt.Errorf("decode duckdb validation rows: %w", err)
	}
	return rows, nil
}

func readCandleRowsFallback(_ context.Context, path string) (rows []candleRow, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("fallback parquet validation read failed: %v", recovered)
		}
	}()
	fr, err := local.NewLocalFileReader(path)
	if err != nil {
		return nil, fmt.Errorf("open parquet for validation: %w", err)
	}
	defer fr.Close()
	pr, err := reader.NewParquetReader(fr, new(candleRow), 1)
	if err != nil {
		return nil, fmt.Errorf("create fallback parquet validator: %w", err)
	}
	defer pr.ReadStop()
	rows = make([]candleRow, int(pr.GetNumRows()))
	if len(rows) > 0 {
		if err := pr.Read(&rows); err != nil {
			return nil, fmt.Errorf("fallback parquet validation read failed: %w", err)
		}
	}
	return rows, nil
}

func validateCandleRows(rows []candleRow, expected CandleExpectations) (ParquetStats, error) {
	if len(rows) == 0 {
		return ParquetStats{}, fmt.Errorf("parquet file is empty")
	}
	if expected.Market == "" {
		expected.Market = rows[0].Market
	}
	if expected.Symbol == "" {
		expected.Symbol = rows[0].Symbol
	}
	if expected.Interval == "" {
		expected.Interval = rows[0].Interval
	}
	durationMS, err := intervalMS(expected.Interval)
	if err != nil {
		return ParquetStats{}, err
	}
	stats := ParquetStats{RowCount: int64(len(rows)), MinOpenTimeMS: rows[0].OpenTimeMS, MaxOpenTimeMS: rows[0].OpenTimeMS}
	for i, row := range rows {
		if row.Market != expected.Market || row.Symbol != expected.Symbol || row.Interval != expected.Interval {
			return ParquetStats{}, fmt.Errorf("row %d: scope mismatch got %s/%s/%s want %s/%s/%s", i, row.Market, row.Symbol, row.Interval, expected.Market, expected.Symbol, expected.Interval)
		}
		if row.OpenTimeMS <= 0 || row.CloseTimeMS != row.OpenTimeMS+durationMS-1 {
			return ParquetStats{}, fmt.Errorf("row %d: invalid open/close time", i)
		}
		if !finitePositive(row.Open) || !finitePositive(row.High) || !finitePositive(row.Low) || !finitePositive(row.Close) {
			return ParquetStats{}, fmt.Errorf("row %d: non-finite or nonpositive OHLC", i)
		}
		if row.High < row.Low || row.Open < row.Low || row.Open > row.High || row.Close < row.Low || row.Close > row.High {
			return ParquetStats{}, fmt.Errorf("row %d: malformed OHLC", i)
		}
		if !finiteNonnegative(row.Volume) || !finiteNonnegative(row.QuoteAssetVolume) || !finiteNonnegative(row.TakerBuyBaseVolume) || !finiteNonnegative(row.TakerBuyQuoteVolume) || row.NumberOfTrades < 0 {
			return ParquetStats{}, fmt.Errorf("row %d: invalid volume/trades", i)
		}
		if i > 0 {
			delta := row.OpenTimeMS - rows[i-1].OpenTimeMS
			if delta <= 0 {
				return ParquetStats{}, fmt.Errorf("row %d: duplicate or out-of-order open_time_ms", i)
			}
			if delta != durationMS {
				return ParquetStats{}, fmt.Errorf("row %d: cadence mismatch got %d want %d", i, delta, durationMS)
			}
		}
		if row.OpenTimeMS < stats.MinOpenTimeMS {
			stats.MinOpenTimeMS = row.OpenTimeMS
		}
		if row.OpenTimeMS > stats.MaxOpenTimeMS {
			stats.MaxOpenTimeMS = row.OpenTimeMS
		}
	}
	return stats, nil
}

func intervalMS(interval string) (int64, error) {
	if len(interval) < 2 {
		return 0, fmt.Errorf("invalid interval %q", interval)
	}
	value, err := strconv.ParseInt(interval[:len(interval)-1], 10, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid interval %q", interval)
	}
	switch interval[len(interval)-1] {
	case 'm':
		return value * 60 * 1000, nil
	case 'h':
		return value * 60 * 60 * 1000, nil
	case 'd':
		return value * 24 * 60 * 60 * 1000, nil
	case 'w':
		return value * 7 * 24 * 60 * 60 * 1000, nil
	default:
		return 0, fmt.Errorf("invalid interval %q", interval)
	}
}

func finitePositive(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func finiteNonnegative(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}
