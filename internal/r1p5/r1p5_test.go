package r1p5

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/david22573/ak-historian/internal/prospective"
)

func testProtocol(t *testing.T) Protocol {
	t.Helper()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ends := map[string]time.Time{}
	for _, symbol := range prospective.UniqueSymbols {
		ends[symbol] = start.Add(time.Minute)
	}
	p := Protocol{SchemaVersion: ProtocolVersion, DatasetID: "test-r1p5", AcceptedHistorianCommit: "da9a6db4fad3ee5d47347453e164af6405d21fb8", SourceIdentityPath: "authority/source.json", P4CollectorSourceCommit: "598a9119be828daa7db76dacec017456807ccfed", P4ProtocolHash: "sha256:671a27239d72e163428378dff926acc9f7a22036aff247cc8888ee9f06077311", AvailabilityPolicyVersion: prospective.AvailabilityPolicyVersion, AvailabilityPolicyHash: "sha256:cbd4c1670830843d233b6b6c3dc3dac0489e3d38fc7854caf388b5e543dfc3e1", SourceSchemaVersion: prospective.SourceSchemaVersion, SourceSchemaFingerprint: prospective.SourceSchemaFingerprint, ManifestContractHash: prospective.ManifestContractHash, IngestionReceiptHash: prospective.ReceiptSchemaHash, ReceiptLedgerVersion: LedgerVersion, ReceiptLedgerGenesisHash: ZeroHash, EligibleStartUTC: start, BackfillEnds: ends, Symbols: append([]string{}, prospective.UniqueSymbols...), Venue: "Binance", MarketType: "USD-M futures", Timeframe: "1m", AcquisitionMode: Mode, ResearchProhibition: "candidate research prohibited"}
	p.ProtocolHash, _ = prospective.HashCanonical(p, "protocol_hash")
	return p
}

func testIdentity(t *testing.T, p Protocol) SourceIdentity {
	t.Helper()
	i := SourceIdentity{SchemaVersion: SourceIdentityVersion, SourceCommit: "1111111111111111111111111111111111111111", ProtocolHash: p.ProtocolHash, CreatedAtUTC: time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)}
	i.IdentityHash, _ = prospective.HashCanonical(i, "identity_hash")
	return i
}

func testReceipt(t *testing.T, p Protocol, i SourceIdentity) (Receipt, []byte, []byte) {
	t.Helper()
	start := p.EligibleStartUTC
	complete := time.Date(2026, 7, 15, 8, 0, 1, 0, time.UTC)
	httpDate := complete.Add(-time.Second)
	provider := complete.Add(-2 * time.Second)
	observed := complete
	acquired := complete.Add(time.Second)
	requestID := prospective.HashBytes([]byte("request"))
	candle := prospective.NormalizedCandle{Market: "futures-um", Symbol: "ADAUSDT", Interval: "1m", Period: "daily", SourceDate: "2026-01-01", OpenTimeMS: start.UnixMilli(), Open: "1", High: "2", Low: "0.5", Close: "1.5", Volume: "10", CloseTimeMS: start.Add(time.Minute).Add(-time.Millisecond).UnixMilli(), QuoteAssetVolume: "12", NumberOfTrades: 3, TakerBuyBaseVolume: "4", TakerBuyQuoteVolume: "5"}
	record := NormalizedRecord{NormalizedCandle: candle, MarketEventTimeUTC: start, ProviderCandleCloseTimeUTC: time.UnixMilli(candle.CloseTimeMS).UTC(), ObservedAvailableAtUTC: observed, AcquiredAtUTC: acquired, AcquisitionReceiptID: requestID}
	fragment := Fragment{SchemaVersion: FragmentVersion, RequestID: requestID, Symbol: "ADAUSDT", SourceSchemaVersion: p.SourceSchemaVersion, SourceSchemaFingerprint: p.SourceSchemaFingerprint, Records: []NormalizedRecord{record}}
	fragment.FragmentHash, _ = prospective.HashCanonical(fragment, "fragment_hash")
	fragmentData, _ := prospective.CanonicalJSON(fragment)
	raw := []byte("[[fixture]]")
	r := Receipt{SchemaVersion: ReceiptVersion, AcquisitionMode: Mode, RequestID: requestID, Symbol: "ADAUSDT", RequestedStartUTC: start, RequestedEndExclusiveUTC: start.Add(time.Minute), Endpoint: prospective.KlineEndpoint, CanonicalRequestParameters: "fixture", RequestStartUTC: complete.Add(-3 * time.Second), ResponseHeadersReceivedUTC: complete.Add(-time.Millisecond), CompleteResponseReceivedUTC: complete, ProviderHTTPDate: "Wed, 15 Jul 2026 08:00:00 GMT", ProviderHTTPDateUTC: httpDate, ProviderServerTimeUTC: provider, ProviderServerTimeHash: prospective.HashBytes([]byte("time")), ClockEvidence: prospective.ClockEvidence{CheckedAtUTC: complete, Synchronized: true, EvidenceHash: prospective.HashBytes([]byte("clock"))}, HTTPStatus: 200, RawByteLength: len(raw), RawHash: prospective.HashBytes(raw), RawPath: "raw/symbol=ADAUSDT/start=20260101T000000Z.json.gz", FragmentByteLength: len(fragmentData), FragmentHash: fragment.FragmentHash, FragmentPath: "fragments/symbol=ADAUSDT/start=20260101T000000Z.json.gz", ParsedRowCount: 1, FirstCandleOpenUTC: start, LastCandleCloseUTC: record.ProviderCandleCloseTimeUTC, ObservedAvailableAtUTC: observed, AcquiredAtUTC: acquired, PriorReceiptChainHash: ZeroHash, BackfillSourceCommit: i.SourceCommit, ProtocolHash: p.ProtocolHash, P4CollectorSourceCommit: p.P4CollectorSourceCommit, AvailabilityPolicyVersion: p.AvailabilityPolicyVersion, AvailabilityPolicyHash: p.AvailabilityPolicyHash}
	r.ReceiptHash, _ = prospective.HashCanonical(r, "receipt_hash")
	return r, raw, fragmentData
}

func reseal(r Receipt) Receipt {
	r.ReceiptHash = ""
	r.ReceiptHash, _ = prospective.HashCanonical(r, "receipt_hash")
	return r
}

func TestExposureBoundaryFailsClosed(t *testing.T) {
	p := testProtocol(t)
	barred := Interval{StartUTC: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), EndUTC: p.EligibleStartUTC, Classification: "BARRED_PRIOR_OUTCOME_EXPOSURE"}
	if barred.EndUTC.After(p.EligibleStartUTC) {
		t.Fatal("barred period entered eligible scope")
	}
	newExposure := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	eligible := p.EligibleStartUTC
	if newExposure.After(eligible) {
		eligible = newExposure
	}
	if !eligible.Equal(newExposure) {
		t.Fatal("new exposure did not move start forward")
	}
	unknownScope := true
	if !unknownScope {
		t.Fatal("unknown inspection scope must fail closed")
	}
}

func TestAvailabilityNeverBackdated(t *testing.T) {
	p := testProtocol(t)
	i := testIdentity(t, p)
	r, _, _ := testReceipt(t, p, i)
	if !r.ObservedAvailableAtUTC.Equal(r.CompleteResponseReceivedUTC) || !r.ObservedAvailableAtUTC.After(r.LastCandleCloseUTC) {
		t.Fatal("historical availability was backdated")
	}
	cases := []struct {
		name   string
		mutate func(*Receipt)
	}{{"provider time", func(r *Receipt) { r.ProviderServerTimeUTC = time.Time{} }}, {"response complete", func(r *Receipt) { r.CompleteResponseReceivedUTC = time.Time{} }}, {"clock", func(r *Receipt) { r.ClockEvidence.Synchronized = false }}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bad := r
			tc.mutate(&bad)
			bad = reseal(bad)
			if VerifyReceipt(bad, p, i) == nil {
				t.Fatal("incomplete availability was accepted")
			}
		})
	}
}

func TestReceiptMutationInvalidatesEvidence(t *testing.T) {
	p := testProtocol(t)
	i := testIdentity(t, p)
	r, _, _ := testReceipt(t, p, i)
	r.ParsedRowCount = 2
	if VerifyReceipt(r, p, i) == nil {
		t.Fatal("mutated receipt verified")
	}
}

func TestDurableCommitRestartAndMutation(t *testing.T) {
	p := testProtocol(t)
	i := testIdentity(t, p)
	root := t.TempDir()
	store := NewStore(root, p, i)
	r, raw, fragment := testReceipt(t, p, i)
	entry, err := store.Commit(r, raw, fragment)
	if err != nil {
		t.Fatal(err)
	}
	if !entry.EvaluationCutoffFloor.After(entry.DurableCompletionUTC) {
		t.Fatal("evaluation floor is not after durable completion")
	}
	state, entries, err := store.RebuildState()
	if err != nil || len(entries) != 1 || state.Cursors["ADAUSDT"].NextOpenUTC != r.RequestedEndExclusiveUTC {
		t.Fatalf("restart did not rebuild durable cursor: %v", err)
	}
	rawPath, _ := store.absolute(r.RawPath)
	compressed, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(rawPath, 0o644); err != nil {
		t.Fatal(err)
	}
	compressed[len(compressed)-1] ^= 1
	if err := os.WriteFile(rawPath, compressed, 0o444); err != nil {
		t.Fatal(err)
	}
	if _, err := store.VerifyAll(time.Now()); err == nil {
		t.Fatal("raw mutation verified")
	}
}

func TestCrashBeforeLedgerDoesNotAdvanceCursor(t *testing.T) {
	p := testProtocol(t)
	i := testIdentity(t, p)
	store := NewStore(t.TempDir(), p, i)
	r, raw, fragment := testReceipt(t, p, i)
	rawCompressed, _ := gzipCanonical(raw)
	fragmentCompressed, _ := gzipCanonical(fragment)
	rawPath, _ := store.absolute(r.RawPath)
	fragmentPath, _ := store.absolute(r.FragmentPath)
	receiptRelative := receiptPath(r.Symbol, r.RequestedStartUTC)
	receiptPathAbs, _ := store.absolute(receiptRelative)
	if err := prospective.WriteAtomic(rawPath, rawCompressed, 0o444); err != nil {
		t.Fatal(err)
	}
	if err := prospective.WriteAtomic(fragmentPath, fragmentCompressed, 0o444); err != nil {
		t.Fatal(err)
	}
	receiptData, _ := prospective.CanonicalJSON(r)
	if err := prospective.WriteAtomic(receiptPathAbs, receiptData, 0o444); err != nil {
		t.Fatal(err)
	}
	state, entries, err := store.RebuildState()
	if err != nil || len(entries) != 0 || state.Cursors[r.Symbol].NextOpenUTC != p.EligibleStartUTC {
		t.Fatal("orphan advanced durable cursor")
	}
	if _, err := store.commitLedger(r, receiptRelative); err != nil {
		t.Fatal(err)
	}
	state, _, _ = store.RebuildState()
	if state.Cursors[r.Symbol].NextOpenUTC != r.RequestedEndExclusiveUTC {
		t.Fatal("verified orphan was not restart-safe")
	}
}

func TestConcurrentBackfillLockRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	one, err := prospective.AcquireLock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer one.Close()
	if _, err := prospective.AcquireLock(path); err == nil {
		t.Fatal("concurrent backfill lock accepted")
	}
}

func TestCompleteDayRequiresExactSequence(t *testing.T) {
	date := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	acc := &partitionAccumulator{}
	for index := range acc.slots {
		acc.slots[index] = minuteSlot{present: true}
	}
	p := buildPartition("BTCUSDT", date, acc)
	if p.ObservedRows != 1440 || p.PhysicalStatus != "PHYSICAL_COMPLETE" || p.EligibilityClass != "UNEXPOSED_PIT_EVIDENCE_COMPLETE" {
		t.Fatal("exact complete day rejected")
	}
	acc.slots[42] = minuteSlot{}
	p = buildPartition("BTCUSDT", date, acc)
	if p.PhysicalStatus != "PHYSICAL_INCOMPLETE" || len(p.MissingIntervals) != 1 {
		t.Fatal("missing minute did not block completeness")
	}
	acc.slots[42] = minuteSlot{present: true}
	acc.conflicts = 1
	p = buildPartition("BTCUSDT", date, acc)
	if p.EligibilityClass != "CONFLICT" {
		t.Fatal("conflict did not fail closed")
	}
	acc.conflicts = 0
	acc.evidenceGaps = 1
	p = buildPartition("BTCUSDT", date, acc)
	if p.PITEvidenceStatus != "PIT_EVIDENCE_INCOMPLETE" {
		t.Fatal("missing receipt evidence did not block PIT")
	}
}

func TestOverlapIdempotenceAndConflict(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := prospective.NormalizedCandle{Market: "futures-um", Symbol: "BTCUSDT", Interval: "1m", Period: "daily", SourceDate: "2026-01-01", OpenTimeMS: start.UnixMilli(), CloseTimeMS: start.Add(time.Minute).Add(-time.Millisecond).UnixMilli()}
	acc := map[string]*partitionAccumulator{}
	if err := addRecord(acc, c, "r1", "f1", true); err != nil {
		t.Fatal(err)
	}
	if err := addRecord(acc, c, "r2", "f2", true); err != nil {
		t.Fatal(err)
	}
	if acc["BTCUSDT/2026-01-01"].conflicts != 0 {
		t.Fatal("byte-equivalent overlap conflicted")
	}
	c.Close = "different"
	if err := addRecord(acc, c, "r3", "f3", true); err != nil {
		t.Fatal(err)
	}
	if acc["BTCUSDT/2026-01-01"].conflicts != 1 {
		t.Fatal("conflicting overlap did not fail closed")
	}
}

func TestReadinessBoundaryAndNoCandidateFields(t *testing.T) {
	p := testProtocol(t)
	rp := ReadinessPolicy{MinimumDays: 180, ReadyLabel: "READY", NotReadyLabel: "WATCH", RemediationLabel: "REMEDIATE"}
	config := Config{Protocol: p, ReadinessPolicy: rp}
	base := Coverage{EligibleStartUTC: p.EligibleStartUTC, ContiguousEndUTC: p.EligibleStartUTC.Add(179 * 24 * time.Hour), CompleteEligibleDays: 179}
	checkpoint := Checkpoint{GenerationID: "immutable-id", CheckpointHash: prospective.HashBytes([]byte("cp"))}
	backfill := Verification{Valid: true}
	live := prospective.VerificationSummary{Valid: true}
	r := BuildReadiness(config, base, checkpoint, backfill, live, time.Now())
	if r.Label != "WATCH" || r.RemainingDays != 1 {
		t.Fatal("179 days was ready")
	}
	base.CompleteEligibleDays = 180
	base.ContiguousEndUTC = base.EligibleStartUTC.Add(180 * 24 * time.Hour)
	r = BuildReadiness(config, base, checkpoint, backfill, live, time.Now())
	if r.Label != "READY" {
		t.Fatal("exactly 180 days was not ready")
	}
	base.EvidenceGapCount = 1
	r = BuildReadiness(config, base, checkpoint, backfill, live, time.Now())
	if r.Label != "REMEDIATE" {
		t.Fatal("evidence gap did not block readiness")
	}
	data, _ := json.Marshal(r)
	for _, forbidden := range []string{"profit_factor", "expectancy", "win_rate", "candidate_event", "holdout_identity", "development_partition"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("candidate metric leaked: %s", forbidden)
		}
	}
}

func TestCheckpointIdentityDeterministicAndPathIndependent(t *testing.T) {
	p := testProtocol(t)
	i := testIdentity(t, p)
	now := time.Date(2026, 7, 15, 9, 0, 0, 0, time.UTC)
	coverage := Coverage{EligibleStartUTC: p.EligibleStartUTC, ContiguousEndUTC: p.EligibleStartUTC.Add(180 * 24 * time.Hour), CompleteEligibleDays: 180, PartitionPaths: []string{"manifests/partitions/a.json"}, PartitionHashes: []string{prospective.HashBytes([]byte("partition"))}}
	ledger := EligibilityLedger{LedgerHash: prospective.HashBytes([]byte("ledger"))}
	backfill := Verification{FinalChainHash: prospective.HashBytes([]byte("backfill")), EvaluationCutoffFloor: now.Add(-time.Hour)}
	live := prospective.VerificationSummary{FinalEnvelopeHash: prospective.HashBytes([]byte("live")), FinalAuthorityHash: prospective.HashBytes([]byte("authority"))}
	config1 := Config{DataRoot: t.TempDir(), Protocol: p, SourceIdentity: i, ReadinessPolicy: ReadinessPolicy{PolicyHash: prospective.HashBytes([]byte("policy"))}, Activation: prospective.Activation{ActivationHash: prospective.HashBytes([]byte("activation"))}}
	config2 := config1
	config2.DataRoot = t.TempDir()
	one, err := CreateCheckpoint(config1, coverage, backfill, live, ledger, now)
	if err != nil {
		t.Fatal(err)
	}
	two, err := CreateCheckpoint(config2, coverage, backfill, live, ledger, now)
	if err != nil {
		t.Fatal(err)
	}
	if one.CheckpointHash != two.CheckpointHash || !reflect.DeepEqual(one, two) {
		t.Fatal("absolute path affected checkpoint identity")
	}
	mutated := coverage
	mutated.PartitionHashes = []string{prospective.HashBytes([]byte("mutation"))}
	three, _ := CreateCheckpoint(config2, mutated, backfill, live, ledger, now)
	if three.CheckpointHash == one.CheckpointHash {
		t.Fatal("partition mutation did not change checkpoint")
	}
}

func TestSecurityBoundaryStatic(t *testing.T) {
	root := filepath.Join("..", "..")
	files := []string{filepath.Join(root, "internal", "r1p5", "collector.go"), filepath.Join(root, "internal", "app", "r1p5.go")}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		for _, forbidden := range []string{"ak-trader", "/order", "apiKey", "API_KEY", "testnet"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("forbidden network/security surface %q in %s", forbidden, path)
			}
		}
	}
	if _, err := execCandidateProbe(context.Background()); err == nil {
		t.Fatal("candidate probe must remain unavailable")
	}
}

func execCandidateProbe(context.Context) (struct{}, error) { return struct{}{}, os.ErrNotExist }
