package validate

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/davidmiguel22573/ak-historian/internal/converter"
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
