package app

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/david22573/ak-historian/internal/pitcoverage"
	"github.com/spf13/cobra"
)

var (
	pitLifecycleManifest string
	pitUniverseManifest  string
	pitDatasetManifest   string
	pitSnapshotManifest  string
	pitResearchStart     string
	pitResearchEnd       string
	pitOut               string
	pitMdOut             string
	pitStrict            bool
	pitAllowUnverified   bool
)

var pitEvidenceCoverageCmd = &cobra.Command{
	Use:   "pit-evidence-coverage",
	Short: "Generate a point-in-time evidence coverage report",
	RunE: func(cmd *cobra.Command, args []string) error {
		builder := &pitcoverage.Builder{
			LifecycleManifestPath: pitLifecycleManifest,
			UniverseManifestPath:  pitUniverseManifest,
			DatasetManifestPath:   pitDatasetManifest,
			SnapshotManifestPath:  pitSnapshotManifest,
			ResearchStartUTC:      pitResearchStart,
			ResearchEndUTC:        pitResearchEnd,
			Strict:                pitStrict,
			AllowUnverified:       pitAllowUnverified,
		}

		report, err := builder.Build()
		if err != nil && report == nil {
			return err
		}

		outBytes, marshalErr := json.MarshalIndent(report, "", "  ")
		if marshalErr != nil {
			return marshalErr
		}

		if pitOut != "" {
			if writeErr := os.WriteFile(pitOut, outBytes, 0644); writeErr != nil {
				return writeErr
			}
			fmt.Printf("Wrote PIT evidence coverage report to %s\n", pitOut)
		} else {
			fmt.Println(string(outBytes))
		}

		if pitMdOut != "" {
			// Generate MD report
			md := generateMdReport(report)
			if writeErr := os.WriteFile(pitMdOut, []byte(md), 0644); writeErr != nil {
				return writeErr
			}
			fmt.Printf("Wrote PIT evidence coverage MD report to %s\n", pitMdOut)
		}

		return err
	},
}

func generateMdReport(r *pitcoverage.Report) string {
	md := fmt.Sprintf("# Point-in-Time Evidence Coverage Report\n\n")
	md += fmt.Sprintf("- **Report ID:** %s\n", r.CoverageReportID)
	md += fmt.Sprintf("- **Generated At (UTC):** %s\n", r.GeneratedAtUTC)
	md += fmt.Sprintf("- **Research Window:** %s to %s\n", r.ResearchWindowStartUTC, r.ResearchWindowEndUTC)
	md += fmt.Sprintf("- **Overall Status:** %s\n", r.OverallStatus)
	md += fmt.Sprintf("- **Promotion Recommendation:** %s\n", r.PromotionRecommendation)
	md += fmt.Sprintf("- **Survivorship Bias Risk:** %s\n\n", r.SurvivorshipBiasRisk)

	md += "## Symbols\n\n"
	for _, s := range r.Symbols {
		md += fmt.Sprintf("### %s\n", s.Symbol)
		md += fmt.Sprintf("- **PIT Status:** %s\n", s.PointInTimeStatus)
		md += fmt.Sprintf("- **Lifecycle Evidence Level:** %s\n", s.EvidenceLevel)
		if len(s.PromotionBlockingReasons) > 0 {
			md += fmt.Sprintf("- **Blocking Reasons:** %v\n", s.PromotionBlockingReasons)
		}
		md += "\n"
	}
	return md
}

func init() {
	rootCmd.AddCommand(pitEvidenceCoverageCmd)

	pitEvidenceCoverageCmd.Flags().StringVar(&pitLifecycleManifest, "asset-lifecycle-manifest", "", "Path to asset_lifecycle_manifest.json")
	pitEvidenceCoverageCmd.Flags().StringVar(&pitUniverseManifest, "universe-manifest", "", "Path to universe_manifest.json")
	pitEvidenceCoverageCmd.Flags().StringVar(&pitDatasetManifest, "dataset-manifest", "", "Path to dataset_manifest.json (optional)")
	pitEvidenceCoverageCmd.Flags().StringVar(&pitSnapshotManifest, "exchange-snapshot-manifest", "", "Path to exchange_metadata_snapshot_manifest.json (optional)")
	pitEvidenceCoverageCmd.Flags().StringVar(&pitResearchStart, "research-start", "", "Research window start UTC")
	pitEvidenceCoverageCmd.Flags().StringVar(&pitResearchEnd, "research-end", "", "Research window end UTC")
	pitEvidenceCoverageCmd.Flags().StringVar(&pitOut, "out", "", "Path to write pit_evidence_coverage_report.json")
	pitEvidenceCoverageCmd.Flags().StringVar(&pitMdOut, "md-out", "", "Path to write pit_evidence_coverage_report.md (optional)")
	pitEvidenceCoverageCmd.Flags().BoolVar(&pitStrict, "strict", false, "Fail if PIT coverage is missing/weak")
	pitEvidenceCoverageCmd.Flags().BoolVar(&pitAllowUnverified, "allow-unverified", false, "Allow unverified backfill evidence without blocking promotion")
}
