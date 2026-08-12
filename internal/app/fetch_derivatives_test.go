package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/david22573/ak-historian/internal/datasets"
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
		Source:        "binance",
		Dataset:       derivatives.DatasetOpenInterest,
		Market:        "futures-um",
		Symbols:       []string{"LINKUSDT"},
		Start:         "2023-01-01",
		End:           "2023-12-31",
		Out:           t.TempDir(),
		Format:        "json",
		WriteManifest: true,
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
		Source:        "binance",
		Dataset:       derivatives.DatasetFundingRate,
		Market:        "futures-um",
		Symbols:       []string{"LINKUSDT"},
		Start:         "2023-01-01",
		End:           "2023-01-31",
		Out:           dir,
		Format:        "json",
		WriteManifest: true,
		Client: fakeDerivativesFetcher{
			rows: []derivatives.Row{
				{
					Source:                    "binance",
					Dataset:                   derivatives.DatasetFundingRate,
					Market:                    "futures-um",
					Symbol:                    "LINKUSDT",
					Interval:                  "8h",
					EventTimeMS:               event,
					AvailableAtMS:             event,
					IngestedAtMS:              event,
					Value:                     0.0001,
					SourceVersion:             derivatives.SourceVersionBinanceFundingRate,
					AvailabilityPolicyID:      derivatives.AvailabilityPolicyObservedIngestionID,
					AvailabilityPolicyVersion: derivatives.AvailabilityPolicyObservedIngestionVersion,
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
	if len(result.Manifests) != 1 {
		t.Fatalf("manifest missing: %+v", result)
	}
	manifestBytes, err := os.ReadFile(result.Manifests[0])
	if err != nil {
		t.Fatal(err)
	}
	var manifest datasets.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.AvailabilityPolicyID != derivatives.AvailabilityPolicyObservedIngestionID || manifest.AvailabilityPolicyVersion != derivatives.AvailabilityPolicyObservedIngestionVersion || len(manifest.Objects) != 1 || manifest.Objects[0].ContentHash == "" {
		t.Fatalf("manifest does not bind policy/object content: %+v", manifest)
	}
}

func TestFetchDerivativesRejectsSourceDisorderBeforeWriting(t *testing.T) {
	first := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC).UnixMilli()
	row := func(event int64) derivatives.Row {
		return derivatives.Row{Source: "binance", Dataset: derivatives.DatasetFundingRate, Market: "futures-um", Symbol: "LINKUSDT", Interval: "8h", EventTimeMS: event, AvailableAtMS: event + 1000, IngestedAtMS: event + 1000, Value: 0.001, SourceVersion: derivatives.SourceVersionBinanceFundingRate, AvailabilityPolicyID: derivatives.AvailabilityPolicyObservedIngestionID, AvailabilityPolicyVersion: derivatives.AvailabilityPolicyObservedIngestionVersion}
	}
	result, err := runFetchDerivatives(context.Background(), FetchDerivativesOptions{
		Source: "binance", Dataset: derivatives.DatasetFundingRate, Market: "futures-um", Symbols: []string{"LINKUSDT"}, Interval: "8h",
		Start: "2026-08-01", End: "2026-08-02", Out: t.TempDir(), Format: "json",
		WriteManifest: true,
		Client:        fakeDerivativesFetcher{rows: []derivatives.Row{row(first + 8*60*60*1000), row(first)}},
	})
	if err == nil || result.Status != "FAIL" {
		t.Fatalf("source disorder was normalized: result=%+v err=%v", result, err)
	}
}
