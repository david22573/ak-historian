package prospective

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func CreateActivation(repositoryRoot, dataRoot, output, sourceCommit, buildID string, now time.Time) (Activation, error) {
	var protocol Protocol
	if err := ReadStrict(filepath.Join(repositoryRoot, "authority", "pr4b0_r1p4_collection_protocol.json"), &protocol); err != nil {
		return Activation{}, err
	}
	if err := VerifyProtocol(protocol); err != nil {
		return Activation{}, err
	}
	var policy AvailabilityPolicy
	if err := ReadStrict(filepath.Join(repositoryRoot, "authority", "pr4b0_r1p4_availability_policy.json"), &policy); err != nil {
		return Activation{}, err
	}
	if err := VerifyAvailabilityPolicy(policy); err != nil {
		return Activation{}, err
	}
	if !validGitCommit(sourceCommit) || CollectorSourceCommit == "UNSET" || sourceCommit != CollectorSourceCommit {
		return Activation{}, errors.New("activation requires the exact embedded committed collector-source identity")
	}
	if !validHash(buildID) {
		return Activation{}, errors.New("collector build ID must be a SHA-256 identity")
	}
	head, err := gitOutput(repositoryRoot, "rev-parse", "HEAD")
	if err != nil || head != sourceCommit {
		return Activation{}, errors.New("repository HEAD is not the embedded collector-source commit")
	}
	if err := exec.Command("git", "-C", repositoryRoot, "diff", "--quiet").Run(); err != nil {
		return Activation{}, errors.New("tracked repository changes prevent activation identity creation")
	}
	activated := now.UTC()
	activation := Activation{SchemaVersion: ActivationVersion, DatasetID: protocol.DatasetID, Generation: "r1-activated-" + activated.Format("20060102T150405Z"), ActivationTimestamp: activated, CollectorSourceCommit: sourceCommit, CollectorBuildID: buildID, ProtocolHash: protocol.ProtocolHash, SourceSchemaVersion: SourceSchemaVersion, SourceSchemaFingerprint: SourceSchemaFingerprint, AvailabilityPolicyVersion: AvailabilityPolicyVersion, AvailabilityPolicyHash: policy.PolicyHash, CoveragePolicyVersion: CoveragePolicyVersion, IngestionReceiptVersion: ReceiptSchemaVersion, IngestionReceiptHash: ReceiptSchemaHash, ManifestContractVersion: ManifestContractVersion, ManifestContractHash: ManifestContractHash, UniqueSymbols: append([]string{}, UniqueSymbols...), Timeframe: "1m", CadenceSeconds: 300, PartitionPolicy: "UTC calendar day per symbol; immutable checkpoints bind sorted partition hashes", ReceiptLedgerGenesisHash: ZeroHash, CheckpointRule: "append-only checkpoint identity binds this activation hash, final receipt-chain hash, and sorted partition manifest identities/hashes"}
	hash, err := HashCanonical(activation, "activation_hash")
	if err != nil {
		return Activation{}, err
	}
	activation.ActivationHash = hash
	if err := VerifyActivation(activation, protocol, policy); err != nil {
		return Activation{}, err
	}
	data, _ := CanonicalJSON(activation)
	if err := WriteAtomic(output, data, 0o444); err != nil {
		return Activation{}, err
	}
	contractsPath := filepath.Join(dataRoot, "contracts", "dataset_activation.json")
	if filepath.Clean(output) != filepath.Clean(contractsPath) {
		if err := WriteAtomic(contractsPath, data, 0o444); err != nil {
			return Activation{}, err
		}
	}
	return activation, nil
}

func gitOutput(root string, args ...string) (string, error) {
	commandArgs := append([]string{"-C", root}, args...)
	output, err := exec.Command("git", commandArgs...).CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func ExecutableBuildID() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return HashBytes(data), nil
}

type VerificationSummary struct {
	SchemaVersion      string    `json:"schema_version"`
	VerifiedAtUTC      time.Time `json:"verified_at_utc"`
	ReceiptCount       int       `json:"receipt_count"`
	RawResponseCount   int       `json:"raw_response_count"`
	FragmentCount      int       `json:"normalized_fragment_count"`
	CycleCount         int       `json:"cycle_count"`
	FullSuccessCycles  int       `json:"full_success_cycle_count"`
	FinalEnvelopeHash  string    `json:"final_receipt_chain_hash"`
	FinalAuthorityHash string    `json:"final_authority_receipt_hash"`
	ConflictCount      int       `json:"conflict_count"`
	SchemaFailureCount int       `json:"schema_failure_count"`
	ClockErrorCount    int       `json:"clock_error_count"`
	Valid              bool      `json:"valid"`
}

func (store *Store) VerifyAll(now time.Time) (VerificationSummary, error) {
	state, receipts, err := store.RebuildState()
	if err != nil {
		return VerificationSummary{}, err
	}
	summary := VerificationSummary{SchemaVersion: "ak-historian.pr4b0-r1p4.verification-summary.v1", VerifiedAtUTC: now.UTC(), ReceiptCount: len(receipts), FinalEnvelopeHash: state.LastEnvelopeHash, FinalAuthorityHash: state.LastAuthorityHash, Valid: true}
	for _, receipt := range receipts {
		rawPath, err := store.absolute(receipt.RawRelativePath)
		if err != nil {
			return VerificationSummary{}, err
		}
		raw, err := os.ReadFile(rawPath)
		if err != nil || HashBytes(raw) != receipt.RawResponseSHA256 || len(raw) != receipt.ResponseBodyByteLength {
			return VerificationSummary{}, fmt.Errorf("raw response verification failed: %s", receipt.RawRelativePath)
		}
		summary.RawResponseCount++
		fragmentPath, err := store.absolute(receipt.FragmentRelativePath)
		if err != nil {
			return VerificationSummary{}, err
		}
		fragmentData, err := os.ReadFile(fragmentPath)
		if err != nil {
			return VerificationSummary{}, err
		}
		var fragment Fragment
		if err := StrictDecode(fragmentData, &fragment); err != nil {
			return VerificationSummary{}, err
		}
		if err := VerifyCanonicalHash(fragment, "fragment_hash", fragment.FragmentHash); err != nil || fragment.FragmentHash != receipt.FragmentHash {
			return VerificationSummary{}, fmt.Errorf("normalized fragment verification failed: %s", receipt.FragmentRelativePath)
		}
		summary.FragmentCount++
	}
	if err := ReadJSONLines(store.CycleLedgerPath(), func(line []byte) error {
		var cycle CycleResult
		if err := StrictDecode(line, &cycle); err != nil {
			return err
		}
		if err := VerifyCanonicalHash(cycle, "cycle_hash", cycle.CycleHash); err != nil {
			return err
		}
		summary.CycleCount++
		if cycle.FullUniverseSuccess {
			summary.FullSuccessCycles++
		}
		if !cycle.ClockEvidence.Synchronized {
			summary.ClockErrorCount++
		}
		for _, status := range cycle.Symbols {
			summary.ConflictCount += status.Conflicts
			summary.SchemaFailureCount += status.SchemaFailures
		}
		return nil
	}); err != nil {
		return VerificationSummary{}, err
	}
	return summary, nil
}

type HealthStatus struct {
	SchemaVersion       string            `json:"schema_version"`
	GeneratedAtUTC      time.Time         `json:"generated_at_utc"`
	ServiceState        string            `json:"service_state"`
	TimerState          string            `json:"timer_state"`
	LastSuccessfulCycle string            `json:"last_successful_cycle"`
	LastSuccessfulUTC   time.Time         `json:"last_successful_cycle_utc"`
	PerSymbolCursor     map[string]Cursor `json:"per_symbol_cursor"`
	ReceiptChainValid   bool              `json:"receipt_chain_valid"`
	OpenPartitions      int               `json:"open_partitions"`
	PhysicalGaps        int               `json:"physical_gap_count"`
	EvidenceGaps        int               `json:"evidence_gap_count"`
	Conflicts           int               `json:"conflict_count"`
	ClockEvidenceStatus string            `json:"clock_evidence_status"`
	ReceiptCount        int               `json:"receipt_count"`
}

func (store *Store) Health(now time.Time) (HealthStatus, error) {
	state, receipts, err := store.RebuildState()
	if err != nil {
		return HealthStatus{}, err
	}
	manifests, err := store.BuildPartitionManifests(now)
	if err != nil {
		return HealthStatus{}, err
	}
	status := HealthStatus{SchemaVersion: "ak-historian.pr4b0-r1p4.collection-health.v1", GeneratedAtUTC: now.UTC(), ServiceState: systemdState("ak-historian-prospective.service"), TimerState: systemdState("ak-historian-prospective.timer"), LastSuccessfulCycle: state.LastSuccessfulCycleID, LastSuccessfulUTC: state.LastSuccessfulCycleUTC, PerSymbolCursor: state.Cursors, ReceiptChainValid: true, ReceiptCount: len(receipts), ClockEvidenceStatus: "ACCEPTABLE"}
	for _, manifest := range manifests {
		if manifest.PartitionStatus == "PARTITION_OPEN" {
			status.OpenPartitions++
		}
		status.PhysicalGaps += len(manifest.MissingIntervals)
		status.Conflicts += manifest.ConflictCount
		if manifest.PITEvidenceStatus == "PIT_EVIDENCE_INCOMPLETE" {
			status.EvidenceGaps++
		}
	}
	if len(receipts) == 0 {
		status.ClockEvidenceStatus = "NO_RECEIPTS"
	}
	return status, nil
}

func systemdState(unit string) string {
	output, err := exec.Command("systemctl", "--user", "is-active", unit).CombinedOutput()
	state := strings.TrimSpace(string(output))
	if err != nil && state == "" {
		return "unavailable"
	}
	return state
}

func (store *Store) CreateCheckpoint(now time.Time) (DatasetCheckpoint, error) {
	state, receipts, err := store.RebuildState()
	if err != nil {
		return DatasetCheckpoint{}, err
	}
	manifests, err := store.BuildPartitionManifests(now)
	if err != nil {
		return DatasetCheckpoint{}, err
	}
	checkpoint := DatasetCheckpoint{SchemaVersion: CheckpointVersion, DatasetID: store.Activation.DatasetID, Generation: store.Activation.Generation, ActivationHash: store.Activation.ActivationHash, CreatedAtUTC: now.UTC(), ReceiptCount: len(receipts), FinalReceiptHash: state.LastEnvelopeHash, Eligible: len(receipts) > 0}
	for _, manifest := range manifests {
		path := store.relative("manifests", "partitions", "symbol="+manifest.Symbol, "date="+manifest.UTCDate+".json")
		checkpoint.PartitionManifests = append(checkpoint.PartitionManifests, path)
		checkpoint.PartitionHashes = append(checkpoint.PartitionHashes, manifest.PartitionHash)
		if manifest.PartitionStatus == "CONFLICT" || manifest.PhysicalStatus != "PHYSICAL_COMPLETE" || manifest.PITEvidenceStatus != "PIT_EVIDENCE_COMPLETE" {
			checkpoint.Eligible = false
			checkpoint.IneligibleReasons = append(checkpoint.IneligibleReasons, manifest.Symbol+"/"+manifest.UTCDate+":"+manifest.PartitionStatus)
		}
	}
	sort.Strings(checkpoint.PartitionManifests)
	sort.Strings(checkpoint.PartitionHashes)
	sort.Strings(checkpoint.IneligibleReasons)
	checkpoint.CheckpointHash, err = HashCanonical(checkpoint, "checkpoint_hash")
	if err != nil {
		return DatasetCheckpoint{}, err
	}
	data, _ := CanonicalJSON(checkpoint)
	name := "checkpoint-" + now.UTC().Format("20060102T150405Z") + ".json"
	if err := WriteAtomic(filepath.Join(store.Root, "manifests", "checkpoints", name), data, 0o444); err != nil {
		return DatasetCheckpoint{}, err
	}
	return checkpoint, nil
}

func InstallSupervisor(repositoryRoot, binaryPath, dataRoot, activationPath string) error {
	for _, path := range []string{repositoryRoot, binaryPath, dataRoot, activationPath} {
		if !filepath.IsAbs(path) || strings.ContainsAny(path, " \t\n\r\x00") {
			return errors.New("supervisor paths must be safe whitespace-free absolute paths")
		}
	}
	configRoot, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	unitDir := filepath.Join(configRoot, "systemd", "user")
	serviceTemplate, err := os.ReadFile(filepath.Join(repositoryRoot, "config", "systemd", "ak-historian-prospective.service.in"))
	if err != nil {
		return err
	}
	timerTemplate, err := os.ReadFile(filepath.Join(repositoryRoot, "config", "systemd", "ak-historian-prospective.timer.in"))
	if err != nil {
		return err
	}
	service := bytes.ReplaceAll(serviceTemplate, []byte("@BINARY@"), []byte(binaryPath))
	service = bytes.ReplaceAll(service, []byte("@REPOSITORY@"), []byte(repositoryRoot))
	service = bytes.ReplaceAll(service, []byte("@DATA_ROOT@"), []byte(dataRoot))
	service = bytes.ReplaceAll(service, []byte("@ACTIVATION@"), []byte(activationPath))
	if err := WriteAtomic(filepath.Join(unitDir, "ak-historian-prospective.service"), service, 0o644); err != nil {
		return err
	}
	if err := WriteAtomic(filepath.Join(unitDir, "ak-historian-prospective.timer"), timerTemplate, 0o644); err != nil {
		return err
	}
	if output, err := exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("systemd daemon-reload: %s: %w", strings.TrimSpace(string(output)), err)
	}
	if output, err := exec.Command("systemctl", "--user", "enable", "--now", "ak-historian-prospective.timer").CombinedOutput(); err != nil {
		return fmt.Errorf("systemd enable timer: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

func StopSupervisor() error {
	output, err := exec.Command("systemctl", "--user", "disable", "--now", "ak-historian-prospective.timer").CombinedOutput()
	if err != nil {
		return fmt.Errorf("stop supervisor: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

func UninstallSupervisor() error {
	_ = StopSupervisor()
	configRoot, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	for _, name := range []string{"ak-historian-prospective.service", "ak-historian-prospective.timer"} {
		if err := os.Remove(filepath.Join(configRoot, "systemd", "user", name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return exec.Command("systemctl", "--user", "daemon-reload").Run()
}

func SupervisorStatus(ctx context.Context) (map[string]string, error) {
	result := map[string]string{"service": systemdState("ak-historian-prospective.service"), "timer": systemdState("ak-historian-prospective.timer")}
	command := exec.CommandContext(ctx, "systemctl", "--user", "show", "ak-historian-prospective.timer", "--property=LoadState,ActiveState,SubState,NextElapseUSecRealtime,LastTriggerUSec", "--no-pager")
	output, err := command.CombinedOutput()
	result["details"] = strings.TrimSpace(string(output))
	return result, err
}
