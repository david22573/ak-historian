package exchange_meta

const (
	SchemaVersion   = "1.0.0"
	SnapshotVersion = "1.0.0"
	ManifestVersion = "1.0.0"

	StatusActive     = "ACTIVE"
	StatusTrading    = "TRADING"
	StatusBreak      = "BREAK"
	StatusHalt       = "HALT"
	StatusPreTrading = "PRE_TRADING"
	StatusSettling   = "SETTLING"
	StatusDelivered  = "DELIVERED"
	StatusDelisted   = "DELISTED"
	StatusExpired    = "EXPIRED"
	StatusUnknown    = "UNKNOWN"

	CodeEmptySymbolSet      = "EXCHANGE_META_EMPTY_SYMBOL_SET"
	CodeDuplicateSymbol     = "EXCHANGE_META_DUPLICATE_SYMBOL"
	CodeSymbolMalformed     = "EXCHANGE_META_SYMBOL_MALFORMED"
	CodeStatusUnknown       = "EXCHANGE_META_STATUS_UNKNOWN"
	CodeOnboardDateMissing  = "EXCHANGE_META_ONBOARD_DATE_MISSING"
	CodeDeliveryDateMissing = "EXCHANGE_META_DELIVERY_DATE_MISSING"
	CodeRawPayloadMissing   = "EXCHANGE_META_RAW_PAYLOAD_MISSING"
	CodeCurrentOnlySource   = "EXCHANGE_META_CURRENT_ONLY_SOURCE"
	CodeSourceTimeUnknown   = "EXCHANGE_META_SOURCE_TIME_UNKNOWN"
	CodeStatusUnmapped      = "EXCHANGE_META_SYMBOL_STATUS_UNMAPPED"

	TrustLevelOfficialArchive            = "OFFICIAL_ARCHIVE"
	TrustLevelExchangeRawResponseArchive = "EXCHANGE_RAW_RESPONSE_ARCHIVE"
	TrustLevelUserProvidedVerifiedHash   = "USER_PROVIDED_VERIFIED_HASH"
	TrustLevelUserProvidedUnverified     = "USER_PROVIDED_UNVERIFIED"
	TrustLevelThirdPartyArchive          = "THIRD_PARTY_ARCHIVE"
	TrustLevelUnknown                    = "UNKNOWN"

	CodeBackfillInputEmpty               = "EXCHANGE_BACKFILL_INPUT_EMPTY"
	CodeBackfillInputParseFailed         = "EXCHANGE_BACKFILL_INPUT_PARSE_FAILED"
	CodeBackfillObservedTimeMissing      = "EXCHANGE_BACKFILL_OBSERVED_TIME_MISSING"
	CodeBackfillTrustLevelUnknown        = "EXCHANGE_BACKFILL_TRUST_LEVEL_UNKNOWN"
	CodeBackfillUserProvidedUnverified   = "EXCHANGE_BACKFILL_USER_PROVIDED_UNVERIFIED"
	CodeBackfillDuplicateSnapshot        = "EXCHANGE_BACKFILL_DUPLICATE_SNAPSHOT"
	CodeBackfillConflictingSnapshot      = "EXCHANGE_BACKFILL_CONFLICTING_SNAPSHOT"
	CodeBackfillSourceHashMissing        = "EXCHANGE_BACKFILL_SOURCE_HASH_MISSING"
	CodeBackfillSourceTimeAfterCollected = "EXCHANGE_BACKFILL_SOURCE_TIME_AFTER_COLLECTED_TIME"
	CodeBackfillUnsupportedFormat        = "EXCHANGE_BACKFILL_UNSUPPORTED_FORMAT"
	CodeBackfillArchiveVerifyFailed      = "EXCHANGE_BACKFILL_ARCHIVE_VERIFY_FAILED"

	CodeLifecycleSnapshotObservedTimeMissing = "LIFECYCLE_SNAPSHOT_OBSERVED_TIME_MISSING"
	CodeLifecycleSnapshotUnverifiedSource    = "LIFECYCLE_SNAPSHOT_UNVERIFIED_SOURCE"
	CodeLifecycleSnapshotTrustWeak           = "LIFECYCLE_SNAPSHOT_TRUST_WEAK"
	CodeLifecycleBackfillEvidencePartial     = "LIFECYCLE_BACKFILL_EVIDENCE_PARTIAL"
)

type Warning struct {
	Code    string `json:"code"`
	Target  string `json:"target,omitempty"`
	Message string `json:"message"`
}

type Validation struct {
	IsValid      bool     `json:"is_valid"`
	Status       string   `json:"status"`
	WarningCodes []string `json:"warning_codes,omitempty"`
	SymbolCount  int      `json:"symbol_count"`
	CurrentOnly  bool     `json:"current_only"`
}

type Symbol struct {
	Symbol            string    `json:"symbol"`
	BaseAsset         string    `json:"base_asset"`
	QuoteAsset        string    `json:"quote_asset"`
	MarketType        string    `json:"market_type"`
	Exchange          string    `json:"exchange"`
	Status            string    `json:"status"`
	ContractType      *string   `json:"contract_type"`
	OnboardDateUTC    *string   `json:"onboard_date_utc"`
	DeliveryDateUTC   *string   `json:"delivery_date_utc"`
	FirstTradeDateUTC *string   `json:"first_trade_date_utc"`
	Permissions       []string  `json:"permissions,omitempty"`
	RawStatus         string    `json:"raw_status,omitempty"`
	SourceFields      []string  `json:"source_fields"`
	Warnings          []Warning `json:"warnings"`
}

type Hashes struct {
	SnapshotHash          string `json:"snapshot_hash"`
	SymbolSetHash         string `json:"symbol_set_hash"`
	NormalizedPayloadHash string `json:"normalized_payload_hash"`
}

type Snapshot struct {
	SchemaVersion           string     `json:"schema_version"`
	SnapshotVersion         string     `json:"snapshot_version"`
	SnapshotID              string     `json:"snapshot_id"`
	Exchange                string     `json:"exchange"`
	MarketType              string     `json:"market_type"`
	QuoteAssetFilter        string     `json:"quote_asset_filter"`
	SourceType              string     `json:"source_type"`
	SourceName              string     `json:"source_name"`
	SourceURI               string     `json:"source_uri"`
	CollectedAtUTC          string     `json:"collected_at_utc"`
	SourceObservedTimeUTC   *string    `json:"source_observed_time_utc"`
	TrustLevel              string     `json:"trust_level"`
	CollectorGitSHA         string     `json:"collector_git_sha"`
	RawPayloadSHA256        string     `json:"raw_payload_sha256"`
	NormalizedPayloadSHA256 string     `json:"normalized_payload_sha256"`
	Symbols                 []Symbol   `json:"symbols"`
	Validation              Validation `json:"validation"`
	Warnings                []Warning  `json:"warnings"`
	Hashes                  Hashes     `json:"hashes"`
}

type ManifestSnapshotRef struct {
	SnapshotID     string `json:"snapshot_id"`
	CollectedAtUTC string `json:"collected_at_utc"`
	SourceName     string `json:"source_name"`
	SnapshotHash   string `json:"snapshot_hash"`
	TrustLevel     string `json:"trust_level"`
	SymbolCount    int    `json:"symbol_count"`
	RelativePath   string `json:"relative_path"`
}

type SymbolLifecycleEvidence struct {
	FirstSeenUTC    string   `json:"first_seen_utc"`
	LastSeenUTC     string   `json:"last_seen_utc"`
	SeenInSnapshots int      `json:"seen_in_snapshots"`
	Statuses        []string `json:"statuses"`
	HasOnboardDate  bool     `json:"has_onboard_date"`
	HasDeliveryDate bool     `json:"has_delivery_date"`
	SnapshotHashes  []string `json:"snapshot_hashes"`
}

type ManifestHashes struct {
	ArchiveHash        string `json:"archive_hash"`
	ManifestHash       string `json:"manifest_hash"`
	SymbolPresenceHash string `json:"symbol_presence_hash"`
}

type SnapshotManifest struct {
	SchemaVersion                  string                             `json:"schema_version"`
	ManifestVersion                string                             `json:"manifest_version"`
	ArchiveID                      string                             `json:"archive_id"`
	Exchange                       string                             `json:"exchange"`
	MarketType                     string                             `json:"market_type"`
	EffectiveStartUTC              string                             `json:"effective_start_utc"`
	EffectiveEndUTC                string                             `json:"effective_end_utc"`
	SnapshotCount                  int                                `json:"snapshot_count"`
	Snapshots                      []ManifestSnapshotRef              `json:"snapshots"`
	SymbolLifecycleEvidenceSummary map[string]SymbolLifecycleEvidence `json:"symbol_lifecycle_evidence_summary"`
	TrustLevelSummary              map[string]int                     `json:"trust_level_summary,omitempty"`
	EarliestObservedTimeUTC        string                             `json:"earliest_observed_time_utc,omitempty"`
	LatestObservedTimeUTC          string                             `json:"latest_observed_time_utc,omitempty"`
	SourceCount                    int                                `json:"source_count"`
	OfficialSourceCount            int                                `json:"official_source_count"`
	UnverifiedSourceCount          int                                `json:"unverified_source_count"`
	ObservedTimeMissingCount       int                                `json:"observed_time_missing_count"`
	ProvenanceWarnings             []Warning                          `json:"provenance_warnings,omitempty"`
	Validation                     Validation                         `json:"validation"`
	Warnings                       []Warning                          `json:"warnings"`
	Hashes                         ManifestHashes                     `json:"hashes"`
}
