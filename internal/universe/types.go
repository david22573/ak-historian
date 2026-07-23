package universe

const (
	PolicyPointInTimeExchangeUniverse  = "POINT_IN_TIME_EXCHANGE_UNIVERSE"
	PolicyPointInTimeVolumeFiltered    = "POINT_IN_TIME_VOLUME_FILTERED_UNIVERSE"
	PolicyPointInTimeMarketCapFiltered = "POINT_IN_TIME_MARKET_CAP_FILTERED_UNIVERSE"
	PolicyExplicitSymbolList           = "EXPLICIT_SYMBOL_LIST"
	PolicyCurrentActiveSymbolList      = "CURRENT_ACTIVE_SYMBOL_LIST"
	PolicyLocalDataDiscoveredSymbols   = "LOCAL_DATA_DISCOVERED_SYMBOLS"
	PolicyUnknown                      = "UNKNOWN"

	RiskLow     = "LOW"
	RiskMedium  = "MEDIUM"
	RiskHigh    = "HIGH"
	RiskUnknown = "UNKNOWN"

	CodeUniverseEmpty                              = "UNIVERSE_EMPTY"
	CodeUniverseDuplicateSymbol                    = "UNIVERSE_DUPLICATE_SYMBOL"
	CodeUniverseSymbolMalformed                    = "UNIVERSE_SYMBOL_MALFORMED"
	CodeUniversePolicyUnknown                      = "UNIVERSE_POLICY_UNKNOWN"
	CodeUniverseExplicitSymbolListSurvivorshipRisk = "UNIVERSE_EXPLICIT_SYMBOL_LIST_SURVIVORSHIP_RISK"
	CodeUniverseCurrentActiveSurvivorshipRisk      = "UNIVERSE_CURRENT_ACTIVE_SURVIVORSHIP_RISK"
	CodeUniverseLocalDataDiscoveryNotPointInTime   = "UNIVERSE_LOCAL_DATA_DISCOVERY_NOT_POINT_IN_TIME"
	CodeUniverseDelistedStatusUnknown              = "UNIVERSE_DELISTED_STATUS_UNKNOWN"
	CodeUniverseLowRiskUnproven                    = "UNIVERSE_LOW_RISK_UNPROVEN"
	CodeUniverseEffectiveWindowInvalid             = "UNIVERSE_EFFECTIVE_WINDOW_INVALID"
	CodeUniverseLifecycleManifestMissing           = "UNIVERSE_LIFECYCLE_MANIFEST_MISSING"
	CodeUniverseSymbolMissingLifecycle             = "UNIVERSE_SYMBOL_MISSING_LIFECYCLE"
	CodeUniverseSymbolNotActiveDuringWindow        = "UNIVERSE_SYMBOL_NOT_ACTIVE_DURING_WINDOW"
	CodeUniverseLifecycleWindowMismatch            = "UNIVERSE_LIFECYCLE_WINDOW_MISMATCH"
	CodeUniverseLifecycleEvidenceWeak              = "UNIVERSE_LIFECYCLE_EVIDENCE_WEAK"
	CodeUniverseDelistingEvidenceMissing           = "UNIVERSE_DELISTING_EVIDENCE_MISSING"
	CodeUniverseListingEvidenceMissing             = "UNIVERSE_LISTING_EVIDENCE_MISSING"
	CodeUniverseSnapshotArchiveDoesNotCoverWindow  = "UNIVERSE_SNAPSHOT_ARCHIVE_DOES_NOT_COVER_WINDOW"
	CodeUniverseSnapshotArchiveCurrentOnly         = "UNIVERSE_SNAPSHOT_ARCHIVE_CURRENT_ONLY"
	CodeUniverseDelistingNotProvenBySnapshots      = "UNIVERSE_DELISTING_NOT_PROVEN_BY_SNAPSHOTS"
	CodeUniverseListingNotProvenBySnapshots        = "UNIVERSE_LISTING_NOT_PROVEN_BY_SNAPSHOTS"
	CodeUniversePointInTimeEvidencePartial         = "UNIVERSE_POINT_IN_TIME_EVIDENCE_PARTIAL"
)

type Validation struct {
	IsValid bool `json:"is_valid"`
}

type Warning struct {
	Code    string `json:"code"`
	Target  string `json:"target,omitempty"`
	Message string `json:"message"`
}

type SymbolEntry struct {
	Symbol             string    `json:"symbol"`
	BaseAsset          string    `json:"base_asset"`
	QuoteAsset         string    `json:"quote_asset"`
	MarketType         string    `json:"market_type"`
	FirstSeenUTC       *string   `json:"first_seen_utc"`
	ListedAtUTC        *string   `json:"listed_at_utc"`
	DelistedAtUTC      *string   `json:"delisted_at_utc"`
	ActiveDuringWindow bool      `json:"active_during_window"`
	AvailableFromData  *string   `json:"available_from_data"`
	AvailableUntilData *string   `json:"available_until_data"`
	Source             string    `json:"source,omitempty"`
	Confidence         string    `json:"confidence,omitempty"`
	Warnings           []Warning `json:"warnings,omitempty"`
}

type Hashes struct {
	UniverseHash string `json:"universe_hash"`
	ManifestHash string `json:"manifest_hash"`
}

type Manifest struct {
	SchemaVersion                            string         `json:"schema_version"`
	ManifestVersion                          string         `json:"manifest_version"`
	UniverseID                               string         `json:"universe_id"`
	UniverseName                             string         `json:"universe_name"`
	SourceRepo                               string         `json:"source_repo"`
	SourceGitSha                             string         `json:"source_git_sha"`
	SourceType                               string         `json:"source_type"`
	GeneratedAtUTC                           string         `json:"generated_at_utc"`
	EffectiveStartUTC                        string         `json:"effective_start_utc"`
	EffectiveEndUTC                          string         `json:"effective_end_utc"`
	QuoteAsset                               string         `json:"quote_asset"`
	MarketType                               string         `json:"market_type"`
	IntervalGranularity                      string         `json:"interval_granularity,omitempty"`
	UniversePolicy                           string         `json:"universe_policy"`
	IncludesDelistedAssets                   string         `json:"includes_delisted_assets"`
	SurvivorshipBiasRisk                     string         `json:"survivorship_bias_risk"`
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
	PointInTimeCoverageHash                  string         `json:"point_in_time_coverage_hash,omitempty"`
	PointInTimePromotionRecommendation       string         `json:"point_in_time_promotion_recommendation,omitempty"`
	Symbols                                  []SymbolEntry  `json:"symbols"`
	Validation                               Validation     `json:"validation"`
	Warnings                                 []Warning      `json:"warnings"`
	Hashes                                   Hashes         `json:"hashes"`
}
