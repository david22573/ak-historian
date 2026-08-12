package exchange_meta

import (
	"bytes"
	"context"
	"errors"
	"os"
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

func TestCollectArchiveLatestManifestFailurePreservesPriorPointer(t *testing.T) {
	tmpDir := t.TempDir()
	archiveRoot := filepath.Join(tmpDir, "archive")
	originalFixture := filepath.Join("..", "..", "testdata", "exchange", "binance_futures_exchangeInfo_small.json")
	opts := CollectOptions{Exchange: "binance", MarketType: "futures_um", ArchiveRoot: archiveRoot, RawJSONPath: originalFixture, WriteRaw: true, RefreshManifest: true}
	if _, err := CollectArchive(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	latestPath := filepath.Join(archiveRoot, "binance", "futures_um", "latest", "latest_manifest.json")
	prior, err := os.ReadFile(latestPath)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(originalFixture)
	if err != nil {
		t.Fatal(err)
	}
	changedPath := filepath.Join(tmpDir, "changed.json")
	changed := bytes.Replace(raw, []byte("BTCUSDT"), []byte("XRPUSDT"), 1)
	if bytes.Equal(raw, changed) {
		t.Fatal("fixture did not contain expected symbol")
	}
	if err := os.WriteFile(changedPath, changed, 0644); err != nil {
		t.Fatal(err)
	}
	opts.RawJSONPath = changedPath
	opts.writeManifest = func(path string, manifest *SnapshotManifest) error {
		if filepath.Base(path) == "latest_manifest.json" {
			return errors.New("injected latest publication failure")
		}
		return WriteSnapshotManifest(path, manifest)
	}
	if _, err := CollectArchive(context.Background(), opts); err == nil {
		t.Fatal("CollectArchive() error=nil, want latest publication failure")
	}
	after, err := os.ReadFile(latestPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, prior) {
		t.Fatal("authoritative prior latest manifest changed after failed atomic publication")
	}
}
