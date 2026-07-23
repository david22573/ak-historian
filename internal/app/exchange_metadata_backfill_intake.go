package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/david22573/ak-historian/internal/exchange_meta"
	"github.com/spf13/cobra"
)

var (
	embiInputFiles               []string
	embiInputDir                 string
	embiArchiveRoot              string
	embiExchange                 string
	embiMarketType               string
	embiQuoteAsset               string
	embiSourceType               string
	embiSourceName               string
	embiSourceURI                string
	embiTrustLevel               string
	embiObservedTime             string
	embiObservedTimeFromFilename bool
	embiFilenameTimeLayout       string
	embiRefreshManifest          bool
	embiVerifyArchive            bool
	embiOut                      string
	embiDryRun                   bool
)

var exchangeMetadataBackfillIntakeCmd = &cobra.Command{
	Use:   "exchange-metadata-backfill-intake",
	Short: "Intake historical exchange metadata snapshots",
	RunE: func(cmd *cobra.Command, args []string) error {
		if embiArchiveRoot == "" {
			return fmt.Errorf("missing --archive-root")
		}

		opts := exchange_meta.IntakeOptions{
			InputFiles:               embiInputFiles,
			InputDir:                 embiInputDir,
			ArchiveRoot:              embiArchiveRoot,
			Exchange:                 embiExchange,
			MarketType:               embiMarketType,
			QuoteAssetFilter:         embiQuoteAsset,
			SourceType:               embiSourceType,
			SourceName:               embiSourceName,
			SourceURI:                embiSourceURI,
			TrustLevel:               embiTrustLevel,
			ObservedTime:             embiObservedTime,
			ObservedTimeFromFilename: embiObservedTimeFromFilename,
			FilenameTimeLayout:       embiFilenameTimeLayout,
			RefreshManifest:          embiRefreshManifest,
			VerifyArchive:            embiVerifyArchive,
			DryRun:                   embiDryRun,
		}

		report, err := exchange_meta.PerformBackfillIntake(opts)
		if err != nil {
			return err
		}

		if embiVerifyArchive && !embiDryRun {
			// verification logic
			vOpts := exchange_meta.VerifyOptions{
				ArchiveRoot: embiArchiveRoot,
				Exchange:    report.Exchange,
				MarketType:  report.MarketType,
			}
			vReport, err := exchange_meta.VerifyArchive(vOpts)
			if err != nil {
				return err
			}
			if !vReport.Valid {
				report.Warnings = append(report.Warnings, exchange_meta.Warning{Code: exchange_meta.CodeBackfillArchiveVerifyFailed, Message: "Archive verification failed post-intake"})
				report.Validation.WarningCodes = append(report.Validation.WarningCodes, exchange_meta.CodeBackfillArchiveVerifyFailed)
			}
		}

		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}

		if embiOut != "" {
			if err := os.MkdirAll(filepath.Dir(embiOut), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(embiOut, out, 0644); err != nil {
				return err
			}
			fmt.Printf("Wrote exchange metadata backfill intake report to %s\n", embiOut)
		} else {
			fmt.Println(string(out))
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(exchangeMetadataBackfillIntakeCmd)

	exchangeMetadataBackfillIntakeCmd.Flags().StringSliceVar(&embiInputFiles, "input-file", nil, "Input raw JSON file (repeatable)")
	exchangeMetadataBackfillIntakeCmd.Flags().StringVar(&embiInputDir, "input-dir", "", "Directory of input raw JSON files")
	exchangeMetadataBackfillIntakeCmd.Flags().StringVar(&embiArchiveRoot, "archive-root", "data/exchange_metadata", "Path to archive root")
	exchangeMetadataBackfillIntakeCmd.Flags().StringVar(&embiExchange, "exchange", "binance", "Exchange identifier")
	exchangeMetadataBackfillIntakeCmd.Flags().StringVar(&embiMarketType, "market-type", "futures_um", "Market type")
	exchangeMetadataBackfillIntakeCmd.Flags().StringVar(&embiQuoteAsset, "quote-asset", "USDT", "Optional quote asset filter")

	exchangeMetadataBackfillIntakeCmd.Flags().StringVar(&embiSourceType, "source-type", "file_import_historical", "Source type")
	exchangeMetadataBackfillIntakeCmd.Flags().StringVar(&embiSourceName, "source-name", "historical_backfill", "Source name")
	exchangeMetadataBackfillIntakeCmd.Flags().StringVar(&embiSourceURI, "source-uri", "", "Source URI or path prefix")
	exchangeMetadataBackfillIntakeCmd.Flags().StringVar(&embiTrustLevel, "trust-level", exchange_meta.TrustLevelUserProvidedUnverified, "Trust level")

	exchangeMetadataBackfillIntakeCmd.Flags().StringVar(&embiObservedTime, "observed-time", "", "Fallback observed time UTC")
	exchangeMetadataBackfillIntakeCmd.Flags().BoolVar(&embiObservedTimeFromFilename, "observed-time-from-filename", false, "Parse observed time from filename")
	exchangeMetadataBackfillIntakeCmd.Flags().StringVar(&embiFilenameTimeLayout, "filename-time-layout", "", "Time layout for filename parsing")

	exchangeMetadataBackfillIntakeCmd.Flags().BoolVar(&embiRefreshManifest, "refresh-manifest", true, "Refresh archive manifest")
	exchangeMetadataBackfillIntakeCmd.Flags().BoolVar(&embiVerifyArchive, "verify-archive", true, "Verify archive after intake")

	exchangeMetadataBackfillIntakeCmd.Flags().StringVar(&embiOut, "out", "", "Path to output intake report JSON")
	exchangeMetadataBackfillIntakeCmd.Flags().BoolVar(&embiDryRun, "dry-run", false, "Do not write any files")
}
