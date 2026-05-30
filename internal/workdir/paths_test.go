package workdir

import (
	"path/filepath"
	"testing"

	"github.com/davidmiguel22573/ak-historian/internal/binance"
)

func TestBuildPaths(t *testing.T) {
	root := ".ak-historian/work"
	spec := binance.ArchiveSpec{
		Market:   "futures-um",
		Symbol:   "BTCUSDT",
		Interval: "1m",
		Period:   "monthly",
		Date:     "2024-01",
	}

	got, err := BuildPaths(root, spec)
	if err != nil {
		t.Fatalf("BuildPaths() error = %v", err)
	}

	wantItemDir := filepath.Join(root, "futures-um", "1m", "BTCUSDT", "monthly", "2024-01")
	if got.ItemDir != wantItemDir {
		t.Errorf("ItemDir got = %v, want %v", got.ItemDir, wantItemDir)
	}

	wantZip := filepath.Join(wantItemDir, "BTCUSDT-1m-2024-01.zip")
	if got.ZipPath != wantZip {
		t.Errorf("ZipPath got = %v, want %v", got.ZipPath, wantZip)
	}

	wantCSV := filepath.Join(wantItemDir, "BTCUSDT-1m-2024-01.csv")
	if got.CSVPath != wantCSV {
		t.Errorf("CSVPath got = %v, want %v", got.CSVPath, wantCSV)
	}

	wantParquet := filepath.Join(wantItemDir, "BTCUSDT-1m-2024-01.parquet")
	if got.ParquetPath != wantParquet {
		t.Errorf("ParquetPath got = %v, want %v", got.ParquetPath, wantParquet)
	}
}
