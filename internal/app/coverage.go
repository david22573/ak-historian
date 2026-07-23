package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/david22573/ak-historian/internal/config"
	"github.com/david22573/ak-historian/internal/coverage"
	"github.com/david22573/ak-historian/internal/parquetutil"
	"github.com/david22573/ak-historian/internal/storage"
	"github.com/spf13/cobra"
)

var (
	covMarket   string
	covSymbol   string
	covInterval string
	covFrom     string
	covTo       string
	covSource   string
	covPath     string
)

var coverageCmd = &cobra.Command{
	Use:   "verify-coverage",
	Short: "Verify dataset coverage and continuity",
	RunE: func(cmd *cobra.Command, args []string) error {
		fromTime, err := time.Parse("2006-01-02", covFrom)
		if err != nil {
			return fmt.Errorf("invalid from date: %w", err)
		}
		toTime, err := time.Parse("2006-01-02", covTo)
		if err != nil {
			return fmt.Errorf("invalid to date: %w", err)
		}

		// Align to minute boundary
		fromTime = fromTime.Truncate(time.Minute)
		toTime = toTime.Add(23*time.Hour + 59*time.Minute).Truncate(time.Minute)

		if err := coverage.ValidateSymbol(covSymbol); err != nil {
			return err
		}
		if _, err := coverage.CalculateExpectedCandles(fromTime, toTime, covInterval); err != nil {
			return err
		}

		log.Printf("Verifying coverage for %s %s from %s to %s", covSymbol, covMarket, fromTime.Format(time.RFC3339), toTime.Format(time.RFC3339))

		openTimes, err := LoadOpenTimes(cmd.Context(), covMarket, covSymbol, covInterval, fromTime, toTime, covSource, covPath)
		if err != nil {
			return err
		}

		report := coverage.VerifyCoverage(covMarket, covSymbol, covInterval, fromTime, toTime, openTimes)

		printCoverageReport(report)
		if err := printCoverageJSON(report); err != nil {
			return err
		}

		if report.Status == coverage.StatusFail {
			return fmt.Errorf("coverage verification failed")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(coverageCmd)

	coverageCmd.Flags().StringVar(&covMarket, "market", "futures-um", "market type")
	coverageCmd.Flags().StringVar(&covSymbol, "symbol", "LINKUSDT", "symbol")
	coverageCmd.Flags().StringVar(&covInterval, "interval", "1m", "interval")
	coverageCmd.Flags().StringVar(&covFrom, "from", "", "start date (YYYY-MM-DD)")
	coverageCmd.Flags().StringVar(&covTo, "to", "", "end date (YYYY-MM-DD)")
	coverageCmd.Flags().StringVar(&covSource, "source", "local", "data source (local, r2)")
	coverageCmd.Flags().StringVar(&covPath, "path", ".ak-historian/work", "local path if source=local")

	_ = coverageCmd.MarkFlagRequired("from")
	_ = coverageCmd.MarkFlagRequired("to")
}

func printCoverageReport(report coverage.Report) {
	fmt.Printf("\nCoverage Report\n")
	fmt.Printf("market: %s\n", report.Market)
	fmt.Printf("symbol: %s\n", report.Symbol)
	fmt.Printf("interval: %s\n", report.Interval)
	fmt.Printf("from: %s\n", report.From.Format(time.RFC3339))
	fmt.Printf("to: %s\n", report.To.Format(time.RFC3339))
	fmt.Printf("\n")
	fmt.Printf("expected_candles: %d\n", report.ExpectedCandles)
	fmt.Printf("actual_candles: %d\n", report.ActualCandles)
	fmt.Printf("unique_open_times: %d\n", report.UniqueOpenTimes)
	fmt.Printf("duplicate_open_times: %d\n", report.DuplicateOpenTimes)
	fmt.Printf("missing_candles: %d\n", report.MissingCandles)
	if !report.FirstOpenTime.IsZero() {
		fmt.Printf("first_open_time: %s\n", report.FirstOpenTime.Format(time.RFC3339))
	} else {
		fmt.Printf("first_open_time: none\n")
	}
	if !report.LastOpenTime.IsZero() {
		fmt.Printf("last_open_time: %s\n", report.LastOpenTime.Format(time.RFC3339))
	} else {
		fmt.Printf("last_open_time: none\n")
	}
	if report.FirstGap != nil {
		fmt.Printf("first_gap: %s\n", report.FirstGap.Format(time.RFC3339))
	} else {
		fmt.Printf("first_gap: none\n")
	}
	if report.LastGap != nil {
		fmt.Printf("last_gap: %s\n", report.LastGap.Format(time.RFC3339))
	} else {
		fmt.Printf("last_gap: none\n")
	}
	fmt.Printf("status: %s\n", report.Status)
}

func printCoverageJSON(report coverage.Report) error {
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal coverage report: %w", err)
	}
	fmt.Printf("\n%s\n", payload)
	return nil
}

func LoadOpenTimes(ctx context.Context, market, symbol, interval string, from, to time.Time, source, path string) ([]int64, error) {
	var parquetFiles []string

	if source == "local" {
		// Glob local files
		// candles/{market}/{interval}/symbol={SYMBOL}/year={YYYY}/month={MM}/{SYMBOL}-{INTERVAL}-{DATE}.parquet
		pattern := filepath.Join(path, "candles", market, interval, "symbol="+symbol, "year=*", "month=*", "*.parquet")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to glob local files: %w", err)
		}
		parquetFiles = matches
	} else {
		// Download from R2 with local cache
		cfg, err := config.LoadR2Config()
		if err != nil {
			return nil, err
		}
		r2, err := storage.NewR2Client(ctx, cfg)
		if err != nil {
			return nil, err
		}

		// List all objects for this symbol
		prefix := fmt.Sprintf("candles/%s/%s/symbol=%s/", market, interval, symbol)
		allKeys, err := r2.ListObjects(ctx, prefix)
		if err != nil {
			return nil, err
		}

		// Filter keys by range
		keys := FilterKeysByRange(allKeys, from, to)

		if len(keys) == 0 {
			return nil, fmt.Errorf("no parquet files found in R2 for the given range")
		}

		for _, key := range keys {
			localPath := filepath.Join(path, key)

			// Ensure directory exists
			if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
				return nil, fmt.Errorf("failed to create cache directory: %w", err)
			}

			// Check if exists in cache
			if _, err := os.Stat(localPath); os.IsNotExist(err) {
				log.Printf("Downloading %s to cache...", key)
				err = r2.DownloadFile(ctx, key, localPath)
				if err != nil {
					return nil, err
				}
			} else {
				log.Printf("Using cached: %s", key)
			}
			parquetFiles = append(parquetFiles, localPath)
		}
	}

	if len(parquetFiles) == 0 {
		return nil, fmt.Errorf("no parquet files found")
	}
	return parquetutil.ReadOpenTimes(parquetFiles)
}

func FilterKeysByRange(allKeys []string, from, to time.Time) []string {
	var filtered []string
	for _, key := range allKeys {
		// Extract date part
		// Example: LINKUSDT-1m-2023-01.parquet
		// Example: LINKUSDT-1m-2026-05-01.parquet
		base := filepath.Base(key)
		base = strings.TrimSuffix(base, ".parquet")
		parts := strings.Split(base, "-")
		if len(parts) < 3 {
			continue
		}

		datePart := strings.Join(parts[2:], "-")
		var objStart, objEnd time.Time

		if len(datePart) == 7 { // YYYY-MM
			t, err := time.Parse("2006-01", datePart)
			if err != nil {
				continue
			}
			objStart = t
			objEnd = objStart.AddDate(0, 1, 0).Add(-time.Nanosecond)
		} else if len(datePart) == 10 { // YYYY-MM-DD
			t, err := time.Parse("2006-01-02", datePart)
			if err != nil {
				continue
			}
			objStart = t
			objEnd = objStart.AddDate(0, 0, 1).Add(-time.Nanosecond)
		} else {
			continue
		}

		// Check overlap
		// [objStart, objEnd] overlaps with [from, to] if:
		// objStart <= to && objEnd >= from
		if (objStart.Before(to) || objStart.Equal(to)) && (objEnd.After(from) || objEnd.Equal(from)) {
			filtered = append(filtered, key)
		}
	}
	return filtered
}

// Need DownloadFile in r2.go
