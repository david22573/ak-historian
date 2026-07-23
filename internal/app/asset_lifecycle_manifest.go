package app

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/david22573/ak-historian/internal/lifecycle"
	"github.com/spf13/cobra"
)

var (
	almOut                      string
	almLifecycleID              string
	almLifecycleName            string
	almExchange                 string
	almMarketType               string
	almQuoteAsset               string
	almEffectiveStart           string
	almEffectiveEnd             string
	almDataRoot                 string
	almInputCSV                 string
	almInputJSON                string
	almExchangeSnapshot         string
	almExchangeSnapshotManifest string
	almSourceType               string
	almStrict                   bool
)

var assetLifecycleManifestCmd = &cobra.Command{
	Use:   "asset-lifecycle-manifest",
	Short: "Generate an asset_lifecycle_manifest.json",
	RunE: func(cmd *cobra.Command, args []string) error {
		gitSHA := "unknown"
		if out, err := getGitSHA(); err == nil {
			gitSHA = out
		}

		builder := &lifecycle.Builder{
			LifecycleID:                  almLifecycleID,
			LifecycleName:                almLifecycleName,
			SourceRepo:                   "ak-historian",
			SourceGitSHA:                 gitSHA,
			SourceType:                   almSourceType,
			EffectiveStartUTC:            almEffectiveStart,
			EffectiveEndUTC:              almEffectiveEnd,
			Exchange:                     almExchange,
			MarketType:                   almMarketType,
			QuoteAsset:                   almQuoteAsset,
			DataRoot:                     almDataRoot,
			InputCSV:                     almInputCSV,
			InputJSON:                    almInputJSON,
			ExchangeSnapshotPath:         almExchangeSnapshot,
			ExchangeSnapshotManifestPath: almExchangeSnapshotManifest,
			Strict:                       almStrict,
		}

		manifest, err := builder.Build()
		if err != nil && manifest == nil {
			return err
		}

		outBytes, marshalErr := json.MarshalIndent(manifest, "", "  ")
		if marshalErr != nil {
			return marshalErr
		}
		if almOut != "" {
			if writeErr := os.WriteFile(almOut, outBytes, 0644); writeErr != nil {
				return writeErr
			}
			fmt.Printf("Wrote asset lifecycle manifest to %s\n", almOut)
		} else {
			fmt.Println(string(outBytes))
		}
		return err
	},
}

func init() {
	rootCmd.AddCommand(assetLifecycleManifestCmd)

	assetLifecycleManifestCmd.Flags().StringVar(&almOut, "out", "", "Path to write asset_lifecycle_manifest.json")
	assetLifecycleManifestCmd.Flags().StringVar(&almLifecycleID, "lifecycle-id", "default_lifecycle", "Lifecycle ID")
	assetLifecycleManifestCmd.Flags().StringVar(&almLifecycleName, "lifecycle-name", "Default Asset Lifecycle", "Lifecycle name")
	assetLifecycleManifestCmd.Flags().StringVar(&almExchange, "exchange", "binance", "Exchange identifier")
	assetLifecycleManifestCmd.Flags().StringVar(&almMarketType, "market-type", "futures", "Market type")
	assetLifecycleManifestCmd.Flags().StringVar(&almQuoteAsset, "quote-asset", "USDT", "Quote asset")
	assetLifecycleManifestCmd.Flags().StringVar(&almEffectiveStart, "effective-start", "", "Effective start UTC (RFC3339)")
	assetLifecycleManifestCmd.Flags().StringVar(&almEffectiveEnd, "effective-end", "", "Effective end UTC (RFC3339)")
	assetLifecycleManifestCmd.Flags().StringVar(&almDataRoot, "data-root", "", "Path to local data root")
	assetLifecycleManifestCmd.Flags().StringVar(&almInputCSV, "input-csv", "", "Path to user-provided lifecycle CSV")
	assetLifecycleManifestCmd.Flags().StringVar(&almInputJSON, "input-json", "", "Path to user-provided lifecycle JSON")
	assetLifecycleManifestCmd.Flags().StringVar(&almExchangeSnapshot, "exchange-snapshot", "", "Path to exchange_metadata_snapshot.json")
	assetLifecycleManifestCmd.Flags().StringVar(&almExchangeSnapshotManifest, "exchange-snapshot-manifest", "", "Path to exchange_metadata_snapshot_manifest.json")
	assetLifecycleManifestCmd.Flags().StringVar(&almSourceType, "source-type", "local_data", "Lifecycle source type")
	assetLifecycleManifestCmd.Flags().BoolVar(&almStrict, "strict", false, "Fail when validation status is not valid")
}
