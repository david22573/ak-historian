package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeRestoreStore struct {
	objects   map[string][]byte
	exists    []string
	downloads []string
}

func (f *fakeRestoreStore) ObjectExists(_ context.Context, key string) (bool, error) {
	f.exists = append(f.exists, key)
	_, ok := f.objects[key]
	return ok, nil
}

func (f *fakeRestoreStore) DownloadFile(_ context.Context, objectKey string, localPath string) error {
	f.downloads = append(f.downloads, objectKey)
	body, ok := f.objects[objectKey]
	if !ok {
		return fmt.Errorf("missing object")
	}
	return os.WriteFile(localPath, body, 0644)
}

func baseRestoreOpts(t *testing.T) R2CandleRestoreOptions {
	t.Helper()
	return R2CandleRestoreOptions{
		Market:      "futures-um",
		Interval:    "1m",
		Symbols:     []string{"AVAXUSDT"},
		Start:       "2024-01",
		End:         "2024-01",
		Out:         filepath.Join(t.TempDir(), "out"),
		DryRun:      false,
		Overwrite:   false,
		Verify:      true,
		Concurrency: 1,
		ReportDir:   filepath.Join(t.TempDir(), "reports"),
		CommandsRun: []string{"ak-historian restore-r2-candles --dry-run false"},
	}
}

func readRestoreReport(t *testing.T, dir string) R2CandleRestoreReport {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "r2_restore_candles.json"))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var report R2CandleRestoreReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	return report
}

func TestPrepareR2CandleRestoreResolvesKeys(t *testing.T) {
	opts := baseRestoreOpts(t)
	opts.Symbols = []string{"avaxusdt", " SOLUSDT "}
	opts.Start = "2024-01"
	opts.End = "2024-02"

	got, months, jobs, err := prepareR2CandleRestore(opts)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if strings.Join(got.Symbols, ",") != "AVAXUSDT,SOLUSDT" {
		t.Fatalf("symbols = %v", got.Symbols)
	}
	if strings.Join(months, ",") != "2024-01,2024-02" {
		t.Fatalf("months = %v", months)
	}
	if len(jobs) != 4 {
		t.Fatalf("jobs = %d", len(jobs))
	}
	wantKey := "candles/futures-um/1m/symbol=AVAXUSDT/year=2024/month=01/AVAXUSDT-1m-2024-01.parquet"
	if jobs[0].ObjectKey != wantKey {
		t.Fatalf("key = %s", jobs[0].ObjectKey)
	}
	if jobs[0].LocalPath != filepath.Join(opts.Out, filepath.FromSlash(wantKey)) {
		t.Fatalf("local path = %s", jobs[0].LocalPath)
	}
}

func TestRestoreR2CandlesDryRunDoesNotDownload(t *testing.T) {
	opts := baseRestoreOpts(t)
	opts.DryRun = true
	store := &fakeRestoreStore{objects: map[string][]byte{
		"candles/futures-um/1m/symbol=AVAXUSDT/year=2024/month=01/AVAXUSDT-1m-2024-01.parquet": []byte("parquet"),
	}}

	if err := runRestoreR2Candles(context.Background(), opts, store, nil); err != nil {
		t.Fatalf("restore dry-run: %v", err)
	}
	if len(store.downloads) != 0 {
		t.Fatalf("dry run downloaded: %v", store.downloads)
	}
	report := readRestoreReport(t, opts.ReportDir)
	if report.FinalLabel != "dry_run_complete" {
		t.Fatalf("label = %s", report.FinalLabel)
	}
	if report.ObjectsExpected != 1 || report.ObjectsFound != 1 {
		t.Fatalf("counts expected/found = %d/%d", report.ObjectsExpected, report.ObjectsFound)
	}
}

func TestRestoreR2CandlesSkipsExistingWithoutOverwrite(t *testing.T) {
	opts := baseRestoreOpts(t)
	key := "candles/futures-um/1m/symbol=AVAXUSDT/year=2024/month=01/AVAXUSDT-1m-2024-01.parquet"
	localPath := filepath.Join(opts.Out, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(localPath, []byte("local"), 0644); err != nil {
		t.Fatal(err)
	}
	store := &fakeRestoreStore{objects: map[string][]byte{key: []byte("remote")}}

	if err := runRestoreR2Candles(context.Background(), opts, store, func(context.Context, string) error { return nil }); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if len(store.downloads) != 0 {
		t.Fatalf("downloaded despite existing local file: %v", store.downloads)
	}
	body, _ := os.ReadFile(localPath)
	if string(body) != "local" {
		t.Fatalf("local file overwritten: %q", string(body))
	}
	report := readRestoreReport(t, opts.ReportDir)
	if len(report.FilesSkippedExisting) != 1 {
		t.Fatalf("skipped existing = %v", report.FilesSkippedExisting)
	}
}

func TestRestoreR2CandlesOverwriteTrueReplacesExisting(t *testing.T) {
	opts := baseRestoreOpts(t)
	opts.Overwrite = true
	key := "candles/futures-um/1m/symbol=AVAXUSDT/year=2024/month=01/AVAXUSDT-1m-2024-01.parquet"
	localPath := filepath.Join(opts.Out, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(localPath, []byte("local"), 0644); err != nil {
		t.Fatal(err)
	}
	store := &fakeRestoreStore{objects: map[string][]byte{key: []byte("remote")}}

	if err := runRestoreR2Candles(context.Background(), opts, store, func(context.Context, string) error { return nil }); err != nil {
		t.Fatalf("restore: %v", err)
	}
	body, _ := os.ReadFile(localPath)
	if string(body) != "remote" {
		t.Fatalf("local file not overwritten: %q", string(body))
	}
}

func TestRestoreR2CandlesReportsMissingR2Object(t *testing.T) {
	opts := baseRestoreOpts(t)
	store := &fakeRestoreStore{objects: map[string][]byte{}}

	err := runRestoreR2Candles(context.Background(), opts, store, func(context.Context, string) error { return nil })
	if err == nil {
		t.Fatalf("expected missing object error")
	}
	report := readRestoreReport(t, opts.ReportDir)
	if report.FinalLabel != "blocked_missing_r2_objects" {
		t.Fatalf("label = %s", report.FinalLabel)
	}
	if len(report.ObjectsMissing) != 1 {
		t.Fatalf("missing = %v", report.ObjectsMissing)
	}
}

func TestRestoreR2CandlesReportGeneration(t *testing.T) {
	opts := baseRestoreOpts(t)
	opts.Verify = false
	key := "candles/futures-um/1m/symbol=AVAXUSDT/year=2024/month=01/AVAXUSDT-1m-2024-01.parquet"
	store := &fakeRestoreStore{objects: map[string][]byte{key: []byte("remote")}}

	if err := runRestoreR2Candles(context.Background(), opts, store, nil); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if _, err := os.Stat(filepath.Join(opts.ReportDir, "r2_restore_candles.md")); err != nil {
		t.Fatalf("markdown report missing: %v", err)
	}
	report := readRestoreReport(t, opts.ReportDir)
	if report.FinalLabel != "restore_complete" {
		t.Fatalf("label = %s", report.FinalLabel)
	}
	if len(report.FilesRestored) != 1 {
		t.Fatalf("restored = %v", report.FilesRestored)
	}
}

func TestRestoreR2CandlesValidationFailureCleansTempFile(t *testing.T) {
	opts := baseRestoreOpts(t)
	key := "candles/futures-um/1m/symbol=AVAXUSDT/year=2024/month=01/AVAXUSDT-1m-2024-01.parquet"
	store := &fakeRestoreStore{objects: map[string][]byte{key: []byte("not parquet")}}

	err := runRestoreR2Candles(context.Background(), opts, store, func(context.Context, string) error {
		return fmt.Errorf("bad parquet")
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	report := readRestoreReport(t, opts.ReportDir)
	if report.FinalLabel != "blocked_validation_failure" {
		t.Fatalf("label = %s", report.FinalLabel)
	}
	localPath := filepath.Join(opts.Out, filepath.FromSlash(key))
	if _, err := os.Stat(localPath); !os.IsNotExist(err) {
		t.Fatalf("final file should not exist after validation failure")
	}
	leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(localPath), ".*.tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(leftovers) != 0 {
		t.Fatalf("temp files left behind: %v", leftovers)
	}
}
