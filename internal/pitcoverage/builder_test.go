package pitcoverage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/david22573/ak-historian/internal/exchange_meta"
	"github.com/david22573/ak-historian/internal/lifecycle"
	"github.com/david22573/ak-historian/internal/manifest"
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
	um.Hashes = universe.ComputeHashes(&um)
	umData, _ := json.Marshal(um)
	os.WriteFile(umPath, umData, 0644)

	lm := lifecycle.Manifest{
		Symbols: []lifecycle.SymbolEntry{},
	}
	lm.Hashes = lifecycle.ComputeHashes(&lm)
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
	um.Hashes = universe.ComputeHashes(&um)
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
						Confidence:    exchange_meta.TrustLevelOfficialArchive,
					},
				},
			},
		},
	}
	lm.Hashes = lifecycle.ComputeHashes(&lm)
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
	if report.PromotionRecommendation != PromoExploratoryOnly {
		t.Errorf("expected EXPLORATORY_ONLY for legacy PIT, got %s", report.PromotionRecommendation)
	}
}

func TestBuilder_UnverifiedUnknownEvidenceAlwaysIneligible(t *testing.T) {
	for _, allowUnverified := range []bool{false, true} {
		t.Run(map[bool]string{false: "default", true: "compatibility_flag"}[allowUnverified], func(t *testing.T) {
			dir := t.TempDir()
			um := universe.Manifest{
				UniversePolicy:       "POINT_IN_TIME_EXCHANGE_UNIVERSE",
				SurvivorshipBiasRisk: universe.RiskUnknown,
				Symbols:              []universe.SymbolEntry{{Symbol: "BTCUSDT", ActiveDuringWindow: true}},
			}
			um.Hashes = universe.ComputeHashes(&um)
			lm := lifecycle.Manifest{Symbols: []lifecycle.SymbolEntry{{
				Symbol:        "BTCUSDT",
				EvidenceLevel: lifecycle.EvidenceUserProvidedUnverified,
				DelistedAtUTC: lifecycle.StatusUnknown,
				Sources: []lifecycle.SourceEntry{{
					ObservedAtUTC: lifecycle.StatusUnknown,
					Confidence:    "MEDIUM",
				}},
			}}}
			lm.Hashes = lifecycle.ComputeHashes(&lm)
			umPath := writePITTestJSON(t, dir, "universe.json", um)
			lmPath := writePITTestJSON(t, dir, "lifecycle.json", lm)

			report, err := (&Builder{
				LifecycleManifestPath: lmPath,
				UniverseManifestPath:  umPath,
				ResearchStartUTC:      "2021-01-01T00:00:00Z",
				ResearchEndUTC:        "2022-01-01T00:00:00Z",
				AllowUnverified:       allowUnverified,
			}).Build()
			if err != nil {
				t.Fatal(err)
			}
			if report.OverallStatus != StatusPitNotEligible || report.PromotionRecommendation != PromoBlockStrict || report.SurvivorshipBiasRisk != RiskHigh || report.Validation.IsValid {
				t.Fatalf("unverified evidence escaped containment: %+v", report)
			}
			if got := report.Symbols[0].PointInTimeStatus; got != SymStatusUnverifiedOnly {
				t.Fatalf("point_in_time_status=%s", got)
			}
			if err := ValidateReport(report); err != nil {
				t.Fatalf("internally consistent blocked report rejected: %v", err)
			}

			tampered := *report
			tampered.OverallStatus = StatusPitEligible
			if err := ValidateReport(&tampered); err == nil {
				t.Fatal("status tampering was accepted")
			}
			tampered.Hashes.CoverageHash, _ = tampered.ComputeCoverageHash()
			tampered.Hashes.ReportHash, _ = tampered.ComputeReportHash()
			if err := ValidateReport(&tampered); err == nil {
				t.Fatal("rehashed ineligible symbol was accepted as eligible")
			}
		})
	}
}

func writePITTestJSON(t *testing.T, dir, name string, value any) string {
	t.Helper()
	path := filepath.Join(dir, name)
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLegacyPITLoadersRejectTamperedIdentityAndPaths(t *testing.T) {
	dir := t.TempDir()
	lm := lifecycle.Manifest{Symbols: []lifecycle.SymbolEntry{{Symbol: "BTCUSDT", EvidenceLevel: lifecycle.EvidenceUnknown}}}
	lm.Hashes = lifecycle.ComputeHashes(&lm)
	lm.Symbols[0].EvidenceLevel = lifecycle.EvidenceVerifiedExchangeListing
	lmPath := writePITTestJSON(t, dir, "lifecycle.json", lm)
	if _, err := loadLifecycleManifest(lmPath); err == nil || !strings.Contains(err.Error(), "hash mismatch") {
		t.Fatalf("tampered lifecycle manifest accepted: %v", err)
	}

	um := universe.Manifest{SurvivorshipBiasRisk: universe.RiskHigh, Symbols: []universe.SymbolEntry{{Symbol: "BTCUSDT"}}}
	um.Hashes = universe.ComputeHashes(&um)
	um.SurvivorshipBiasRisk = universe.RiskLow
	umPath := writePITTestJSON(t, dir, "universe.json", um)
	if _, err := loadUniverseManifest(umPath); err == nil || !strings.Contains(err.Error(), "hash mismatch") {
		t.Fatalf("tampered universe manifest accepted: %v", err)
	}

	dataset := manifest.DatasetManifest{
		SchemaVersion: "1.0.0", ManifestVersion: "1.0.0", DatasetID: "dataset",
		Symbols: []string{"BTCUSDT"}, Intervals: []string{"1m"},
		Files:  []manifest.FileEntry{{RelativePath: "BTCUSDT/1m/data.parquet", SHA256: "sha256"}},
		Hashes: manifest.Hashes{DatasetHash: "dataset-hash"},
	}
	dataset.Hashes.ManifestHash, _ = dataset.ComputeHash()
	dataset.Files[0].RelativePath = "../escape.parquet"
	dataset.Hashes.ManifestHash, _ = dataset.ComputeHash()
	datasetPath := writePITTestJSON(t, dir, "dataset.json", dataset)
	if _, err := loadDatasetManifest(datasetPath); err == nil || !strings.Contains(err.Error(), "file identity") {
		t.Fatalf("dataset path escape accepted: %v", err)
	}
}
