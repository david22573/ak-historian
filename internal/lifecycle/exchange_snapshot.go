package lifecycle

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/david22573/ak-historian/internal/exchange_meta"
)

func (b *Builder) loadExchangeSnapshots(m *Manifest) error {
	if b.ExchangeSnapshotPath != "" {
		snapshot, err := exchange_meta.ReadSnapshot(b.ExchangeSnapshotPath)
		if err != nil {
			return fmt.Errorf("read exchange metadata snapshot: %w", err)
		}
		applyExchangeSnapshotSet(m, []*exchange_meta.Snapshot{snapshot}, nil, b.ExchangeSnapshotPath)
	}
	if b.ExchangeSnapshotManifestPath != "" {
		manifest, err := exchange_meta.ReadSnapshotManifest(b.ExchangeSnapshotManifestPath)
		if err != nil {
			return fmt.Errorf("read exchange metadata snapshot manifest: %w", err)
		}
		var snapshots []*exchange_meta.Snapshot
		baseDir := filepath.Dir(b.ExchangeSnapshotManifestPath)
		for _, ref := range manifest.Snapshots {
			path := ref.RelativePath
			if !filepath.IsAbs(path) {
				path = filepath.Join(baseDir, filepath.FromSlash(path))
			}
			snapshot, err := exchange_meta.ReadSnapshot(path)
			if err != nil {
				return fmt.Errorf("read exchange metadata snapshot %s: %w", ref.RelativePath, err)
			}
			snapshots = append(snapshots, snapshot)
		}
		applyExchangeSnapshotSet(m, snapshots, manifest, b.ExchangeSnapshotManifestPath)
	}
	return nil
}

func applyExchangeSnapshotSet(m *Manifest, snapshots []*exchange_meta.Snapshot, archive *exchange_meta.SnapshotManifest, sourcePath string) {
	if len(snapshots) == 0 {
		return
	}
	sort.SliceStable(snapshots, func(i, j int) bool {
		if snapshots[i].CollectedAtUTC != snapshots[j].CollectedAtUTC {
			return snapshots[i].CollectedAtUTC < snapshots[j].CollectedAtUTC
		}
		return snapshots[i].SnapshotID < snapshots[j].SnapshotID
	})

	index := map[string]int{}
	for i, sym := range m.Symbols {
		index[sym.Symbol] = i
	}

	seenBySnapshot := make([]map[string]bool, len(snapshots))
	terminalStatus := map[string]bool{}
	for snapshotIndex, snapshot := range snapshots {
		seenBySnapshot[snapshotIndex] = map[string]bool{}
		if snapshot.Validation.CurrentOnly || exchangeSnapshotHasWarning(snapshot, exchange_meta.CodeCurrentOnlySource) {
			m.Warnings = append(m.Warnings, Warning{
				Code:    CodeLifecycleExchangeSnapshotCurrentOnly,
				Target:  snapshot.SnapshotID,
				Message: "Exchange metadata snapshot is current-only and does not prove historical availability before collection",
			})
		}

		missingObservedTime := snapshot.SourceObservedTimeUTC == nil
		isUnverified := snapshot.TrustLevel == exchange_meta.TrustLevelUserProvidedUnverified || snapshot.TrustLevel == exchange_meta.TrustLevelUnknown || snapshot.TrustLevel == ""

		if missingObservedTime {
			m.Warnings = append(m.Warnings, Warning{
				Code:    CodeLifecycleSnapshotObservedTimeMissing,
				Target:  snapshot.SnapshotID,
				Message: "Exchange metadata snapshot missing observed time; cannot prove historical point-in-time existence",
			})
		}

		if isUnverified {
			m.Warnings = append(m.Warnings, Warning{
				Code:    CodeLifecycleSnapshotUnverifiedSource,
				Target:  snapshot.SnapshotID,
				Message: "Exchange metadata snapshot comes from an unverified source",
			})
			m.Warnings = append(m.Warnings, Warning{
				Code:    CodeLifecycleSnapshotTrustWeak,
				Target:  snapshot.SnapshotID,
				Message: "Trust level is too weak to support strong survivorship-free claims",
			})
		}

		m.Warnings = append(m.Warnings, Warning{
			Code:    CodeLifecycleExchangeSnapshotEvidence,
			Target:  snapshot.SnapshotID,
			Message: "Asset lifecycle evidence includes exchange metadata snapshot observation",
		})

		for _, metaSym := range snapshot.Symbols {
			seenBySnapshot[snapshotIndex][metaSym.Symbol] = true
			i, ok := index[metaSym.Symbol]
			if !ok {
				entry := SymbolEntry{
					Symbol:        metaSym.Symbol,
					BaseAsset:     normalizeDefault(metaSym.BaseAsset, inferBaseAsset(metaSym.Symbol, metaSym.QuoteAsset)),
					QuoteAsset:    normalizeDefault(metaSym.QuoteAsset, m.QuoteAsset),
					MarketType:    normalizeDefault(metaSym.MarketType, m.MarketType),
					Exchange:      normalizeDefault(metaSym.Exchange, m.Exchange),
					Status:        StatusUnknown,
					ListedAtUTC:   StatusUnknown,
					DelistedAtUTC: StatusUnknown,
					FirstSeenUTC:  StatusUnknown,
					LastSeenUTC:   StatusUnknown,
					EvidenceLevel: EvidenceUnknown,
					Sources:       []SourceEntry{},
					Warnings:      []Warning{},
				}
				m.Symbols = append(m.Symbols, entry)
				i = len(m.Symbols) - 1
				index[metaSym.Symbol] = i
			}

			entry := &m.Symbols[i]
			entry.BaseAsset = normalizeDefault(entry.BaseAsset, metaSym.BaseAsset)
			entry.QuoteAsset = normalizeDefault(entry.QuoteAsset, metaSym.QuoteAsset)
			entry.MarketType = normalizeDefault(entry.MarketType, metaSym.MarketType)
			entry.Exchange = normalizeDefault(entry.Exchange, metaSym.Exchange)
			entry.EvidenceLevel = atLeastHistoricalSnapshot(entry.EvidenceLevel)

			observedForSym := snapshot.CollectedAtUTC
			if snapshot.SourceObservedTimeUTC != nil {
				observedForSym = *snapshot.SourceObservedTimeUTC
			} else if snapshot.SourceType == "file_import_historical" {
				observedForSym = ""
			}

			if observedForSym != "" {
				entry.FirstSeenUTC = earliestTimestamp(entry.FirstSeenUTC, observedForSym)
				entry.LastSeenUTC = latestTimestamp(entry.LastSeenUTC, observedForSym)
			}

			lifecycleStatus := lifecycleStatusFromExchangeStatus(metaSym.Status)
			if lifecycleStatus == StatusDelist || lifecycleStatus == StatusExpired {
				terminalStatus[metaSym.Symbol] = true
			}
			entry.Status = mergeLifecycleStatus(entry.Status, lifecycleStatus)

			fields := []string{"first_seen_utc", "last_seen_utc", "status"}
			if metaSym.OnboardDateUTC != nil && *metaSym.OnboardDateUTC != "" && entry.ListedAtUTC == StatusUnknown {
				entry.ListedAtUTC = normalizeTimestampOrUnknown(*metaSym.OnboardDateUTC)
				fields = append(fields, "listed_at_utc")
				m.Warnings = append(m.Warnings, Warning{
					Code:    CodeLifecycleListingFromExchangeMetadata,
					Target:  entry.Symbol,
					Message: "listed_at_utc came from explicit exchange metadata onboard date field",
				})
			}
			if metaSym.DeliveryDateUTC != nil && *metaSym.DeliveryDateUTC != "" && isTerminalLifecycleStatus(entry.Status) && entry.DelistedAtUTC == StatusUnknown {
				entry.DelistedAtUTC = normalizeTimestampOrUnknown(*metaSym.DeliveryDateUTC)
				fields = append(fields, "delisted_at_utc")
				m.Warnings = append(m.Warnings, Warning{
					Code:    CodeLifecycleDelistingFromExchangeMetadata,
					Target:  entry.Symbol,
					Message: "delisted_at_utc came from explicit exchange metadata delivery date field",
				})
			}
			sort.Strings(fields)
			entry.Sources = append(entry.Sources, SourceEntry{
				SourceType:      exchangeSnapshotSourceType(archive),
				SourceName:      snapshot.SourceName,
				SourceURIOrPath: normalizeSourcePath(sourcePath),
				SourceHash:      snapshot.Hashes.SnapshotHash,
				ObservedAtUTC:   normalizeTimestampOrUnknown(observedForSym),
				EvidenceFields:  fields,
				Confidence:      "MEDIUM",
				Notes:           "Exchange metadata snapshot proves observation in this snapshot only; absence alone is not delisting proof.",
			})
			if isUnverified {
				entry.EvidenceLevel = EvidenceUserProvidedUnverified
			}
		}
	}

	if len(snapshots) > 1 {
		lastSeenSet := seenBySnapshot[len(seenBySnapshot)-1]
		for symbol, i := range index {
			if !appearedInSnapshots(symbol, seenBySnapshot) || lastSeenSet[symbol] || terminalStatus[symbol] || isTerminalLifecycleStatus(m.Symbols[i].Status) {
				continue
			}
			m.Symbols[i].Warnings = append(m.Symbols[i].Warnings, Warning{
				Code:    CodeLifecycleSymbolDisappearedNoProof,
				Target:  symbol,
				Message: "Symbol disappeared from later exchange metadata snapshot without source delisting proof",
			})
		}
	}

	applyExchangeSnapshotSummary(m, snapshots, archive)
}

func applyExchangeSnapshotSummary(m *Manifest, snapshots []*exchange_meta.Snapshot, archive *exchange_meta.SnapshotManifest) {
	m.ExchangeMetadataSnapshotEvidenceLevel = EvidenceHistoricalSnapshot
	currentOnly := false
	for _, snapshot := range snapshots {
		if snapshot.Validation.CurrentOnly || exchangeSnapshotHasWarning(snapshot, exchange_meta.CodeCurrentOnlySource) {
			currentOnly = true
		}
	}
	m.ExchangeMetadataSnapshotCurrentOnly = currentOnly
	if archive != nil {
		m.ExchangeMetadataSnapshotManifestHash = archive.Hashes.ManifestHash
		m.ExchangeMetadataSnapshotArchiveHash = archive.Hashes.ArchiveHash
		m.ExchangeMetadataSnapshotCoverageStartUTC = archive.EffectiveStartUTC
		m.ExchangeMetadataSnapshotCoverageEndUTC = archive.EffectiveEndUTC
	} else if len(snapshots) == 1 {
		m.ExchangeMetadataSnapshotHash = snapshots[0].Hashes.SnapshotHash
		m.ExchangeMetadataSnapshotCoverageStartUTC = snapshots[0].CollectedAtUTC
		m.ExchangeMetadataSnapshotCoverageEndUTC = snapshots[0].CollectedAtUTC
	}
	m.PointInTimeCoverageStatus = snapshotCoverageStatus(
		m.EffectiveStartUTC,
		m.EffectiveEndUTC,
		m.ExchangeMetadataSnapshotCoverageStartUTC,
		m.ExchangeMetadataSnapshotCoverageEndUTC,
		currentOnly,
	)
}

func exchangeSnapshotSourceType(archive *exchange_meta.SnapshotManifest) string {
	if archive != nil {
		return "exchange_snapshot_manifest"
	}
	return "exchange_snapshot"
}

func exchangeSnapshotHasWarning(snapshot *exchange_meta.Snapshot, code string) bool {
	for _, warning := range snapshot.Warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}

func atLeastHistoricalSnapshot(current string) string {
	switch current {
	case EvidenceVerifiedExchangeListing, EvidenceVerifiedExchangeDelisting, EvidenceHistoricalSnapshot:
		return current
	default:
		return EvidenceHistoricalSnapshot
	}
}

func earliestTimestamp(current, candidate string) string {
	current = normalizeTimestampOrUnknown(current)
	candidate = normalizeTimestampOrUnknown(candidate)
	if current == StatusUnknown {
		return candidate
	}
	if candidate == StatusUnknown {
		return current
	}
	if candidate < current {
		return candidate
	}
	return current
}

func latestTimestamp(current, candidate string) string {
	current = normalizeTimestampOrUnknown(current)
	candidate = normalizeTimestampOrUnknown(candidate)
	if current == StatusUnknown {
		return candidate
	}
	if candidate == StatusUnknown {
		return current
	}
	if candidate > current {
		return candidate
	}
	return current
}

func lifecycleStatusFromExchangeStatus(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case exchange_meta.StatusDelivered, exchange_meta.StatusDelisted:
		return StatusDelist
	case exchange_meta.StatusExpired:
		return StatusExpired
	case exchange_meta.StatusActive, exchange_meta.StatusTrading, exchange_meta.StatusBreak, exchange_meta.StatusHalt, exchange_meta.StatusPreTrading, exchange_meta.StatusSettling:
		return StatusActive
	default:
		return StatusUnknown
	}
}

func mergeLifecycleStatus(current, next string) string {
	current = normalizeDefault(strings.ToUpper(strings.TrimSpace(current)), StatusUnknown)
	next = normalizeDefault(strings.ToUpper(strings.TrimSpace(next)), StatusUnknown)
	if isTerminalLifecycleStatus(next) {
		return next
	}
	if current == "" || current == StatusUnknown {
		return next
	}
	return current
}

func isTerminalLifecycleStatus(status string) bool {
	return status == StatusDelist || status == StatusExpired
}

func appearedInSnapshots(symbol string, seen []map[string]bool) bool {
	for _, set := range seen {
		if set[symbol] {
			return true
		}
	}
	return false
}

func snapshotCoverageStatus(effectiveStart, effectiveEnd, coverageStart, coverageEnd string, currentOnly bool) string {
	if currentOnly {
		return "CURRENT_ONLY"
	}
	start, startOK := parseOptionalTime(effectiveStart)
	end, endOK := parseOptionalTime(effectiveEnd)
	covStart, covStartOK := parseOptionalTime(coverageStart)
	covEnd, covEndOK := parseOptionalTime(coverageEnd)
	if !startOK || !endOK || !covStartOK || !covEndOK || covStart.IsZero() || covEnd.IsZero() {
		return "UNKNOWN"
	}
	if (!start.IsZero() && start.Before(covStart)) || (!end.IsZero() && end.After(covEnd)) {
		return "PARTIAL"
	}
	return "COVERS_WINDOW"
}

func parseOptionalTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, StatusUnknown) {
		return time.Time{}, true
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}
