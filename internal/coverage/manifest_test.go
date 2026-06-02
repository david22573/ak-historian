package coverage

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestManifestJSONIncludesSchemaVersion(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: 1,
		Market:        "futures-um",
		Symbol:        "LINKUSDT",
		Interval:      "1m",
		CoverageStart: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		CoverageEnd:   time.Date(2023, 1, 1, 0, 1, 0, 0, time.UTC),
		ObjectCount:   1,
		Status:        StatusPass,
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal manifest: %v", err)
	}

	if string(data) == "" || !json.Valid(data) {
		t.Fatalf("manifest JSON invalid: %s", string(data))
	}
	if string(data) != "" && !strings.Contains(string(data), `"schema_version":1`) {
		t.Fatalf("schema_version missing: %s", string(data))
	}
}

func TestManifestKey(t *testing.T) {
	got := ManifestKey("futures-um", "1m", "LINKUSDT")
	want := "manifests/futures-um/1m/symbol=LINKUSDT/manifest.json"
	if got != want {
		t.Fatalf("ManifestKey = %s, want %s", got, want)
	}
}

func TestValidateSymbol(t *testing.T) {
	if err := ValidateSymbol("LINKUSDT"); err != nil {
		t.Fatalf("ValidateSymbol valid: %v", err)
	}
	if err := ValidateSymbol("link/usdt"); err == nil {
		t.Fatal("ValidateSymbol should reject invalid symbol")
	}
}
