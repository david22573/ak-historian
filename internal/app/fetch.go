package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/david22573/ak-historian/internal/binance"
	"github.com/david22573/ak-historian/internal/config"
	"github.com/david22573/ak-historian/internal/converter"
	"github.com/david22573/ak-historian/internal/storage"
	"github.com/david22573/ak-historian/internal/validate"
	"github.com/david22573/ak-historian/internal/workdir"
	"github.com/spf13/cobra"
)

var (
	market      string
	symbols     string
	interval    string
	period      string
	start       string
	end         string
	workDirFlag string
	concurrency int
	force       bool
	dryRun      bool
	keep        bool
	verify      bool
)

type FetchOptions struct {
	Market      string
	Symbols     []string
	Interval    string
	Period      string
	Start       string
	End         string
	WorkDir     string
	Concurrency int
	Force       bool
	DryRun      bool
	Keep        bool
	Verify      bool
}

type Summary struct {
	Planned         int
	DryRunPlanned   int
	Uploaded        int
	SkippedExisting int
	SkippedMissing  int
	Failed          int
	mu              sync.Mutex
}

func (s *Summary) IncPlanned() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Planned++
}

func (s *Summary) IncDryRunPlanned() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.DryRunPlanned++
}

func (s *Summary) IncUploaded() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Uploaded++
}

func (s *Summary) IncSkippedExisting() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SkippedExisting++
}

func (s *Summary) IncSkippedMissing() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SkippedMissing++
}

func (s *Summary) IncFailed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Failed++
}

type SummarySnapshot struct {
	Planned         int
	DryRunPlanned   int
	Uploaded        int
	SkippedExisting int
	SkippedMissing  int
	Failed          int
}

func (s *Summary) Snapshot() SummarySnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return SummarySnapshot{
		Planned:         s.Planned,
		DryRunPlanned:   s.DryRunPlanned,
		Uploaded:        s.Uploaded,
		SkippedExisting: s.SkippedExisting,
		SkippedMissing:  s.SkippedMissing,
		Failed:          s.Failed,
	}
}

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Fetch and process Binance historical data",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := FetchOptions{
			Market:      market,
			Symbols:     strings.Split(symbols, ","),
			Interval:    interval,
			Period:      period,
			Start:       start,
			End:         end,
			WorkDir:     workDirFlag,
			Concurrency: concurrency,
			Force:       force,
			DryRun:      dryRun,
			Keep:        keep,
			Verify:      verify,
		}

		return RunFetch(cmd.Context(), opts)
	},
}

func init() {
	rootCmd.AddCommand(fetchCmd)

	defaultConcurrency := 2
	if os.Getenv("PREFIX") == "/data/data/com.termux/files/usr" || os.Getenv("CROS_USER_ID_HASH") != "" || os.Getenv("SOMMELIER_VERSION") != "" {
		defaultConcurrency = 1
	}

	fetchCmd.Flags().StringVar(&market, "market", "futures-um", "market type (futures-um, futures-cm, spot)")
	fetchCmd.Flags().StringVar(&symbols, "symbols", "BTCUSDT", "comma-separated list of symbols")
	fetchCmd.Flags().StringVar(&interval, "interval", "1m", "kline interval")
	fetchCmd.Flags().StringVar(&period, "period", "monthly", "data period (monthly, daily)")
	fetchCmd.Flags().StringVar(&start, "start", "", "start date (YYYY-MM or YYYY-MM-DD)")
	fetchCmd.Flags().StringVar(&end, "end", "", "end date (YYYY-MM or YYYY-MM-DD)")
	fetchCmd.Flags().StringVar(&workDirFlag, "workdir", ".ak-historian/work", "working directory")
	fetchCmd.Flags().IntVar(&concurrency, "concurrency", defaultConcurrency, "number of concurrent downloads")
	fetchCmd.Flags().BoolVar(&force, "force", false, "force re-download and re-process")
	fetchCmd.Flags().BoolVar(&dryRun, "dry-run", false, "dry run (only show what would be done)")
	fetchCmd.Flags().BoolVar(&keep, "keep", false, "keep temporary files after processing")
	fetchCmd.Flags().BoolVar(&verify, "verify", true, "verify checksums")

	_ = fetchCmd.MarkFlagRequired("start")
	_ = fetchCmd.MarkFlagRequired("end")
}

type FetchJob struct {
	Symbol string
	Date   string
}

type ObjectStore interface {
	ObjectExists(ctx context.Context, key string) (bool, error)
	UploadFile(ctx context.Context, localPath string, objectKey string) error
}

type ArchiveDownloader interface {
	DownloadArchive(ctx context.Context, url string, destPath string, force bool) (binance.DownloadStatus, error)
	DownloadChecksum(ctx context.Context, checksumURL string) (string, error)
}

func RunFetch(ctx context.Context, opts FetchOptions) error {
	if opts.Concurrency < 1 {
		return fmt.Errorf("concurrency must be at least 1")
	}
	if opts.WorkDir == "" {
		return fmt.Errorf("workdir cannot be empty")
	}

	switch opts.Market {
	case "futures-um", "futures-cm", "spot":
		// ok
	default:
		return fmt.Errorf("invalid market: %s", opts.Market)
	}

	switch opts.Period {
	case "monthly", "daily":
		// ok
	default:
		return fmt.Errorf("invalid period: %s", opts.Period)
	}

	if opts.Interval == "" || strings.ContainsAny(opts.Interval, `/\`) {
		return fmt.Errorf("invalid interval: %s", opts.Interval)
	}

	if len(opts.Symbols) == 0 {
		return fmt.Errorf("at least one symbol must be provided")
	}

	validSymbols := make([]string, 0, len(opts.Symbols))
	for _, s := range opts.Symbols {
		s = strings.ToUpper(strings.TrimSpace(s))
		if s == "" || strings.ContainsAny(s, `/\`) || strings.ContainsAny(s, " \t\n\r") {
			return fmt.Errorf("invalid symbol: %s", s)
		}
		validSymbols = append(validSymbols, s)
	}
	opts.Symbols = validSymbols

	cfg, err := config.LoadR2Config()
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	r2, err := storage.NewR2Client(ctx, cfg)
	if err != nil {
		return fmt.Errorf("storage error: %w", err)
	}

	downloader := binance.NewDownloader()

	return runFetch(ctx, opts, r2, downloader)
}

func runFetch(ctx context.Context, opts FetchOptions, r2 ObjectStore, downloader ArchiveDownloader) error {
	dates, err := binance.ExpandDates(opts.Period, opts.Start, opts.End)
	if err != nil {
		return fmt.Errorf("date expansion error: %w", err)
	}

	summary := &Summary{}

	jobs := make(chan FetchJob)
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < opts.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-jobs:
					if !ok {
						return
					}
					err := processItem(ctx, opts, job.Symbol, job.Date, downloader, r2, summary)
					if err != nil {
						log.Printf("Error processing %s %s: %v", job.Symbol, job.Date, err)
						summary.IncFailed()
					}
				}
			}
		}()
	}

	// Feed jobs
feedLoop:
	for _, symbol := range opts.Symbols {
		for _, date := range dates {
			summary.IncPlanned()

			select {
			case jobs <- FetchJob{Symbol: symbol, Date: date}:
			case <-ctx.Done():
				break feedLoop
			}
		}
	}
	close(jobs)

	wg.Wait()

	snap := summary.Snapshot()
	fmt.Printf("\nSummary:\n")
	fmt.Printf("  planned: %d\n", snap.Planned)
	if opts.DryRun {
		fmt.Printf("  dry_run_planned: %d\n", snap.DryRunPlanned)
	}
	fmt.Printf("  uploaded: %d\n", snap.Uploaded)
	fmt.Printf("  skipped_existing: %d\n", snap.SkippedExisting)
	fmt.Printf("  skipped_missing: %d\n", snap.SkippedMissing)
	fmt.Printf("  failed: %d\n", snap.Failed)

	if snap.Failed > 0 {
		return fmt.Errorf("completed with %d failures", snap.Failed)
	}

	return nil
}

func processItem(
	ctx context.Context,
	opts FetchOptions,
	symbol, date string,
	downloader ArchiveDownloader,
	r2 ObjectStore,
	summary *Summary,
) error {
	spec := binance.ArchiveSpec{
		Market:   opts.Market,
		Symbol:   symbol,
		Interval: opts.Interval,
		Period:   opts.Period,
		Date:     date,
	}

	archiveURL, err := binance.BuildArchiveURL(spec)
	if err != nil {
		return err
	}

	objectKey, err := binance.ObjectKey(spec)
	if err != nil {
		return err
	}

	paths, err := workdir.BuildPaths(opts.WorkDir, spec)
	if err != nil {
		return err
	}

	if !opts.Force && !opts.DryRun {
		exists, err := r2.ObjectExists(ctx, objectKey)
		if err != nil {
			return err
		}
		if exists {
			log.Printf("Skip existing: %s %s", symbol, date)
			summary.IncSkippedExisting()
			return nil
		}
	}

	if opts.DryRun {
		log.Printf("Plan: %s %s -> %s", symbol, date, objectKey)
		summary.IncDryRunPlanned()
		return nil
	}

	// Download
	status, err := downloader.DownloadArchive(ctx, archiveURL, paths.ZipPath, opts.Force)
	if err != nil {
		return err
	}
	if status == binance.NotFound {
		log.Printf("Skip missing: %s %s", symbol, date)
		summary.IncSkippedMissing()
		return nil
	}

	// Checksum
	if opts.Verify {
		checksumURL := binance.BuildChecksumURL(archiveURL)
		expectedChecksum, err := downloader.DownloadChecksum(ctx, checksumURL)
		if err != nil {
			if errors.Is(err, binance.ErrChecksumNotFound) {
				log.Printf("Warning: checksum missing for %s %s, continuing", symbol, date)
			} else {
				return fmt.Errorf("checksum download failed: %w", err)
			}
		} else {
			err = binance.VerifySHA256(paths.ZipPath, expectedChecksum)
			if err != nil {
				_ = os.Remove(paths.ZipPath)
				return fmt.Errorf("checksum verification failed: %w", err)
			}
		}
	}

	// Extract
	csvPath, err := binance.ExtractExpectedCSV(paths.ZipPath, binance.CSVFileName(spec), paths.ItemDir)
	if err != nil {
		return err
	}

	// Convert
	convOpts := converter.ConvertOptions{
		CSVPath:     csvPath,
		ParquetPath: paths.ParquetPath,
		Market:      opts.Market,
		Symbol:      symbol,
		Interval:    opts.Interval,
		Period:      opts.Period,
		SourceDate:  date,
	}
	err = converter.ConvertKlinesCSVToParquet(ctx, convOpts)
	if err != nil {
		return err
	}

	// Validate
	stats, err := validate.ValidateParquet(ctx, paths.ParquetPath)
	if err != nil {
		return err
	}
	log.Printf("Validated: %s %s rows=%d", symbol, date, stats.RowCount)

	// Upload
	err = r2.UploadFile(ctx, paths.ParquetPath, objectKey)
	if err != nil {
		return err
	}

	// Cleanup
	if !opts.Keep {
		_ = os.Remove(paths.ZipPath)
		_ = os.Remove(csvPath)
		_ = os.Remove(paths.ParquetPath)
		// Try remove itemDir if empty
		_ = os.Remove(paths.ItemDir)
	}

	summary.IncUploaded()

	return nil
}
