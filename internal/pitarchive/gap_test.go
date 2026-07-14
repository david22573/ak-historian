package pitarchive

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestGapManifestCanonicalIdentityAndMutation(t *testing.T) {
	root := t.TempDir()
	writeGapFixture(t, root, "AAAUSDT", "2025-01", "alpha")
	writeGapFixture(t, root, "BTCUSDT", "2025-01", "btc")
	writeGapFixture(t, root, "ETHUSDT", "2025-01", "eth")
	options := validGapOptions(root)
	first, err := BuildGapManifest(options)
	if err != nil {
		t.Fatal(err)
	}
	options.RequiredContextSymbols = []string{"ETHUSDT", "BTCUSDT"}
	second, err := BuildGapManifest(options)
	if err != nil {
		t.Fatal(err)
	}
	if first.ManifestHash != second.ManifestHash || first.DatasetVersion != second.DatasetVersion {
		t.Fatal("reordered inputs changed canonical identity")
	}
	if err := VerifyGapManifest(first); err != nil {
		t.Fatalf("generated identity chain did not verify: %v", err)
	}
	reordered := first
	slices.Reverse(reordered.RequiredSymbols)
	slices.Reverse(reordered.RequiredContextSymbols)
	slices.Reverse(reordered.ExpectedPartitions)
	slices.Reverse(reordered.Snapshots)
	slices.Reverse(reordered.MissingEvidence)
	for index := range reordered.Snapshots {
		slices.Reverse(reordered.Snapshots[index].EvidenceGaps)
	}
	for index := range reordered.MissingEvidence {
		slices.Reverse(reordered.MissingEvidence[index].Reasons)
	}
	reorderedSnapshotSet, err := ComputeGapSnapshotSetHash(reordered.Snapshots)
	if err != nil {
		t.Fatal(err)
	}
	reorderedManifestHash, err := ComputeGapManifestHash(reordered)
	if err != nil {
		t.Fatal(err)
	}
	if reorderedSnapshotSet != first.SnapshotSetHash || reorderedManifestHash != first.ManifestHash {
		t.Fatal("reordered manifest entries changed canonical identity")
	}
	if err := VerifyGapManifest(reordered); err != nil {
		t.Fatalf("reordered canonical manifest did not verify: %v", err)
	}
	writeGapFixture(t, root, "AAAUSDT", "2025-01", "changed")
	changed, err := BuildGapManifest(options)
	if err != nil {
		t.Fatal(err)
	}
	if changed.ManifestHash == first.ManifestHash || changed.DatasetVersion == first.DatasetVersion {
		t.Fatal("snapshot mutation did not change immutable identity")
	}
	tampered := first
	tampered.Snapshots = append([]GapSnapshot{}, first.Snapshots...)
	tampered.Snapshots[0].ContentHash = "sha256:" + strings.Repeat("f", 64)
	if err := VerifyGapManifest(tampered); err == nil {
		t.Fatal("changed snapshot did not invalidate recorded identity hashes")
	}
}

func TestGapManifestFailsClosedForMissingContextAvailabilityAndCutoff(t *testing.T) {
	root := t.TempDir()
	writeGapFixture(t, root, "AAAUSDT", "2025-01", "alpha")
	writeGapFixture(t, root, "BTCUSDT", "2025-01", "btc")
	options := validGapOptions(root)
	late := options.EvaluationCutoff.Add(time.Hour)
	options.SourceAvailability = map[string]time.Time{
		"futures-um/1m/AAAUSDT/2025-01": late,
		"futures-um/1m/BTCUSDT/2025-01": options.WindowEnd.Add(-time.Hour),
	}
	manifest, err := BuildGapManifest(options)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		string(ReasonSnapshotMissing):              false,
		string(ReasonAvailabilityTimestampMissing): false,
		string(ReasonAvailableAfterEvaluation):     false,
		string(ReasonPublicationDelayViolation):    false,
	}
	for _, missing := range manifest.MissingEvidence {
		for _, reason := range missing.Reasons {
			if _, ok := want[string(reason)]; ok {
				want[string(reason)] = true
			}
		}
	}
	for reason, found := range want {
		if !found {
			t.Fatalf("missing fail-closed reason %s", reason)
		}
	}
	if manifest.ProvablePITCoverage.Available || manifest.Status != VerdictEvidenceIncomplete {
		t.Fatal("incomplete context/availability incorrectly produced PIT coverage")
	}
}

func TestGapManifestFailsClosedForMissingBTCAndETHContextIndependently(t *testing.T) {
	for _, missingSymbol := range []string{"BTCUSDT", "ETHUSDT"} {
		t.Run(missingSymbol, func(t *testing.T) {
			root := t.TempDir()
			writeGapFixture(t, root, "AAAUSDT", "2025-01", "alpha")
			for _, contextSymbol := range []string{"BTCUSDT", "ETHUSDT"} {
				if contextSymbol != missingSymbol {
					writeGapFixture(t, root, contextSymbol, "2025-01", strings.ToLower(contextSymbol))
				}
			}
			manifest, err := BuildGapManifest(validGapOptions(root))
			if err != nil {
				t.Fatal(err)
			}
			partition := "futures-um/1m/" + missingSymbol + "/2025-01"
			if !missingPartitionHasReason(manifest, partition, ReasonSnapshotMissing) {
				t.Fatalf("missing %s context did not fail closed", missingSymbol)
			}
			if manifest.ProvablePITCoverage.Available {
				t.Fatalf("missing %s context produced PIT coverage", missingSymbol)
			}
		})
	}
}

func TestGapManifestMissingExpectedPartitionFailsVerification(t *testing.T) {
	root := t.TempDir()
	writeGapFixture(t, root, "AAAUSDT", "2025-01", "alpha")
	writeGapFixture(t, root, "BTCUSDT", "2025-01", "btc")
	writeGapFixture(t, root, "ETHUSDT", "2025-01", "eth")
	manifest, err := BuildGapManifest(validGapOptions(root))
	if err != nil {
		t.Fatal(err)
	}
	manifest.Snapshots = manifest.Snapshots[1:]
	if err := VerifyGapManifest(manifest); err == nil {
		t.Fatal("missing expected snapshot passed identity verification")
	}
}

func TestGapManifestRejectsMutableAndPathIdentities(t *testing.T) {
	root := t.TempDir()
	for _, value := range []string{"latest", "/tmp/archive", "local/path"} {
		options := validGapOptions(root)
		options.DatasetID = value
		if _, err := BuildGapManifest(options); err == nil {
			t.Fatalf("accepted unsafe dataset identity %q", value)
		}
	}
	for _, value := range []string{"current", "local/manifest"} {
		options := validGapOptions(root)
		options.ManifestID = value
		if _, err := BuildGapManifest(options); err == nil {
			t.Fatalf("accepted unsafe manifest identity %q", value)
		}
	}
}

func TestGapManifestIgnoresUndeclaredFilesAndRootPath(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	for _, root := range []string{left, right} {
		writeGapFixture(t, root, "AAAUSDT", "2025-01", "alpha")
		writeGapFixture(t, root, "BTCUSDT", "2025-01", "btc")
		writeGapFixture(t, root, "ETHUSDT", "2025-01", "eth")
	}
	if err := os.WriteFile(filepath.Join(left, "undeclared.parquet"), []byte("ignored"), 0o600); err != nil {
		t.Fatal(err)
	}
	leftManifest, err := BuildGapManifest(validGapOptions(left))
	if err != nil {
		t.Fatal(err)
	}
	rightManifest, err := BuildGapManifest(validGapOptions(right))
	if err != nil {
		t.Fatal(err)
	}
	if leftManifest.ManifestHash != rightManifest.ManifestHash {
		t.Fatal("local root or undeclared file affected identity")
	}
}

func TestCommittedGapEvidenceIdentityChainVerifies(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "evidence", "pr4b0_r1p_historian_identity_gap.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest GapManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if err := VerifyGapManifest(manifest); err != nil {
		t.Fatalf("committed gap evidence failed verification: %v", err)
	}
}

func validGapOptions(root string) GapBuildOptions {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	cutoff := end.Add(24 * time.Hour)
	return GapBuildOptions{
		ArchiveRoot: root, DatasetID: "dataset-test-v1", ManifestID: "manifest-test-v1",
		CandidateID: "candidate", CandidateVersion: "v1", ImplementationHash: "sha256:" + strings.Repeat("a", 64),
		Market: "futures-um", Interval: "1m", WindowStart: start, WindowEnd: end,
		EvaluationCutoff: cutoff, GeneratedAt: cutoff.Add(time.Hour), HistorianBuild: "test-build",
		EventSchemaVersion: "legacy-unversioned", RequiredSymbols: []string{"AAAUSDT"}, RequiredContextSymbols: []string{"BTCUSDT", "ETHUSDT"},
		SourceAvailability: map[string]time.Time{},
	}
}

func writeGapFixture(t *testing.T, root, symbol, month, contents string) {
	t.Helper()
	parsed, err := time.Parse("2006-01", month)
	if err != nil {
		t.Fatal(err)
	}
	relative := filepath.Join("candles", "futures-um", "1m", "symbol="+symbol, "year="+parsed.Format("2006"), "month="+parsed.Format("01"), symbol+"-1m-"+month+".parquet")
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func missingPartitionHasReason(manifest GapManifest, partition string, reason ReasonCode) bool {
	for _, evidence := range manifest.MissingEvidence {
		if evidence.PartitionKey == partition && containsReason(evidence.Reasons, reason) {
			return true
		}
	}
	return false
}
