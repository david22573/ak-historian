package coverage

import (
	"testing"
	"time"
)

func TestCalculateExpectedCandles(t *testing.T) {
	from := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2023, 1, 1, 0, 2, 0, 0, time.UTC) // 00:00, 00:01, 00:02 -> 3 candles

	expected, err := CalculateExpectedCandles(from, to, "1m")
	if err != nil {
		t.Fatalf("CalculateExpectedCandles: %v", err)
	}
	if expected != int64(3) {
		t.Fatalf("expected = %d, want 3", expected)
	}

	// Leap year
	from = time.Date(2024, 2, 28, 0, 0, 0, 0, time.UTC)
	to = time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	// 2024-02-28 (1440) + 2024-02-29 (1440) + 2024-03-01 00:00 (1) = 2881
	expected, err = CalculateExpectedCandles(from, to, "1m")
	if err != nil {
		t.Fatalf("CalculateExpectedCandles leap year: %v", err)
	}
	if expected != int64(2881) {
		t.Fatalf("expected leap year = %d, want 2881", expected)
	}
}

func TestVerifyCoverage(t *testing.T) {
	from := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2023, 1, 1, 0, 2, 0, 0, time.UTC)
	market := "futures-um"
	symbol := "LINKUSDT"
	interval := "1m"

	t.Run("perfect sequence", func(t *testing.T) {
		openTimes := []int64{
			from.UnixMilli(),
			from.Add(time.Minute).UnixMilli(),
			to.UnixMilli(),
		}
		report := VerifyCoverage(market, symbol, interval, from, to, openTimes)
		if report.Status != StatusPass || report.ExpectedCandles != 3 || report.ActualCandles != 3 || report.MissingCandles != 0 || report.DuplicateOpenTimes != 0 {
			t.Fatalf("unexpected report: %+v", report)
		}
	})

	t.Run("one missing candle", func(t *testing.T) {
		openTimes := []int64{
			from.UnixMilli(),
			to.UnixMilli(),
		}
		report := VerifyCoverage(market, symbol, interval, from, to, openTimes)
		if report.Status != StatusFail || report.MissingCandles != 1 || report.FirstGap == nil || report.FirstGap.UnixMilli() != from.Add(time.Minute).UnixMilli() {
			t.Fatalf("unexpected report: %+v", report)
		}
	})

	t.Run("duplicate candle", func(t *testing.T) {
		openTimes := []int64{
			from.UnixMilli(),
			from.Add(time.Minute).UnixMilli(),
			from.Add(time.Minute).UnixMilli(),
			to.UnixMilli(),
		}
		report := VerifyCoverage(market, symbol, interval, from, to, openTimes)
		if report.Status != StatusFail || report.DuplicateOpenTimes != 1 {
			t.Fatalf("unexpected report: %+v", report)
		}
	})

	t.Run("empty dataset", func(t *testing.T) {
		report := VerifyCoverage(market, symbol, interval, from, to, []int64{})
		if report.Status != StatusFail || report.MissingCandles != 3 {
			t.Fatalf("unexpected report: %+v", report)
		}
	})

	t.Run("wrong interval step", func(t *testing.T) {
		// Expect 1m, but got 2m
		openTimes := []int64{
			from.UnixMilli(),
			from.Add(2 * time.Minute).UnixMilli(),
		}
		report := VerifyCoverage(market, symbol, interval, from, to, openTimes)
		if report.Status != StatusFail || report.MissingCandles != 1 {
			t.Fatalf("unexpected report: %+v", report)
		}
	})

	t.Run("unsupported interval", func(t *testing.T) {
		report := VerifyCoverage(market, symbol, "2m", from, to, []int64{from.UnixMilli()})
		if report.Status != StatusFail {
			t.Fatalf("unexpected report: %+v", report)
		}
	})

	t.Run("start after end", func(t *testing.T) {
		report := VerifyCoverage(market, symbol, interval, to, from, []int64{})
		if report.Status != StatusFail {
			t.Fatalf("unexpected report: %+v", report)
		}
	})

	t.Run("partial range", func(t *testing.T) {
		partialTo := from.Add(5 * time.Minute)
		openTimes := []int64{
			from.Add(2 * time.Minute).UnixMilli(),
			from.Add(3 * time.Minute).UnixMilli(),
		}
		report := VerifyCoverage(market, symbol, interval, from, partialTo, openTimes)
		if report.Status != StatusFail || report.MissingCandles != 4 {
			t.Fatalf("unexpected report: %+v", report)
		}
	})

	t.Run("monthly boundary", func(t *testing.T) {
		monthFrom := time.Date(2023, 1, 31, 23, 59, 0, 0, time.UTC)
		monthTo := time.Date(2023, 2, 1, 0, 1, 0, 0, time.UTC)
		openTimes := []int64{
			monthFrom.UnixMilli(),
			monthFrom.Add(time.Minute).UnixMilli(),
			monthTo.UnixMilli(),
		}
		report := VerifyCoverage(market, symbol, interval, monthFrom, monthTo, openTimes)
		if report.Status != StatusPass {
			t.Fatalf("unexpected report: %+v", report)
		}
	})

	t.Run("daily boundary", func(t *testing.T) {
		dayFrom := time.Date(2023, 5, 1, 23, 59, 0, 0, time.UTC)
		dayTo := time.Date(2023, 5, 2, 0, 1, 0, 0, time.UTC)
		openTimes := []int64{
			dayFrom.UnixMilli(),
			dayFrom.Add(time.Minute).UnixMilli(),
			dayTo.UnixMilli(),
		}
		report := VerifyCoverage(market, symbol, interval, dayFrom, dayTo, openTimes)
		if report.Status != StatusPass {
			t.Fatalf("unexpected report: %+v", report)
		}
	})
}
