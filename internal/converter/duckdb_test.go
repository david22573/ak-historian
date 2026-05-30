package converter

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertKlinesCSVToParquet(t *testing.T) {
	// Skip if duckdb is not in PATH
	if _, err := exec.LookPath("duckdb"); err != nil {
		t.Skip("duckdb not found in PATH, skipping converter test")
	}

	tmpDir, _ := os.MkdirTemp("", "converter-test")
	defer os.RemoveAll(tmpDir)

	t.Run("success", func(t *testing.T) {
		csvPath := filepath.Join(tmpDir, "test.csv")
		parquetPath := filepath.Join(tmpDir, "test.parquet")

		// Sample Binance kline CSV (11 columns)
		csvContent := "1704067200000,42283.58,42345.67,42270.01,42300.00,10.5,1704067259999,444150.0,500,5.0,210000.0\n"
		_ = os.WriteFile(csvPath, []byte(csvContent), 0644)

		opts := ConvertOptions{
			CSVPath:     csvPath,
			ParquetPath: parquetPath,
			Market:      "futures-um",
			Symbol:      "BTCUSDT",
			Interval:    "1m",
			Period:      "monthly",
			SourceDate:  "2024-01",
		}

		err := ConvertKlinesCSVToParquet(context.Background(), opts)
		if err != nil {
			t.Fatalf("ConvertKlinesCSVToParquet() error = %v", err)
		}

		if _, err := os.Stat(parquetPath); os.IsNotExist(err) {
			t.Error("Parquet file was not created")
		}

		// Verify schema using DuckDB
		verifyQuery := fmt.Sprintf("DESCRIBE SELECT * FROM '%s';", parquetPath)
		verifyCmd := exec.Command("duckdb", "-c", verifyQuery)
		verifyOutput, err := verifyCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to describe parquet: %v, output: %s", err, string(verifyOutput))
		}

		outputStr := string(verifyOutput)
		expectedColumns := []string{
			"market", "symbol", "interval", "period", "source_date",
			"open_time_ms", "open", "high", "low", "close", "volume",
			"close_time_ms", "quote_asset_volume", "number_of_trades",
			"taker_buy_base_volume", "taker_buy_quote_volume",
		}
		for _, col := range expectedColumns {
			if !strings.Contains(outputStr, col) {
				t.Errorf("Expected column %s not found in schema:\n%s", col, outputStr)
			}
		}
	})

	t.Run("bad csv", func(t *testing.T) {
		csvPath := filepath.Join(tmpDir, "bad.csv")
		parquetPath := filepath.Join(tmpDir, "bad.parquet")

		// CSV with wrong number of columns (3 instead of 11)
		csvContent := "1,2,3\n"
		_ = os.WriteFile(csvPath, []byte(csvContent), 0644)

		opts := ConvertOptions{
			CSVPath:     csvPath,
			ParquetPath: parquetPath,
		}

		err := ConvertKlinesCSVToParquet(context.Background(), opts)
		if err == nil {
			t.Error("Expected error for bad CSV, got nil")
		}
	})
}
