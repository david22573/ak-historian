package r1p5

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/david22573/ak-historian/internal/prospective"
)

type Store struct {
	Root           string
	Protocol       Protocol
	SourceIdentity SourceIdentity
}

func NewStore(root string, protocol Protocol, identity SourceIdentity) *Store {
	return &Store{Root: filepath.Clean(root), Protocol: protocol, SourceIdentity: identity}
}

func (s *Store) LockPath() string   { return filepath.Join(s.Root, "locks", "backfill.lock") }
func (s *Store) LedgerPath() string { return filepath.Join(s.Root, "ledgers", "receipts.jsonl") }
func (s *Store) StatePath() string  { return filepath.Join(s.Root, "state", "backfill.json") }

func (s *Store) absolute(relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) || strings.Contains(relative, "..") || strings.Contains(relative, `\`) {
		return "", errors.New("unsafe manifest-relative path")
	}
	path := filepath.Join(s.Root, filepath.FromSlash(relative))
	rel, err := filepath.Rel(s.Root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", errors.New("path escapes data root")
	}
	return path, nil
}

func gzipCanonical(data []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buffer, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	writer.Header.ModTime = time.Unix(0, 0).UTC()
	writer.Header.OS = 255
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func gunzip(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(io.LimitReader(reader, 16*1024*1024+1))
}

func (s *Store) readCompressed(relative string) ([]byte, error) {
	path, err := s.absolute(relative)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return gunzip(data)
}

func (s *Store) initialState() State {
	cursors := map[string]Cursor{}
	for _, symbol := range prospective.UniqueSymbols {
		cursors[symbol] = Cursor{NextOpenUTC: s.Protocol.EligibleStartUTC}
	}
	return State{SchemaVersion: StateVersion, NextSequence: 1, ChainTerminal: ZeroHash, Cursors: cursors}
}

func (s *Store) RebuildState() (State, []LedgerEntry, error) {
	state := s.initialState()
	entries := []LedgerEntry{}
	err := prospective.ReadJSONLines(s.LedgerPath(), func(line []byte) error {
		var entry LedgerEntry
		if err := prospective.StrictDecode(line, &entry); err != nil {
			return err
		}
		if entry.SchemaVersion != LedgerVersion || entry.Sequence != state.NextSequence || entry.PriorChainHash != state.ChainTerminal || entry.EvaluationCutoffFloor != entry.DurableCompletionUTC.Add(time.Nanosecond) {
			return errors.New("receipt ledger sequence, chain, or evaluation floor invalid")
		}
		if err := prospective.VerifyCanonicalHash(entry, "current_chain_hash", entry.CurrentChainHash); err != nil {
			return err
		}
		receipt, err := s.readReceipt(entry.ReceiptPath)
		if err != nil {
			return err
		}
		if receipt.ReceiptHash != entry.ReceiptHash || receipt.PriorReceiptChainHash != entry.PriorChainHash {
			return errors.New("ledger receipt binding invalid")
		}
		if err := s.verifyObjects(receipt); err != nil {
			return err
		}
		cursor := state.Cursors[receipt.Symbol]
		if receipt.RequestedStartUTC != cursor.NextOpenUTC {
			return fmt.Errorf("non-contiguous durable cursor for %s", receipt.Symbol)
		}
		cursor.NextOpenUTC = receipt.RequestedEndExclusiveUTC
		cursor.Requests++
		cursor.Rows += receipt.ParsedRowCount
		state.Cursors[receipt.Symbol] = cursor
		state.NextSequence++
		state.ChainTerminal = entry.CurrentChainHash
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return State{}, nil, err
	}
	state.StateHash, err = prospective.HashCanonical(state, "state_hash")
	return state, entries, err
}

func (s *Store) readReceipt(relative string) (Receipt, error) {
	path, err := s.absolute(relative)
	if err != nil {
		return Receipt{}, err
	}
	var receipt Receipt
	if err := prospective.ReadStrict(path, &receipt); err != nil {
		return Receipt{}, err
	}
	if err := VerifyReceipt(receipt, s.Protocol, s.SourceIdentity); err != nil {
		return Receipt{}, err
	}
	return receipt, nil
}

func VerifyReceipt(r Receipt, p Protocol, identity SourceIdentity) error {
	if r.SchemaVersion != ReceiptVersion || r.AcquisitionMode != Mode || r.Symbol == "" || r.Endpoint != prospective.KlineEndpoint || r.HTTPStatus != 200 || r.RawByteLength <= 0 || r.FragmentByteLength <= 0 || r.ParsedRowCount <= 0 || r.CompleteResponseReceivedUTC.IsZero() || r.ProviderHTTPDateUTC.IsZero() || r.ProviderServerTimeUTC.IsZero() || !r.ClockEvidence.Synchronized || r.ObservedAvailableAtUTC != maxTime(r.CompleteResponseReceivedUTC, r.ProviderHTTPDateUTC, r.ProviderServerTimeUTC) || r.AcquiredAtUTC.Before(r.ObservedAvailableAtUTC) || r.BackfillSourceCommit != identity.SourceCommit || r.ProtocolHash != p.ProtocolHash || r.P4CollectorSourceCommit != p.P4CollectorSourceCommit || r.AvailabilityPolicyVersion != p.AvailabilityPolicyVersion || r.AvailabilityPolicyHash != p.AvailabilityPolicyHash || r.RequestedStartUTC.Before(p.EligibleStartUTC) || r.RequestedEndExclusiveUTC.After(p.BackfillEnds[r.Symbol]) || r.RequestedEndExclusiveUTC.Sub(r.RequestedStartUTC) > 1000*time.Minute || r.ParsedRowCount != int(r.RequestedEndExclusiveUTC.Sub(r.RequestedStartUTC)/time.Minute) {
		return errors.New("historical receipt required authority invalid")
	}
	return prospective.VerifyCanonicalHash(r, "receipt_hash", r.ReceiptHash)
}

func (s *Store) verifyObjects(r Receipt) error {
	raw, err := s.readCompressed(r.RawPath)
	if err != nil || len(raw) != r.RawByteLength || prospective.HashBytes(raw) != r.RawHash {
		return fmt.Errorf("raw evidence invalid for %s", r.RequestID)
	}
	fragmentData, err := s.readCompressed(r.FragmentPath)
	if err != nil || len(fragmentData) != r.FragmentByteLength {
		return fmt.Errorf("fragment evidence invalid for %s", r.RequestID)
	}
	var fragment Fragment
	if err := prospective.StrictDecode(fragmentData, &fragment); err != nil {
		return err
	}
	if err := prospective.VerifyCanonicalHash(fragment, "fragment_hash", fragment.FragmentHash); err != nil || fragment.FragmentHash != r.FragmentHash || fragment.RequestID != r.RequestID || fragment.Symbol != r.Symbol || len(fragment.Records) != r.ParsedRowCount {
		return fmt.Errorf("fragment binding invalid for %s", r.RequestID)
	}
	for index, record := range fragment.Records {
		expected := r.RequestedStartUTC.Add(time.Duration(index) * time.Minute)
		if time.UnixMilli(record.OpenTimeMS).UTC() != expected || record.MarketEventTimeUTC != expected || record.ProviderCandleCloseTimeUTC != time.UnixMilli(record.CloseTimeMS).UTC() || record.ObservedAvailableAtUTC != r.ObservedAvailableAtUTC || record.AcquiredAtUTC != r.AcquiredAtUTC || record.AcquisitionReceiptID != r.RequestID {
			return errors.New("normalized historical timestamp or acquisition evidence invalid")
		}
	}
	return nil
}

func (s *Store) Commit(receipt Receipt, raw, fragmentData []byte) (LedgerEntry, error) {
	rawPath, err := s.absolute(receipt.RawPath)
	if err != nil {
		return LedgerEntry{}, err
	}
	fragmentPath, err := s.absolute(receipt.FragmentPath)
	if err != nil {
		return LedgerEntry{}, err
	}
	receiptRelative := receiptPath(receipt.Symbol, receipt.RequestedStartUTC)
	receiptPathAbs, err := s.absolute(receiptRelative)
	if err != nil {
		return LedgerEntry{}, err
	}
	if existing, err := os.ReadFile(receiptPathAbs); err == nil {
		var prior Receipt
		if prospective.StrictDecode(existing, &prior) != nil || prior.ReceiptHash != receipt.ReceiptHash {
			return LedgerEntry{}, errors.New("conflicting receipt at immutable request identity")
		}
		return s.commitLedger(prior, receiptRelative)
	}
	compressedRaw, err := gzipCanonical(raw)
	if err != nil {
		return LedgerEntry{}, err
	}
	compressedFragment, err := gzipCanonical(fragmentData)
	if err != nil {
		return LedgerEntry{}, err
	}
	if err := writeImmutable(rawPath, compressedRaw, 0o444); err != nil {
		return LedgerEntry{}, err
	}
	if err := writeImmutable(fragmentPath, compressedFragment, 0o444); err != nil {
		return LedgerEntry{}, err
	}
	receiptData, err := prospective.CanonicalJSON(receipt)
	if err != nil {
		return LedgerEntry{}, err
	}
	if err := writeImmutable(receiptPathAbs, receiptData, 0o444); err != nil {
		return LedgerEntry{}, err
	}
	return s.commitLedger(receipt, receiptRelative)
}

func writeImmutable(path string, data []byte, mode os.FileMode) error {
	if existing, err := os.ReadFile(path); err == nil {
		if !bytes.Equal(existing, data) {
			return fmt.Errorf("conflicting immutable object: %s", filepath.Base(path))
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return prospective.WriteAtomic(path, data, mode)
}

func (s *Store) commitLedger(receipt Receipt, receiptRelative string) (LedgerEntry, error) {
	state, entries, err := s.RebuildState()
	if err != nil {
		return LedgerEntry{}, err
	}
	for _, entry := range entries {
		if entry.ReceiptHash == receipt.ReceiptHash {
			return entry, nil
		}
	}
	if receipt.PriorReceiptChainHash != state.ChainTerminal || receipt.RequestedStartUTC != state.Cursors[receipt.Symbol].NextOpenUTC {
		return LedgerEntry{}, errors.New("orphan receipt no longer joins current durable chain")
	}
	completed := time.Now().UTC()
	entry := LedgerEntry{SchemaVersion: LedgerVersion, Sequence: state.NextSequence, ReceiptPath: receiptRelative, ReceiptHash: receipt.ReceiptHash, PriorChainHash: state.ChainTerminal, DurableCompletionUTC: completed, EvaluationCutoffFloor: completed.Add(time.Nanosecond)}
	entry.CurrentChainHash, err = prospective.HashCanonical(entry, "current_chain_hash")
	if err != nil {
		return LedgerEntry{}, err
	}
	data, _ := prospective.CanonicalJSON(entry)
	if err := prospective.AppendDurable(s.LedgerPath(), data); err != nil {
		return LedgerEntry{}, err
	}
	state.Cursors[receipt.Symbol] = Cursor{NextOpenUTC: receipt.RequestedEndExclusiveUTC, Requests: state.Cursors[receipt.Symbol].Requests + 1, Rows: state.Cursors[receipt.Symbol].Rows + receipt.ParsedRowCount}
	state.NextSequence++
	state.ChainTerminal = entry.CurrentChainHash
	state.StateHash, _ = prospective.HashCanonical(state, "state_hash")
	stateData, _ := prospective.CanonicalJSON(state)
	if err := prospective.WriteAtomic(s.StatePath(), stateData, 0o600); err != nil {
		return LedgerEntry{}, err
	}
	return entry, nil
}

func receiptPath(symbol string, start time.Time) string {
	return filepath.ToSlash(filepath.Join("receipts", "symbol="+symbol, "start="+start.UTC().Format("20060102T150405Z")+".json"))
}

func (s *Store) VerifyAll(now time.Time) (Verification, error) {
	state, entries, err := s.RebuildState()
	if err != nil {
		return Verification{}, err
	}
	v := Verification{SchemaVersion: "ak-historian.pr4b0-r1p5.verification-summary.v1", VerifiedAtUTC: now.UTC(), RequestCount: len(entries), ReceiptCount: len(entries), RawResponseCount: len(entries), FragmentCount: len(entries), FinalChainHash: state.ChainTerminal, Valid: true}
	for _, entry := range entries {
		r, err := s.readReceipt(entry.ReceiptPath)
		if err != nil {
			return Verification{}, err
		}
		v.CandleCount += r.ParsedRowCount
		if entry.EvaluationCutoffFloor.After(v.EvaluationCutoffFloor) {
			v.EvaluationCutoffFloor = entry.EvaluationCutoffFloor
		}
		if !r.ClockEvidence.Synchronized {
			v.ClockErrorCount++
		}
	}
	for _, symbol := range prospective.UniqueSymbols {
		if state.Cursors[symbol].NextOpenUTC != s.Protocol.BackfillEnds[symbol] {
			v.Valid = false
			v.EvidenceGapCount++
		}
	}
	return v, nil
}

func (s *Store) Entries() ([]LedgerEntry, error) {
	_, entries, err := s.RebuildState()
	return entries, err
}
func (s *Store) Receipt(entry LedgerEntry) (Receipt, error) { return s.readReceipt(entry.ReceiptPath) }
func (s *Store) Fragment(receipt Receipt) (Fragment, error) {
	data, err := s.readCompressed(receipt.FragmentPath)
	if err != nil {
		return Fragment{}, err
	}
	var fragment Fragment
	if err := prospective.StrictDecode(data, &fragment); err != nil {
		return Fragment{}, err
	}
	return fragment, nil
}

func maxTime(values ...time.Time) time.Time {
	var result time.Time
	for _, value := range values {
		if value.After(result) {
			result = value.UTC()
		}
	}
	return result
}
func uniqueSorted(values []string) []string {
	set := map[string]struct{}{}
	for _, v := range values {
		set[v] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
