package binance

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ExtractExpectedCSV(zipPath string, expectedCSVName string, destDir string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("failed to open zip: %w", err)
	}
	defer r.Close()

	var targetFile *zip.File
	for _, f := range r.File {
		if f.Name == expectedCSVName {
			targetFile = f
			break
		}
	}

	if targetFile == nil {
		return "", fmt.Errorf("expected CSV %s not found in zip", expectedCSVName)
	}

	// Security check: path traversal
	// Clean and check target name
	cleanTargetName := filepath.Clean(targetFile.Name)
	if filepath.IsAbs(cleanTargetName) || strings.HasPrefix(cleanTargetName, ".."+string(filepath.Separator)) || cleanTargetName == ".." {
		return "", fmt.Errorf("unsafe file name in zip: %s", targetFile.Name)
	}

	destPath := filepath.Join(destDir, cleanTargetName)

	// Ensure destPath is within destDir using filepath.Rel
	rel, err := filepath.Rel(destDir, destPath)
	if err != nil {
		return "", fmt.Errorf("failed to calculate relative path: %w", err)
	}
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", fmt.Errorf("unsafe extraction path (traversal): %s", rel)
	}

	err = os.MkdirAll(filepath.Dir(destPath), 0755)
	if err != nil {
		return "", err
	}

	rc, err := targetFile.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	out, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	if err != nil {
		return "", err
	}

	return destPath, nil
}
