package exchange_meta

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type SnapshotOptions struct {
	Exchange              string
	MarketType            string
	QuoteAssetFilter      string
	SourceType            string
	SourceName            string
	SourceURI             string
	CollectedAtUTC        string
	SourceObservedTimeUTC string
	TrustLevel            string
	CollectorGitSHA       string
	RawPayload            []byte
}

func BuildSnapshot(opts SnapshotOptions) (*Snapshot, error) {
	exchange := normalizeDefault(strings.ToLower(opts.Exchange), "binance")
	marketType := normalizeDefault(strings.ToLower(opts.MarketType), "futures_um")
	quoteFilter := strings.ToUpper(strings.TrimSpace(opts.QuoteAssetFilter))
	collected := normalizeTimestampOrNow(opts.CollectedAtUTC)
	sourceObserved := normalizeNullableTimestamp(opts.SourceObservedTimeUTC)

	rawHash := StatusUnknown
	if len(opts.RawPayload) > 0 {
		sum := sha256.Sum256(opts.RawPayload)
		rawHash = hex.EncodeToString(sum[:])
	}

	snapshot := &Snapshot{
		SchemaVersion:           SchemaVersion,
		SnapshotVersion:         SnapshotVersion,
		Exchange:                exchange,
		MarketType:              marketType,
		QuoteAssetFilter:        quoteFilter,
		SourceType:              normalizeDefault(opts.SourceType, "file_import_current"),
		SourceName:              normalizeDefault(opts.SourceName, "exchange_metadata"),
		SourceURI:               normalizeDefault(opts.SourceURI, StatusUnknown),
		CollectedAtUTC:          collected,
		SourceObservedTimeUTC:   sourceObserved,
		TrustLevel:              normalizeDefault(opts.TrustLevel, TrustLevelUnknown),
		CollectorGitSHA:         normalizeDefault(opts.CollectorGitSHA, StatusUnknown),
		RawPayloadSHA256:        rawHash,
		NormalizedPayloadSHA256: StatusUnknown,
		Symbols:                 []Symbol{},
		Warnings:                []Warning{},
	}

	if len(opts.RawPayload) > 0 {
		symbols, observed, err := normalizeRawPayload(opts.RawPayload, snapshot)
		if err != nil {
			return nil, err
		}
		snapshot.Symbols = symbols
		if snapshot.SourceObservedTimeUTC == nil && observed != nil {
			snapshot.SourceObservedTimeUTC = observed
		}
	}

	ValidateSnapshot(snapshot)
	snapshot.Hashes = ComputeSnapshotHashes(snapshot)
	snapshot.NormalizedPayloadSHA256 = snapshot.Hashes.NormalizedPayloadHash
	snapshot.SnapshotID = buildSnapshotID(snapshot)
	return snapshot, nil
}

func FetchBinanceFuturesExchangeInfo(ctx context.Context, client *http.Client, baseURL string) ([]byte, string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if baseURL == "" {
		baseURL = "https://fapi.binance.com"
	}
	endpoint, err := url.JoinPath(strings.TrimRight(baseURL, "/"), "/fapi/v1/exchangeInfo")
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("binance exchangeInfo failed: status %d body %s", resp.StatusCode, string(body))
	}
	return body, endpoint, nil
}

func normalizeRawPayload(data []byte, snapshot *Snapshot) ([]Symbol, *string, error) {
	switch {
	case snapshot.Exchange == "binance" && (snapshot.MarketType == "futures_um" || snapshot.MarketType == "futures-um" || snapshot.MarketType == "futures"):
		return normalizeBinanceFuturesExchangeInfo(data, snapshot)
	default:
		return nil, nil, fmt.Errorf("unsupported exchange metadata source: exchange=%s market_type=%s", snapshot.Exchange, snapshot.MarketType)
	}
}

type binanceExchangeInfo struct {
	ServerTime int64                   `json:"serverTime"`
	Symbols    []binanceExchangeSymbol `json:"symbols"`
}

type binanceExchangeSymbol struct {
	Symbol         string   `json:"symbol"`
	Pair           string   `json:"pair"`
	ContractType   string   `json:"contractType"`
	DeliveryDate   int64    `json:"deliveryDate"`
	OnboardDate    int64    `json:"onboardDate"`
	Status         string   `json:"status"`
	MaintMarginPct string   `json:"maintMarginPercent"`
	RequiredMargin string   `json:"requiredMarginPercent"`
	BaseAsset      string   `json:"baseAsset"`
	QuoteAsset     string   `json:"quoteAsset"`
	MarginAsset    string   `json:"marginAsset"`
	UnderlyingType string   `json:"underlyingType"`
	Permissions    []string `json:"permissions"`
}

func normalizeBinanceFuturesExchangeInfo(data []byte, snapshot *Snapshot) ([]Symbol, *string, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var payload binanceExchangeInfo
	if err := dec.Decode(&payload); err != nil {
		return nil, nil, fmt.Errorf("parse Binance futures exchangeInfo: %w", err)
	}

	var observed *string
	if payload.ServerTime > 0 {
		value := time.UnixMilli(payload.ServerTime).UTC().Format(time.RFC3339)
		observed = &value
	}

	filter := strings.ToUpper(strings.TrimSpace(snapshot.QuoteAssetFilter))
	symbols := make([]Symbol, 0, len(payload.Symbols))
	for _, raw := range payload.Symbols {
		symbol := strings.ToUpper(strings.TrimSpace(raw.Symbol))
		quote := strings.ToUpper(strings.TrimSpace(raw.QuoteAsset))
		if filter != "" && quote != filter {
			continue
		}
		contract := nullableUpper(raw.ContractType)
		perms := append([]string{}, raw.Permissions...)
		sort.Strings(perms)

		sourceFields := []string{"symbol"}
		if raw.BaseAsset != "" {
			sourceFields = append(sourceFields, "baseAsset")
		}
		if raw.QuoteAsset != "" {
			sourceFields = append(sourceFields, "quoteAsset")
		}
		if raw.Status != "" {
			sourceFields = append(sourceFields, "status")
		}
		if raw.ContractType != "" {
			sourceFields = append(sourceFields, "contractType")
		}
		if raw.OnboardDate > 0 {
			sourceFields = append(sourceFields, "onboardDate")
		}
		if raw.DeliveryDate > 0 {
			sourceFields = append(sourceFields, "deliveryDate")
		}
		if len(raw.Permissions) > 0 {
			sourceFields = append(sourceFields, "permissions")
		}

		entry := Symbol{
			Symbol:            symbol,
			BaseAsset:         normalizeDefault(strings.ToUpper(raw.BaseAsset), inferBaseAsset(symbol, quote)),
			QuoteAsset:        normalizeDefault(quote, inferQuoteAsset(symbol)),
			MarketType:        snapshot.MarketType,
			Exchange:          snapshot.Exchange,
			Status:            mapStatus(raw.Status),
			ContractType:      contract,
			OnboardDateUTC:    millisPtr(raw.OnboardDate),
			DeliveryDateUTC:   millisPtr(raw.DeliveryDate),
			FirstTradeDateUTC: nil,
			Permissions:       perms,
			RawStatus:         strings.ToUpper(strings.TrimSpace(raw.Status)),
			SourceFields:      sourceFields,
			Warnings:          []Warning{},
		}
		symbols = append(symbols, entry)
	}
	return symbols, observed, nil
}

func mapStatus(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case StatusActive:
		return StatusActive
	case StatusTrading:
		return StatusTrading
	case StatusBreak:
		return StatusBreak
	case StatusHalt:
		return StatusHalt
	case "PENDING_TRADING", StatusPreTrading:
		return StatusPreTrading
	case StatusSettling:
		return StatusSettling
	case StatusDelivered:
		return StatusDelivered
	case StatusDelisted:
		return StatusDelisted
	case StatusExpired:
		return StatusExpired
	case "":
		return StatusUnknown
	default:
		return StatusUnknown
	}
}

func millisPtr(ms int64) *string {
	if ms <= 0 {
		return nil
	}
	value := time.UnixMilli(ms).UTC().Format(time.RFC3339)
	return &value
}

func nullableUpper(value string) *string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return nil
	}
	return &value
}

func normalizeDefault(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func normalizeTimestampOrNow(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Now().UTC().Format(time.RFC3339)
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	return t.UTC().Format(time.RFC3339)
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

func normalizeNullableTimestamp(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, StatusUnknown) {
		return nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return &value
	}
	normalized := t.UTC().Format(time.RFC3339)
	return &normalized
}

func normalizeURIForHash(uri string) string {
	uri = strings.TrimSpace(uri)
	if uri == "" || strings.EqualFold(uri, StatusUnknown) {
		return StatusUnknown
	}
	uri = filepath.ToSlash(uri)
	if filepath.IsAbs(uri) {
		return filepath.Base(uri)
	}
	return uri
}

func inferBaseAsset(symbol, quoteAsset string) string {
	if quoteAsset != "" && strings.HasSuffix(symbol, quoteAsset) && len(symbol) > len(quoteAsset) {
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

func buildSnapshotID(snapshot *Snapshot) string {
	hash := snapshot.Hashes.SnapshotHash
	if len(hash) > 12 {
		hash = hash[:12]
	}
	collected := strings.NewReplacer("-", "", ":", "", "T", "_", "Z", "").Replace(snapshot.CollectedAtUTC)
	return strings.ToLower(fmt.Sprintf("%s_%s_%s_%s", snapshot.Exchange, snapshot.MarketType, collected, hash))
}
