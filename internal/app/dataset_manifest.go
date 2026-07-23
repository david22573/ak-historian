package app

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/david22573/ak-historian/internal/manifest"
	"github.com/david22573/ak-historian/internal/pitcoverage"
	"github.com/spf13/cobra"
)

var (
	dmDataRoot                  string
	dmOut                       string
	dmDatasetID                 string
	dmDatasetRole               string
	dmSourceType                string
	dmSymbols                   string
	dmIntervals                 string
	dmIncCoverage               bool
	dmCovMode                   string
	dmCovStart                  string
	dmCovEnd                    string
	dmCovMinPct                 float64
	dmUniverseManifest          string
	dmPitEvidenceCoverageReport string
)

var datasetManifestCmd = &cobra.Command{
	Use:   "dataset-manifest",
	Short: "Generate a dataset_manifest.json for a dataset",
	RunE: func(cmd *cobra.Command, args []string) error {
		if dmDataRoot == "" {
			return fmt.Errorf("missing --data-root")
		}
		if dmOut == "" {
			return fmt.Errorf("missing --out")
		}
		if dmDatasetID == "" {
			return fmt.Errorf("missing --dataset-id")
		}
		if dmDatasetRole == "" {
			return fmt.Errorf("missing --dataset-role")
		}
		if dmSourceType == "" {
			return fmt.Errorf("missing --source-type")
		}

		symbols := []string{}
		if dmSymbols != "" {
			symbols = strings.Split(dmSymbols, ",")
		}
		intervals := []string{}
		if dmIntervals != "" {
			intervals = strings.Split(dmIntervals, ",")
		}

		// Try to get git sha if not provided
		gitSHA := "unknown"
		if out, err := getGitSHA(); err == nil {
			gitSHA = out
		}

		builder := &manifest.Builder{
			DataRoot:             dmDataRoot,
			DatasetID:            dmDatasetID,
			DatasetRole:          dmDatasetRole,
			SourceRepo:           "ak-historian",
			SourceGitSHA:         gitSHA,
			SourceType:           dmSourceType,
			Symbols:              symbols,
			Intervals:            intervals,
			IncludeCoverage:      dmIncCoverage,
			CoverageMode:         dmCovMode,
			CoverageStart:        dmCovStart,
			CoverageEnd:          dmCovEnd,
			CoverageMinPct:       dmCovMinPct,
			UniverseManifestPath: dmUniverseManifest,
		}

		m, err := builder.Build()
		if err != nil {
			return fmt.Errorf("failed to build manifest: %w", err)
		}

		if dmPitEvidenceCoverageReport != "" {
			reportData, err := os.ReadFile(dmPitEvidenceCoverageReport)
			if err != nil {
				return err
			}
			var pitReport pitcoverage.Report
			if err := json.Unmarshal(reportData, &pitReport); err != nil {
				return err
			}
			m.Survivorship.PointInTimeCoverageStatus = pitReport.OverallStatus
			m.Survivorship.PointInTimeCoverageHash = pitReport.Hashes.CoverageHash
			m.Survivorship.PointInTimePromotionRecommendation = pitReport.PromotionRecommendation
			m.Survivorship.SurvivorshipBiasRisk = pitReport.SurvivorshipBiasRisk
			for _, w := range pitReport.Warnings {
				m.Survivorship.Warnings = append(m.Survivorship.Warnings, w.Reason)
			}
			h, _ := m.ComputeHash()
			m.Hashes.ManifestHash = h
		}

		b, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal manifest: %w", err)
		}

		if err := os.WriteFile(dmOut, b, 0644); err != nil {
			return fmt.Errorf("failed to write manifest to %s: %w", dmOut, err)
		}

		fmt.Printf("Successfully generated dataset manifest at %s\n", dmOut)
		return nil
	},
}

func init() {
	datasetManifestCmd.Flags().StringVar(&dmDataRoot, "data-root", "", "Path to the root of the dataset")
	datasetManifestCmd.Flags().StringVar(&dmOut, "out", "", "Path to write dataset_manifest.json")
	datasetManifestCmd.Flags().StringVar(&dmDatasetID, "dataset-id", "", "Dataset ID")
	datasetManifestCmd.Flags().StringVar(&dmDatasetRole, "dataset-role", "", "Dataset Role (e.g. candles)")
	datasetManifestCmd.Flags().StringVar(&dmSourceType, "source-type", "", "Source type (e.g. local-parquet)")
	datasetManifestCmd.Flags().StringVar(&dmSymbols, "symbols", "", "Comma-separated symbols")
	datasetManifestCmd.Flags().StringVar(&dmIntervals, "intervals", "", "Comma-separated intervals")
	datasetManifestCmd.Flags().BoolVar(&dmIncCoverage, "include-coverage", false, "Include coverage data")
	datasetManifestCmd.Flags().StringVar(&dmCovMode, "coverage-mode", "fast", "Coverage mode (fast|strict)")
	datasetManifestCmd.Flags().StringVar(&dmCovStart, "coverage-start", "", "Coverage start time")
	datasetManifestCmd.Flags().StringVar(&dmCovEnd, "coverage-end", "", "Coverage end time")
	datasetManifestCmd.Flags().Float64Var(&dmCovMinPct, "coverage-min-pct", 99.0, "Coverage minimum percentage")
	datasetManifestCmd.Flags().StringVar(&dmUniverseManifest, "universe-manifest", "", "Path to universe_manifest.json")
	datasetManifestCmd.Flags().StringVar(&dmPitEvidenceCoverageReport, "pit-evidence-coverage-report", "", "Path to pit_evidence_coverage_report.json")
	rootCmd.AddCommand(datasetManifestCmd)
}

func getGitSHA() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "unknown", err
	}
	return strings.TrimSpace(string(out)), nil
}
