package pitarchive

import (
	"fmt"
	"os"
	"path/filepath"
)

func writeAtomic(path string, data []byte, maxBytes int64) error {
	if path == "" {
		return fmt.Errorf("output path is required")
	}
	if maxBytes <= 0 || int64(len(data)) > maxBytes {
		return fmt.Errorf("artifact size %d exceeds limit %d", len(data), maxBytes)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create artifact directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create artifact temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write artifact temporary file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync artifact temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close artifact temporary file: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return fmt.Errorf("set artifact permissions: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace artifact: %w", err)
	}
	removeTemp = false

	dirFile, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open artifact directory for sync: %w", err)
	}
	if err := dirFile.Sync(); err != nil {
		_ = dirFile.Close()
		return fmt.Errorf("sync artifact directory: %w", err)
	}
	if err := dirFile.Close(); err != nil {
		return fmt.Errorf("close artifact directory: %w", err)
	}
	return nil
}
