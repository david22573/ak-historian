package pitarchive

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type pitFixture struct {
	archiveRoot  string
	manifestPath string
	manifest     SnapshotManifest
	options      EvaluateOptions
}

func int64Pointer(value int64) *int64 { return &value }

func contentDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func newPITFixture(t *testing.T) pitFixture {
	t.Helper()
	root := t.TempDir()
	windowStart := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	windowEnd := windowStart.Add(3 * time.Hour)
	manifest := SnapshotManifest{
		SchemaVersion: SnapshotManifestSchemaVersion, ManifestID: "manifest-test-v1", DatasetID: "candles:BTCUSDT:1h",
		DatasetVersion: "2025-01-v1", ResearchWindowStart: windowStart, ResearchWindowEnd: windowEnd,
		GeneratedAt: windowEnd.Add(time.Hour), Source: "test-exchange", ArchiveID: "archive-test-v1",
		CoveragePolicy: CoveragePolicy{
			SchemaVersion: CoveragePolicySchemaVersion, PolicyID: "hourly-continuous-v1", PartitionModel: "explicit-hourly-[start,end)",
			MaximumGapSeconds: int64Pointer(0), Required: []PartitionRequirement{}, Exceptions: []CoverageException{},
		},
		AvailabilityPolicy: AvailabilityPolicy{
			SchemaVersion: AvailabilityPolicySchemaVersion, PolicyID: "available-after-close-v1", RequiredPublicationDelaySeconds: int64Pointer(0),
		},
		Snapshots: []SnapshotReference{},
	}
	for i := 0; i < 3; i++ {
		partitionStart := windowStart.Add(time.Duration(i) * time.Hour)
		partitionEnd := partitionStart.Add(time.Hour)
		partitionKey := fmt.Sprintf("BTCUSDT/2025-01-01T%02d", i)
		relativePath := filepath.ToSlash(filepath.Join("BTCUSDT", fmt.Sprintf("part-%02d.bin", i)))
		data := []byte(fmt.Sprintf("snapshot-content-%02d", i))
		fullPath := filepath.Join(root, filepath.FromSlash(relativePath))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
		manifest.CoveragePolicy.Required = append(manifest.CoveragePolicy.Required, PartitionRequirement{
			PartitionKey: partitionKey, EventTimeStart: partitionStart, EventTimeEnd: partitionEnd,
		})
		manifest.Snapshots = append(manifest.Snapshots, SnapshotReference{
			SnapshotID: fmt.Sprintf("snapshot-%02d", i), PartitionKey: partitionKey, RelativePath: relativePath,
			EventTimeStart: partitionStart, EventTimeEnd: partitionEnd, AvailableAt: partitionEnd,
			IngestedAt: partitionEnd.Add(time.Minute), ContentHash: contentDigest(data), SchemaVersion: SnapshotSchemaVersion, ByteSize: int64(len(data)),
		})
	}
	manifest.SnapshotCount = len(manifest.Snapshots)
	rehashManifest(t, &manifest)
	manifestPath := filepath.Join(root, "snapshot-manifest.json")
	if err := WriteManifest(manifestPath, manifest); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	return pitFixture{
		archiveRoot: root, manifestPath: manifestPath, manifest: manifest,
		options: EvaluateOptions{
			ManifestPath: manifestPath, ArchiveRoot: root, DatasetID: manifest.DatasetID, DatasetVersion: manifest.DatasetVersion,
			ResearchWindowStart: windowStart, ResearchWindowEnd: windowEnd,
			EvaluationCutoff: windowEnd.Add(12 * time.Hour), Strict: true,
			Now: windowEnd.Add(24 * time.Hour), HistorianBuild: "ak-historian/test-build",
		},
	}
}

func rehashManifest(t *testing.T, manifest *SnapshotManifest) {
	t.Helper()
	hash, err := ComputeManifestHash(*manifest)
	if err != nil {
		t.Fatalf("ComputeManifestHash: %v", err)
	}
	manifest.ManifestHash = hash
}

func writeRawManifest(t *testing.T, path string, manifest SnapshotManifest) {
	t.Helper()
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func evaluateFixture(t *testing.T, fixture pitFixture) EvaluationResult {
	t.Helper()
	result, err := Evaluate(fixture.options)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	return result
}

func hasReason(result EvaluationResult, code ReasonCode) bool {
	for _, failure := range result.Failures {
		if failure.Code == code {
			return true
		}
	}
	return false
}

func requireReason(t *testing.T, result EvaluationResult, code ReasonCode) {
	t.Helper()
	if !hasReason(result, code) {
		t.Fatalf("missing reason %s in %+v", code, result.Failures)
	}
}

func rewriteFixtureManifest(t *testing.T, fixture *pitFixture) {
	t.Helper()
	rehashManifest(t, &fixture.manifest)
	writeRawManifest(t, fixture.manifestPath, fixture.manifest)
}
