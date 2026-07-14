package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/david22573/ak-historian/internal/prospective"
	"github.com/spf13/cobra"
)

type prospectiveFlags struct {
	repositoryRoot string
	dataRoot       string
	activationPath string
}

func defaultRepositoryRoot() string {
	root, err := os.Getwd()
	if err != nil {
		return "."
	}
	return root
}

func defaultDataRoot() string {
	if value := os.Getenv("XDG_STATE_HOME"); value != "" {
		return filepath.Join(value, "ak-historian", "prospective")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".ak-historian-prospective")
	}
	return filepath.Join(home, ".local", "state", "ak-historian", "prospective")
}

func addProspectiveFlags(command *cobra.Command, flags *prospectiveFlags) {
	command.Flags().StringVar(&flags.repositoryRoot, "repository-root", defaultRepositoryRoot(), "collector source repository root")
	command.Flags().StringVar(&flags.dataRoot, "data-root", defaultDataRoot(), "durable prospective data root")
	command.Flags().StringVar(&flags.activationPath, "activation", "", "immutable activation identity JSON")
}

func (flags prospectiveFlags) config() (prospective.Config, error) {
	activation := flags.activationPath
	if activation == "" {
		activation = filepath.Join(flags.dataRoot, "contracts", "dataset_activation.json")
	}
	return prospective.LoadConfig(flags.repositoryRoot, flags.dataRoot, activation)
}

func printJSON(command *cobra.Command, value any) error {
	encoder := json.NewEncoder(command.OutOrStdout())
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func newProspectiveActivateCommand() *cobra.Command {
	var flags prospectiveFlags
	var output string
	command := &cobra.Command{Use: "prospective-activate", Short: "Create an immutable prospective dataset activation after source commit", RunE: func(command *cobra.Command, args []string) error {
		if output == "" {
			output = filepath.Join(flags.dataRoot, "contracts", "dataset_activation.json")
		}
		buildID, err := prospective.ExecutableBuildID()
		if err != nil {
			return err
		}
		activation, err := prospective.CreateActivation(flags.repositoryRoot, flags.dataRoot, output, prospective.CollectorSourceCommit, buildID, time.Now().UTC())
		if err != nil {
			return err
		}
		return printJSON(command, activation)
	}}
	addProspectiveFlags(command, &flags)
	command.Flags().StringVar(&output, "output", "", "activation JSON output (defaults under data root)")
	return command
}

func newProspectiveCollectOnceCommand() *cobra.Command {
	var flags prospectiveFlags
	command := &cobra.Command{Use: "prospective-collect-once", Short: "Collect one bounded public nine-symbol prospective cycle", RunE: func(command *cobra.Command, args []string) error {
		config, err := flags.config()
		if err != nil {
			return err
		}
		collector, err := prospective.NewCollector(config)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(command.Context(), 4*time.Minute)
		defer cancel()
		cycle, err := collector.CollectOnce(ctx)
		if printErr := printJSON(command, cycle); printErr != nil {
			return printErr
		}
		return err
	}}
	addProspectiveFlags(command, &flags)
	return command
}

func newProspectiveStatusCommand() *cobra.Command {
	var flags prospectiveFlags
	command := &cobra.Command{Use: "prospective-status", Short: "Report collection health and structural coverage only", RunE: func(command *cobra.Command, args []string) error {
		config, err := flags.config()
		if err != nil {
			return err
		}
		returnStatus, err := prospective.NewStore(config.DataRoot, config.Activation).Health(time.Now().UTC())
		if err != nil {
			return err
		}
		return printJSON(command, returnStatus)
	}}
	addProspectiveFlags(command, &flags)
	return command
}

func newProspectiveVerifyCommand() *cobra.Command {
	var flags prospectiveFlags
	command := &cobra.Command{Use: "prospective-verify", Short: "Verify receipts, chains, raw responses, fragments, and manifests", RunE: func(command *cobra.Command, args []string) error {
		config, err := flags.config()
		if err != nil {
			return err
		}
		summary, err := prospective.NewStore(config.DataRoot, config.Activation).VerifyAll(time.Now().UTC())
		if err != nil {
			return err
		}
		return printJSON(command, summary)
	}}
	addProspectiveFlags(command, &flags)
	return command
}

func newProspectiveGapReportCommand() *cobra.Command {
	var flags prospectiveFlags
	command := &cobra.Command{Use: "prospective-gap-report", Short: "Report physical and availability-evidence gaps by symbol/day", RunE: func(command *cobra.Command, args []string) error {
		config, err := flags.config()
		if err != nil {
			return err
		}
		manifests, err := prospective.NewStore(config.DataRoot, config.Activation).BuildPartitionManifests(time.Now().UTC())
		if err != nil {
			return err
		}
		return printJSON(command, map[string]any{"schema_version": "ak-historian.pr4b0-r1p4.gap-report.v1", "generated_at_utc": time.Now().UTC(), "partitions": manifests})
	}}
	addProspectiveFlags(command, &flags)
	return command
}

func newProspectiveCheckpointCommand() *cobra.Command {
	var flags prospectiveFlags
	command := &cobra.Command{Use: "prospective-manifest-checkpoint", Short: "Cut an immutable append-only dataset checkpoint", RunE: func(command *cobra.Command, args []string) error {
		config, err := flags.config()
		if err != nil {
			return err
		}
		checkpoint, err := prospective.NewStore(config.DataRoot, config.Activation).CreateCheckpoint(time.Now().UTC())
		if err != nil {
			return err
		}
		return printJSON(command, checkpoint)
	}}
	addProspectiveFlags(command, &flags)
	return command
}

func newProspectiveSupervisorInstallCommand() *cobra.Command {
	var flags prospectiveFlags
	var binary string
	command := &cobra.Command{Use: "prospective-supervisor-install", Short: "Install and start the rootless five-minute systemd timer", RunE: func(command *cobra.Command, args []string) error {
		if binary == "" {
			var err error
			binary, err = os.Executable()
			if err != nil {
				return err
			}
		}
		activation := flags.activationPath
		if activation == "" {
			activation = filepath.Join(flags.dataRoot, "contracts", "dataset_activation.json")
		}
		if _, err := flags.config(); err != nil {
			return err
		}
		if err := prospective.InstallSupervisor(flags.repositoryRoot, binary, flags.dataRoot, activation); err != nil {
			return err
		}
		_, err := fmt.Fprintln(command.OutOrStdout(), "PROSPECTIVE_SUPERVISOR_INSTALLED")
		return err
	}}
	addProspectiveFlags(command, &flags)
	command.Flags().StringVar(&binary, "binary", "", "exact committed collector binary")
	return command
}

func newProspectiveSupervisorStatusCommand() *cobra.Command {
	return &cobra.Command{Use: "prospective-supervisor-status", Short: "Report live user supervisor status", RunE: func(command *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(command.Context(), 5*time.Second)
		defer cancel()
		status, err := prospective.SupervisorStatus(ctx)
		if printErr := printJSON(command, status); printErr != nil {
			return printErr
		}
		return err
	}}
}

func newProspectiveSupervisorStopCommand() *cobra.Command {
	return &cobra.Command{Use: "prospective-supervisor-stop", Short: "Stop and disable the user timer", RunE: func(command *cobra.Command, args []string) error {
		return prospective.StopSupervisor()
	}}
}

func newProspectiveSupervisorUninstallCommand() *cobra.Command {
	return &cobra.Command{Use: "prospective-supervisor-uninstall", Short: "Remove generated local user units", RunE: func(command *cobra.Command, args []string) error {
		return prospective.UninstallSupervisor()
	}}
}

func init() {
	rootCmd.AddCommand(
		newProspectiveActivateCommand(),
		newProspectiveCollectOnceCommand(),
		newProspectiveStatusCommand(),
		newProspectiveVerifyCommand(),
		newProspectiveGapReportCommand(),
		newProspectiveCheckpointCommand(),
		newProspectiveSupervisorInstallCommand(),
		newProspectiveSupervisorStatusCommand(),
		newProspectiveSupervisorStopCommand(),
		newProspectiveSupervisorUninstallCommand(),
	)
}
