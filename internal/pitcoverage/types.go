package pitcoverage

const (
	StatusPitEligible    = "PIT_ELIGIBLE"
	StatusPitPartial     = "PIT_PARTIAL"
	StatusPitNotEligible = "PIT_NOT_ELIGIBLE"
	StatusUnknown        = "UNKNOWN"

	PromoAllowStrict     = "ALLOW_STRICT_PROMOTION"
	PromoDowngrade       = "DOWNGRADE_PROMOTION"
	PromoBlockStrict     = "BLOCK_STRICT_PROMOTION"
	PromoExploratoryOnly = "EXPLORATORY_ONLY"

	RiskLow     = "LOW"
	RiskMedium  = "MEDIUM"
	RiskHigh    = "HIGH"
	RiskUnknown = "UNKNOWN"

	SymStatusVerifiedForWindow = "VERIFIED_FOR_WINDOW"
	SymStatusPartialForWindow  = "PARTIAL_FOR_WINDOW"
	SymStatusCurrentOnly       = "CURRENT_ONLY"
	SymStatusLocalDataOnly     = "LOCAL_DATA_ONLY"
	SymStatusUnverifiedOnly    = "UNVERIFIED_ONLY"
	SymStatusMissingLifecycle  = "MISSING_LIFECYCLE"
	SymStatusUnknown           = "UNKNOWN"

	CodeLifecycleManifestMissing       = "PIT_LIFECYCLE_MANIFEST_MISSING"
	CodeUniverseManifestMissing        = "PIT_UNIVERSE_MANIFEST_MISSING"
	CodeDatasetManifestMissing         = "PIT_DATASET_MANIFEST_MISSING"
	CodeSnapshotManifestMissing        = "PIT_SNAPSHOT_MANIFEST_MISSING"
	CodeResearchWindowInvalid          = "PIT_RESEARCH_WINDOW_INVALID"
	CodeSymbolMissingLifecycle         = "PIT_SYMBOL_MISSING_LIFECYCLE"
	CodeSymbolLocalDataOnly            = "PIT_SYMBOL_LOCAL_DATA_ONLY"
	CodeSymbolCurrentOnly              = "PIT_SYMBOL_CURRENT_ONLY"
	CodeSymbolUnverifiedOnly           = "PIT_SYMBOL_UNVERIFIED_ONLY"
	CodeSymbolObservedTimeMissing      = "PIT_SYMBOL_OBSERVED_TIME_MISSING"
	CodeSymbolPartialSnapshotCoverage  = "PIT_SYMBOL_PARTIAL_SNAPSHOT_COVERAGE"
	CodeListingEvidenceMissing         = "PIT_LISTING_EVIDENCE_MISSING"
	CodeDelistingEvidenceMissing       = "PIT_DELISTING_EVIDENCE_MISSING"
	CodeSnapshotArchiveDoesNotCoverWin = "PIT_SNAPSHOT_ARCHIVE_DOES_NOT_COVER_WINDOW"
	CodeSurvivorshipLowRiskUnproven    = "PIT_SURVIVORSHIP_LOW_RISK_UNPROVEN"
	CodePromotionBlocked               = "PIT_PROMOTION_BLOCKED"
	CodePromotionDowngraded            = "PIT_PROMOTION_DOWNGRADED"
)

type Warning struct {
	Code            string `json:"code"`
	Severity        string `json:"severity"`
	Reason          string `json:"reason"`
	TargetArtifact  string `json:"target_artifact"`
	TargetSymbol    string `json:"target_symbol,omitempty"`
	BlocksPromotion bool   `json:"blocks_promotion"`
	RecommendedFix  string `json:"recommended_fix"`
}

type Window struct {
	StartUTC string `json:"start_utc"`
	EndUTC   string `json:"end_utc"`
}

type SymbolEntry struct {
	Symbol                     string         `json:"symbol"`
	LifecycleStatus            string         `json:"lifecycle_status"`
	EvidenceLevel              string         `json:"evidence_level"`
	TrustLevelSummary          map[string]int `json:"trust_level_summary"`
	ListedAtUTC                *string        `json:"listed_at_utc"`
	DelistedAtUTC              *string        `json:"delisted_at_utc"`
	FirstSeenUTC               *string        `json:"first_seen_utc"`
	LastSeenUTC                *string        `json:"last_seen_utc"`
	ActiveDuringResearchWindow bool           `json:"active_during_research_window"`
	ListingEvidenceStatus      string         `json:"listing_evidence_status"`
	DelistingEvidenceStatus    string         `json:"delisting_evidence_status"`
	SnapshotPresenceCount      int            `json:"snapshot_presence_count"`
	ObservedSnapshotCount      int            `json:"observed_snapshot_count"`
	UnverifiedSnapshotCount    int            `json:"unverified_snapshot_count"`
	MissingObservedTimeCount   int            `json:"missing_observed_time_count"`
	CoverageStartUTC           string         `json:"coverage_start_utc"`
	CoverageEndUTC             string         `json:"coverage_end_utc"`
	PointInTimeStatus          string         `json:"point_in_time_status"`
	PromotionBlockingReasons   []string       `json:"promotion_blocking_reasons"`
	Warnings                   []Warning      `json:"warnings"`
}

type Validation struct {
	IsValid bool `json:"is_valid"`
}

type Hashes struct {
	CoverageHash string `json:"coverage_hash"`
	ReportHash   string `json:"report_hash"`
}

type Report struct {
	SchemaVersion           string        `json:"schema_version"`
	ReportVersion           string        `json:"report_version"`
	CoverageReportID        string        `json:"coverage_report_id"`
	GeneratedAtUTC          string        `json:"generated_at_utc"`
	ResearchWindowStartUTC  string        `json:"research_window_start_utc"`
	ResearchWindowEndUTC    string        `json:"research_window_end_utc"`
	UniverseID              string        `json:"universe_id"`
	UniverseHash            string        `json:"universe_hash"`
	LifecycleID             string        `json:"lifecycle_id"`
	LifecycleHash           string        `json:"lifecycle_hash"`
	SnapshotArchiveHash     string        `json:"snapshot_archive_hash"`
	DatasetID               string        `json:"dataset_id"`
	DatasetHash             string        `json:"dataset_hash"`
	OverallStatus           string        `json:"overall_status"`
	PromotionRecommendation string        `json:"promotion_recommendation"`
	SurvivorshipBiasRisk    string        `json:"survivorship_bias_risk"`
	Symbols                 []SymbolEntry `json:"symbols"`
	Windows                 []Window      `json:"windows"`
	Validation              Validation    `json:"validation"`
	Warnings                []Warning     `json:"warnings"`
	Hashes                  Hashes        `json:"hashes"`
}
