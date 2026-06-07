package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/davidmiguel22573/ak-historian/internal/datasets"
	"github.com/davidmiguel22573/ak-historian/internal/datasets/derivatives"
	"github.com/spf13/cobra"
)

var (
	vdKind     string
	vdSource   string
	vdDataset  string
	vdMarket   string
	vdSymbol   string
	vdScope    string
	vdInterval string
	vdFrom     string
	vdTo       string
	vdPath     string
)

func init() {
	rootCmd.AddCommand(verifyDatasetCmd)

	verifyDatasetCmd.Flags().StringVar(&vdKind, "kind", "", "sentiment or derivatives")
	verifyDatasetCmd.Flags().StringVar(&vdSource, "source", "", "required")
	verifyDatasetCmd.Flags().StringVar(&vdDataset, "dataset", "", "required")
	verifyDatasetCmd.Flags().StringVar(&vdMarket, "market", "", "required for derivatives")
	verifyDatasetCmd.Flags().StringVar(&vdSymbol, "symbol", "", "required for derivatives")
	verifyDatasetCmd.Flags().StringVar(&vdScope, "scope", "", "optional")
	verifyDatasetCmd.Flags().StringVar(&vdInterval, "interval", "", "optional; defaults by dataset for derivatives")
	verifyDatasetCmd.Flags().StringVar(&vdFrom, "from", "", "required YYYY-MM-DD")
	verifyDatasetCmd.Flags().StringVar(&vdTo, "to", "", "required YYYY-MM-DD")
	verifyDatasetCmd.Flags().StringVar(&vdPath, "path", "", "local path")

	verifyDatasetCmd.MarkFlagRequired("kind")
	verifyDatasetCmd.MarkFlagRequired("source")
	verifyDatasetCmd.MarkFlagRequired("dataset")
	verifyDatasetCmd.MarkFlagRequired("from")
	verifyDatasetCmd.MarkFlagRequired("to")
	verifyDatasetCmd.MarkFlagRequired("path")
}

var verifyDatasetCmd = &cobra.Command{
	Use:   "verify-dataset",
	Short: "Verify dataset parquet files locally",
	Run: func(cmd *cobra.Command, args []string) {
		fromT, err := time.Parse("2006-01-02", vdFrom)
		if err != nil {
			fmt.Printf("{\"status\":\"FAIL\",\"error\":\"invalid from: %v\"}\n", err)
			os.Exit(1)
		}

		toT, err := time.Parse("2006-01-02", vdTo)
		if err != nil {
			fmt.Printf("{\"status\":\"FAIL\",\"error\":\"invalid to: %v\"}\n", err)
			os.Exit(1)
		}

		interval := vdInterval
		if interval == "" && vdKind == string(datasets.KindDerivatives) {
			interval = derivativesDefaultInterval(vdDataset)
		}
		if interval == "" {
			fmt.Printf("{\"status\":\"FAIL\",\"error\":\"missing --interval\"}\n")
			os.Exit(1)
		}

		spec := datasets.DatasetSpec{
			Kind:     datasets.DatasetKind(vdKind),
			Source:   vdSource,
			Dataset:  vdDataset,
			Market:   vdMarket,
			Symbol:   strings.ToUpper(vdSymbol),
			Scope:    vdScope,
			Interval: interval,
			Date:     "9999-99",
		}

		keyGlob, err := datasets.ObjectKey(spec)
		if err != nil {
			fmt.Printf("{\"status\":\"FAIL\",\"error\":\"path build failed: %v\"}\n", err)
			os.Exit(1)
		}

		keyGlob = strings.ReplaceAll(keyGlob, "year=9999/month=99", "year=*/month=*")
		keyGlob = strings.ReplaceAll(keyGlob, "9999-99", "*")

		fullGlob := filepath.Join(vdPath, keyGlob)
		files, err := filepath.Glob(fullGlob)
		if err != nil {
			fmt.Printf("{\"status\":\"FAIL\",\"error\":\"glob failed: %v\"}\n", err)
			os.Exit(1)
		}

		if len(files) == 0 {
			fmt.Printf("{\"status\":\"FAIL\",\"error\":\"no files matched glob %s\"}\n", fullGlob)
			os.Exit(1)
		}

		duckGlob := fullGlob
		query := fmt.Sprintf(`
WITH data AS (
    SELECT 
        event_time_ms,
        available_at_ms,
        LAG(event_time_ms) OVER (ORDER BY event_time_ms) as prev_event_time_ms
    FROM read_parquet('%s')
)
SELECT
    COUNT(*) as total_rows,
    SUM(CASE WHEN prev_event_time_ms IS NOT NULL AND event_time_ms <= prev_event_time_ms THEN 1 ELSE 0 END) as sort_or_dup_errors,
    SUM(CASE WHEN available_at_ms < event_time_ms THEN 1 ELSE 0 END) as avail_errors,
    SUM(CASE WHEN event_time_ms >= %d AND event_time_ms <= %d THEN 1 ELSE 0 END) as rows_in_range
FROM data;
`, duckGlob, fromT.UnixMilli(), toT.UnixMilli()+86400000)

		statsCmd := executeDuckDB(context.Background(), query)
		if statsCmd.Err != nil {
			fmt.Printf("{\"status\":\"FAIL\",\"error\":\"duckdb verify failed: %v\"}\n", statsCmd.Err)
			os.Exit(1)
		}

		if statsCmd.TotalRows == 0 {
			fmt.Printf("{\"status\":\"FAIL\",\"error\":\"empty dataset\"}\n")
			os.Exit(1)
		}
		if statsCmd.SortOrDupErrors > 0 {
			fmt.Printf("{\"status\":\"FAIL\",\"error\":\"dataset has duplicates or is out of order\"}\n")
			os.Exit(1)
		}
		if statsCmd.AvailErrors > 0 {
			fmt.Printf("{\"status\":\"FAIL\",\"error\":\"available_at_ms < event_time_ms found\"}\n")
			os.Exit(1)
		}
		if statsCmd.RowsInRange == 0 {
			fmt.Printf("{\"status\":\"FAIL\",\"error\":\"no rows in requested range\"}\n")
			os.Exit(1)
		}

		res := map[string]interface{}{
			"status": "PASS",
			"report": map[string]interface{}{
				"files":         files,
				"total_rows":    statsCmd.TotalRows,
				"rows_in_range": statsCmd.RowsInRange,
			},
		}
		b, _ := json.Marshal(res)
		fmt.Println(string(b))
	},
}

type verifyStats struct {
	TotalRows       int64
	SortOrDupErrors int64
	AvailErrors     int64
	RowsInRange     int64
	Err             error
}

func executeDuckDB(ctx context.Context, query string) verifyStats {
	cmd := exec.CommandContext(ctx, "duckdb", "-csv", "-noheader", "-c", query)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return verifyStats{Err: fmt.Errorf("%s: %w", string(out), err)}
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 {
		return verifyStats{Err: fmt.Errorf("no output")}
	}
	fields := strings.Split(lines[0], ",")
	if len(fields) != 4 {
		return verifyStats{Err: fmt.Errorf("expected 4 fields, got %d", len(fields))}
	}

	var stats verifyStats
	stats.TotalRows, _ = strconv.ParseInt(fields[0], 10, 64)
	if fields[1] != "" {
		stats.SortOrDupErrors, _ = strconv.ParseInt(fields[1], 10, 64)
	}
	if fields[2] != "" {
		stats.AvailErrors, _ = strconv.ParseInt(fields[2], 10, 64)
	}
	if fields[3] != "" {
		stats.RowsInRange, _ = strconv.ParseInt(fields[3], 10, 64)
	}

	return stats
}

func derivativesDefaultInterval(dataset string) string {
	return derivatives.DefaultInterval(dataset)
}
