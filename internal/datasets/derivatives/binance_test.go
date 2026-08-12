package derivatives

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLimitedHistoryStatusReportsHistoricalEndpointLimit(t *testing.T) {
	now := time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC)
	start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)
	limited, reason := LimitedHistoryStatus(DatasetOpenInterest, start, end, now)
	if !limited {
		t.Fatalf("expected open_interest to be limited for 2023 backfill")
	}
	if reason != "endpoint does not expose requested historical range" {
		t.Fatalf("reason = %q", reason)
	}

	limited, reason = LimitedHistoryStatus(DatasetFundingRate, start, end, now)
	if limited || reason != "" {
		t.Fatalf("funding_rate should support historical backfill, limited=%t reason=%q", limited, reason)
	}
}

func TestBinanceDerivativeAvailabilityUsesObservedIngestionForEveryDataset(t *testing.T) {
	event := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	observed := event.Add(2 * time.Minute)
	tests := []struct {
		name     string
		dataset  string
		response string
	}{
		{"funding", DatasetFundingRate, `[{"symbol":"BTCUSDT","fundingRate":"0.001","fundingTime":` + fmt.Sprint(event.UnixMilli()) + `,"markPrice":"100"}]`},
		{"open interest", DatasetOpenInterest, `[{"symbol":"BTCUSDT","sumOpenInterest":"10","sumOpenInterestValue":"1000","timestamp":` + fmt.Sprint(event.UnixMilli()) + `}]`},
		{"long short", DatasetLongShortRatio, `[{"symbol":"BTCUSDT","longShortRatio":"1.1","longAccount":"0.52","shortAccount":"0.48","timestamp":` + fmt.Sprint(event.UnixMilli()) + `}]`},
		{"top trader", DatasetTopTraderLongShortRatio, `[{"symbol":"BTCUSDT","longShortRatio":"1.2","longAccount":"0.55","shortAccount":"0.45","timestamp":` + fmt.Sprint(event.UnixMilli()) + `}]`},
		{"taker", DatasetTakerBuySellVolume, `[{"symbol":"BTCUSDT","buySellRatio":"1.5","buyVol":"60","sellVol":"40","timestamp":` + fmt.Sprint(event.UnixMilli()) + `}]`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tc.response))
			}))
			defer server.Close()
			client := &BinanceClient{BaseURL: server.URL, HTTPClient: server.Client(), Now: func() time.Time { return observed }}
			rows, err := client.Fetch(context.Background(), FetchRequest{
				Source: "binance", Dataset: tc.dataset, Market: "futures-um", Symbol: "BTCUSDT",
				Interval: DefaultInterval(tc.dataset), Start: event.Add(-time.Minute), End: observed,
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 1 {
				t.Fatalf("rows=%d, want 1", len(rows))
			}
			row := rows[0]
			if row.AvailableAtMS != observed.UnixMilli() || row.IngestedAtMS != observed.UnixMilli() || row.AvailableAtMS <= row.EventTimeMS {
				t.Fatalf("delayed publication was backdated: %+v", row)
			}
			if row.AvailabilityPolicyID != AvailabilityPolicyObservedIngestionID || row.AvailabilityPolicyVersion != AvailabilityPolicyObservedIngestionVersion {
				t.Fatalf("availability policy missing: %+v", row)
			}
		})
	}
}
