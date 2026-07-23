package workdir

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type ArchivedStatus string

const (
	ArchivedStatusUnknown          ArchivedStatus = "unknown"
	ArchivedStatusLocalOnly        ArchivedStatus = "local_only"
	ArchivedStatusArchivedR2       ArchivedStatus = "archived_r2"
	ArchivedStatusArchivedExternal ArchivedStatus = "archived_external"
	ArchivedStatusVerifiedArchive  ArchivedStatus = "verified_archive"
)

type LocalParquetObj struct {
	Market          string         `json:"market"`
	Interval        string         `json:"interval"`
	Symbol          string         `json:"symbol"`
	Year            string         `json:"year"`
	Month           string         `json:"month"`
	Path            string         `json:"path"`
	SizeBytes       int64          `json:"size_bytes"`
	RowCountIfKnown int64          `json:"row_count_if_known"`
	ChecksumIfAvail string         `json:"checksum_if_available"`
	CoverageStatus  string         `json:"coverage_status"`
	ArchivedStatus  ArchivedStatus `json:"archived_status"`
	ArchiveLocation string         `json:"archive_location"`
	LastVerifiedAt  time.Time      `json:"last_verified_at"`
	SafeToDelete    bool           `json:"safe_to_delete"`
}

type LocalSourceManifest struct {
	Objects map[string]*LocalParquetObj `json:"objects"`
}

var manifestMu sync.Mutex

func LoadLocalSourceManifest(workdir string) (*LocalSourceManifest, error) {
	manifestMu.Lock()
	defer manifestMu.Unlock()

	manifestDir := filepath.Join(workdir, "manifests")
	err := os.MkdirAll(manifestDir, 0755)
	if err != nil {
		return nil, err
	}

	manifestPath := filepath.Join(manifestDir, "local_source_manifest.json")

	m := &LocalSourceManifest{
		Objects: make(map[string]*LocalParquetObj),
	}

	b, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return m, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(b, m); err != nil {
		return nil, err
	}
	if m.Objects == nil {
		m.Objects = make(map[string]*LocalParquetObj)
	}

	return m, nil
}

func SaveLocalSourceManifest(workdir string, m *LocalSourceManifest) error {
	manifestMu.Lock()
	defer manifestMu.Unlock()

	manifestDir := filepath.Join(workdir, "manifests")
	err := os.MkdirAll(manifestDir, 0755)
	if err != nil {
		return err
	}

	for _, obj := range m.Objects {
		obj.SafeToDelete = (obj.ArchivedStatus == ArchivedStatusVerifiedArchive)
	}

	manifestPath := filepath.Join(manifestDir, "local_source_manifest.json")
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(manifestPath, b, 0644)
}
