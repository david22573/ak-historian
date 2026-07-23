package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/david22573/ak-historian/internal/exchange_meta"
	"github.com/spf13/cobra"
)

var (
	emsExchange           string
	emsMarketType         string
	emsQuoteAsset         string
	emsRawJSON            string
	emsOut                string
	emsCollectedAt        string
	emsSourceObservedTime string
	emsSourceName         string
	emsSourceURI          string
	emsBaseURL            string

	emsManifestSnapshotDir string
	emsManifestOut         string
	emsManifestArchiveID   string
	emsManifestExchange    string
	emsManifestMarketType  string
)

var exchangeMetadataSnapshotCmd = &cobra.Command{
	Use:   "exchange-metadata-snapshot",
	Short: "Generate an exchange_metadata_snapshot.json",
	RunE: func(cmd *cobra.Command, args []string) error {
		if emsOut == "" {
			return fmt.Errorf("missing --out")
		}
		gitSHA := "unknown"
		if out, err := getGitSHA(); err == nil {
			gitSHA = out
		}

		var raw []byte
		sourceType := "public_endpoint_current"
		sourceName := emsSourceName
		sourceURI := emsSourceURI
		if emsRawJSON != "" {
			data, err := os.ReadFile(emsRawJSON)
			if err != nil {
				return fmt.Errorf("read raw exchange metadata JSON: %w", err)
			}
			raw = data
			sourceType = "file_import_current"
			if sourceName == "" {
				sourceName = "binance_futures_exchangeInfo_v1"
			}
			if sourceURI == "" {
				sourceURI = filepath.ToSlash(emsRawJSON)
			}
		} else {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			data, uri, err := exchange_meta.FetchBinanceFuturesExchangeInfo(ctx, nil, emsBaseURL)
			if err != nil {
				return err
			}
			raw = data
			if sourceName == "" {
				sourceName = "binance_futures_exchangeInfo_v1"
			}
			if sourceURI == "" {
				sourceURI = uri
			}
		}

		snapshot, err := exchange_meta.BuildSnapshot(exchange_meta.SnapshotOptions{
			Exchange:              emsExchange,
			MarketType:            emsMarketType,
			QuoteAssetFilter:      emsQuoteAsset,
			SourceType:            sourceType,
			SourceName:            sourceName,
			SourceURI:             sourceURI,
			CollectedAtUTC:        emsCollectedAt,
			SourceObservedTimeUTC: emsSourceObservedTime,
			CollectorGitSHA:       gitSHA,
			RawPayload:            raw,
		})
		if err != nil {
			return err
		}
		if err := exchange_meta.WriteSnapshot(emsOut, snapshot); err != nil {
			return err
		}
		fmt.Printf("Wrote exchange metadata snapshot to %s\n", emsOut)
		return nil
	},
}

var exchangeMetadataSnapshotManifestCmd = &cobra.Command{
	Use:   "exchange-metadata-snapshot-manifest",
	Short: "Generate an exchange_metadata_snapshot_manifest.json",
	RunE: func(cmd *cobra.Command, args []string) error {
		if emsManifestSnapshotDir == "" {
			return fmt.Errorf("missing --snapshot-dir")
		}
		if emsManifestOut == "" {
			return fmt.Errorf("missing --out")
		}
		manifest, err := exchange_meta.BuildSnapshotManifest(exchange_meta.ManifestOptions{
			SnapshotDir: emsManifestSnapshotDir,
			ArchiveID:   emsManifestArchiveID,
			Exchange:    emsManifestExchange,
			MarketType:  emsManifestMarketType,
		})
		if err != nil {
			return err
		}
		if err := exchange_meta.WriteSnapshotManifest(emsManifestOut, manifest); err != nil {
			return err
		}
		fmt.Printf("Wrote exchange metadata snapshot manifest to %s\n", emsManifestOut)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(exchangeMetadataSnapshotCmd)
	rootCmd.AddCommand(exchangeMetadataSnapshotManifestCmd)

	exchangeMetadataSnapshotCmd.Flags().StringVar(&emsExchange, "exchange", "binance", "Exchange identifier")
	exchangeMetadataSnapshotCmd.Flags().StringVar(&emsMarketType, "market-type", "futures_um", "Market type")
	exchangeMetadataSnapshotCmd.Flags().StringVar(&emsQuoteAsset, "quote-asset", "", "Optional quote asset filter")
	exchangeMetadataSnapshotCmd.Flags().StringVar(&emsRawJSON, "raw-json", "", "Path to raw exchangeInfo JSON to import")
	exchangeMetadataSnapshotCmd.Flags().StringVar(&emsOut, "out", "", "Path to write exchange_metadata_snapshot.json")
	exchangeMetadataSnapshotCmd.Flags().StringVar(&emsCollectedAt, "collected-at", "", "Collection time UTC (RFC3339)")
	exchangeMetadataSnapshotCmd.Flags().StringVar(&emsSourceObservedTime, "source-observed-time", "", "Source-observed time UTC (RFC3339)")
	exchangeMetadataSnapshotCmd.Flags().StringVar(&emsSourceName, "source-name", "", "Source name")
	exchangeMetadataSnapshotCmd.Flags().StringVar(&emsSourceURI, "source-uri", "", "Source URI or archive path")
	exchangeMetadataSnapshotCmd.Flags().StringVar(&emsBaseURL, "base-url", "", "Public metadata API base URL")

	exchangeMetadataSnapshotManifestCmd.Flags().StringVar(&emsManifestSnapshotDir, "snapshot-dir", "", "Directory containing exchange metadata snapshots")
	exchangeMetadataSnapshotManifestCmd.Flags().StringVar(&emsManifestOut, "out", "", "Path to write exchange_metadata_snapshot_manifest.json")
	exchangeMetadataSnapshotManifestCmd.Flags().StringVar(&emsManifestArchiveID, "archive-id", "default_exchange_metadata_archive", "Archive ID")
	exchangeMetadataSnapshotManifestCmd.Flags().StringVar(&emsManifestExchange, "exchange", "binance", "Exchange identifier")
	exchangeMetadataSnapshotManifestCmd.Flags().StringVar(&emsManifestMarketType, "market-type", "futures_um", "Market type")
}
