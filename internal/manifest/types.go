package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/david22573/ak-historian/internal/coverage"
)

type Validation struct {
	Status   string   `json:"status"`
	Warnings []string `json:"warnings"`
}

type Survivorship struct {
	UniverseID                               string         `json:"universe_id,omitempty"`
	UniverseHash                             string         `json:"universe_hash,omitempty"`
	UniverseManifestHash                     string         `json:"universe_manifest_hash,omitempty"`
	UniversePolicy                           string         `json:"universe_policy"`
	IncludesDelistedAssets                   string         `json:"includes_delisted_assets"`
	SurvivorshipBiasRisk                     string         `json:"survivorship_bias_risk"`
	LifecycleID                              string         `json:"lifecycle_id,omitempty"`
	LifecycleHash                            string         `json:"lifecycle_hash,omitempty"`
	LifecycleManifestHash                    string         `json:"lifecycle_manifest_hash,omitempty"`
	LifecycleEvidenceLevelSummary            map[string]int `json:"lifecycle_evidence_level_summary,omitempty"`
	LifecycleWarnings                        []string       `json:"lifecycle_warnings,omitempty"`
	ListingEvidenceStatus                    string         `json:"listing_evidence_status,omitempty"`
	DelistingEvidenceStatus                  string         `json:"delisting_evidence_status,omitempty"`
	SurvivorshipSupportStatus                string         `json:"survivorship_support_status,omitempty"`
	ExchangeMetadataSnapshotHash             string         `json:"exchange_metadata_snapshot_hash,omitempty"`
	ExchangeMetadataSnapshotManifestHash     string         `json:"exchange_metadata_snapshot_manifest_hash,omitempty"`
	ExchangeMetadataSnapshotArchiveHash      string         `json:"exchange_metadata_snapshot_archive_hash,omitempty"`
	ExchangeMetadataSnapshotCoverageStartUTC string         `json:"exchange_metadata_snapshot_coverage_start_utc,omitempty"`
	ExchangeMetadataSnapshotCoverageEndUTC   string         `json:"exchange_metadata_snapshot_coverage_end_utc,omitempty"`
	ExchangeMetadataSnapshotEvidenceLevel    string         `json:"exchange_metadata_snapshot_evidence_level,omitempty"`
	ExchangeMetadataSnapshotCurrentOnly      bool           `json:"exchange_metadata_snapshot_current_only,omitempty"`
	PointInTimeCoverageStatus                string         `json:"point_in_time_coverage_status,omitempty"`
	PointInTimeCoverageHash                  string         `json:"point_in_time_coverage_hash,omitempty"`
	PointInTimePromotionRecommendation       string         `json:"point_in_time_promotion_recommendation,omitempty"`
	WarningCode                              string         `json:"warning_code,omitempty"`
	Warnings                                 []string       `json:"warnings,omitempty"`
}

type Hashes struct {
	DatasetHash  string `json:"dataset_hash"`
	ManifestHash string `json:"manifest_hash"`
}

type FileEntry struct {
	RelativePath      string `json:"relative_path"`
	Symbol            string `json:"symbol,omitempty"`
	Interval          string `json:"interval,omitempty"`
	MinTimestampUTC   string `json:"min_timestamp_utc,omitempty"`
	MaxTimestampUTC   string `json:"max_timestamp_utc,omitempty"`
	RowCount          *int64 `json:"row_count,omitempty"`
	SchemaFingerprint string `json:"schema_fingerprint,omitempty"`
	SHA256            string `json:"sha256"`
}

type DatasetManifest struct {
	SchemaVersion   string                    `json:"schema_version"`
	ManifestVersion string                    `json:"manifest_version"`
	DatasetID       string                    `json:"dataset_id"`
	DatasetRole     string                    `json:"dataset_role"`
	SourceRepo      string                    `json:"source_repo"`
	SourceGitSHA    string                    `json:"source_git_sha"`
	SourceType      string                    `json:"source_type"`
	GeneratedAtUTC  string                    `json:"generated_at_utc,omitempty"`
	DataRoot        string                    `json:"data_root,omitempty"`
	Symbols         []string                  `json:"symbols"`
	Intervals       []string                  `json:"intervals"`
	MinTimestampUTC string                    `json:"min_timestamp_utc,omitempty"`
	MaxTimestampUTC string                    `json:"max_timestamp_utc,omitempty"`
	RowCountTotal   *int64                    `json:"row_count_total,omitempty"`
	Coverage        *coverage.DatasetCoverage `json:"coverage,omitempty"`
	Validation      Validation                `json:"validation"`
	Survivorship    Survivorship              `json:"survivorship"`
	Hashes          Hashes                    `json:"hashes"`
	Files           []FileEntry               `json:"files"`
}

// ComputeHash calculates the manifest_hash deterministically.
func (m *DatasetManifest) ComputeHash() (string, error) {
	// Create a copy to zero out fields that shouldn't be hashed
	copyM := *m
	copyM.GeneratedAtUTC = ""
	copyM.DataRoot = ""
	copyM.Hashes.ManifestHash = ""

	// Ensure files, symbols, and intervals are non-nil for stable hashing
	if copyM.Files == nil {
		copyM.Files = []FileEntry{}
	}
	if copyM.Symbols == nil {
		copyM.Symbols = []string{}
	}
	if copyM.Intervals == nil {
		copyM.Intervals = []string{}
	}
	sort.Strings(copyM.Symbols)
	sort.Strings(copyM.Intervals)
	sort.Strings(copyM.Validation.Warnings)
	sort.Strings(copyM.Survivorship.Warnings)
	sort.Strings(copyM.Survivorship.LifecycleWarnings)
	sort.SliceStable(copyM.Files, func(i, j int) bool {
		return copyM.Files[i].RelativePath < copyM.Files[j].RelativePath
	})

	b, err := json.Marshal(copyM)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:]), nil
}
