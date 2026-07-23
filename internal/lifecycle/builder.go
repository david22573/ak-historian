package lifecycle

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/david22573/ak-historian/internal/parquetutil"
)

type Builder struct {
	LifecycleID                  string
	LifecycleName                string
	SourceRepo                   string
	SourceGitSHA                 string
	SourceType                   string
	EffectiveStartUTC            string
	EffectiveEndUTC              string
	Exchange                     string
	MarketType                   string
	QuoteAsset                   string
	DataRoot                     string
	InputCSV                     string
	InputJSON                    string
	ExchangeSnapshotPath         string
	ExchangeSnapshotManifestPath string
	Strict                       bool
}

func (b *Builder) Build() (*Manifest, error) {
	sourceType := normalizeUnknown(b.SourceType)
	if sourceType == StatusUnknown {
		sourceType = "local_data"
	}
	if b.InputCSV != "" && (b.SourceType == "" || b.SourceType == "local_data") {
		sourceType = "user_csv"
	}
	if b.InputJSON != "" && (b.SourceType == "" || b.SourceType == "local_data") {
		sourceType = "user_json"
	}
	if (b.ExchangeSnapshotPath != "" || b.ExchangeSnapshotManifestPath != "") && (b.SourceType == "" || b.SourceType == "local_data") && b.DataRoot == "" && b.InputCSV == "" && b.InputJSON == "" {
		sourceType = "exchange_snapshot"
	}

	m := &Manifest{
		SchemaVersion:     "1.0.0",
		ManifestVersion:   "1.0.0",
		LifecycleID:       normalizeDefault(b.LifecycleID, "default_lifecycle"),
		LifecycleName:     normalizeDefault(b.LifecycleName, "Default Asset Lifecycle"),
		SourceRepo:        normalizeDefault(b.SourceRepo, "ak-historian"),
		SourceGitSHA:      normalizeDefault(b.SourceGitSHA, StatusUnknown),
		SourceType:        sourceType,
		GeneratedAtUTC:    time.Now().UTC().Format(time.RFC3339),
		Exchange:          normalizeDefault(b.Exchange, StatusUnknown),
		MarketType:        normalizeDefault(b.MarketType, StatusUnknown),
		QuoteAsset:        normalizeDefault(b.QuoteAsset, StatusUnknown),
		EffectiveStartUTC: normalizeTimestampOrUnknown(b.EffectiveStartUTC),
		EffectiveEndUTC:   normalizeTimestampOrUnknown(b.EffectiveEndUTC),
		Symbols:           []SymbolEntry{},
		Warnings:          []Warning{},
	}

	switch {
	case b.InputCSV != "":
		if err := b.loadCSV(m, b.InputCSV); err != nil {
			return nil, err
		}
	case b.InputJSON != "":
		if err := b.loadJSON(m, b.InputJSON); err != nil {
			return nil, err
		}
	case b.DataRoot != "":
		if err := b.loadLocalData(m, b.DataRoot); err != nil {
			return nil, err
		}
	default:
		// Validation will mark the manifest empty. Keeping this path lets callers
		// inspect a deterministic invalid artifact in non-strict mode.
	}
	if b.ExchangeSnapshotPath != "" || b.ExchangeSnapshotManifestPath != "" {
		if err := b.loadExchangeSnapshots(m); err != nil {
			return nil, err
		}
	}

	Validate(m)
	m.Hashes = ComputeHashes(m)
	if b.Strict && !m.Validation.IsValid {
		return m, fmt.Errorf("asset lifecycle manifest validation failed")
	}
	return m, nil
}

func (b *Builder) loadLocalData(m *Manifest, root string) error {
	type fileInfo struct {
		absPath  string
		relPath  string
		hash     string
		interval string
		market   string
	}

	groups := map[string][]fileInfo{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".parquet") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		symbol, interval := inferSymbolInterval(rel)
		if symbol == "" {
			return nil
		}
		fileHash, err := hashFile(path)
		if err != nil {
			return fmt.Errorf("hash local data source %s: %w", path, err)
		}
		groups[symbol] = append(groups[symbol], fileInfo{
			absPath:  path,
			relPath:  rel,
			hash:     fileHash,
			interval: interval,
			market:   inferMarketType(rel),
		})
		return nil
	})
	if err != nil {
		return err
	}

	symbols := make([]string, 0, len(groups))
	for symbol := range groups {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)

	for _, symbol := range symbols {
		files := groups[symbol]
		sort.SliceStable(files, func(i, j int) bool { return files[i].relPath < files[j].relPath })
		abs := make([]string, 0, len(files))
		relHashes := make([]string, 0, len(files))
		intervals := map[string]bool{}
		markets := map[string]bool{}
		for _, f := range files {
			abs = append(abs, f.absPath)
			relHashes = append(relHashes, f.relPath+":"+f.hash)
			if f.interval != "" {
				intervals[f.interval] = true
			}
			if f.market != "" {
				markets[f.market] = true
			}
		}

		firstSeen := StatusUnknown
		lastSeen := StatusUnknown
		evidenceLevel := EvidenceUnknown
		sourceFields := []string{"symbol"}
		confidence := "LOW"
		if times, err := parquetutil.ReadOpenTimesStrict(abs); err == nil && len(times) > 0 {
			sort.SliceStable(times, func(i, j int) bool { return times[i] < times[j] })
			firstSeen = time.UnixMilli(times[0]).UTC().Format(time.RFC3339)
			lastSeen = time.UnixMilli(times[len(times)-1]).UTC().Format(time.RFC3339)
			evidenceLevel = EvidenceLocalDataFirstSeen
			sourceFields = []string{"first_seen_utc", "last_seen_utc"}
			confidence = "MEDIUM"
		}

		entryMarket := m.MarketType
		if entryMarket == StatusUnknown {
			entryMarket = singleKeyOrUnknown(markets)
		}
		entry := SymbolEntry{
			Symbol:        symbol,
			BaseAsset:     inferBaseAsset(symbol, m.QuoteAsset),
			QuoteAsset:    m.QuoteAsset,
			MarketType:    entryMarket,
			Exchange:      m.Exchange,
			Status:        StatusUnknown,
			ListedAtUTC:   StatusUnknown,
			DelistedAtUTC: StatusUnknown,
			FirstSeenUTC:  firstSeen,
			LastSeenUTC:   lastSeen,
			EvidenceLevel: evidenceLevel,
			Sources: []SourceEntry{
				{
					SourceType:      "local_data",
					SourceName:      symbol,
					SourceURIOrPath: "symbol=" + symbol,
					SourceHash:      hashStrings(relHashes),
					ObservedAtUTC:   stableObservedAt(m),
					EvidenceFields:  sourceFields,
					Confidence:      confidence,
					Notes:           "Local data proves local observation only, not exchange listing or delisting.",
				},
			},
		}
		if m.MarketType == StatusUnknown {
			m.MarketType = entryMarket
		}
		if m.QuoteAsset == StatusUnknown {
			entry.QuoteAsset = inferQuoteAsset(symbol)
			m.QuoteAsset = entry.QuoteAsset
			entry.BaseAsset = inferBaseAsset(symbol, entry.QuoteAsset)
		}
		if len(intervals) > 1 {
			entry.Warnings = append(entry.Warnings, Warning{
				Code:    CodeLifecycleEvidenceMissing,
				Target:  symbol,
				Message: "Multiple intervals were present; lifecycle evidence is aggregated at symbol level.",
			})
		}
		m.Symbols = append(m.Symbols, entry)
	}
	return nil
}

func (b *Builder) loadCSV(m *Manifest, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.TrimLeadingSpace = true
	header, err := r.Read()
	if err != nil {
		return fmt.Errorf("read lifecycle CSV header: %w", err)
	}
	index := map[string]int{}
	for i, h := range header {
		index[strings.ToLower(strings.TrimSpace(h))] = i
	}

	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read lifecycle CSV: %w", err)
		}
		get := func(name string) string {
			i, ok := index[name]
			if !ok || i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}

		rawSourcePath := strings.TrimSpace(get("source_uri_or_path"))
		sourcePath := normalizeSourcePath(rawSourcePath)
		sourceHash := normalizeUnknown(get("source_hash"))
		if rawSourcePath != "" {
			if h, ok := hashIfFileExists(rawSourcePath); ok {
				sourceHash = h
			}
		}

		evidence := normalizeDefault(get("evidence_level"), EvidenceUserProvidedUnverified)
		if sourceHash == StatusUnknown && evidence != EvidenceUnknown {
			evidence = EvidenceUserProvidedUnverified
		}
		confidence := "LOW"
		if evidence != EvidenceUserProvidedUnverified && sourceHash != StatusUnknown {
			confidence = "MEDIUM"
		}

		symbol := strings.ToUpper(strings.TrimSpace(get("symbol")))
		quote := normalizeDefault(get("quote_asset"), m.QuoteAsset)
		if quote == StatusUnknown {
			quote = inferQuoteAsset(symbol)
		}
		entry := SymbolEntry{
			Symbol:        symbol,
			BaseAsset:     normalizeDefault(get("base_asset"), inferBaseAsset(symbol, quote)),
			QuoteAsset:    quote,
			MarketType:    normalizeDefault(get("market_type"), m.MarketType),
			Exchange:      normalizeDefault(get("exchange"), m.Exchange),
			Status:        normalizeDefault(strings.ToUpper(get("status")), StatusUnknown),
			ListedAtUTC:   normalizeTimestampOrUnknown(get("listed_at_utc")),
			DelistedAtUTC: normalizeTimestampOrUnknown(get("delisted_at_utc")),
			FirstSeenUTC:  normalizeTimestampOrUnknown(get("first_seen_utc")),
			LastSeenUTC:   normalizeTimestampOrUnknown(get("last_seen_utc")),
			EvidenceLevel: evidence,
			Sources: []SourceEntry{
				{
					SourceType:      normalizeDefault(get("source_type"), "user_csv"),
					SourceName:      normalizeDefault(get("source_name"), "user_csv"),
					SourceURIOrPath: sourcePath,
					SourceHash:      sourceHash,
					ObservedAtUTC:   stableObservedAt(m),
					EvidenceFields:  evidenceFieldsFor(entryTimestamps(get("listed_at_utc"), get("delisted_at_utc"), get("first_seen_utc"), get("last_seen_utc"))),
					Confidence:      confidence,
				},
			},
		}
		m.Symbols = append(m.Symbols, entry)
	}
	return nil
}

func (b *Builder) loadJSON(m *Manifest, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var full Manifest
	if err := json.Unmarshal(data, &full); err == nil && len(full.Symbols) > 0 {
		full.GeneratedAtUTC = m.GeneratedAtUTC
		if full.SchemaVersion == "" {
			full.SchemaVersion = m.SchemaVersion
		}
		if full.ManifestVersion == "" {
			full.ManifestVersion = m.ManifestVersion
		}
		if full.LifecycleID == "" {
			full.LifecycleID = m.LifecycleID
		}
		if full.LifecycleName == "" {
			full.LifecycleName = m.LifecycleName
		}
		if full.SourceRepo == "" {
			full.SourceRepo = m.SourceRepo
		}
		if full.SourceGitSHA == "" {
			full.SourceGitSHA = m.SourceGitSHA
		}
		if full.SourceType == "" {
			full.SourceType = m.SourceType
		}
		if full.Exchange == "" {
			full.Exchange = m.Exchange
		}
		if full.MarketType == "" {
			full.MarketType = m.MarketType
		}
		if full.QuoteAsset == "" {
			full.QuoteAsset = m.QuoteAsset
		}
		if full.EffectiveStartUTC == "" {
			full.EffectiveStartUTC = m.EffectiveStartUTC
		}
		if full.EffectiveEndUTC == "" {
			full.EffectiveEndUTC = m.EffectiveEndUTC
		}
		*m = full
		return nil
	}

	var symbols []SymbolEntry
	if err := json.Unmarshal(data, &symbols); err != nil {
		return fmt.Errorf("parse lifecycle JSON: %w", err)
	}
	for i := range symbols {
		if len(symbols[i].Sources) == 0 {
			symbols[i].Sources = []SourceEntry{
				{
					SourceType:      "user_json",
					SourceName:      filepath.Base(path),
					SourceURIOrPath: normalizeSourcePath(path),
					SourceHash:      hashStrings([]string{string(data)}),
					ObservedAtUTC:   stableObservedAt(m),
					EvidenceFields:  evidenceFieldsForSymbol(symbols[i]),
					Confidence:      "LOW",
				},
			}
		}
		if symbols[i].EvidenceLevel == "" || symbols[i].EvidenceLevel == EvidenceUnknown {
			symbols[i].EvidenceLevel = EvidenceUserProvidedUnverified
		}
	}
	m.Symbols = symbols
	return nil
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
	if symbol == "" {
		base := filepath.Base(relPath)
		if idx := strings.Index(base, "-"); idx > 0 {
			symbol = strings.ToUpper(base[:idx])
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

func inferBaseAsset(symbol, quoteAsset string) string {
	if quoteAsset != "" && quoteAsset != StatusUnknown && strings.HasSuffix(symbol, quoteAsset) && len(symbol) > len(quoteAsset) {
		return strings.TrimSuffix(symbol, quoteAsset)
	}
	return StatusUnknown
}

func inferQuoteAsset(symbol string) string {
	for _, quote := range []string{"USDT", "USDC", "USD", "BTC", "ETH"} {
		if strings.HasSuffix(symbol, quote) && len(symbol) > len(quote) {
			return quote
		}
	}
	return StatusUnknown
}

func normalizeDefault(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func normalizeUnknown(value string) string {
	return normalizeDefault(value, StatusUnknown)
}

func normalizeTimestampOrUnknown(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, StatusUnknown) {
		return StatusUnknown
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	return t.UTC().Format(time.RFC3339)
}

func normalizeSourcePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || strings.EqualFold(path, StatusUnknown) {
		return StatusUnknown
	}
	path = filepath.ToSlash(path)
	if filepath.IsAbs(path) {
		return filepath.Base(path)
	}
	return path
}

func stableObservedAt(m *Manifest) string {
	if m.EffectiveEndUTC != "" && m.EffectiveEndUTC != StatusUnknown {
		return m.EffectiveEndUTC
	}
	if m.EffectiveStartUTC != "" && m.EffectiveStartUTC != StatusUnknown {
		return m.EffectiveStartUTC
	}
	return StatusUnknown
}

func singleKeyOrUnknown(values map[string]bool) string {
	var keys []string
	for value := range values {
		if value != "" {
			keys = append(keys, value)
		}
	}
	sort.Strings(keys)
	if len(keys) == 1 {
		return keys[0]
	}
	return StatusUnknown
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

func hashIfFileExists(path string) (string, bool) {
	if path == StatusUnknown {
		return "", false
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", false
	}
	h, err := hashFile(path)
	if err != nil {
		return "", false
	}
	return h, true
}

func hashStrings(values []string) string {
	sort.Strings(values)
	h := sha256.New()
	for _, value := range values {
		_, _ = h.Write([]byte(filepath.ToSlash(value)))
		_, _ = h.Write([]byte{'\n'})
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}

type timestampValues struct {
	listed   string
	delisted string
	first    string
	last     string
}

func entryTimestamps(listed, delisted, first, last string) timestampValues {
	return timestampValues{listed: listed, delisted: delisted, first: first, last: last}
}

func evidenceFieldsFor(values timestampValues) []string {
	var out []string
	if normalizeTimestampOrUnknown(values.listed) != StatusUnknown {
		out = append(out, "listed_at_utc")
	}
	if normalizeTimestampOrUnknown(values.delisted) != StatusUnknown {
		out = append(out, "delisted_at_utc")
	}
	if normalizeTimestampOrUnknown(values.first) != StatusUnknown {
		out = append(out, "first_seen_utc")
	}
	if normalizeTimestampOrUnknown(values.last) != StatusUnknown {
		out = append(out, "last_seen_utc")
	}
	if len(out) == 0 {
		out = append(out, "symbol")
	}
	return out
}

func evidenceFieldsForSymbol(sym SymbolEntry) []string {
	return evidenceFieldsFor(timestampValues{
		listed:   sym.ListedAtUTC,
		delisted: sym.DelistedAtUTC,
		first:    sym.FirstSeenUTC,
		last:     sym.LastSeenUTC,
	})
}
