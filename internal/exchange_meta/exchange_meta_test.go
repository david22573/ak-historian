package exchange_meta

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

const collectedAt = "2024-01-15T00:00:00Z"

func TestSnapshotNormalizationDeterministic(t *testing.T) {
	raw := rawExchangeInfo("BTCUSDT", "TRADING", 1704067200000, 0)
	first := buildTestSnapshot(t, raw, collectedAt)
	second := buildTestSnapshot(t, raw, collectedAt)
	if first.Hashes.SnapshotHash != second.Hashes.SnapshotHash {
		t.Fatalf("snapshot hash changed: %s != %s", first.Hashes.SnapshotHash, second.Hashes.SnapshotHash)
	}
	if first.Hashes.NormalizedPayloadHash != second.Hashes.NormalizedPayloadHash {
		t.Fatalf("normalized hash changed")
	}
}

func TestSymbolOrderDoesNotChangeSnapshotHash(t *testing.T) {
	rawA := `{"serverTime":1705276800000,"symbols":[` +
		rawSymbol("BTCUSDT", "TRADING", 1704067200000, 0) + `,` +
		rawSymbol("ETHUSDT", "TRADING", 1704067200000, 0) + `]}`
	rawB := `{"symbols":[` +
		rawSymbol("ETHUSDT", "TRADING", 1704067200000, 0) + `,` +
		rawSymbol("BTCUSDT", "TRADING", 1704067200000, 0) + `],"serverTime":1705276800000}`
	first := buildTestSnapshot(t, rawA, collectedAt)
	second := buildTestSnapshot(t, rawB, collectedAt)
	if first.Hashes.SnapshotHash != second.Hashes.SnapshotHash {
		t.Fatalf("snapshot hash should ignore raw symbol order: %s != %s", first.Hashes.SnapshotHash, second.Hashes.SnapshotHash)
	}
}

func TestCollectedAtDoesNotAffectNormalizedPayloadHash(t *testing.T) {
	raw := rawExchangeInfo("BTCUSDT", "TRADING", 1704067200000, 0)
	first := buildTestSnapshot(t, raw, "2024-01-15T00:00:00Z")
	second := buildTestSnapshot(t, raw, "2024-01-16T00:00:00Z")
	if first.Hashes.NormalizedPayloadHash != second.Hashes.NormalizedPayloadHash {
		t.Fatalf("normalized payload hash should ignore collected_at_utc")
	}
}

func TestChangingSymbolStatusChangesSnapshotHash(t *testing.T) {
	trading := buildTestSnapshot(t, rawExchangeInfo("BTCUSDT", "TRADING", 1704067200000, 0), collectedAt)
	delivered := buildTestSnapshot(t, rawExchangeInfo("BTCUSDT", "DELIVERED", 1704067200000, 1705276800000), collectedAt)
	if trading.Hashes.SnapshotHash == delivered.Hashes.SnapshotHash {
		t.Fatalf("status change should change snapshot hash")
	}
}

func TestDuplicateSymbolsDetected(t *testing.T) {
	raw := `{"serverTime":1705276800000,"symbols":[` +
		rawSymbol("BTCUSDT", "TRADING", 1704067200000, 0) + `,` +
		rawSymbol("BTCUSDT", "TRADING", 1704067200000, 0) + `]}`
	snapshot := buildTestSnapshot(t, raw, collectedAt)
	if snapshot.Validation.IsValid {
		t.Fatalf("duplicate symbols should invalidate snapshot")
	}
	if !hasWarning(snapshot.Warnings, CodeDuplicateSymbol) {
		t.Fatalf("expected duplicate symbol warning")
	}
}

func TestUnknownStatusEmitsWarning(t *testing.T) {
	snapshot := buildTestSnapshot(t, rawExchangeInfo("BTCUSDT", "AUCTION_MATCH", 1704067200000, 0), collectedAt)
	if !hasWarning(snapshot.Warnings, CodeStatusUnmapped) {
		t.Fatalf("expected unmapped status warning")
	}
	if snapshot.Symbols[0].Status != StatusUnknown {
		t.Fatalf("status = %s, want UNKNOWN", snapshot.Symbols[0].Status)
	}
}

func TestCurrentOnlySnapshotWarning(t *testing.T) {
	snapshot := buildTestSnapshot(t, rawExchangeInfo("BTCUSDT", "TRADING", 1704067200000, 0), collectedAt)
	if !snapshot.Validation.CurrentOnly {
		t.Fatalf("expected current-only validation flag")
	}
	if !hasWarning(snapshot.Warnings, CodeCurrentOnlySource) {
		t.Fatalf("expected current-only warning")
	}
}

func TestSnapshotManifestDeterministicOrdering(t *testing.T) {
	dir := t.TempDir()
	later := buildTestSnapshot(t, rawExchangeInfo("ETHUSDT", "TRADING", 1704067200000, 0), "2024-01-16T00:00:00Z")
	earlier := buildTestSnapshot(t, rawExchangeInfo("BTCUSDT", "TRADING", 1704067200000, 0), "2024-01-15T00:00:00Z")
	if err := WriteSnapshot(filepath.Join(dir, "b.json"), later); err != nil {
		t.Fatal(err)
	}
	if err := WriteSnapshot(filepath.Join(dir, "a.json"), earlier); err != nil {
		t.Fatal(err)
	}
	manifest, err := BuildSnapshotManifest(ManifestOptions{SnapshotDir: dir, ArchiveID: "test", Exchange: "binance", MarketType: "futures_um"})
	if err != nil {
		t.Fatal(err)
	}
	if got := manifest.Snapshots[0].SnapshotID; got != earlier.SnapshotID {
		t.Fatalf("first snapshot = %s, want %s", got, earlier.SnapshotID)
	}
	repeated, err := BuildSnapshotManifest(ManifestOptions{SnapshotDir: dir, ArchiveID: "test", Exchange: "binance", MarketType: "futures_um"})
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Hashes.ArchiveHash != repeated.Hashes.ArchiveHash {
		t.Fatalf("archive hash changed")
	}
}

func TestReadFixtureSnapshot(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "exchange", "binance_futures_exchangeInfo_small.json"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot := buildTestSnapshot(t, string(data), collectedAt)
	if len(snapshot.Symbols) != 1 || snapshot.Symbols[0].Symbol != "BTCUSDT" {
		t.Fatalf("unexpected fixture symbols: %#v", snapshot.Symbols)
	}
}

func buildTestSnapshot(t *testing.T, raw string, collected string) *Snapshot {
	t.Helper()
	snapshot, err := BuildSnapshot(SnapshotOptions{
		Exchange:         "binance",
		MarketType:       "futures_um",
		QuoteAssetFilter: "USDT",
		SourceType:       "file_import_current",
		SourceName:       "binance_futures_exchangeInfo_v1",
		SourceURI:        "exchangeInfo.json",
		CollectedAtUTC:   collected,
		CollectorGitSHA:  "test",
		RawPayload:       []byte(raw),
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func rawExchangeInfo(symbol, status string, onboard, delivery int64) string {
	return `{"serverTime":1705276800000,"symbols":[` + rawSymbol(symbol, status, onboard, delivery) + `]}`
}

func rawSymbol(symbol, status string, onboard, delivery int64) string {
	base := symbol[:len(symbol)-4]
	return `{"symbol":"` + symbol + `","pair":"` + symbol + `","contractType":"PERPETUAL","deliveryDate":` +
		intString(delivery) + `,"onboardDate":` + intString(onboard) + `,"status":"` + status +
		`","baseAsset":"` + base + `","quoteAsset":"USDT","marginAsset":"USDT","underlyingType":"COIN","permissions":["GRID","DCA"]}`
}

func intString(value int64) string {
	if value == 0 {
		return "0"
	}
	return fmtInt(value)
}

func fmtInt(value int64) string {
	return strconv.FormatInt(value, 10)
}

func hasWarning(warnings []Warning, code string) bool {
	for _, warning := range warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}
