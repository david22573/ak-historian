package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/david22573/ak-historian/internal/coverage"
	"github.com/david22573/ak-historian/internal/universe"
)

type Builder struct {
	DataRoot             string
	DatasetID            string
	DatasetRole          string
	SourceRepo           string
	SourceGitSHA         string
	SourceType           string
	Symbols              []string
	Intervals            []string
	UniversePolicy       string
	IncludeCoverage      bool
	CoverageMode         string
	CoverageStart        string
	CoverageEnd          string
	CoverageMinPct       float64
	UniverseManifestPath string
}

func (b *Builder) Build() (*DatasetManifest, error) {
	symbolSet := make(map[string]bool)
	for _, sym := range b.Symbols {
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if sym != "" {
			symbolSet[sym] = true
		}
	}
	intervalSet := make(map[string]bool)
	for _, interval := range b.Intervals {
		interval = strings.TrimSpace(interval)
		if interval != "" {
			intervalSet[interval] = true
		}
	}
	marketTypeSet := make(map[string]bool)

	manifest := &DatasetManifest{
		SchemaVersion:   "1.0.0",
		ManifestVersion: "1.0.0",
		DatasetID:       b.DatasetID,
		DatasetRole:     b.DatasetRole,
		SourceRepo:      b.SourceRepo,
		SourceGitSHA:    b.SourceGitSHA,
		SourceType:      b.SourceType,
		GeneratedAtUTC:  time.Now().UTC().Format(time.RFC3339),
		DataRoot:        b.DataRoot,
		Symbols:         b.Symbols,
		Intervals:       b.Intervals,
		Files:           []FileEntry{},
		Validation: Validation{
			Status:   "PASS",
			Warnings: []string{},
		},
		Survivorship: Survivorship{
			UniversePolicy:         b.UniversePolicy,
			IncludesDelistedAssets: "unknown",
			SurvivorshipBiasRisk:   "HIGH",
			WarningCode:            "DATASET_SURVIVORSHIP_BIAS_RISK",
		},
	}

	var uManifest *universe.Manifest
	if b.UniverseManifestPath != "" {
		uBytes, err := os.ReadFile(b.UniverseManifestPath)
		if err != nil {
			return nil, fmt.Errorf("read universe manifest: %w", err)
		}
		uManifest = &universe.Manifest{}
		if err := json.Unmarshal(uBytes, uManifest); err != nil {
			return nil, fmt.Errorf("parse universe manifest: %w", err)
		}
		manifest.Survivorship.UniverseID = uManifest.UniverseID
		manifest.Survivorship.UniverseHash = uManifest.Hashes.UniverseHash
		manifest.Survivorship.UniverseManifestHash = uManifest.Hashes.ManifestHash
		manifest.Survivorship.UniversePolicy = uManifest.UniversePolicy
		manifest.Survivorship.SurvivorshipBiasRisk = uManifest.SurvivorshipBiasRisk
		manifest.Survivorship.IncludesDelistedAssets = uManifest.IncludesDelistedAssets
		manifest.Survivorship.LifecycleID = uManifest.LifecycleID
		manifest.Survivorship.LifecycleHash = uManifest.LifecycleHash
		manifest.Survivorship.LifecycleManifestHash = uManifest.LifecycleManifestHash
		manifest.Survivorship.LifecycleEvidenceLevelSummary = uManifest.LifecycleEvidenceLevelSummary
		manifest.Survivorship.LifecycleWarnings = append([]string{}, uManifest.LifecycleWarnings...)
		manifest.Survivorship.ListingEvidenceStatus = uManifest.ListingEvidenceStatus
		manifest.Survivorship.DelistingEvidenceStatus = uManifest.DelistingEvidenceStatus
		manifest.Survivorship.SurvivorshipSupportStatus = uManifest.SurvivorshipSupportStatus
		manifest.Survivorship.ExchangeMetadataSnapshotHash = uManifest.ExchangeMetadataSnapshotHash
		manifest.Survivorship.ExchangeMetadataSnapshotManifestHash = uManifest.ExchangeMetadataSnapshotManifestHash
		manifest.Survivorship.ExchangeMetadataSnapshotArchiveHash = uManifest.ExchangeMetadataSnapshotArchiveHash
		manifest.Survivorship.ExchangeMetadataSnapshotCoverageStartUTC = uManifest.ExchangeMetadataSnapshotCoverageStartUTC
		manifest.Survivorship.ExchangeMetadataSnapshotCoverageEndUTC = uManifest.ExchangeMetadataSnapshotCoverageEndUTC
		manifest.Survivorship.ExchangeMetadataSnapshotEvidenceLevel = uManifest.ExchangeMetadataSnapshotEvidenceLevel
		manifest.Survivorship.ExchangeMetadataSnapshotCurrentOnly = uManifest.ExchangeMetadataSnapshotCurrentOnly
		manifest.Survivorship.PointInTimeCoverageStatus = uManifest.PointInTimeCoverageStatus
		manifest.Survivorship.WarningCode = ""
		for _, warn := range uManifest.Warnings {
			appendValidationWarning(manifest, warn.Code, false)
			manifest.Survivorship.Warnings = append(manifest.Survivorship.Warnings, warn.Code)
		}
	} else if manifest.Survivorship.UniversePolicy == "" {
		manifest.Survivorship.UniversePolicy = "EXPLICIT_SYMBOL_LIST"
	}

	// We'll walk the DataRoot and find .parquet files
	var files []FileEntry
	datasetHasher := sha256.New()
	var rowCountTotal int64
	hasCounts := false
	missingCounts := false

	err := filepath.Walk(b.DataRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".parquet") {
			return nil
		}

		relPath, err := filepath.Rel(b.DataRoot, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		fileHash, err := hashFile(path)
		if err != nil {
			return fmt.Errorf("failed to hash %s: %w", path, err)
		}

		// Since extracting row counts from parquet might be expensive or require duckdb,
		// we'll use a V1 fallback unless we easily can.
		// For now, we skip row count extraction (V1 fallback)
		missingCounts = true

		entry := FileEntry{
			RelativePath: relPath,
			SHA256:       fileHash,
		}

		entry.Symbol, entry.Interval = inferSymbolInterval(relPath)
		if entry.Symbol != "" {
			symbolSet[entry.Symbol] = true
		}
		if entry.Interval != "" {
			intervalSet[entry.Interval] = true
		}
		if marketType := inferMarketType(relPath); marketType != "" {
			marketTypeSet[marketType] = true
		}

		files = append(files, entry)
		return nil
	})
	if err != nil {
		return nil, err
	}

	if b.IncludeCoverage {
		var startT, endT time.Time
		if b.CoverageStart != "" {
			startT, _ = time.Parse(time.RFC3339, b.CoverageStart)
		}
		if b.CoverageEnd != "" {
			endT, _ = time.Parse(time.RFC3339, b.CoverageEnd)
		}

		absFiles := make([]string, 0, len(files))
		for _, f := range files {
			absFiles = append(absFiles, filepath.Join(b.DataRoot, f.RelativePath))
		}

		opts := coverage.AuditorOptions{
			Mode:   b.CoverageMode,
			Start:  startT,
			End:    endT,
			MinPct: b.CoverageMinPct,
		}
		covRes, err := coverage.AuditFiles(absFiles, opts)
		if err == nil {
			manifest.Coverage = covRes
			rowCountTotal = covRes.TotalRows
			hasCounts = true
			missingCounts = false

			// Map file level info if possible
			for _, symCov := range covRes.Symbols {
				if symCov.Symbol != "" {
					symbolSet[symCov.Symbol] = true
				}
				if symCov.Interval != "" {
					intervalSet[symCov.Interval] = true
				}
				updateManifestRange(manifest, symCov.MinTimestampUTC, symCov.MaxTimestampUTC)
				for i := range files {
					if files[i].Symbol == symCov.Symbol && files[i].Interval == symCov.Interval {
						// we could map row counts per file but V1 auditor computes it per symbol.
					}
				}
				manifest.Validation.Warnings = append(manifest.Validation.Warnings, symCov.Warnings...)
			}
			if covRes.Status != coverage.StatusPass {
				manifest.Validation.Status = covRes.Status
			}
		} else {
			missingCounts = true
		}
	}

	if missingCounts {
		appendValidationWarning(manifest, "DATASET_ROW_COUNTS_UNAVAILABLE_V1", false)
	}

	manifest.Symbols = sortedKeys(symbolSet)
	manifest.Intervals = sortedKeys(intervalSet)

	if uManifest != nil {
		uSyms := make(map[string]bool)
		for _, us := range uManifest.Symbols {
			uSyms[us.Symbol] = true
		}

		for _, ds := range manifest.Symbols {
			if !uSyms[ds] {
				appendValidationWarning(manifest, "DATASET_SYMBOL_NOT_IN_UNIVERSE", true)
			}
			if uManifest.QuoteAsset != "" && !strings.HasSuffix(ds, uManifest.QuoteAsset) {
				appendValidationWarning(manifest, "UNIVERSE_DATASET_QUOTE_ASSET_MISMATCH", true)
			}
		}
		for _, us := range uManifest.Symbols {
			if !symbolSet[us.Symbol] {
				appendValidationWarning(manifest, "UNIVERSE_SYMBOL_MISSING_DATA", false)
			}
		}

		if uManifest.MarketType != "" && len(marketTypeSet) > 0 && !marketTypeSet[uManifest.MarketType] {
			appendValidationWarning(manifest, "UNIVERSE_DATASET_MARKET_TYPE_MISMATCH", true)
		}
		if rangeOutsideUniverse(manifest.MinTimestampUTC, manifest.MaxTimestampUTC, uManifest.EffectiveStartUTC, uManifest.EffectiveEndUTC) {
			appendValidationWarning(manifest, "DATASET_RANGE_OUTSIDE_UNIVERSE_WINDOW", true)
		}
	}

	// Sort files by relative path
	sort.Slice(files, func(i, j int) bool {
		return files[i].RelativePath < files[j].RelativePath
	})

	for _, f := range files {
		datasetHasher.Write([]byte(f.RelativePath + ":" + f.SHA256 + "\n"))
	}
	manifest.Hashes.DatasetHash = hex.EncodeToString(datasetHasher.Sum(nil))

	manifest.Files = files

	if hasCounts {
		manifest.RowCountTotal = &rowCountTotal
	}

	sort.Strings(manifest.Validation.Warnings)
	manifest.Validation.Warnings = dedupeStrings(manifest.Validation.Warnings)
	sort.Strings(manifest.Survivorship.Warnings)
	manifest.Survivorship.Warnings = dedupeStrings(manifest.Survivorship.Warnings)
	sort.Strings(manifest.Survivorship.LifecycleWarnings)
	manifest.Survivorship.LifecycleWarnings = dedupeStrings(manifest.Survivorship.LifecycleWarnings)

	manifestHash, err := manifest.ComputeHash()
	if err != nil {
		return nil, err
	}
	manifest.Hashes.ManifestHash = manifestHash

	return manifest, nil
}

func inferSymbolInterval(relPath string) (string, string) {
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	var symbol, interval string
	for i, part := range parts {
		if strings.HasPrefix(part, "symbol=") {
			symbol = strings.ToUpper(strings.TrimPrefix(part, "symbol="))
		}
		if strings.HasPrefix(part, "interval=") {
			interval = strings.TrimPrefix(part, "interval=")
		}
		if interval == "" && i > 0 && isIntervalToken(part) {
			interval = part
		}
	}
	return symbol, interval
}

func inferMarketType(relPath string) string {
	p := strings.ToLower(filepath.ToSlash(relPath))
	switch {
	case strings.Contains(p, "futures"):
		return "futures"
	case strings.Contains(p, "spot"):
		return "spot"
	default:
		return ""
	}
}

func isIntervalToken(value string) bool {
	if len(value) < 2 {
		return false
	}
	unit := value[len(value)-1]
	if unit != 'm' && unit != 'h' && unit != 'd' && unit != 'w' {
		return false
	}
	for _, r := range value[:len(value)-1] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func sortedKeys(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func appendValidationWarning(manifest *DatasetManifest, code string, fail bool) {
	manifest.Validation.Warnings = append(manifest.Validation.Warnings, code)
	if fail {
		manifest.Validation.Status = "FAIL"
	} else if manifest.Validation.Status == "PASS" {
		manifest.Validation.Status = "WARN"
	}
}

func updateManifestRange(manifest *DatasetManifest, minTime, maxTime time.Time) {
	if !minTime.IsZero() {
		minValue := minTime.UTC().Format(time.RFC3339)
		if manifest.MinTimestampUTC == "" || minValue < manifest.MinTimestampUTC {
			manifest.MinTimestampUTC = minValue
		}
	}
	if !maxTime.IsZero() {
		maxValue := maxTime.UTC().Format(time.RFC3339)
		if manifest.MaxTimestampUTC == "" || maxValue > manifest.MaxTimestampUTC {
			manifest.MaxTimestampUTC = maxValue
		}
	}
}

func rangeOutsideUniverse(datasetStart, datasetEnd, universeStart, universeEnd string) bool {
	ds, dsOK := parseRFC3339(datasetStart)
	de, deOK := parseRFC3339(datasetEnd)
	us, usOK := parseRFC3339(universeStart)
	ue, ueOK := parseRFC3339(universeEnd)
	if dsOK && usOK && !ds.IsZero() && !us.IsZero() && ds.Before(us) {
		return true
	}
	if deOK && ueOK && !de.IsZero() && !ue.IsZero() && de.After(ue) {
		return true
	}
	return false
}

func parseRFC3339(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	out := values[:0]
	var prev string
	for i, value := range values {
		if i == 0 || value != prev {
			out = append(out, value)
			prev = value
		}
	}
	return out
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
