package atomicfile

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

type fileOps struct {
	createTemp func(string, string) (*os.File, error)
	rename     func(string, string) error
	openDir    func(string) (*os.File, error)
}

var systemOps = fileOps{createTemp: os.CreateTemp, rename: os.Rename, openDir: os.Open}

// WriteFile durably replaces path using a temporary file in the same
// directory, file sync, atomic rename, and parent-directory sync.
func WriteFile(path string, data []byte, perm os.FileMode) error {
	return writeFile(path, data, perm, systemOps)
}

func writeFile(path string, data []byte, perm os.FileMode, ops fileOps) (err error) {
	if path == "" {
		return fmt.Errorf("atomic write path is required")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	tmp, err := ops.createTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()
	if err := tmp.Chmod(perm); err != nil {
		return fmt.Errorf("set temporary permissions: %w", err)
	}
	if _, err := tmp.ReadFrom(bytes.NewReader(data)); err != nil {
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := ops.rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temporary file: %w", err)
	}
	d, err := ops.openDir(dir)
	if err != nil {
		return fmt.Errorf("open parent directory: %w", err)
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("sync parent directory: %w", err)
	}
	return nil
}
