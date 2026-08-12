package validate

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/david22573/ak-historian/internal/converter"
	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/writer"
)

func TestValidateParquet(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "validate-test")
	defer os.RemoveAll(tmpDir)

	csvPath := filepath.Join(tmpDir, "test.csv")
	parquetPath := filepath.Join(tmpDir, "test.parquet")

	csvContent := "1704067200000,42283.58,42345.67,42270.01,42300.00,10.5,1704067259999,444150.0,500,5.0,210000.0\n"
	os.WriteFile(csvPath, []byte(csvContent), 0644)

	_ = converter.ConvertKlinesCSVToParquet(context.Background(), converter.ConvertOptions{
		CSVPath:     csvPath,
		ParquetPath: parquetPath,
		Market:      "futures-um",
		Symbol:      "BTCUSDT",
		Interval:    "1m",
		Period:      "monthly",
		SourceDate:  "2024-01",
	})

	stats, err := ValidateParquet(context.Background(), parquetPath)
	if err != nil {
		t.Fatalf("ValidateParquet() error = %v", err)
	}

	if stats.RowCount != 1 {
		t.Errorf("RowCount got = %d, want 1", stats.RowCount)
	}

	if stats.MinOpenTimeMS != 1704067200000 {
		t.Errorf("MinOpenTimeMS got = %d, want 1704067200000", stats.MinOpenTimeMS)
	}
}

func TestDuckDBAndFallbackApplySameCandleInvariants(t *testing.T) {
	if _, err := exec.LookPath("duckdb"); err != nil {
		t.Skip("duckdb not found")
	}
	base := int64(1704067200000)
	valid := func(open int64) candleRow {
		return candleRow{Market: "futures-um", Symbol: "BTCUSDT", Interval: "1m", OpenTimeMS: open, CloseTimeMS: open + 59999, Open: 100, High: 101, Low: 99, Close: 100, Volume: 1}
	}
	tests := []struct {
		name string
		rows []candleRow
	}{
		{"empty", nil},
		{"sub cadence", []candleRow{valid(base), valid(base + 30000)}},
		{"wrong scope", []candleRow{func() candleRow { row := valid(base); row.Symbol = "ETHUSDT"; return row }()}},
		{"nonfinite", []candleRow{func() candleRow { row := valid(base); row.Open = math.NaN(); return row }()}},
		{"nonpositive", []candleRow{func() candleRow { row := valid(base); row.Low = 0; return row }()}},
		{"bad close", []candleRow{func() candleRow { row := valid(base); row.CloseTimeMS++; return row }()}},
	}
	expected := CandleExpectations{Market: "futures-um", Symbol: "BTCUSDT", Interval: "1m"}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "invalid.parquet")
			writeCandleRows(t, path, tc.rows)
			for name, read := range map[string]candleRowReader{"duckdb": readCandleRowsDuckDB, "fallback": readCandleRowsFallback} {
				t.Run(name, func(t *testing.T) {
					if _, err := validateParquet(context.Background(), path, expected, read); err == nil {
						t.Fatal("invalid corpus was accepted")
					}
				})
			}
		})
	}

	path := filepath.Join(t.TempDir(), "valid.parquet")
	writeCandleRows(t, path, []candleRow{valid(base), valid(base + 60000)})
	duck, err := validateParquet(context.Background(), path, expected, readCandleRowsDuckDB)
	if err != nil {
		t.Fatal(err)
	}
	fallback, err := validateParquet(context.Background(), path, expected, readCandleRowsFallback)
	if err != nil {
		t.Fatal(err)
	}
	if duck != fallback {
		t.Fatalf("reader stats differ: duckdb=%+v fallback=%+v", duck, fallback)
	}
}

func writeCandleRows(t *testing.T, path string, rows []candleRow) {
	t.Helper()
	fw, err := local.NewLocalFileWriter(path)
	if err != nil {
		t.Fatal(err)
	}
	pw, err := writer.NewParquetWriter(fw, new(candleRow), 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if err := pw.Write(row); err != nil {
			t.Fatal(err)
		}
	}
	if err := pw.WriteStop(); err != nil {
		t.Fatal(err)
	}
	if err := fw.Close(); err != nil {
		t.Fatal(err)
	}
}
