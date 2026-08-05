package researchidentity

import (
	"time"

	"github.com/david22573/ak-historian/internal/canonicalcontract"
)

const (
	ManifestSchemaName         = "ak.historian.research_identity_manifest"
	ManifestSchemaVersion      = 1
	ManifestArtifactRole       = "research_identity_manifest"
	DatasetSchemaName          = "ak.historian.dataset_identity"
	DatasetArtifactRole        = "dataset_identity"
	ArchiveSchemaName          = "ak.historian.archive_identity"
	ArchiveArtifactRole        = "source_archive"
	AvailabilitySchemaName     = "ak.historian.availability_policy"
	AvailabilityArtifactRole   = "availability_policy"
	CoveragePolicySchemaName   = "ak.historian.coverage_policy"
	CoveragePolicyArtifactRole = "coverage_policy"
	CoverageSchemaName         = "ak.historian.coverage_evidence"
	CoverageArtifactRole       = "coverage_evidence"
	PITSchemaName              = "ak.historian.pit_evidence"
	PITArtifactRole            = "pit_evidence"
	StrictCoverageMode         = "STRICT_ZERO_DEFECT_FULL_WINDOW"
	EvidenceStatusPass         = "PASS"
	RawObjectContractName      = "ak.raw.object"
)

type AvailabilityPolicy struct {
	Contract            canonicalcontract.ContractHeader `json:"contract"`
	ArtifactHash        string                           `json:"artifact_hash"`
	PolicyID            string                           `json:"policy_id"`
	PolicyVersion       string                           `json:"policy_version"`
	AvailabilityDelayNS int64                            `json:"availability_delay_ns"`
}

type CoveragePolicy struct {
	Contract           canonicalcontract.ContractHeader `json:"contract"`
	ArtifactHash       string                           `json:"artifact_hash"`
	PolicyID           string                           `json:"policy_id"`
	PolicyVersion      string                           `json:"policy_version"`
	Mode               string                           `json:"mode"`
	IntervalNS         int64                            `json:"interval_ns"`
	AllowGaps          bool                             `json:"allow_gaps"`
	AllowDuplicates    bool                             `json:"allow_duplicates"`
	AllowOutOfOrder    bool                             `json:"allow_out_of_order"`
	AllowPartialWindow bool                             `json:"allow_partial_window"`
}

type RawObjectReference struct {
	LogicalID    string `json:"logical_id"`
	MediaType    string `json:"media_type"`
	ObjectHash   string `json:"object_hash"`
	RelativePath string `json:"relative_path"`
	SizeBytes    int64  `json:"size_bytes"`
}

type ArchiveIdentity struct {
	Contract      canonicalcontract.ContractHeader `json:"contract"`
	ArtifactHash  string                           `json:"artifact_hash"`
	ArchiveID     string                           `json:"archive_id"`
	MediaType     string                           `json:"media_type"`
	RawObjectHash string                           `json:"raw_object_hash"`
	RelativePath  string                           `json:"relative_path"`
	SizeBytes     int64                            `json:"size_bytes"`
}

type DatasetObjectIdentity struct {
	RelativePath       string `json:"relative_path"`
	Symbol             string `json:"symbol"`
	Interval           string `json:"interval"`
	SizeBytes          int64  `json:"size_bytes"`
	RawObjectHash      string `json:"raw_object_hash"`
	RowCount           int64  `json:"row_count"`
	WindowRowCount     int64  `json:"window_row_count"`
	EarliestEventUTC   string `json:"earliest_event_utc"`
	LatestEventUTC     string `json:"latest_event_utc"`
	LatestAvailableUTC string `json:"latest_available_utc"`
}

type DatasetIdentity struct {
	Contract             canonicalcontract.ContractHeader `json:"contract"`
	ArtifactHash         string                           `json:"artifact_hash"`
	DatasetID            string                           `json:"dataset_id"`
	DatasetVersion       string                           `json:"dataset_version"`
	InstrumentUniverseID string                           `json:"instrument_universe_id"`
	Symbols              []string                         `json:"symbols"`
	DatasetStartUTC      string                           `json:"dataset_start_utc"`
	DatasetEndUTC        string                           `json:"dataset_end_utc"`
	Objects              []DatasetObjectIdentity          `json:"objects"`
}

type CoverageEvidence struct {
	Contract                canonicalcontract.ContractHeader `json:"contract"`
	ArtifactHash            string                           `json:"artifact_hash"`
	Status                  string                           `json:"status"`
	FullWindow              bool                             `json:"full_window"`
	RequestedStartUTC       string                           `json:"requested_start_utc"`
	RequestedEndUTC         string                           `json:"requested_end_utc"`
	EarliestEventUTC        string                           `json:"earliest_event_utc"`
	LatestEventUTC          string                           `json:"latest_event_utc"`
	RowCount                int64                            `json:"row_count"`
	ExpectedRowCount        int64                            `json:"expected_row_count"`
	SeriesCount             int                              `json:"series_count"`
	GapCount                int64                            `json:"gap_count"`
	DuplicateTimestampCount int64                            `json:"duplicate_timestamp_count"`
	OutOfOrderCount         int64                            `json:"out_of_order_count"`
}

type PITEvidence struct {
	Contract                canonicalcontract.ContractHeader `json:"contract"`
	ArtifactHash            string                           `json:"artifact_hash"`
	EvidenceID              string                           `json:"evidence_id"`
	EvidenceVersion         string                           `json:"evidence_version"`
	Status                  string                           `json:"status"`
	DatasetID               string                           `json:"dataset_id"`
	DatasetVersion          string                           `json:"dataset_version"`
	DatasetHash             string                           `json:"dataset_hash"`
	SourceArchiveHash       string                           `json:"source_archive_hash"`
	AvailabilityPolicyHash  string                           `json:"availability_policy_hash"`
	CoveragePolicyHash      string                           `json:"coverage_policy_hash"`
	EvaluationCutoffUTC     string                           `json:"evaluation_cutoff_utc"`
	EarliestEventUTC        string                           `json:"earliest_event_utc"`
	LatestEventUTC          string                           `json:"latest_event_utc"`
	EarliestAvailableUTC    string                           `json:"earliest_available_utc"`
	LatestAvailableUTC      string                           `json:"latest_available_utc"`
	FullWindowCoverage      bool                             `json:"full_window_coverage"`
	AvailabilityDelayNS     int64                            `json:"availability_delay_ns"`
	GapCount                int64                            `json:"gap_count"`
	DuplicateTimestampCount int64                            `json:"duplicate_timestamp_count"`
	OutOfOrderCount         int64                            `json:"out_of_order_count"`
}

type Manifest struct {
	Contract                 canonicalcontract.ContractHeader `json:"contract"`
	ArtifactHash             string                           `json:"artifact_hash"`
	ManifestID               string                           `json:"manifest_id"`
	ManifestVersion          string                           `json:"manifest_version"`
	DatasetStartUTC          string                           `json:"dataset_start_utc"`
	DatasetEndUTC            string                           `json:"dataset_end_utc"`
	PointInTimeCutoffUTC     string                           `json:"point_in_time_cutoff_utc"`
	Dataset                  DatasetIdentity                  `json:"dataset"`
	SourceArchive            ArchiveIdentity                  `json:"source_archive"`
	AvailabilityPolicy       AvailabilityPolicy               `json:"availability_policy"`
	AvailabilityPolicySource RawObjectReference               `json:"availability_policy_source"`
	CoveragePolicy           CoveragePolicy                   `json:"coverage_policy"`
	CoveragePolicySource     RawObjectReference               `json:"coverage_policy_source"`
	CoverageEvidence         CoverageEvidence                 `json:"coverage_evidence"`
	PITEvidence              PITEvidence                      `json:"pit_evidence"`
}

type BuildOptions struct {
	DataRoot               string
	EvidenceRoot           string
	ManifestID             string
	ManifestVersion        string
	DatasetID              string
	DatasetVersion         string
	SourceArchiveID        string
	SourceArchivePath      string
	InstrumentUniverseID   string
	DatasetStartUTC        string
	DatasetEndUTC          string
	PointInTimeCutoffUTC   string
	AvailabilityPolicyPath string
	CoveragePolicyPath     string
	PITEvidenceID          string
	PITEvidenceVersion     string
	Now                    time.Time
}
