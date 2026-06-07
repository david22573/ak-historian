package datasets

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Manifest struct {
	SchemaVersion    int      `json:"schema_version"`
	Kind             string   `json:"kind"`
	Source           string   `json:"source"`
	Dataset          string   `json:"dataset"`
	Market           string   `json:"market,omitempty"`
	Symbol           string   `json:"symbol,omitempty"`
	Scope            string   `json:"scope,omitempty"`
	Interval         string   `json:"interval"`
	CoverageStartMS  int64    `json:"coverage_start_ms"`
	CoverageEndMS    int64    `json:"coverage_end_ms"`
	ObjectCount      int      `json:"object_count"`
	Objects          []Object `json:"objects"`
	LastVerifiedAtMS int64    `json:"last_verified_at_ms"`
	Status           string   `json:"status"`
}

type Object struct {
	Key              string `json:"key"`
	Period           string `json:"period"`
	RowCount         int64  `json:"row_count"`
	MinEventTimeMS   int64  `json:"min_event_time_ms"`
	MaxEventTimeMS   int64  `json:"max_event_time_ms"`
	MinAvailableAtMS int64  `json:"min_available_at_ms"`
	MaxAvailableAtMS int64  `json:"max_available_at_ms"`
}

func WriteManifest(path string, m Manifest) error {
	m.Status = "PASS"

	if len(m.Objects) == 0 {
		m.Status = "FAIL"
	}

	seenKeys := make(map[string]bool)
	for _, obj := range m.Objects {
		if obj.RowCount <= 0 {
			m.Status = "FAIL"
		}
		if obj.MinEventTimeMS <= 0 || obj.MaxEventTimeMS <= 0 || obj.MinEventTimeMS > obj.MaxEventTimeMS {
			m.Status = "FAIL"
		}
		if obj.MinAvailableAtMS <= 0 || obj.MaxAvailableAtMS <= 0 || obj.MinAvailableAtMS > obj.MaxAvailableAtMS {
			m.Status = "FAIL"
		}
		if seenKeys[obj.Key] {
			m.Status = "FAIL"
		}
		seenKeys[obj.Key] = true
	}

	if m.CoverageStartMS <= 0 || m.CoverageEndMS <= 0 || m.CoverageStartMS > m.CoverageEndMS {
		m.Status = "FAIL"
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create manifest directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write manifest file: %w", err)
	}

	return nil
}
