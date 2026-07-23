package exchange_meta

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

func ComputeSnapshotHashes(snapshot *Snapshot) Hashes {
	sortSnapshot(snapshot)

	normalizedPayload := map[string]interface{}{
		"exchange":           snapshot.Exchange,
		"market_type":        snapshot.MarketType,
		"quote_asset_filter": snapshot.QuoteAssetFilter,
		"symbols":            normalizedSymbolsForHash(snapshot.Symbols),
	}
	normalizedPayloadHash := hashStable(normalizedPayload)

	symbolNames := make([]string, 0, len(snapshot.Symbols))
	for _, sym := range snapshot.Symbols {
		symbolNames = append(symbolNames, sym.Symbol)
	}
	sort.Strings(symbolNames)
	symbolSetHash := hashStable(symbolNames)

	snapshotOnly := map[string]interface{}{
		"schema_version":            snapshot.SchemaVersion,
		"snapshot_version":          snapshot.SnapshotVersion,
		"exchange":                  snapshot.Exchange,
		"market_type":               snapshot.MarketType,
		"quote_asset_filter":        snapshot.QuoteAssetFilter,
		"source_type":               snapshot.SourceType,
		"source_name":               snapshot.SourceName,
		"source_uri":                normalizeURIForHash(snapshot.SourceURI),
		"collected_at_utc":          snapshot.CollectedAtUTC,
		"source_observed_time_utc":  snapshot.SourceObservedTimeUTC,
		"collector_git_sha":         snapshot.CollectorGitSHA,
		"normalized_payload_sha256": normalizedPayloadHash,
		"symbol_set_hash":           symbolSetHash,
		"symbols":                   normalizedSymbolsForHash(snapshot.Symbols),
		"validation":                snapshot.Validation,
		"warnings":                  snapshot.Warnings,
	}
	return Hashes{
		SnapshotHash:          hashStable(snapshotOnly),
		SymbolSetHash:         symbolSetHash,
		NormalizedPayloadHash: normalizedPayloadHash,
	}
}

func ComputeManifestHashes(manifest *SnapshotManifest) ManifestHashes {
	sortManifest(manifest)
	archiveHash := hashStable(manifest.Snapshots)
	presenceHash := hashStable(manifest.SymbolLifecycleEvidenceSummary)
	manifestOnly := map[string]interface{}{
		"schema_version":                    manifest.SchemaVersion,
		"manifest_version":                  manifest.ManifestVersion,
		"archive_id":                        manifest.ArchiveID,
		"exchange":                          manifest.Exchange,
		"market_type":                       manifest.MarketType,
		"effective_start_utc":               manifest.EffectiveStartUTC,
		"effective_end_utc":                 manifest.EffectiveEndUTC,
		"snapshot_count":                    manifest.SnapshotCount,
		"snapshots":                         manifest.Snapshots,
		"symbol_lifecycle_evidence_summary": manifest.SymbolLifecycleEvidenceSummary,
		"validation":                        manifest.Validation,
		"warnings":                          manifest.Warnings,
		"archive_hash":                      archiveHash,
		"symbol_presence_hash":              presenceHash,
	}
	return ManifestHashes{
		ArchiveHash:        archiveHash,
		ManifestHash:       hashStable(manifestOnly),
		SymbolPresenceHash: presenceHash,
	}
}

func hashStable(value interface{}) string {
	b, _ := json.Marshal(value)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func sortSnapshot(snapshot *Snapshot) {
	sort.SliceStable(snapshot.Symbols, func(i, j int) bool {
		return snapshot.Symbols[i].Symbol < snapshot.Symbols[j].Symbol
	})
	for i := range snapshot.Symbols {
		sort.Strings(snapshot.Symbols[i].Permissions)
		sort.Strings(snapshot.Symbols[i].SourceFields)
		sort.SliceStable(snapshot.Symbols[i].Warnings, func(a, b int) bool {
			if snapshot.Symbols[i].Warnings[a].Code != snapshot.Symbols[i].Warnings[b].Code {
				return snapshot.Symbols[i].Warnings[a].Code < snapshot.Symbols[i].Warnings[b].Code
			}
			return snapshot.Symbols[i].Warnings[a].Target < snapshot.Symbols[i].Warnings[b].Target
		})
	}
	sort.SliceStable(snapshot.Warnings, func(i, j int) bool {
		if snapshot.Warnings[i].Code != snapshot.Warnings[j].Code {
			return snapshot.Warnings[i].Code < snapshot.Warnings[j].Code
		}
		return snapshot.Warnings[i].Target < snapshot.Warnings[j].Target
	})
}

func sortManifest(manifest *SnapshotManifest) {
	sort.SliceStable(manifest.Snapshots, func(i, j int) bool {
		if manifest.Snapshots[i].CollectedAtUTC != manifest.Snapshots[j].CollectedAtUTC {
			return manifest.Snapshots[i].CollectedAtUTC < manifest.Snapshots[j].CollectedAtUTC
		}
		return manifest.Snapshots[i].SnapshotID < manifest.Snapshots[j].SnapshotID
	})
	for symbol, summary := range manifest.SymbolLifecycleEvidenceSummary {
		sort.Strings(summary.Statuses)
		sort.Strings(summary.SnapshotHashes)
		manifest.SymbolLifecycleEvidenceSummary[symbol] = summary
	}
	manifest.Warnings = dedupeWarnings(manifest.Warnings)
}

func normalizedSymbolsForHash(symbols []Symbol) []Symbol {
	out := make([]Symbol, len(symbols))
	copy(out, symbols)
	for i := range out {
		sort.Strings(out[i].Permissions)
		sort.Strings(out[i].SourceFields)
		out[i].Warnings = dedupeWarnings(out[i].Warnings)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Symbol < out[j].Symbol
	})
	return out
}
