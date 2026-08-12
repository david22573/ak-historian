package exchange_meta

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/david22573/ak-historian/internal/atomicfile"
)

func WriteSnapshot(path string, snapshot *Snapshot) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.WriteFile(path, data, 0644)
}

func ReadSnapshot(path string) (*Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("parse exchange metadata snapshot: %w", err)
	}
	if snapshot.SchemaVersion != SchemaVersion || snapshot.SnapshotVersion != SnapshotVersion || snapshot.SnapshotID == "" || !snapshot.Validation.IsValid || snapshot.Validation.Status == "FAIL" {
		return nil, fmt.Errorf("exchange metadata snapshot contract is invalid")
	}
	claimedHashes := snapshot.Hashes
	expectedHashes := ComputeSnapshotHashes(&snapshot)
	if !reflect.DeepEqual(claimedHashes, expectedHashes) {
		return nil, fmt.Errorf("exchange metadata snapshot hash mismatch")
	}
	if snapshot.SnapshotID != buildSnapshotID(&snapshot) {
		return nil, fmt.Errorf("exchange metadata snapshot id mismatch")
	}
	return &snapshot, nil
}

func WriteSnapshotManifest(path string, manifest *SnapshotManifest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.WriteFile(path, data, 0644)
}

func ReadSnapshotManifest(path string) (*SnapshotManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var manifest SnapshotManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse exchange metadata snapshot manifest: %w", err)
	}
	if manifest.SchemaVersion != SchemaVersion || manifest.ManifestVersion != ManifestVersion || manifest.ArchiveID == "" || !manifest.Validation.IsValid || manifest.Validation.Status == "FAIL" || manifest.SnapshotCount != len(manifest.Snapshots) {
		return nil, fmt.Errorf("exchange metadata snapshot manifest contract is invalid")
	}
	for _, ref := range manifest.Snapshots {
		if !validRelativeArtifactPath(ref.RelativePath) {
			return nil, fmt.Errorf("exchange metadata snapshot path is not contained: %q", ref.RelativePath)
		}
	}
	claimedHashes := manifest.Hashes
	expectedHashes := ComputeManifestHashes(&manifest)
	if !reflect.DeepEqual(claimedHashes, expectedHashes) {
		return nil, fmt.Errorf("exchange metadata snapshot manifest hash mismatch: claimed=%+v expected=%+v", claimedHashes, expectedHashes)
	}
	return &manifest, nil
}

func validRelativeArtifactPath(path string) bool {
	if path == "" || filepath.IsAbs(path) || strings.Contains(path, "\\") {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	return clean == path && clean != "." && clean != ".."
}
