package universe

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/david22573/ak-historian/internal/lifecycle"
)

type Builder struct {
	UniverseID                 string
	UniverseName               string
	SourceRepo                 string
	SourceGitSha               string
	SourceType                 string
	EffectiveStartUTC          string
	EffectiveEndUTC            string
	QuoteAsset                 string
	MarketType                 string
	IntervalGranularity        string
	UniversePolicy             string
	IncludesDelistedAssets     string
	SurvivorshipBiasRisk       string
	Symbols                    []string
	DataRoot                   string
	AssetLifecycleManifestPath string
}

func (b *Builder) Build() (*Manifest, error) {
	effectiveStart := normalizeTimestamp(b.EffectiveStartUTC)
	effectiveEnd := normalizeTimestamp(b.EffectiveEndUTC)
	m := &Manifest{
		SchemaVersion:          "1.0.0",
		ManifestVersion:        "1.0.0",
		UniverseID:             b.UniverseID,
		UniverseName:           b.UniverseName,
		SourceRepo:             b.SourceRepo,
		SourceGitSha:           b.SourceGitSha,
		SourceType:             b.SourceType,
		GeneratedAtUTC:         time.Now().UTC().Format(time.RFC3339),
		EffectiveStartUTC:      effectiveStart,
		EffectiveEndUTC:        effectiveEnd,
		QuoteAsset:             b.QuoteAsset,
		MarketType:             b.MarketType,
		IntervalGranularity:    b.IntervalGranularity,
		UniversePolicy:         b.UniversePolicy,
		IncludesDelistedAssets: b.IncludesDelistedAssets,
	}

	m.SurvivorshipBiasRisk = b.SurvivorshipBiasRisk
	if m.SurvivorshipBiasRisk == "" {
		m.SurvivorshipBiasRisk = defaultRiskForPolicy(b.UniversePolicy)
	}

	var lifecycleManifest *lifecycle.Manifest
	lifecycleBySymbol := map[string]lifecycle.SymbolEntry{}
	if b.AssetLifecycleManifestPath != "" {
		loaded, err := loadLifecycleManifest(b.AssetLifecycleManifestPath)
		if err != nil {
			return nil, err
		}
		lifecycleManifest = loaded
		summary := lifecycle.ManifestSummary(loaded)
		m.LifecycleID = summary.LifecycleID
		m.LifecycleHash = summary.LifecycleHash
		m.LifecycleManifestHash = summary.LifecycleManifestHash
		m.LifecycleEvidenceLevelSummary = summary.LifecycleEvidenceLevelSummary
		m.LifecycleSourceType = summary.LifecycleSourceType
		m.LifecycleWarnings = summary.LifecycleWarnings
		m.ListingEvidenceStatus = summary.ListingEvidenceStatus
		m.DelistingEvidenceStatus = summary.DelistingEvidenceStatus
		m.SurvivorshipSupportStatus = summary.SurvivorshipSupportStatus
		m.ExchangeMetadataSnapshotHash = summary.ExchangeMetadataSnapshotHash
		m.ExchangeMetadataSnapshotManifestHash = summary.ExchangeMetadataSnapshotManifestHash
		m.ExchangeMetadataSnapshotArchiveHash = summary.ExchangeMetadataSnapshotArchiveHash
		m.ExchangeMetadataSnapshotCoverageStartUTC = summary.ExchangeMetadataSnapshotCoverageStartUTC
		m.ExchangeMetadataSnapshotCoverageEndUTC = summary.ExchangeMetadataSnapshotCoverageEndUTC
		m.ExchangeMetadataSnapshotEvidenceLevel = summary.ExchangeMetadataSnapshotEvidenceLevel
		m.ExchangeMetadataSnapshotCurrentOnly = summary.ExchangeMetadataSnapshotCurrentOnly
		m.PointInTimeCoverageStatus = summary.PointInTimeCoverageStatus
		if lifecycleWindowMismatch(m.EffectiveStartUTC, m.EffectiveEndUTC, loaded.EffectiveStartUTC, loaded.EffectiveEndUTC) {
			m.Warnings = append(m.Warnings, Warning{
				Code:    CodeUniverseLifecycleWindowMismatch,
				Message: "Universe effective window is outside asset lifecycle evidence window",
			})
		}
		for _, sym := range loaded.Symbols {
			lifecycleBySymbol[sym.Symbol] = sym
		}
	}

	// Determine symbols
	var symbols []string
	if len(b.Symbols) == 0 && lifecycleManifest != nil {
		for _, sym := range lifecycleManifest.Symbols {
			symbols = append(symbols, sym.Symbol)
		}
	} else if b.UniversePolicy == PolicyExplicitSymbolList {
		symbols = b.Symbols
	} else if b.UniversePolicy == PolicyLocalDataDiscoveredSymbols {
		discovered, err := DiscoverSymbols(b.DataRoot)
		if err != nil {
			return nil, err
		}
		symbols = discovered
	} else {
		// For others, just use what's provided for now
		symbols = b.Symbols
	}

	for _, raw := range symbols {
		s := strings.ToUpper(strings.TrimSpace(raw))
		if s == "" {
			continue
		}

		entry := SymbolEntry{
			Symbol:             s,
			BaseAsset:          inferBaseAsset(s, b.QuoteAsset),
			QuoteAsset:         b.QuoteAsset,
			MarketType:         b.MarketType,
			FirstSeenUTC:       stringPtrOrNil(effectiveStart),
			ActiveDuringWindow: true,
			Source:             m.SourceType,
			Confidence:         "high",
		}
		if lifecycleSymbol, ok := lifecycleBySymbol[s]; ok {
			entry.FirstSeenUTC = lifecycleStringPtr(lifecycleSymbol.FirstSeenUTC)
			entry.ListedAtUTC = lifecycleStringPtr(lifecycleSymbol.ListedAtUTC)
			entry.DelistedAtUTC = lifecycleStringPtr(lifecycleSymbol.DelistedAtUTC)
			entry.AvailableFromData = lifecycleStringPtr(lifecycleSymbol.FirstSeenUTC)
			entry.AvailableUntilData = lifecycleStringPtr(lifecycleSymbol.LastSeenUTC)
			entry.Source = "asset_lifecycle_manifest"
			entry.Confidence = lifecycleSymbol.EvidenceLevel
			entry.ActiveDuringWindow = activeDuringWindow(lifecycleSymbol, effectiveStart, effectiveEnd)
		}

		m.Symbols = append(m.Symbols, entry)
	}

	Validate(m)
	m.Hashes = ComputeHashes(m)

	return m, nil
}

func loadLifecycleManifest(path string) (*lifecycle.Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read asset lifecycle manifest: %w", err)
	}
	var m lifecycle.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse asset lifecycle manifest: %w", err)
	}
	return &m, nil
}

func DiscoverSymbols(root string) ([]string, error) {
	if root == "" {
		return nil, nil
	}

	symbolSet := make(map[string]bool)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, "symbol=") {
				sym := strings.TrimPrefix(name, "symbol=")
				symbolSet[sym] = true
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	var symbols []string
	for s := range symbolSet {
		symbols = append(symbols, s)
	}
	sort.Strings(symbols)
	return symbols, nil
}

func defaultRiskForPolicy(policy string) string {
	switch policy {
	case PolicyExplicitSymbolList, PolicyCurrentActiveSymbolList:
		return RiskHigh
	case PolicyLocalDataDiscoveredSymbols:
		return RiskMedium
	case PolicyPointInTimeExchangeUniverse, PolicyPointInTimeVolumeFiltered, PolicyPointInTimeMarketCapFiltered:
		return RiskUnknown
	default:
		return RiskUnknown
	}
}

func inferBaseAsset(symbol, quoteAsset string) string {
	if quoteAsset != "" && strings.HasSuffix(symbol, quoteAsset) && len(symbol) > len(quoteAsset) {
		return strings.TrimSuffix(symbol, quoteAsset)
	}
	return ""
}

func normalizeTimestamp(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	return t.UTC().Format(time.RFC3339)
}

func stringPtrOrNil(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func lifecycleStringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, lifecycle.StatusUnknown) {
		return nil
	}
	return &value
}

func activeDuringWindow(sym lifecycle.SymbolEntry, startValue, endValue string) bool {
	start, startOK := parseOptionalTime(startValue)
	end, endOK := parseOptionalTime(endValue)
	if !startOK || !endOK {
		return true
	}
	listed, listedOK := parseOptionalTime(sym.ListedAtUTC)
	delisted, delistedOK := parseOptionalTime(sym.DelistedAtUTC)
	if listedOK && !listed.IsZero() && !end.IsZero() && listed.After(end) {
		return false
	}
	if delistedOK && !delisted.IsZero() && !start.IsZero() && delisted.Before(start) {
		return false
	}
	return true
}

func lifecycleWindowMismatch(universeStartValue, universeEndValue, lifecycleStartValue, lifecycleEndValue string) bool {
	universeStart, universeStartOK := parseOptionalTime(universeStartValue)
	universeEnd, universeEndOK := parseOptionalTime(universeEndValue)
	lifecycleStart, lifecycleStartOK := parseOptionalTime(lifecycleStartValue)
	lifecycleEnd, lifecycleEndOK := parseOptionalTime(lifecycleEndValue)
	if universeStartOK && lifecycleStartOK && !universeStart.IsZero() && !lifecycleStart.IsZero() && universeStart.Before(lifecycleStart) {
		return true
	}
	if universeEndOK && lifecycleEndOK && !universeEnd.IsZero() && !lifecycleEnd.IsZero() && universeEnd.After(lifecycleEnd) {
		return true
	}
	return false
}
