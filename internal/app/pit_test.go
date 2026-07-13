package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/david22573/ak-historian/internal/pitarchive"
)

func TestVerifyPITCommandConsumesSnapshotManifest(t *testing.T) {
	archiveRoot := t.TempDir()
	start := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	data := []byte("real-snapshot-bytes")
	snapshotPath := filepath.Join(archiveRoot, "partition.bin")
	if err := os.WriteFile(snapshotPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	zero := int64(0)
	manifest := pitarchive.SnapshotManifest{
		SchemaVersion: pitarchive.SnapshotManifestSchemaVersion, ManifestID: "cli-manifest-v1",
		DatasetID: "cli-dataset", DatasetVersion: "v1", ResearchWindowStart: start, ResearchWindowEnd: end,
		GeneratedAt: end, Source: "test", ArchiveID: "cli-archive-v1",
		CoveragePolicy: pitarchive.CoveragePolicy{
			SchemaVersion: pitarchive.CoveragePolicySchemaVersion, PolicyID: "cli-coverage-v1", PartitionModel: "explicit-hourly-[start,end)", MaximumGapSeconds: &zero,
			Required: []pitarchive.PartitionRequirement{{PartitionKey: "p0", EventTimeStart: start, EventTimeEnd: end}}, Exceptions: []pitarchive.CoverageException{},
		},
		AvailabilityPolicy: pitarchive.AvailabilityPolicy{
			SchemaVersion: pitarchive.AvailabilityPolicySchemaVersion, PolicyID: "cli-availability-v1", RequiredPublicationDelaySeconds: &zero,
		},
		SnapshotCount: 1,
		Snapshots: []pitarchive.SnapshotReference{{
			SnapshotID: "s0", PartitionKey: "p0", RelativePath: "partition.bin", EventTimeStart: start, EventTimeEnd: end,
			AvailableAt: end, IngestedAt: end.Add(time.Minute), ContentHash: "sha256:" + hex.EncodeToString(sum[:]), SchemaVersion: pitarchive.SnapshotSchemaVersion, ByteSize: int64(len(data)),
		}},
	}
	manifestHash, err := pitarchive.ComputeManifestHash(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifest.ManifestHash = manifestHash
	manifestPath := filepath.Join(archiveRoot, "snapshot-manifest.json")
	if err := pitarchive.WriteManifest(manifestPath, manifest); err != nil {
		t.Fatal(err)
	}

	args := []string{
		"--snapshot-manifest", manifestPath, "--archive-root", archiveRoot,
		"--dataset-id", manifest.DatasetID, "--dataset-version", manifest.DatasetVersion,
		"--window-start", start.Format(time.RFC3339), "--window-end", end.Format(time.RFC3339),
		"--evaluation-cutoff", end.Format(time.RFC3339), "--historian-build", "ak-historian/cli-test",
	}
	command := newPITCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs(args)
	if err := command.Execute(); err != nil {
		t.Fatalf("valid manifest command failed: %v\n%s", err, output.String())
	}
	if !strings.Contains(output.String(), `"verdict": "PIT_ELIGIBLE"`) {
		t.Fatalf("valid manifest did not produce eligibility: %s", output.String())
	}

	command = newPITCommand()
	output.Reset()
	command.SetOut(&output)
	command.SetErr(&output)
	args[1] = filepath.Join(archiveRoot, "missing-manifest.json")
	command.SetArgs(args)
	if err := command.Execute(); err == nil {
		t.Fatalf("missing manifest unexpectedly passed: %s", output.String())
	}
	if !strings.Contains(output.String(), string(pitarchive.ReasonManifestMissing)) || strings.Contains(output.String(), `"verdict": "PIT_ELIGIBLE"`) {
		t.Fatalf("snapshot-manifest argument was not observably enforced: %s", output.String())
	}
}
