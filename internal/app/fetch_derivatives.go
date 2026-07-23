package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/david22573/ak-historian/internal/datasets"
	"github.com/david22573/ak-historian/internal/datasets/derivatives"
	"github.com/spf13/cobra"
)

var (
	fdSource        string
	fdDataset       string
	fdMarket        string
	fdSymbols       string
	fdInterval      string
	fdStart         string
	fdEnd           string
	fdOut           string
	fdFormat        string
	fdWriteManifest bool
)

func init() {
	rootCmd.AddCommand(fetchDerivativesCmd)

	fetchDerivativesCmd.Flags().StringVar(&fdSource, "source", "", "required, only binance")
	fetchDerivativesCmd.Flags().StringVar(&fdDataset, "dataset", "", "required derivatives dataset")
	fetchDerivativesCmd.Flags().StringVar(&fdMarket, "market", "futures-um", "market type")
	fetchDerivativesCmd.Flags().StringVar(&fdSymbols, "symbols", "", "comma-separated symbols")
	fetchDerivativesCmd.Flags().StringVar(&fdInterval, "interval", "", "dataset interval/period; default 8h for funding_rate, 5m otherwise")
	fetchDerivativesCmd.Flags().StringVar(&fdStart, "start", "", "required YYYY-MM-DD")
	fetchDerivativesCmd.Flags().StringVar(&fdEnd, "end", "", "required YYYY-MM-DD")
	fetchDerivativesCmd.Flags().StringVar(&fdOut, "out", ".ak-historian/work", "output dir")
	fetchDerivativesCmd.Flags().StringVar(&fdFormat, "format", "parquet", "json | csv | parquet")
	fetchDerivativesCmd.Flags().BoolVar(&fdWriteManifest, "write-manifest", false, "write manifest")

	fetchDerivativesCmd.MarkFlagRequired("source")
	fetchDerivativesCmd.MarkFlagRequired("dataset")
	fetchDerivativesCmd.MarkFlagRequired("symbols")
	fetchDerivativesCmd.MarkFlagRequired("start")
	fetchDerivativesCmd.MarkFlagRequired("end")
}

var fetchDerivativesCmd = &cobra.Command{
	Use:   "fetch-derivatives",
	Short: "Fetch Binance futures derivatives research datasets",
	Run: func(cmd *cobra.Command, args []string) {
		result, err := runFetchDerivatives(cmd.Context(), FetchDerivativesOptions{
			Source:        fdSource,
			Dataset:       fdDataset,
			Market:        fdMarket,
			Symbols:       parseDerivativesSymbols(fdSymbols),
			Interval:      fdInterval,
			Start:         fdStart,
			End:           fdEnd,
			Out:           fdOut,
			Format:        fdFormat,
			WriteManifest: fdWriteManifest,
			Client:        derivatives.NewBinanceClient(),
		})
		if result != nil {
			b, _ := json.Marshal(result)
			fmt.Println(string(b))
		}
		if err != nil {
			os.Exit(1)
		}
	},
}

type FetchDerivativesOptions struct {
	Source        string
	Dataset       string
	Market        string
	Symbols       []string
	Interval      string
	Start         string
	End           string
	Out           string
	Format        string
	WriteManifest bool
	Client        DerivativesFetcher
}

type DerivativesFetcher interface {
	Fetch(ctx context.Context, req derivatives.FetchRequest) ([]derivatives.Row, error)
}

type FetchDerivativesResult struct {
	Status    string                   `json:"status"`
	Reason    string                   `json:"reason,omitempty"`
	Source    string                   `json:"source"`
	Dataset   string                   `json:"dataset"`
	Market    string                   `json:"market"`
	Interval  string                   `json:"interval"`
	Rows      int                      `json:"rows,omitempty"`
	Objects   int                      `json:"objects,omitempty"`
	Manifests []string                 `json:"manifests,omitempty"`
	Symbols   []FetchDerivativesSymbol `json:"symbols,omitempty"`
}

type FetchDerivativesSymbol struct {
	Symbol   string `json:"symbol"`
	Rows     int    `json:"rows"`
	Objects  int    `json:"objects"`
	Manifest string `json:"manifest,omitempty"`
}

func runFetchDerivatives(ctx context.Context, opts FetchDerivativesOptions) (*FetchDerivativesResult, error) {
	if opts.Source != "binance" {
		return failDerivativesResult(opts, "", fmt.Errorf("unsupported source: %s", opts.Source))
	}
	if !derivatives.IsSupportedDataset(opts.Dataset) {
		return failDerivativesResult(opts, "", fmt.Errorf("unsupported dataset: %s", opts.Dataset))
	}
	if opts.Market != "futures-um" {
		return failDerivativesResult(opts, "", fmt.Errorf("unsupported market: %s", opts.Market))
	}
	if len(opts.Symbols) == 0 {
		return failDerivativesResult(opts, "", fmt.Errorf("at least one symbol required"))
	}
	if opts.Interval == "" {
		opts.Interval = derivatives.DefaultInterval(opts.Dataset)
	}
	if opts.Out == "" {
		return failDerivativesResult(opts, "", fmt.Errorf("out cannot be empty"))
	}
	if opts.Client == nil {
		opts.Client = derivatives.NewBinanceClient()
	}

	startT, err := time.Parse("2006-01-02", opts.Start)
	if err != nil {
		return failDerivativesResult(opts, "", fmt.Errorf("invalid start: %w", err))
	}
	endT, err := time.Parse("2006-01-02", opts.End)
	if err != nil {
		return failDerivativesResult(opts, "", fmt.Errorf("invalid end: %w", err))
	}
	endT = endT.Add(24*time.Hour - time.Millisecond)
	if endT.Before(startT) {
		return failDerivativesResult(opts, "", fmt.Errorf("end before start"))
	}

	result := &FetchDerivativesResult{
		Status:   "PASS",
		Source:   opts.Source,
		Dataset:  opts.Dataset,
		Market:   opts.Market,
		Interval: opts.Interval,
	}
	for _, symbol := range opts.Symbols {
		rows, err := opts.Client.Fetch(ctx, derivatives.FetchRequest{
			Source:   opts.Source,
			Dataset:  opts.Dataset,
			Market:   opts.Market,
			Symbol:   symbol,
			Interval: opts.Interval,
			Start:    startT,
			End:      endT,
		})
		var limited derivatives.LimitedHistoryError
		if errors.As(err, &limited) {
			return &FetchDerivativesResult{
				Status:   "limited_history",
				Reason:   limited.Reason,
				Source:   opts.Source,
				Dataset:  opts.Dataset,
				Market:   opts.Market,
				Interval: opts.Interval,
			}, nil
		}
		if err != nil {
			return failDerivativesResult(opts, symbol, fmt.Errorf("fetch failed: %w", err))
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].EventTimeMS < rows[j].EventTimeMS })
		if err := datasets.ValidateDerivativesRows(rows); err != nil {
			return failDerivativesResult(opts, symbol, fmt.Errorf("validation failed: %w", err))
		}
		symbolResult, err := writeDerivativesSymbol(ctx, opts, symbol, rows)
		if err != nil {
			return failDerivativesResult(opts, symbol, err)
		}
		result.Rows += symbolResult.Rows
		result.Objects += symbolResult.Objects
		if symbolResult.Manifest != "" {
			result.Manifests = append(result.Manifests, symbolResult.Manifest)
		}
		result.Symbols = append(result.Symbols, symbolResult)
	}
	return result, nil
}

func failDerivativesResult(opts FetchDerivativesOptions, symbol string, err error) (*FetchDerivativesResult, error) {
	res := &FetchDerivativesResult{
		Status:   "FAIL",
		Reason:   err.Error(),
		Source:   opts.Source,
		Dataset:  opts.Dataset,
		Market:   opts.Market,
		Interval: opts.Interval,
	}
	if symbol != "" {
		res.Symbols = []FetchDerivativesSymbol{{Symbol: symbol}}
	}
	return res, err
}

func writeDerivativesSymbol(ctx context.Context, opts FetchDerivativesOptions, symbol string, rows []derivatives.Row) (FetchDerivativesSymbol, error) {
	partitions := make(map[string][]derivatives.Row)
	for _, r := range rows {
		period := time.UnixMilli(r.EventTimeMS).UTC().Format("2006-01")
		partitions[period] = append(partitions[period], r)
	}

	var periods []string
	for period := range partitions {
		periods = append(periods, period)
	}
	sort.Strings(periods)

	var manifestObjs []datasets.Object
	for _, period := range periods {
		pRows := partitions[period]
		spec := datasets.DatasetSpec{
			Kind:     datasets.KindDerivatives,
			Source:   opts.Source,
			Dataset:  opts.Dataset,
			Market:   opts.Market,
			Symbol:   symbol,
			Interval: opts.Interval,
			Date:     period,
		}
		key, err := datasets.ObjectKey(spec)
		if err != nil {
			return FetchDerivativesSymbol{}, fmt.Errorf("path build failed: %w", err)
		}
		if opts.Format != "parquet" {
			key = strings.TrimSuffix(key, ".parquet") + "." + opts.Format
		}
		outPath := filepath.Join(opts.Out, key)
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return FetchDerivativesSymbol{}, fmt.Errorf("mkdir failed: %w", err)
		}

		var stats datasets.RowStats
		switch opts.Format {
		case "json":
			if err := datasets.WriteDerivativesRowsJSON(outPath, pRows); err != nil {
				return FetchDerivativesSymbol{}, fmt.Errorf("write json failed: %w", err)
			}
			stats = computeDerivativesLocalStats(pRows)
		case "csv":
			if err := datasets.WriteDerivativesRowsCSV(outPath, pRows); err != nil {
				return FetchDerivativesSymbol{}, fmt.Errorf("write csv failed: %w", err)
			}
			stats = computeDerivativesLocalStats(pRows)
		case "parquet":
			csvPath := outPath + ".csv"
			if err := datasets.WriteDerivativesRowsCSV(csvPath, pRows); err != nil {
				return FetchDerivativesSymbol{}, fmt.Errorf("write temp csv failed: %w", err)
			}
			if err := datasets.WriteDerivativesRowsParquet(csvPath, outPath); err != nil {
				return FetchDerivativesSymbol{}, fmt.Errorf("write parquet failed: %w", err)
			}
			os.Remove(csvPath)
			var err error
			stats, err = datasets.ValidateDatasetParquet(ctx, outPath)
			if err != nil {
				return FetchDerivativesSymbol{}, fmt.Errorf("parquet validation failed: %w", err)
			}
		default:
			return FetchDerivativesSymbol{}, fmt.Errorf("unsupported format: %s", opts.Format)
		}
		manifestObjs = append(manifestObjs, datasets.Object{
			Key:              key,
			Period:           spec.Date,
			RowCount:         stats.RowCount,
			MinEventTimeMS:   stats.MinEventTimeMS,
			MaxEventTimeMS:   stats.MaxEventTimeMS,
			MinAvailableAtMS: stats.MinAvailableAtMS,
			MaxAvailableAtMS: stats.MaxAvailableAtMS,
		})
	}

	symbolResult := FetchDerivativesSymbol{Symbol: symbol, Rows: len(rows), Objects: len(manifestObjs)}
	if opts.WriteManifest {
		spec := datasets.DatasetSpec{
			Kind:     datasets.KindDerivatives,
			Source:   opts.Source,
			Dataset:  opts.Dataset,
			Market:   opts.Market,
			Symbol:   symbol,
			Interval: opts.Interval,
		}
		key, err := datasets.ManifestKey(spec)
		if err != nil {
			return FetchDerivativesSymbol{}, fmt.Errorf("manifest key failed: %w", err)
		}
		manifestPath := filepath.Join(opts.Out, key)
		var minEvent, maxEvent int64
		for i, o := range manifestObjs {
			if i == 0 || o.MinEventTimeMS < minEvent {
				minEvent = o.MinEventTimeMS
			}
			if i == 0 || o.MaxEventTimeMS > maxEvent {
				maxEvent = o.MaxEventTimeMS
			}
		}
		m := datasets.Manifest{
			SchemaVersion:    1,
			Kind:             string(datasets.KindDerivatives),
			Source:           opts.Source,
			Dataset:          opts.Dataset,
			Market:           opts.Market,
			Symbol:           symbol,
			Interval:         opts.Interval,
			CoverageStartMS:  minEvent,
			CoverageEndMS:    maxEvent,
			ObjectCount:      len(manifestObjs),
			Objects:          manifestObjs,
			LastVerifiedAtMS: time.Now().UTC().UnixMilli(),
		}
		if err := datasets.WriteManifest(manifestPath, m); err != nil {
			return FetchDerivativesSymbol{}, fmt.Errorf("write manifest failed: %w", err)
		}
		symbolResult.Manifest = manifestPath
	}
	return symbolResult, nil
}

func parseDerivativesSymbols(csv string) []string {
	var symbols []string
	seen := make(map[string]bool)
	for _, raw := range strings.Split(csv, ",") {
		symbol := strings.ToUpper(strings.TrimSpace(raw))
		if symbol == "" || seen[symbol] {
			continue
		}
		seen[symbol] = true
		symbols = append(symbols, symbol)
	}
	return symbols
}

func computeDerivativesLocalStats(rows []derivatives.Row) datasets.RowStats {
	var stats datasets.RowStats
	stats.RowCount = int64(len(rows))
	for i, r := range rows {
		if i == 0 || r.EventTimeMS < stats.MinEventTimeMS {
			stats.MinEventTimeMS = r.EventTimeMS
		}
		if i == 0 || r.EventTimeMS > stats.MaxEventTimeMS {
			stats.MaxEventTimeMS = r.EventTimeMS
		}
		if i == 0 || r.AvailableAtMS < stats.MinAvailableAtMS {
			stats.MinAvailableAtMS = r.AvailableAtMS
		}
		if i == 0 || r.AvailableAtMS > stats.MaxAvailableAtMS {
			stats.MaxAvailableAtMS = r.AvailableAtMS
		}
	}
	return stats
}
