package archiveauthority

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func testDigest(value byte) string { return "sha256:" + strings.Repeat(string(value), 64) }

func TestAvailabilityAuthorityClassificationsAndCanonicalHash(t *testing.T) {
	available := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	verified := []AvailabilityEvidence{{EvidenceType: "SIGNED_ORIGINAL_ACQUISITION_RECEIPT", EvidenceArtifactHash: testDigest('a'), AvailabilityTimestamp: &available, OriginalAcquisition: true}}
	status, got, _ := ClassifyAvailability(verified)
	if status != AvailabilityVerified || got == nil || !got.Equal(available) {
		t.Fatal("immutable original acquisition evidence did not verify")
	}

	for name, evidence := range map[string][]AvailabilityEvidence{
		"filesystem timestamps":      {{EvidenceType: "CURRENT_FILESYSTEM_MTIME", EvidenceArtifactHash: testDigest('b'), AvailabilityTimestamp: &available, SupportingOnly: true}},
		"migrated object timestamps": {{EvidenceType: "POST_MIGRATION_OBJECT_UPLOAD_TIME", EvidenceArtifactHash: testDigest('c'), AvailabilityTimestamp: &available, SupportingOnly: true}},
	} {
		t.Run(name+" rejected", func(t *testing.T) {
			status, timestamp, _ := ClassifyAvailability(evidence)
			if status != AvailabilityPartial || timestamp != nil {
				t.Fatalf("invalid substitute classified as %s", status)
			}
		})
	}

	conflict := append(append([]AvailabilityEvidence{}, verified...), AvailabilityEvidence{EvidenceType: "SIGNED_ORIGINAL_ACQUISITION_RECEIPT", EvidenceArtifactHash: testDigest('d'), AvailabilityTimestamp: timePointer(available.Add(time.Hour)), OriginalAcquisition: true})
	if status, timestamp, _ := ClassifyAvailability(conflict); status != AvailabilityConflict || timestamp != nil {
		t.Fatal("conflicting evidence did not fail closed")
	}
	if status, timestamp, _ := ClassifyAvailability(nil); status != AvailabilityMissing || timestamp != nil {
		t.Fatal("missing evidence was invented")
	}

	authority := validAvailabilityAuthority()
	authority.Records[0].Evidence = []AvailabilityEvidence{
		{EvidenceType: "LATER_COPY", EvidenceArtifactHash: testDigest('c'), AvailabilityTimestamp: &available, SupportingOnly: true},
		{EvidenceType: "FILESYSTEM_METADATA", EvidenceArtifactHash: testDigest('b'), SupportingOnly: true},
		{EvidenceType: "SAME_TYPE", EvidenceArtifactHash: testDigest('d'), OriginalAcquisition: true},
		{EvidenceType: "SAME_TYPE", EvidenceArtifactHash: testDigest('d'), SupportingOnly: true},
	}
	first, err := SealAvailabilityAuthority(authority)
	if err != nil {
		t.Fatal(err)
	}
	slices.Reverse(authority.Records)
	slices.Reverse(authority.SearchedArtifacts)
	slices.Reverse(authority.Records[1].Evidence)
	second, err := SealAvailabilityAuthority(authority)
	if err != nil {
		t.Fatal(err)
	}
	if first.AuthorityHash != second.AuthorityHash {
		t.Fatal("reordered evidence changed canonical identity")
	}
	authority.Records[0].ContentHash = testDigest('e')
	mutated, err := SealAvailabilityAuthority(authority)
	if err != nil {
		t.Fatal(err)
	}
	if mutated.AuthorityHash == first.AuthorityHash {
		t.Fatal("evidence mutation did not change authority hash")
	}
}

func validAvailabilityAuthority() AvailabilityAuthority {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	searchHash := testDigest('f')
	record := func(symbol string) AvailabilityRecord {
		return AvailabilityRecord{ManifestRelativeIdentity: symbol + ".parquet", PartitionKey: "futures-um/1m/" + symbol + "/2025-01", Symbol: symbol, CoveredMonth: "2025-01", ContentHash: testDigest('a'), ClaimedSourcePeriodStart: start, ClaimedSourcePeriodEnd: start.AddDate(0, 1, 0), EvidenceType: "NO_AUTHORITATIVE_ACQUISITION_EVIDENCE_FOUND", EvidenceArtifactHash: searchHash, Status: AvailabilityMissing, Reason: "search exhausted; no authoritative original-acquisition evidence recovered"}
	}
	return AvailabilityAuthority{PhysicalArchiveClassification: "IMMUTABLE_PHYSICAL_ARCHIVE_GAP_IDENTITY", DatasetID: "dataset-v1", DatasetVersion: testDigest('b'), ManifestID: "manifest-v1", ManifestHash: testDigest('c'), SearchScope: []string{"repository history", "archive metadata"}, SearchedArtifacts: []SearchArtifact{{ArtifactID: "search-evidence", SHA256: searchHash, EvidenceType: "SEARCH_RECORD", Disposition: "GAP_ONLY", Reason: "no authority"}}, Records: []AvailabilityRecord{record("BBB"), record("AAA")}}
}

func timePointer(value time.Time) *time.Time { return &value }
