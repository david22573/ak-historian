package pitarchive

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestSnapshotIntegrity(t *testing.T) {
	t.Run("all valid hashes pass", func(t *testing.T) {
		fixture := newPITFixture(t)
		result := evaluateFixture(t, fixture)
		if result.Evidence.SnapshotIntegrity.StrictVerdict != CheckPass || result.Evidence.SnapshotIntegrity.VerifiedCount != 3 {
			t.Fatalf("unexpected integrity report: %+v", result.Evidence.SnapshotIntegrity)
		}
	})

	t.Run("missing snapshot fails", func(t *testing.T) {
		fixture := newPITFixture(t)
		if err := os.Remove(filepath.Join(fixture.archiveRoot, filepath.FromSlash(fixture.manifest.Snapshots[1].RelativePath))); err != nil {
			t.Fatal(err)
		}
		result := evaluateFixture(t, fixture)
		requireReason(t, result, ReasonSnapshotMissing)
		if result.StrictPromotionAllowed {
			t.Fatal("missing snapshot allowed strict promotion")
		}
	})

	t.Run("modified snapshot fails", func(t *testing.T) {
		fixture := newPITFixture(t)
		path := filepath.Join(fixture.archiveRoot, filepath.FromSlash(fixture.manifest.Snapshots[1].RelativePath))
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		data[0] ^= 0xff
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		result := evaluateFixture(t, fixture)
		requireReason(t, result, ReasonSnapshotHashMismatch)
	})

	t.Run("truncated snapshot fails", func(t *testing.T) {
		fixture := newPITFixture(t)
		path := filepath.Join(fixture.archiveRoot, filepath.FromSlash(fixture.manifest.Snapshots[1].RelativePath))
		if err := os.Truncate(path, 3); err != nil {
			t.Fatal(err)
		}
		result := evaluateFixture(t, fixture)
		requireReason(t, result, ReasonSnapshotSizeMismatch)
	})

	t.Run("conflicting partition references fail", func(t *testing.T) {
		fixture := newPITFixture(t)
		fixture.manifest.Snapshots[1].PartitionKey = fixture.manifest.Snapshots[0].PartitionKey
		fixture.manifest.Snapshots[1].EventTimeStart = fixture.manifest.Snapshots[0].EventTimeStart
		fixture.manifest.Snapshots[1].EventTimeEnd = fixture.manifest.Snapshots[0].EventTimeEnd
		rewriteFixtureManifest(t, &fixture)
		result := evaluateFixture(t, fixture)
		requireReason(t, result, ReasonSnapshotPartitionConflict)
		if result.Evidence == nil || len(result.Evidence.Coverage.DuplicatePartitions) != 1 {
			t.Fatalf("duplicate partition missing from structured coverage: %+v", result.Evidence)
		}
	})

	t.Run("path traversal fails", func(t *testing.T) {
		fixture := newPITFixture(t)
		fixture.manifest.Snapshots[0].RelativePath = "../outside.bin"
		rewriteFixtureManifest(t, &fixture)
		result := evaluateFixture(t, fixture)
		requireReason(t, result, ReasonSnapshotPathInvalid)
	})

	t.Run("symlink escape fails", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlink creation requires platform privileges")
		}
		fixture := newPITFixture(t)
		outsideDir := t.TempDir()
		outside := filepath.Join(outsideDir, "outside.bin")
		data := []byte("snapshot-content-00")
		if err := os.WriteFile(outside, data, 0o644); err != nil {
			t.Fatal(err)
		}
		inside := filepath.Join(fixture.archiveRoot, filepath.FromSlash(fixture.manifest.Snapshots[0].RelativePath))
		if err := os.Remove(inside); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, inside); err != nil {
			t.Fatal(err)
		}
		result := evaluateFixture(t, fixture)
		requireReason(t, result, ReasonSnapshotPathInvalid)
	})

	t.Run("oversized snapshot fails safely", func(t *testing.T) {
		fixture := newPITFixture(t)
		fixture.options.MaxSnapshotBytes = fixture.manifest.Snapshots[0].ByteSize - 1
		result := evaluateFixture(t, fixture)
		requireReason(t, result, ReasonSnapshotTooLarge)
	})
}

func TestCoveragePolicy(t *testing.T) {
	t.Run("complete expected coverage passes", func(t *testing.T) {
		fixture := newPITFixture(t)
		result := evaluateFixture(t, fixture)
		coverage := result.Evidence.Coverage
		if coverage.StrictVerdict != CheckPass || coverage.CoverageRatio != "1.000000" || len(coverage.ExpectedPartitions) != 3 || len(coverage.ObservedPartitions) != 3 {
			t.Fatalf("unexpected coverage: %+v", coverage)
		}
	})

	for _, index := range []int{0, 1, 2} {
		name := []string{"missing first partition", "missing middle partition", "missing final partition"}[index]
		t.Run(name+" fails", func(t *testing.T) {
			fixture := newPITFixture(t)
			fixture.manifest.Snapshots = append(fixture.manifest.Snapshots[:index], fixture.manifest.Snapshots[index+1:]...)
			fixture.manifest.SnapshotCount = len(fixture.manifest.Snapshots)
			rewriteFixtureManifest(t, &fixture)
			result := evaluateFixture(t, fixture)
			requireReason(t, result, ReasonCoverageIncomplete)
			if result.Evidence == nil || len(result.Evidence.Coverage.MissingPartitions) != 1 || result.Evidence.Coverage.MaximumObservedGapSeconds != 3600 || result.Verdict == VerdictEligible {
				t.Fatalf("gap was not represented: %+v", result)
			}
		})
	}

	t.Run("out of window snapshot is reported and not counted", func(t *testing.T) {
		fixture := newPITFixture(t)
		fixture.manifest.Snapshots[1].EventTimeStart = fixture.manifest.ResearchWindowEnd
		fixture.manifest.Snapshots[1].EventTimeEnd = fixture.manifest.ResearchWindowEnd.Add(time.Hour)
		fixture.manifest.Snapshots[1].AvailableAt = fixture.manifest.ResearchWindowEnd.Add(time.Hour)
		rewriteFixtureManifest(t, &fixture)
		result := evaluateFixture(t, fixture)
		if result.Evidence == nil || len(result.Evidence.Coverage.OutOfWindowPartitions) != 1 || len(result.Evidence.Coverage.ObservedPartitions) != 2 {
			t.Fatalf("out-of-window partition handling: %+v", result.Evidence)
		}
	})

	t.Run("endpoint span does not hide internal gap", func(t *testing.T) {
		fixture := newPITFixture(t)
		fixture.manifest.Snapshots = []SnapshotReference{fixture.manifest.Snapshots[0], fixture.manifest.Snapshots[2]}
		fixture.manifest.SnapshotCount = 2
		rewriteFixtureManifest(t, &fixture)
		result := evaluateFixture(t, fixture)
		if result.Evidence == nil || result.Evidence.Coverage.MissingPartitions[0] != fixture.manifest.CoveragePolicy.Required[1].PartitionKey {
			t.Fatalf("internal gap hidden: %+v", result)
		}
	})

	t.Run("declared closure exception passes", func(t *testing.T) {
		fixture := newPITFixture(t)
		middle := fixture.manifest.CoveragePolicy.Required[1]
		fixture.manifest.CoveragePolicy.Required = []PartitionRequirement{fixture.manifest.CoveragePolicy.Required[0], fixture.manifest.CoveragePolicy.Required[2]}
		fixture.manifest.CoveragePolicy.Exceptions = []CoverageException{{
			ExceptionID: "declared-maintenance-01", EventTimeStart: middle.EventTimeStart, EventTimeEnd: middle.EventTimeEnd, Reason: "source maintenance window",
		}}
		fixture.manifest.Snapshots = []SnapshotReference{fixture.manifest.Snapshots[0], fixture.manifest.Snapshots[2]}
		fixture.manifest.SnapshotCount = 2
		rewriteFixtureManifest(t, &fixture)
		result := evaluateFixture(t, fixture)
		if result.Verdict != VerdictEligible || !result.StrictPromotionAllowed {
			t.Fatalf("declared exception did not pass: %+v", result)
		}
	})

	t.Run("undeclared closure fails", func(t *testing.T) {
		fixture := newPITFixture(t)
		fixture.manifest.CoveragePolicy.Required = []PartitionRequirement{fixture.manifest.CoveragePolicy.Required[0], fixture.manifest.CoveragePolicy.Required[2]}
		fixture.manifest.Snapshots = []SnapshotReference{fixture.manifest.Snapshots[0], fixture.manifest.Snapshots[2]}
		fixture.manifest.SnapshotCount = 2
		rewriteFixtureManifest(t, &fixture)
		result := evaluateFixture(t, fixture)
		requireReason(t, result, ReasonCoveragePolicyInvalid)
	})
}

func TestAvailabilityEnforcement(t *testing.T) {
	t.Run("before cutoff passes", func(t *testing.T) {
		fixture := newPITFixture(t)
		result := evaluateFixture(t, fixture)
		if result.Evidence.Availability.StrictVerdict != CheckPass {
			t.Fatalf("availability failed: %+v", result.Evidence.Availability)
		}
	})

	t.Run("exactly at cutoff passes", func(t *testing.T) {
		fixture := newPITFixture(t)
		fixture.options.EvaluationCutoff = fixture.manifest.Snapshots[2].AvailableAt
		result := evaluateFixture(t, fixture)
		if result.Verdict != VerdictEligible {
			t.Fatalf("inclusive cutoff boundary failed: %+v", result)
		}
	})

	t.Run("after cutoff fails", func(t *testing.T) {
		fixture := newPITFixture(t)
		fixture.options.EvaluationCutoff = fixture.manifest.Snapshots[1].AvailableAt.Add(-time.Second)
		result := evaluateFixture(t, fixture)
		requireReason(t, result, ReasonAvailableAfterEvaluation)
	})

	t.Run("missing availability cannot be replaced by ingestion time", func(t *testing.T) {
		fixture := newPITFixture(t)
		fixture.manifest.Snapshots[0].AvailableAt = time.Time{}
		fixture.manifest.Snapshots[0].IngestedAt = fixture.options.EvaluationCutoff.Add(-time.Hour)
		rewriteFixtureManifest(t, &fixture)
		result := evaluateFixture(t, fixture)
		requireReason(t, result, ReasonAvailabilityTimestampMissing)
	})

	t.Run("future timestamp fails", func(t *testing.T) {
		fixture := newPITFixture(t)
		fixture.options.Now = fixture.manifest.Snapshots[2].AvailableAt.Add(-time.Second)
		fixture.options.EvaluationCutoff = fixture.options.Now
		result := evaluateFixture(t, fixture)
		requireReason(t, result, ReasonFutureSnapshotTimestamp)
	})

	t.Run("publication delay violation fails", func(t *testing.T) {
		fixture := newPITFixture(t)
		fixture.manifest.AvailabilityPolicy.RequiredPublicationDelaySeconds = int64Pointer(3600)
		rewriteFixtureManifest(t, &fixture)
		result := evaluateFixture(t, fixture)
		requireReason(t, result, ReasonPublicationDelayViolation)
	})
}

func TestManifestBypassRegression(t *testing.T) {
	fixture := newPITFixture(t)
	fixture.options.ManifestPath = ""
	result := evaluateFixture(t, fixture)
	if result.Verdict == VerdictEligible || result.StrictPromotionAllowed || result.Evidence != nil {
		t.Fatalf("perfect-window manifest bypass regressed: %+v", result)
	}
	requireReason(t, result, ReasonManifestMissing)

	fixture = newPITFixture(t)
	fixture.options.Strict = false
	result = evaluateFixture(t, fixture)
	if result.Verdict != VerdictDiagnosticOnly || result.StrictPromotionAllowed {
		t.Fatalf("diagnostic mode reused strict eligibility: %+v", result)
	}
}
