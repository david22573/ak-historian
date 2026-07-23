package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(auditWorkdirCmd)
	auditWorkdirCmd.Flags().String("workdir", ".ak-historian/work", "working directory")
}

var auditWorkdirCmd = &cobra.Command{
	Use:   "audit-workdir",
	Short: "Audit historian workdir disk usage",
	RunE: func(cmd *cobra.Command, args []string) error {
		workdir, _ := cmd.Flags().GetString("workdir")
		return runAuditWorkdir(workdir)
	},
}

type DirStat struct {
	Path                 string   `json:"path"`
	Exists               bool     `json:"exists"`
	SizeBytes            int64    `json:"size_bytes"`
	SizeHuman            string   `json:"size_human"`
	FileCount            int      `json:"file_count"`
	ParquetCount         int      `json:"parquet_count"`
	LargestFiles         []string `json:"largest_files"`
	SafeCleanupCandidate bool     `json:"safe_cleanup_candidate"`
	ArchiveCandidate     bool     `json:"archive_candidate"`
}

type AuditReport struct {
	TotalWorkdirSize     int64              `json:"total_workdir_size"`
	TotalCandleSize      int64              `json:"total_candle_size"`
	TotalDatasetSize     int64              `json:"total_dataset_size"`
	LargestSymbolsBySize map[string]int64   `json:"largest_symbols_by_size"`
	LargestYearsBySize   map[string]int64   `json:"largest_years_by_size"`
	LargestMonthsBySize  map[string]int64   `json:"largest_months_by_size"`
	Directories          map[string]DirStat `json:"directories"`
}

type fileInfoEx struct {
	path string
	size int64
}

func humanSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func runAuditWorkdir(workdir string) error {
	report := &AuditReport{
		LargestSymbolsBySize: make(map[string]int64),
		LargestYearsBySize:   make(map[string]int64),
		LargestMonthsBySize:  make(map[string]int64),
		Directories:          make(map[string]DirStat),
	}

	absWorkdir, err := filepath.Abs(workdir)
	if err != nil {
		return err
	}

	// Tracks all files by their directory
	dirFiles := make(map[string][]fileInfoEx)
	dirBytes := make(map[string]int64)
	dirParquet := make(map[string]int)

	err = filepath.Walk(absWorkdir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			// Do not follow symlinks outside workdir. Actually, just skip them for safety.
			return nil
		}
		if !info.IsDir() {
			report.TotalWorkdirSize += info.Size()

			// Categorize
			rel, err := filepath.Rel(absWorkdir, path)
			if err == nil {
				parts := strings.Split(rel, string(filepath.Separator))

				// Track sizes for candles
				if len(parts) > 0 && parts[0] == "candles" {
					report.TotalCandleSize += info.Size()

					// candles/{market}/{interval}/symbol={SYMBOL}/year={YYYY}/month={MM}/...
					var symbol, year, month string
					for _, p := range parts {
						if strings.HasPrefix(p, "symbol=") {
							symbol = strings.TrimPrefix(p, "symbol=")
						} else if strings.HasPrefix(p, "year=") {
							year = strings.TrimPrefix(p, "year=")
						} else if strings.HasPrefix(p, "month=") {
							month = strings.TrimPrefix(p, "month=")
						}
					}
					if symbol != "" {
						report.LargestSymbolsBySize[symbol] += info.Size()
					}
					if year != "" {
						report.LargestYearsBySize[year] += info.Size()
					}
					if month != "" {
						report.LargestMonthsBySize[month] += info.Size()
					}
				} else if len(parts) > 0 && parts[0] == "datasets" {
					report.TotalDatasetSize += info.Size()
				}

				// Record in all parent directories
				dir := filepath.Dir(path)
				for {
					dirBytes[dir] += info.Size()
					dirFiles[dir] = append(dirFiles[dir], fileInfoEx{path: rel, size: info.Size()})
					if strings.HasSuffix(info.Name(), ".parquet") {
						dirParquet[dir]++
					}
					if dir == absWorkdir {
						break
					}
					dir = filepath.Dir(dir)
				}
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	targets := []string{
		absWorkdir,
		filepath.Join(absWorkdir, "candles"),
		filepath.Join(absWorkdir, "datasets"),
		filepath.Join(absWorkdir, "manifests"),
		filepath.Join(absWorkdir, "tmp"),
		filepath.Join(absWorkdir, "downloads"),
		filepath.Join(absWorkdir, "raw"),
		filepath.Join(absWorkdir, "converted"),
	}

	// Add candles/{market} and below
	candlesDir := filepath.Join(absWorkdir, "candles")
	if dirs, err := os.ReadDir(candlesDir); err == nil {
		for _, m := range dirs {
			if m.IsDir() {
				marketDir := filepath.Join(candlesDir, m.Name())
				targets = append(targets, marketDir)
				if idirs, err := os.ReadDir(marketDir); err == nil {
					for _, i := range idirs {
						if i.IsDir() {
							intervalDir := filepath.Join(marketDir, i.Name())
							targets = append(targets, intervalDir)
							if sdirs, err := os.ReadDir(intervalDir); err == nil {
								for _, s := range sdirs {
									if s.IsDir() && strings.HasPrefix(s.Name(), "symbol=") {
										targets = append(targets, filepath.Join(intervalDir, s.Name()))
									}
								}
							}
						}
					}
				}
			}
		}
	}

	for _, t := range targets {
		rel, _ := filepath.Rel(absWorkdir, t)
		if rel == "." {
			rel = "workdir root"
		}
		_, err := os.Stat(t)
		exists := !os.IsNotExist(err)

		var largest []string
		files := dirFiles[t]
		sort.Slice(files, func(i, j int) bool {
			return files[i].size > files[j].size
		})
		for i := 0; i < 5 && i < len(files); i++ {
			largest = append(largest, files[i].path)
		}

		stat := DirStat{
			Path:         rel,
			Exists:       exists,
			SizeBytes:    dirBytes[t],
			SizeHuman:    humanSize(dirBytes[t]),
			FileCount:    len(dirFiles[t]),
			ParquetCount: dirParquet[t],
			LargestFiles: largest,
		}

		// Heuristics
		if strings.HasPrefix(rel, "tmp") || strings.HasPrefix(rel, "downloads") || strings.HasPrefix(rel, "raw") || strings.HasPrefix(rel, "converted") {
			stat.SafeCleanupCandidate = true
		}
		if strings.HasPrefix(rel, "candles") {
			stat.ArchiveCandidate = true
		}

		report.Directories[rel] = stat
	}

	// Output
	reportsDir := filepath.Join(absWorkdir, "reports")
	os.MkdirAll(reportsDir, 0755)

	jBytes, _ := json.MarshalIndent(report, "", "  ")
	jsonPath := filepath.Join(reportsDir, "phase10_5e_workdir_audit.json")
	os.WriteFile(jsonPath, jBytes, 0644)

	mdPath := filepath.Join(reportsDir, "phase10_5e_workdir_audit.md")
	md := fmt.Sprintf("# Workdir Audit\n\nTotal Size: %s\nCandles Size: %s\nDatasets Size: %s\n",
		humanSize(report.TotalWorkdirSize),
		humanSize(report.TotalCandleSize),
		humanSize(report.TotalDatasetSize))
	md += "\n## Largest Symbols\n"
	for s, b := range report.LargestSymbolsBySize {
		md += fmt.Sprintf("- %s: %s\n", s, humanSize(b))
	}
	os.WriteFile(mdPath, []byte(md), 0644)

	fmt.Printf("Workdir audit complete.\n")
	fmt.Printf("Total Workdir Size: %s\n", humanSize(report.TotalWorkdirSize))
	fmt.Printf("Total Candle Size: %s\n", humanSize(report.TotalCandleSize))
	fmt.Printf("Total Dataset Size: %s\n", humanSize(report.TotalDatasetSize))

	fmt.Println("Largest Symbols:")
	for s, b := range report.LargestSymbolsBySize {
		fmt.Printf("  %s: %s\n", s, humanSize(b))
	}
	fmt.Println("Largest Years:")
	for y, b := range report.LargestYearsBySize {
		fmt.Printf("  %s: %s\n", y, humanSize(b))
	}
	fmt.Println("Largest Months:")
	for m, b := range report.LargestMonthsBySize {
		fmt.Printf("  %s: %s\n", m, humanSize(b))
	}

	return nil
}
