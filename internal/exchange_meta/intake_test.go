package exchange_meta

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func createTestFixture(t *testing.T, path, status string, serverTime int64) {
	fixture := map[string]interface{}{
		"serverTime": serverTime,
		"symbols": []map[string]interface{}{
			{
				"symbol":       "BTCUSDT",
				"status":       status,
				"baseAsset":    "BTC",
				"quoteAsset":   "USDT",
				"contractType": "PERPETUAL",
			},
		},
	}
	data, err := json.Marshal(fixture)
	if err != nil {
		t.Fatalf("Failed to create fixture: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("Failed to write fixture: %v", err)
	}
}

func TestPerformBackfillIntake(t *testing.T) {
	tempDir := t.TempDir()
	archiveRoot := filepath.Join(tempDir, "archive")
	inputDir := filepath.Join(tempDir, "inputs")
	os.MkdirAll(inputDir, 0755)

	fixture1Path := filepath.Join(inputDir, "2023-01-01T00_00_00Z.json")
	fixture2Path := filepath.Join(inputDir, "2023-01-02T00_00_00Z.json")

	// Create fixtures. One with serverTime, one without.
	createTestFixture(t, fixture1Path, StatusTrading, 1672531200000)
	createTestFixture(t, fixture2Path, StatusTrading, 0) // No serverTime

	opts := IntakeOptions{
		InputFiles:       []string{fixture1Path, fixture2Path},
		ArchiveRoot:      archiveRoot,
		Exchange:         "binance",
		MarketType:       "futures_um",
		QuoteAssetFilter: "USDT",
		TrustLevel:       TrustLevelUserProvidedVerifiedHash,
		RefreshManifest:  true,
		DryRun:           false,
	}

	report, err := PerformBackfillIntake(opts)
	if err != nil {
		t.Fatalf("PerformBackfillIntake failed: %v", err)
	}

	if !report.Validation.IsValid {
		t.Errorf("Expected valid report, got invalid. Warnings: %v", report.Warnings)
	}

	if len(report.ImportedSnapshots) != 2 {
		t.Errorf("Expected 2 imported snapshots, got %d", len(report.ImportedSnapshots))
	}

	// 1. backfill intake is deterministic & 2. input file order does not change intake hash
	optsReverse := opts
	optsReverse.InputFiles = []string{fixture2Path, fixture1Path}
	optsReverse.ArchiveRoot = filepath.Join(t.TempDir(), "archive")
	reportReverse, _ := PerformBackfillIntake(optsReverse)

	if report.Hashes.IntakeHash != reportReverse.Hashes.IntakeHash {
		t.Errorf("Expected deterministic intake hash. Forward: %s, Reverse: %s", report.Hashes.IntakeHash, reportReverse.Hashes.IntakeHash)
	}

	// 3. duplicate snapshots are deduped
	reportDedupe, _ := PerformBackfillIntake(opts)
	if len(reportDedupe.DuplicateSnapshots) != 2 {
		t.Errorf("Expected 2 duplicates on rerun, got %d", len(reportDedupe.DuplicateSnapshots))
	}

	// 7. missing observed time emits warning
	hasMissingWarning := false
	for _, w := range report.Warnings {
		if w.Code == CodeBackfillObservedTimeMissing {
			hasMissingWarning = true
		}
	}
	if !hasMissingWarning {
		t.Error("Expected CodeBackfillObservedTimeMissing warning for fixture2")
	}

	// 11. archive manifest trust summary is deterministic
	manifestPath := filepath.Join(archiveRoot, "binance", "futures_um", "manifests", "exchange_metadata_snapshot_manifest.json")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("Manifest not found: %v", err)
	}
	manifest, err := ReadSnapshotManifest(manifestPath)
	if err != nil {
		t.Fatalf("Read manifest: %v", err)
	}
	if manifest.TrustLevelSummary[TrustLevelUserProvidedVerifiedHash] != 2 {
		t.Errorf("Expected 2 user provided verified hash, got %d", manifest.TrustLevelSummary[TrustLevelUserProvidedVerifiedHash])
	}
}

func TestBackfillObservedTimeFlags(t *testing.T) {
	tempDir := t.TempDir()
	archiveRoot := filepath.Join(tempDir, "archive")
	inputDir := filepath.Join(tempDir, "inputs")
	os.MkdirAll(inputDir, 0755)

	// fixture3 has no serverTime
	fixture3Path := filepath.Join(inputDir, "2023-05-01T12-30-00.json")
	createTestFixture(t, fixture3Path, StatusTrading, 0)

	// 5. observed time from flag is used correctly
	optsFlag := IntakeOptions{
		InputFiles:   []string{fixture3Path},
		ArchiveRoot:  archiveRoot,
		Exchange:     "binance",
		MarketType:   "futures_um",
		ObservedTime: "2023-04-01T00:00:00Z",
		DryRun:       true,
	}
	reportFlag, _ := PerformBackfillIntake(optsFlag)
	if reportFlag.ImportedSnapshots[0].SourceObservedTimeUTC == nil || *reportFlag.ImportedSnapshots[0].SourceObservedTimeUTC != "2023-04-01T00:00:00Z" {
		t.Errorf("Expected observed time from flag, got %v", reportFlag.ImportedSnapshots[0].SourceObservedTimeUTC)
	}

	// 6. observed time from filename is parsed correctly
	optsFile := IntakeOptions{
		InputFiles:               []string{fixture3Path},
		ArchiveRoot:              archiveRoot,
		Exchange:                 "binance",
		MarketType:               "futures_um",
		ObservedTimeFromFilename: true,
		FilenameTimeLayout:       "2006-01-02T15-04-05",
		DryRun:                   true,
	}
	reportFile, _ := PerformBackfillIntake(optsFile)
	expectedFileTime := "2023-05-01T12:30:00Z"
	if reportFile.ImportedSnapshots[0].SourceObservedTimeUTC == nil || *reportFile.ImportedSnapshots[0].SourceObservedTimeUTC != expectedFileTime {
		t.Errorf("Expected observed time from filename (%s), got %v", expectedFileTime, reportFile.ImportedSnapshots[0].SourceObservedTimeUTC)
	}
}

func TestBackfillUnverifiedWarning(t *testing.T) {
	tempDir := t.TempDir()
	archiveRoot := filepath.Join(tempDir, "archive")
	fixturePath := filepath.Join(tempDir, "fixture.json")
	createTestFixture(t, fixturePath, StatusTrading, 1672531200000)

	// 8. USER_PROVIDED_UNVERIFIED emits warning
	opts := IntakeOptions{
		InputFiles:  []string{fixturePath},
		ArchiveRoot: archiveRoot,
		Exchange:    "binance",
		MarketType:  "futures_um",
		TrustLevel:  TrustLevelUserProvidedUnverified,
		DryRun:      true,
	}
	report, _ := PerformBackfillIntake(opts)

	hasUnverifiedWarning := false
	for _, w := range report.Warnings {
		if w.Code == CodeBackfillUserProvidedUnverified {
			hasUnverifiedWarning = true
		}
	}
	if !hasUnverifiedWarning {
		t.Error("Expected CodeBackfillUserProvidedUnverified warning")
	}
}

func TestBackfillUnsupportedJSON(t *testing.T) {
	tempDir := t.TempDir()
	archiveRoot := filepath.Join(tempDir, "archive")
	fixturePath := filepath.Join(tempDir, "fixture.json")
	os.WriteFile(fixturePath, []byte(`{"invalid": true}`), 0644)

	// 9. unsupported JSON fails gracefully
	opts := IntakeOptions{
		InputFiles:  []string{fixturePath},
		ArchiveRoot: archiveRoot,
		Exchange:    "binance",
		MarketType:  "futures_um",
		DryRun:      true,
	}
	report, _ := PerformBackfillIntake(opts)
	if len(report.SkippedFiles) != 1 {
		t.Errorf("Expected 1 skipped file for invalid JSON, got %d", len(report.SkippedFiles))
	}
}

func TestBackfillDryRun(t *testing.T) {
	tempDir := t.TempDir()
	archiveRoot := filepath.Join(tempDir, "archive")
	fixturePath := filepath.Join(tempDir, "fixture.json")
	createTestFixture(t, fixturePath, StatusTrading, 1672531200000)

	// 10. dry-run writes nothing
	opts := IntakeOptions{
		InputFiles:  []string{fixturePath},
		ArchiveRoot: archiveRoot,
		Exchange:    "binance",
		MarketType:  "futures_um",
		DryRun:      true,
	}
	PerformBackfillIntake(opts)

	if _, err := os.Stat(filepath.Join(archiveRoot, "binance")); !os.IsNotExist(err) {
		t.Error("Expected dry-run to not write anything, but binance directory exists")
	}
}
