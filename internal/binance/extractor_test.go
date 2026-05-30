package binance

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractExpectedCSV(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "extractor-test")
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, "test.zip")
	csvName := "test.csv"
	csvContent := "open_time,open,high,low,close"

	// Create zip
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	f, _ := zw.Create(csvName)
	f.Write([]byte(csvContent))
	zw.Close()
	os.WriteFile(zipPath, buf.Bytes(), 0644)

	t.Run("success", func(t *testing.T) {
		got, err := ExtractExpectedCSV(zipPath, csvName, tmpDir)
		if err != nil {
			t.Fatalf("ExtractExpectedCSV() error = %v", err)
		}
		if filepath.Base(got) != csvName {
			t.Errorf("got = %v, want %v", filepath.Base(got), csvName)
		}
		content, _ := os.ReadFile(got)
		if string(content) != csvContent {
			t.Errorf("content = %v, want %v", string(content), csvContent)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := ExtractExpectedCSV(zipPath, "missing.csv", tmpDir)
		if err == nil {
			t.Error("ExtractExpectedCSV() should have failed for missing file")
		}
	})

	t.Run("path traversal cases", func(t *testing.T) {
		traversalNames := []string{
			"../traversal.csv",
			"/absolute/path.csv",
			"nested/../../evil.csv",
		}

		for _, name := range traversalNames {
			buf2 := new(bytes.Buffer)
			zw2 := zip.NewWriter(buf2)
			f2, _ := zw2.Create(name)
			_, _ = f2.Write([]byte("bad"))
			_ = zw2.Close()
			badZipPath := filepath.Join(tmpDir, filepath.Base(name)+".zip")
			_ = os.WriteFile(badZipPath, buf2.Bytes(), 0644)

			_, err := ExtractExpectedCSV(badZipPath, name, tmpDir)
			if err == nil {
				t.Errorf("ExtractExpectedCSV() should have failed for path traversal: %s", name)
			}
		}
	})
}
