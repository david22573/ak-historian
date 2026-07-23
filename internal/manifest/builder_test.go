package manifest_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/david22573/ak-historian/internal/manifest"
)

func TestDeterminism(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	f1 := filepath.Join(tempDir, "candles", "um", "1m", "symbol=BTCUSDT", "BTC-1m.parquet")
	os.MkdirAll(filepath.Dir(f1), 0755)
	os.WriteFile(f1, []byte("test data A"), 0644)

	f2 := filepath.Join(tempDir, "candles", "um", "1m", "symbol=ETHUSDT", "ETH-1m.parquet")
	os.MkdirAll(filepath.Dir(f2), 0755)
	os.WriteFile(f2, []byte("test data B"), 0644)

	builder1 := &manifest.Builder{
		DataRoot:     tempDir,
		DatasetID:    "test-dataset",
		DatasetRole:  "candles",
		SourceRepo:   "test-repo",
		SourceGitSHA: "abc",
		SourceType:   "local",
		Symbols:      []string{"ETHUSDT", "BTCUSDT"}, // deliberately out of order
		Intervals:    []string{"1m"},
	}

	m1, err := builder1.Build()
	if err != nil {
		t.Fatalf("build 1 failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond) // Ensure generated_at_utc changes if not properly ignored

	builder2 := &manifest.Builder{
		DataRoot:     tempDir,
		DatasetID:    "test-dataset",
		DatasetRole:  "candles",
		SourceRepo:   "test-repo",
		SourceGitSHA: "abc",
		SourceType:   "local",
		Symbols:      []string{"BTCUSDT", "ETHUSDT"},
		Intervals:    []string{"1m"},
	}

	m2, err := builder2.Build()
	if err != nil {
		t.Fatalf("build 2 failed: %v", err)
	}

	if m1.Hashes.DatasetHash != m2.Hashes.DatasetHash {
		t.Errorf("dataset_hash should be identical: %s vs %s", m1.Hashes.DatasetHash, m2.Hashes.DatasetHash)
	}
	if m1.Hashes.ManifestHash != m2.Hashes.ManifestHash {
		t.Errorf("manifest_hash should be identical despite different generated_at_utc and initial symbol order: %s vs %s", m1.Hashes.ManifestHash, m2.Hashes.ManifestHash)
	}

	// Change file content
	os.WriteFile(f1, []byte("test data A modified"), 0644)
	m3, _ := builder1.Build()
	if m1.Hashes.DatasetHash == m3.Hashes.DatasetHash {
		t.Errorf("dataset_hash should change when file content changes")
	}
	if m1.Hashes.ManifestHash == m3.Hashes.ManifestHash {
		t.Errorf("manifest_hash should change when file content changes")
	}

	// Revert content, change interval
	os.WriteFile(f1, []byte("test data A"), 0644)
	builder4 := &manifest.Builder{
		DataRoot:     tempDir,
		DatasetID:    "test-dataset",
		DatasetRole:  "candles",
		SourceRepo:   "test-repo",
		SourceGitSHA: "abc",
		SourceType:   "local",
		Symbols:      []string{"BTCUSDT", "ETHUSDT"},
		Intervals:    []string{"1m", "5m"},
	}
	m4, _ := builder4.Build()
	if m1.Hashes.ManifestHash == m4.Hashes.ManifestHash {
		t.Errorf("manifest_hash should change when intervals change")
	}
}
