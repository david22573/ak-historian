package lifecycle

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/david22573/ak-historian/internal/parquetutil"
	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/writer"
)

func writeLifecycleParquetFixture(t *testing.T, path string, times []int64) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir fixture: %v", err)
	}
	fw, err := local.NewLocalFileWriter(path)
	if err != nil {
		t.Fatalf("create parquet writer: %v", err)
	}
	defer fw.Close()

	pw, err := writer.NewParquetWriter(fw, new(parquetutil.OpenTimeRow), 4)
	if err != nil {
		t.Fatalf("create parquet row writer: %v", err)
	}
	defer pw.WriteStop()

	for _, ts := range times {
		if err := pw.Write(parquetutil.OpenTimeRow{OpenTimeMS: ts}); err != nil {
			t.Fatalf("write parquet row: %v", err)
		}
	}
}

func buildLocalLifecycle(t *testing.T, root string) *Manifest {
	t.Helper()
	m, err := (&Builder{
		LifecycleID:       "test_lifecycle",
		LifecycleName:     "Test Lifecycle",
		SourceType:        "local_data",
		DataRoot:          root,
		Exchange:          "binance",
		MarketType:        "futures",
		QuoteAsset:        "USDT",
		EffectiveStartUTC: "2024-01-01T00:00:00Z",
		EffectiveEndUTC:   "2024-01-31T23:59:59Z",
	}).Build()
	if err != nil {
		t.Fatalf("build local lifecycle: %v", err)
	}
	return m
}

func testSymbol(symbol string) SymbolEntry {
	return SymbolEntry{
		Symbol:        symbol,
		BaseAsset:     inferBaseAsset(symbol, "USDT"),
		QuoteAsset:    "USDT",
		MarketType:    "futures",
		Exchange:      "binance",
		Status:        StatusActive,
		ListedAtUTC:   "2024-01-01T00:00:00Z",
		DelistedAtUTC: "2024-12-31T00:00:00Z",
		FirstSeenUTC:  "2024-01-01T00:00:00Z",
		LastSeenUTC:   "2024-01-02T00:00:00Z",
		EvidenceLevel: EvidenceHistoricalSnapshot,
		Sources: []SourceEntry{
			{
				SourceType:      "exchange_snapshot",
				SourceName:      "fixture",
				SourceURIOrPath: "fixtures/snapshot.json",
				SourceHash:      "hash-" + symbol,
				ObservedAtUTC:   "2024-12-31T00:00:00Z",
				EvidenceFields:  []string{"listed_at_utc", "delisted_at_utc"},
				Confidence:      "HIGH",
			},
		},
	}
}

func buildManualLifecycle(symbols []SymbolEntry) *Manifest {
	m := &Manifest{
		SchemaVersion:     "1.0.0",
		ManifestVersion:   "1.0.0",
		LifecycleID:       "manual",
		LifecycleName:     "Manual",
		SourceRepo:        "ak-historian",
		SourceGitSHA:      "test",
		SourceType:        "exchange_snapshot",
		GeneratedAtUTC:    "2026-07-07T00:00:00Z",
		Exchange:          "binance",
		MarketType:        "futures",
		QuoteAsset:        "USDT",
		EffectiveStartUTC: "2024-01-01T00:00:00Z",
		EffectiveEndUTC:   "2024-12-31T00:00:00Z",
		Symbols:           symbols,
	}
	Validate(m)
	m.Hashes = ComputeHashes(m)
	return m
}

func hasLifecycleWarning(warnings []Warning, code string) bool {
	for _, w := range warnings {
		if w.Code == code {
			return true
		}
	}
	return false
}

func TestLocalDataLifecycleManifestIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	path := filepath.Join(dir, "candles", "futures-um", "1m", "symbol=BTCUSDT", "year=2024", "month=01", "BTCUSDT-1m-2024-01-01.parquet")
	writeLifecycleParquetFixture(t, path, []int64{start, start + 60000})

	m1 := buildLocalLifecycle(t, dir)
	m2 := buildLocalLifecycle(t, dir)

	if m1.Hashes.LifecycleHash != m2.Hashes.LifecycleHash {
		t.Fatalf("lifecycle hashes differ: %s vs %s", m1.Hashes.LifecycleHash, m2.Hashes.LifecycleHash)
	}
	if m1.Hashes.ManifestHash != m2.Hashes.ManifestHash {
		t.Fatalf("manifest hashes differ: %s vs %s", m1.Hashes.ManifestHash, m2.Hashes.ManifestHash)
	}
	if got := m1.Symbols[0].EvidenceLevel; got != EvidenceLocalDataFirstSeen {
		t.Fatalf("evidence_level=%s want %s", got, EvidenceLocalDataFirstSeen)
	}
	if m1.Symbols[0].ListedAtUTC != StatusUnknown || m1.Symbols[0].DelistedAtUTC != StatusUnknown {
		t.Fatalf("local data must not claim listing/delisting dates: %#v", m1.Symbols[0])
	}
}

func TestSymbolOrderDoesNotAffectLifecycleHash(t *testing.T) {
	m1 := buildManualLifecycle([]SymbolEntry{testSymbol("BTCUSDT"), testSymbol("ETHUSDT")})
	m2 := buildManualLifecycle([]SymbolEntry{testSymbol("ETHUSDT"), testSymbol("BTCUSDT")})
	if m1.Hashes.LifecycleHash != m2.Hashes.LifecycleHash {
		t.Fatalf("symbol order changed lifecycle hash")
	}
}

func TestGeneratedAtDoesNotAffectLifecycleHash(t *testing.T) {
	m := buildManualLifecycle([]SymbolEntry{testSymbol("BTCUSDT")})
	original := m.Hashes
	m.GeneratedAtUTC = "2030-01-01T00:00:00Z"
	next := ComputeHashes(m)
	if original.LifecycleHash != next.LifecycleHash {
		t.Fatalf("generated_at changed lifecycle hash")
	}
	if original.ManifestHash != next.ManifestHash {
		t.Fatalf("generated_at changed manifest hash")
	}
}

func TestChangingFirstSeenLastSeenChangesLifecycleHash(t *testing.T) {
	s1 := testSymbol("BTCUSDT")
	s2 := testSymbol("BTCUSDT")
	s2.FirstSeenUTC = "2024-01-02T00:00:00Z"
	s2.LastSeenUTC = "2024-01-03T00:00:00Z"
	m1 := buildManualLifecycle([]SymbolEntry{s1})
	m2 := buildManualLifecycle([]SymbolEntry{s2})
	if m1.Hashes.LifecycleHash == m2.Hashes.LifecycleHash {
		t.Fatalf("changing first/last seen should change lifecycle hash")
	}
}

func TestAddingSymbolChangesLifecycleHash(t *testing.T) {
	m1 := buildManualLifecycle([]SymbolEntry{testSymbol("BTCUSDT")})
	m2 := buildManualLifecycle([]SymbolEntry{testSymbol("BTCUSDT"), testSymbol("ETHUSDT")})
	if m1.Hashes.LifecycleHash == m2.Hashes.LifecycleHash {
		t.Fatalf("adding symbol should change lifecycle hash")
	}
}

func TestDuplicateSymbolsAreDetected(t *testing.T) {
	m := buildManualLifecycle([]SymbolEntry{testSymbol("BTCUSDT"), testSymbol("BTCUSDT")})
	if m.Validation.IsValid {
		t.Fatalf("duplicate symbols should fail validation")
	}
	if !hasLifecycleWarning(m.Warnings, CodeLifecycleDuplicateSymbol) {
		t.Fatalf("missing %s", CodeLifecycleDuplicateSymbol)
	}
}

func TestListedAfterDelistedFailsValidation(t *testing.T) {
	s := testSymbol("BTCUSDT")
	s.ListedAtUTC = "2024-02-01T00:00:00Z"
	s.DelistedAtUTC = "2024-01-01T00:00:00Z"
	m := buildManualLifecycle([]SymbolEntry{s})
	if m.Validation.IsValid {
		t.Fatalf("listed after delisted should fail validation")
	}
	if !hasLifecycleWarning(m.Warnings, CodeLifecycleListedAfterDelisted) {
		t.Fatalf("missing %s", CodeLifecycleListedAfterDelisted)
	}
}

func TestLocalDataOnlyEvidenceEmitsWarning(t *testing.T) {
	s := testSymbol("BTCUSDT")
	s.ListedAtUTC = StatusUnknown
	s.DelistedAtUTC = StatusUnknown
	s.Status = StatusUnknown
	s.EvidenceLevel = EvidenceLocalDataFirstSeen
	m := buildManualLifecycle([]SymbolEntry{s})
	if !hasLifecycleWarning(m.Warnings, CodeLifecycleLocalDataOnlyNotListingProof) {
		t.Fatalf("missing %s", CodeLifecycleLocalDataOnlyNotListingProof)
	}
	if m.Validation.SurvivorshipSupportStatus == SupportLowSupported {
		t.Fatalf("local data only should not support LOW survivorship risk")
	}
}

func TestCurrentActiveOnlyEvidenceEmitsWarning(t *testing.T) {
	s := testSymbol("BTCUSDT")
	s.ListedAtUTC = StatusUnknown
	s.DelistedAtUTC = StatusUnknown
	s.EvidenceLevel = EvidenceCurrentActiveOnly
	m := buildManualLifecycle([]SymbolEntry{s})
	if !hasLifecycleWarning(m.Warnings, CodeLifecycleCurrentActiveOnlyRisk) {
		t.Fatalf("missing %s", CodeLifecycleCurrentActiveOnlyRisk)
	}
}

func TestUserProvidedUnverifiedCannotProduceLowSupport(t *testing.T) {
	s := testSymbol("BTCUSDT")
	s.EvidenceLevel = EvidenceUserProvidedUnverified
	s.Sources[0].SourceHash = StatusUnknown
	m := buildManualLifecycle([]SymbolEntry{s})
	if m.Validation.SurvivorshipSupportStatus == SupportLowSupported {
		t.Fatalf("user-provided unverified evidence must not support LOW survivorship risk")
	}
	if !hasLifecycleWarning(m.Warnings, CodeLifecycleUserProvidedUnverified) {
		t.Fatalf("missing %s", CodeLifecycleUserProvidedUnverified)
	}
	if !hasLifecycleWarning(m.Warnings, CodeLifecycleLowRiskUnproven) {
		t.Fatalf("missing %s", CodeLifecycleLowRiskUnproven)
	}
}

func TestMalformedSymbolsAreDetected(t *testing.T) {
	m := buildManualLifecycle([]SymbolEntry{testSymbol("btc/usdt")})
	if m.Validation.IsValid {
		t.Fatalf("malformed symbol should fail validation")
	}
	if !hasLifecycleWarning(m.Warnings, CodeLifecycleSymbolMalformed) {
		t.Fatalf("missing %s", CodeLifecycleSymbolMalformed)
	}
}

func TestEmptyLifecycleFailsValidation(t *testing.T) {
	m := buildManualLifecycle(nil)
	if m.Validation.IsValid {
		t.Fatalf("empty lifecycle should fail validation")
	}
	if !hasLifecycleWarning(m.Warnings, CodeLifecycleEmpty) {
		t.Fatalf("missing %s", CodeLifecycleEmpty)
	}
}
