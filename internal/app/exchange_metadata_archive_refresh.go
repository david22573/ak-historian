package app

import (
	"fmt"
	"path/filepath"

	"github.com/david22573/ak-historian/internal/exchange_meta"
	"github.com/spf13/cobra"
)

var (
	emarArchiveRoot string
	emarExchange    string
	emarMarketType  string
	emarOut         string
)

var exchangeMetadataArchiveRefreshCmd = &cobra.Command{
	Use:   "exchange-metadata-archive-refresh",
	Short: "Refresh exchange_metadata_snapshot_manifest.json for an archive",
	RunE: func(cmd *cobra.Command, args []string) error {
		if emarArchiveRoot == "" {
			return fmt.Errorf("missing --archive-root")
		}

		baseDir := filepath.Join(emarArchiveRoot, emarExchange, emarMarketType)
		snapshotsDir := filepath.Join(baseDir, "snapshots")

		mOpts := exchange_meta.ManifestOptions{
			SnapshotDir: snapshotsDir,
			BaseDir:     filepath.Join(baseDir, "manifests"),
			ArchiveID:   "default_exchange_metadata_archive",
			Exchange:    emarExchange,
			MarketType:  emarMarketType,
		}

		manifest, err := exchange_meta.BuildSnapshotManifest(mOpts)
		if err != nil {
			return err
		}

		outPath := emarOut
		if outPath == "" {
			outPath = filepath.Join(baseDir, "manifests", "exchange_metadata_snapshot_manifest.json")
		}

		if err := exchange_meta.WriteSnapshotManifest(outPath, manifest); err != nil {
			return err
		}
		fmt.Printf("Wrote refreshed exchange metadata archive manifest to %s\n", outPath)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(exchangeMetadataArchiveRefreshCmd)

	exchangeMetadataArchiveRefreshCmd.Flags().StringVar(&emarArchiveRoot, "archive-root", "data/exchange_metadata", "Path to archive root")
	exchangeMetadataArchiveRefreshCmd.Flags().StringVar(&emarExchange, "exchange", "binance", "Exchange identifier")
	exchangeMetadataArchiveRefreshCmd.Flags().StringVar(&emarMarketType, "market-type", "futures_um", "Market type")
	exchangeMetadataArchiveRefreshCmd.Flags().StringVar(&emarOut, "out", "", "Optional custom output path")
}
