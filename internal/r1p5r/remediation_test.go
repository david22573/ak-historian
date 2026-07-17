package r1p5r

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/david22573/ak-historian/internal/prospective"
)

type fixtureProvider struct {
	requestAt  time.Time
	serverTime time.Time
	klines     int
}

func (provider *fixtureProvider) ServerTime(context.Context) (time.Time, string, prospective.ResponseEvidence, error) {
	body := []byte(`{"serverTime":1784102400000}`)
	return provider.serverTime, prospective.HashBytes(body), prospective.ResponseEvidence{RequestStartUTC: provider.requestAt, ResponseHeadersReceivedUTC: provider.requestAt.Add(time.Millisecond), CompleteResponseReceivedUTC: provider.requestAt.Add(2 * time.Millisecond), HTTPStatus: 200, ProviderHTTPDate: provider.requestAt.Format(time.RFC1123), ProviderHTTPDateUTC: provider.requestAt, Body: body}, nil
}

func (provider *fixtureProvider) KlinesRange(_ context.Context, _ string, startMS, endMS int64) (prospective.ResponseEvidence, string, error) {
	provider.klines++
	rows := make([][]any, 0, int((endMS-startMS)/60000)+1)
	for opened := startMS; opened <= endMS; opened += 60000 {
		rows = append(rows, []any{opened, "1", "2", "0.5", "1.5", "10", opened + 59999, "12", 3, "4", "5", "0"})
	}
	body, _ := json.Marshal(rows)
	return prospective.ResponseEvidence{RequestStartUTC: provider.requestAt, ResponseHeadersReceivedUTC: provider.requestAt.Add(time.Millisecond), CompleteResponseReceivedUTC: provider.requestAt.Add(2 * time.Millisecond), HTTPStatus: 200, ProviderHTTPDate: provider.requestAt.Format(time.RFC1123), ProviderHTTPDateUTC: provider.requestAt, Body: body}, "fixture", nil
}

func fixtureCollector(t *testing.T) (*Collector, *fixtureProvider) {
	t.Helper()
	protocol := testProtocol(t)
	identity := testIdentity(t, protocol)
	seal := testSeal(t, protocol, identity)
	root := filepath.Join(t.TempDir(), protocol.StorageNamespace)
	config := Config{DataRoot: root, Protocol: protocol, SourceIdentity: identity, PreacquisitionSeal: seal, AbandonedRegistry: testRegistry(t, protocol)}
	provider := &fixtureProvider{requestAt: seal.VerificationCompletedAtUTC.Add(time.Hour), serverTime: seal.VerificationCompletedAtUTC.Add(24 * time.Hour)}
	clock := prospective.ClockEvidence{CheckedAtUTC: provider.requestAt, Synchronized: true, EvidenceHash: prospective.HashBytes([]byte("clock"))}
	collector := &Collector{Config: config, Store: NewStore(root, protocol, identity, seal), Client: provider, Clock: prospective.StaticClockChecker{Evidence: clock}, Now: func() time.Time { return provider.requestAt.Add(time.Second) }}
	return collector, provider
}

func TestCollectorFetchesNewResponsesIntoNewChain(t *testing.T) {
	collector, provider := fixtureCollector(t)
	status, err := collector.CollectAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !status.Complete || provider.klines != len(prospective.UniqueSymbols) || status.Requests != len(prospective.UniqueSymbols) || status.ChainTerminal == ZeroHash || status.ChainTerminal == collector.Config.AbandonedRegistry.OldBackfillTerminal {
		t.Fatalf("new reacquisition did not build an independent complete chain: %+v requests=%d", status, provider.klines)
	}
	verification, err := collector.Store.VerifyAll(provider.requestAt.Add(2 * time.Hour))
	if err != nil || !verification.Valid || verification.RawResponseCount != len(prospective.UniqueSymbols) || verification.FragmentCount != len(prospective.UniqueSymbols) {
		t.Fatalf("newly fetched evidence did not verify: %+v %v", verification, err)
	}
}

func TestOldCachedBytesAreRejectedAfterNewFetch(t *testing.T) {
	collector, provider := fixtureCollector(t)
	path, err := collector.Store.absolute("raw/symbol=ADAUSDT/start=20260101T000000Z.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	if err := prospective.WriteAtomic(path, []byte("abandoned cached bytes"), 0o444); err != nil {
		t.Fatal(err)
	}
	if _, err := collector.CollectAll(context.Background()); err == nil || provider.klines != 1 {
		t.Fatalf("cached bytes were not rejected after a new provider fetch: %v requests=%d", err, provider.klines)
	}
}

func TestAbandonedReceiptAndCursorCannotEnterNewState(t *testing.T) {
	protocol := testProtocol(t)
	identity := testIdentity(t, protocol)
	seal := testSeal(t, protocol, identity)
	store := NewStore(t.TempDir(), protocol, identity, seal)
	receipt, raw, fragment := testReceipt(t, protocol, identity)
	receipt.RepairSourceCommit = "59951efd756a8024455608d298c5534c778e5121"
	receipt = reseal(receipt)
	if _, err := store.Commit(receipt, raw, fragment); err == nil {
		t.Fatal("abandoned R1P5 receipt entered the new ledger")
	}

	cursorRoot := t.TempDir()
	cursorStore := NewStore(cursorRoot, protocol, identity, seal)
	oldState := []byte(`{"schema_version":"ak-historian.pr4b0-r1p5.backfill-state.v1"}`)
	if err := prospective.WriteAtomic(cursorStore.StatePath(), oldState, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := cursorStore.RebuildState(); err == nil {
		t.Fatal("abandoned cursor initialized new state")
	}
}

func TestReceiptRejectsOldIdentitiesAndPreSealOrdering(t *testing.T) {
	protocol := testProtocol(t)
	identity := testIdentity(t, protocol)
	seal := testSeal(t, protocol, identity)
	base, _, _ := testReceipt(t, protocol, identity)
	cases := []struct {
		name   string
		mutate func(*Receipt)
	}{
		{"old source", func(receipt *Receipt) { receipt.RepairSourceCommit = "59951efd756a8024455608d298c5534c778e5121" }},
		{"old seal", func(receipt *Receipt) { receipt.SourceSealCommit = "59951efd756a8024455608d298c5534c778e5121" }},
		{"old terminal", func(receipt *Receipt) { receipt.PriorReceiptChainHash = AbandonedBackfillTerminal }},
		{"binary", func(receipt *Receipt) { receipt.SealedBinaryHash = prospective.HashBytes([]byte("mutation")) }},
		{"protocol", func(receipt *Receipt) { receipt.ProtocolHash = prospective.HashBytes([]byte("mutation")) }},
		{"pre-seal request", func(receipt *Receipt) {
			receipt.RequestStartUTC = seal.VerificationCompletedAtUTC.Add(-time.Nanosecond)
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			mutated := base
			test.mutate(&mutated)
			mutated = reseal(mutated)
			if VerifyReceipt(mutated, protocol, identity, seal) == nil {
				t.Fatal("abandoned or unsealed authority was accepted")
			}
		})
	}
}

func TestIdentityRegistryAndBinaryMutationChangeIdentity(t *testing.T) {
	protocol := testProtocol(t)
	identity := testIdentity(t, protocol)
	mutatedIdentity := identity
	mutatedIdentity.RepairSourceCommit = "3333333333333333333333333333333333333333"
	mutatedIdentity.IdentityHash, _ = prospective.HashCanonical(mutatedIdentity, "identity_hash")
	if mutatedIdentity.IdentityHash == identity.IdentityHash {
		t.Fatal("source mutation did not change source identity")
	}

	registry := testRegistry(t, protocol)
	registry.RegistryHash, _ = prospective.HashCanonical(registry, "registry_hash")
	mutatedRegistry := registry
	mutatedRegistry.ProhibitedInputs = append(mutatedRegistry.ProhibitedInputs, "legacy-allowlist")
	mutatedRegistry.RegistryHash, _ = prospective.HashCanonical(mutatedRegistry, "registry_hash")
	if mutatedRegistry.RegistryHash == registry.RegistryHash {
		t.Fatal("quarantine registry mutation did not change its hash")
	}

	binary := filepath.Join(t.TempDir(), "collector")
	if err := os.WriteFile(binary, []byte("sealed"), 0o755); err != nil {
		t.Fatal(err)
	}
	expected := prospective.HashBytes([]byte("sealed"))
	if err := VerifyBinaryFile(binary, expected); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, []byte("mutated"), 0o755); err != nil {
		t.Fatal(err)
	}
	if VerifyBinaryFile(binary, expected) == nil {
		t.Fatal("mutated binary retained sealed identity")
	}
}

func TestP4AndR1P5RLocksCannotConflict(t *testing.T) {
	root := t.TempDir()
	protocol := testProtocol(t)
	identity := testIdentity(t, protocol)
	r1p5rLock := NewStore(filepath.Join(root, "r1p5r"), protocol, identity, testSeal(t, protocol, identity)).LockPath()
	p4Lock := prospective.NewStore(filepath.Join(root, "p4"), prospective.Activation{}).LockPath()
	if r1p5rLock == p4Lock || filepath.Dir(r1p5rLock) == filepath.Dir(p4Lock) {
		t.Fatal("P4 and R1P5R lock namespaces overlap")
	}
}

func TestOldCheckpointAndMissingSymbolCannotProduceReady(t *testing.T) {
	protocol := testProtocol(t)
	identity := testIdentity(t, protocol)
	seal := testSeal(t, protocol, identity)
	registry := testRegistry(t, protocol)
	config := Config{Protocol: protocol, SourceIdentity: identity, PreacquisitionSeal: seal, AbandonedRegistry: registry, ReadinessPolicy: ReadinessPolicy{MinimumDays: 180}}
	coverage := Coverage{EligibleStartUTC: protocol.EligibleStartUTC, ContiguousEndUTC: protocol.EligibleStartUTC.Add(180 * 24 * time.Hour), CompleteEligibleDays: 180}
	for _, symbol := range prospective.UniqueSymbols {
		coverage.PerSymbol = append(coverage.PerSymbol, SymbolCoverage{Symbol: symbol, CompleteUTCDateCount: 180})
	}
	valid := Checkpoint{SchemaVersion: CheckpointVersion, GenerationID: "r1p5r-checkpoint-new", CheckpointHash: prospective.HashBytes([]byte("new")), RepairSourceCommit: identity.RepairSourceCommit, SourceSealCommit: seal.SourceSealCommit, SealedBinaryHash: seal.BinarySHA256, ProtocolHash: protocol.ProtocolHash, AbandonedRegistryHash: registry.RegistryHash, BackfillChainTerminal: prospective.HashBytes([]byte("new-chain"))}
	verification := Verification{Valid: true}
	live := prospective.VerificationSummary{Valid: true}

	old := valid
	old.GenerationID = registry.OldCheckpointID
	old.CheckpointHash = registry.OldCheckpointHash
	old.BackfillChainTerminal = registry.OldBackfillTerminal
	if readiness := BuildReadiness(config, coverage, old, verification, live, valid.CreatedAtUTC); readiness.Label == "PR4B0_R1P5R_SOURCE_REPAIRED_AND_COVERAGE_REACQUIRED" {
		t.Fatal("old checkpoint produced ready")
	}
	coverage.PerSymbol = coverage.PerSymbol[:len(coverage.PerSymbol)-1]
	if readiness := BuildReadiness(config, coverage, valid, verification, live, valid.CreatedAtUTC); readiness.Label == "PR4B0_R1P5R_SOURCE_REPAIRED_AND_COVERAGE_REACQUIRED" {
		t.Fatal("missing symbol produced ready")
	}
}

func TestNoLegacyAllowlistOrResearchSurface(t *testing.T) {
	for _, path := range []string{"config.go", "collector.go", "coverage.go", "store.go", filepath.Join("..", "app", "r1p5r.go")} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		lower := strings.ToLower(string(data))
		for _, forbidden := range []string{"allow_legacy", "legacy_allow", "downtrendmidvolrelieflong240m", "profit_factor", "candidate_event_count"} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("forbidden bypass or research surface %q in %s", forbidden, path)
			}
		}
	}
}
