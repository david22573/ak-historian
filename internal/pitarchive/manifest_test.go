package pitarchive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifestValidation(t *testing.T) {
	t.Run("valid manifest passes", func(t *testing.T) {
		fixture := newPITFixture(t)
		result := evaluateFixture(t, fixture)
		if result.Verdict != VerdictEligible || !result.StrictPromotionAllowed || result.Evidence == nil {
			t.Fatalf("unexpected result: %+v", result)
		}
	})

	t.Run("missing manifest fails strict PIT", func(t *testing.T) {
		fixture := newPITFixture(t)
		fixture.options.ManifestPath = filepath.Join(fixture.archiveRoot, "missing.json")
		result := evaluateFixture(t, fixture)
		if result.Verdict == VerdictEligible || result.StrictPromotionAllowed {
			t.Fatalf("missing manifest was eligible: %+v", result)
		}
		requireReason(t, result, ReasonManifestMissing)
	})

	t.Run("empty manifest fails", func(t *testing.T) {
		fixture := newPITFixture(t)
		if err := os.WriteFile(fixture.manifestPath, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		result := evaluateFixture(t, fixture)
		requireReason(t, result, ReasonManifestEmpty)
	})

	t.Run("malformed manifest fails without panic", func(t *testing.T) {
		fixture := newPITFixture(t)
		if err := os.WriteFile(fixture.manifestPath, []byte(`{"schema_version":`), 0o644); err != nil {
			t.Fatal(err)
		}
		result := evaluateFixture(t, fixture)
		requireReason(t, result, ReasonManifestMalformed)
	})

	t.Run("unknown schema fails", func(t *testing.T) {
		fixture := newPITFixture(t)
		fixture.manifest.SchemaVersion = "unknown.v9"
		rewriteFixtureManifest(t, &fixture)
		result := evaluateFixture(t, fixture)
		requireReason(t, result, ReasonManifestSchemaUnsupported)
	})

	t.Run("missing dataset identity fails", func(t *testing.T) {
		fixture := newPITFixture(t)
		fixture.manifest.DatasetID = ""
		rewriteFixtureManifest(t, &fixture)
		result := evaluateFixture(t, fixture)
		requireReason(t, result, ReasonManifestFieldMissing)
	})

	t.Run("research window mismatch fails", func(t *testing.T) {
		fixture := newPITFixture(t)
		fixture.options.ResearchWindowEnd = fixture.options.ResearchWindowEnd.Add(-1)
		result := evaluateFixture(t, fixture)
		requireReason(t, result, ReasonManifestWindowMismatch)
	})

	t.Run("manifest hash mismatch fails", func(t *testing.T) {
		fixture := newPITFixture(t)
		fixture.manifest.Source = "substituted-source"
		writeRawManifest(t, fixture.manifestPath, fixture.manifest)
		result := evaluateFixture(t, fixture)
		requireReason(t, result, ReasonManifestHashMismatch)
	})

	t.Run("zero snapshots fails", func(t *testing.T) {
		fixture := newPITFixture(t)
		fixture.manifest.Snapshots = nil
		fixture.manifest.SnapshotCount = 0
		rewriteFixtureManifest(t, &fixture)
		result := evaluateFixture(t, fixture)
		requireReason(t, result, ReasonManifestSnapshotCountMismatch)
	})

	t.Run("duplicate snapshot IDs fail", func(t *testing.T) {
		fixture := newPITFixture(t)
		fixture.manifest.Snapshots[1].SnapshotID = fixture.manifest.Snapshots[0].SnapshotID
		rewriteFixtureManifest(t, &fixture)
		result := evaluateFixture(t, fixture)
		requireReason(t, result, ReasonSnapshotDuplicateID)
	})

	t.Run("unknown JSON field fails closed", func(t *testing.T) {
		fixture := newPITFixture(t)
		data, err := os.ReadFile(fixture.manifestPath)
		if err != nil {
			t.Fatal(err)
		}
		data = []byte(strings.TrimSuffix(strings.TrimSpace(string(data)), "}") + `,"unexpected":true}`)
		if err := os.WriteFile(fixture.manifestPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
		result := evaluateFixture(t, fixture)
		requireReason(t, result, ReasonManifestMalformed)
	})

	t.Run("malformed timestamp fails closed", func(t *testing.T) {
		fixture := newPITFixture(t)
		data, err := os.ReadFile(fixture.manifestPath)
		if err != nil {
			t.Fatal(err)
		}
		data = []byte(strings.Replace(string(data), fixture.manifest.ResearchWindowStart.Format("2006-01-02T15:04:05Z07:00"), "not-a-time", 1))
		if err := os.WriteFile(fixture.manifestPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
		result := evaluateFixture(t, fixture)
		requireReason(t, result, ReasonManifestMalformed)
	})

	t.Run("oversized manifest is bounded", func(t *testing.T) {
		fixture := newPITFixture(t)
		fixture.options.MaxManifestBytes = 8
		result := evaluateFixture(t, fixture)
		requireReason(t, result, ReasonManifestUnreadable)
	})

	t.Run("unsupported snapshot schema fails", func(t *testing.T) {
		fixture := newPITFixture(t)
		fixture.manifest.Snapshots[0].SchemaVersion = "unknown.snapshot.v1"
		rewriteFixtureManifest(t, &fixture)
		result := evaluateFixture(t, fixture)
		requireReason(t, result, ReasonSnapshotSchemaUnsupported)
	})

	t.Run("hash algorithm confusion fails", func(t *testing.T) {
		fixture := newPITFixture(t)
		fixture.manifest.Snapshots[0].ContentHash = "sha512:" + strings.Repeat("a", 128)
		rewriteFixtureManifest(t, &fixture)
		result := evaluateFixture(t, fixture)
		requireReason(t, result, ReasonManifestFieldMissing)
	})

	t.Run("temporary manifest is rejected", func(t *testing.T) {
		fixture := newPITFixture(t)
		temporary := filepath.Join(fixture.archiveRoot, ".snapshot-manifest.json.tmp-123")
		data, err := os.ReadFile(fixture.manifestPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(temporary, data, 0o644); err != nil {
			t.Fatal(err)
		}
		fixture.options.ManifestPath = temporary
		result := evaluateFixture(t, fixture)
		requireReason(t, result, ReasonManifestUnreadable)
	})
}

func TestManifestHashIsOrderIndependent(t *testing.T) {
	fixture := newPITFixture(t)
	first := fixture.manifest.Snapshots[0]
	fixture.manifest.Snapshots[0] = fixture.manifest.Snapshots[2]
	fixture.manifest.Snapshots[2] = first
	got, err := ComputeManifestHash(fixture.manifest)
	if err != nil {
		t.Fatal(err)
	}
	if got != fixture.manifest.ManifestHash {
		t.Fatalf("canonical manifest hash changed after snapshot reordering: %s != %s", got, fixture.manifest.ManifestHash)
	}
}
