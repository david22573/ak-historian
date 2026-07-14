package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/david22573/ak-historian/internal/pitarchive"
	"github.com/spf13/cobra"
)

func newPITGapCommand() *cobra.Command {
	var options pitarchive.GapBuildOptions
	var start, end, cutoff, generatedAt, symbols, contextSymbols, output string
	command := &cobra.Command{
		Use: "build-pit-gap-manifest", Short: "Hash an explicit archive scope and preserve missing PIT evidence without synthesizing it",
		RunE: func(cmd *cobra.Command, args []string) error {
			parse := func(name, value string) (time.Time, error) {
				parsed, err := time.Parse(time.RFC3339, value)
				if err != nil {
					return time.Time{}, fmt.Errorf("invalid %s: %w", name, err)
				}
				return parsed, nil
			}
			var err error
			if options.WindowStart, err = parse("window-start", start); err != nil {
				return err
			}
			if options.WindowEnd, err = parse("window-end", end); err != nil {
				return err
			}
			if options.EvaluationCutoff, err = parse("evaluation-cutoff", cutoff); err != nil {
				return err
			}
			if options.GeneratedAt, err = parse("generated-at", generatedAt); err != nil {
				return err
			}
			options.RequiredSymbols = splitCSV(symbols)
			options.RequiredContextSymbols = splitCSV(contextSymbols)
			options.SourceAvailability = map[string]time.Time{}
			manifest, err := pitarchive.BuildGapManifest(options)
			if err != nil {
				return err
			}
			if output != "" {
				if err := pitarchive.WriteGapManifest(output, manifest); err != nil {
					return err
				}
			}
			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			return encoder.Encode(manifest)
		},
	}
	flags := command.Flags()
	flags.StringVar(&options.ArchiveRoot, "archive-root", "", "approved archive root")
	flags.StringVar(&options.DatasetID, "dataset-id", "", "stable logical dataset ID")
	flags.StringVar(&options.ManifestID, "manifest-id", "", "stable gap-manifest ID")
	flags.StringVar(&options.CandidateID, "candidate-id", "", "candidate identity")
	flags.StringVar(&options.CandidateVersion, "candidate-version", "", "candidate version")
	flags.StringVar(&options.ImplementationHash, "implementation-hash", "", "candidate implementation SHA-256")
	flags.StringVar(&options.Market, "market", "futures-um", "market")
	flags.StringVar(&options.Interval, "interval", "1m", "interval")
	flags.StringVar(&start, "window-start", "", "UTC month-aligned start")
	flags.StringVar(&end, "window-end", "", "UTC month-aligned exclusive end")
	flags.StringVar(&cutoff, "evaluation-cutoff", "", "evaluation cutoff")
	flags.StringVar(&generatedAt, "generated-at", "", "explicit deterministic creation timestamp")
	flags.StringVar(&options.HistorianBuild, "historian-build", "", "Historian commit/build identity")
	flags.StringVar(&options.EventSchemaVersion, "event-schema-version", "legacy-unversioned", "source event schema version or explicit unversioned marker")
	flags.StringVar(&symbols, "symbols", "", "comma-separated required primary symbols")
	flags.StringVar(&contextSymbols, "context-symbols", "", "comma-separated required context symbols")
	flags.StringVar(&output, "output", "", "atomic JSON output path")
	flags.Int64Var(&options.MaxSnapshotBytes, "max-snapshot-bytes", pitarchive.DefaultMaxSnapshotBytes, "bounded per-snapshot read limit")
	return command
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func init() { rootCmd.AddCommand(newPITGapCommand()) }
