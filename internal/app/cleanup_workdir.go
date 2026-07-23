package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/david22573/ak-historian/internal/workdir"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(cleanupWorkdirCmd)
	cleanupWorkdirCmd.Flags().String("workdir", ".ak-historian/work", "working directory")
	cleanupWorkdirCmd.Flags().String("market", "", "market type")
	cleanupWorkdirCmd.Flags().String("interval", "", "interval")
	cleanupWorkdirCmd.Flags().String("symbols", "", "comma separated symbols")
	cleanupWorkdirCmd.Flags().String("from", "", "from YYYY-MM")
	cleanupWorkdirCmd.Flags().String("to", "", "to YYYY-MM")
	cleanupWorkdirCmd.Flags().String("retain-symbols", "BTCUSDT,ETHUSDT", "symbols to keep")

	cleanupWorkdirCmd.Flags().Bool("dry-run", true, "dry run mode")
	cleanupWorkdirCmd.Flags().Bool("force", false, "force delete")
	cleanupWorkdirCmd.Flags().Bool("only-verified-archive", true, "only delete verified archive")
	cleanupWorkdirCmd.Flags().Bool("allow-delete-local-only", false, "allow deleting unarchived local data")
	cleanupWorkdirCmd.Flags().Int("max-delete-gb", 0, "max GB to delete (0=unlimited)")
	cleanupWorkdirCmd.Flags().Bool("include-datasets", false, "include datasets in cleanup")
	cleanupWorkdirCmd.Flags().Bool("include-manifests", false, "include manifests in cleanup")
}

var cleanupWorkdirCmd = &cobra.Command{
	Use:   "cleanup-workdir",
	Short: "Clean up historian workdir",
	RunE: func(cmd *cobra.Command, args []string) error {
		wd, _ := cmd.Flags().GetString("workdir")
		market, _ := cmd.Flags().GetString("market")
		interval, _ := cmd.Flags().GetString("interval")
		symbols, _ := cmd.Flags().GetString("symbols")
		from, _ := cmd.Flags().GetString("from")
		to, _ := cmd.Flags().GetString("to")
		retainSymbols, _ := cmd.Flags().GetString("retain-symbols")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		force, _ := cmd.Flags().GetBool("force")
		onlyVerified, _ := cmd.Flags().GetBool("only-verified-archive")
		allowLocal, _ := cmd.Flags().GetBool("allow-delete-local-only")
		maxGB, _ := cmd.Flags().GetInt("max-delete-gb")
		incDatasets, _ := cmd.Flags().GetBool("include-datasets")
		incManifests, _ := cmd.Flags().GetBool("include-manifests")

		if force {
			dryRun = false
		}

		return runCleanupWorkdir(wd, market, interval, symbols, from, to, retainSymbols, dryRun, force, onlyVerified, allowLocal, maxGB, incDatasets, incManifests)
	},
}

type CleanupReport struct {
	FilesToDelete    []string          `json:"files_to_delete"`
	BytesToDelete    int64             `json:"bytes_to_delete"`
	FilesToKeep      []string          `json:"files_to_keep"`
	BlockedDeletions map[string]string `json:"blocked_deletions"` // path -> reason
}

func runCleanupWorkdir(wd, market, interval, symbolsStr, from, to, retainSymbolsStr string, dryRun, force, onlyVerified, allowLocal bool, maxGB int, incDatasets, incManifests bool) error {
	absWd, err := filepath.Abs(wd)
	if err != nil {
		return err
	}

	retainMap := make(map[string]bool)
	if retainSymbolsStr != "" {
		for _, s := range strings.Split(retainSymbolsStr, ",") {
			retainMap[s] = true
		}
	}
	targetMap := make(map[string]bool)
	if symbolsStr != "" {
		for _, s := range strings.Split(symbolsStr, ",") {
			targetMap[s] = true
		}
	}

	manifest, err := workdir.LoadLocalSourceManifest(absWd)
	if err != nil {
		return fmt.Errorf("could not load manifest: %w", err)
	}

	report := CleanupReport{
		BlockedDeletions: make(map[string]string),
	}

	maxBytes := int64(maxGB) * 1024 * 1024 * 1024

	fromYear, toYear := "", ""
	if len(from) >= 4 {
		fromYear = from[:4]
	}
	if len(to) >= 4 {
		toYear = to[:4]
	}

	err = filepath.Walk(absWd, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			// Do not traverse symlinks
			return nil
		}

		// Ensure path is within absWd
		rel, err := filepath.Rel(absWd, path)
		if err != nil || strings.HasPrefix(rel, "..") {
			report.BlockedDeletions[path] = "outside workdir"
			return nil
		}

		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) == 0 {
			return nil
		}

		isReport := parts[0] == "reports"
		isManifest := parts[0] == "manifests"
		isDataset := parts[0] == "datasets"
		isCandle := parts[0] == "candles"

		if isReport {
			report.BlockedDeletions[rel] = "never delete reports"
			report.FilesToKeep = append(report.FilesToKeep, rel)
			return nil
		}
		if isManifest && !incManifests {
			report.BlockedDeletions[rel] = "manifests excluded by default"
			report.FilesToKeep = append(report.FilesToKeep, rel)
			return nil
		}
		if isDataset && !incDatasets {
			report.BlockedDeletions[rel] = "datasets excluded by default"
			report.FilesToKeep = append(report.FilesToKeep, rel)
			return nil
		}

		if isCandle && strings.HasSuffix(info.Name(), ".parquet") {
			// Parse structure: candles/{market}/{interval}/symbol={SYMBOL}/year={YYYY}/month={MM}/{file}.parquet
			var sym, yr string
			for _, p := range parts {
				if strings.HasPrefix(p, "symbol=") {
					sym = strings.TrimPrefix(p, "symbol=")
				}
				if strings.HasPrefix(p, "year=") {
					yr = strings.TrimPrefix(p, "year=")
				}
			}

			if retainMap[sym] {
				report.BlockedDeletions[rel] = "retained symbol"
				report.FilesToKeep = append(report.FilesToKeep, rel)
				return nil
			}
			if len(targetMap) > 0 && !targetMap[sym] {
				// Not in target set, but not explicitly blocked? Actually, if targets specified, only delete targets.
				report.FilesToKeep = append(report.FilesToKeep, rel)
				return nil
			}
			if fromYear != "" && yr < fromYear {
				report.FilesToKeep = append(report.FilesToKeep, rel)
				return nil
			}
			if toYear != "" && yr > toYear {
				report.FilesToKeep = append(report.FilesToKeep, rel)
				return nil
			}

			// Check archive status
			obj, ok := manifest.Objects[rel]
			isVerified := false
			if ok && obj.ArchivedStatus == workdir.ArchivedStatusVerifiedArchive {
				isVerified = true
			}

			if !isVerified && !allowLocal {
				report.BlockedDeletions[rel] = "unverified archive, --allow-delete-local-only not set"
				report.FilesToKeep = append(report.FilesToKeep, rel)
				return nil
			}
			if onlyVerified && !isVerified {
				report.BlockedDeletions[rel] = "--only-verified-archive active and not verified"
				report.FilesToKeep = append(report.FilesToKeep, rel)
				return nil
			}

			// If we got here, it's safe to delete based on rules
			if maxGB > 0 && report.BytesToDelete+info.Size() > maxBytes {
				report.BlockedDeletions[rel] = "exceeds max-delete-gb"
				report.FilesToKeep = append(report.FilesToKeep, rel)
				return nil
			}

			report.FilesToDelete = append(report.FilesToDelete, rel)
			report.BytesToDelete += info.Size()

			if !dryRun && force {
				os.Remove(path)
			}
			return nil
		}

		// Keep anything else by default unless it's a target (like empty dirs, which we aren't cleaning up yet)
		report.FilesToKeep = append(report.FilesToKeep, rel)
		return nil
	})

	if err != nil {
		return err
	}

	if dryRun {
		fmt.Println("DRY RUN: No files deleted.")
		fmt.Printf("Files to delete: %d\n", len(report.FilesToDelete))
		fmt.Printf("Bytes to delete: %s\n", humanSize(report.BytesToDelete))
		fmt.Printf("Files to keep: %d\n", len(report.FilesToKeep))
		fmt.Printf("Blocked deletions: %d\n", len(report.BlockedDeletions))
		for k, v := range report.BlockedDeletions {
			fmt.Printf("  %s: %s\n", k, v)
		}
		return nil
	}

	reportsDir := filepath.Join(absWd, "reports")
	os.MkdirAll(reportsDir, 0755)

	jBytes, _ := json.MarshalIndent(report, "", "  ")
	os.WriteFile(filepath.Join(reportsDir, "phase10_5e_cleanup_workdir_report.json"), jBytes, 0644)

	md := "# Cleanup Report\n\n"
	md += fmt.Sprintf("Files Deleted: %d\n", len(report.FilesToDelete))
	md += fmt.Sprintf("Bytes Freed: %s\n", humanSize(report.BytesToDelete))
	md += fmt.Sprintf("Files Kept: %d\n", len(report.FilesToKeep))
	md += fmt.Sprintf("Blocked Deletions: %d\n", len(report.BlockedDeletions))
	os.WriteFile(filepath.Join(reportsDir, "phase10_5e_cleanup_workdir_report.md"), []byte(md), 0644)

	fmt.Printf("Cleanup complete. Freed %s\n", humanSize(report.BytesToDelete))
	return nil
}
