package universe

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/david22573/ak-historian/internal/lifecycle"
)

func writeLifecycleManifestFixture(t *testing.T, dir string, symbols []lifecycle.SymbolEntry) string {
	t.Helper()
	m := &lifecycle.Manifest{
		SchemaVersion:     "1.0.0",
		ManifestVersion:   "1.0.0",
		LifecycleID:       "fixture_lifecycle",
		LifecycleName:     "Fixture Lifecycle",
		SourceRepo:        "ak-historian",
		SourceGitSHA:      "test",
		SourceType:        "fixture",
		GeneratedAtUTC:    "2026-07-07T00:00:00Z",
		Exchange:          "binance",
		MarketType:        "futures",
		QuoteAsset:        "USDT",
		EffectiveStartUTC: "2024-01-01T00:00:00Z",
		EffectiveEndUTC:   "2024-12-31T23:59:59Z",
		Symbols:           symbols,
	}
	lifecycle.Validate(m)
	m.Hashes = lifecycle.ComputeHashes(m)

	path := filepath.Join(dir, "asset_lifecycle_manifest.json")
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal lifecycle fixture: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write lifecycle fixture: %v", err)
	}
	return path
}

func localLifecycleSymbol(symbol string) lifecycle.SymbolEntry {
	return lifecycle.SymbolEntry{
		Symbol:        symbol,
		BaseAsset:     "BTC",
		QuoteAsset:    "USDT",
		MarketType:    "futures",
		Exchange:      "binance",
		Status:        lifecycle.StatusUnknown,
		ListedAtUTC:   lifecycle.StatusUnknown,
		DelistedAtUTC: lifecycle.StatusUnknown,
		FirstSeenUTC:  "2024-01-01T00:00:00Z",
		LastSeenUTC:   "2024-01-31T23:59:00Z",
		EvidenceLevel: lifecycle.EvidenceLocalDataFirstSeen,
		Sources: []lifecycle.SourceEntry{
			{
				SourceType:      "local_data",
				SourceName:      symbol,
				SourceURIOrPath: "symbol=" + symbol,
				SourceHash:      "hash-" + symbol,
				ObservedAtUTC:   "2024-01-31T23:59:59Z",
				EvidenceFields:  []string{"first_seen_utc", "last_seen_utc"},
				Confidence:      "MEDIUM",
			},
		},
	}
}

func verifiedLifecycleSymbol(symbol, listed, delisted string) lifecycle.SymbolEntry {
	base := "BTC"
	if symbol == "ETHUSDT" {
		base = "ETH"
	}
	return lifecycle.SymbolEntry{
		Symbol:        symbol,
		BaseAsset:     base,
		QuoteAsset:    "USDT",
		MarketType:    "futures",
		Exchange:      "binance",
		Status:        lifecycle.StatusActive,
		ListedAtUTC:   listed,
		DelistedAtUTC: delisted,
		FirstSeenUTC:  listed,
		LastSeenUTC:   delisted,
		EvidenceLevel: lifecycle.EvidenceHistoricalSnapshot,
		Sources: []lifecycle.SourceEntry{
			{
				SourceType:      "exchange_snapshot",
				SourceName:      symbol,
				SourceURIOrPath: "fixtures/exchange_snapshot.json",
				SourceHash:      "hash-" + symbol,
				ObservedAtUTC:   "2024-12-31T00:00:00Z",
				EvidenceFields:  []string{"listed_at_utc", "delisted_at_utc"},
				Confidence:      "HIGH",
			},
		},
	}
}

func hasUniverseWarning(warnings []Warning, code string) bool {
	for _, w := range warnings {
		if w.Code == code {
			return true
		}
	}
	return false
}

func TestUniverseManifestConsumesAssetLifecycleManifest(t *testing.T) {
	dir := t.TempDir()
	path := writeLifecycleManifestFixture(t, dir, []lifecycle.SymbolEntry{localLifecycleSymbol("BTCUSDT")})

	m, err := (&Builder{
		UniversePolicy:             PolicyLocalDataDiscoveredSymbols,
		AssetLifecycleManifestPath: path,
		EffectiveStartUTC:          "2024-01-01T00:00:00Z",
		EffectiveEndUTC:            "2024-01-31T23:59:59Z",
		QuoteAsset:                 "USDT",
		MarketType:                 "futures",
	}).Build()
	if err != nil {
		t.Fatalf("build universe: %v", err)
	}

	if m.LifecycleID != "fixture_lifecycle" || m.LifecycleHash == "" || m.LifecycleManifestHash == "" {
		t.Fatalf("lifecycle hashes not embedded: %#v", m)
	}
	if len(m.Symbols) != 1 || m.Symbols[0].Symbol != "BTCUSDT" {
		t.Fatalf("expected lifecycle symbols to populate universe, got %#v", m.Symbols)
	}
	if m.Symbols[0].FirstSeenUTC == nil || *m.Symbols[0].FirstSeenUTC != "2024-01-01T00:00:00Z" {
		t.Fatalf("first_seen_utc not copied from lifecycle: %#v", m.Symbols[0])
	}
	if !hasUniverseWarning(m.Warnings, CodeUniverseLifecycleEvidenceWeak) {
		t.Fatalf("missing %s", CodeUniverseLifecycleEvidenceWeak)
	}
	if m.SurvivorshipBiasRisk == RiskLow {
		t.Fatalf("local lifecycle evidence must not produce LOW risk")
	}
}

func TestUniverseSymbolsMissingLifecycleEvidenceProduceWarnings(t *testing.T) {
	dir := t.TempDir()
	path := writeLifecycleManifestFixture(t, dir, []lifecycle.SymbolEntry{localLifecycleSymbol("BTCUSDT")})

	m, err := (&Builder{
		UniversePolicy:             PolicyExplicitSymbolList,
		AssetLifecycleManifestPath: path,
		Symbols:                    []string{"BTCUSDT", "ETHUSDT"},
		EffectiveStartUTC:          "2024-01-01T00:00:00Z",
		EffectiveEndUTC:            "2024-01-31T23:59:59Z",
		QuoteAsset:                 "USDT",
		MarketType:                 "futures",
	}).Build()
	if err != nil {
		t.Fatalf("build universe: %v", err)
	}
	if m.Validation.IsValid {
		t.Fatalf("missing lifecycle symbol should fail validation")
	}
	if !hasUniverseWarning(m.Warnings, CodeUniverseSymbolMissingLifecycle) {
		t.Fatalf("missing %s", CodeUniverseSymbolMissingLifecycle)
	}
}

func TestUniverseSymbolsInactiveDuringWindowProduceWarnings(t *testing.T) {
	dir := t.TempDir()
	path := writeLifecycleManifestFixture(t, dir, []lifecycle.SymbolEntry{
		verifiedLifecycleSymbol("BTCUSDT", "2025-01-01T00:00:00Z", "2025-12-31T00:00:00Z"),
	})

	m, err := (&Builder{
		UniversePolicy:             PolicyPointInTimeExchangeUniverse,
		AssetLifecycleManifestPath: path,
		EffectiveStartUTC:          "2024-01-01T00:00:00Z",
		EffectiveEndUTC:            "2024-01-31T23:59:59Z",
		QuoteAsset:                 "USDT",
		MarketType:                 "futures",
		IncludesDelistedAssets:     "true",
		SurvivorshipBiasRisk:       RiskLow,
	}).Build()
	if err != nil {
		t.Fatalf("build universe: %v", err)
	}
	if m.Validation.IsValid {
		t.Fatalf("inactive during universe window should fail validation")
	}
	if !hasUniverseWarning(m.Warnings, CodeUniverseSymbolNotActiveDuringWindow) {
		t.Fatalf("missing %s", CodeUniverseSymbolNotActiveDuringWindow)
	}
}

func TestLocalDataLifecycleDoesNotVerifyPointInTimeUniverse(t *testing.T) {
	dir := t.TempDir()
	path := writeLifecycleManifestFixture(t, dir, []lifecycle.SymbolEntry{localLifecycleSymbol("BTCUSDT")})

	m, err := (&Builder{
		UniversePolicy:             PolicyPointInTimeExchangeUniverse,
		AssetLifecycleManifestPath: path,
		EffectiveStartUTC:          "2024-01-01T00:00:00Z",
		EffectiveEndUTC:            "2024-01-31T23:59:59Z",
		QuoteAsset:                 "USDT",
		MarketType:                 "futures",
		IncludesDelistedAssets:     "true",
		SurvivorshipBiasRisk:       RiskLow,
	}).Build()
	if err != nil {
		t.Fatalf("build universe: %v", err)
	}
	if m.SurvivorshipBiasRisk == RiskLow {
		t.Fatalf("local data lifecycle should not verify LOW point-in-time risk")
	}
	if !hasUniverseWarning(m.Warnings, CodeUniverseLifecycleEvidenceWeak) {
		t.Fatalf("missing %s", CodeUniverseLifecycleEvidenceWeak)
	}
	if !hasUniverseWarning(m.Warnings, CodeUniverseLowRiskUnproven) {
		t.Fatalf("missing %s", CodeUniverseLowRiskUnproven)
	}
}

func TestLifecycleBackedUniverseManifestHashIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	path := writeLifecycleManifestFixture(t, dir, []lifecycle.SymbolEntry{localLifecycleSymbol("BTCUSDT")})
	builder := &Builder{
		UniversePolicy:             PolicyLocalDataDiscoveredSymbols,
		AssetLifecycleManifestPath: path,
		EffectiveStartUTC:          "2024-01-01T00:00:00Z",
		EffectiveEndUTC:            "2024-01-31T23:59:59Z",
		QuoteAsset:                 "USDT",
		MarketType:                 "futures",
	}
	m1, err := builder.Build()
	if err != nil {
		t.Fatalf("build m1: %v", err)
	}
	m2, err := builder.Build()
	if err != nil {
		t.Fatalf("build m2: %v", err)
	}
	if m1.Hashes.ManifestHash != m2.Hashes.ManifestHash {
		t.Fatalf("manifest hash not deterministic: %s vs %s", m1.Hashes.ManifestHash, m2.Hashes.ManifestHash)
	}
}

func TestSurvivorshipRiskElevatedWhenDelistingEvidenceMissing(t *testing.T) {
	dir := t.TempDir()
	path := writeLifecycleManifestFixture(t, dir, []lifecycle.SymbolEntry{
		verifiedLifecycleSymbol("BTCUSDT", "2024-01-01T00:00:00Z", lifecycle.StatusUnknown),
	})

	m, err := (&Builder{
		UniversePolicy:             PolicyPointInTimeExchangeUniverse,
		AssetLifecycleManifestPath: path,
		EffectiveStartUTC:          "2024-01-01T00:00:00Z",
		EffectiveEndUTC:            "2024-01-31T23:59:59Z",
		QuoteAsset:                 "USDT",
		MarketType:                 "futures",
		IncludesDelistedAssets:     "true",
		SurvivorshipBiasRisk:       RiskLow,
	}).Build()
	if err != nil {
		t.Fatalf("build universe: %v", err)
	}
	if m.SurvivorshipBiasRisk == RiskLow {
		t.Fatalf("missing delisting evidence should keep risk elevated")
	}
	if !hasUniverseWarning(m.Warnings, CodeUniverseDelistingEvidenceMissing) {
		t.Fatalf("missing %s", CodeUniverseDelistingEvidenceMissing)
	}
}
