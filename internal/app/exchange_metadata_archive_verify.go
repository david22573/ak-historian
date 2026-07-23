package app

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/david22573/ak-historian/internal/exchange_meta"
	"github.com/spf13/cobra"
)

var (
	emavArchiveRoot string
	emavExchange    string
	emavMarketType  string
	emavStrict      bool
)

var exchangeMetadataArchiveVerifyCmd = &cobra.Command{
	Use:   "exchange-metadata-archive-verify",
	Short: "Verify exchange metadata archive integrity",
	RunE: func(cmd *cobra.Command, args []string) error {
		if emavArchiveRoot == "" {
			return fmt.Errorf("missing --archive-root")
		}

		opts := exchange_meta.VerifyOptions{
			ArchiveRoot: emavArchiveRoot,
			Exchange:    emavExchange,
			MarketType:  emavMarketType,
			Strict:      emavStrict,
		}

		report, err := exchange_meta.VerifyArchive(opts)
		if err != nil {
			return err
		}

		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))

		if !report.Valid {
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(exchangeMetadataArchiveVerifyCmd)

	exchangeMetadataArchiveVerifyCmd.Flags().StringVar(&emavArchiveRoot, "archive-root", "data/exchange_metadata", "Path to archive root")
	exchangeMetadataArchiveVerifyCmd.Flags().StringVar(&emavExchange, "exchange", "binance", "Exchange identifier")
	exchangeMetadataArchiveVerifyCmd.Flags().StringVar(&emavMarketType, "market-type", "futures_um", "Market type")
	exchangeMetadataArchiveVerifyCmd.Flags().BoolVar(&emavStrict, "strict", false, "Strict verification")
}
