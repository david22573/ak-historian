package universe

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/david22573/ak-historian/internal/lifecycle"
)

func TestUniverseSnapshotEvidenceRemainsElevatedWhenPartial(t *testing.T) {
	dir := t.TempDir()
	lifecycleManifest := &lifecycle.Manifest{
		SchemaVersion:                            "1.0.0",
		ManifestVersion:                          "1.0.0",
		LifecycleID:                              "snapshot_lifecycle",
		LifecycleName:                            "Snapshot Lifecycle",
		SourceRepo:                               "ak-historian",
		SourceGitSHA:                             "test",
		SourceType:                               "exchange_snapshot",
		Exchange:                                 "binance",
		MarketType:                               "futures_um",
		QuoteAsset:                               "USDT",
		EffectiveStartUTC:                        "2024-01-01T00:00:00Z",
		EffectiveEndUTC:                          "2024-01-31T23:59:59Z",
		ExchangeMetadataSnapshotHash:             "snapshot_hash",
		ExchangeMetadataSnapshotCoverageStartUTC: "2024-01-15T00:00:00Z",
		ExchangeMetadataSnapshotCoverageEndUTC:   "2024-01-15T00:00:00Z",
		ExchangeMetadataSnapshotEvidenceLevel:    lifecycle.EvidenceHistoricalSnapshot,
		ExchangeMetadataSnapshotCurrentOnly:      true,
		PointInTimeCoverageStatus:                "CURRENT_ONLY",
		Symbols: []lifecycle.SymbolEntry{
			{
				Symbol:        "BTCUSDT",
				BaseAsset:     "BTC",
				QuoteAsset:    "USDT",
				MarketType:    "futures_um",
				Exchange:      "binance",
				Status:        lifecycle.StatusActive,
				ListedAtUTC:   "2024-01-01T00:00:00Z",
				DelistedAtUTC: lifecycle.StatusUnknown,
				FirstSeenUTC:  "2024-01-15T00:00:00Z",
				LastSeenUTC:   "2024-01-15T00:00:00Z",
				EvidenceLevel: lifecycle.EvidenceHistoricalSnapshot,
				Sources: []lifecycle.SourceEntry{
					{
						SourceType:      "exchange_snapshot",
						SourceName:      "fixture",
						SourceURIOrPath: "snapshot.json",
						SourceHash:      "snapshot_hash",
						ObservedAtUTC:   "2024-01-15T00:00:00Z",
						EvidenceFields:  []string{"listed_at_utc", "first_seen_utc", "last_seen_utc"},
						Confidence:      "MEDIUM",
					},
				},
			},
		},
	}
	lifecycle.Validate(lifecycleManifest)
	lifecycleManifest.Hashes = lifecycle.ComputeHashes(lifecycleManifest)
	data, err := json.MarshalIndent(lifecycleManifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "asset_lifecycle_manifest.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	manifest, err := (&Builder{
		UniverseID:                 "u",
		UniverseName:               "u",
		SourceRepo:                 "ak-historian",
		SourceGitSha:               "test",
		SourceType:                 "asset_lifecycle_manifest",
		EffectiveStartUTC:          "2024-01-01T00:00:00Z",
		EffectiveEndUTC:            "2024-01-31T23:59:59Z",
		QuoteAsset:                 "USDT",
		MarketType:                 "futures_um",
		UniversePolicy:             PolicyPointInTimeExchangeUniverse,
		IncludesDelistedAssets:     "unknown",
		AssetLifecycleManifestPath: path,
	}).Build()
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SurvivorshipBiasRisk == RiskLow {
		t.Fatalf("snapshot evidence should not lower risk to LOW")
	}
	if !universeHasWarning(manifest, CodeUniverseSnapshotArchiveCurrentOnly) {
		t.Fatalf("expected current-only snapshot warning")
	}
	if !universeHasWarning(manifest, CodeUniversePointInTimeEvidencePartial) {
		t.Fatalf("expected partial point-in-time warning")
	}
}

func universeHasWarning(manifest *Manifest, code string) bool {
	for _, warning := range manifest.Warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}
