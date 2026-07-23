package pitcoverage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/david22573/ak-historian/internal/lifecycle"
	"github.com/david22573/ak-historian/internal/universe"
)

func TestBuilder_MissingLifecycle(t *testing.T) {
	tmpDir := t.TempDir()

	umPath := filepath.Join(tmpDir, "universe.json")
	lmPath := filepath.Join(tmpDir, "lifecycle.json")

	um := universe.Manifest{
		UniversePolicy: "POINT_IN_TIME_EXCHANGE_UNIVERSE",
		Symbols: []universe.SymbolEntry{
			{Symbol: "BTCUSDT", ActiveDuringWindow: true},
		},
	}
	umData, _ := json.Marshal(um)
	os.WriteFile(umPath, umData, 0644)

	lm := lifecycle.Manifest{
		Symbols: []lifecycle.SymbolEntry{},
	}
	lmData, _ := json.Marshal(lm)
	os.WriteFile(lmPath, lmData, 0644)

	b := &Builder{
		LifecycleManifestPath: lmPath,
		UniverseManifestPath:  umPath,
		ResearchStartUTC:      time.Now().UTC().Format(time.RFC3339),
		ResearchEndUTC:        time.Now().UTC().Format(time.RFC3339),
	}

	report, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.OverallStatus != StatusPitNotEligible {
		t.Errorf("expected PIT_NOT_ELIGIBLE, got %s", report.OverallStatus)
	}

	if len(report.Symbols) != 1 {
		t.Fatalf("expected 1 symbol in report, got %d", len(report.Symbols))
	}
	if report.Symbols[0].PointInTimeStatus != SymStatusMissingLifecycle {
		t.Errorf("expected MISSING_LIFECYCLE, got %s", report.Symbols[0].PointInTimeStatus)
	}
}

func TestBuilder_PerfectWindow(t *testing.T) {
	tmpDir := t.TempDir()

	umPath := filepath.Join(tmpDir, "universe.json")
	lmPath := filepath.Join(tmpDir, "lifecycle.json")

	um := universe.Manifest{
		UniversePolicy:       "POINT_IN_TIME_EXCHANGE_UNIVERSE",
		SurvivorshipBiasRisk: "LOW",
		Symbols: []universe.SymbolEntry{
			{Symbol: "BTCUSDT", ActiveDuringWindow: true},
		},
	}
	umData, _ := json.Marshal(um)
	os.WriteFile(umPath, umData, 0644)

	delisted := "2023-01-01T00:00:00Z"
	lm := lifecycle.Manifest{
		Symbols: []lifecycle.SymbolEntry{
			{
				Symbol:        "BTCUSDT",
				EvidenceLevel: lifecycle.EvidenceVerifiedExchangeDelisting,
				DelistedAtUTC: delisted,
				Sources: []lifecycle.SourceEntry{
					{
						ObservedAtUTC: "2022-01-01T00:00:00Z",
					},
				},
			},
		},
	}
	lmData, _ := json.Marshal(lm)
	os.WriteFile(lmPath, lmData, 0644)

	b := &Builder{
		LifecycleManifestPath: lmPath,
		UniverseManifestPath:  umPath,
		ResearchStartUTC:      "2021-01-01T00:00:00Z",
		ResearchEndUTC:        "2022-01-01T00:00:00Z",
	}

	report, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.OverallStatus != StatusPitEligible {
		t.Errorf("expected PIT_ELIGIBLE, got %s", report.OverallStatus)
	}
	if report.PromotionRecommendation != PromoAllowStrict {
		t.Errorf("expected ALLOW_STRICT_PROMOTION, got %s", report.PromotionRecommendation)
	}
}
