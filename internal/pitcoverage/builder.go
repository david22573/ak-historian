package pitcoverage

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/david22573/ak-historian/internal/exchange_meta"
	"github.com/david22573/ak-historian/internal/lifecycle"
	"github.com/david22573/ak-historian/internal/manifest"
	"github.com/david22573/ak-historian/internal/universe"
)

type Builder struct {
	LifecycleManifestPath string
	UniverseManifestPath  string
	DatasetManifestPath   string
	SnapshotManifestPath  string
	ResearchStartUTC      string
	ResearchEndUTC        string
	Strict                bool
	AllowUnverified       bool
}

func (b *Builder) Build() (*Report, error) {
	if b.LifecycleManifestPath == "" {
		return nil, fmt.Errorf("lifecycle manifest path is required")
	}
	if b.UniverseManifestPath == "" {
		return nil, fmt.Errorf("universe manifest path is required")
	}
	if b.ResearchStartUTC == "" || b.ResearchEndUTC == "" {
		return nil, fmt.Errorf("research window start and end are required")
	}

	lm, err := loadLifecycleManifest(b.LifecycleManifestPath)
	if err != nil {
		return nil, err
	}

	um, err := loadUniverseManifest(b.UniverseManifestPath)
	if err != nil {
		return nil, err
	}

	var dm *manifest.DatasetManifest
	if b.DatasetManifestPath != "" {
		dm, err = loadDatasetManifest(b.DatasetManifestPath)
		if err != nil {
			return nil, err
		}
	}

	var sm *exchange_meta.SnapshotManifest
	if b.SnapshotManifestPath != "" {
		sm, err = loadSnapshotManifest(b.SnapshotManifestPath)
		if err != nil {
			return nil, err
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	report := &Report{
		SchemaVersion:          "1.0.0",
		ReportVersion:          "1.0.0",
		CoverageReportID:       fmt.Sprintf("pit_coverage_%s_%s", b.ResearchStartUTC, b.ResearchEndUTC),
		GeneratedAtUTC:         now,
		ResearchWindowStartUTC: b.ResearchStartUTC,
		ResearchWindowEndUTC:   b.ResearchEndUTC,
		UniverseID:             um.UniverseID,
		UniverseHash:           um.Hashes.UniverseHash,
		LifecycleID:            lm.LifecycleID,
		LifecycleHash:          lm.Hashes.LifecycleHash,
		Symbols:                []SymbolEntry{},
		Windows:                []Window{{StartUTC: b.ResearchStartUTC, EndUTC: b.ResearchEndUTC}},
		Validation:             Validation{IsValid: true},
		Warnings:               []Warning{},
	}

	if dm != nil {
		report.DatasetID = dm.DatasetID
		report.DatasetHash = dm.Hashes.DatasetHash
	}
	if sm != nil {
		report.SnapshotArchiveHash = sm.Hashes.ArchiveHash
	}

	err = b.evaluateCoverage(report, lm, um, sm)
	if err != nil {
		return nil, err
	}

	if b.Strict && !report.Validation.IsValid {
		return report, fmt.Errorf("strict mode: validation failed for point-in-time evidence coverage")
	}

	return report, nil
}

func loadLifecycleManifest(path string) (*lifecycle.Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m lifecycle.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func loadUniverseManifest(path string) (*universe.Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m universe.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func loadDatasetManifest(path string) (*manifest.DatasetManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m manifest.DatasetManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func loadSnapshotManifest(path string) (*exchange_meta.SnapshotManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m exchange_meta.SnapshotManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
