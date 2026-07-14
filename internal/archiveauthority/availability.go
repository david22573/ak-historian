package archiveauthority

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	AvailabilityAuthoritySchemaVersion = "ak-historian.source-availability-authority.v1"
	AvailabilityGapSchemaVersion       = "ak-historian.source-availability-gap-manifest.v1"

	AvailabilityVerified = "AVAILABILITY_AUTHORITY_VERIFIED"
	AvailabilityPartial  = "AVAILABILITY_AUTHORITY_PARTIAL"
	AvailabilityMissing  = "AVAILABILITY_AUTHORITY_MISSING"
	AvailabilityConflict = "AVAILABILITY_AUTHORITY_CONFLICT"
)

type SearchArtifact struct {
	ArtifactID   string `json:"artifact_id"`
	SHA256       string `json:"sha256"`
	EvidenceType string `json:"evidence_type"`
	Disposition  string `json:"disposition"`
	Reason       string `json:"reason"`
}

type AvailabilityEvidence struct {
	EvidenceType          string     `json:"evidence_type"`
	EvidenceArtifactHash  string     `json:"evidence_artifact_hash"`
	AvailabilityTimestamp *time.Time `json:"availability_timestamp"`
	OriginalAcquisition   bool       `json:"original_immutable_acquisition"`
	SupportingOnly        bool       `json:"supporting_only"`
}

type AvailabilityRecord struct {
	ManifestRelativeIdentity          string                 `json:"manifest_relative_identity"`
	PartitionKey                      string                 `json:"partition_key"`
	Symbol                            string                 `json:"symbol"`
	CoveredMonth                      string                 `json:"covered_month"`
	ContentHash                       string                 `json:"content_hash"`
	ClaimedSourcePeriodStart          time.Time              `json:"claimed_source_period_start"`
	ClaimedSourcePeriodEnd            time.Time              `json:"claimed_source_period_end"`
	EarliestAuthoritativeAvailability *time.Time             `json:"earliest_authoritative_availability_timestamp"`
	EvidenceType                      string                 `json:"evidence_type"`
	EvidenceArtifactHash              string                 `json:"evidence_artifact_hash"`
	Status                            string                 `json:"status"`
	Reason                            string                 `json:"reason"`
	Evidence                          []AvailabilityEvidence `json:"evidence,omitempty"`
}

type AvailabilityAuthority struct {
	SchemaVersion                 string               `json:"schema_version"`
	PhysicalArchiveClassification string               `json:"physical_archive_classification"`
	DatasetID                     string               `json:"dataset_id"`
	DatasetVersion                string               `json:"dataset_version"`
	ManifestID                    string               `json:"manifest_id"`
	ManifestHash                  string               `json:"manifest_hash"`
	SearchScope                   []string             `json:"search_scope"`
	SearchedArtifacts             []SearchArtifact     `json:"searched_artifacts"`
	Records                       []AvailabilityRecord `json:"records"`
	StatusCounts                  map[string]int       `json:"status_counts"`
	AuthorityHash                 string               `json:"authority_hash"`
}

type AvailabilityGapManifest struct {
	SchemaVersion  string               `json:"schema_version"`
	DatasetID      string               `json:"dataset_id"`
	DatasetVersion string               `json:"dataset_version"`
	ManifestID     string               `json:"manifest_id"`
	ManifestHash   string               `json:"manifest_hash"`
	Records        []AvailabilityRecord `json:"records"`
	GapHash        string               `json:"gap_hash"`
}

func SearchArtifactSetHash(artifacts []SearchArtifact) (string, error) {
	copyArtifacts := append([]SearchArtifact{}, artifacts...)
	sort.Slice(copyArtifacts, func(i, j int) bool { return copyArtifacts[i].ArtifactID < copyArtifacts[j].ArtifactID })
	return canonicalHash(copyArtifacts)
}

func ClassifyAvailability(evidence []AvailabilityEvidence) (string, *time.Time, string) {
	var authoritative []time.Time
	partial := false
	for _, item := range evidence {
		if item.SupportingOnly || !item.OriginalAcquisition {
			partial = true
			continue
		}
		if item.AvailabilityTimestamp != nil && validDigest(item.EvidenceArtifactHash) {
			authoritative = append(authoritative, item.AvailabilityTimestamp.UTC())
		}
	}
	if len(authoritative) == 0 {
		if partial {
			return AvailabilityPartial, nil, "supporting or later-copy metadata cannot establish original source availability"
		}
		return AvailabilityMissing, nil, "no authoritative immutable original-acquisition evidence was recovered"
	}
	sort.Slice(authoritative, func(i, j int) bool { return authoritative[i].Before(authoritative[j]) })
	for _, value := range authoritative[1:] {
		if !value.Equal(authoritative[0]) {
			return AvailabilityConflict, nil, "authoritative availability evidence conflicts"
		}
	}
	value := authoritative[0]
	return AvailabilityVerified, &value, "original immutable acquisition evidence verifies"
}

func SealAvailabilityAuthority(authority AvailabilityAuthority) (AvailabilityAuthority, error) {
	authority.SchemaVersion = AvailabilityAuthoritySchemaVersion
	authority.AuthorityHash = ""
	authority.SearchScope = append([]string{}, authority.SearchScope...)
	authority.SearchedArtifacts = append([]SearchArtifact{}, authority.SearchedArtifacts...)
	authority.Records = append([]AvailabilityRecord{}, authority.Records...)
	sort.Strings(authority.SearchScope)
	sort.Slice(authority.SearchedArtifacts, func(i, j int) bool {
		return authority.SearchedArtifacts[i].ArtifactID < authority.SearchedArtifacts[j].ArtifactID
	})
	sort.Slice(authority.Records, func(i, j int) bool {
		return authority.Records[i].ManifestRelativeIdentity < authority.Records[j].ManifestRelativeIdentity
	})
	counts := map[string]int{AvailabilityVerified: 0, AvailabilityPartial: 0, AvailabilityMissing: 0, AvailabilityConflict: 0}
	for index := range authority.Records {
		record := &authority.Records[index]
		record.Evidence = append([]AvailabilityEvidence{}, record.Evidence...)
		sort.Slice(record.Evidence, func(i, j int) bool {
			left, right := record.Evidence[i], record.Evidence[j]
			if left.EvidenceType != right.EvidenceType {
				return left.EvidenceType < right.EvidenceType
			}
			if left.EvidenceArtifactHash != right.EvidenceArtifactHash {
				return left.EvidenceArtifactHash < right.EvidenceArtifactHash
			}
			if (left.AvailabilityTimestamp == nil) != (right.AvailabilityTimestamp == nil) {
				return left.AvailabilityTimestamp == nil
			}
			if left.AvailabilityTimestamp != nil && !left.AvailabilityTimestamp.Equal(*right.AvailabilityTimestamp) {
				return left.AvailabilityTimestamp.Before(*right.AvailabilityTimestamp)
			}
			if left.OriginalAcquisition != right.OriginalAcquisition {
				return !left.OriginalAcquisition
			}
			return !left.SupportingOnly && right.SupportingOnly
		})
		status, timestamp, reason := ClassifyAvailability(record.Evidence)
		if len(record.Evidence) == 0 && record.Status == AvailabilityMissing {
			status, timestamp, reason = AvailabilityMissing, nil, record.Reason
		}
		record.Status, record.EarliestAuthoritativeAvailability = status, timestamp
		if strings.TrimSpace(record.Reason) == "" {
			record.Reason = reason
		}
		if _, ok := counts[status]; !ok {
			return AvailabilityAuthority{}, fmt.Errorf("record %d has unknown status", index)
		}
		counts[status]++
		if strings.TrimSpace(record.ManifestRelativeIdentity) == "" || strings.TrimSpace(record.Symbol) == "" || strings.TrimSpace(record.CoveredMonth) == "" || !validDigest(record.ContentHash) || !record.ClaimedSourcePeriodStart.Before(record.ClaimedSourcePeriodEnd) || !validDigest(record.EvidenceArtifactHash) {
			return AvailabilityAuthority{}, fmt.Errorf("record %d is incomplete", index)
		}
	}
	authority.StatusCounts = counts
	if authority.PhysicalArchiveClassification != "IMMUTABLE_PHYSICAL_ARCHIVE_GAP_IDENTITY" || strings.TrimSpace(authority.DatasetID) == "" || !validDigest(authority.DatasetVersion) || strings.TrimSpace(authority.ManifestID) == "" || !validDigest(authority.ManifestHash) || len(authority.Records) == 0 {
		return AvailabilityAuthority{}, errors.New("availability authority identity is invalid")
	}
	hash, err := canonicalHash(authority)
	if err != nil {
		return AvailabilityAuthority{}, err
	}
	authority.AuthorityHash = hash
	return authority, nil
}

func VerifyAvailabilityAuthority(authority AvailabilityAuthority) error {
	want := authority.AuthorityHash
	sealed, err := SealAvailabilityAuthority(authority)
	if err != nil {
		return err
	}
	if want != sealed.AuthorityHash {
		return errors.New("availability authority hash mismatch")
	}
	return nil
}

func BuildAvailabilityGap(authority AvailabilityAuthority) (AvailabilityGapManifest, error) {
	if err := VerifyAvailabilityAuthority(authority); err != nil {
		return AvailabilityGapManifest{}, err
	}
	records := make([]AvailabilityRecord, 0)
	for _, record := range authority.Records {
		if record.Status != AvailabilityVerified {
			record.Evidence = nil
			records = append(records, record)
		}
	}
	gap := AvailabilityGapManifest{SchemaVersion: AvailabilityGapSchemaVersion, DatasetID: authority.DatasetID, DatasetVersion: authority.DatasetVersion, ManifestID: authority.ManifestID, ManifestHash: authority.ManifestHash, Records: records}
	hash, err := canonicalHash(gap)
	if err != nil {
		return AvailabilityGapManifest{}, err
	}
	gap.GapHash = hash
	return gap, nil
}
