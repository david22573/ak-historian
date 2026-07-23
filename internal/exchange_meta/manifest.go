package exchange_meta

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ManifestOptions struct {
	SnapshotDir string
	BaseDir     string
	ArchiveID   string
	Exchange    string
	MarketType  string
}

func BuildSnapshotManifest(opts ManifestOptions) (*SnapshotManifest, error) {
	if opts.SnapshotDir == "" {
		return nil, fmt.Errorf("missing snapshot dir")
	}

	manifest := &SnapshotManifest{
		SchemaVersion:                  SchemaVersion,
		ManifestVersion:                ManifestVersion,
		ArchiveID:                      normalizeDefault(opts.ArchiveID, "default_exchange_metadata_archive"),
		Exchange:                       normalizeDefault(strings.ToLower(opts.Exchange), StatusUnknown),
		MarketType:                     normalizeDefault(strings.ToLower(opts.MarketType), StatusUnknown),
		Snapshots:                      []ManifestSnapshotRef{},
		SymbolLifecycleEvidenceSummary: map[string]SymbolLifecycleEvidence{},
		TrustLevelSummary:              map[string]int{},
		Warnings:                       []Warning{},
		ProvenanceWarnings:             []Warning{},
	}

	var snapshots []snapshotWithPath
	err := filepath.WalkDir(opts.SnapshotDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".json") {
			return nil
		}
		snapshot, err := ReadSnapshot(path)
		if err != nil {
			return nil
		}
		if snapshot.SchemaVersion == "" || snapshot.SnapshotVersion == "" || len(snapshot.Symbols) == 0 {
			return nil
		}
		relBase := opts.BaseDir
		if relBase == "" {
			relBase = opts.SnapshotDir
		}
		rel, err := filepath.Rel(relBase, path)
		if err != nil {
			return err
		}
		snapshots = append(snapshots, snapshotWithPath{snapshot: snapshot, relativePath: filepath.ToSlash(rel)})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.SliceStable(snapshots, func(i, j int) bool {
		if snapshots[i].snapshot.CollectedAtUTC != snapshots[j].snapshot.CollectedAtUTC {
			return snapshots[i].snapshot.CollectedAtUTC < snapshots[j].snapshot.CollectedAtUTC
		}
		return snapshots[i].snapshot.SnapshotID < snapshots[j].snapshot.SnapshotID
	})

	for _, item := range snapshots {
		snapshot := item.snapshot
		if manifest.Exchange == StatusUnknown {
			manifest.Exchange = snapshot.Exchange
		}
		if manifest.MarketType == StatusUnknown {
			manifest.MarketType = snapshot.MarketType
		}
		if manifest.EffectiveStartUTC == "" || snapshot.CollectedAtUTC < manifest.EffectiveStartUTC {
			manifest.EffectiveStartUTC = snapshot.CollectedAtUTC
		}
		if manifest.EffectiveEndUTC == "" || snapshot.CollectedAtUTC > manifest.EffectiveEndUTC {
			manifest.EffectiveEndUTC = snapshot.CollectedAtUTC
		}

		manifest.TrustLevelSummary[snapshot.TrustLevel]++
		manifest.SourceCount++

		if snapshot.TrustLevel == TrustLevelOfficialArchive || snapshot.TrustLevel == TrustLevelExchangeRawResponseArchive {
			manifest.OfficialSourceCount++
		}
		if snapshot.TrustLevel == TrustLevelUserProvidedUnverified || snapshot.TrustLevel == TrustLevelUnknown {
			manifest.UnverifiedSourceCount++
		}

		if snapshot.SourceObservedTimeUTC == nil {
			manifest.ObservedTimeMissingCount++
		} else {
			if manifest.EarliestObservedTimeUTC == "" || *snapshot.SourceObservedTimeUTC < manifest.EarliestObservedTimeUTC {
				manifest.EarliestObservedTimeUTC = *snapshot.SourceObservedTimeUTC
			}
			if manifest.LatestObservedTimeUTC == "" || *snapshot.SourceObservedTimeUTC > manifest.LatestObservedTimeUTC {
				manifest.LatestObservedTimeUTC = *snapshot.SourceObservedTimeUTC
			}
		}

		manifest.Snapshots = append(manifest.Snapshots, ManifestSnapshotRef{
			SnapshotID:     snapshot.SnapshotID,
			CollectedAtUTC: snapshot.CollectedAtUTC,
			SourceName:     snapshot.SourceName,
			SnapshotHash:   snapshot.Hashes.SnapshotHash,
			TrustLevel:     snapshot.TrustLevel,
			SymbolCount:    len(snapshot.Symbols),
			RelativePath:   item.relativePath,
		})
		for _, warning := range snapshot.Warnings {
			if warning.Code == CodeCurrentOnlySource {
				manifest.Warnings = append(manifest.Warnings, Warning{
					Code:    CodeCurrentOnlySource,
					Target:  snapshot.SnapshotID,
					Message: "Archive contains a current-only exchange metadata snapshot",
				})
			}
		}
		for _, sym := range snapshot.Symbols {
			summary := manifest.SymbolLifecycleEvidenceSummary[sym.Symbol]
			if summary.FirstSeenUTC == "" || snapshot.CollectedAtUTC < summary.FirstSeenUTC {
				summary.FirstSeenUTC = snapshot.CollectedAtUTC
			}
			if summary.LastSeenUTC == "" || snapshot.CollectedAtUTC > summary.LastSeenUTC {
				summary.LastSeenUTC = snapshot.CollectedAtUTC
			}
			summary.SeenInSnapshots++
			summary.Statuses = appendUnique(summary.Statuses, sym.Status)
			if sym.OnboardDateUTC != nil {
				summary.HasOnboardDate = true
			}
			if sym.DeliveryDateUTC != nil {
				summary.HasDeliveryDate = true
			}
			summary.SnapshotHashes = appendUnique(summary.SnapshotHashes, snapshot.Hashes.SnapshotHash)
			manifest.SymbolLifecycleEvidenceSummary[sym.Symbol] = summary
		}
	}
	manifest.SnapshotCount = len(manifest.Snapshots)
	if manifest.EffectiveStartUTC == "" {
		manifest.EffectiveStartUTC = StatusUnknown
	}
	if manifest.EffectiveEndUTC == "" {
		manifest.EffectiveEndUTC = StatusUnknown
	}

	if manifest.UnverifiedSourceCount > 0 {
		manifest.ProvenanceWarnings = append(manifest.ProvenanceWarnings, Warning{
			Code:    CodeBackfillUserProvidedUnverified,
			Message: "Archive contains unverified backfill evidence",
		})
	}
	if manifest.ObservedTimeMissingCount > 0 {
		manifest.ProvenanceWarnings = append(manifest.ProvenanceWarnings, Warning{
			Code:    CodeBackfillObservedTimeMissing,
			Message: "Archive contains snapshots with missing observed time",
		})
	}

	ValidateManifest(manifest)
	manifest.Hashes = ComputeManifestHashes(manifest)
	return manifest, nil
}

type snapshotWithPath struct {
	snapshot     *Snapshot
	relativePath string
}

func appendUnique(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
