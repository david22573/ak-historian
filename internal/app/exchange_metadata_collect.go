package app

import (
	"encoding/json"
	"fmt"

	"github.com/david22573/ak-historian/internal/exchange_meta"
	"github.com/spf13/cobra"
)

var (
	emcExchange        string
	emcMarketType      string
	emcQuoteAsset      string
	emcArchiveRoot     string
	emcSource          string
	emcDryRun          bool
	emcRefreshManifest bool
	emcWriteRaw        bool
	emcAllowNetwork    bool
	emcRawJSON         string
)

var exchangeMetadataCollectCmd = &cobra.Command{
	Use:   "exchange-metadata-collect",
	Short: "Collect an exchange metadata snapshot into an archive",
	RunE: func(cmd *cobra.Command, args []string) error {
		if emcArchiveRoot == "" {
			return fmt.Errorf("missing --archive-root")
		}

		opts := exchange_meta.CollectOptions{
			Exchange:         emcExchange,
			MarketType:       emcMarketType,
			QuoteAssetFilter: emcQuoteAsset,
			ArchiveRoot:      emcArchiveRoot,
			SourceType:       emcSource,
			DryRun:           emcDryRun,
			RefreshManifest:  emcRefreshManifest,
			WriteRaw:         emcWriteRaw,
			AllowNetwork:     emcAllowNetwork,
			RawJSONPath:      emcRawJSON,
		}

		report, err := exchange_meta.CollectArchive(cmd.Context(), opts)
		if err != nil {
			return err
		}

		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(exchangeMetadataCollectCmd)

	exchangeMetadataCollectCmd.Flags().StringVar(&emcExchange, "exchange", "binance", "Exchange identifier")
	exchangeMetadataCollectCmd.Flags().StringVar(&emcMarketType, "market-type", "futures_um", "Market type")
	exchangeMetadataCollectCmd.Flags().StringVar(&emcQuoteAsset, "quote-asset", "USDT", "Optional quote asset filter")
	exchangeMetadataCollectCmd.Flags().StringVar(&emcArchiveRoot, "archive-root", "data/exchange_metadata", "Path to archive root")
	exchangeMetadataCollectCmd.Flags().StringVar(&emcSource, "source", "public_exchange_info", "Source type")
	exchangeMetadataCollectCmd.Flags().BoolVar(&emcDryRun, "dry-run", false, "Do not write files")
	exchangeMetadataCollectCmd.Flags().BoolVar(&emcRefreshManifest, "refresh-manifest", true, "Refresh archive manifest")
	exchangeMetadataCollectCmd.Flags().BoolVar(&emcWriteRaw, "write-raw", true, "Store raw payload")
	exchangeMetadataCollectCmd.Flags().BoolVar(&emcAllowNetwork, "allow-network", false, "Allow fetching from network")
	exchangeMetadataCollectCmd.Flags().StringVar(&emcRawJSON, "raw-json", "", "Optional fixture/import path")
}
