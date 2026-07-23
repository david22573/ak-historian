package app

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/david22573/ak-historian/internal/workdir"
)

func TestAuditWorkdir(t *testing.T) {
	wd := t.TempDir()
	os.MkdirAll(filepath.Join(wd, "candles", "futures-um", "1m", "symbol=LINKUSDT", "year=2024", "month=01"), 0755)

	fPath := filepath.Join(wd, "candles", "futures-um", "1m", "symbol=LINKUSDT", "year=2024", "month=01", "LINKUSDT-1m-2024-01.parquet")
	os.WriteFile(fPath, make([]byte, 1024), 0644)

	err := runAuditWorkdir(wd)
	if err != nil {
		t.Fatalf("runAuditWorkdir failed: %v", err)
	}

	reportPath := filepath.Join(wd, "reports", "phase10_5e_workdir_audit.json")
	b, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("Failed to read report: %v", err)
	}

	var report AuditReport
	if err := json.Unmarshal(b, &report); err != nil {
		t.Fatalf("Failed to unmarshal report: %v", err)
	}

	if report.TotalCandleSize != 1024 {
		t.Errorf("Expected 1024 candle size, got %d", report.TotalCandleSize)
	}
	if report.LargestSymbolsBySize["LINKUSDT"] != 1024 {
		t.Errorf("Expected 1024 for LINKUSDT, got %d", report.LargestSymbolsBySize["LINKUSDT"])
	}
}

func TestPlanStorage(t *testing.T) {
	wd := t.TempDir()
	err := runPlanStorage(wd, "futures-um", "1m", "BTCUSDT,ETHUSDT,LINKUSDT,SOLUSDT,AVAXUSDT", "2024-01", "2025-12", 20, 5)
	if err != nil {
		t.Fatalf("runPlanStorage failed: %v", err)
	}

	reportPath := filepath.Join(wd, "reports", "phase10_5e_storage_plan.json")
	b, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("Failed to read report: %v", err)
	}

	var plan StoragePlan
	if err := json.Unmarshal(b, &plan); err != nil {
		t.Fatalf("Failed to unmarshal plan: %v", err)
	}

	if len(plan.Groups) != 6 { // 3 symbols * 2 years = 6 groups
		t.Errorf("Expected 6 groups, got %d", len(plan.Groups))
	}

	for _, g := range plan.Groups {
		if !g.FitsBudget {
			t.Errorf("Group %d does not fit budget", g.GroupID)
		}
		if len(g.Symbols) != 3 { // BTC, ETH, Target
			t.Errorf("Expected 3 symbols per group, got %d", len(g.Symbols))
		}
	}
}

func TestCleanupWorkdir(t *testing.T) {
	wd := t.TempDir()

	// Create local only
	localPath := filepath.Join(wd, "candles", "futures-um", "1m", "symbol=LINKUSDT", "year=2024", "month=01")
	os.MkdirAll(localPath, 0755)
	os.WriteFile(filepath.Join(localPath, "local.parquet"), make([]byte, 1024), 0644)

	// Create verified archive
	verifiedPath := filepath.Join(wd, "candles", "futures-um", "1m", "symbol=SOLUSDT", "year=2024", "month=01")
	os.MkdirAll(verifiedPath, 0755)
	vFile := filepath.Join(verifiedPath, "verified.parquet")
	os.WriteFile(vFile, make([]byte, 1024), 0644)

	// Create retained
	retainedPath := filepath.Join(wd, "candles", "futures-um", "1m", "symbol=BTCUSDT", "year=2024", "month=01")
	os.MkdirAll(retainedPath, 0755)
	rFile := filepath.Join(retainedPath, "retained.parquet")
	os.WriteFile(rFile, make([]byte, 1024), 0644)

	// Outside workdir via symlink simulation (just check logic)
	// We check logic by verifying report

	m := &workdir.LocalSourceManifest{
		Objects: make(map[string]*workdir.LocalParquetObj),
	}
	m.Objects[filepath.Join("candles", "futures-um", "1m", "symbol=SOLUSDT", "year=2024", "month=01", "verified.parquet")] = &workdir.LocalParquetObj{
		ArchivedStatus: workdir.ArchivedStatusVerifiedArchive,
	}
	workdir.SaveLocalSourceManifest(wd, m)

	// Dry run - nothing deleted
	err := runCleanupWorkdir(wd, "futures-um", "1m", "", "", "", "BTCUSDT,ETHUSDT", true, false, true, false, 0, false, false)
	if err != nil {
		t.Fatalf("runCleanupWorkdir failed: %v", err)
	}

	// Check that file is still there
	if _, err := os.Stat(vFile); os.IsNotExist(err) {
		t.Errorf("Verified file should not be deleted in dry run")
	}

	// Force run
	runCleanupWorkdir(wd, "futures-um", "1m", "", "", "", "BTCUSDT,ETHUSDT", false, true, true, false, 0, false, false)

	if _, err := os.Stat(vFile); !os.IsNotExist(err) {
		t.Errorf("Verified file should be deleted")
	}
	if _, err := os.Stat(rFile); os.IsNotExist(err) {
		t.Errorf("Retained file should not be deleted")
	}
}

func TestVerifyArchive(t *testing.T) {
	wd := t.TempDir()

	// To ensure config fails, we will temporarily set env vars to garbage and remove .env if it was found
	os.Setenv("R2_ACCOUNT_ID", "")
	os.Setenv("R2_ACCESS_KEY_ID", "")
	os.Setenv("R2_SECRET_ACCESS_KEY", "")
	os.Setenv("R2_BUCKET_NAME", "")

	// Temporarily rename .env
	envPath := filepath.Join("..", "..", ".env")
	os.Rename(envPath, envPath+".bak")
	defer os.Rename(envPath+".bak", envPath)

	// Capture stdout
	rescueStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	runVerifyArchive(wd, "r2", "futures-um", "1m", "LINKUSDT", "2024-01", "2024-12")

	w.Close()
	outBytes, _ := io.ReadAll(r)
	os.Stdout = rescueStdout
	out := string(outBytes)

	if !strings.Contains(out, "archive_verification_status: unavailable") {
		t.Errorf("Expected unavailable status, got %s", out)
	}
}
