package app

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ak-historian",
	Short: "ak-historian is a bulk backfill tool for Binance historical data",
	Long:  `ak-historian downloads Binance Vision historical kline archives, converts them to Parquet, and uploads to Cloudflare R2.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Root flags if any
}
