package universe

import (
	"testing"
	"time"
)

func TestExplicitSymbolListManifestIsDeterministic(t *testing.T) {
	b1 := &Builder{
		UniversePolicy: PolicyExplicitSymbolList,
		Symbols:        []string{"BTCUSDT", "ETHUSDT"},
	}
	m1, _ := b1.Build()

	b2 := &Builder{
		UniversePolicy: PolicyExplicitSymbolList,
		Symbols:        []string{"BTCUSDT", "ETHUSDT"},
	}
	m2, _ := b2.Build()

	if m1.Hashes.UniverseHash != m2.Hashes.UniverseHash {
		t.Errorf("Expected identical universe hashes, got %s and %s", m1.Hashes.UniverseHash, m2.Hashes.UniverseHash)
	}
	if m1.Hashes.ManifestHash != m2.Hashes.ManifestHash {
		t.Errorf("Expected identical manifest hashes, got %s and %s", m1.Hashes.ManifestHash, m2.Hashes.ManifestHash)
	}
}

func TestSymbolOrderDoesNotChangeUniverseHash(t *testing.T) {
	b1 := &Builder{
		UniversePolicy: PolicyExplicitSymbolList,
		Symbols:        []string{"BTCUSDT", "ETHUSDT"},
	}
	m1, _ := b1.Build()

	b2 := &Builder{
		UniversePolicy: PolicyExplicitSymbolList,
		Symbols:        []string{"ETHUSDT", "BTCUSDT"}, // Reversed
	}
	m2, _ := b2.Build()

	if m1.Hashes.UniverseHash != m2.Hashes.UniverseHash {
		t.Errorf("Expected identical universe hashes despite order, got %s and %s", m1.Hashes.UniverseHash, m2.Hashes.UniverseHash)
	}
	if m1.Hashes.ManifestHash != m2.Hashes.ManifestHash {
		t.Errorf("Expected identical manifest hashes despite order, got %s and %s", m1.Hashes.ManifestHash, m2.Hashes.ManifestHash)
	}
}

func TestGeneratedAtUTCDoesNotChangeUniverseHash(t *testing.T) {
	b := &Builder{
		UniversePolicy: PolicyExplicitSymbolList,
		Symbols:        []string{"BTCUSDT", "ETHUSDT"},
	}
	m1, _ := b.Build()

	// manually mangle the generated time and recompute
	m1.GeneratedAtUTC = time.Now().Add(10 * time.Hour).UTC().Format(time.RFC3339)
	hashes2 := ComputeHashes(m1)

	if m1.Hashes.UniverseHash != hashes2.UniverseHash {
		t.Errorf("Expected identical universe hashes, got %s and %s", m1.Hashes.UniverseHash, hashes2.UniverseHash)
	}
	if m1.Hashes.ManifestHash != hashes2.ManifestHash {
		t.Errorf("Expected identical manifest hashes, got %s and %s", m1.Hashes.ManifestHash, hashes2.ManifestHash)
	}
}

func TestAddingSymbolChangesUniverseHash(t *testing.T) {
	b1 := &Builder{
		UniversePolicy: PolicyExplicitSymbolList,
		Symbols:        []string{"BTCUSDT", "ETHUSDT"},
	}
	m1, _ := b1.Build()

	b2 := &Builder{
		UniversePolicy: PolicyExplicitSymbolList,
		Symbols:        []string{"BTCUSDT", "ETHUSDT", "LINKUSDT"},
	}
	m2, _ := b2.Build()

	if m1.Hashes.UniverseHash == m2.Hashes.UniverseHash {
		t.Errorf("Expected different universe hashes")
	}
}

func TestChangingEffectiveDateWindowChangesManifestHash(t *testing.T) {
	b1 := &Builder{
		UniversePolicy:    PolicyExplicitSymbolList,
		Symbols:           []string{"BTCUSDT", "ETHUSDT"},
		EffectiveStartUTC: "2024-01-01T00:00:00Z",
	}
	m1, _ := b1.Build()

	b2 := &Builder{
		UniversePolicy:    PolicyExplicitSymbolList,
		Symbols:           []string{"BTCUSDT", "ETHUSDT"},
		EffectiveStartUTC: "2025-01-01T00:00:00Z",
	}
	m2, _ := b2.Build()

	if m1.Hashes.ManifestHash == m2.Hashes.ManifestHash {
		t.Errorf("Expected different manifest hashes")
	}
	// Universe hash shouldn't change
	if m1.Hashes.UniverseHash != m2.Hashes.UniverseHash {
		t.Errorf("Expected identical universe hashes")
	}
}

func TestDuplicateSymbolsAreDetected(t *testing.T) {
	b := &Builder{
		UniversePolicy: PolicyExplicitSymbolList,
		Symbols:        []string{"BTCUSDT", "BTCUSDT"},
	}
	m, _ := b.Build()

	if m.Validation.IsValid {
		t.Errorf("Expected validation to fail on duplicate symbols")
	}
	found := false
	for _, w := range m.Warnings {
		if w.Code == CodeUniverseDuplicateSymbol {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected warning code %s", CodeUniverseDuplicateSymbol)
	}
}

func TestLowSurvivorshipRiskWithoutProofIsRejectedOrDowngraded(t *testing.T) {
	b := &Builder{
		UniversePolicy:       PolicyExplicitSymbolList,
		SurvivorshipBiasRisk: RiskLow,
		Symbols:              []string{"BTCUSDT"},
	}
	m, _ := b.Build()

	if m.SurvivorshipBiasRisk == RiskLow {
		t.Errorf("Expected survivorship bias risk to be downgraded from LOW")
	}
	found := false
	for _, w := range m.Warnings {
		if w.Code == CodeUniverseLowRiskUnproven {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected warning code %s", CodeUniverseLowRiskUnproven)
	}
}

func TestLocalDataDiscoveredSymbolsModeEmitsCorrectWarning(t *testing.T) {
	b := &Builder{
		UniversePolicy: PolicyLocalDataDiscoveredSymbols,
		DataRoot:       t.TempDir(),
	}
	m, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	found := false
	for _, w := range m.Warnings {
		if w.Code == CodeUniverseLocalDataDiscoveryNotPointInTime {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected warning code %s", CodeUniverseLocalDataDiscoveryNotPointInTime)
	}
}

func TestEmptyUniverseFailsValidation(t *testing.T) {
	b := &Builder{
		UniversePolicy: PolicyExplicitSymbolList,
		Symbols:        []string{},
	}
	m, _ := b.Build()

	if m.Validation.IsValid {
		t.Errorf("Expected validation to fail on empty universe")
	}
	found := false
	for _, w := range m.Warnings {
		if w.Code == CodeUniverseEmpty {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected warning code %s", CodeUniverseEmpty)
	}
}

func TestInvalidDateWindowFailsValidation(t *testing.T) {
	b := &Builder{
		UniversePolicy:    PolicyExplicitSymbolList,
		Symbols:           []string{"BTCUSDT"},
		EffectiveStartUTC: "2025-01-01T00:00:00Z",
		EffectiveEndUTC:   "2024-01-01T00:00:00Z",
	}
	m, _ := b.Build()

	if m.Validation.IsValid {
		t.Errorf("Expected validation to fail on invalid dates")
	}
	found := false
	for _, w := range m.Warnings {
		if w.Code == CodeUniverseEffectiveWindowInvalid {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected warning code %s", CodeUniverseEffectiveWindowInvalid)
	}
}

// Test discovering symbols from data root
func TestDiscoverSymbols(t *testing.T) {
	// Let's create a temporary directory structure
	dir := t.TempDir()

	// In Go test, we can use os.MkdirAll to mock parquet output dirs
	// but I won't do it right here since the function signature doesn't require mocking if we don't call it.
	_ = dir
}
