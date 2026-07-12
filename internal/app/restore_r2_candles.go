package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/david22573/ak-historian/internal/binance"
	"github.com/david22573/ak-historian/internal/config"
	"github.com/david22573/ak-historian/internal/storage"
	"github.com/david22573/ak-historian/internal/validate"
	"github.com/spf13/cobra"
)

var (
	restoreR2Market      string
	restoreR2Interval    string
	restoreR2Symbols     string
	restoreR2Start       string
	restoreR2End         string
	restoreR2Out         string
	restoreR2DryRun      string
	restoreR2Overwrite   string
	restoreR2Verify      string
	restoreR2Concurrency int
)

type R2CandleRestoreStore interface {
	ObjectExists(ctx context.Context, key string) (bool, error)
	DownloadFile(ctx context.Context, objectKey string, localPath string) error
}

type R2CandleRestoreOptions struct {
	Market      string
	Interval    string
	Symbols     []string
	Start       string
	End         string
	Out         string
	DryRun      bool
	Overwrite   bool
	Verify      bool
	Concurrency int
	ReportDir   string
	CommandsRun []string
}

type R2CandleRestoreReport struct {
	FinalLabel            string   `json:"final_label"`
	Market                string   `json:"market"`
	Interval              string   `json:"interval"`
	Symbols               []string `json:"symbols"`
	MonthsRequested       []string `json:"months_requested"`
	ObjectsExpected       int      `json:"objects_expected"`
	ObjectsFound          int      `json:"objects_found"`
	ObjectsMissing        []string `json:"objects_missing"`
	FilesRestored         []string `json:"files_restored"`
	FilesSkippedExisting  []string `json:"files_skipped_existing"`
	FilesFailedValidation []string `json:"files_failed_validation"`
	FilesFailedRestore    []string `json:"files_failed_restore"`
	LocalOutputRoot       string   `json:"local_output_root"`
	Overwrite             bool     `json:"overwrite"`
	DryRun                bool     `json:"dry_run"`
	Verify                bool     `json:"verify"`
	CommandsRun           []string `json:"commands_run"`
}

type r2CandleRestoreJob struct {
	Symbol    string
	Month     string
	ObjectKey string
	LocalPath string
}

type parquetValidator func(context.Context, string) error

var restoreR2CandlesCmd = &cobra.Command{
	Use:   "restore-r2-candles",
	Short: "Restore candle Parquet files from R2 into a local archive workdir",
	RunE: func(cmd *cobra.Command, args []string) error {
		dryRun, err := parseRestoreBoolFlag("dry-run", restoreR2DryRun)
		if err != nil {
			return err
		}
		overwrite, err := parseRestoreBoolFlag("overwrite", restoreR2Overwrite)
		if err != nil {
			return err
		}
		verify, err := parseRestoreBoolFlag("verify", restoreR2Verify)
		if err != nil {
			return err
		}
		opts := R2CandleRestoreOptions{
			Market:      restoreR2Market,
			Interval:    restoreR2Interval,
			Symbols:     strings.Split(restoreR2Symbols, ","),
			Start:       restoreR2Start,
			End:         restoreR2End,
			Out:         restoreR2Out,
			DryRun:      dryRun,
			Overwrite:   overwrite,
			Verify:      verify,
			Concurrency: restoreR2Concurrency,
			ReportDir:   filepath.Join("runs", "reports"),
			CommandsRun: []string{strings.Join(os.Args, " ")},
		}
		return RunRestoreR2Candles(cmd.Context(), opts)
	},
}

func init() {
	rootCmd.AddCommand(restoreR2CandlesCmd)

	restoreR2CandlesCmd.Flags().StringVar(&restoreR2Market, "market", "futures-um", "market type (futures-um, futures-cm, spot)")
	restoreR2CandlesCmd.Flags().StringVar(&restoreR2Interval, "interval", "1m", "kline interval")
	restoreR2CandlesCmd.Flags().StringVar(&restoreR2Symbols, "symbols", "", "comma-separated symbols")
	restoreR2CandlesCmd.Flags().StringVar(&restoreR2Start, "start", "", "start month YYYY-MM")
	restoreR2CandlesCmd.Flags().StringVar(&restoreR2End, "end", "", "end month YYYY-MM")
	restoreR2CandlesCmd.Flags().StringVar(&restoreR2Out, "out", ".ak-historian/work", "local candle parquet workdir")
	restoreR2CandlesCmd.Flags().StringVar(&restoreR2DryRun, "dry-run", "true", "true/false; plan and verify R2 objects without downloading")
	restoreR2CandlesCmd.Flags().StringVar(&restoreR2Overwrite, "overwrite", "false", "true/false; overwrite existing local parquet files")
	restoreR2CandlesCmd.Flags().StringVar(&restoreR2Verify, "verify", "true", "true/false; validate downloaded parquet before moving into place")
	restoreR2CandlesCmd.Flags().IntVar(&restoreR2Concurrency, "concurrency", 2, "number of concurrent R2 restores")

	_ = restoreR2CandlesCmd.MarkFlagRequired("symbols")
	_ = restoreR2CandlesCmd.MarkFlagRequired("start")
	_ = restoreR2CandlesCmd.MarkFlagRequired("end")
	_ = restoreR2CandlesCmd.MarkFlagRequired("out")
}

func RunRestoreR2Candles(ctx context.Context, opts R2CandleRestoreOptions) error {
	cfg, err := config.LoadR2Config()
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}
	r2, err := storage.NewR2Client(ctx, cfg)
	if err != nil {
		return fmt.Errorf("storage error: %w", err)
	}
	return runRestoreR2Candles(ctx, opts, r2, validateDownloadedParquet)
}

func validateDownloadedParquet(ctx context.Context, path string) error {
	_, err := validate.ValidateParquet(ctx, path)
	return err
}

func parseRestoreBoolFlag(name string, value string) (bool, error) {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return false, fmt.Errorf("--%s must be true or false", name)
	}
	return parsed, nil
}

func runRestoreR2Candles(ctx context.Context, opts R2CandleRestoreOptions, store R2CandleRestoreStore, validator parquetValidator) error {
	opts, months, jobs, err := prepareR2CandleRestore(opts)
	if err != nil {
		return err
	}
	if validator == nil {
		validator = validateDownloadedParquet
	}

	report := &R2CandleRestoreReport{
		Market:                opts.Market,
		Interval:              opts.Interval,
		Symbols:               opts.Symbols,
		MonthsRequested:       months,
		ObjectsExpected:       len(jobs),
		ObjectsMissing:        []string{},
		FilesRestored:         []string{},
		FilesSkippedExisting:  []string{},
		FilesFailedValidation: []string{},
		FilesFailedRestore:    []string{},
		LocalOutputRoot:       opts.Out,
		Overwrite:             opts.Overwrite,
		DryRun:                opts.DryRun,
		Verify:                opts.Verify,
		CommandsRun:           opts.CommandsRun,
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	jobCh := make(chan r2CandleRestoreJob)

	for i := 0; i < opts.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				found, localState, err := restoreOneR2Candle(ctx, opts, store, validator, job)
				mu.Lock()
				if found {
					report.ObjectsFound++
				} else {
					report.ObjectsMissing = append(report.ObjectsMissing, job.ObjectKey)
				}
				switch localState {
				case "restored":
					report.FilesRestored = append(report.FilesRestored, job.LocalPath)
				case "skipped_existing":
					report.FilesSkippedExisting = append(report.FilesSkippedExisting, job.LocalPath)
				case "failed_validation":
					report.FilesFailedValidation = append(report.FilesFailedValidation, job.LocalPath)
				case "failed_restore":
					report.FilesFailedRestore = append(report.FilesFailedRestore, job.LocalPath)
				}
				if err != nil && localState == "" {
					report.FilesFailedRestore = append(report.FilesFailedRestore, job.LocalPath)
				}
				mu.Unlock()
			}
		}()
	}

feedLoop:
	for _, job := range jobs {
		select {
		case <-ctx.Done():
			break feedLoop
		case jobCh <- job:
		}
	}
	close(jobCh)
	wg.Wait()

	report.FinalLabel = r2CandleRestoreFinalLabel(report)
	if err := writeR2CandleRestoreReports(opts.ReportDir, report); err != nil {
		return err
	}

	switch report.FinalLabel {
	case "blocked_missing_r2_objects":
		return fmt.Errorf("missing %d requested R2 objects; report: %s", len(report.ObjectsMissing), filepath.Join(opts.ReportDir, "r2_restore_candles.json"))
	case "blocked_validation_failure":
		return fmt.Errorf("validation failed for %d files; report: %s", len(report.FilesFailedValidation), filepath.Join(opts.ReportDir, "r2_restore_candles.json"))
	case "partial_restore":
		return fmt.Errorf("restore completed with %d failed files; report: %s", len(report.FilesFailedRestore), filepath.Join(opts.ReportDir, "r2_restore_candles.json"))
	default:
		return nil
	}
}

func prepareR2CandleRestore(opts R2CandleRestoreOptions) (R2CandleRestoreOptions, []string, []r2CandleRestoreJob, error) {
	if opts.Concurrency < 1 {
		return opts, nil, nil, fmt.Errorf("concurrency must be at least 1")
	}
	if opts.Out == "" {
		return opts, nil, nil, fmt.Errorf("out cannot be empty")
	}
	if opts.ReportDir == "" {
		opts.ReportDir = filepath.Join("runs", "reports")
	}
	switch opts.Market {
	case "futures-um", "futures-cm", "spot":
	default:
		return opts, nil, nil, fmt.Errorf("invalid market: %s", opts.Market)
	}
	if opts.Interval == "" || strings.ContainsAny(opts.Interval, `/\`) {
		return opts, nil, nil, fmt.Errorf("invalid interval: %s", opts.Interval)
	}

	symbols := make([]string, 0, len(opts.Symbols))
	for _, symbol := range opts.Symbols {
		symbol = strings.ToUpper(strings.TrimSpace(symbol))
		if symbol == "" || strings.ContainsAny(symbol, `/\`) || strings.ContainsAny(symbol, " \t\n\r") {
			return opts, nil, nil, fmt.Errorf("invalid symbol: %s", symbol)
		}
		symbols = append(symbols, symbol)
	}
	if len(symbols) == 0 {
		return opts, nil, nil, fmt.Errorf("at least one symbol must be provided")
	}
	opts.Symbols = symbols

	months, err := binance.ExpandDates("monthly", opts.Start, opts.End)
	if err != nil {
		return opts, nil, nil, fmt.Errorf("month expansion error: %w", err)
	}

	jobs := make([]r2CandleRestoreJob, 0, len(symbols)*len(months))
	for _, symbol := range symbols {
		for _, month := range months {
			spec := binance.ArchiveSpec{
				Market:   opts.Market,
				Symbol:   symbol,
				Interval: opts.Interval,
				Period:   "monthly",
				Date:     month,
			}
			key, err := binance.ObjectKey(spec)
			if err != nil {
				return opts, nil, nil, err
			}
			jobs = append(jobs, r2CandleRestoreJob{
				Symbol:    symbol,
				Month:     month,
				ObjectKey: key,
				LocalPath: filepath.Join(opts.Out, filepath.FromSlash(key)),
			})
		}
	}

	return opts, months, jobs, nil
}

func restoreOneR2Candle(
	ctx context.Context,
	opts R2CandleRestoreOptions,
	store R2CandleRestoreStore,
	validator parquetValidator,
	job r2CandleRestoreJob,
) (bool, string, error) {
	exists, err := store.ObjectExists(ctx, job.ObjectKey)
	if err != nil {
		return false, "failed_restore", fmt.Errorf("check R2 object %s: %w", job.ObjectKey, err)
	}
	if !exists {
		return false, "", nil
	}

	if localFileExists(job.LocalPath) && !opts.Overwrite {
		return true, "skipped_existing", nil
	}
	if opts.DryRun {
		return true, "", nil
	}

	if err := os.MkdirAll(filepath.Dir(job.LocalPath), 0755); err != nil {
		return true, "failed_restore", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(job.LocalPath), "."+filepath.Base(job.LocalPath)+".tmp-*")
	if err != nil {
		return true, "failed_restore", err
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return true, "failed_restore", err
	}

	moved := false
	defer func() {
		if !moved {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := store.DownloadFile(ctx, job.ObjectKey, tmpPath); err != nil {
		return true, "failed_restore", fmt.Errorf("download %s: %w", job.ObjectKey, err)
	}
	if opts.Verify {
		if err := validator(ctx, tmpPath); err != nil {
			return true, "failed_validation", fmt.Errorf("validate %s: %w", job.ObjectKey, err)
		}
	}
	if err := os.Rename(tmpPath, job.LocalPath); err != nil {
		return true, "failed_restore", fmt.Errorf("move restored file into place: %w", err)
	}
	moved = true
	return true, "restored", nil
}

func localFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func r2CandleRestoreFinalLabel(report *R2CandleRestoreReport) string {
	if len(report.ObjectsMissing) > 0 {
		return "blocked_missing_r2_objects"
	}
	if len(report.FilesFailedValidation) > 0 {
		return "blocked_validation_failure"
	}
	if len(report.FilesFailedRestore) > 0 {
		return "partial_restore"
	}
	if report.DryRun {
		return "dry_run_complete"
	}
	return "restore_complete"
}

func writeR2CandleRestoreReports(reportDir string, report *R2CandleRestoreReport) error {
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		return err
	}
	jsonPath := filepath.Join(reportDir, "r2_restore_candles.json")
	mdPath := filepath.Join(reportDir, "r2_restore_candles.md")

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(jsonPath, append(data, '\n'), 0644); err != nil {
		return err
	}

	md := fmt.Sprintf(`# R2 Candle Restore

final_label: %s
market: %s
interval: %s
symbols: %s
months_requested: %s
objects_expected: %d
objects_found: %d
objects_missing: %d
files_restored: %d
files_skipped_existing: %d
files_failed_validation: %d
files_failed_restore: %d
local_output_root: %s
overwrite: %t
dry_run: %t
verify: %t
`,
		report.FinalLabel,
		report.Market,
		report.Interval,
		strings.Join(report.Symbols, ","),
		strings.Join(report.MonthsRequested, ","),
		report.ObjectsExpected,
		report.ObjectsFound,
		len(report.ObjectsMissing),
		len(report.FilesRestored),
		len(report.FilesSkippedExisting),
		len(report.FilesFailedValidation),
		len(report.FilesFailedRestore),
		report.LocalOutputRoot,
		report.Overwrite,
		report.DryRun,
		report.Verify,
	)
	return os.WriteFile(mdPath, []byte(md), 0644)
}
