package prospective

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Store struct {
	Root       string
	Activation Activation
}

type accumulatedPartition struct {
	records      map[int64]NormalizedCandle
	duplicates   []time.Time
	fragments    []string
	receipts     []string
	availability []time.Time
	conflicts    int
}

func NewStore(root string, activation Activation) *Store {
	return &Store{Root: filepath.Clean(root), Activation: activation}
}

func (store *Store) LedgerPath() string {
	return filepath.Join(store.Root, "ledgers", "receipts.jsonl")
}
func (store *Store) CycleLedgerPath() string {
	return filepath.Join(store.Root, "ledgers", "cycles.jsonl")
}
func (store *Store) StatePath() string { return filepath.Join(store.Root, "state", "collector.json") }
func (store *Store) LockPath() string  { return filepath.Join(store.Root, "state", "collector.lock") }

func (store *Store) relative(parts ...string) string {
	return filepath.ToSlash(filepath.Join(parts...))
}

func (store *Store) absolute(relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) || strings.Contains(relative, "..") || strings.Contains(relative, `\`) {
		return "", errors.New("manifest-relative path is invalid")
	}
	path := filepath.Join(store.Root, filepath.FromSlash(relative))
	rel, err := filepath.Rel(store.Root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", errors.New("manifest-relative path escapes data root")
	}
	return path, nil
}

func (store *Store) InitialState() State {
	return State{SchemaVersion: StateVersion, DatasetID: store.Activation.DatasetID, ActivationHash: store.Activation.ActivationHash, NextRegistration: 1, LastAuthorityHash: ZeroHash, LastEnvelopeHash: ZeroHash, Cursors: map[string]Cursor{}}
}

func sealState(state State) (State, error) {
	state.SchemaVersion = StateVersion
	state.StateHash = ""
	hash, err := HashCanonical(state, "state_hash")
	if err != nil {
		return State{}, err
	}
	state.StateHash = hash
	return state, nil
}

func (store *Store) SaveState(state State) error {
	sealed, err := sealState(state)
	if err != nil {
		return err
	}
	data, err := CanonicalJSON(sealed)
	if err != nil {
		return err
	}
	return WriteAtomic(store.StatePath(), data, 0o600)
}

func (store *Store) RebuildState() (State, []Receipt, error) {
	state := store.InitialState()
	receipts := []Receipt{}
	err := ReadJSONLines(store.LedgerPath(), func(line []byte) error {
		var receipt Receipt
		if err := StrictDecode(line, &receipt); err != nil {
			return fmt.Errorf("receipt ledger decode: %w", err)
		}
		if err := VerifyReceipt(receipt, store.Activation, state.LastEnvelopeHash, state.LastAuthorityHash, state.NextRegistration); err != nil {
			return err
		}
		state.LastEnvelopeHash = receipt.CurrentReceiptChainHash
		state.LastAuthorityHash = receipt.AuthorityReceipt.ReceiptHash
		state.NextRegistration++
		if receipt.ParsedRecordCount > 0 {
			state.Cursors[receipt.Symbol] = Cursor{LastOpenTimeMS: receipt.FinalCandleCloseTimeUTC.UnixMilli() - 59999, LastReceiptHash: receipt.CurrentReceiptChainHash}
		}
		receipts = append(receipts, receipt)
		return nil
	})
	if err != nil {
		return State{}, nil, err
	}
	var lastSuccess CycleResult
	if err := ReadJSONLines(store.CycleLedgerPath(), func(line []byte) error {
		var cycle CycleResult
		if err := StrictDecode(line, &cycle); err != nil {
			return err
		}
		if err := VerifyCanonicalHash(cycle, "cycle_hash", cycle.CycleHash); err != nil {
			return err
		}
		if cycle.FullUniverseSuccess {
			lastSuccess = cycle
		}
		return nil
	}); err != nil {
		return State{}, nil, fmt.Errorf("cycle ledger: %w", err)
	}
	state.LastSuccessfulCycleID = lastSuccess.CycleID
	state.LastSuccessfulCycleUTC = lastSuccess.CompletedAtUTC
	return state, receipts, nil
}

func (store *Store) CommitReceipt(receipt Receipt, raw, fragment []byte, rawRelative, fragmentRelative, receiptRelative string) (bool, error) {
	rawPath, err := store.absolute(rawRelative)
	if err != nil {
		return false, err
	}
	fragmentPath, err := store.absolute(fragmentRelative)
	if err != nil {
		return false, err
	}
	receiptPath, err := store.absolute(receiptRelative)
	if err != nil {
		return false, err
	}
	if existing, err := os.ReadFile(receiptPath); err == nil {
		var prior Receipt
		if StrictDecode(existing, &prior) != nil || !equalJSON(prior, receipt) {
			return false, errors.New("conflicting receipt at immutable identity")
		}
		return false, nil
	}
	if existing, err := os.ReadFile(rawPath); err == nil && HashBytes(existing) != receipt.RawResponseSHA256 {
		return false, errors.New("conflicting raw response at immutable identity")
	}
	if existing, err := os.ReadFile(fragmentPath); err == nil {
		var prior Fragment
		if StrictDecode(existing, &prior) != nil || VerifyCanonicalHash(prior, "fragment_hash", receipt.FragmentHash) != nil {
			return false, errors.New("conflicting normalized fragment at immutable identity")
		}
	}
	if err := WriteAtomic(rawPath, raw, 0o444); err != nil {
		return false, err
	}
	if err := WriteAtomic(fragmentPath, fragment, 0o444); err != nil {
		return false, err
	}
	receiptData, err := CanonicalJSON(receipt)
	if err != nil {
		return false, err
	}
	if err := WriteAtomic(receiptPath, receiptData, 0o444); err != nil {
		return false, err
	}
	if err := AppendDurable(store.LedgerPath(), receiptData); err != nil {
		return false, err
	}
	return true, nil
}

func (store *Store) CommitCycle(cycle CycleResult) error {
	data, err := CanonicalJSON(cycle)
	if err != nil {
		return err
	}
	return AppendDurable(store.CycleLedgerPath(), data)
}

func (store *Store) RetainIncomplete(cycleID, symbol string, evidence ResponseEvidence, reason string) error {
	if len(evidence.Body) == 0 {
		return nil
	}
	record := struct {
		SchemaVersion  string    `json:"schema_version"`
		CycleID        string    `json:"cycle_id"`
		Symbol         string    `json:"symbol"`
		Status         string    `json:"status"`
		Reason         string    `json:"reason"`
		RetainedAtUTC  time.Time `json:"retained_at_utc"`
		HTTPStatus     int       `json:"http_status"`
		BodyByteLength int       `json:"body_byte_length"`
		BodyHash       string    `json:"body_hash"`
	}{SchemaVersion: "ak-historian.pr4b0-r1p4.incomplete-evidence.v1", CycleID: cycleID, Symbol: symbol, Status: "AVAILABILITY_EVIDENCE_INCOMPLETE", Reason: reason, RetainedAtUTC: time.Now().UTC(), HTTPStatus: evidence.HTTPStatus, BodyByteLength: len(evidence.Body), BodyHash: HashBytes(evidence.Body)}
	base := filepath.Join(store.Root, "quarantine", "availability-incomplete", "cycle="+cycleID, "symbol="+symbol)
	if err := WriteAtomic(base+".raw", evidence.Body, 0o444); err != nil {
		return err
	}
	data, err := CanonicalJSON(record)
	if err != nil {
		return err
	}
	return WriteAtomic(base+".json", data, 0o444)
}

func (store *Store) BuildPartitionManifests(now time.Time) ([]PartitionManifest, error) {
	_, receipts, err := store.RebuildState()
	if err != nil {
		return nil, err
	}
	partitions := map[string]*accumulatedPartition{}
	for _, receipt := range receipts {
		path, err := store.absolute(receipt.FragmentRelativePath)
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("fragment verification failed for %s", receipt.FragmentRelativePath)
		}
		var fragment Fragment
		if err := StrictDecode(data, &fragment); err != nil {
			return nil, err
		}
		if err := VerifyCanonicalHash(fragment, "fragment_hash", receipt.FragmentHash); err != nil {
			return nil, fmt.Errorf("fragment verification failed for %s: %w", receipt.FragmentRelativePath, err)
		}
		for _, record := range fragment.Records {
			key := record.Symbol + "/" + record.SourceDate
			partition := partitions[key]
			if partition == nil {
				partition = &accumulatedPartition{records: map[int64]NormalizedCandle{}}
				partitions[key] = partition
			}
			if existing, duplicate := partition.records[record.OpenTimeMS]; duplicate {
				partition.duplicates = append(partition.duplicates, time.UnixMilli(record.OpenTimeMS).UTC())
				if !equalJSON(existing, record) {
					partition.conflicts++
				}
			} else {
				partition.records[record.OpenTimeMS] = record
			}
			partition.fragments = append(partition.fragments, receipt.FragmentHash)
			partition.receipts = append(partition.receipts, receipt.CurrentReceiptChainHash)
			partition.availability = append(partition.availability, receipt.ObservedAvailableAtUTC)
		}
	}
	keys := make([]string, 0, len(partitions))
	for key := range partitions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	manifests := make([]PartitionManifest, 0, len(keys))
	for _, key := range keys {
		parts := strings.Split(key, "/")
		manifest, err := buildPartitionManifest(store.Activation, parts[0], parts[1], partitions[key], now.UTC())
		if err != nil {
			return nil, err
		}
		relative := store.relative("manifests", "partitions", "symbol="+manifest.Symbol, "date="+manifest.UTCDate+".json")
		path, _ := store.absolute(relative)
		data, _ := CanonicalJSON(manifest)
		if err := WriteAtomic(path, data, 0o444); err != nil {
			return nil, err
		}
		manifests = append(manifests, manifest)
	}
	return manifests, nil
}

func buildPartitionManifest(activation Activation, symbol, date string, partition *accumulatedPartition, now time.Time) (PartitionManifest, error) {
	dayStart, err := time.Parse("2006-01-02", date)
	if err != nil {
		return PartitionManifest{}, err
	}
	dayEnd := dayStart.Add(24 * time.Hour)
	openTimes := make([]int64, 0, len(partition.records))
	for value := range partition.records {
		openTimes = append(openTimes, value)
	}
	sort.Slice(openTimes, func(i, j int) bool { return openTimes[i] < openTimes[j] })
	missing := []MissingInterval{}
	for cursor := dayStart.UnixMilli(); cursor < dayEnd.UnixMilli(); {
		if _, ok := partition.records[cursor]; ok {
			cursor += 60000
			continue
		}
		start := cursor
		for cursor < dayEnd.UnixMilli() {
			if _, ok := partition.records[cursor]; ok {
				break
			}
			cursor += 60000
		}
		missing = append(missing, MissingInterval{StartOpenTimeUTC: time.UnixMilli(start).UTC(), EndOpenTimeUTC: time.UnixMilli(cursor).UTC()})
	}
	manifest := PartitionManifest{SchemaVersion: PartitionManifestVersion, DatasetID: activation.DatasetID, Generation: activation.Generation, Symbol: symbol, UTCDate: date, ExpectedRows: 1440, ObservedRows: len(openTimes), MissingIntervals: missing, DuplicateIntervals: partition.duplicates, ConstituentFragmentHashes: uniqueSorted(partition.fragments), ReceiptIdentities: uniqueSorted(partition.receipts), SourceSchemaHash: SourceSchemaFingerprint, ConflictCount: partition.conflicts}
	if len(openTimes) > 0 {
		first := time.UnixMilli(openTimes[0]).UTC()
		last := time.UnixMilli(openTimes[len(openTimes)-1] + 59999).UTC()
		manifest.FirstOpenTimeUTC, manifest.LastCloseTimeUTC = &first, &last
	}
	if len(partition.availability) > 0 {
		sort.Slice(partition.availability, func(i, j int) bool { return partition.availability[i].Before(partition.availability[j]) })
		earliest, latest := partition.availability[0], partition.availability[len(partition.availability)-1]
		manifest.EarliestAvailabilityUTC, manifest.LatestAvailabilityUTC = &earliest, &latest
	}
	manifest.PhysicalStatus = "PHYSICAL_INCOMPLETE"
	manifest.PITEvidenceStatus = "PIT_EVIDENCE_COMPLETE"
	manifest.PartitionStatus = "PARTITION_OPEN"
	if partition.conflicts > 0 {
		manifest.PartitionStatus = "CONFLICT"
		manifest.PITEvidenceStatus = "PIT_EVIDENCE_INCOMPLETE"
	} else if !now.Before(dayEnd) {
		if len(openTimes) == 1440 && len(missing) == 0 {
			manifest.PhysicalStatus = "PHYSICAL_COMPLETE"
			manifest.PartitionStatus = "PIT_EVIDENCE_COMPLETE"
		} else {
			manifest.PartitionStatus = "PHYSICAL_INCOMPLETE"
		}
	}
	manifest.PartitionHash, err = HashCanonical(manifest, "partition_hash")
	return manifest, err
}

func uniqueSorted(values []string) []string {
	set := map[string]struct{}{}
	for _, value := range values {
		set[value] = struct{}{}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
