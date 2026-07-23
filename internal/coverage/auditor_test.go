package coverage_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/david22573/ak-historian/internal/coverage"
	"github.com/david22573/ak-historian/internal/parquetutil"
	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/writer"
)

func writeFixture(t *testing.T, path string, times []int64) {
	fw, err := local.NewLocalFileWriter(path)
	if err != nil {
		t.Fatalf("failed to create file writer: %v", err)
	}
	defer fw.Close()

	pw, err := writer.NewParquetWriter(fw, new(parquetutil.OpenTimeRow), 4)
	if err != nil {
		t.Fatalf("failed to create parquet writer: %v", err)
	}
	defer pw.WriteStop()

	for _, ts := range times {
		err := pw.Write(parquetutil.OpenTimeRow{OpenTimeMS: ts})
		if err != nil {
			t.Fatalf("failed to write row: %v", err)
		}
	}
}

func TestAuditor_PerfectContinuous(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "BTCUSDT-1m-2024-01-01.parquet")

	var times []int64
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	for i := 0; i < 60; i++ {
		times = append(times, start+int64(i)*60000)
	}
	writeFixture(t, path, times)

	res, err := coverage.AuditFiles([]string{path}, coverage.AuditorOptions{
		Mode:         "strict",
		SymbolHint:   "BTCUSDT",
		IntervalHint: "1m",
	})
	if err != nil {
		t.Fatalf("audit failed: %v", err)
	}

	if res.Status != coverage.CoverageStatusPass {
		t.Errorf("expected PASS, got %s", res.Status)
	}
	if res.TotalRows != 60 {
		t.Errorf("expected 60 rows, got %d", res.TotalRows)
	}
	if res.TotalGapCount != 0 {
		t.Errorf("expected 0 gaps, got %d", res.TotalGapCount)
	}
	if res.TotalDuplicateTimestamps != 0 {
		t.Errorf("expected 0 dups, got %d", res.TotalDuplicateTimestamps)
	}
}

func TestAuditor_MissingCandle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "BTCUSDT-1m-2024-01-01.parquet")

	var times []int64
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	for i := 0; i < 60; i++ {
		if i == 30 {
			continue // skip one
		}
		times = append(times, start+int64(i)*60000)
	}
	writeFixture(t, path, times)

	res, err := coverage.AuditFiles([]string{path}, coverage.AuditorOptions{
		Mode:         "strict",
		SymbolHint:   "BTCUSDT",
		IntervalHint: "1m",
		MinPct:       100.0,
	})
	if err != nil {
		t.Fatalf("audit failed: %v", err)
	}

	if res.Status != coverage.CoverageStatusFail {
		t.Errorf("expected FAIL due to strict MinPct, got %s", res.Status)
	}
	if res.TotalGapCount != 1 {
		t.Errorf("expected 1 gap, got %d", res.TotalGapCount)
	}
	if res.TotalMissingRows != 1 {
		t.Errorf("expected 1 missing row, got %d", res.TotalMissingRows)
	}
	hasWarn := false
	for _, w := range res.Symbols[0].Warnings {
		if w == coverage.WarnGapsDetected {
			hasWarn = true
		}
	}
	if !hasWarn {
		t.Errorf("missing DATASET_GAPS_DETECTED warning")
	}
}

func TestAuditor_DuplicateTimestamp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "BTCUSDT-1m-2024-01-01.parquet")

	var times []int64
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	for i := 0; i < 10; i++ {
		times = append(times, start+int64(i)*60000)
	}
	times = append(times, start+int64(5)*60000) // dup
	writeFixture(t, path, times)

	res, err := coverage.AuditFiles([]string{path}, coverage.AuditorOptions{
		Mode:         "strict",
		SymbolHint:   "BTCUSDT",
		IntervalHint: "1m",
	})
	if err != nil {
		t.Fatalf("audit failed: %v", err)
	}

	if res.Status != coverage.CoverageStatusWarn {
		t.Errorf("expected WARN, got %s", res.Status)
	}
	if res.TotalDuplicateTimestamps != 1 {
		t.Errorf("expected 1 dup, got %d", res.TotalDuplicateTimestamps)
	}
	hasWarn := false
	for _, w := range res.Symbols[0].Warnings {
		if w == coverage.WarnDuplicateTimestamps {
			hasWarn = true
		}
	}
	if !hasWarn {
		t.Errorf("missing DATASET_DUPLICATE_TIMESTAMPS warning")
	}
}

func TestAuditor_OutOfOrderTimestamp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "BTCUSDT-1m-2024-01-01.parquet")

	var times []int64
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	times = append(times, start+int64(0)*60000)
	times = append(times, start+int64(2)*60000)
	times = append(times, start+int64(1)*60000) // out of order
	times = append(times, start+int64(3)*60000)
	writeFixture(t, path, times)

	res, err := coverage.AuditFiles([]string{path}, coverage.AuditorOptions{
		Mode:         "strict",
		SymbolHint:   "BTCUSDT",
		IntervalHint: "1m",
	})
	if err != nil {
		t.Fatalf("audit failed: %v", err)
	}

	if res.TotalOutOfOrderTimestamps != 1 {
		t.Errorf("expected 1 out of order, got %d", res.TotalOutOfOrderTimestamps)
	}
}
