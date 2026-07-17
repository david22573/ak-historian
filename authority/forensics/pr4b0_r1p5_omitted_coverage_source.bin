package r1p5

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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
	backfillStore := NewStore(config.DataRoot, config.Protocol, config.SourceIdentity)
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
	coverage := Coverage{SchemaVersion: "ak-historian.pr4b0-r1p5.coverage.v1", GeneratedAtUTC: now.UTC(), EligibleStartUTC: config.Protocol.EligibleStartUTC, ContiguousEndUTC: config.Protocol.EligibleStartUTC}
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
	ledger := EligibilityLedger{SchemaVersion: "ak-historian.pr4b0-r1p5.exposure-eligibility-ledger.v1", GeneratedAtUTC: now.UTC(), ExposurePolicyHash: config.ExposurePolicy.PolicyHash, Intervals: append([]Interval{}, config.Protocol.BarredIntervals...)}
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
	generation := "r1p5-checkpoint-" + now.UTC().Format("20060102T150405Z")
	checkpoint := Checkpoint{SchemaVersion: CheckpointVersion, DatasetID: config.Protocol.DatasetID, GenerationID: generation, CreatedAtUTC: now.UTC(), CoverageStartUTC: coverage.EligibleStartUTC, CoverageEndUTC: coverage.ContiguousEndUTC, EvaluationCutoffFloor: backfill.EvaluationCutoffFloor, RequiredSymbols: append([]string{}, prospective.UniqueSymbols...), SourceSchemaHash: config.Protocol.SourceSchemaFingerprint, AvailabilityPolicyHash: config.Protocol.AvailabilityPolicyHash, CoveragePolicyHash: config.ReadinessPolicy.PolicyHash, ManifestContractHash: config.Protocol.ManifestContractHash, IngestionReceiptHash: config.Protocol.IngestionReceiptHash, P4ActivationHash: config.Activation.ActivationHash, P4LiveChainTerminal: live.FinalEnvelopeHash, P4LiveAuthorityTerminal: live.FinalAuthorityHash, BackfillChainTerminal: backfill.FinalChainHash, BackfillSourceCommit: config.SourceIdentity.SourceCommit, P4CollectorSourceCommit: config.Protocol.P4CollectorSourceCommit, ProtocolHash: config.Protocol.ProtocolHash, ExposureLedgerHash: ledger.LedgerHash, PartitionPaths: coverage.PartitionPaths, PartitionHashes: coverage.PartitionHashes, PhysicalComplete: coverage.MissingIntervals == 0 && coverage.ConflictCount == 0, PITEvidenceComplete: coverage.EvidenceGapCount == 0 && coverage.ClockErrorCount == 0 && coverage.ConflictCount == 0, CompleteEligibleDays: coverage.CompleteEligibleDays, IndependenceV3Hash: "sha256:84a6863b354b453dbe13698b9854ec4adcd116466a0831e7107efb892042cc1f", UncertaintyV2Hash: "sha256:1a91541c94378cc6f34e62a39ae504d3d013b5dab63a2b622641cdd1088148fb", PerSymbol: coverage.PerSymbol}
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
	readiness := Readiness{SchemaVersion: "ak-historian.pr4b0-r1p5.readiness-status.v1", GeneratedAtUTC: now.UTC(), CheckpointGenerationID: checkpoint.GenerationID, CheckpointHash: checkpoint.CheckpointHash, EligibleStartUTC: coverage.EligibleStartUTC, EligibleEndUTC: coverage.ContiguousEndUTC, CompleteEligibleDays: coverage.CompleteEligibleDays, MinimumDays: config.ReadinessPolicy.MinimumDays, RemainingDays: remaining, PerSymbol: coverage.PerSymbol, MissingIntervals: coverage.MissingIntervals, EvidenceGaps: coverage.EvidenceGapCount, Conflicts: coverage.ConflictCount, ReceiptChainHealthy: backfill.Valid && live.Valid, LiveCollectorHealthy: liveHealthy, BackfillComplete: backfill.Valid, ClockEvidenceStatus: "ACCEPTABLE", FutureSplitFeasible: coverage.CompleteEligibleDays >= 3, WatcherState: systemdState("ak-historian-readiness-watch.timer"), Label: config.ReadinessPolicy.RemediationLabel}
	if remaining > 0 {
		projected := coverage.ContiguousEndUTC.Add(time.Duration(remaining) * 24 * time.Hour)
		readiness.ProjectedReadyDateUTC = &projected
	}
	if readiness.ReceiptChainHealthy && readiness.LiveCollectorHealthy && readiness.BackfillComplete && coverage.MissingIntervals == 0 && coverage.EvidenceGapCount == 0 && coverage.ConflictCount == 0 {
		if remaining == 0 {
			readiness.Label = config.ReadinessPolicy.ReadyLabel
		} else {
			readiness.Label = config.ReadinessPolicy.NotReadyLabel
		}
	}
	return readiness
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
	matches, err := filepath.Glob(filepath.Join(dataRoot, "manifests", "checkpoints", "r1p5-checkpoint-*.json"))
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
