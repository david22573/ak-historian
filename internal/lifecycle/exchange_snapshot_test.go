package lifecycle

import (
	"path/filepath"
	"testing"

	"github.com/david22573/ak-historian/internal/exchange_meta"
)

func TestLifecycleConsumesSingleExchangeSnapshot(t *testing.T) {
	dir := t.TempDir()
	snapshot := lifecycleTestSnapshot(t, lifecycleRaw("BTCUSDT", "TRADING", 1704067200000, 0), "2024-01-15T00:00:00Z")
	snapshotPath := filepath.Join(dir, "snapshot.json")
	if err := exchange_meta.WriteSnapshot(snapshotPath, snapshot); err != nil {
		t.Fatal(err)
	}

	manifest, err := (&Builder{
		SourceType:           "exchange_snapshot",
		Exchange:             "binance",
		MarketType:           "futures_um",
		QuoteAsset:           "USDT",
		EffectiveStartUTC:    "2024-01-01T00:00:00Z",
		EffectiveEndUTC:      "2024-01-31T23:59:59Z",
		ExchangeSnapshotPath: snapshotPath,
	}).Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Symbols) != 1 {
		t.Fatalf("symbols = %d, want 1", len(manifest.Symbols))
	}
	sym := manifest.Symbols[0]
	if sym.EvidenceLevel != EvidenceHistoricalSnapshot {
		t.Fatalf("evidence = %s", sym.EvidenceLevel)
	}
	if sym.ListedAtUTC != "2024-01-01T00:00:00Z" {
		t.Fatalf("listed_at_utc = %s", sym.ListedAtUTC)
	}
	if sym.DelistedAtUTC != StatusUnknown {
		t.Fatalf("delisted_at_utc = %s", sym.DelistedAtUTC)
	}
	if sym.FirstSeenUTC != "2024-01-15T00:00:00Z" || sym.LastSeenUTC != "2024-01-15T00:00:00Z" {
		t.Fatalf("snapshot first/last seen not applied: %#v", sym)
	}
	if manifest.ExchangeMetadataSnapshotHash == "" || !manifest.ExchangeMetadataSnapshotCurrentOnly {
		t.Fatalf("snapshot summary missing: %#v", manifest)
	}
}

func TestLifecycleConsumesSnapshotManifestAndDoesNotInferDelistingFromDisappearance(t *testing.T) {
	dir := t.TempDir()
	first := lifecycleTestSnapshot(t, `{"serverTime":1705276800000,"symbols":[`+
		lifecycleRawSymbol("BTCUSDT", "TRADING", 1704067200000, 0)+`,`+
		lifecycleRawSymbol("ETHUSDT", "TRADING", 1704067200000, 0)+`]}`, "2024-01-15T00:00:00Z")
	second := lifecycleTestSnapshot(t, lifecycleRaw("BTCUSDT", "TRADING", 1704067200000, 0), "2024-01-16T00:00:00Z")
	if err := exchange_meta.WriteSnapshot(filepath.Join(dir, "first.json"), first); err != nil {
		t.Fatal(err)
	}
	if err := exchange_meta.WriteSnapshot(filepath.Join(dir, "second.json"), second); err != nil {
		t.Fatal(err)
	}
	archive, err := exchange_meta.BuildSnapshotManifest(exchange_meta.ManifestOptions{SnapshotDir: dir, ArchiveID: "test", Exchange: "binance", MarketType: "futures_um"})
	if err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(dir, "manifest.json")
	if err := exchange_meta.WriteSnapshotManifest(archivePath, archive); err != nil {
		t.Fatal(err)
	}

	manifest, err := (&Builder{
		SourceType:                   "exchange_snapshot",
		Exchange:                     "binance",
		MarketType:                   "futures_um",
		QuoteAsset:                   "USDT",
		EffectiveStartUTC:            "2024-01-15T00:00:00Z",
		EffectiveEndUTC:              "2024-01-16T00:00:00Z",
		ExchangeSnapshotManifestPath: archivePath,
	}).Build()
	if err != nil {
		t.Fatal(err)
	}
	eth := findLifecycleSymbol(t, manifest, "ETHUSDT")
	if eth.DelistedAtUTC != StatusUnknown {
		t.Fatalf("disappearance should not set delisted_at_utc: %s", eth.DelistedAtUTC)
	}
	if !lifecycleSymbolHasWarning(eth, CodeLifecycleSymbolDisappearedNoProof) {
		t.Fatalf("expected disappearance warning")
	}
	if manifest.ExchangeMetadataSnapshotManifestHash == "" || manifest.ExchangeMetadataSnapshotArchiveHash == "" {
		t.Fatalf("archive summary missing")
	}
}

func TestOnboardAndDeliveryDatesOnlyPopulateWhenSourceFieldsExist(t *testing.T) {
	dir := t.TempDir()
	snapshot := lifecycleTestSnapshot(t, `{"serverTime":1705276800000,"symbols":[`+
		lifecycleRawSymbol("BTCUSDT", "TRADING", 1704067200000, 0)+`,`+
		lifecycleRawSymbol("OLDUSDT", "DELIVERED", 0, 1705276800000)+`]}`, "2024-01-15T00:00:00Z")
	path := filepath.Join(dir, "snapshot.json")
	if err := exchange_meta.WriteSnapshot(path, snapshot); err != nil {
		t.Fatal(err)
	}
	manifest, err := (&Builder{
		SourceType:           "exchange_snapshot",
		Exchange:             "binance",
		MarketType:           "futures_um",
		QuoteAsset:           "USDT",
		EffectiveStartUTC:    "2024-01-01T00:00:00Z",
		EffectiveEndUTC:      "2024-01-31T23:59:59Z",
		ExchangeSnapshotPath: path,
	}).Build()
	if err != nil {
		t.Fatal(err)
	}
	btc := findLifecycleSymbol(t, manifest, "BTCUSDT")
	old := findLifecycleSymbol(t, manifest, "OLDUSDT")
	if btc.ListedAtUTC != "2024-01-01T00:00:00Z" || btc.DelistedAtUTC != StatusUnknown {
		t.Fatalf("BTC dates not populated as expected: %#v", btc)
	}
	if old.ListedAtUTC != StatusUnknown || old.DelistedAtUTC != "2024-01-15T00:00:00Z" {
		t.Fatalf("OLD dates not populated as expected: %#v", old)
	}
}

func lifecycleTestSnapshot(t *testing.T, raw string, collected string) *exchange_meta.Snapshot {
	t.Helper()
	snapshot, err := exchange_meta.BuildSnapshot(exchange_meta.SnapshotOptions{
		Exchange:         "binance",
		MarketType:       "futures_um",
		QuoteAssetFilter: "USDT",
		SourceType:       "file_import_current",
		SourceName:       "binance_futures_exchangeInfo_v1",
		SourceURI:        "exchangeInfo.json",
		CollectedAtUTC:   collected,
		TrustLevel:       exchange_meta.TrustLevelOfficialArchive,
		CollectorGitSHA:  "test",
		RawPayload:       []byte(raw),
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func lifecycleRaw(symbol, status string, onboard, delivery int64) string {
	return `{"serverTime":1705276800000,"symbols":[` + lifecycleRawSymbol(symbol, status, onboard, delivery) + `]}`
}

func lifecycleRawSymbol(symbol, status string, onboard, delivery int64) string {
	base := symbol[:len(symbol)-4]
	return `{"symbol":"` + symbol + `","pair":"` + symbol + `","contractType":"PERPETUAL","deliveryDate":` +
		lifecycleInt(delivery) + `,"onboardDate":` + lifecycleInt(onboard) + `,"status":"` + status +
		`","baseAsset":"` + base + `","quoteAsset":"USDT","marginAsset":"USDT","underlyingType":"COIN","permissions":["GRID"]}`
}

func lifecycleInt(value int64) string {
	switch value {
	case 0:
		return "0"
	case 1704067200000:
		return "1704067200000"
	case 1705276800000:
		return "1705276800000"
	default:
		return "0"
	}
}

func findLifecycleSymbol(t *testing.T, manifest *Manifest, symbol string) SymbolEntry {
	t.Helper()
	for _, sym := range manifest.Symbols {
		if sym.Symbol == symbol {
			return sym
		}
	}
	t.Fatalf("symbol %s not found", symbol)
	return SymbolEntry{}
}

func lifecycleSymbolHasWarning(sym SymbolEntry, code string) bool {
	for _, warning := range sym.Warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}
