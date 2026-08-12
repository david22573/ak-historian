package researchidentity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/david22573/ak-historian/internal/canonicalcontract"
	"github.com/david22573/ak-historian/internal/parquetutil"
	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/writer"
)

func TestBuilderDerivesStableStrictIdentity(t *testing.T) {
	opts, _ := identityFixture(t, nil)
	first, err := (Builder{}).Build(opts)
	if err != nil {
		t.Fatal(err)
	}
	second, err := (Builder{}).Build(opts)
	if err != nil {
		t.Fatal(err)
	}
	if first.Dataset.ArtifactHash != second.Dataset.ArtifactHash || first.PITEvidence.ArtifactHash != second.PITEvidence.ArtifactHash || first.ArtifactHash != second.ArtifactHash {
		t.Fatalf("identity is not stable: %#v %#v", first, second)
	}
	if first.CoverageEvidence.Status != EvidenceStatusPass || !first.CoverageEvidence.FullWindow || first.PITEvidence.Status != EvidenceStatusPass {
		t.Fatalf("strict evidence did not pass: %#v", first)
	}
	if first.SourceArchive.RawObjectHash == "" || first.AvailabilityPolicySource.ObjectHash == "" || first.CoveragePolicySource.ObjectHash == "" {
		t.Fatalf("raw identities missing: %#v", first)
	}
	if len(first.Dataset.Objects) != 1 || first.Dataset.Objects[0].WindowRowCount != 5 || first.CoverageEvidence.RowCount != 5 {
		t.Fatalf("object/count identity mismatch: %#v", first)
	}
}

func TestChangedArchiveChangesPITEvidenceIdentity(t *testing.T) {
	opts, _ := identityFixture(t, nil)
	first, err := (Builder{}).Build(opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(opts.SourceArchivePath, []byte("changed archive bytes"), 0644); err != nil {
		t.Fatal(err)
	}
	second, err := (Builder{}).Build(opts)
	if err != nil {
		t.Fatal(err)
	}
	if first.SourceArchive.RawObjectHash == second.SourceArchive.RawObjectHash || first.PITEvidence.ArtifactHash == second.PITEvidence.ArtifactHash {
		t.Fatalf("archive byte change did not change identity")
	}
	if first.Dataset.ArtifactHash != second.Dataset.ArtifactHash {
		t.Fatalf("archive-only change unexpectedly changed dataset inventory")
	}
}

func TestStrictCoverageDefectsFailClosed(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	tests := []struct {
		name  string
		times []int64
		want  string
	}{
		{name: "empty", times: []int64{}, want: "empty parquet object"},
		{name: "partial", times: []int64{start + 60000, start + 120000, start + 180000, start + 240000}, want: "partial-window"},
		{name: "gap", times: []int64{start, start + 60000, start + 180000, start + 240000}, want: "coverage defects"},
		{name: "duplicate", times: []int64{start, start + 60000, start + 60000, start + 120000, start + 180000, start + 240000}, want: "coverage defects"},
		{name: "out of order", times: []int64{start, start + 120000, start + 60000, start + 180000, start + 240000}, want: "coverage defects"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts, _ := identityFixture(t, tc.times)
			_, err := (Builder{}).Build(opts)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q failure, got %v", tc.want, err)
			}
		})
	}
}

func TestLateAvailabilityAndInvalidWindowsFailClosed(t *testing.T) {
	opts, _ := identityFixture(t, nil)
	opts.PointInTimeCutoffUTC = opts.DatasetEndUTC
	if _, err := (Builder{}).Build(opts); err == nil || !strings.Contains(err.Error(), "availability exceeds") {
		t.Fatalf("late availability did not fail: %v", err)
	}

	opts, _ = identityFixture(t, nil)
	opts.DatasetStartUTC = opts.DatasetEndUTC
	if _, err := (Builder{}).Build(opts); err == nil || !strings.Contains(err.Error(), "strictly before") {
		t.Fatalf("invalid window did not fail: %v", err)
	}
}

func TestRowsOutsideDeclaredDatasetWindowFailClosed(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	opts, _ := identityFixture(t, []int64{start - 60000, start, start + 60000, start + 120000, start + 180000, start + 240000})
	if _, err := (Builder{}).Build(opts); err == nil || !strings.Contains(err.Error(), "outside requested window") {
		t.Fatalf("out-of-window object rows did not fail: %v", err)
	}
}

func TestParquetReplacementDuringIdentityConstructionFailsClosed(t *testing.T) {
	_, path := identityFixture(t, nil)
	replacement := filepath.Join(t.TempDir(), "replacement.parquet")
	writeParquetTimes(t, replacement, []int64{1, 2, 3})
	_, _, _, err := readConsistentParquetObject(path, func(snapshotPath string) ([]int64, error) {
		times, err := parquetutil.ReadOpenTimesStrict([]string{snapshotPath})
		if err != nil {
			return nil, err
		}
		bytes, err := os.ReadFile(replacement)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, bytes, 0644); err != nil {
			return nil, err
		}
		return times, nil
	})
	if err == nil || !strings.Contains(err.Error(), "changed during identity construction") {
		t.Fatalf("concurrent replacement was not rejected: %v", err)
	}
}

func TestMissingArchiveAndUnsafeDatasetPathFail(t *testing.T) {
	opts, dataFile := identityFixture(t, nil)
	opts.SourceArchivePath = filepath.Join(opts.EvidenceRoot, "missing.zip")
	if _, err := (Builder{}).Build(opts); err == nil {
		t.Fatal("missing archive did not fail")
	}

	opts, dataFile = identityFixture(t, nil)
	if err := os.Remove(dataFile); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.parquet")
	writeParquetTimes(t, outside, defaultTimes())
	if err := os.Symlink(outside, dataFile); err != nil {
		t.Fatal(err)
	}
	if _, err := (Builder{}).Build(opts); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("dataset symlink did not fail: %v", err)
	}
}

func TestPolicyUnknownAndDuplicateFieldsFail(t *testing.T) {
	opts, _ := identityFixture(t, nil)
	valid, err := os.ReadFile(opts.AvailabilityPolicyPath)
	if err != nil {
		t.Fatal(err)
	}
	unknown := strings.TrimSuffix(string(valid), "}") + `,"secret":"x"}`
	if err := os.WriteFile(opts.AvailabilityPolicyPath, []byte(unknown), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := (Builder{}).Build(opts); err == nil || !strings.Contains(err.Error(), string(canonicalcontract.CodeUnknownField)) {
		t.Fatalf("unknown policy field did not fail: %v", err)
	}

	opts, _ = identityFixture(t, nil)
	valid, err = os.ReadFile(opts.AvailabilityPolicyPath)
	if err != nil {
		t.Fatal(err)
	}
	duplicate := strings.Replace(string(valid), `"policy_id":"availability.binance.candles"`, `"policy_id":"availability.binance.candles","policy_id":"conflict"`, 1)
	if err := os.WriteFile(opts.AvailabilityPolicyPath, []byte(duplicate), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := (Builder{}).Build(opts); err == nil || !strings.Contains(err.Error(), string(canonicalcontract.CodeDuplicateKey)) {
		t.Fatalf("duplicate policy key did not fail: %v", err)
	}
}

func TestWriteManifestIsCanonicalSelfHashedAndHasNoGovernanceFields(t *testing.T) {
	opts, _ := identityFixture(t, nil)
	manifest, err := (Builder{}).Build(opts)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(opts.EvidenceRoot, "research_identity_manifest.json")
	if err := WriteManifest(path, manifest); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(data))
	if strings.HasSuffix(string(data), "\n") || !strings.Contains(lower, "artifact_hash") || manifest.ArtifactHash == "" {
		t.Fatal("canonical manifest self hash is missing or bytes contain a newline")
	}
	if _, err := canonicalcontract.ValidateArtifact(data, true); err != nil {
		t.Fatalf("written manifest does not validate: %v", err)
	}
	for _, forbidden := range []string{"promot", "approved", "frozen", "paper_eligible", "authorized"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("manifest contains prohibited field/term %q", forbidden)
		}
	}
}

func identityFixture(t *testing.T, customTimes []int64) (BuildOptions, string) {
	t.Helper()
	evidenceRoot := t.TempDir()
	dataRoot := filepath.Join(evidenceRoot, "dataset")
	dataFile := filepath.Join(dataRoot, "candles", "futures-um", "1m", "symbol=BTCUSDT", "year=2024", "month=01", "BTCUSDT-1m-2024-01.parquet")
	if err := os.MkdirAll(filepath.Dir(dataFile), 0755); err != nil {
		t.Fatal(err)
	}
	times := customTimes
	if times == nil {
		times = defaultTimes()
	}
	writeParquetTimes(t, dataFile, times)

	archivePath := filepath.Join(evidenceRoot, "source", "BTCUSDT-1m-2024-01.zip")
	if err := os.MkdirAll(filepath.Dir(archivePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archivePath, []byte("authoritative source archive"), 0644); err != nil {
		t.Fatal(err)
	}
	availabilityPath := filepath.Join(evidenceRoot, "policies", "availability.json")
	coveragePath := filepath.Join(evidenceRoot, "policies", "coverage.json")
	if err := os.MkdirAll(filepath.Dir(availabilityPath), 0755); err != nil {
		t.Fatal(err)
	}
	availability := AvailabilityPolicy{
		Contract: canonicalcontract.NewHeader(AvailabilitySchemaName, 1, AvailabilityArtifactRole),
		PolicyID: "availability.binance.candles", PolicyVersion: "1", AvailabilityDelayNS: int64(time.Minute),
	}
	var err error
	availability.ArtifactHash, err = computeAvailabilityPolicyHash(availability)
	if err != nil {
		t.Fatal(err)
	}
	writeCanonicalArtifact(t, availabilityPath, availability)
	coverage := CoveragePolicy{
		Contract: canonicalcontract.NewHeader(CoveragePolicySchemaName, 1, CoveragePolicyArtifactRole),
		PolicyID: "coverage.zero-defect.1m", PolicyVersion: "1", Mode: StrictCoverageMode,
		IntervalNS: int64(time.Minute),
	}
	coverage.ArtifactHash, err = computeCoveragePolicyHash(coverage)
	if err != nil {
		t.Fatal(err)
	}
	writeCanonicalArtifact(t, coveragePath, coverage)

	return BuildOptions{
		DataRoot: dataRoot, EvidenceRoot: evidenceRoot,
		ManifestID: "manifest.btcusdt.2024-01", ManifestVersion: "1",
		DatasetID: "binance.futures-um.btcusdt.1m.2024-01", DatasetVersion: "2024-01.v1",
		SourceArchiveID: "binance.vision.btcusdt.1m.2024-01", SourceArchivePath: archivePath,
		InstrumentUniverseID: "universe.btcusdt",
		DatasetStartUTC:      "2024-01-01T00:00:00.000000000Z", DatasetEndUTC: "2024-01-01T00:04:00.000000000Z",
		PointInTimeCutoffUTC:   "2024-01-01T00:05:00.000000000Z",
		AvailabilityPolicyPath: availabilityPath, CoveragePolicyPath: coveragePath,
		PITEvidenceID: "pit.btcusdt.2024-01", PITEvidenceVersion: "1",
		Now: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
	}, dataFile
}

func defaultTimes() []int64 {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	return []int64{start, start + 60000, start + 120000, start + 180000, start + 240000}
}

func writeParquetTimes(t *testing.T, path string, times []int64) {
	t.Helper()
	fw, err := local.NewLocalFileWriter(path)
	if err != nil {
		t.Fatal(err)
	}
	pw, err := writer.NewParquetWriter(fw, new(parquetutil.OpenTimeRow), 1)
	if err != nil {
		_ = fw.Close()
		t.Fatal(err)
	}
	for _, value := range times {
		if err := pw.Write(parquetutil.OpenTimeRow{OpenTimeMS: value}); err != nil {
			t.Fatal(err)
		}
	}
	if err := pw.WriteStop(); err != nil {
		t.Fatal(err)
	}
	if err := fw.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeCanonicalArtifact(t *testing.T, path string, value any) {
	t.Helper()
	data, err := canonicalcontract.CanonicalizeValue(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}
