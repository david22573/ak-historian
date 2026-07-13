package app

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/david22573/ak-historian/internal/pitarchive"
	"github.com/spf13/cobra"
)

func newPITCommand() *cobra.Command {
	var manifestPath string
	var archiveRoot string
	var datasetID string
	var datasetVersion string
	var windowStart string
	var windowEnd string
	var cutoff string
	var evidenceOutput string
	var resultOutput string
	var diagnosticOnly bool
	var maxSnapshotBytes int64
	var historianBuild string

	command := &cobra.Command{
		Use:   "verify-pit",
		Short: "Verify point-in-time archive eligibility from concrete snapshot evidence",
		RunE: func(cmd *cobra.Command, args []string) error {
			parseTime := func(name, value string) (time.Time, error) {
				parsed, err := time.Parse(time.RFC3339, value)
				if err != nil {
					return time.Time{}, fmt.Errorf("invalid %s: expected RFC3339: %w", name, err)
				}
				return parsed, nil
			}
			start, err := parseTime("window-start", windowStart)
			if err != nil {
				return err
			}
			end, err := parseTime("window-end", windowEnd)
			if err != nil {
				return err
			}
			evaluationCutoff, err := parseTime("evaluation-cutoff", cutoff)
			if err != nil {
				return err
			}
			result, err := pitarchive.Evaluate(pitarchive.EvaluateOptions{
				ManifestPath: manifestPath, ArchiveRoot: archiveRoot, DatasetID: datasetID, DatasetVersion: datasetVersion,
				ResearchWindowStart: start, ResearchWindowEnd: end, EvaluationCutoff: evaluationCutoff,
				Strict: !diagnosticOnly, MaxSnapshotBytes: maxSnapshotBytes, HistorianBuild: historianBuild,
			})
			if err != nil {
				return err
			}
			if resultOutput != "" {
				if err := pitarchive.WriteEvaluationResult(resultOutput, result); err != nil {
					return err
				}
			}
			if evidenceOutput != "" {
				if result.Evidence == nil {
					return fmt.Errorf("authoritative evidence was not produced for verdict %s", result.Verdict)
				}
				if err := pitarchive.WriteEvidence(evidenceOutput, *result.Evidence); err != nil {
					return err
				}
			}
			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(result); err != nil {
				return fmt.Errorf("write PIT evaluation result: %w", err)
			}
			if !diagnosticOnly && !result.StrictPromotionAllowed {
				return fmt.Errorf("strict PIT evaluation failed with verdict %s", result.Verdict)
			}
			return nil
		},
	}
	command.Flags().StringVar(&manifestPath, "snapshot-manifest", "", "versioned snapshot manifest JSON (required in strict mode)")
	command.Flags().StringVar(&archiveRoot, "archive-root", "", "approved local archive root")
	command.Flags().StringVar(&datasetID, "dataset-id", "", "expected dataset identity")
	command.Flags().StringVar(&datasetVersion, "dataset-version", "", "expected dataset version")
	command.Flags().StringVar(&windowStart, "window-start", "", "research-window start (RFC3339, inclusive)")
	command.Flags().StringVar(&windowEnd, "window-end", "", "research-window end (RFC3339, exclusive)")
	command.Flags().StringVar(&cutoff, "evaluation-cutoff", "", "historical evaluation cutoff (RFC3339, inclusive)")
	command.Flags().StringVar(&evidenceOutput, "evidence-output", "", "atomic output path for integrity-hashed PIT evidence")
	command.Flags().StringVar(&resultOutput, "result-output", "", "atomic output path for the full evaluation result")
	command.Flags().BoolVar(&diagnosticOnly, "diagnostic-only", false, "report findings without permitting strict promotion")
	command.Flags().Int64Var(&maxSnapshotBytes, "max-snapshot-bytes", pitarchive.DefaultMaxSnapshotBytes, "maximum bytes read from any snapshot")
	command.Flags().StringVar(&historianBuild, "historian-build", "ak-historian/dev", "historian build or version identity")
	return command
}

func init() {
	rootCmd.AddCommand(newPITCommand())
}
