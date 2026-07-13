package pitarchive

import "time"

const (
	SnapshotManifestSchemaVersion   = "ak-historian.snapshot-manifest.v1"
	SnapshotSchemaVersion           = "ak-historian.snapshot.v1"
	CoveragePolicySchemaVersion     = "ak-historian.coverage-policy.v1"
	AvailabilityPolicySchemaVersion = "ak-historian.availability-policy.v1"
	EvidenceSchemaVersion           = "ak-historian.pit-evidence.v1"
	EvaluationResultSchemaVersion   = "ak-historian.pit-evaluation-result.v1"

	DefaultMaxManifestBytes int64 = 4 << 20
	DefaultMaxSnapshotBytes int64 = 1 << 30
	DefaultMaxEvidenceBytes int64 = 4 << 20
)

type Verdict string

const (
	VerdictEligible           Verdict = "PIT_ELIGIBLE"
	VerdictIneligible         Verdict = "PIT_INELIGIBLE"
	VerdictEvidenceIncomplete Verdict = "PIT_EVIDENCE_INCOMPLETE"
	VerdictEvidenceCorrupt    Verdict = "PIT_EVIDENCE_CORRUPT"
	VerdictDiagnosticOnly     Verdict = "PIT_DIAGNOSTIC_ONLY"
)

type CheckVerdict string

const (
	CheckPass CheckVerdict = "PASS"
	CheckFail CheckVerdict = "FAIL"
)

type ReasonCode string

const (
	ReasonManifestMissing                ReasonCode = "MANIFEST_MISSING"
	ReasonManifestEmpty                  ReasonCode = "MANIFEST_EMPTY"
	ReasonManifestUnreadable             ReasonCode = "MANIFEST_UNREADABLE"
	ReasonManifestMalformed              ReasonCode = "MANIFEST_MALFORMED"
	ReasonManifestSchemaUnsupported      ReasonCode = "MANIFEST_SCHEMA_UNSUPPORTED"
	ReasonManifestFieldMissing           ReasonCode = "MANIFEST_FIELD_MISSING"
	ReasonManifestDatasetMismatch        ReasonCode = "MANIFEST_DATASET_MISMATCH"
	ReasonManifestWindowMismatch         ReasonCode = "MANIFEST_WINDOW_MISMATCH"
	ReasonManifestHashMismatch           ReasonCode = "MANIFEST_HASH_MISMATCH"
	ReasonManifestSnapshotCountMismatch  ReasonCode = "MANIFEST_SNAPSHOT_COUNT_MISMATCH"
	ReasonSnapshotMissing                ReasonCode = "SNAPSHOT_MISSING"
	ReasonSnapshotHashMismatch           ReasonCode = "SNAPSHOT_HASH_MISMATCH"
	ReasonSnapshotDuplicateID            ReasonCode = "SNAPSHOT_DUPLICATE_ID"
	ReasonSnapshotPartitionConflict      ReasonCode = "SNAPSHOT_PARTITION_CONFLICT"
	ReasonSnapshotSchemaUnsupported      ReasonCode = "SNAPSHOT_SCHEMA_UNSUPPORTED"
	ReasonSnapshotPathInvalid            ReasonCode = "SNAPSHOT_PATH_INVALID"
	ReasonSnapshotTooLarge               ReasonCode = "SNAPSHOT_TOO_LARGE"
	ReasonSnapshotSizeMismatch           ReasonCode = "SNAPSHOT_SIZE_MISMATCH"
	ReasonCoveragePolicyUnsupported      ReasonCode = "COVERAGE_POLICY_UNSUPPORTED"
	ReasonCoveragePolicyInvalid          ReasonCode = "COVERAGE_POLICY_INVALID"
	ReasonCoverageIncomplete             ReasonCode = "COVERAGE_INCOMPLETE"
	ReasonAvailabilityTimestampMissing   ReasonCode = "AVAILABILITY_TIMESTAMP_MISSING"
	ReasonAvailableAfterEvaluation       ReasonCode = "AVAILABLE_AFTER_EVALUATION"
	ReasonPublicationDelayViolation      ReasonCode = "PUBLICATION_DELAY_VIOLATION"
	ReasonFutureSnapshotTimestamp        ReasonCode = "FUTURE_SNAPSHOT_TIMESTAMP"
	ReasonEvaluationCutoffMissing        ReasonCode = "EVALUATION_CUTOFF_MISSING"
	ReasonEvidenceSchemaUnsupported      ReasonCode = "EVIDENCE_SCHEMA_UNSUPPORTED"
	ReasonEvidenceIntegrityHashMismatch  ReasonCode = "EVIDENCE_INTEGRITY_HASH_MISMATCH"
	ReasonEvidenceStrictPromotionInvalid ReasonCode = "EVIDENCE_STRICT_PROMOTION_INVALID"
	ReasonEvidenceSecurityFieldMissing   ReasonCode = "EVIDENCE_SECURITY_FIELD_MISSING"
)

type Failure struct {
	Code         ReasonCode `json:"code"`
	Message      string     `json:"message"`
	SnapshotID   string     `json:"snapshot_id,omitempty"`
	PartitionKey string     `json:"partition_key,omitempty"`
}

type SnapshotManifest struct {
	SchemaVersion       string              `json:"schema_version"`
	ManifestID          string              `json:"manifest_id"`
	DatasetID           string              `json:"dataset_id"`
	DatasetVersion      string              `json:"dataset_version"`
	ResearchWindowStart time.Time           `json:"research_window_start"`
	ResearchWindowEnd   time.Time           `json:"research_window_end"`
	GeneratedAt         time.Time           `json:"generated_at"`
	Source              string              `json:"source"`
	ArchiveID           string              `json:"archive_id"`
	CoveragePolicy      CoveragePolicy      `json:"coverage_policy"`
	AvailabilityPolicy  AvailabilityPolicy  `json:"availability_policy"`
	SnapshotCount       int                 `json:"snapshot_count"`
	Snapshots           []SnapshotReference `json:"snapshots"`
	ManifestHash        string              `json:"manifest_hash"`
}

type SnapshotReference struct {
	SnapshotID     string    `json:"snapshot_id"`
	PartitionKey   string    `json:"partition_key"`
	RelativePath   string    `json:"relative_path"`
	EventTimeStart time.Time `json:"event_time_start"`
	EventTimeEnd   time.Time `json:"event_time_end"`
	AvailableAt    time.Time `json:"available_at"`
	IngestedAt     time.Time `json:"ingested_at,omitempty"`
	ContentHash    string    `json:"content_hash"`
	SchemaVersion  string    `json:"schema_version"`
	ByteSize       int64     `json:"byte_size"`
}

type CoveragePolicy struct {
	SchemaVersion     string                 `json:"schema_version"`
	PolicyID          string                 `json:"policy_id"`
	PartitionModel    string                 `json:"partition_model"`
	MaximumGapSeconds *int64                 `json:"maximum_gap_seconds"`
	Required          []PartitionRequirement `json:"required_partitions"`
	Exceptions        []CoverageException    `json:"exceptions"`
}

type PartitionRequirement struct {
	PartitionKey   string    `json:"partition_key"`
	EventTimeStart time.Time `json:"event_time_start"`
	EventTimeEnd   time.Time `json:"event_time_end"`
}

type CoverageException struct {
	ExceptionID    string    `json:"exception_id"`
	EventTimeStart time.Time `json:"event_time_start"`
	EventTimeEnd   time.Time `json:"event_time_end"`
	Reason         string    `json:"reason"`
}

type AvailabilityPolicy struct {
	SchemaVersion                   string `json:"schema_version"`
	PolicyID                        string `json:"policy_id"`
	RequiredPublicationDelaySeconds *int64 `json:"required_publication_delay_seconds"`
}

type CoverageReport struct {
	PolicyID                   string       `json:"policy_id"`
	PolicySchemaVersion        string       `json:"policy_schema_version"`
	ExpectedPartitions         []string     `json:"expected_partitions"`
	ObservedPartitions         []string     `json:"observed_partitions"`
	MissingPartitions          []string     `json:"missing_partitions"`
	DuplicatePartitions        []string     `json:"duplicate_partitions"`
	OutOfWindowPartitions      []string     `json:"out_of_window_partitions"`
	CoverageRatio              string       `json:"coverage_ratio"`
	MaximumObservedGapSeconds  int64        `json:"maximum_observed_gap_seconds"`
	MaximumPermittedGapSeconds int64        `json:"maximum_permitted_gap_seconds"`
	StrictVerdict              CheckVerdict `json:"strict_verdict"`
}

type AvailabilityReport struct {
	PolicyID         string       `json:"policy_id"`
	EvaluationCutoff time.Time    `json:"evaluation_cutoff"`
	AcceptedCount    int          `json:"accepted_count"`
	RejectedCount    int          `json:"rejected_count"`
	StrictVerdict    CheckVerdict `json:"strict_verdict"`
}

type SnapshotIntegrityReport struct {
	VerifiedCount int          `json:"verified_count"`
	RejectedCount int          `json:"rejected_count"`
	StrictVerdict CheckVerdict `json:"strict_verdict"`
}

type EvidenceEnvelope struct {
	SchemaVersion             string                  `json:"schema_version"`
	EvidenceID                string                  `json:"evidence_id"`
	DatasetID                 string                  `json:"dataset_id"`
	DatasetVersion            string                  `json:"dataset_version"`
	ResearchWindowStart       time.Time               `json:"research_window_start"`
	ResearchWindowEnd         time.Time               `json:"research_window_end"`
	EvaluationCutoff          time.Time               `json:"evaluation_cutoff"`
	ManifestID                string                  `json:"manifest_id"`
	ManifestHash              string                  `json:"manifest_hash"`
	ArchiveID                 string                  `json:"archive_id"`
	CoveragePolicyVersion     string                  `json:"coverage_policy_version"`
	ManifestValidationVerdict CheckVerdict            `json:"manifest_validation_verdict"`
	SnapshotIntegrity         SnapshotIntegrityReport `json:"snapshot_integrity"`
	Coverage                  CoverageReport          `json:"coverage"`
	SnapshotCount             int                     `json:"snapshot_count"`
	SnapshotSetDigest         string                  `json:"snapshot_set_digest"`
	Availability              AvailabilityReport      `json:"availability"`
	FinalVerdict              Verdict                 `json:"final_verdict"`
	StrictPromotionAllowed    bool                    `json:"strict_promotion_allowed"`
	GeneratedAt               time.Time               `json:"generated_at"`
	HistorianBuild            string                  `json:"historian_build"`
	IntegrityHash             string                  `json:"integrity_hash"`
}

type EvaluationResult struct {
	SchemaVersion          string            `json:"schema_version"`
	Verdict                Verdict           `json:"verdict"`
	StrictPromotionAllowed bool              `json:"strict_promotion_allowed"`
	Failures               []Failure         `json:"failures"`
	Evidence               *EvidenceEnvelope `json:"evidence,omitempty"`
}

type EvaluateOptions struct {
	ManifestPath        string
	ArchiveRoot         string
	DatasetID           string
	DatasetVersion      string
	ResearchWindowStart time.Time
	ResearchWindowEnd   time.Time
	EvaluationCutoff    time.Time
	Strict              bool
	MaxManifestBytes    int64
	MaxSnapshotBytes    int64
	Now                 time.Time
	HistorianBuild      string
}
