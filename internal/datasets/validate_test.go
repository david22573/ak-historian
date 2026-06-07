package datasets

import (
	"testing"

	"github.com/davidmiguel22573/ak-historian/internal/datasets/derivatives"
	"github.com/davidmiguel22573/ak-historian/internal/datasets/sentiment"
)

func TestValidateSentimentRows_Duplicates(t *testing.T) {
	rows := []sentiment.Row{
		{
			Source:        "alternative_me",
			Dataset:       "fear_greed",
			Scope:         "global",
			Interval:      "1d",
			EventTimeMS:   1000,
			AvailableAtMS: 2000,
			Score:         50,
			Intensity:     0,
		},
		{
			Source:        "alternative_me",
			Dataset:       "fear_greed",
			Scope:         "global",
			Interval:      "1d",
			EventTimeMS:   1000,
			AvailableAtMS: 2000,
			Score:         50,
			Intensity:     0,
		},
	}

	err := ValidateSentimentRows(rows)
	if err == nil || err.Error() != "row 1: duplicate event_time_ms 1000" {
		t.Errorf("expected duplicate error, got %v", err)
	}
}

func TestValidateSentimentRows_AvailableAt(t *testing.T) {
	rows := []sentiment.Row{
		{
			Source:        "alternative_me",
			Dataset:       "fear_greed",
			Scope:         "global",
			Interval:      "1d",
			EventTimeMS:   2000,
			AvailableAtMS: 1000,
			Score:         50,
			Intensity:     0,
		},
	}

	err := ValidateSentimentRows(rows)
	if err == nil || err.Error() != "row 0: available_at_ms < event_time_ms" {
		t.Errorf("expected available_at_ms error, got %v", err)
	}
}

func TestValidateDerivativesRowsRequiresEventAndAvailableTimes(t *testing.T) {
	rows := []derivatives.Row{
		{
			Source:   "binance",
			Dataset:  "funding_rate",
			Market:   "futures-um",
			Symbol:   "LINKUSDT",
			Interval: "8h",
			Value:    0.0001,
		},
	}
	err := ValidateDerivativesRows(rows)
	if err == nil || err.Error() != "row 0: event_time_ms <= 0" {
		t.Fatalf("expected event_time_ms validation error, got %v", err)
	}

	rows[0].EventTimeMS = 2000
	err = ValidateDerivativesRows(rows)
	if err == nil || err.Error() != "row 0: available_at_ms <= 0" {
		t.Fatalf("expected available_at_ms validation error, got %v", err)
	}
}

func TestValidateDerivativesRowsRejectsAvailableBeforeEvent(t *testing.T) {
	rows := []derivatives.Row{
		{
			Source:        "binance",
			Dataset:       "funding_rate",
			Market:        "futures-um",
			Symbol:        "LINKUSDT",
			Interval:      "8h",
			EventTimeMS:   2000,
			AvailableAtMS: 1000,
			Value:         0.0001,
		},
	}
	err := ValidateDerivativesRows(rows)
	if err == nil || err.Error() != "row 0: available_at_ms < event_time_ms" {
		t.Fatalf("expected available_at_ms < event_time_ms error, got %v", err)
	}
}
