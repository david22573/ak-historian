package app

import (
	"fmt"
	"path/filepath"

	"github.com/david22573/ak-historian/internal/researchidentity"
	"github.com/spf13/cobra"
)

var (
	rimDataRoot               string
	rimEvidenceRoot           string
	rimOut                    string
	rimManifestID             string
	rimManifestVersion        string
	rimDatasetID              string
	rimDatasetVersion         string
	rimSourceArchiveID        string
	rimSourceArchivePath      string
	rimInstrumentUniverseID   string
	rimDatasetStartUTC        string
	rimDatasetEndUTC          string
	rimPointInTimeCutoffUTC   string
	rimAvailabilityPolicyPath string
	rimCoveragePolicyPath     string
	rimPITEvidenceID          string
	rimPITEvidenceVersion     string
)

var researchIdentityManifestCmd = &cobra.Command{
	Use:   "research-identity-manifest",
	Short: "Build strict research-only dataset, archive, PIT, and coverage identity",
	RunE: func(cmd *cobra.Command, args []string) error {
		if rimOut == "" {
			return fmt.Errorf("missing --out")
		}
		evidenceRoot := rimEvidenceRoot
		if evidenceRoot == "" {
			evidenceRoot = filepath.Dir(rimOut)
		}
		manifest, err := (researchidentity.Builder{}).Build(researchidentity.BuildOptions{
			DataRoot:               rimDataRoot,
			EvidenceRoot:           evidenceRoot,
			ManifestID:             rimManifestID,
			ManifestVersion:        rimManifestVersion,
			DatasetID:              rimDatasetID,
			DatasetVersion:         rimDatasetVersion,
			SourceArchiveID:        rimSourceArchiveID,
			SourceArchivePath:      rimSourceArchivePath,
			InstrumentUniverseID:   rimInstrumentUniverseID,
			DatasetStartUTC:        rimDatasetStartUTC,
			DatasetEndUTC:          rimDatasetEndUTC,
			PointInTimeCutoffUTC:   rimPointInTimeCutoffUTC,
			AvailabilityPolicyPath: rimAvailabilityPolicyPath,
			CoveragePolicyPath:     rimCoveragePolicyPath,
			PITEvidenceID:          rimPITEvidenceID,
			PITEvidenceVersion:     rimPITEvidenceVersion,
		})
		if err != nil {
			return fmt.Errorf("derive research identity manifest: %w", err)
		}
		if err := researchidentity.WriteManifest(rimOut, manifest); err != nil {
			return err
		}
		fmt.Printf("Wrote research-only identity manifest to %s\n", rimOut)
		return nil
	},
}

func init() {
	researchIdentityManifestCmd.Flags().StringVar(&rimDataRoot, "data-root", "", "Root containing the exact parquet dataset objects")
	researchIdentityManifestCmd.Flags().StringVar(&rimEvidenceRoot, "evidence-root", "", "Root containing the archive and policy records (defaults to --out directory)")
	researchIdentityManifestCmd.Flags().StringVar(&rimOut, "out", "", "Output research_identity_manifest.json path")
	researchIdentityManifestCmd.Flags().StringVar(&rimManifestID, "manifest-id", "", "Immutable manifest ID")
	researchIdentityManifestCmd.Flags().StringVar(&rimManifestVersion, "manifest-version", "", "Immutable manifest version")
	researchIdentityManifestCmd.Flags().StringVar(&rimDatasetID, "dataset-id", "", "Immutable dataset ID")
	researchIdentityManifestCmd.Flags().StringVar(&rimDatasetVersion, "dataset-version", "", "Immutable dataset version")
	researchIdentityManifestCmd.Flags().StringVar(&rimSourceArchiveID, "source-archive-id", "", "Immutable source archive ID")
	researchIdentityManifestCmd.Flags().StringVar(&rimSourceArchivePath, "source-archive", "", "Path to the authoritative source archive bytes")
	researchIdentityManifestCmd.Flags().StringVar(&rimInstrumentUniverseID, "instrument-universe-id", "", "Immutable instrument or universe ID")
	researchIdentityManifestCmd.Flags().StringVar(&rimDatasetStartUTC, "dataset-start", "", "Exact requested dataset window start in canonical UTC")
	researchIdentityManifestCmd.Flags().StringVar(&rimDatasetEndUTC, "dataset-end", "", "Exact requested dataset window end in canonical UTC")
	researchIdentityManifestCmd.Flags().StringVar(&rimPointInTimeCutoffUTC, "point-in-time-cutoff", "", "Exact evaluation availability cutoff in canonical UTC")
	researchIdentityManifestCmd.Flags().StringVar(&rimAvailabilityPolicyPath, "availability-policy", "", "Path to the strict availability policy record")
	researchIdentityManifestCmd.Flags().StringVar(&rimCoveragePolicyPath, "coverage-policy", "", "Path to the strict zero-defect coverage policy record")
	researchIdentityManifestCmd.Flags().StringVar(&rimPITEvidenceID, "pit-evidence-id", "", "Immutable PIT evidence ID")
	researchIdentityManifestCmd.Flags().StringVar(&rimPITEvidenceVersion, "pit-evidence-version", "", "Immutable PIT evidence version")
	rootCmd.AddCommand(researchIdentityManifestCmd)
}
