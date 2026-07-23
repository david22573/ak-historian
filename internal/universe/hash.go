package universe

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// ComputeHashes computes both universe_hash and manifest_hash
func ComputeHashes(m *Manifest) Hashes {
	hashes := Hashes{}

	// Ensure symbols and warnings are sorted for determinism
	sortManifest(m)

	// Universe Hash includes only the universe definition
	universeOnly := map[string]interface{}{
		"universe_policy":                          m.UniversePolicy,
		"includes_delisted_assets":                 m.IncludesDelistedAssets,
		"quote_asset":                              m.QuoteAsset,
		"market_type":                              m.MarketType,
		"lifecycle_hash":                           m.LifecycleHash,
		"lifecycle_manifest_hash":                  m.LifecycleManifestHash,
		"listing_evidence_status":                  m.ListingEvidenceStatus,
		"delisting_evidence_status":                m.DelistingEvidenceStatus,
		"exchange_metadata_snapshot_hash":          m.ExchangeMetadataSnapshotHash,
		"exchange_metadata_snapshot_manifest_hash": m.ExchangeMetadataSnapshotManifestHash,
		"exchange_metadata_snapshot_archive_hash":  m.ExchangeMetadataSnapshotArchiveHash,
		"point_in_time_coverage_status":            m.PointInTimeCoverageStatus,
		"symbols":                                  universeHashSymbols(m.Symbols),
	}
	uBytes, _ := json.Marshal(universeOnly)
	uSum := sha256.Sum256(uBytes)
	hashes.UniverseHash = hex.EncodeToString(uSum[:])

	// Manifest Hash includes everything except generated_at_utc and hashes
	manifestOnly := map[string]interface{}{
		"schema_version":                                m.SchemaVersion,
		"manifest_version":                              m.ManifestVersion,
		"universe_id":                                   m.UniverseID,
		"universe_name":                                 m.UniverseName,
		"source_repo":                                   m.SourceRepo,
		"source_git_sha":                                m.SourceGitSha,
		"source_type":                                   m.SourceType,
		"effective_start_utc":                           m.EffectiveStartUTC,
		"effective_end_utc":                             m.EffectiveEndUTC,
		"quote_asset":                                   m.QuoteAsset,
		"market_type":                                   m.MarketType,
		"interval_granularity":                          m.IntervalGranularity,
		"universe_policy":                               m.UniversePolicy,
		"includes_delisted_assets":                      m.IncludesDelistedAssets,
		"survivorship_bias_risk":                        m.SurvivorshipBiasRisk,
		"lifecycle_id":                                  m.LifecycleID,
		"lifecycle_hash":                                m.LifecycleHash,
		"lifecycle_manifest_hash":                       m.LifecycleManifestHash,
		"lifecycle_evidence_level_summary":              m.LifecycleEvidenceLevelSummary,
		"lifecycle_source_type":                         m.LifecycleSourceType,
		"lifecycle_warnings":                            m.LifecycleWarnings,
		"listing_evidence_status":                       m.ListingEvidenceStatus,
		"delisting_evidence_status":                     m.DelistingEvidenceStatus,
		"survivorship_support_status":                   m.SurvivorshipSupportStatus,
		"exchange_metadata_snapshot_hash":               m.ExchangeMetadataSnapshotHash,
		"exchange_metadata_snapshot_manifest_hash":      m.ExchangeMetadataSnapshotManifestHash,
		"exchange_metadata_snapshot_archive_hash":       m.ExchangeMetadataSnapshotArchiveHash,
		"exchange_metadata_snapshot_coverage_start_utc": m.ExchangeMetadataSnapshotCoverageStartUTC,
		"exchange_metadata_snapshot_coverage_end_utc":   m.ExchangeMetadataSnapshotCoverageEndUTC,
		"exchange_metadata_snapshot_evidence_level":     m.ExchangeMetadataSnapshotEvidenceLevel,
		"exchange_metadata_snapshot_current_only":       m.ExchangeMetadataSnapshotCurrentOnly,
		"point_in_time_coverage_status":                 m.PointInTimeCoverageStatus,
		"symbols":                                       m.Symbols,
		"validation":                                    m.Validation,
		"warnings":                                      m.Warnings,
		"universe_hash":                                 hashes.UniverseHash, // Note: includes the universe hash
	}
	mBytes, _ := json.Marshal(manifestOnly)
	mSum := sha256.Sum256(mBytes)
	hashes.ManifestHash = hex.EncodeToString(mSum[:])

	return hashes
}

func universeHashSymbols(symbols []SymbolEntry) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(symbols))
	for _, sym := range symbols {
		out = append(out, map[string]interface{}{
			"symbol":               sym.Symbol,
			"base_asset":           sym.BaseAsset,
			"quote_asset":          sym.QuoteAsset,
			"market_type":          sym.MarketType,
			"listed_at_utc":        sym.ListedAtUTC,
			"delisted_at_utc":      sym.DelistedAtUTC,
			"active_during_window": sym.ActiveDuringWindow,
		})
	}
	return out
}

func sortManifest(m *Manifest) {
	sort.SliceStable(m.Symbols, func(i, j int) bool {
		return m.Symbols[i].Symbol < m.Symbols[j].Symbol
	})
	for i := range m.Symbols {
		sort.SliceStable(m.Symbols[i].Warnings, func(a, b int) bool {
			if m.Symbols[i].Warnings[a].Code == m.Symbols[i].Warnings[b].Code {
				return m.Symbols[i].Warnings[a].Target < m.Symbols[i].Warnings[b].Target
			}
			return m.Symbols[i].Warnings[a].Code < m.Symbols[i].Warnings[b].Code
		})
	}

	sort.SliceStable(m.Warnings, func(i, j int) bool {
		if m.Warnings[i].Code == m.Warnings[j].Code {
			return m.Warnings[i].Target < m.Warnings[j].Target
		}
		return m.Warnings[i].Code < m.Warnings[j].Code
	})
}
