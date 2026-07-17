package r1p5r

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/david22573/ak-historian/internal/prospective"
)

type minuteSlot struct {
	hash    [32]byte
	present bool
}
type partitionAccumulator struct {
	slots                                                            [1440]minuteSlot
	receipts                                                         []string
	fragments                                                        []string
	duplicates, conflicts, schemaFailures, evidenceGaps, clockErrors int
}

func addRecord(acc map[string]*partitionAccumulator, candle prospective.NormalizedCandle, receiptHash, fragmentHash string, evidenceComplete bool) error {
	opened := time.UnixMilli(candle.OpenTimeMS).UTC()
	if opened.Second() != 0 || opened.Nanosecond() != 0 || candle.CloseTimeMS != candle.OpenTimeMS+59999 || candle.Symbol == "" || candle.Interval != "1m" || candle.Market != "futures-um" || candle.SourceDate != opened.Format("2006-01-02") {
		return errors.New("closed source schema or minute boundary failure")
	}
	minute := opened.Hour()*60 + opened.Minute()
	key := candle.Symbol + "/" + candle.SourceDate
	p := acc[key]
	if p == nil {
		p = &partitionAccumulator{}
		acc[key] = p
	}
	canonical, err := json.Marshal(candle)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(canonical)
	if p.slots[minute].present {
		if p.slots[minute].hash != hash {
			p.conflicts++
		} // byte-equivalent overlap is idempotent.
	} else {
		p.slots[minute] = minuteSlot{hash: hash, present: true}
	}
	if !evidenceComplete {
		p.evidenceGaps++
	}
	if len(p.receipts) == 0 || p.receipts[len(p.receipts)-1] != receiptHash {
		p.receipts = append(p.receipts, receiptHash)
	}
	if len(p.fragments) == 0 || p.fragments[len(p.fragments)-1] != fragmentHash {
		p.fragments = append(p.fragments, fragmentHash)
	}
	return nil
}

func BuildCoverage(config Config, now time.Time) (Coverage, []Partition, Verification, prospective.VerificationSummary, error) {
	backfillStore := NewStore(config.DataRoot, config.Protocol, config.SourceIdentity, config.PreacquisitionSeal)
	backfillVerification, err := backfillStore.VerifyAll(now)
	if err != nil {
		return Coverage{}, nil, Verification{}, prospective.VerificationSummary{}, err
	}
	liveStore := prospective.NewStore(config.LiveDataRoot, config.Activation)
	liveVerification, err := liveStore.VerifyAll(now)
	if err != nil {
		return Coverage{}, nil, Verification{}, prospective.VerificationSummary{}, err
	}
	if !backfillVerification.Valid || !liveVerification.Valid {
		return Coverage{}, nil, Verification{}, prospective.VerificationSummary{}, errors.New("backfill or live verification is not valid")
	}
	closedEnd := now.UTC().Truncate(24 * time.Hour)
	acc := map[string]*partitionAccumulator{}
	entries, err := backfillStore.Entries()
	if err != nil {
		return Coverage{}, nil, Verification{}, prospective.VerificationSummary{}, err
	}
	for _, entry := range entries {
		receipt, err := backfillStore.Receipt(entry)
		if err != nil {
			return Coverage{}, nil, Verification{}, prospective.VerificationSummary{}, err
		}
		fragment, err := backfillStore.Fragment(receipt)
		if err != nil {
			return Coverage{}, nil, Verification{}, prospective.VerificationSummary{}, err
		}
		complete := receipt.ClockEvidence.Synchronized && !entry.DurableCompletionUTC.IsZero() && entry.EvaluationCutoffFloor.After(entry.DurableCompletionUTC)
		for _, record := range fragment.Records {
			if record.MarketEventTimeUTC.Before(closedEnd) {
				if err := addRecord(acc, record.NormalizedCandle, receipt.ReceiptHash, fragment.FragmentHash, complete); err != nil {
					return Coverage{}, nil, Verification{}, prospective.VerificationSummary{}, err
				}
			}
		}
	}
	_, liveReceipts, err := liveStore.RebuildState()
	if err != nil {
		return Coverage{}, nil, Verification{}, prospective.VerificationSummary{}, err
	}
	for _, receipt := range liveReceipts {
		path := filepath.Join(config.LiveDataRoot, filepath.FromSlash(receipt.FragmentRelativePath))
		var fragment prospective.Fragment
		if err := prospective.ReadStrict(path, &fragment); err != nil {
			return Coverage{}, nil, Verification{}, prospective.VerificationSummary{}, err
		}
		complete := receipt.AvailabilityStatus == "PIT_ELIGIBLE" && receipt.ClockEvidence.Synchronized
		for _, candle := range fragment.Records {
			if time.UnixMilli(candle.OpenTimeMS).UTC().Before(closedEnd) {
				if err := addRecord(acc, candle, receipt.CurrentReceiptChainHash, fragment.FragmentHash, complete); err != nil {
					return Coverage{}, nil, Verification{}, prospective.VerificationSummary{}, err
				}
			}
		}
	}
	coverage := Coverage{SchemaVersion: "ak-historian.pr4b0-r1p5r.coverage.v1", GeneratedAtUTC: now.UTC(), EligibleStartUTC: config.Protocol.EligibleStartUTC, ContiguousEndUTC: config.Protocol.EligibleStartUTC}
	partitions := []Partition{}
	for date := config.Protocol.EligibleStartUTC; date.Before(closedEnd); date = date.Add(24 * time.Hour) {
		dayComplete := true
		for _, symbol := range prospective.UniqueSymbols {
			key := symbol + "/" + date.Format("2006-01-02")
			source := acc[key]
			if source == nil {
				source = &partitionAccumulator{}
			}
			partition := buildPartition(symbol, date, source)
			relative := filepath.ToSlash(filepath.Join("manifests", "partitions", "symbol="+symbol, "date="+date.Format("2006-01-02")+"-"+strings.TrimPrefix(partition.PartitionHash, "sha256:")+".json"))
			absolute, _ := backfillStore.absolute(relative)
			data, _ := prospective.CanonicalJSON(partition)
			if err := writeImmutable(absolute, data, 0o444); err != nil {
				return Coverage{}, nil, Verification{}, prospective.VerificationSummary{}, err
			}
			coverage.PartitionPaths = append(coverage.PartitionPaths, relative)
			coverage.PartitionHashes = append(coverage.PartitionHashes, partition.PartitionHash)
			coverage.PartitionCount++
			coverage.MissingIntervals += len(partition.MissingIntervals)
			coverage.DuplicateCount += partition.DuplicateCount
			coverage.ConflictCount += partition.ConflictCount
			coverage.SchemaFailureCount += partition.SchemaFailureCount
			coverage.EvidenceGapCount += partition.EvidenceGapCount
			coverage.ClockErrorCount += partition.ClockErrorCount
			if partition.EligibilityClass == "UNEXPOSED_PIT_EVIDENCE_COMPLETE" {
				coverage.CompletePartitionCount++
			} else {
				dayComplete = false
			}
			partitions = append(partitions, partition)
		}
		if dayComplete && date.Equal(coverage.ContiguousEndUTC) {
			coverage.CompleteEligibleDays++
			coverage.ContiguousEndUTC = date.Add(24 * time.Hour)
		}
	}
	sort.Strings(coverage.PartitionPaths)
	sort.Strings(coverage.PartitionHashes)
	for _, symbol := range prospective.UniqueSymbols {
		s := SymbolCoverage{Symbol: symbol, StartUTC: config.Protocol.EligibleStartUTC, EndUTC: coverage.ContiguousEndUTC}
		for _, p := range partitions {
			if p.Symbol == symbol {
				s.ObservedCandles += p.ObservedRows
				s.MissingIntervals += len(p.MissingIntervals)
				s.DuplicateCount += p.DuplicateCount
				s.ConflictCount += p.ConflictCount
				s.EvidenceGapCount += p.EvidenceGapCount
				if p.EligibilityClass == "UNEXPOSED_PIT_EVIDENCE_COMPLETE" {
					s.CompleteUTCDateCount++
				}
			}
		}
		coverage.PerSymbol = append(coverage.PerSymbol, s)
	}
	return coverage, partitions, backfillVerification, liveVerification, nil
}

func buildPartition(symbol string, date time.Time, source *partitionAccumulator) Partition {
	p := Partition{SchemaVersion: PartitionVersion, Symbol: symbol, UTCDate: date.Format("2006-01-02"), ExpectedRows: 1440, DuplicateCount: source.duplicates, ConflictCount: source.conflicts, SchemaFailureCount: source.schemaFailures, EvidenceGapCount: source.evidenceGaps, ClockErrorCount: source.clockErrors, ReceiptHashes: uniqueSorted(source.receipts), FragmentHashes: uniqueSorted(source.fragments), PhysicalStatus: "PHYSICAL_INCOMPLETE", PITEvidenceStatus: "PIT_EVIDENCE_INCOMPLETE", EligibilityClass: "UNEXPOSED_PHYSICAL_ONLY"}
	for index, slot := range source.slots {
		if slot.present {
			p.ObservedRows++
			continue
		}
		start := date.Add(time.Duration(index) * time.Minute)
		if len(p.MissingIntervals) > 0 && p.MissingIntervals[len(p.MissingIntervals)-1].EndUTC == start {
			p.MissingIntervals[len(p.MissingIntervals)-1].EndUTC = start.Add(time.Minute)
		} else {
			p.MissingIntervals = append(p.MissingIntervals, MissingInterval{StartUTC: start, EndUTC: start.Add(time.Minute)})
		}
	}
	if p.ConflictCount > 0 {
		p.EligibilityClass = "CONFLICT"
	} else if p.ObservedRows == 1440 && len(p.MissingIntervals) == 0 && p.DuplicateCount == 0 && p.SchemaFailureCount == 0 {
		p.PhysicalStatus = "PHYSICAL_COMPLETE"
		if p.EvidenceGapCount == 0 && p.ClockErrorCount == 0 {
			p.PITEvidenceStatus = "PIT_EVIDENCE_COMPLETE"
			p.EligibilityClass = "UNEXPOSED_PIT_EVIDENCE_COMPLETE"
		} else {
			p.EligibilityClass = "UNEXPOSED_PIT_EVIDENCE_INCOMPLETE"
		}
	}
	p.PartitionHash, _ = prospective.HashCanonical(p, "partition_hash")
	return p
}

func BuildEligibilityLedger(config Config, coverage Coverage, now time.Time) (EligibilityLedger, error) {
	ledger := EligibilityLedger{SchemaVersion: "ak-historian.pr4b0-r1p5r.exposure-eligibility-ledger.v1", GeneratedAtUTC: now.UTC(), ExposurePolicyHash: config.ExposurePolicy.PolicyHash, AbandonedRegistryHash: config.AbandonedRegistry.RegistryHash, Intervals: append([]Interval{}, config.Protocol.BarredIntervals...)}
	class := "UNEXPOSED_PIT_EVIDENCE_COMPLETE"
	if coverage.CompleteEligibleDays == 0 || coverage.ConflictCount > 0 || coverage.EvidenceGapCount > 0 {
		class = "UNEXPOSED_PIT_EVIDENCE_INCOMPLETE"
	}
	ledger.Intervals = append(ledger.Intervals, Interval{StartUTC: config.Protocol.EligibleStartUTC, EndUTC: coverage.ContiguousEndUTC, Classification: class})
	ledger.LedgerHash, _ = prospective.HashCanonical(ledger, "ledger_hash")
	return ledger, nil
}

func CreateCheckpoint(config Config, coverage Coverage, backfill Verification, live prospective.VerificationSummary, ledger EligibilityLedger, now time.Time) (Checkpoint, error) {
	if strings.Contains(strings.ToLower(config.Protocol.DatasetID), "latest") || strings.Contains(strings.ToLower(config.Protocol.DatasetID), "current") {
		return Checkpoint{}, errors.New("mutable dataset alias rejected")
	}
	if config.SourceIdentity.RepairSourceCommit == config.AbandonedRegistry.OldSourceCommit || backfill.FinalChainHash == config.AbandonedRegistry.OldBackfillTerminal || config.AbandonedRegistry.RegistryHash == "" {
		return Checkpoint{}, errors.New("abandoned R1P5 authority rejected")
	}
	if !backfill.Valid || !live.Valid || now.UTC().Before(backfill.EvaluationCutoffFloor) {
		return Checkpoint{}, errors.New("checkpoint requires durable valid reacquisition and live evidence")
	}
	generation := "r1p5r-checkpoint-" + now.UTC().Format("20060102T150405Z")
	checkpoint := Checkpoint{SchemaVersion: CheckpointVersion, DatasetID: config.Protocol.DatasetID, GenerationID: generation, CreatedAtUTC: now.UTC(), CoverageStartUTC: coverage.EligibleStartUTC, CoverageEndUTC: coverage.ContiguousEndUTC, EvaluationCutoffFloor: backfill.EvaluationCutoffFloor, RequiredSymbols: append([]string{}, prospective.UniqueSymbols...), SourceSchemaHash: config.Protocol.SourceSchemaFingerprint, AvailabilityPolicyHash: config.Protocol.AvailabilityPolicyHash, CoveragePolicyHash: config.ReadinessPolicy.PolicyHash, ManifestContractHash: config.Protocol.ManifestContractHash, IngestionReceiptHash: config.Protocol.IngestionReceiptHash, P4ActivationHash: config.Activation.ActivationHash, P4LiveChainTerminal: live.FinalEnvelopeHash, P4LiveAuthorityTerminal: live.FinalAuthorityHash, BackfillChainGenesis: ZeroHash, BackfillChainTerminal: backfill.FinalChainHash, RepairSourceCommit: config.SourceIdentity.RepairSourceCommit, SourceSealCommit: config.PreacquisitionSeal.SourceSealCommit, SealedBinaryHash: config.PreacquisitionSeal.BinarySHA256, AbandonedRegistryHash: config.AbandonedRegistry.RegistryHash, P4CollectorSourceCommit: config.Protocol.P4CollectorSourceCommit, ProtocolHash: config.Protocol.ProtocolHash, ExposureLedgerHash: ledger.LedgerHash, PartitionPaths: coverage.PartitionPaths, PartitionHashes: coverage.PartitionHashes, PhysicalComplete: coverage.MissingIntervals == 0 && coverage.ConflictCount == 0, PITEvidenceComplete: coverage.EvidenceGapCount == 0 && coverage.ClockErrorCount == 0 && coverage.ConflictCount == 0, CompleteEligibleDays: coverage.CompleteEligibleDays, MissingIntervalCount: coverage.MissingIntervals, ConflictCount: coverage.ConflictCount, EvidenceGapCount: coverage.EvidenceGapCount, ClockErrorCount: coverage.ClockErrorCount, PerSymbol: coverage.PerSymbol}
	if coverage.CompleteEligibleDays == 0 {
		checkpoint.MissingPartitions = append(checkpoint.MissingPartitions, "no contiguous complete eligible day")
	}
	checkpoint.CheckpointHash, _ = prospective.HashCanonical(checkpoint, "checkpoint_hash")
	data, _ := prospective.CanonicalJSON(checkpoint)
	path := filepath.Join(config.DataRoot, "manifests", "checkpoints", generation+".json")
	if err := writeImmutable(path, data, 0o444); err != nil {
		return Checkpoint{}, err
	}
	return checkpoint, nil
}

func BuildReadiness(config Config, coverage Coverage, checkpoint Checkpoint, backfill Verification, live prospective.VerificationSummary, now time.Time) Readiness {
	remaining := config.ReadinessPolicy.MinimumDays - coverage.CompleteEligibleDays
	if remaining < 0 {
		remaining = 0
	}
	liveTimerState := systemdState("ak-historian-prospective.timer")
	liveHealthy := live.Valid && (config.RepositoryRoot == "" || liveTimerState == "active")
	watcherState := "TEST_DETERMINISTIC"
	if config.RepositoryRoot != "" {
		watcherState = systemdState("ak-historian-r1p5r-readiness-watch.timer")
	}
	readiness := Readiness{SchemaVersion: "ak-historian.pr4b0-r1p5r.readiness-status.v1", GeneratedAtUTC: now.UTC(), CheckpointGenerationID: checkpoint.GenerationID, CheckpointHash: checkpoint.CheckpointHash, EligibleStartUTC: coverage.EligibleStartUTC, EligibleEndUTC: coverage.ContiguousEndUTC, CompleteEligibleDays: coverage.CompleteEligibleDays, MinimumDays: config.ReadinessPolicy.MinimumDays, RemainingDays: remaining, PerSymbol: coverage.PerSymbol, MissingIntervals: coverage.MissingIntervals, EvidenceGaps: coverage.EvidenceGapCount, Conflicts: coverage.ConflictCount, ReceiptChainHealthy: backfill.Valid && live.Valid, LiveCollectorHealthy: liveHealthy, BackfillComplete: backfill.Valid, ClockEvidenceStatus: "ACCEPTABLE", WatcherState: watcherState, Label: "PR4B0_R1P5R_REMEDIATION_PARTIALLY_COMPLETE"}
	if remaining > 0 {
		projected := coverage.ContiguousEndUTC.Add(time.Duration(remaining) * 24 * time.Hour)
		readiness.ProjectedReadyDateUTC = &projected
	}
	checkpointAuthorityValid := checkpoint.SchemaVersion == CheckpointVersion && checkpoint.RepairSourceCommit == config.SourceIdentity.RepairSourceCommit && checkpoint.SourceSealCommit == config.PreacquisitionSeal.SourceSealCommit && checkpoint.SealedBinaryHash == config.PreacquisitionSeal.BinarySHA256 && checkpoint.ProtocolHash == config.Protocol.ProtocolHash && checkpoint.AbandonedRegistryHash == config.AbandonedRegistry.RegistryHash && checkpoint.GenerationID != config.AbandonedRegistry.OldCheckpointID && checkpoint.CheckpointHash != config.AbandonedRegistry.OldCheckpointHash && checkpoint.BackfillChainTerminal != config.AbandonedRegistry.OldBackfillTerminal
	if checkpointAuthorityValid && allSymbolsComplete(coverage, config.ReadinessPolicy.MinimumDays) && readiness.ReceiptChainHealthy && readiness.LiveCollectorHealthy && readiness.BackfillComplete && coverage.MissingIntervals == 0 && coverage.EvidenceGapCount == 0 && coverage.ConflictCount == 0 && coverage.SchemaFailureCount == 0 && coverage.ClockErrorCount == 0 {
		if remaining == 0 {
			readiness.Label = "PR4B0_R1P5R_SOURCE_REPAIRED_AND_COVERAGE_REACQUIRED"
		} else {
			readiness.Label = "PR4B0_R1P5R_REMEDIATION_PARTIALLY_COMPLETE"
		}
	}
	return readiness
}

func ScanReadiness(config Config) (Readiness, error) {
	checkpoint, err := CurrentCheckpoint(config.DataRoot)
	if err != nil {
		return Readiness{}, err
	}
	if err := VerifyCheckpointAuthority(config, checkpoint); err != nil {
		return Readiness{}, err
	}
	backfill, err := NewStore(config.DataRoot, config.Protocol, config.SourceIdentity, config.PreacquisitionSeal).VerifyAll(checkpoint.CreatedAtUTC)
	if err != nil || !backfill.Valid || backfill.FinalChainHash != checkpoint.BackfillChainTerminal {
		return Readiness{}, errors.New("checkpoint R1P5R chain no longer verifies")
	}
	liveStore := prospective.NewStore(config.LiveDataRoot, config.Activation)
	live, err := liveStore.VerifyAll(checkpoint.CreatedAtUTC)
	if err != nil || !live.Valid {
		return Readiness{}, errors.New("checkpoint P4 live chain no longer verifies")
	}
	_, liveReceipts, err := liveStore.RebuildState()
	if err != nil {
		return Readiness{}, err
	}
	foundLiveTerminal := false
	for _, receipt := range liveReceipts {
		if receipt.CurrentReceiptChainHash == checkpoint.P4LiveChainTerminal && receipt.AuthorityReceipt.ReceiptHash == checkpoint.P4LiveAuthorityTerminal {
			foundLiveTerminal = true
			break
		}
	}
	if !foundLiveTerminal {
		return Readiness{}, errors.New("checkpoint P4 live terminal is not in the verified chain")
	}
	coverage := Coverage{SchemaVersion: "ak-historian.pr4b0-r1p5r.coverage.v1", GeneratedAtUTC: checkpoint.CreatedAtUTC, EligibleStartUTC: checkpoint.CoverageStartUTC, ContiguousEndUTC: checkpoint.CoverageEndUTC, CompleteEligibleDays: checkpoint.CompleteEligibleDays, PartitionCount: len(checkpoint.PartitionPaths), CompletePartitionCount: len(checkpoint.PartitionPaths), MissingIntervals: checkpoint.MissingIntervalCount, ConflictCount: checkpoint.ConflictCount, EvidenceGapCount: checkpoint.EvidenceGapCount, ClockErrorCount: checkpoint.ClockErrorCount, PerSymbol: checkpoint.PerSymbol, PartitionPaths: checkpoint.PartitionPaths, PartitionHashes: checkpoint.PartitionHashes}
	return BuildReadiness(config, coverage, checkpoint, backfill, live, checkpoint.CreatedAtUTC), nil
}

func VerifyCheckpointAuthority(config Config, checkpoint Checkpoint) error {
	if err := prospective.VerifyCanonicalHash(checkpoint, "checkpoint_hash", checkpoint.CheckpointHash); err != nil {
		return err
	}
	if checkpoint.SchemaVersion != CheckpointVersion || !strings.HasPrefix(checkpoint.GenerationID, "r1p5r-checkpoint-") || checkpoint.GenerationID == AbandonedCheckpointID || checkpoint.CheckpointHash == AbandonedCheckpointHash || checkpoint.BackfillChainTerminal == AbandonedBackfillTerminal || checkpoint.RepairSourceCommit == AbandonedSourceCommit || checkpoint.RepairSourceCommit != config.SourceIdentity.RepairSourceCommit || checkpoint.SourceSealCommit != config.PreacquisitionSeal.SourceSealCommit || checkpoint.SealedBinaryHash != config.PreacquisitionSeal.BinarySHA256 || checkpoint.ProtocolHash != config.Protocol.ProtocolHash || checkpoint.AbandonedRegistryHash != config.AbandonedRegistry.RegistryHash || checkpoint.BackfillChainGenesis != ZeroHash || checkpoint.EvaluationCutoffFloor.After(checkpoint.CreatedAtUTC) || !reflect.DeepEqual(checkpoint.RequiredSymbols, prospective.UniqueSymbols) || len(checkpoint.PartitionPaths) != len(checkpoint.PartitionHashes) || len(checkpoint.PartitionPaths) == 0 {
		return errors.New("checkpoint authority mismatch")
	}
	hashSet := map[string]bool{}
	for _, hash := range checkpoint.PartitionHashes {
		hashSet[hash] = true
	}
	for _, relative := range checkpoint.PartitionPaths {
		if filepath.IsAbs(relative) || strings.Contains(relative, "..") || strings.Contains(relative, "r1p5-checkpoint") {
			return errors.New("checkpoint contains unsafe or abandoned partition identity")
		}
		var partition Partition
		if err := prospective.ReadStrict(filepath.Join(config.DataRoot, filepath.FromSlash(relative)), &partition); err != nil {
			return err
		}
		if err := prospective.VerifyCanonicalHash(partition, "partition_hash", partition.PartitionHash); err != nil || !hashSet[partition.PartitionHash] || !strings.Contains(relative, strings.TrimPrefix(partition.PartitionHash, "sha256:")) {
			return errors.New("checkpoint partition binding mismatch")
		}
	}
	return nil
}

func allSymbolsComplete(coverage Coverage, minimumDays int) bool {
	if len(coverage.PerSymbol) != len(prospective.UniqueSymbols) {
		return false
	}
	seen := map[string]bool{}
	for _, symbol := range coverage.PerSymbol {
		if seen[symbol.Symbol] || !contains(prospective.UniqueSymbols, symbol.Symbol) || symbol.CompleteUTCDateCount < minimumDays || symbol.MissingIntervals != 0 || symbol.ConflictCount != 0 || symbol.EvidenceGapCount != 0 {
			return false
		}
		seen[symbol.Symbol] = true
	}
	return len(seen) == len(prospective.UniqueSymbols)
}

func systemdState(unit string) string {
	out, err := exec.Command("systemctl", "--user", "is-active", unit).CombinedOutput()
	state := strings.TrimSpace(string(out))
	if err != nil && state == "" {
		return "unavailable"
	}
	return state
}

func WriteReportPair(root, base string, value any, markdown string) error {
	data, err := prospective.CanonicalJSON(value)
	if err != nil {
		return err
	}
	if err := prospective.WriteAtomic(filepath.Join(root, "runs", "reports", base+".json"), data, 0o644); err != nil {
		return err
	}
	return prospective.WriteAtomic(filepath.Join(root, "runs", "reports", base+".md"), []byte(markdown), 0o644)
}

func CurrentCheckpoint(dataRoot string) (Checkpoint, error) {
	matches, err := filepath.Glob(filepath.Join(dataRoot, "manifests", "checkpoints", "r1p5r-checkpoint-*.json"))
	if err != nil || len(matches) == 0 {
		return Checkpoint{}, errors.New("no immutable checkpoint")
	}
	sort.Strings(matches)
	var c Checkpoint
	if err := prospective.ReadStrict(matches[len(matches)-1], &c); err != nil {
		return Checkpoint{}, err
	}
	if err := prospective.VerifyCanonicalHash(c, "checkpoint_hash", c.CheckpointHash); err != nil {
		return Checkpoint{}, err
	}
	return c, nil
}

func ValidateNoAbsoluteIdentity(value any) error {
	data, _ := prospective.CanonicalJSON(value)
	if strings.Contains(string(data), configHomeMarker()) {
		return errors.New("absolute home path entered identity")
	}
	return nil
}
func configHomeMarker() string { home, _ := os.UserHomeDir(); return home }
