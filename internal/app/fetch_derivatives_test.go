package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/david22573/ak-historian/internal/datasets/derivatives"
)

type fakeDerivativesFetcher struct {
	rows []derivatives.Row
	err  error
}

func (f fakeDerivativesFetcher) Fetch(ctx context.Context, req derivatives.FetchRequest) ([]derivatives.Row, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.rows, nil
}

func TestFetchDerivativesReportsLimitedHistory(t *testing.T) {
	result, err := runFetchDerivatives(context.Background(), FetchDerivativesOptions{
		Source:  "binance",
		Dataset: derivatives.DatasetOpenInterest,
		Market:  "futures-um",
		Symbols: []string{"LINKUSDT"},
		Start:   "2023-01-01",
		End:     "2023-12-31",
		Out:     t.TempDir(),
		Format:  "json",
		Client: fakeDerivativesFetcher{
			err: derivatives.LimitedHistoryError{
				Dataset: derivatives.DatasetOpenInterest,
				Reason:  "endpoint does not expose requested historical range",
			},
		},
	})
	if err != nil {
		t.Fatalf("limited history should be reported without hard failure: %v", err)
	}
	if result.Status != "limited_history" {
		t.Fatalf("status = %s, want limited_history", result.Status)
	}
	if result.Reason != "endpoint does not expose requested historical range" {
		t.Fatalf("reason = %q", result.Reason)
	}
}

func TestFetchDerivativesWritesJSONWithoutZeroFilledMissingRows(t *testing.T) {
	event := time.Date(2023, 1, 1, 8, 0, 0, 0, time.UTC).UnixMilli()
	dir := t.TempDir()
	result, err := runFetchDerivatives(context.Background(), FetchDerivativesOptions{
		Source:  "binance",
		Dataset: derivatives.DatasetFundingRate,
		Market:  "futures-um",
		Symbols: []string{"LINKUSDT"},
		Start:   "2023-01-01",
		End:     "2023-01-31",
		Out:     dir,
		Format:  "json",
		Client: fakeDerivativesFetcher{
			rows: []derivatives.Row{
				{
					Source:        "binance",
					Dataset:       derivatives.DatasetFundingRate,
					Market:        "futures-um",
					Symbol:        "LINKUSDT",
					Interval:      "8h",
					EventTimeMS:   event,
					AvailableAtMS: event,
					IngestedAtMS:  event,
					Value:         0.0001,
					SourceVersion: derivatives.SourceVersionBinanceFundingRate,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("fetch derivatives: %v", err)
	}
	if result.Status != "PASS" || result.Rows != 1 || result.Objects != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	path := filepath.Join(dir, "datasets", "derivatives", "source=binance", "dataset=funding_rate", "market=futures-um", "symbol=LINKUSDT", "interval=8h", "year=2023", "month=01", "LINKUSDT-funding_rate-2023-01.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read derivatives json: %v", err)
	}
	var rows []derivatives.Row
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatalf("unmarshal derivatives json: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected only fetched rows, got %d", len(rows))
	}
	if rows[0].EventTimeMS == 0 || rows[0].AvailableAtMS == 0 {
		t.Fatalf("row looks zero-filled: %+v", rows[0])
	}
}
