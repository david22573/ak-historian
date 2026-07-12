package app

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/david22573/ak-historian/internal/config"
	"github.com/david22573/ak-historian/internal/coverage"
	"github.com/david22573/ak-historian/internal/storage"
	"github.com/david22573/ak-historian/internal/validate"
	"github.com/spf13/cobra"
)

var (
	manMarket      string
	manSymbol      string
	manInterval    string
	manFrom        string
	manTo          string
	manSource      string
	manPath        string
	manAllowFailed bool
)

var manifestCmd = &cobra.Command{
	Use:   "write-manifest",
	Short: "Generate and upload dataset manifest to R2",
	RunE: func(cmd *cobra.Command, args []string) error {
		fromTime, err := time.Parse("2006-01-02", manFrom)
		if err != nil {
			return fmt.Errorf("invalid from date: %w", err)
		}
		toTime, err := time.Parse("2006-01-02", manTo)
		if err != nil {
			return fmt.Errorf("invalid to date: %w", err)
		}

		// Align to minute boundary
		fromTime = fromTime.Truncate(time.Minute)
		toTime = toTime.Add(23*time.Hour + 59*time.Minute).Truncate(time.Minute)

		if err := coverage.ValidateSymbol(manSymbol); err != nil {
			return err
		}
		if _, err := coverage.CalculateExpectedCandles(fromTime, toTime, manInterval); err != nil {
			return err
		}

		log.Printf("Generating manifest for %s %s from %s to %s", manSymbol, manMarket, fromTime.Format(time.RFC3339), toTime.Format(time.RFC3339))

		cfg, err := config.LoadR2Config()
		if err != nil {
			return err
		}
		r2, err := storage.NewR2Client(cmd.Context(), cfg)
		if err != nil {
			return err
		}

		// 1. Coverage Check
		openTimes, err := LoadOpenTimes(cmd.Context(), manMarket, manSymbol, manInterval, fromTime, toTime, manSource, manPath)
		if err != nil {
			return err
		}

		report := coverage.VerifyCoverage(manMarket, manSymbol, manInterval, fromTime, toTime, openTimes)
		if report.Status != coverage.StatusPass && !manAllowFailed {
			return fmt.Errorf("coverage check failed (status: %s), manifest will not be written unless --allow-failed is used", report.Status)
		}

		// 2. Gather Object Stats
		// We need to re-list because LoadOpenTimes doesn't return the keys used.
		// Actually LoadOpenTimes does internal listing.
		prefix := fmt.Sprintf("candles/%s/%s/symbol=%s/", manMarket, manInterval, manSymbol)
		allKeys, err := r2.ListObjects(cmd.Context(), prefix)
		if err != nil {
			return err
		}
		keys := FilterKeysByRange(allKeys, fromTime, toTime)

		var objects []coverage.ObjectStats
		for _, key := range keys {
			localPath := filepath.Join(manPath, key)
			// Assuming files are already in cache from LoadOpenTimes
			stats, err := validate.ValidateParquet(cmd.Context(), localPath)
			if err != nil {
				return fmt.Errorf("failed to validate %s: %w", localPath, err)
			}

			// Parse period and source date from key
			base := filepath.Base(key)
			base = strings.TrimSuffix(base, ".parquet")
			parts := strings.Split(base, "-")
			datePart := strings.Join(parts[2:], "-")
			period := "daily"
			if len(datePart) == 7 {
				period = "monthly"
			}

			objects = append(objects, coverage.ObjectStats{
				Key:           key,
				Period:        period,
				SourceDate:    datePart,
				RowCount:      stats.RowCount,
				MinOpenTimeMS: stats.MinOpenTimeMS,
				MaxOpenTimeMS: stats.MaxOpenTimeMS,
			})
		}

		// 3. Create Manifest
		manifest := coverage.Manifest{
			SchemaVersion:      1,
			Market:             manMarket,
			Symbol:             manSymbol,
			Interval:           manInterval,
			CoverageStart:      fromTime,
			CoverageEnd:        toTime,
			ExpectedCandles:    report.ExpectedCandles,
			ActualCandles:      report.ActualCandles,
			UniqueOpenTimes:    report.UniqueOpenTimes,
			DuplicateOpenTimes: report.DuplicateOpenTimes,
			MissingCandles:     report.MissingCandles,
			ObjectCount:        len(objects),
			Objects:            objects,
			LastVerifiedAt:     time.Now(),
			Status:             report.Status,
		}

		manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal manifest: %w", err)
		}

		// 4. Upload to R2
		manifestKey := coverage.ManifestKey(manMarket, manInterval, manSymbol)

		tempFile, err := os.CreateTemp("", "ak-historian-manifest-*.json")
		if err != nil {
			return err
		}
		defer os.Remove(tempFile.Name())

		if _, err := tempFile.Write(manifestJSON); err != nil {
			return err
		}
		tempFile.Close()

		log.Printf("Uploading manifest to R2: %s", manifestKey)
		err = r2.UploadFile(cmd.Context(), tempFile.Name(), manifestKey)
		if err != nil {
			return err
		}

		fmt.Printf("Manifest written successfully to %s\n", manifestKey)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(manifestCmd)

	manifestCmd.Flags().StringVar(&manMarket, "market", "futures-um", "market type")
	manifestCmd.Flags().StringVar(&manSymbol, "symbol", "LINKUSDT", "symbol")
	manifestCmd.Flags().StringVar(&manInterval, "interval", "1m", "interval")
	manifestCmd.Flags().StringVar(&manFrom, "from", "", "start date (YYYY-MM-DD)")
	manifestCmd.Flags().StringVar(&manTo, "to", "", "end date (YYYY-MM-DD)")
	manifestCmd.Flags().StringVar(&manSource, "source", "r2", "data source")
	manifestCmd.Flags().StringVar(&manPath, "path", ".ak-historian/work", "cache path")
	manifestCmd.Flags().BoolVar(&manAllowFailed, "allow-failed", false, "allow writing manifest even if coverage fails")

	_ = manifestCmd.MarkFlagRequired("from")
	_ = manifestCmd.MarkFlagRequired("to")
}
