package derivatives

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	DatasetFundingRate               = "funding_rate"
	DatasetOpenInterest              = "open_interest"
	DatasetLongShortRatio            = "long_short_ratio"
	DatasetTopTraderLongShortRatio   = "top_trader_long_short_ratio"
	DatasetTakerBuySellVolume        = "taker_buy_sell_volume"
	SourceVersionBinanceFundingRate  = "binance_usdm:/fapi/v1/fundingRate"
	SourceVersionBinanceOpenInterest = "binance_usdm:/futures/data/openInterestHist"
	SourceVersionBinanceLongShort    = "binance_usdm:/futures/data/globalLongShortAccountRatio"
	SourceVersionBinanceTopTrader    = "binance_usdm:/futures/data/topLongShortPositionRatio"
	SourceVersionBinanceTaker        = "binance_usdm:/futures/data/takerlongshortRatio"
)

type FetchRequest struct {
	Source   string
	Dataset  string
	Market   string
	Symbol   string
	Interval string
	Start    time.Time
	End      time.Time
}

type LimitedHistoryError struct {
	Dataset string
	Reason  string
}

func (e LimitedHistoryError) Error() string {
	return e.Reason
}

type BinanceClient struct {
	BaseURL    string
	HTTPClient *http.Client
	Now        func() time.Time
}

func NewBinanceClient() *BinanceClient {
	return &BinanceClient{
		BaseURL:    "https://fapi.binance.com",
		HTTPClient: http.DefaultClient,
		Now:        time.Now,
	}
}

func DefaultInterval(dataset string) string {
	if dataset == DatasetFundingRate {
		return "8h"
	}
	return "5m"
}

func IsSupportedDataset(dataset string) bool {
	switch dataset {
	case DatasetFundingRate, DatasetOpenInterest, DatasetLongShortRatio, DatasetTopTraderLongShortRatio, DatasetTakerBuySellVolume:
		return true
	default:
		return false
	}
}

func LimitedHistoryStatus(dataset string, start, end, now time.Time) (bool, string) {
	if dataset == DatasetFundingRate {
		return false, ""
	}
	if !IsSupportedDataset(dataset) {
		return false, ""
	}
	windowStart := now.UTC().AddDate(0, 0, -30)
	if dataset == DatasetOpenInterest {
		windowStart = now.UTC().AddDate(0, -1, 0)
	}
	if start.UTC().Before(windowStart) || end.UTC().Before(windowStart) {
		return true, "endpoint does not expose requested historical range"
	}
	return false, ""
}

func (c *BinanceClient) Fetch(ctx context.Context, req FetchRequest) ([]Row, error) {
	if req.Source != "" && req.Source != "binance" {
		return nil, fmt.Errorf("unsupported source: %s", req.Source)
	}
	if req.Market != "futures-um" {
		return nil, fmt.Errorf("unsupported market: %s", req.Market)
	}
	if req.Interval == "" {
		req.Interval = DefaultInterval(req.Dataset)
	}
	nowFn := c.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	if limited, reason := LimitedHistoryStatus(req.Dataset, req.Start, req.End, nowFn()); limited {
		return nil, LimitedHistoryError{Dataset: req.Dataset, Reason: reason}
	}
	switch req.Dataset {
	case DatasetFundingRate:
		return c.fetchFundingRates(ctx, req)
	case DatasetOpenInterest, DatasetLongShortRatio, DatasetTopTraderLongShortRatio, DatasetTakerBuySellVolume:
		return c.fetchLimitedStats(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported dataset: %s", req.Dataset)
	}
}

func (c *BinanceClient) fetchFundingRates(ctx context.Context, req FetchRequest) ([]Row, error) {
	var rows []Row
	startMS := req.Start.UTC().UnixMilli()
	endMS := req.End.UTC().UnixMilli()
	for {
		values := url.Values{}
		values.Set("symbol", strings.ToUpper(req.Symbol))
		values.Set("startTime", strconv.FormatInt(startMS, 10))
		values.Set("endTime", strconv.FormatInt(endMS, 10))
		values.Set("limit", "1000")
		var records []fundingRateRecord
		if err := c.getJSON(ctx, "/fapi/v1/fundingRate", values, &records); err != nil {
			return nil, err
		}
		if len(records) == 0 {
			break
		}
		for _, rec := range records {
			eventMS := rec.FundingTime
			if eventMS < req.Start.UTC().UnixMilli() || eventMS > endMS {
				continue
			}
			value, err := strconv.ParseFloat(rec.FundingRate, 64)
			if err != nil {
				return nil, fmt.Errorf("parse fundingRate: %w", err)
			}
			mark, _ := strconv.ParseFloat(rec.MarkPrice, 64)
			rows = append(rows, Row{
				Source:        "binance",
				Dataset:       req.Dataset,
				Market:        req.Market,
				Symbol:        strings.ToUpper(req.Symbol),
				Interval:      req.Interval,
				EventTimeMS:   eventMS,
				AvailableAtMS: eventMS,
				IngestedAtMS:  time.Now().UTC().UnixMilli(),
				Value:         value,
				Extra1:        mark,
				SourceVersion: SourceVersionBinanceFundingRate,
			})
		}
		last := records[len(records)-1].FundingTime
		if last >= endMS || last < startMS || len(records) < 1000 {
			break
		}
		startMS = last + 1
	}
	return rows, nil
}

func (c *BinanceClient) fetchLimitedStats(ctx context.Context, req FetchRequest) ([]Row, error) {
	var rows []Row
	startMS := req.Start.UTC().UnixMilli()
	endMS := req.End.UTC().UnixMilli()
	path, sourceVersion := limitedDatasetEndpoint(req.Dataset)
	for {
		values := url.Values{}
		values.Set("symbol", strings.ToUpper(req.Symbol))
		values.Set("period", req.Interval)
		values.Set("startTime", strconv.FormatInt(startMS, 10))
		values.Set("endTime", strconv.FormatInt(endMS, 10))
		values.Set("limit", "500")
		var records []limitedStatsRecord
		if err := c.getJSON(ctx, path, values, &records); err != nil {
			return nil, err
		}
		if len(records) == 0 {
			break
		}
		last := int64(0)
		for _, rec := range records {
			row, err := rec.toRow(req, sourceVersion)
			if err != nil {
				return nil, err
			}
			if row.EventTimeMS < req.Start.UTC().UnixMilli() || row.EventTimeMS > endMS {
				continue
			}
			row.IngestedAtMS = time.Now().UTC().UnixMilli()
			rows = append(rows, row)
			last = row.EventTimeMS
		}
		if last == 0 || last >= endMS || len(records) < 500 {
			break
		}
		startMS = last + 1
	}
	return rows, nil
}

func limitedDatasetEndpoint(dataset string) (string, string) {
	switch dataset {
	case DatasetOpenInterest:
		return "/futures/data/openInterestHist", SourceVersionBinanceOpenInterest
	case DatasetLongShortRatio:
		return "/futures/data/globalLongShortAccountRatio", SourceVersionBinanceLongShort
	case DatasetTopTraderLongShortRatio:
		return "/futures/data/topLongShortPositionRatio", SourceVersionBinanceTopTrader
	case DatasetTakerBuySellVolume:
		return "/futures/data/takerlongshortRatio", SourceVersionBinanceTaker
	default:
		return "", ""
	}
}

func (c *BinanceClient) getJSON(ctx context.Context, path string, values url.Values, dest any) error {
	base := c.BaseURL
	if base == "" {
		base = "https://fapi.binance.com"
	}
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	u := strings.TrimRight(base, "/") + path + "?" + values.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("binance %s failed: status %d body %s", path, resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("decode binance response: %w", err)
	}
	return nil
}

type fundingRateRecord struct {
	Symbol      string `json:"symbol"`
	FundingRate string `json:"fundingRate"`
	FundingTime int64  `json:"fundingTime"`
	MarkPrice   string `json:"markPrice"`
}

type limitedStatsRecord struct {
	Symbol               string          `json:"symbol"`
	SumOpenInterest      string          `json:"sumOpenInterest"`
	SumOpenInterestValue string          `json:"sumOpenInterestValue"`
	LongShortRatio       string          `json:"longShortRatio"`
	LongAccount          string          `json:"longAccount"`
	ShortAccount         string          `json:"shortAccount"`
	BuySellRatio         string          `json:"buySellRatio"`
	BuyVol               string          `json:"buyVol"`
	SellVol              string          `json:"sellVol"`
	Timestamp            json.RawMessage `json:"timestamp"`
}

func (r limitedStatsRecord) toRow(req FetchRequest, sourceVersion string) (Row, error) {
	eventMS, err := rawInt64(r.Timestamp)
	if err != nil {
		return Row{}, err
	}
	row := Row{
		Source:        "binance",
		Dataset:       req.Dataset,
		Market:        req.Market,
		Symbol:        strings.ToUpper(req.Symbol),
		Interval:      req.Interval,
		EventTimeMS:   eventMS,
		AvailableAtMS: eventMS,
		SourceVersion: sourceVersion,
	}
	switch req.Dataset {
	case DatasetOpenInterest:
		row.Value, err = strconv.ParseFloat(r.SumOpenInterest, 64)
		row.Extra1, _ = strconv.ParseFloat(r.SumOpenInterestValue, 64)
	case DatasetLongShortRatio, DatasetTopTraderLongShortRatio:
		row.Value, err = strconv.ParseFloat(r.LongShortRatio, 64)
		row.Extra1, _ = strconv.ParseFloat(r.LongAccount, 64)
		row.Extra2, _ = strconv.ParseFloat(r.ShortAccount, 64)
	case DatasetTakerBuySellVolume:
		row.Value, err = strconv.ParseFloat(r.BuySellRatio, 64)
		row.Extra1, _ = strconv.ParseFloat(r.BuyVol, 64)
		row.Extra2, _ = strconv.ParseFloat(r.SellVol, 64)
	}
	if err != nil {
		return Row{}, err
	}
	return row, nil
}

func rawInt64(raw json.RawMessage) (int64, error) {
	text := strings.Trim(string(raw), `"`)
	if text == "" || text == "null" {
		return 0, fmt.Errorf("missing timestamp")
	}
	return strconv.ParseInt(text, 10, 64)
}
