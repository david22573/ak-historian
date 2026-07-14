package prospective

import (
	"time"

	"github.com/david22573/ak-historian/internal/archiveauthority"
)

const (
	ProtocolVersion           = "ak-historian.pr4b0-r1p4.collection-protocol.v1"
	AvailabilityPolicyVersion = "ak-historian.pr4b0-r1p4.availability-policy.v1"
	CoveragePolicyVersion     = "ak-historian.pr4b0-r1p4.coverage-policy.v1"
	ActivationVersion         = "ak-historian.pr4b0-r1p4.dataset-activation.v1"
	SupervisorContractVersion = "ak-historian.pr4b0-r1p4.supervisor-contract.v1"
	ReceiptEnvelopeVersion    = "ak-historian.pr4b0-r1p4.acquisition-receipt.v1"
	FragmentVersion           = "ak-historian.pr4b0-r1p4.normalized-fragment.v1"
	CycleVersion              = "ak-historian.pr4b0-r1p4.collection-cycle.v1"
	PartitionManifestVersion  = "ak-historian.pr4b0-r1p4.partition-manifest.v1"
	CheckpointVersion         = "ak-historian.pr4b0-r1p4.dataset-checkpoint.v1"
	StateVersion              = "ak-historian.pr4b0-r1p4.collection-state.v1"
	SourceSchemaVersion       = archiveauthority.SourceCandleSchemaVersion
	ManifestContractVersion   = "ak-historian.prospective-manifest-contract.v1"
	ReceiptSchemaVersion      = archiveauthority.ProspectiveReceiptSchemaVersion
	SourceSchemaFingerprint   = "sha256:eb5cc448b9333dd76a51fea226833f054d200c2c22a0b8ff3a4ba21905f7c1be"
	SourceSchemaAuthorityHash = "sha256:4ff5ef49773e3d9a65d50e64d3a7d3ecc6a50d32699c8af1de41ed3518cc99c5"
	ManifestContractHash      = "sha256:a07c34075721250db17e56a13d55d66cbcea934d013866305b61506a37323882"
	ReceiptSchemaHash         = "sha256:7b69a348fed2facf918a869a333c79d5c2d6d9df2bd10c9c52efd2f179398ff1"
	ZeroHash                  = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
)

var (
	PrimarySymbols = []string{"ADAUSDT", "AVAXUSDT", "BNBUSDT", "DOGEUSDT", "ETHUSDT", "LINKUSDT", "SOLUSDT", "XRPUSDT"}
	ContextSymbols = []string{"BTCUSDT", "ETHUSDT"}
	UniqueSymbols  = []string{"ADAUSDT", "AVAXUSDT", "BNBUSDT", "BTCUSDT", "DOGEUSDT", "ETHUSDT", "LINKUSDT", "SOLUSDT", "XRPUSDT"}
)

type Protocol struct {
	SchemaVersion              string            `json:"schema_version"`
	DatasetID                  string            `json:"dataset_id"`
	GenerationRule             string            `json:"dataset_generation_rule"`
	Venue                      string            `json:"source_venue"`
	MarketType                 string            `json:"market_type"`
	EndpointFamily             []string          `json:"public_endpoint_family"`
	SourceSchemaVersion        string            `json:"source_schema_version"`
	SourceSchemaFingerprint    string            `json:"source_schema_fingerprint"`
	SourceSchemaAuthorityHash  string            `json:"source_schema_authority_hash"`
	IngestionReceiptVersion    string            `json:"ingestion_receipt_version"`
	IngestionReceiptHash       string            `json:"ingestion_receipt_hash"`
	ManifestContractVersion    string            `json:"manifest_contract_version"`
	ManifestContractHash       string            `json:"manifest_contract_hash"`
	PrimarySymbols             []string          `json:"required_primary_symbols"`
	ContextSymbols             []string          `json:"required_context_symbols"`
	UniqueSymbols              []string          `json:"total_unique_symbol_universe"`
	Timeframe                  string            `json:"timeframe"`
	CadenceSeconds             int               `json:"collection_cadence_seconds"`
	CompletedCandleRule        string            `json:"completed_candle_rule"`
	AvailabilityEvidencePolicy string            `json:"availability_evidence_policy"`
	ClockEvidencePolicy        string            `json:"clock_evidence_policy"`
	RetryRateLimitPolicy       map[string]any    `json:"retry_and_rate_limit_policy"`
	CanonicalSerialization     string            `json:"canonical_serialization"`
	RawRetentionPolicy         string            `json:"raw_response_retention_policy"`
	FragmentRetentionPolicy    string            `json:"normalized_fragment_retention_policy"`
	PartitioningPolicy         string            `json:"partitioning_policy"`
	HashChainPolicy            string            `json:"hash_chain_policy"`
	DuplicateConflictPolicy    string            `json:"duplicate_and_conflict_handling"`
	RestartCatchupPolicy       string            `json:"restart_and_catch_up_behavior"`
	GapSemantics               string            `json:"gap_semantics"`
	SupervisorMechanism        string            `json:"supervisor_mechanism"`
	ActivationSequence         []string          `json:"activation_sequence"`
	SecurityBoundaries         []string          `json:"security_boundaries"`
	ResearchProhibition        string            `json:"candidate_evaluation_prohibition"`
	StartingCommits            map[string]string `json:"accepted_starting_commits"`
	GovernanceHashes           map[string]string `json:"accepted_governance_hashes"`
	ProtocolHash               string            `json:"protocol_hash"`
}

type AvailabilityPolicy struct {
	SchemaVersion                string   `json:"schema_version"`
	ObservedAvailableCalculation string   `json:"observed_available_at_calculation"`
	PITEligibilityRequirements   []string `json:"pit_eligibility_requirements"`
	IncompleteEvidenceStatus     string   `json:"incomplete_evidence_status"`
	ForbiddenAvailabilitySources []string `json:"forbidden_availability_inference_sources"`
	LocalClockRequirement        string   `json:"local_clock_requirement"`
	PolicyHash                   string   `json:"policy_hash"`
}

type SupervisorContract struct {
	SchemaVersion  string   `json:"schema_version"`
	Mechanism      string   `json:"mechanism"`
	UserLevel      bool     `json:"user_level"`
	CadenceSeconds int      `json:"cadence_seconds"`
	Recovery       string   `json:"recovery"`
	Commands       []string `json:"commands"`
	PathPolicy     string   `json:"path_policy"`
	SecretPolicy   string   `json:"secret_policy"`
	LoggingPolicy  string   `json:"logging_policy"`
	SingleInstance string   `json:"single_instance"`
	ContractHash   string   `json:"contract_hash"`
}

type Activation struct {
	SchemaVersion             string    `json:"schema_version"`
	DatasetID                 string    `json:"dataset_id"`
	Generation                string    `json:"generation"`
	ActivationTimestamp       time.Time `json:"activation_timestamp"`
	CollectorSourceCommit     string    `json:"collector_source_commit"`
	CollectorBuildID          string    `json:"collector_build_id"`
	ProtocolHash              string    `json:"protocol_hash"`
	SourceSchemaVersion       string    `json:"source_schema_version"`
	SourceSchemaFingerprint   string    `json:"source_schema_fingerprint"`
	AvailabilityPolicyVersion string    `json:"availability_policy_version"`
	AvailabilityPolicyHash    string    `json:"availability_policy_hash"`
	CoveragePolicyVersion     string    `json:"coverage_policy_version"`
	IngestionReceiptVersion   string    `json:"ingestion_receipt_version"`
	IngestionReceiptHash      string    `json:"ingestion_receipt_hash"`
	ManifestContractVersion   string    `json:"manifest_contract_version"`
	ManifestContractHash      string    `json:"manifest_contract_hash"`
	UniqueSymbols             []string  `json:"required_symbol_universe"`
	Timeframe                 string    `json:"timeframe"`
	CadenceSeconds            int       `json:"cadence_seconds"`
	PartitionPolicy           string    `json:"partition_policy"`
	ReceiptLedgerGenesisHash  string    `json:"receipt_ledger_genesis_hash"`
	CheckpointRule            string    `json:"checkpoint_rule"`
	ActivationHash            string    `json:"activation_hash"`
}

type ClockEvidence struct {
	CheckedAtUTC time.Time `json:"checked_at_utc"`
	Method       string    `json:"method"`
	Synchronized bool      `json:"synchronized"`
	EvidenceHash string    `json:"evidence_hash"`
	Diagnostic   string    `json:"diagnostic"`
}

type NormalizedCandle struct {
	Market              string `json:"market"`
	Symbol              string `json:"symbol"`
	Interval            string `json:"interval"`
	Period              string `json:"period"`
	SourceDate          string `json:"source_date"`
	OpenTimeMS          int64  `json:"open_time_ms"`
	Open                string `json:"open"`
	High                string `json:"high"`
	Low                 string `json:"low"`
	Close               string `json:"close"`
	Volume              string `json:"volume"`
	CloseTimeMS         int64  `json:"close_time_ms"`
	QuoteAssetVolume    string `json:"quote_asset_volume"`
	NumberOfTrades      int64  `json:"number_of_trades"`
	TakerBuyBaseVolume  string `json:"taker_buy_base_volume"`
	TakerBuyQuoteVolume string `json:"taker_buy_quote_volume"`
}

type Fragment struct {
	SchemaVersion           string             `json:"schema_version"`
	NormalizationVersion    string             `json:"normalization_version"`
	CycleID                 string             `json:"cycle_id"`
	Symbol                  string             `json:"symbol"`
	SourceSchemaVersion     string             `json:"source_schema_version"`
	SourceSchemaFingerprint string             `json:"source_schema_fingerprint"`
	Records                 []NormalizedCandle `json:"records"`
	FragmentHash            string             `json:"fragment_hash"`
}

type Receipt struct {
	SchemaVersion               string                              `json:"schema_version"`
	CycleID                     string                              `json:"cycle_id"`
	CollectorSourceCommit       string                              `json:"collector_source_commit"`
	ProtocolHash                string                              `json:"protocol_hash"`
	RequestID                   string                              `json:"request_id"`
	Symbol                      string                              `json:"symbol"`
	Endpoint                    string                              `json:"exact_endpoint"`
	CanonicalRequestParameters  string                              `json:"canonical_request_parameters"`
	RequestStartUTC             time.Time                           `json:"request_start_utc"`
	ResponseHeadersReceivedUTC  time.Time                           `json:"response_headers_received_utc"`
	CompleteResponseReceivedUTC time.Time                           `json:"complete_response_received_utc"`
	ProviderHTTPDate            string                              `json:"provider_http_date"`
	ProviderHTTPDateUTC         time.Time                           `json:"provider_http_date_utc"`
	ProviderServerTimeUTC       time.Time                           `json:"provider_server_time_utc"`
	ProviderServerTimeHash      string                              `json:"provider_server_time_response_hash"`
	ClockEvidence               ClockEvidence                       `json:"local_clock_synchronization_evidence"`
	HTTPStatus                  int                                 `json:"http_status"`
	RetryNumber                 int                                 `json:"retry_number"`
	ResponseBodyByteLength      int                                 `json:"response_body_byte_length"`
	RawResponseSHA256           string                              `json:"raw_response_sha256"`
	ParsedRecordCount           int                                 `json:"parsed_record_count"`
	FirstCandleOpenTimeUTC      time.Time                           `json:"first_candle_open_time_utc"`
	FinalCandleCloseTimeUTC     time.Time                           `json:"final_candle_close_time_utc"`
	ReceiptCreationTimeUTC      time.Time                           `json:"receipt_creation_time_utc"`
	ObservedAvailableAtUTC      time.Time                           `json:"observed_available_at_utc"`
	AvailabilityStatus          string                              `json:"availability_status"`
	RawRelativePath             string                              `json:"raw_relative_path"`
	FragmentRelativePath        string                              `json:"fragment_relative_path"`
	FragmentHash                string                              `json:"fragment_hash"`
	PriorReceiptChainHash       string                              `json:"prior_receipt_chain_hash"`
	CurrentReceiptChainHash     string                              `json:"current_receipt_chain_hash"`
	AuthorityReceipt            archiveauthority.ProspectiveReceipt `json:"accepted_authority_receipt"`
}

type Cursor struct {
	LastOpenTimeMS  int64  `json:"last_open_time_ms"`
	LastReceiptHash string `json:"last_receipt_hash"`
}

type State struct {
	SchemaVersion          string            `json:"schema_version"`
	DatasetID              string            `json:"dataset_id"`
	ActivationHash         string            `json:"activation_hash"`
	NextRegistration       uint64            `json:"next_registration_sequence"`
	LastAuthorityHash      string            `json:"last_authority_receipt_hash"`
	LastEnvelopeHash       string            `json:"last_envelope_receipt_hash"`
	Cursors                map[string]Cursor `json:"cursors"`
	LastSuccessfulCycleID  string            `json:"last_successful_cycle_id"`
	LastSuccessfulCycleUTC time.Time         `json:"last_successful_cycle_utc"`
	StateHash              string            `json:"state_hash"`
}

type SymbolCycleStatus struct {
	Symbol         string `json:"symbol"`
	Success        bool   `json:"success"`
	ReceiptHash    string `json:"receipt_hash,omitempty"`
	Records        int    `json:"normalized_record_count"`
	Duplicates     int    `json:"duplicate_count"`
	Conflicts      int    `json:"conflict_count"`
	SchemaFailures int    `json:"schema_failure_count"`
	Error          string `json:"error,omitempty"`
}

type CycleResult struct {
	SchemaVersion         string              `json:"schema_version"`
	CycleID               string              `json:"cycle_id"`
	StartedAtUTC          time.Time           `json:"started_at_utc"`
	CompletedAtUTC        time.Time           `json:"completed_at_utc"`
	ProviderServerTimeUTC time.Time           `json:"provider_server_time_utc"`
	ClockEvidence         ClockEvidence       `json:"clock_evidence"`
	FullUniverseSuccess   bool                `json:"full_universe_success"`
	Symbols               []SymbolCycleStatus `json:"symbols"`
	CycleHash             string              `json:"cycle_hash"`
}

type MissingInterval struct {
	StartOpenTimeUTC time.Time `json:"start_open_time_utc"`
	EndOpenTimeUTC   time.Time `json:"end_open_time_utc"`
}

type PartitionManifest struct {
	SchemaVersion             string            `json:"schema_version"`
	DatasetID                 string            `json:"dataset_id"`
	Generation                string            `json:"generation"`
	Symbol                    string            `json:"symbol"`
	UTCDate                   string            `json:"utc_date"`
	ExpectedRows              int               `json:"expected_rows"`
	ObservedRows              int               `json:"observed_rows"`
	MissingIntervals          []MissingInterval `json:"missing_intervals"`
	DuplicateIntervals        []time.Time       `json:"duplicate_intervals"`
	FirstOpenTimeUTC          *time.Time        `json:"first_open_time_utc"`
	LastCloseTimeUTC          *time.Time        `json:"last_close_time_utc"`
	ConstituentFragmentHashes []string          `json:"constituent_fragment_hashes"`
	ReceiptIdentities         []string          `json:"receipt_identities"`
	EarliestAvailabilityUTC   *time.Time        `json:"earliest_observed_availability_utc"`
	LatestAvailabilityUTC     *time.Time        `json:"latest_observed_availability_utc"`
	SourceSchemaHash          string            `json:"source_schema_hash"`
	PartitionStatus           string            `json:"status"`
	PhysicalStatus            string            `json:"physical_status"`
	PITEvidenceStatus         string            `json:"pit_evidence_status"`
	ConflictCount             int               `json:"conflict_count"`
	PartitionHash             string            `json:"partition_hash"`
}

type DatasetCheckpoint struct {
	SchemaVersion      string    `json:"schema_version"`
	DatasetID          string    `json:"dataset_id"`
	Generation         string    `json:"generation"`
	ActivationHash     string    `json:"activation_hash"`
	CreatedAtUTC       time.Time `json:"created_at_utc"`
	ReceiptCount       int       `json:"receipt_count"`
	FinalReceiptHash   string    `json:"final_receipt_chain_hash"`
	PartitionManifests []string  `json:"partition_manifest_relative_paths"`
	PartitionHashes    []string  `json:"partition_hashes"`
	Eligible           bool      `json:"checkpoint_eligible"`
	IneligibleReasons  []string  `json:"ineligible_reasons"`
	CheckpointHash     string    `json:"checkpoint_hash"`
}
