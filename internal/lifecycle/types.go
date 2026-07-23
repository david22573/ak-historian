package lifecycle

const (
	StatusActive  = "ACTIVE"
	StatusDelist  = "DELISTED"
	StatusExpired = "EXPIRED"
	StatusRenamed = "RENAMED"
	StatusUnknown = "UNKNOWN"

	EvidenceVerifiedExchangeListing   = "VERIFIED_EXCHANGE_LISTING"
	EvidenceVerifiedExchangeDelisting = "VERIFIED_EXCHANGE_DELISTING"
	EvidenceHistoricalSnapshot        = "HISTORICAL_SNAPSHOT_EVIDENCE"
	EvidenceLocalDataFirstSeen        = "LOCAL_DATA_FIRST_SEEN"
	EvidenceCurrentActiveOnly         = "CURRENT_ACTIVE_ONLY"
	EvidenceUserProvidedUnverified    = "USER_PROVIDED_UNVERIFIED"
	EvidenceUnknown                   = "UNKNOWN"

	SupportLowSupported = "LOW_SUPPORTED"
	SupportElevated     = "ELEVATED"
	SupportUnknown      = "UNKNOWN"

	ListingEvidenceVerified      = "VERIFIED"
	ListingEvidenceFirstSeenOnly = "FIRST_SEEN_ONLY"
	ListingEvidenceCurrentOnly   = "CURRENT_ACTIVE_ONLY"
	ListingEvidenceUserProvided  = "USER_PROVIDED_UNVERIFIED"
	ListingEvidenceMissing       = "MISSING"
	ListingEvidenceUnknown       = "UNKNOWN"

	DelistingEvidenceVerified = "VERIFIED"
	DelistingEvidenceMissing  = "MISSING"
	DelistingEvidenceUnknown  = "UNKNOWN"

	CodeLifecycleEmpty                         = "LIFECYCLE_EMPTY"
	CodeLifecycleDuplicateSymbol               = "LIFECYCLE_DUPLICATE_SYMBOL"
	CodeLifecycleSymbolMalformed               = "LIFECYCLE_SYMBOL_MALFORMED"
	CodeLifecycleListedAfterDelisted           = "LIFECYCLE_LISTED_AFTER_DELISTED"
	CodeLifecycleStatusUnknown                 = "LIFECYCLE_STATUS_UNKNOWN"
	CodeLifecycleDelistedDateMissing           = "LIFECYCLE_DELISTED_DATE_MISSING"
	CodeLifecycleListedDateMissing             = "LIFECYCLE_LISTED_DATE_MISSING"
	CodeLifecycleEvidenceMissing               = "LIFECYCLE_EVIDENCE_MISSING"
	CodeLifecycleSourceHashMissing             = "LIFECYCLE_SOURCE_HASH_MISSING"
	CodeLifecycleCurrentActiveOnlyRisk         = "LIFECYCLE_CURRENT_ACTIVE_ONLY_RISK"
	CodeLifecycleLocalDataOnlyNotListingProof  = "LIFECYCLE_LOCAL_DATA_ONLY_NOT_LISTING_PROOF"
	CodeLifecycleUserProvidedUnverified        = "LIFECYCLE_USER_PROVIDED_UNVERIFIED"
	CodeLifecycleLowRiskUnproven               = "LIFECYCLE_LOW_RISK_UNPROVEN"
	CodeLifecycleExchangeSnapshotCurrentOnly   = "LIFECYCLE_EXCHANGE_SNAPSHOT_CURRENT_ONLY"
	CodeLifecycleExchangeSnapshotEvidence      = "LIFECYCLE_EXCHANGE_SNAPSHOT_EVIDENCE"
	CodeLifecycleSymbolDisappearedNoProof      = "LIFECYCLE_SYMBOL_DISAPPEARED_WITHOUT_DELISTING_PROOF"
	CodeLifecycleListingFromExchangeMetadata   = "LIFECYCLE_LISTING_DATE_FROM_EXCHANGE_METADATA"
	CodeLifecycleDelistingFromExchangeMetadata = "LIFECYCLE_DELISTING_DATE_FROM_EXCHANGE_METADATA"
	CodeLifecycleSnapshotObservedTimeMissing   = "LIFECYCLE_SNAPSHOT_OBSERVED_TIME_MISSING"
	CodeLifecycleSnapshotUnverifiedSource      = "LIFECYCLE_SNAPSHOT_UNVERIFIED_SOURCE"
	CodeLifecycleSnapshotTrustWeak             = "LIFECYCLE_SNAPSHOT_TRUST_WEAK"
	CodeLifecycleBackfillEvidencePartial       = "LIFECYCLE_BACKFILL_EVIDENCE_PARTIAL"
)

type Warning struct {
	Code    string `json:"code"`
	Target  string `json:"target,omitempty"`
	Message string `json:"message"`
}

type Validation struct {
	IsValid                    bool     `json:"is_valid"`
	Status                     string   `json:"status"`
	WarningCodes               []string `json:"warning_codes,omitempty"`
	ListingEvidenceStatus      string   `json:"listing_evidence_status"`
	DelistingEvidenceStatus    string   `json:"delisting_evidence_status"`
	SurvivorshipSupportStatus  string   `json:"survivorship_support_status"`
	LifecycleEvidenceSupported bool     `json:"lifecycle_evidence_supported"`
}

type SourceEntry struct {
	SourceType      string   `json:"source_type"`
	SourceName      string   `json:"source_name"`
	SourceURIOrPath string   `json:"source_uri_or_path"`
	SourceHash      string   `json:"source_hash"`
	ObservedAtUTC   string   `json:"observed_at_utc"`
	EvidenceFields  []string `json:"evidence_fields"`
	Confidence      string   `json:"confidence"`
	Notes           string   `json:"notes,omitempty"`
}

type SymbolEntry struct {
	Symbol        string        `json:"symbol"`
	BaseAsset     string        `json:"base_asset"`
	QuoteAsset    string        `json:"quote_asset"`
	MarketType    string        `json:"market_type"`
	Exchange      string        `json:"exchange"`
	Status        string        `json:"status"`
	ListedAtUTC   string        `json:"listed_at_utc"`
	DelistedAtUTC string        `json:"delisted_at_utc"`
	FirstSeenUTC  string        `json:"first_seen_utc"`
	LastSeenUTC   string        `json:"last_seen_utc"`
	EvidenceLevel string        `json:"evidence_level"`
	Sources       []SourceEntry `json:"sources"`
	Warnings      []Warning     `json:"warnings"`
}

type Hashes struct {
	LifecycleHash string `json:"lifecycle_hash"`
	ManifestHash  string `json:"manifest_hash"`
}

type Manifest struct {
	SchemaVersion                            string        `json:"schema_version"`
	ManifestVersion                          string        `json:"manifest_version"`
	LifecycleID                              string        `json:"lifecycle_id"`
	LifecycleName                            string        `json:"lifecycle_name"`
	SourceRepo                               string        `json:"source_repo"`
	SourceGitSHA                             string        `json:"source_git_sha"`
	SourceType                               string        `json:"source_type"`
	GeneratedAtUTC                           string        `json:"generated_at_utc"`
	Exchange                                 string        `json:"exchange"`
	MarketType                               string        `json:"market_type"`
	QuoteAsset                               string        `json:"quote_asset"`
	EffectiveStartUTC                        string        `json:"effective_start_utc"`
	EffectiveEndUTC                          string        `json:"effective_end_utc"`
	ExchangeMetadataSnapshotHash             string        `json:"exchange_metadata_snapshot_hash,omitempty"`
	ExchangeMetadataSnapshotManifestHash     string        `json:"exchange_metadata_snapshot_manifest_hash,omitempty"`
	ExchangeMetadataSnapshotArchiveHash      string        `json:"exchange_metadata_snapshot_archive_hash,omitempty"`
	ExchangeMetadataSnapshotCoverageStartUTC string        `json:"exchange_metadata_snapshot_coverage_start_utc,omitempty"`
	ExchangeMetadataSnapshotCoverageEndUTC   string        `json:"exchange_metadata_snapshot_coverage_end_utc,omitempty"`
	ExchangeMetadataSnapshotEvidenceLevel    string        `json:"exchange_metadata_snapshot_evidence_level,omitempty"`
	ExchangeMetadataSnapshotCurrentOnly      bool          `json:"exchange_metadata_snapshot_current_only,omitempty"`
	PointInTimeCoverageStatus                string        `json:"point_in_time_coverage_status,omitempty"`
	Symbols                                  []SymbolEntry `json:"symbols"`
	Validation                               Validation    `json:"validation"`
	Warnings                                 []Warning     `json:"warnings"`
	Hashes                                   Hashes        `json:"hashes"`
}

type Summary struct {
	LifecycleID                              string         `json:"lifecycle_id,omitempty"`
	LifecycleHash                            string         `json:"lifecycle_hash,omitempty"`
	LifecycleManifestHash                    string         `json:"lifecycle_manifest_hash,omitempty"`
	LifecycleEvidenceLevelSummary            map[string]int `json:"lifecycle_evidence_level_summary,omitempty"`
	LifecycleSourceType                      string         `json:"lifecycle_source_type,omitempty"`
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
}
