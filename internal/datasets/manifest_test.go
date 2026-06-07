package datasets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteManifest_ZeroObjects(t *testing.T) {
	m := Manifest{
		SchemaVersion:   1,
		Kind:            string(KindSentiment),
		Source:          "test",
		Dataset:         "test",
		Interval:        "1d",
		CoverageStartMS: 1000,
		CoverageEndMS:   2000,
		Objects:         []Object{},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	
	err := WriteManifest(path, m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), `"status": "FAIL"`) {
		t.Errorf("expected status FAIL for zero objects")
	}
}

func TestWriteManifest_ValidObjects(t *testing.T) {
	m := Manifest{
		SchemaVersion:   1,
		Kind:            string(KindSentiment),
		Source:          "test",
		Dataset:         "test",
		Interval:        "1d",
		CoverageStartMS: 1000,
		CoverageEndMS:   2000,
		Objects: []Object{
			{
				Key:              "test.parquet",
				RowCount:         100,
				MinEventTimeMS:   1000,
				MaxEventTimeMS:   2000,
				MinAvailableAtMS: 1000,
				MaxAvailableAtMS: 2000,
			},
		},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	
	err := WriteManifest(path, m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), `"status": "PASS"`) {
		t.Errorf("expected status PASS for valid objects")
	}
}
