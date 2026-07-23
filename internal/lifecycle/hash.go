package lifecycle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

func ComputeHashes(m *Manifest) Hashes {
	sortManifest(m)

	lifecycleOnly := map[string]interface{}{
		"exchange":                        m.Exchange,
		"market_type":                     m.MarketType,
		"quote_asset":                     m.QuoteAsset,
		"effective_start_utc":             m.EffectiveStartUTC,
		"effective_end_utc":               m.EffectiveEndUTC,
		"exchange_metadata_snapshot_hash": m.ExchangeMetadataSnapshotHash,
		"exchange_metadata_snapshot_manifest_hash":      m.ExchangeMetadataSnapshotManifestHash,
		"exchange_metadata_snapshot_archive_hash":       m.ExchangeMetadataSnapshotArchiveHash,
		"exchange_metadata_snapshot_coverage_start_utc": m.ExchangeMetadataSnapshotCoverageStartUTC,
		"exchange_metadata_snapshot_coverage_end_utc":   m.ExchangeMetadataSnapshotCoverageEndUTC,
		"exchange_metadata_snapshot_evidence_level":     m.ExchangeMetadataSnapshotEvidenceLevel,
		"exchange_metadata_snapshot_current_only":       m.ExchangeMetadataSnapshotCurrentOnly,
		"point_in_time_coverage_status":                 m.PointInTimeCoverageStatus,
		"symbols":                                       normalizedSymbolsForHash(m.Symbols),
		"warnings":                                      m.Warnings,
	}
	lBytes, _ := json.Marshal(lifecycleOnly)
	lSum := sha256.Sum256(lBytes)
	lifecycleHash := hex.EncodeToString(lSum[:])

	manifestOnly := map[string]interface{}{
		"schema_version":                  m.SchemaVersion,
		"manifest_version":                m.ManifestVersion,
		"lifecycle_id":                    m.LifecycleID,
		"lifecycle_name":                  m.LifecycleName,
		"source_repo":                     m.SourceRepo,
		"source_git_sha":                  m.SourceGitSHA,
		"source_type":                     m.SourceType,
		"exchange":                        m.Exchange,
		"market_type":                     m.MarketType,
		"quote_asset":                     m.QuoteAsset,
		"effective_start_utc":             m.EffectiveStartUTC,
		"effective_end_utc":               m.EffectiveEndUTC,
		"exchange_metadata_snapshot_hash": m.ExchangeMetadataSnapshotHash,
		"exchange_metadata_snapshot_manifest_hash":      m.ExchangeMetadataSnapshotManifestHash,
		"exchange_metadata_snapshot_archive_hash":       m.ExchangeMetadataSnapshotArchiveHash,
		"exchange_metadata_snapshot_coverage_start_utc": m.ExchangeMetadataSnapshotCoverageStartUTC,
		"exchange_metadata_snapshot_coverage_end_utc":   m.ExchangeMetadataSnapshotCoverageEndUTC,
		"exchange_metadata_snapshot_evidence_level":     m.ExchangeMetadataSnapshotEvidenceLevel,
		"exchange_metadata_snapshot_current_only":       m.ExchangeMetadataSnapshotCurrentOnly,
		"point_in_time_coverage_status":                 m.PointInTimeCoverageStatus,
		"symbols":                                       normalizedSymbolsForHash(m.Symbols),
		"validation":                                    m.Validation,
		"warnings":                                      m.Warnings,
		"lifecycle_hash":                                lifecycleHash,
	}
	mBytes, _ := json.Marshal(manifestOnly)
	mSum := sha256.Sum256(mBytes)
	return Hashes{
		LifecycleHash: lifecycleHash,
		ManifestHash:  hex.EncodeToString(mSum[:]),
	}
}

func EvidenceLevelSummary(symbols []SymbolEntry) map[string]int {
	summary := map[string]int{}
	for _, sym := range symbols {
		level := sym.EvidenceLevel
		if level == "" {
			level = EvidenceUnknown
		}
		summary[level]++
	}
	return summary
}

func ManifestSummary(m *Manifest) Summary {
	return Summary{
		LifecycleID:                              m.LifecycleID,
		LifecycleHash:                            m.Hashes.LifecycleHash,
		LifecycleManifestHash:                    m.Hashes.ManifestHash,
		LifecycleEvidenceLevelSummary:            EvidenceLevelSummary(m.Symbols),
		LifecycleSourceType:                      m.SourceType,
		LifecycleWarnings:                        warningCodes(m.Warnings),
		ListingEvidenceStatus:                    m.Validation.ListingEvidenceStatus,
		DelistingEvidenceStatus:                  m.Validation.DelistingEvidenceStatus,
		SurvivorshipSupportStatus:                m.Validation.SurvivorshipSupportStatus,
		ExchangeMetadataSnapshotHash:             m.ExchangeMetadataSnapshotHash,
		ExchangeMetadataSnapshotManifestHash:     m.ExchangeMetadataSnapshotManifestHash,
		ExchangeMetadataSnapshotArchiveHash:      m.ExchangeMetadataSnapshotArchiveHash,
		ExchangeMetadataSnapshotCoverageStartUTC: m.ExchangeMetadataSnapshotCoverageStartUTC,
		ExchangeMetadataSnapshotCoverageEndUTC:   m.ExchangeMetadataSnapshotCoverageEndUTC,
		ExchangeMetadataSnapshotEvidenceLevel:    m.ExchangeMetadataSnapshotEvidenceLevel,
		ExchangeMetadataSnapshotCurrentOnly:      m.ExchangeMetadataSnapshotCurrentOnly,
		PointInTimeCoverageStatus:                m.PointInTimeCoverageStatus,
	}
}

func sortManifest(m *Manifest) {
	sort.SliceStable(m.Symbols, func(i, j int) bool {
		return m.Symbols[i].Symbol < m.Symbols[j].Symbol
	})
	for i := range m.Symbols {
		sort.SliceStable(m.Symbols[i].Sources, func(a, b int) bool {
			if m.Symbols[i].Sources[a].SourceType != m.Symbols[i].Sources[b].SourceType {
				return m.Symbols[i].Sources[a].SourceType < m.Symbols[i].Sources[b].SourceType
			}
			if m.Symbols[i].Sources[a].SourceName != m.Symbols[i].Sources[b].SourceName {
				return m.Symbols[i].Sources[a].SourceName < m.Symbols[i].Sources[b].SourceName
			}
			return m.Symbols[i].Sources[a].SourceHash < m.Symbols[i].Sources[b].SourceHash
		})
		sort.SliceStable(m.Symbols[i].Warnings, func(a, b int) bool {
			if m.Symbols[i].Warnings[a].Code != m.Symbols[i].Warnings[b].Code {
				return m.Symbols[i].Warnings[a].Code < m.Symbols[i].Warnings[b].Code
			}
			return m.Symbols[i].Warnings[a].Target < m.Symbols[i].Warnings[b].Target
		})
	}
	sort.SliceStable(m.Warnings, func(i, j int) bool {
		if m.Warnings[i].Code != m.Warnings[j].Code {
			return m.Warnings[i].Code < m.Warnings[j].Code
		}
		return m.Warnings[i].Target < m.Warnings[j].Target
	})
}

func normalizedSymbolsForHash(symbols []SymbolEntry) []SymbolEntry {
	out := make([]SymbolEntry, len(symbols))
	copy(out, symbols)
	for i := range out {
		for j := range out[i].Sources {
			out[i].Sources[j].SourceURIOrPath = normalizeSourcePath(out[i].Sources[j].SourceURIOrPath)
		}
	}
	return out
}
