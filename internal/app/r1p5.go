package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/david22573/ak-historian/internal/prospective"
	"github.com/david22573/ak-historian/internal/r1p5"
	"github.com/spf13/cobra"
)

type r1p5Flags struct{ repositoryRoot, dataRoot, liveDataRoot, activation string }

func addR1P5Flags(command *cobra.Command, flags *r1p5Flags) {
	command.Flags().StringVar(&flags.repositoryRoot, "repository-root", defaultRepositoryRoot(), "frozen Historian source root")
	command.Flags().StringVar(&flags.dataRoot, "data-root", filepath.Join(defaultDataRoot(), "pr4b0-r1p5-backfill"), "durable R1P5 backfill root")
	command.Flags().StringVar(&flags.liveDataRoot, "live-data-root", filepath.Join(defaultDataRoot(), "pr4b0-r1p4"), "durable P4 live root")
	command.Flags().StringVar(&flags.activation, "activation", "", "P4 activation JSON")
}

func (f r1p5Flags) config() (r1p5.Config, error) {
	if f.activation == "" {
		f.activation = filepath.Join(f.liveDataRoot, "contracts", "dataset_activation.json")
	}
	return r1p5.LoadConfig(f.repositoryRoot, f.dataRoot, f.liveDataRoot, f.activation)
}

func newBackfillCommand() *cobra.Command {
	var f r1p5Flags
	cmd := &cobra.Command{Use: "prospective-backfill", Short: "Run or resume the frozen authoritative R1P5 historical reacquisition", RunE: func(cmd *cobra.Command, args []string) error {
		config, err := f.config()
		if err != nil {
			return err
		}
		status, err := r1p5.NewCollector(config).CollectAll(cmd.Context())
		if printErr := printJSON(cmd, status); printErr != nil {
			return printErr
		}
		return err
	}}
	addR1P5Flags(cmd, &f)
	return cmd
}
func newBackfillStatusCommand() *cobra.Command {
	var f r1p5Flags
	cmd := &cobra.Command{Use: "prospective-backfill-status", Short: "Report bounded structural backfill cursors and counts", RunE: func(cmd *cobra.Command, args []string) error {
		config, err := f.config()
		if err != nil {
			return err
		}
		status, err := r1p5.NewCollector(config).Status()
		if err != nil {
			return err
		}
		return printJSON(cmd, status)
	}}
	addR1P5Flags(cmd, &f)
	return cmd
}
func newBackfillVerifyCommand() *cobra.Command {
	var f r1p5Flags
	cmd := &cobra.Command{Use: "prospective-backfill-verify", Short: "Verify the R1P5 receipt chain and all retained raw/fragment evidence", RunE: func(cmd *cobra.Command, args []string) error {
		config, err := f.config()
		if err != nil {
			return err
		}
		summary, err := r1p5.NewStore(config.DataRoot, config.Protocol, config.SourceIdentity).VerifyAll(time.Now().UTC())
		if err != nil {
			return err
		}
		return printJSON(cmd, summary)
	}}
	addR1P5Flags(cmd, &f)
	return cmd
}

type generatedEvidence struct {
	coverage   r1p5.Coverage
	ledger     r1p5.EligibilityLedger
	checkpoint r1p5.Checkpoint
	readiness  r1p5.Readiness
	backfill   r1p5.Verification
	live       prospective.VerificationSummary
}

func generateEvidence(config r1p5.Config, now time.Time) (generatedEvidence, error) {
	coverage, _, backfill, live, err := r1p5.BuildCoverage(config, now)
	if err != nil {
		return generatedEvidence{}, err
	}
	ledger, err := r1p5.BuildEligibilityLedger(config, coverage, now)
	if err != nil {
		return generatedEvidence{}, err
	}
	checkpoint, err := r1p5.CreateCheckpoint(config, coverage, backfill, live, ledger, now)
	if err != nil {
		return generatedEvidence{}, err
	}
	readiness := r1p5.BuildReadiness(config, coverage, checkpoint, backfill, live, now)
	return generatedEvidence{coverage: coverage, ledger: ledger, checkpoint: checkpoint, readiness: readiness, backfill: backfill, live: live}, nil
}

func sealed(value any) (map[string]any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, err
	}
	hash, err := prospective.HashCanonical(object, "artifact_hash")
	if err != nil {
		return nil, err
	}
	object["artifact_hash"] = hash
	return object, nil
}

func writeRequiredReports(config r1p5.Config, e generatedEvidence) error {
	status, _ := r1p5.NewCollector(config).Status()
	backfillReport, _ := sealed(map[string]any{"schema_version": "ak-historian.pr4b0-r1p5.backfill-summary.v1", "generated_at_utc": e.coverage.GeneratedAtUTC, "backfill_source_commit": config.SourceIdentity.SourceCommit, "protocol_hash": config.Protocol.ProtocolHash, "status": status, "verification": e.backfill})
	ledgerReport, _ := sealed(e.ledger)
	checkpointReport, _ := sealed(e.checkpoint)
	readinessReport, _ := sealed(e.readiness)
	health, _ := sealed(map[string]any{"schema_version": "ak-historian.pr4b0-r1p5.collection-health.v1", "generated_at_utc": e.coverage.GeneratedAtUTC, "live_receipts": e.live.ReceiptCount, "live_chain_terminal": e.live.FinalEnvelopeHash, "live_chain_valid": e.live.Valid, "backfill_chain_terminal": e.backfill.FinalChainHash, "backfill_chain_valid": e.backfill.Valid, "conflicts": e.coverage.ConflictCount, "schema_errors": e.coverage.SchemaFailureCount, "evidence_gaps": e.coverage.EvidenceGapCount, "clock_errors": e.coverage.ClockErrorCount, "live_timer_state": systemdUserState("ak-historian-prospective.timer"), "watch_timer_state": systemdUserState("ak-historian-readiness-watch.timer")})
	decision, _ := sealed(map[string]any{"schema_version": "ak-historian.pr4b0-r1p5.final-decision.v1", "generated_at_utc": e.coverage.GeneratedAtUTC, "label": e.readiness.Label, "checkpoint_hash": e.checkpoint.CheckpointHash, "complete_eligible_utc_days": e.coverage.CompleteEligibleDays, "research_boundary": map[string]any{"candidate_executions": 0, "candidate_calculations": 0, "candidate_partition_reads": 0, "holdout_reads": 0, "real_rif_state_created": false}})
	reports := []struct {
		name  string
		value any
		title string
	}{{"pr4b0_r1p5_backfill_summary", backfillReport, "R1P5 backfill summary"}, {"pr4b0_r1p5_exposure_eligibility_ledger", ledgerReport, "R1P5 exposure eligibility ledger"}, {"pr4b0_r1p5_coverage_checkpoint", checkpointReport, "R1P5 immutable coverage checkpoint"}, {"pr4b0_r1p5_readiness_status", readinessReport, "R1P5 structural readiness"}, {"pr4b0_r1p5_collection_health", health, "R1P5 collection health"}, {"pr4b0_r1p5_final_decision", decision, "R1P5 final decision"}}
	for _, report := range reports {
		markdown := fmt.Sprintf("# %s\n\nThe canonical machine-readable authority is `%s.json`. This report contains structural coverage and acquisition evidence only; no candidate implementation, result, partition, or RIF state was read or created.\n", report.title, report.name)
		if err := r1p5.WriteReportPair(config.RepositoryRoot, report.name, report.value, markdown); err != nil {
			return err
		}
	}
	return nil
}

func newEligibilityLedgerCommand() *cobra.Command {
	var f r1p5Flags
	cmd := &cobra.Command{Use: "prospective-eligibility-ledger", Short: "Build the exposure/acquisition eligibility ledger without candidate inspection", RunE: func(cmd *cobra.Command, args []string) error {
		config, err := f.config()
		if err != nil {
			return err
		}
		coverage, _, _, _, err := r1p5.BuildCoverage(config, time.Now().UTC())
		if err != nil {
			return err
		}
		ledger, err := r1p5.BuildEligibilityLedger(config, coverage, time.Now().UTC())
		if err != nil {
			return err
		}
		return printJSON(cmd, ledger)
	}}
	addR1P5Flags(cmd, &f)
	return cmd
}
func newCoverageCheckpointCommand() *cobra.Command {
	var f r1p5Flags
	cmd := &cobra.Command{Use: "prospective-coverage-checkpoint", Short: "Create a generation-named immutable joined-coverage checkpoint", RunE: func(cmd *cobra.Command, args []string) error {
		config, err := f.config()
		if err != nil {
			return err
		}
		e, err := generateEvidence(config, time.Now().UTC())
		if err != nil {
			return err
		}
		if err := writeRequiredReports(config, e); err != nil {
			return err
		}
		return printJSON(cmd, e.checkpoint)
	}}
	addR1P5Flags(cmd, &f)
	return cmd
}
func newReadinessCommand() *cobra.Command {
	var f r1p5Flags
	cmd := &cobra.Command{Use: "prospective-readiness", Short: "Report structural readiness only", RunE: func(cmd *cobra.Command, args []string) error {
		config, err := f.config()
		if err != nil {
			return err
		}
		e, err := generateEvidence(config, time.Now().UTC())
		if err != nil {
			return err
		}
		if err := writeRequiredReports(config, e); err != nil {
			return err
		}
		return printJSON(cmd, e.readiness)
	}}
	addR1P5Flags(cmd, &f)
	return cmd
}
func newReadinessWatchCommand() *cobra.Command {
	var f r1p5Flags
	cmd := &cobra.Command{Use: "prospective-readiness-watch", Short: "Run one bounded hourly structural readiness watch cycle", RunE: func(cmd *cobra.Command, args []string) error {
		config, err := f.config()
		if err != nil {
			return err
		}
		e, err := generateEvidence(config, time.Now().UTC())
		if err != nil {
			return err
		}
		if err := writeRequiredReports(config, e); err != nil {
			return err
		}
		return printJSON(cmd, e.readiness)
	}}
	addR1P5Flags(cmd, &f)
	return cmd
}

func systemdUserState(unit string) string {
	out, err := exec.Command("systemctl", "--user", "is-active", unit).CombinedOutput()
	state := strings.TrimSpace(string(out))
	if err != nil && state == "" {
		return "unavailable"
	}
	return state
}

func newReadinessWatchInstallCommand() *cobra.Command {
	var f r1p5Flags
	var binary string
	cmd := &cobra.Command{Use: "prospective-readiness-watch-install", Short: "Install and activate the hourly rootless readiness watcher", RunE: func(cmd *cobra.Command, args []string) error {
		config, err := f.config()
		if err != nil {
			return err
		}
		if binary == "" {
			binary, err = os.Executable()
			if err != nil {
				return err
			}
		}
		binary, err = filepath.Abs(binary)
		if err != nil {
			return err
		}
		unitDir := filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user")
		service := fmt.Sprintf("[Unit]\nDescription=AK Historian R1P5 structural readiness watch\nAfter=network-online.target\n\n[Service]\nType=oneshot\nExecStart=%s prospective-readiness-watch --repository-root %s --data-root %s --live-data-root %s --activation %s\nNoNewPrivileges=true\nPrivateTmp=true\n", binary, config.RepositoryRoot, config.DataRoot, config.LiveDataRoot, config.ActivationPath)
		timer := "[Unit]\nDescription=Run AK Historian R1P5 structural readiness watch hourly\n\n[Timer]\nOnCalendar=hourly\nPersistent=true\nRandomizedDelaySec=0\nUnit=ak-historian-readiness-watch.service\n\n[Install]\nWantedBy=timers.target\n"
		if err := prospective.WriteAtomic(filepath.Join(unitDir, "ak-historian-readiness-watch.service"), []byte(service), 0o644); err != nil {
			return err
		}
		if err := prospective.WriteAtomic(filepath.Join(unitDir, "ak-historian-readiness-watch.timer"), []byte(timer), 0o644); err != nil {
			return err
		}
		for _, args := range [][]string{{"--user", "daemon-reload"}, {"--user", "enable", "--now", "ak-historian-readiness-watch.timer"}, {"--user", "start", "ak-historian-readiness-watch.service"}} {
			if out, err := exec.CommandContext(context.Background(), "systemctl", args...).CombinedOutput(); err != nil {
				return fmt.Errorf("systemctl %v: %s: %w", args, strings.TrimSpace(string(out)), err)
			}
		}
		return printJSON(cmd, map[string]any{"installed": true, "timer_state": systemdUserState("ak-historian-readiness-watch.timer")})
	}}
	addR1P5Flags(cmd, &f)
	cmd.Flags().StringVar(&binary, "binary", "", "exact R1P5 binary")
	return cmd
}

func newSourceIdentityCommand() *cobra.Command {
	var repositoryRoot, sourceCommit string
	cmd := &cobra.Command{Use: "prospective-backfill-source-identity", Hidden: true, RunE: func(cmd *cobra.Command, args []string) error {
		if sourceCommit == "" {
			return errors.New("source commit required")
		}
		var protocol r1p5.Protocol
		if err := prospective.ReadStrict(filepath.Join(repositoryRoot, "authority", "pr4b0_r1p5_coverage_protocol.json"), &protocol); err != nil {
			return err
		}
		identity := r1p5.SourceIdentity{SchemaVersion: r1p5.SourceIdentityVersion, SourceCommit: sourceCommit, ProtocolHash: protocol.ProtocolHash, CreatedAtUTC: time.Now().UTC()}
		identity.IdentityHash, _ = prospective.HashCanonical(identity, "identity_hash")
		return printJSON(cmd, identity)
	}}
	cmd.Flags().StringVar(&repositoryRoot, "repository-root", defaultRepositoryRoot(), "repository root")
	cmd.Flags().StringVar(&sourceCommit, "source-commit", "", "exact source commit")
	return cmd
}

func init() {
	commands := []*cobra.Command{newBackfillCommand(), newBackfillStatusCommand(), newBackfillVerifyCommand(), newEligibilityLedgerCommand(), newCoverageCheckpointCommand(), newReadinessCommand(), newReadinessWatchCommand(), newReadinessWatchInstallCommand(), newSourceIdentityCommand()}
	sort.SliceStable(commands, func(i, j int) bool { return commands[i].Name() < commands[j].Name() })
	rootCmd.AddCommand(commands...)
}
