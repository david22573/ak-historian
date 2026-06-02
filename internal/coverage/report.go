package coverage

import (
	"fmt"
	"time"
)

type Report struct {
	Market             string     `json:"market"`
	Symbol             string     `json:"symbol"`
	Interval           string     `json:"interval"`
	From               time.Time  `json:"from"`
	To                 time.Time  `json:"to"`
	ExpectedCandles    int64      `json:"expected_candles"`
	ActualCandles      int64      `json:"actual_candles"`
	UniqueOpenTimes    int64      `json:"unique_open_times"`
	DuplicateOpenTimes int64      `json:"duplicate_open_times"`
	MissingCandles     int64      `json:"missing_candles"`
	FirstOpenTime      time.Time  `json:"first_open_time"`
	LastOpenTime       time.Time  `json:"last_open_time"`
	FirstGap           *time.Time `json:"first_gap"`
	LastGap            *time.Time `json:"last_gap"`
	Status             string     `json:"status"`
}

const (
	StatusPass = "PASS"
	StatusFail = "FAIL"
)

func IntervalToMS(interval string) (int64, bool) {
	switch interval {
	case "1m":
		return 60000, true
	case "3m":
		return 180000, true
	case "5m":
		return 300000, true
	case "15m":
		return 900000, true
	case "1h":
		return 3600000, true
	case "4h":
		return 14400000, true
	case "1d":
		return 86400000, true
	default:
		return 0, false
	}
}

func CalculateExpectedCandles(from, to time.Time, interval string) (int64, error) {
	ms, ok := IntervalToMS(interval)
	if !ok {
		return 0, fmt.Errorf("unsupported interval: %s", interval)
	}

	duration := to.Sub(from)
	if duration < 0 {
		return 0, fmt.Errorf("from must be <= to")
	}

	// inclusive of both endpoints
	return (duration.Milliseconds() / ms) + 1, nil
}

func VerifyCoverage(market, symbol, interval string, from, to time.Time, openTimes []int64) Report {
	expected, err := CalculateExpectedCandles(from, to, interval)
	ms, ok := IntervalToMS(interval)

	report := Report{
		Market:          market,
		Symbol:          symbol,
		Interval:        interval,
		From:            from,
		To:              to,
		ExpectedCandles: expected,
		ActualCandles:   int64(len(openTimes)),
		Status:          StatusPass,
	}

	if err != nil || !ok {
		report.Status = StatusFail
		return report
	}

	if len(openTimes) == 0 {
		if expected > 0 {
			report.Status = StatusFail
			report.MissingCandles = expected
		}
		return report
	}

	report.FirstOpenTime = time.UnixMilli(openTimes[0])
	report.LastOpenTime = time.UnixMilli(openTimes[len(openTimes)-1])

	uniqueTimes := make(map[int64]struct{})
	var duplicates int64
	for _, t := range openTimes {
		if _, ok := uniqueTimes[t]; ok {
			duplicates++
		}
		uniqueTimes[t] = struct{}{}
	}
	report.UniqueOpenTimes = int64(len(uniqueTimes))
	report.DuplicateOpenTimes = duplicates

	// Detect gaps
	var missing int64
	current := from.UnixMilli()
	end := to.UnixMilli()

	// Simple gap detection: iterate expected timestamps
	// For performance with millions of candles, we might want a better way,
	// but for 1m over 3 years (1.8M), this is fine.

	// Better way: since openTimes is sorted, use a pointer
	ptr := 0
	for t := current; t <= end; t += ms {
		found := false
		for ptr < len(openTimes) && openTimes[ptr] < t {
			ptr++
		}
		if ptr < len(openTimes) && openTimes[ptr] == t {
			found = true
		}

		if !found {
			missing++
			gapTime := time.UnixMilli(t)
			if report.FirstGap == nil {
				report.FirstGap = &gapTime
			}
			report.LastGap = &gapTime
		}
	}

	report.MissingCandles = missing

	if report.MissingCandles > 0 || report.DuplicateOpenTimes > 0 || report.ActualCandles == 0 {
		report.Status = StatusFail
	}

	// If the range we got is different from expected from/to, it's also a fail
	// actually MissingCandles already covers it if we checked the whole range.

	return report
}
