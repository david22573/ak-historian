package app

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/david22573/ak-historian/internal/pitcoverage"
	"github.com/david22573/ak-historian/internal/universe"
	"github.com/spf13/cobra"
)

var (
	umOut                       string
	umUniverseID                string
	umUniverseName              string
	umSourceType                string
	umPolicy                    string
	umSymbols                   string
	umDataRoot                  string
	umQuoteAsset                string
	umMarketType                string
	umEffectiveStart            string
	umEffectiveEnd              string
	umIncludeDelisted           string
	umAssetLifecycleManifest    string
	umPitEvidenceCoverageReport string
)

var universeManifestCmd = &cobra.Command{
	Use:   "universe-manifest",
	Short: "Generate a universe_manifest.json",
	RunE: func(cmd *cobra.Command, args []string) error {
		var symbolsList []string
		if umSymbols != "" {
			symbolsList = strings.Split(umSymbols, ",")
		}

		// Use environment variable for Git SHA if available
		gitSha := os.Getenv("GIT_COMMIT_SHA")
		if gitSha == "" {
			gitSha = "unknown"
		}

		builder := &universe.Builder{
			UniverseID:                 umUniverseID,
			UniverseName:               umUniverseName,
			SourceRepo:                 "ak-historian",
			SourceGitSha:               gitSha,
			SourceType:                 umSourceType,
			EffectiveStartUTC:          umEffectiveStart,
			EffectiveEndUTC:            umEffectiveEnd,
			QuoteAsset:                 umQuoteAsset,
			MarketType:                 umMarketType,
			UniversePolicy:             umPolicy,
			IncludesDelistedAssets:     umIncludeDelisted,
			Symbols:                    symbolsList,
			DataRoot:                   umDataRoot,
			AssetLifecycleManifestPath: umAssetLifecycleManifest,
		}

		manifest, err := builder.Build()
		if err != nil {
			return err
		}

		if umPitEvidenceCoverageReport != "" {
			reportData, err := os.ReadFile(umPitEvidenceCoverageReport)
			if err != nil {
				return err
			}
			var pitReport pitcoverage.Report
			if err := json.Unmarshal(reportData, &pitReport); err != nil {
				return err
			}
			manifest.PointInTimeCoverageStatus = pitReport.OverallStatus
			manifest.PointInTimeCoverageHash = pitReport.Hashes.CoverageHash
			manifest.PointInTimePromotionRecommendation = pitReport.PromotionRecommendation
			for _, w := range pitReport.Warnings {
				manifest.Warnings = append(manifest.Warnings, universe.Warning{
					Code:    w.Code,
					Target:  w.TargetArtifact,
					Message: w.Reason,
				})
			}
			// Recompute hashes
			manifest.Hashes = universe.ComputeHashes(manifest)
		}

		outBytes, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return err
		}

		if umOut != "" {
			if err := os.WriteFile(umOut, outBytes, 0644); err != nil {
				return err
			}
			fmt.Printf("Wrote universe manifest to %s\n", umOut)
		} else {
			fmt.Println(string(outBytes))
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(universeManifestCmd)

	universeManifestCmd.Flags().StringVar(&umOut, "out", "", "Path to write universe_manifest.json")
	universeManifestCmd.Flags().StringVar(&umUniverseID, "universe-id", "default_universe", "Universe ID")
	universeManifestCmd.Flags().StringVar(&umUniverseName, "universe-name", "Default Universe", "Universe Name")
	universeManifestCmd.Flags().StringVar(&umSourceType, "source-type", "cli", "Source type")
	universeManifestCmd.Flags().StringVar(&umPolicy, "policy", universe.PolicyUnknown, "Universe policy")
	universeManifestCmd.Flags().StringVar(&umSymbols, "symbols", "", "Comma-separated symbols (optional)")
	universeManifestCmd.Flags().StringVar(&umDataRoot, "data-root", "", "Path to discover symbols from (optional)")
	universeManifestCmd.Flags().StringVar(&umQuoteAsset, "quote-asset", "USDT", "Quote asset")
	universeManifestCmd.Flags().StringVar(&umMarketType, "market-type", "futures", "Market type")
	universeManifestCmd.Flags().StringVar(&umEffectiveStart, "effective-start", "", "Effective start UTC (RFC3339)")
	universeManifestCmd.Flags().StringVar(&umEffectiveEnd, "effective-end", "", "Effective end UTC (RFC3339)")
	universeManifestCmd.Flags().StringVar(&umIncludeDelisted, "include-delisted", "unknown", "Includes delisted assets (true|false|unknown)")
	universeManifestCmd.Flags().StringVar(&umAssetLifecycleManifest, "asset-lifecycle-manifest", "", "Path to asset_lifecycle_manifest.json")
	universeManifestCmd.Flags().StringVar(&umPitEvidenceCoverageReport, "pit-evidence-coverage-report", "", "Path to pit_evidence_coverage_report.json")
}
