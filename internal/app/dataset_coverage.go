package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/david22573/ak-historian/internal/coverage"
	"github.com/spf13/cobra"
)

var (
	dcDataRoot string
	dcOut      string
	dcSymbols  []string
	dcInterval string
	dcStart    string
	dcEnd      string
	dcMode     string
	dcMinPct   float64
)

var datasetCoverageCmd = &cobra.Command{
	Use:   "dataset-coverage",
	Short: "Audit parquet dataset coverage",
	RunE: func(cmd *cobra.Command, args []string) error {
		var startT, endT time.Time
		if dcStart != "" {
			var err error
			startT, err = time.Parse(time.RFC3339, dcStart)
			if err != nil {
				return fmt.Errorf("invalid start time: %w", err)
			}
		}
		if dcEnd != "" {
			var err error
			endT, err = time.Parse(time.RFC3339, dcEnd)
			if err != nil {
				return fmt.Errorf("invalid end time: %w", err)
			}
		}

		opts := coverage.AuditorOptions{
			Mode:         dcMode,
			Start:        startT,
			End:          endT,
			MinPct:       dcMinPct,
			IntervalHint: dcInterval,
		}

		if len(dcSymbols) == 1 {
			opts.SymbolHint = dcSymbols[0]
		}

		var files []string
		err := filepath.Walk(dcDataRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && filepath.Ext(path) == ".parquet" {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to scan data root: %w", err)
		}

		res, err := coverage.AuditFiles(files, opts)
		if err != nil {
			return err
		}

		outBytes, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			return err
		}

		if dcOut != "" {
			if err := os.WriteFile(dcOut, outBytes, 0644); err != nil {
				return err
			}
		} else {
			fmt.Println(string(outBytes))
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(datasetCoverageCmd)
	datasetCoverageCmd.Flags().StringVar(&dcDataRoot, "data-root", "", "Root directory to scan for parquet files")
	datasetCoverageCmd.Flags().StringVar(&dcOut, "out", "", "Output JSON file path")
	datasetCoverageCmd.Flags().StringSliceVar(&dcSymbols, "symbols", nil, "Comma separated list of symbols")
	datasetCoverageCmd.Flags().StringVar(&dcInterval, "interval", "", "Interval (e.g. 1m, 1h)")
	datasetCoverageCmd.Flags().StringVar(&dcStart, "start", "", "Start time RFC3339")
	datasetCoverageCmd.Flags().StringVar(&dcEnd, "end", "", "End time RFC3339")
	datasetCoverageCmd.Flags().StringVar(&dcMode, "mode", "fast", "Coverage mode (fast|strict)")
	datasetCoverageCmd.Flags().Float64Var(&dcMinPct, "min-pct", 99.0, "Minimum coverage percentage")

	datasetCoverageCmd.MarkFlagRequired("data-root")
}
