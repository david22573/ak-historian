package r1p5r

import (
	"time"

	"github.com/david22573/ak-historian/internal/prospective"
)

const (
	ProtocolVersion           = "ak-historian.pr4b0-r1p5r.reacquisition-protocol.v1"
	ExposurePolicyVersion     = "ak-historian.pr4b0-r1p5.exposure-eligibility-policy.v1"
	ReadinessPolicyVersion    = "ak-historian.pr4b0-r1p5.readiness-policy.v1"
	SourceIdentityVersion     = "ak-historian.pr4b0-r1p5r.source-identity.v1"
	AbandonedRegistryVersion  = "ak-historian.pr4b0-r1p5r.abandoned-evidence-registry.v1"
	PreacquisitionSealVersion = "ak-historian.pr4b0-r1p5r.preacquisition-seal.v1"
	ReceiptVersion            = "ak-historian.pr4b0-r1p5r.historical-reacquisition-receipt.v1"
	LedgerVersion             = "ak-historian.pr4b0-r1p5r.receipt-ledger-entry.v1"
	FragmentVersion           = "ak-historian.pr4b0-r1p5r.normalized-fragment.v1"
	StateVersion              = "ak-historian.pr4b0-r1p5r.backfill-state.v1"
	PartitionVersion          = "ak-historian.pr4b0-r1p5r.daily-partition.v1"
	CheckpointVersion         = "ak-historian.pr4b0-r1p5r.coverage-checkpoint.v1"
	Mode                      = "AUTHORITATIVE_HISTORICAL_REACQUISITION_R1P5R"
	ZeroHash                  = prospective.ZeroHash
	AbandonedSourceCommit     = "59951efd756a8024455608d298c5534c778e5121"
	AbandonedCheckpointID     = "r1p5-checkpoint-20260715T094258Z"
	AbandonedCheckpointHash   = "sha256:7d2cb161941deab10896ef95ecc04db7455501fb2621eca4a3ff4b87fa82e1b7"
	AbandonedBackfillTerminal = "sha256:645567987688724cc29d0472b30634aaae74b5688880bb9e3ab6363516154e7f"
)

var SourceSealCommit = "UNSET"

type Interval struct {
	StartUTC       time.Time `json:"start_utc"`
	EndUTC         time.Time `json:"end_utc"`
	Classification string    `json:"classification"`
}

type Protocol struct {
	SchemaVersion                string               `json:"schema_version"`
	DatasetID                    string               `json:"dataset_id"`
	AcceptedHistorianCommit      string               `json:"accepted_historian_commit"`
	RepairSourceCommit           string               `json:"repair_implementation_commit"`
	SourceIdentityPath           string               `json:"source_identity_path"`
	PreacquisitionSealPath       string               `json:"preacquisition_seal_path"`
	AbandonedRegistryPath        string               `json:"abandoned_evidence_registry_path"`
	AbandonedRegistryHash        string               `json:"abandoned_evidence_registry_hash"`
	SourceBinding                string               `json:"backfill_source_binding"`
	P4CollectorSourceCommit      string               `json:"p4_collector_source_commit"`
	P4ProtocolVersion            string               `json:"p4_protocol_version"`
	P4ProtocolHash               string               `json:"p4_protocol_hash"`
	AvailabilityPolicyVersion    string               `json:"availability_policy_version"`
	AvailabilityPolicyHash       string               `json:"availability_policy_hash"`
	SourceSchemaVersion          string               `json:"source_schema_version"`
	SourceSchemaFingerprint      string               `json:"source_schema_fingerprint"`
	SourceSchemaAuthorityHash    string               `json:"source_schema_authority_hash"`
	ManifestContractVersion      string               `json:"manifest_contract_version"`
	ManifestContractHash         string               `json:"manifest_contract_hash"`
	IngestionReceiptVersion      string               `json:"ingestion_receipt_version"`
	IngestionReceiptHash         string               `json:"ingestion_receipt_hash"`
	CoveragePolicyVersion        string               `json:"coverage_policy_version"`
	CoveragePolicyHash           string               `json:"coverage_policy_hash"`
	ExposurePolicyVersion        string               `json:"exposure_policy_version"`
	ExposurePolicyHash           string               `json:"exposure_policy_hash"`
	ReadinessPolicyVersion       string               `json:"readiness_policy_version"`
	ReadinessPolicyHash          string               `json:"readiness_policy_hash"`
	ReceiptLedgerVersion         string               `json:"receipt_ledger_version"`
	ReceiptLedgerGenesisHash     string               `json:"receipt_ledger_genesis_hash"`
	ReceiptChainID               string               `json:"receipt_chain_id"`
	StorageNamespace             string               `json:"storage_namespace"`
	EligibleStartUTC             time.Time            `json:"eligible_start_utc"`
	BarredIntervals              []Interval           `json:"barred_intervals"`
	BackfillEnds                 map[string]time.Time `json:"per_symbol_backfill_end_exclusive_utc"`
	Symbols                      []string             `json:"symbol_universe"`
	Venue                        string               `json:"venue"`
	MarketType                   string               `json:"market_type"`
	Timeframe                    string               `json:"timeframe"`
	PublicEndpoints              []string             `json:"public_endpoints"`
	AcquisitionMode              string               `json:"acquisition_mode"`
	Batching                     map[string]any       `json:"batching"`
	RateLimitPolicy              map[string]any       `json:"rate_limit_policy"`
	ProviderTimePolicy           string               `json:"provider_time_policy"`
	RawRetention                 string               `json:"raw_response_retention"`
	FragmentPolicy               string               `json:"normalized_fragment_policy"`
	ReceiptChainPolicy           string               `json:"receipt_chain_policy"`
	DailyPartitionPolicy         string               `json:"daily_partition_policy"`
	CheckpointPolicy             string               `json:"checkpoint_policy"`
	ConflictDuplicatePolicy      string               `json:"conflict_and_duplicate_policy"`
	ConcurrencyPolicy            string               `json:"live_backfill_concurrency"`
	ReadinessPolicy              string               `json:"readiness_policy"`
	ResearchProhibition          string               `json:"candidate_research_prohibition"`
	AbandonedEvidenceProhibition string               `json:"abandoned_evidence_prohibition"`
	ProtocolHash                 string               `json:"protocol_hash"`
}

type ExposurePolicy struct {
	SchemaVersion          string    `json:"schema_version"`
	ExposureLedgerVersion  string    `json:"prior_exposure_ledger_version"`
	ExposureLedgerHash     string    `json:"prior_exposure_ledger_hash"`
	InspectionAuditVersion string    `json:"inspection_audit_version"`
	InspectionAuditHash    string    `json:"inspection_audit_hash"`
	BarredFloorUTC         time.Time `json:"barred_floor_utc"`
	EligibleFloorUTC       time.Time `json:"eligible_floor_utc"`
	Classifications        []string  `json:"classifications"`
	Rules                  []string  `json:"rules"`
	PolicyHash             string    `json:"policy_hash"`
}

type ReadinessPolicy struct {
	SchemaVersion            string   `json:"schema_version"`
	MinimumDays              int      `json:"minimum_contiguous_complete_utc_days"`
	RequiredSymbols          []string `json:"required_symbols"`
	RequiredClass            string   `json:"required_partition_classification"`
	Conditions               []string `json:"conditions"`
	FeasibilityOnly          bool     `json:"future_partition_feasibility_only"`
	CandidateCountsForbidden bool     `json:"candidate_counts_forbidden"`
	ReportingCadenceSeconds  int      `json:"reporting_cadence_seconds"`
	NotReadyLabel            string   `json:"not_ready_label"`
	ReadyLabel               string   `json:"ready_label"`
	RemediationLabel         string   `json:"remediation_label"`
	PolicyHash               string   `json:"policy_hash"`
}

type SourceIdentity struct {
	SchemaVersion         string    `json:"schema_version"`
	RepairSourceCommit    string    `json:"repair_implementation_commit"`
	ProtocolHash          string    `json:"protocol_hash"`
	AbandonedRegistryHash string    `json:"abandoned_evidence_registry_hash"`
	CreatedAtUTC          time.Time `json:"created_at_utc"`
	IdentityHash          string    `json:"identity_hash"`
}

type PreservedArtifact struct {
	RelativePath string `json:"relative_path"`
	SHA256       string `json:"sha256"`
	Disposition  string `json:"disposition"`
}

type AbandonedEvidenceRegistry struct {
	SchemaVersion             string              `json:"schema_version"`
	CreatedAtUTC              time.Time           `json:"created_at_utc"`
	Reasons                   []string            `json:"reasons"`
	OldSourceCommit           string              `json:"old_source_commit"`
	OldCheckpointID           string              `json:"old_checkpoint_id"`
	OldCheckpointHash         string              `json:"old_checkpoint_hash"`
	OldBackfillTerminal       string              `json:"old_backfill_terminal"`
	OldBinaryHash             string              `json:"old_binary_hash"`
	OmittedSourceForensicHash string              `json:"omitted_source_forensic_hash"`
	PreservedArtifacts        []PreservedArtifact `json:"preserved_artifacts"`
	ProhibitedInputs          []string            `json:"prohibited_inputs"`
	RegistryHash              string              `json:"registry_hash"`
}

type PreacquisitionSeal struct {
	SchemaVersion               string            `json:"schema_version"`
	RepairSourceCommit          string            `json:"repair_implementation_commit"`
	SourceSealCommit            string            `json:"source_seal_commit"`
	ProtocolHash                string            `json:"protocol_hash"`
	SourceIdentityHash          string            `json:"source_identity_hash"`
	AbandonedRegistryHash       string            `json:"abandoned_evidence_registry_hash"`
	GoVersion                   string            `json:"go_version"`
	BuildCommand                string            `json:"build_command"`
	BuildEnvironment            map[string]string `json:"build_environment"`
	BinarySHA256                string            `json:"sealed_binary_sha256"`
	VerificationStartedAtUTC    time.Time         `json:"verification_started_at_utc"`
	VerificationCompletedAtUTC  time.Time         `json:"verification_completed_at_utc"`
	FreshClonePathClass         string            `json:"fresh_clone_path_class"`
	FreshCloneChecksPassed      bool              `json:"fresh_clone_checks_passed"`
	SafetyScansPassed           bool              `json:"safety_scans_passed"`
	NoAcquisitionReceiptsAtSeal bool              `json:"no_acquisition_receipts_at_seal"`
	SealHash                    string            `json:"seal_hash"`
}

type NormalizedRecord struct {
	prospective.NormalizedCandle
	MarketEventTimeUTC         time.Time `json:"market_event_time_utc"`
	ProviderCandleCloseTimeUTC time.Time `json:"provider_candle_close_time_utc"`
	ObservedAvailableAtUTC     time.Time `json:"observed_available_at_utc"`
	AcquiredAtUTC              time.Time `json:"acquired_at_utc"`
	AcquisitionReceiptID       string    `json:"acquisition_receipt_id"`
}

type Fragment struct {
	SchemaVersion           string             `json:"schema_version"`
	RequestID               string             `json:"request_id"`
	Symbol                  string             `json:"symbol"`
	SourceSchemaVersion     string             `json:"source_schema_version"`
	SourceSchemaFingerprint string             `json:"source_schema_fingerprint"`
	Records                 []NormalizedRecord `json:"records"`
	FragmentHash            string             `json:"fragment_hash"`
}

type Receipt struct {
	SchemaVersion               string                    `json:"schema_version"`
	AcquisitionMode             string                    `json:"acquisition_mode"`
	RequestID                   string                    `json:"request_id"`
	Symbol                      string                    `json:"symbol"`
	RequestedStartUTC           time.Time                 `json:"requested_start_utc"`
	RequestedEndExclusiveUTC    time.Time                 `json:"requested_end_exclusive_utc"`
	Endpoint                    string                    `json:"exact_endpoint"`
	CanonicalRequestParameters  string                    `json:"canonical_request_parameters"`
	RequestStartUTC             time.Time                 `json:"request_start_utc"`
	ResponseHeadersReceivedUTC  time.Time                 `json:"response_headers_received_utc"`
	CompleteResponseReceivedUTC time.Time                 `json:"complete_response_received_utc"`
	ProviderHTTPDate            string                    `json:"provider_http_date"`
	ProviderHTTPDateUTC         time.Time                 `json:"provider_http_date_utc"`
	ProviderServerTimeUTC       time.Time                 `json:"provider_server_time_utc"`
	ProviderServerTimeHash      string                    `json:"provider_server_time_response_hash"`
	ClockEvidence               prospective.ClockEvidence `json:"local_clock_synchronization_evidence"`
	HTTPStatus                  int                       `json:"http_status"`
	RetryNumber                 int                       `json:"retry_number"`
	RawByteLength               int                       `json:"raw_response_byte_length"`
	RawHash                     string                    `json:"raw_response_sha256"`
	RawPath                     string                    `json:"raw_relative_path"`
	FragmentByteLength          int                       `json:"normalized_fragment_byte_length"`
	FragmentHash                string                    `json:"normalized_fragment_hash"`
	FragmentPath                string                    `json:"fragment_relative_path"`
	ParsedRowCount              int                       `json:"parsed_row_count"`
	FirstCandleOpenUTC          time.Time                 `json:"first_candle_open_time_utc"`
	LastCandleCloseUTC          time.Time                 `json:"last_candle_close_time_utc"`
	ObservedAvailableAtUTC      time.Time                 `json:"observed_available_at_utc"`
	AcquiredAtUTC               time.Time                 `json:"acquired_at_utc"`
	PriorReceiptChainHash       string                    `json:"prior_receipt_chain_hash"`
	RepairSourceCommit          string                    `json:"repair_implementation_commit"`
	SourceSealCommit            string                    `json:"source_seal_commit"`
	SealedBinaryHash            string                    `json:"sealed_binary_sha256"`
	AbandonedRegistryHash       string                    `json:"abandoned_evidence_registry_hash"`
	ProtocolHash                string                    `json:"protocol_hash"`
	P4CollectorSourceCommit     string                    `json:"p4_collector_source_commit"`
	AvailabilityPolicyVersion   string                    `json:"availability_policy_version"`
	AvailabilityPolicyHash      string                    `json:"availability_policy_hash"`
	SourceSchemaFingerprint     string                    `json:"source_schema_fingerprint"`
	ManifestContractHash        string                    `json:"manifest_contract_hash"`
	IngestionReceiptHash        string                    `json:"ingestion_receipt_hash"`
	ResultingReceiptChainHash   string                    `json:"resulting_receipt_chain_hash"`
	ReceiptHash                 string                    `json:"receipt_hash"`
}

type LedgerEntry struct {
	SchemaVersion         string    `json:"schema_version"`
	Sequence              uint64    `json:"sequence"`
	ReceiptPath           string    `json:"receipt_relative_path"`
	ReceiptHash           string    `json:"receipt_hash"`
	PriorChainHash        string    `json:"prior_chain_hash"`
	DurableCompletionUTC  time.Time `json:"durable_completion_utc"`
	EvaluationCutoffFloor time.Time `json:"evaluation_cutoff_floor"`
	CurrentChainHash      string    `json:"current_chain_hash"`
	EntryHash             string    `json:"entry_hash"`
}

type Cursor struct {
	NextOpenUTC time.Time `json:"next_open_utc"`
	Requests    int       `json:"request_count"`
	Rows        int       `json:"row_count"`
}

type State struct {
	SchemaVersion    string            `json:"schema_version"`
	SourceSealCommit string            `json:"source_seal_commit"`
	ProtocolHash     string            `json:"protocol_hash"`
	SealedBinaryHash string            `json:"sealed_binary_sha256"`
	NextSequence     uint64            `json:"next_sequence"`
	ChainTerminal    string            `json:"receipt_chain_terminal"`
	Cursors          map[string]Cursor `json:"per_symbol_cursors"`
	StateHash        string            `json:"state_hash"`
}

type MissingInterval struct {
	StartUTC time.Time `json:"start_utc"`
	EndUTC   time.Time `json:"end_utc"`
}

type Partition struct {
	SchemaVersion      string            `json:"schema_version"`
	Symbol             string            `json:"symbol"`
	UTCDate            string            `json:"utc_date"`
	ExpectedRows       int               `json:"expected_rows"`
	ObservedRows       int               `json:"observed_rows"`
	MissingIntervals   []MissingInterval `json:"missing_intervals"`
	DuplicateCount     int               `json:"duplicate_count"`
	ConflictCount      int               `json:"conflict_count"`
	SchemaFailureCount int               `json:"schema_failure_count"`
	EvidenceGapCount   int               `json:"evidence_gap_count"`
	ClockErrorCount    int               `json:"clock_error_count"`
	ReceiptHashes      []string          `json:"receipt_hashes"`
	FragmentHashes     []string          `json:"fragment_hashes"`
	PhysicalStatus     string            `json:"physical_status"`
	PITEvidenceStatus  string            `json:"pit_evidence_status"`
	EligibilityClass   string            `json:"eligibility_classification"`
	PartitionHash      string            `json:"partition_hash"`
}

type Verification struct {
	SchemaVersion         string    `json:"schema_version"`
	VerifiedAtUTC         time.Time `json:"verified_at_utc"`
	RequestCount          int       `json:"request_count"`
	ReceiptCount          int       `json:"receipt_count"`
	RawResponseCount      int       `json:"raw_response_count"`
	FragmentCount         int       `json:"normalized_fragment_count"`
	CandleCount           int       `json:"candle_count"`
	FinalChainHash        string    `json:"final_receipt_chain_hash"`
	EvaluationCutoffFloor time.Time `json:"evaluation_cutoff_floor"`
	ConflictCount         int       `json:"conflict_count"`
	SchemaFailureCount    int       `json:"schema_failure_count"`
	EvidenceGapCount      int       `json:"evidence_gap_count"`
	ClockErrorCount       int       `json:"clock_error_count"`
	Valid                 bool      `json:"valid"`
}

type SymbolCoverage struct {
	Symbol               string    `json:"symbol"`
	StartUTC             time.Time `json:"start_utc"`
	EndUTC               time.Time `json:"end_utc"`
	ObservedCandles      int       `json:"observed_candles"`
	CompleteUTCDateCount int       `json:"complete_utc_day_count"`
	MissingIntervals     int       `json:"missing_interval_count"`
	DuplicateCount       int       `json:"duplicate_count"`
	ConflictCount        int       `json:"conflict_count"`
	EvidenceGapCount     int       `json:"evidence_gap_count"`
}

type Coverage struct {
	SchemaVersion          string           `json:"schema_version"`
	GeneratedAtUTC         time.Time        `json:"generated_at_utc"`
	EligibleStartUTC       time.Time        `json:"eligible_start_utc"`
	ContiguousEndUTC       time.Time        `json:"contiguous_end_utc"`
	CompleteEligibleDays   int              `json:"complete_eligible_utc_days"`
	PartitionCount         int              `json:"partition_count"`
	CompletePartitionCount int              `json:"complete_partition_count"`
	MissingIntervals       int              `json:"missing_interval_count"`
	DuplicateCount         int              `json:"duplicate_count"`
	ConflictCount          int              `json:"conflict_count"`
	SchemaFailureCount     int              `json:"schema_failure_count"`
	EvidenceGapCount       int              `json:"evidence_gap_count"`
	ClockErrorCount        int              `json:"clock_error_count"`
	PerSymbol              []SymbolCoverage `json:"per_symbol"`
	PartitionPaths         []string         `json:"partition_relative_paths"`
	PartitionHashes        []string         `json:"partition_hashes"`
}

type EligibilityLedger struct {
	SchemaVersion         string     `json:"schema_version"`
	GeneratedAtUTC        time.Time  `json:"generated_at_utc"`
	ExposurePolicyHash    string     `json:"exposure_policy_hash"`
	AbandonedRegistryHash string     `json:"abandoned_evidence_registry_hash"`
	Intervals             []Interval `json:"intervals"`
	LedgerHash            string     `json:"ledger_hash"`
}

type Checkpoint struct {
	SchemaVersion           string           `json:"schema_version"`
	DatasetID               string           `json:"dataset_id"`
	GenerationID            string           `json:"generation_id"`
	CreatedAtUTC            time.Time        `json:"created_at_utc"`
	CoverageStartUTC        time.Time        `json:"coverage_start_utc"`
	CoverageEndUTC          time.Time        `json:"coverage_end_utc"`
	EvaluationCutoffFloor   time.Time        `json:"evaluation_cutoff_floor"`
	RequiredSymbols         []string         `json:"required_symbol_universe"`
	SourceSchemaHash        string           `json:"source_schema_hash"`
	AvailabilityPolicyHash  string           `json:"availability_policy_hash"`
	CoveragePolicyHash      string           `json:"coverage_policy_hash"`
	ManifestContractHash    string           `json:"manifest_contract_hash"`
	IngestionReceiptHash    string           `json:"ingestion_receipt_hash"`
	P4ActivationHash        string           `json:"p4_activation_hash"`
	P4LiveChainTerminal     string           `json:"p4_live_receipt_chain_terminal"`
	P4LiveAuthorityTerminal string           `json:"p4_live_authority_chain_terminal"`
	BackfillChainGenesis    string           `json:"r1p5r_reacquisition_receipt_chain_genesis"`
	BackfillChainTerminal   string           `json:"r1p5r_reacquisition_receipt_chain_terminal"`
	RepairSourceCommit      string           `json:"repair_implementation_commit"`
	SourceSealCommit        string           `json:"source_seal_commit"`
	SealedBinaryHash        string           `json:"sealed_binary_sha256"`
	AbandonedRegistryHash   string           `json:"abandoned_evidence_registry_hash"`
	P4CollectorSourceCommit string           `json:"p4_collector_source_commit"`
	ProtocolHash            string           `json:"protocol_hash"`
	ExposureLedgerHash      string           `json:"exposure_ledger_hash"`
	PartitionPaths          []string         `json:"daily_partition_relative_paths"`
	PartitionHashes         []string         `json:"daily_partition_hashes"`
	MissingPartitions       []string         `json:"missing_partitions"`
	PhysicalComplete        bool             `json:"physical_complete"`
	PITEvidenceComplete     bool             `json:"pit_evidence_complete"`
	CompleteEligibleDays    int              `json:"complete_eligible_utc_days"`
	MissingIntervalCount    int              `json:"missing_interval_count"`
	ConflictCount           int              `json:"conflict_count"`
	EvidenceGapCount        int              `json:"evidence_gap_count"`
	ClockErrorCount         int              `json:"clock_error_count"`
	PerSymbol               []SymbolCoverage `json:"per_symbol_coverage"`
	CheckpointHash          string           `json:"checkpoint_hash"`
}

type Readiness struct {
	SchemaVersion          string           `json:"schema_version"`
	GeneratedAtUTC         time.Time        `json:"generated_at_utc"`
	CheckpointGenerationID string           `json:"immutable_checkpoint"`
	CheckpointHash         string           `json:"checkpoint_hash"`
	EligibleStartUTC       time.Time        `json:"contiguous_eligible_start_utc"`
	EligibleEndUTC         time.Time        `json:"contiguous_eligible_end_utc"`
	CompleteEligibleDays   int              `json:"complete_eligible_utc_days"`
	MinimumDays            int              `json:"minimum_required_days"`
	RemainingDays          int              `json:"remaining_structural_days"`
	ProjectedReadyDateUTC  *time.Time       `json:"projected_180_day_date_utc,omitempty"`
	PerSymbol              []SymbolCoverage `json:"per_symbol_coverage"`
	MissingIntervals       int              `json:"missing_interval_count"`
	EvidenceGaps           int              `json:"evidence_gap_count"`
	Conflicts              int              `json:"conflict_count"`
	ReceiptChainHealthy    bool             `json:"receipt_chain_healthy"`
	LiveCollectorHealthy   bool             `json:"live_collector_healthy"`
	BackfillComplete       bool             `json:"backfill_complete"`
	ClockEvidenceStatus    string           `json:"clock_evidence_status"`
	WatcherState           string           `json:"readiness_watcher_state"`
	Label                  string           `json:"label"`
}
