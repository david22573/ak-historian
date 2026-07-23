package exchange_meta

import (
	"context"
	"path/filepath"
	"testing"
)

func TestCollectArchive(t *testing.T) {
	tmpDir := t.TempDir()
	archiveRoot := filepath.Join(tmpDir, "archive")

	fixturePath := filepath.Join("..", "..", "testdata", "exchange", "binance_futures_exchangeInfo_small.json")

	opts := CollectOptions{
		Exchange:        "binance",
		MarketType:      "futures_um",
		ArchiveRoot:     archiveRoot,
		RawJSONPath:     fixturePath,
		WriteRaw:        true,
		RefreshManifest: true,
	}

	report1, err := CollectArchive(context.Background(), opts)
	if err != nil {
		t.Fatalf("CollectArchive failed: %v", err)
	}

	if report1.DuplicateSnapshotDetected {
		t.Errorf("expected no duplicate on first run")
	}

	// Verify paths
	if len(report1.FilesWritten) == 0 {
		t.Errorf("expected files written")
	}

	report2, err := CollectArchive(context.Background(), opts)
	if err != nil {
		t.Fatalf("CollectArchive run 2 failed: %v", err)
	}

	if !report2.DuplicateSnapshotDetected {
		t.Errorf("expected duplicate on second run")
	}

	// verify archive logic
	vOpts := VerifyOptions{
		ArchiveRoot: archiveRoot,
		Exchange:    "binance",
		MarketType:  "futures_um",
		Strict:      true,
	}
	vReport, err := VerifyArchive(vOpts)
	if err != nil {
		t.Fatalf("VerifyArchive failed: %v", err)
	}
	if !vReport.Valid {
		t.Errorf("VerifyArchive failed: %v", vReport.Errors)
	}
}
