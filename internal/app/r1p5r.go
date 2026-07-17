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
	"github.com/david22573/ak-historian/internal/r1p5r"
	"github.com/spf13/cobra"
)

type r1p5rFlags struct{ repositoryRoot, dataRoot, liveDataRoot, activation string }

func addR1P5RFlags(command *cobra.Command, flags *r1p5rFlags) {
	command.Flags().StringVar(&flags.repositoryRoot, "repository-root", defaultRepositoryRoot(), "frozen Historian source root")
	command.Flags().StringVar(&flags.dataRoot, "data-root", filepath.Join(defaultDataRoot(), "pr4b0-r1p5r-backfill"), "durable R1P5R backfill root")
	command.Flags().StringVar(&flags.liveDataRoot, "live-data-root", filepath.Join(defaultDataRoot(), "pr4b0-r1p4"), "durable P4 live root")
	command.Flags().StringVar(&flags.activation, "activation", "", "P4 activation JSON")
}

func (f r1p5rFlags) config() (r1p5r.Config, error) {
	if f.activation == "" {
		f.activation = filepath.Join(f.liveDataRoot, "contracts", "dataset_activation.json")
	}
	return r1p5r.LoadConfig(f.repositoryRoot, f.dataRoot, f.liveDataRoot, f.activation)
}

func newR1P5RReacquireCommand() *cobra.Command {
	var f r1p5rFlags
	cmd := &cobra.Command{Use: "r1p5r-reacquire", Short: "Run or resume the frozen authoritative R1P5R historical reacquisition", RunE: func(cmd *cobra.Command, args []string) error {
		config, err := f.config()
		if err != nil {
			return err
		}
		status, err := r1p5r.NewCollector(config).CollectAll(cmd.Context())
		if printErr := printJSON(cmd, status); printErr != nil {
			return printErr
		}
		return err
	}}
	addR1P5RFlags(cmd, &f)
	return cmd
}
func newR1P5RStatusCommand() *cobra.Command {
	var f r1p5rFlags
	cmd := &cobra.Command{Use: "r1p5r-status", Short: "Report bounded structural backfill cursors and counts", RunE: func(cmd *cobra.Command, args []string) error {
		config, err := f.config()
		if err != nil {
			return err
		}
		status, err := r1p5r.NewCollector(config).Status()
		if err != nil {
			return err
		}
		return printJSON(cmd, status)
	}}
	addR1P5RFlags(cmd, &f)
	return cmd
}
func newR1P5RVerifyCommand() *cobra.Command {
	var f r1p5rFlags
	cmd := &cobra.Command{Use: "r1p5r-verify", Short: "Verify the R1P5R receipt chain and all retained raw/fragment evidence", RunE: func(cmd *cobra.Command, args []string) error {
		config, err := f.config()
		if err != nil {
			return err
		}
		summary, err := r1p5r.NewStore(config.DataRoot, config.Protocol, config.SourceIdentity, config.PreacquisitionSeal).VerifyAll(time.Now().UTC())
		if err != nil {
			return err
		}
		return printJSON(cmd, summary)
	}}
	addR1P5RFlags(cmd, &f)
	return cmd
}

type r1p5rGeneratedEvidence struct {
	coverage   r1p5r.Coverage
	ledger     r1p5r.EligibilityLedger
	checkpoint r1p5r.Checkpoint
	readiness  r1p5r.Readiness
	backfill   r1p5r.Verification
	live       prospective.VerificationSummary
}

func generateR1P5REvidence(config r1p5r.Config, now time.Time) (r1p5rGeneratedEvidence, error) {
	coverage, _, backfill, live, err := r1p5r.BuildCoverage(config, now)
	if err != nil {
		return r1p5rGeneratedEvidence{}, err
	}
	ledger, err := r1p5r.BuildEligibilityLedger(config, coverage, now)
	if err != nil {
		return r1p5rGeneratedEvidence{}, err
	}
	checkpoint, err := r1p5r.CreateCheckpoint(config, coverage, backfill, live, ledger, now)
	if err != nil {
		return r1p5rGeneratedEvidence{}, err
	}
	readiness := r1p5r.BuildReadiness(config, coverage, checkpoint, backfill, live, now)
	return r1p5rGeneratedEvidence{coverage: coverage, ledger: ledger, checkpoint: checkpoint, readiness: readiness, backfill: backfill, live: live}, nil
}

func r1p5rSealed(value any) (map[string]any, error) {
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

func writeR1P5RRequiredReports(config r1p5r.Config, e r1p5rGeneratedEvidence) error {
	status, _ := r1p5r.NewCollector(config).Status()
	backfillReport, _ := r1p5rSealed(map[string]any{"schema_version": "ak-historian.pr4b0-r1p5r.reacquisition-summary.v1", "generated_at_utc": e.coverage.GeneratedAtUTC, "repair_implementation_commit": config.SourceIdentity.RepairSourceCommit, "source_seal_commit": config.PreacquisitionSeal.SourceSealCommit, "sealed_binary_sha256": config.PreacquisitionSeal.BinarySHA256, "protocol_hash": config.Protocol.ProtocolHash, "abandoned_evidence_registry_hash": config.AbandonedRegistry.RegistryHash, "status": status, "verification": e.backfill})
	ledgerReport, _ := r1p5rSealed(e.ledger)
	checkpointReport, _ := r1p5rSealed(e.checkpoint)
	readinessReport, _ := r1p5rSealed(e.readiness)
	health, _ := r1p5rSealed(map[string]any{"schema_version": "ak-historian.pr4b0-r1p5r.collection-health.v1", "generated_at_utc": e.coverage.GeneratedAtUTC, "live_receipts": e.live.ReceiptCount, "live_chain_terminal": e.live.FinalEnvelopeHash, "live_chain_valid": e.live.Valid, "backfill_chain_terminal": e.backfill.FinalChainHash, "backfill_chain_valid": e.backfill.Valid, "conflicts": e.coverage.ConflictCount, "schema_errors": e.coverage.SchemaFailureCount, "evidence_gaps": e.coverage.EvidenceGapCount, "clock_errors": e.coverage.ClockErrorCount, "live_timer_state": r1p5rSystemdUserState("ak-historian-prospective.timer"), "watch_timer_state": r1p5rSystemdUserState("ak-historian-r1p5r-readiness-watch.timer")})
	decision, _ := r1p5rSealed(map[string]any{"schema_version": "ak-historian.pr4b0-r1p5r.final-decision.v1", "generated_at_utc": e.coverage.GeneratedAtUTC, "label": e.readiness.Label, "checkpoint_hash": e.checkpoint.CheckpointHash, "complete_eligible_utc_days": e.coverage.CompleteEligibleDays, "research_boundary": map[string]any{"candidate_executions": 0, "candidate_calculations": 0, "candidate_partition_reads": 0, "holdout_reads": 0, "real_rif_state_created": false}})
	reports := []struct {
		name  string
		value any
		title string
	}{{"pr4b0_r1p5r_reacquisition_summary", backfillReport, "R1P5R reacquisition summary"}, {"pr4b0_r1p5r_exposure_eligibility_ledger", ledgerReport, "R1P5R exposure eligibility ledger"}, {"pr4b0_r1p5r_coverage_checkpoint", checkpointReport, "R1P5R immutable coverage checkpoint"}, {"pr4b0_r1p5r_readiness_status", readinessReport, "R1P5R structural readiness"}, {"pr4b0_r1p5r_collection_health", health, "R1P5R collection health"}, {"pr4b0_r1p5r_final_decision", decision, "R1P5R final decision"}}
	for _, report := range reports {
		markdown := fmt.Sprintf("# %s\n\nThe canonical machine-readable authority is `%s.json`. This report contains structural coverage and acquisition evidence only; no candidate implementation, result, partition, or RIF state was read or created.\n", report.title, report.name)
		if err := r1p5r.WriteReportPair(config.RepositoryRoot, report.name, report.value, markdown); err != nil {
			return err
		}
	}
	return nil
}

func newR1P5REligibilityLedgerCommand() *cobra.Command {
	var f r1p5rFlags
	cmd := &cobra.Command{Use: "r1p5r-eligibility-ledger", Short: "Build the exposure/acquisition eligibility ledger without candidate inspection", RunE: func(cmd *cobra.Command, args []string) error {
		config, err := f.config()
		if err != nil {
			return err
		}
		coverage, _, _, _, err := r1p5r.BuildCoverage(config, time.Now().UTC())
		if err != nil {
			return err
		}
		ledger, err := r1p5r.BuildEligibilityLedger(config, coverage, time.Now().UTC())
		if err != nil {
			return err
		}
		return printJSON(cmd, ledger)
	}}
	addR1P5RFlags(cmd, &f)
	return cmd
}
func newR1P5RCheckpointCommand() *cobra.Command {
	var f r1p5rFlags
	cmd := &cobra.Command{Use: "r1p5r-checkpoint", Short: "Create a generation-named immutable joined-coverage checkpoint", RunE: func(cmd *cobra.Command, args []string) error {
		config, err := f.config()
		if err != nil {
			return err
		}
		e, err := generateR1P5REvidence(config, time.Now().UTC())
		if err != nil {
			return err
		}
		if err := writeR1P5RRequiredReports(config, e); err != nil {
			return err
		}
		return printJSON(cmd, e.checkpoint)
	}}
	addR1P5RFlags(cmd, &f)
	return cmd
}
func newR1P5RReadinessCommand() *cobra.Command {
	var f r1p5rFlags
	cmd := &cobra.Command{Use: "r1p5r-readiness", Short: "Report structural readiness only", RunE: func(cmd *cobra.Command, args []string) error {
		config, err := f.config()
		if err != nil {
			return err
		}
		readiness, err := r1p5r.ScanReadiness(config)
		if err != nil {
			return err
		}
		if err := writeR1P5RReadinessReports(config, readiness); err != nil {
			return err
		}
		return printJSON(cmd, readiness)
	}}
	addR1P5RFlags(cmd, &f)
	return cmd
}
func newR1P5RReadinessWatchCommand() *cobra.Command {
	var f r1p5rFlags
	cmd := &cobra.Command{Use: "r1p5r-readiness-watch", Short: "Run one bounded hourly structural readiness watch cycle", RunE: func(cmd *cobra.Command, args []string) error {
		config, err := f.config()
		if err != nil {
			return err
		}
		readiness, err := r1p5r.ScanReadiness(config)
		if err != nil {
			return err
		}
		if err := writeR1P5RReadinessReports(config, readiness); err != nil {
			return err
		}
		return printJSON(cmd, readiness)
	}}
	addR1P5RFlags(cmd, &f)
	return cmd
}

func writeR1P5RReadinessReports(config r1p5r.Config, readiness r1p5r.Readiness) error {
	readinessReport, err := r1p5rSealed(readiness)
	if err != nil {
		return err
	}
	decision, err := r1p5rSealed(map[string]any{"schema_version": "ak-historian.pr4b0-r1p5r.final-decision.v1", "generated_at_utc": readiness.GeneratedAtUTC, "label": readiness.Label, "checkpoint_hash": readiness.CheckpointHash, "complete_eligible_utc_days": readiness.CompleteEligibleDays, "research_boundary": map[string]any{"candidate_executions": 0, "candidate_calculations": 0, "candidate_partition_reads": 0, "holdout_reads": 0, "real_rif_state_created": false}})
	if err != nil {
		return err
	}
	if err := r1p5r.WriteReportPair(config.RepositoryRoot, "pr4b0_r1p5r_readiness_status", readinessReport, "# R1P5R structural readiness\n\nThe canonical machine-readable authority is `pr4b0_r1p5r_readiness_status.json`. This deterministic scan validates only structural acquisition evidence and creates no research or RIF state.\n"); err != nil {
		return err
	}
	return r1p5r.WriteReportPair(config.RepositoryRoot, "pr4b0_r1p5r_final_decision", decision, "# R1P5R final decision\n\nThe canonical machine-readable authority is `pr4b0_r1p5r_final_decision.json`. No candidate execution, calculation, partition read, holdout read, or real RIF state occurred.\n")
}

func r1p5rSystemdUserState(unit string) string {
	out, err := exec.Command("systemctl", "--user", "is-active", unit).CombinedOutput()
	state := strings.TrimSpace(string(out))
	if err != nil && state == "" {
		return "unavailable"
	}
	return state
}

func newR1P5RReadinessWatchInstallCommand() *cobra.Command {
	var f r1p5rFlags
	var binary string
	cmd := &cobra.Command{Use: "r1p5r-readiness-watch-install", Short: "Install and activate the hourly rootless readiness watcher", RunE: func(cmd *cobra.Command, args []string) error {
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
		if err := r1p5r.VerifyBinaryFile(binary, config.PreacquisitionSeal.BinarySHA256); err != nil {
			return err
		}
		unitDir := filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user")
		reportRoot := filepath.Join(config.RepositoryRoot, "runs", "reports")
		service := fmt.Sprintf("[Unit]\nDescription=AK Historian R1P5R structural readiness watch\nAfter=network-online.target\n\n[Service]\nType=oneshot\nExecStart=%s r1p5r-readiness-watch --repository-root %s --data-root %s --live-data-root %s --activation %s\nNoNewPrivileges=true\nPrivateTmp=true\nProtectSystem=strict\nProtectHome=read-only\nReadWritePaths=%s %s\nRestrictAddressFamilies=AF_UNIX\n", binary, config.RepositoryRoot, config.DataRoot, config.LiveDataRoot, config.ActivationPath, reportRoot, config.DataRoot)
		timer := "[Unit]\nDescription=Run AK Historian R1P5R structural readiness watch hourly\n\n[Timer]\nOnCalendar=hourly\nPersistent=true\nRandomizedDelaySec=0\nUnit=ak-historian-r1p5r-readiness-watch.service\n\n[Install]\nWantedBy=timers.target\n"
		if err := prospective.WriteAtomic(filepath.Join(unitDir, "ak-historian-r1p5r-readiness-watch.service"), []byte(service), 0o644); err != nil {
			return err
		}
		if err := prospective.WriteAtomic(filepath.Join(unitDir, "ak-historian-r1p5r-readiness-watch.timer"), []byte(timer), 0o644); err != nil {
			return err
		}
		for _, args := range [][]string{{"--user", "daemon-reload"}, {"--user", "enable", "--now", "ak-historian-r1p5r-readiness-watch.timer"}, {"--user", "start", "ak-historian-r1p5r-readiness-watch.service"}} {
			if out, err := exec.CommandContext(context.Background(), "systemctl", args...).CombinedOutput(); err != nil {
				return fmt.Errorf("systemctl %v: %s: %w", args, strings.TrimSpace(string(out)), err)
			}
		}
		return printJSON(cmd, map[string]any{"installed": true, "timer_state": r1p5rSystemdUserState("ak-historian-r1p5r-readiness-watch.timer")})
	}}
	addR1P5RFlags(cmd, &f)
	cmd.Flags().StringVar(&binary, "binary", "", "exact R1P5R binary")
	return cmd
}

func newR1P5RSourceIdentityCommand() *cobra.Command {
	var repositoryRoot, sourceCommit string
	cmd := &cobra.Command{Use: "r1p5r-source-identity", Hidden: true, RunE: func(cmd *cobra.Command, args []string) error {
		if sourceCommit == "" {
			return errors.New("source commit required")
		}
		var protocol r1p5r.Protocol
		if err := prospective.ReadStrict(filepath.Join(repositoryRoot, "authority", "pr4b0_r1p5r_reacquisition_protocol.json"), &protocol); err != nil {
			return err
		}
		var registry r1p5r.AbandonedEvidenceRegistry
		if err := prospective.ReadStrict(filepath.Join(repositoryRoot, filepath.FromSlash(protocol.AbandonedRegistryPath)), &registry); err != nil {
			return err
		}
		identity := r1p5r.SourceIdentity{SchemaVersion: r1p5r.SourceIdentityVersion, RepairSourceCommit: sourceCommit, ProtocolHash: protocol.ProtocolHash, AbandonedRegistryHash: registry.RegistryHash, CreatedAtUTC: time.Now().UTC()}
		identity.IdentityHash, _ = prospective.HashCanonical(identity, "identity_hash")
		return printJSON(cmd, identity)
	}}
	cmd.Flags().StringVar(&repositoryRoot, "repository-root", defaultRepositoryRoot(), "repository root")
	cmd.Flags().StringVar(&sourceCommit, "source-commit", "", "exact source commit")
	return cmd
}

func init() {
	commands := []*cobra.Command{newR1P5RReacquireCommand(), newR1P5RStatusCommand(), newR1P5RVerifyCommand(), newR1P5REligibilityLedgerCommand(), newR1P5RCheckpointCommand(), newR1P5RReadinessCommand(), newR1P5RReadinessWatchCommand(), newR1P5RReadinessWatchInstallCommand(), newR1P5RSourceIdentityCommand()}
	sort.SliceStable(commands, func(i, j int) bool { return commands[i].Name() < commands[j].Name() })
	rootCmd.AddCommand(commands...)
}
