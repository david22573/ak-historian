package coverage

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/david22573/ak-historian/internal/parquetutil"
)

type AuditorOptions struct {
	Mode         string    // "fast" or "strict"
	Start        time.Time // optional
	End          time.Time // optional
	MinPct       float64   // default 99.0
	IntervalHint string
	SymbolHint   string
}

func AuditFiles(files []string, opts AuditorOptions) (*DatasetCoverage, error) {
	if opts.MinPct == 0 {
		opts.MinPct = 99.0
	}

	res := &DatasetCoverage{
		Status:  CoverageStatusPass,
		Symbols: []SymbolCoverage{},
	}
	res.TotalFiles = len(files)
	if len(files) == 0 {
		return res, nil
	}

	// Group files by symbol and interval
	type groupKey struct {
		symbol   string
		interval string
	}
	groups := make(map[groupKey][]string)

	symRe := regexp.MustCompile(`symbol=([A-Z0-9]+)`)
	intRe := regexp.MustCompile(`interval=([0-9]+[mhdw])`)

	for _, f := range files {
		sym := opts.SymbolHint
		if sym == "" {
			m := symRe.FindStringSubmatch(filepath.ToSlash(f))
			if len(m) == 2 {
				sym = m[1]
			} else {
				// fallback infer from filename maybe?
				base := filepath.Base(f)
				parts := regexp.MustCompile(`^([A-Z0-9]+)-`).FindStringSubmatch(base)
				if len(parts) == 2 {
					sym = parts[1]
				}
			}
		}

		interval := opts.IntervalHint
		if interval == "" {
			m := intRe.FindStringSubmatch(filepath.ToSlash(f))
			if len(m) == 2 {
				interval = m[1]
			} else {
				base := filepath.Base(f)
				parts := regexp.MustCompile(`-([0-9]+[mhdw])-`).FindStringSubmatch(base)
				if len(parts) == 2 {
					interval = parts[1]
				}
			}
		}

		k := groupKey{symbol: sym, interval: interval}
		groups[k] = append(groups[k], f)
	}

	var hasFail, hasWarn, hasUnknown bool

	for k, gFiles := range groups {
		symCov := auditSymbol(k.symbol, k.interval, gFiles, opts)
		res.Symbols = append(res.Symbols, symCov)

		res.TotalRows += symCov.RowCount
		res.TotalExpectedRows += symCov.ExpectedRowCount
		res.TotalMissingRows += symCov.MissingRowCount
		res.TotalDuplicateTimestamps += symCov.DuplicateTimestampCount
		res.TotalOutOfOrderTimestamps += symCov.OutOfOrderTimestampCount
		res.TotalGapCount += symCov.GapCount
		if symCov.LargestGapSeconds > res.LargestGapSeconds {
			res.LargestGapSeconds = symCov.LargestGapSeconds
		}

		switch symCov.Status {
		case CoverageStatusFail:
			hasFail = true
		case CoverageStatusWarn:
			hasWarn = true
		case CoverageStatusUnknown:
			hasUnknown = true
		}
	}

	// Sort symbols for determinism
	sort.Slice(res.Symbols, func(i, j int) bool {
		if res.Symbols[i].Symbol != res.Symbols[j].Symbol {
			return res.Symbols[i].Symbol < res.Symbols[j].Symbol
		}
		return res.Symbols[i].Interval < res.Symbols[j].Interval
	})

	if hasFail {
		res.Status = CoverageStatusFail
	} else if hasWarn {
		res.Status = CoverageStatusWarn
	} else if hasUnknown {
		res.Status = CoverageStatusUnknown
	}

	return res, nil
}

func auditSymbol(symbol, interval string, files []string, opts AuditorOptions) SymbolCoverage {
	cov := SymbolCoverage{
		Symbol:   symbol,
		Interval: interval,
		Status:   CoverageStatusPass,
	}

	if symbol == "" {
		cov.Warnings = append(cov.Warnings, WarnSymbolInferFailed)
		cov.Status = CoverageStatusUnknown
	}

	intervalMS, err := parseIntervalMS(interval)
	if err != nil {
		cov.Warnings = append(cov.Warnings, WarnIntervalInferFailed)
		cov.Status = CoverageStatusUnknown
	}

	if cov.Status == CoverageStatusUnknown {
		return cov
	}

	var allTimes []int64
	var totalRows int64

	if opts.Mode == "strict" {
		// Strict mode reads all timestamps without duckdb ORDER BY to detect out-of-order properly
		times, err := parquetutil.ReadOpenTimesStrict(files)
		if err != nil {
			cov.Warnings = append(cov.Warnings, WarnRowCountsUnavailable, WarnTimestampRangeUnavailable)
			cov.Status = CoverageStatusFail
			return cov
		}
		allTimes = times
		totalRows = int64(len(allTimes))
	} else {
		// Fast mode reads open times via duckdb if available
		times, err := parquetutil.ReadOpenTimes(files)
		if err != nil {
			cov.Warnings = append(cov.Warnings, WarnRowCountsUnavailable, WarnTimestampRangeUnavailable)
			cov.Status = CoverageStatusFail
			return cov
		}
		allTimes = times
		totalRows = int64(len(allTimes))
	}

	cov.RowCount = totalRows
	if len(allTimes) == 0 {
		return cov // Empty
	}

	// Detect out-of-order in strict mode before sorting
	var outOfOrderCount int64
	if opts.Mode == "strict" {
		prev := allTimes[0]
		for i := 1; i < len(allTimes); i++ {
			if allTimes[i] < prev {
				outOfOrderCount++
			}
			prev = allTimes[i]
		}
	}
	cov.OutOfOrderTimestampCount = outOfOrderCount

	// Sort for gap/dup detection (since files might be interleaved if read out of order by the glob,
	// though they usually aren't, sorting helps calculate expected coverage properly)
	sort.SliceStable(allTimes, func(i, j int) bool {
		return allTimes[i] < allTimes[j]
	})

	minTime := allTimes[0]
	maxTime := allTimes[len(allTimes)-1]

	if !opts.Start.IsZero() && opts.Start.UnixMilli() > minTime {
		minTime = opts.Start.UnixMilli()
	}
	if !opts.End.IsZero() && opts.End.UnixMilli() < maxTime {
		maxTime = opts.End.UnixMilli()
	}

	cov.MinTimestampUTC = time.UnixMilli(minTime).UTC()
	cov.MaxTimestampUTC = time.UnixMilli(maxTime).UTC()

	if minTime > maxTime {
		return cov
	}

	expectedCount := (maxTime-minTime)/intervalMS + 1
	cov.ExpectedRowCount = expectedCount

	var gapCount int64
	var largestGap int64
	var dupCount int64
	var missingCount int64

	prev := minTime - intervalMS // setup for first loop
	for _, t := range allTimes {
		if t < minTime || t > maxTime {
			continue
		}
		diff := t - prev
		if diff == 0 {
			dupCount++
		} else if diff > intervalMS {
			gapCount++
			gapSec := (diff - intervalMS) / 1000
			if gapSec > largestGap {
				largestGap = gapSec
			}
			missingCount += (diff / intervalMS) - 1
		}
		prev = t
	}

	cov.GapCount = gapCount
	cov.LargestGapSeconds = largestGap
	cov.DuplicateTimestampCount = dupCount
	cov.MissingRowCount = missingCount

	if expectedCount > 0 {
		// Coverage is rowcount (excluding dups) vs expected.
		validCount := cov.RowCount - dupCount
		if validCount > expectedCount {
			validCount = expectedCount
		}
		cov.CoveragePct = float64(validCount) / float64(expectedCount) * 100.0
	} else {
		cov.CoveragePct = 100.0
	}

	if cov.GapCount > 0 {
		cov.Warnings = append(cov.Warnings, WarnGapsDetected)
	}
	if cov.DuplicateTimestampCount > 0 {
		cov.Warnings = append(cov.Warnings, WarnDuplicateTimestamps)
	}
	if cov.CoveragePct < opts.MinPct {
		cov.Warnings = append(cov.Warnings, WarnCoverageBelowThreshold)
		cov.Status = CoverageStatusFail
	} else if len(cov.Warnings) > 0 {
		cov.Status = CoverageStatusWarn
	}

	return cov
}

func parseIntervalMS(interval string) (int64, error) {
	if len(interval) < 2 {
		return 0, fmt.Errorf("invalid interval format")
	}
	unit := interval[len(interval)-1]
	val, err := strconv.ParseInt(interval[:len(interval)-1], 10, 64)
	if err != nil {
		return 0, err
	}
	switch unit {
	case 'm':
		return val * 60 * 1000, nil
	case 'h':
		return val * 60 * 60 * 1000, nil
	case 'd':
		return val * 24 * 60 * 60 * 1000, nil
	case 'w':
		return val * 7 * 24 * 60 * 60 * 1000, nil
	default:
		return 0, fmt.Errorf("unknown interval unit: %c", unit)
	}
}
