package derivatives

import (
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
