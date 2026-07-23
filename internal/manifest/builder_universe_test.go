package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/david22573/ak-historian/internal/universe"
)

func TestDatasetManifestConsumesUniverseManifest(t *testing.T) {
	// Create a temporary universe manifest
	uManifest := &universe.Manifest{
		UniverseID:             "test_universe",
		UniversePolicy:         universe.PolicyExplicitSymbolList,
		SurvivorshipBiasRisk:   universe.RiskHigh,
		IncludesDelistedAssets: "unknown",
		LifecycleID:            "lifecycle_1",
		LifecycleHash:          "life_hash_123",
		LifecycleManifestHash:  "life_manifest_hash_123",
		LifecycleEvidenceLevelSummary: map[string]int{
			"LOCAL_DATA_FIRST_SEEN": 1,
		},
		LifecycleWarnings:         []string{"LIFECYCLE_LOCAL_DATA_ONLY_NOT_LISTING_PROOF"},
		ListingEvidenceStatus:     "FIRST_SEEN_ONLY",
		DelistingEvidenceStatus:   "MISSING",
		SurvivorshipSupportStatus: "ELEVATED",
		Symbols: []universe.SymbolEntry{
			{Symbol: "BTCUSDT"},
		},
		Warnings: []universe.Warning{
			{Code: "TEST_WARNING", Message: "A test warning"},
		},
		Hashes: universe.Hashes{
			UniverseHash: "u_hash_123",
			ManifestHash: "m_hash_123",
		},
	}

	dir := t.TempDir()
	uPath := filepath.Join(dir, "universe_manifest.json")
	b, _ := json.Marshal(uManifest)
	_ = os.WriteFile(uPath, b, 0644)

	builder := &Builder{
		UniverseManifestPath: uPath,
		Symbols:              []string{"BTCUSDT"},
		DataRoot:             dir,
	}

	manifest, err := builder.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if manifest.Survivorship.UniverseID != "test_universe" {
		t.Errorf("Expected UniverseID test_universe, got %s", manifest.Survivorship.UniverseID)
	}
	if manifest.Survivorship.UniverseHash != "u_hash_123" {
		t.Errorf("Expected UniverseHash u_hash_123, got %s", manifest.Survivorship.UniverseHash)
	}
	if manifest.Survivorship.UniverseManifestHash != "m_hash_123" {
		t.Errorf("Expected UniverseManifestHash m_hash_123, got %s", manifest.Survivorship.UniverseManifestHash)
	}
	if manifest.Survivorship.LifecycleHash != "life_hash_123" {
		t.Errorf("Expected LifecycleHash life_hash_123, got %s", manifest.Survivorship.LifecycleHash)
	}
	if manifest.Survivorship.LifecycleManifestHash != "life_manifest_hash_123" {
		t.Errorf("Expected LifecycleManifestHash life_manifest_hash_123, got %s", manifest.Survivorship.LifecycleManifestHash)
	}
	if manifest.Survivorship.LifecycleEvidenceLevelSummary["LOCAL_DATA_FIRST_SEEN"] != 1 {
		t.Errorf("Expected lifecycle evidence summary to be preserved")
	}
	if manifest.Survivorship.ListingEvidenceStatus != "FIRST_SEEN_ONLY" || manifest.Survivorship.DelistingEvidenceStatus != "MISSING" {
		t.Errorf("Expected lifecycle evidence statuses to be preserved")
	}

	foundWarning := false
	for _, w := range manifest.Survivorship.Warnings {
		if w == "TEST_WARNING" {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("Expected TEST_WARNING to flow into dataset manifest")
	}
	foundValidationWarning := false
	for _, w := range manifest.Validation.Warnings {
		if w == "TEST_WARNING" {
			foundValidationWarning = true
			break
		}
	}
	if !foundValidationWarning {
		t.Errorf("Expected TEST_WARNING to flow into dataset manifest validation")
	}
}

func TestDatasetSymbolNotInUniverseProducesWarning(t *testing.T) {
	uManifest := &universe.Manifest{
		Symbols: []universe.SymbolEntry{
			{Symbol: "BTCUSDT"},
		},
	}
	dir := t.TempDir()
	uPath := filepath.Join(dir, "universe_manifest.json")
	b, _ := json.Marshal(uManifest)
	_ = os.WriteFile(uPath, b, 0644)

	builder := &Builder{
		UniverseManifestPath: uPath,
		Symbols:              []string{"BTCUSDT", "ETHUSDT"}, // ETHUSDT is not in universe
		DataRoot:             dir,
	}

	manifest, err := builder.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	found := false
	for _, w := range manifest.Validation.Warnings {
		if w == "DATASET_SYMBOL_NOT_IN_UNIVERSE" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected DATASET_SYMBOL_NOT_IN_UNIVERSE warning")
	}
	if manifest.Validation.Status != "FAIL" {
		t.Errorf("Expected validation status FAIL, got %s", manifest.Validation.Status)
	}
}

func TestDatasetDateRangeOutsideUniverseWindowProducesWarning(t *testing.T) {
	if !rangeOutsideUniverse(
		"2023-12-31T23:59:59Z",
		"2024-01-31T23:59:59Z",
		"2024-01-01T00:00:00Z",
		"2024-12-31T23:59:59Z",
	) {
		t.Fatalf("Expected rangeOutsideUniverse to detect dataset start before universe window")
	}
	if !rangeOutsideUniverse(
		"2024-01-01T00:00:00Z",
		"2025-01-01T00:00:00Z",
		"2024-01-01T00:00:00Z",
		"2024-12-31T23:59:59Z",
	) {
		t.Fatalf("Expected rangeOutsideUniverse to detect dataset end after universe window")
	}
}

func TestManifestHashRemainsDeterministic(t *testing.T) {
	uManifest := &universe.Manifest{
		UniversePolicy: universe.PolicyExplicitSymbolList,
		Hashes: universe.Hashes{
			UniverseHash: "u_hash",
		},
	}
	dir := t.TempDir()
	uPath := filepath.Join(dir, "universe_manifest.json")
	b, _ := json.Marshal(uManifest)
	_ = os.WriteFile(uPath, b, 0644)

	builder1 := &Builder{
		UniverseManifestPath: uPath,
		DataRoot:             dir,
	}
	m1, _ := builder1.Build()

	builder2 := &Builder{
		UniverseManifestPath: uPath,
		DataRoot:             dir,
	}
	m2, _ := builder2.Build()

	if m1.Hashes.ManifestHash != m2.Hashes.ManifestHash {
		t.Errorf("Manifest hashes should be deterministic")
	}
}
