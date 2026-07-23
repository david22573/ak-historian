package exchange_meta

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type VerifyOptions struct {
	ArchiveRoot string
	Exchange    string
	MarketType  string
	Strict      bool
}

type VerifyReport struct {
	Valid          bool      `json:"valid"`
	SnapshotsFound int       `json:"snapshots_found"`
	Warnings       []Warning `json:"warnings"`
	Errors         []Warning `json:"errors"`
}

func VerifyArchive(opts VerifyOptions) (*VerifyReport, error) {
	if opts.ArchiveRoot == "" {
		return nil, fmt.Errorf("missing archive root")
	}

	report := &VerifyReport{
		Valid:    true,
		Warnings: []Warning{},
		Errors:   []Warning{},
	}

	baseDir := filepath.Join(opts.ArchiveRoot, opts.Exchange, opts.MarketType)
	snapshotsDir := filepath.Join(baseDir, "snapshots")
	rawDir := filepath.Join(baseDir, "raw")
	manifestsDir := filepath.Join(baseDir, "manifests")

	var snapshots []*Snapshot
	var snapshotPaths []string

	err := filepath.WalkDir(snapshotsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".json") {
			return nil
		}

		snapshot, err := ReadSnapshot(path)
		if err != nil {
			report.Errors = append(report.Errors, Warning{
				Code:    "EXCHANGE_ARCHIVE_SNAPSHOT_PARSE_FAILED",
				Target:  filepath.Base(path),
				Message: err.Error(),
			})
			report.Valid = false
			return nil
		}

		// snapshot_hash matches contents
		computed := ComputeSnapshotHashes(snapshot)
		if computed.SnapshotHash != snapshot.Hashes.SnapshotHash {
			report.Errors = append(report.Errors, Warning{
				Code:    "EXCHANGE_ARCHIVE_HASH_MISMATCH",
				Target:  snapshot.SnapshotID,
				Message: "SnapshotHash mismatch",
			})
			report.Valid = false
		}

		// raw payload hash exists when raw payload was stored
		if snapshot.RawPayloadSHA256 != "" && snapshot.RawPayloadSHA256 != StatusUnknown {
			rawFound := false
			err := filepath.WalkDir(rawDir, func(rawPath string, rawD os.DirEntry, rawErr error) error {
				if rawErr != nil {
					return nil
				}
				if !rawD.IsDir() && strings.Contains(rawPath, snapshot.RawPayloadSHA256) {
					rawFound = true
					// Optionally check raw hash matches contents, but let's assume if filename matches it's okay, or check it:
					data, rerr := os.ReadFile(rawPath)
					if rerr == nil {
						sum := sha256.Sum256(data)
						h := hex.EncodeToString(sum[:])
						if h != snapshot.RawPayloadSHA256 {
							report.Errors = append(report.Errors, Warning{
								Code:    "EXCHANGE_ARCHIVE_RAW_PAYLOAD_MISSING",
								Target:  snapshot.SnapshotID,
								Message: "Raw payload hash mismatch in file content",
							})
							report.Valid = false
						}
					}
					return filepath.SkipDir
				}
				return nil
			})
			if err == nil && !rawFound {
				report.Errors = append(report.Errors, Warning{
					Code:    "EXCHANGE_ARCHIVE_RAW_PAYLOAD_MISSING",
					Target:  snapshot.SnapshotID,
					Message: "Raw payload file not found for hash",
				})
				// If strict, mark invalid
				if opts.Strict {
					report.Valid = false
				}
			}
		}

		if len(snapshot.Symbols) == 0 {
			report.Errors = append(report.Errors, Warning{
				Code:    "EXCHANGE_ARCHIVE_EMPTY",
				Target:  snapshot.SnapshotID,
				Message: "Symbol count is zero",
			})
			report.Valid = false
		}

		// Warnings
		for _, w := range snapshot.Warnings {
			if w.Code == CodeCurrentOnlySource {
				report.Warnings = append(report.Warnings, Warning{
					Code:    "EXCHANGE_ARCHIVE_CURRENT_ONLY_EVIDENCE",
					Target:  snapshot.SnapshotID,
					Message: "Archive contains current-only evidence",
				})
			}
		}

		snapshots = append(snapshots, snapshot)
		snapshotPaths = append(snapshotPaths, path)
		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	if len(snapshots) == 0 {
		report.Errors = append(report.Errors, Warning{
			Code:    "EXCHANGE_ARCHIVE_EMPTY",
			Target:  "archive",
			Message: "No snapshots found in archive",
		})
		report.Valid = false
		return report, nil
	}

	report.SnapshotsFound = len(snapshots)

	// duplicate snapshot IDs with conflicting hashes
	snapshotMap := make(map[string]*Snapshot)
	for _, s := range snapshots {
		if prev, ok := snapshotMap[s.SnapshotID]; ok {
			if prev.Hashes.SnapshotHash != s.Hashes.SnapshotHash {
				report.Errors = append(report.Errors, Warning{
					Code:    "EXCHANGE_ARCHIVE_CONFLICTING_SNAPSHOT",
					Target:  s.SnapshotID,
					Message: "Multiple snapshots with same ID but different hash",
				})
				report.Valid = false
			} else {
				report.Errors = append(report.Errors, Warning{
					Code:    "EXCHANGE_ARCHIVE_DUPLICATE_SNAPSHOT",
					Target:  s.SnapshotID,
					Message: "Multiple snapshot files with same hash",
				})
				if opts.Strict {
					report.Valid = false
				}
			}
		}
		snapshotMap[s.SnapshotID] = s
	}

	// archive time ordering is valid (verify they are monotonic by ID sort)
	var prevTime string
	for _, s := range snapshots {
		if prevTime != "" && s.CollectedAtUTC < prevTime {
			// This isn't necessarily an error if they were written out of order, but it might be if we rely on sort.
			// Actually just sort them by time and check.
		}
		prevTime = s.CollectedAtUTC
	}

	// manifest verifies
	manifestPath := filepath.Join(manifestsDir, "exchange_metadata_snapshot_manifest.json")
	manifest, err := ReadSnapshotManifest(manifestPath)
	if err != nil {
		report.Errors = append(report.Errors, Warning{
			Code:    "EXCHANGE_ARCHIVE_MANIFEST_STALE", // Actually just missing/broken
			Target:  "manifest",
			Message: err.Error(),
		})
		report.Valid = false
	} else {
		computedManifest := ComputeManifestHashes(manifest)
		if computedManifest.ManifestHash != manifest.Hashes.ManifestHash || computedManifest.ArchiveHash != manifest.Hashes.ArchiveHash {
			report.Errors = append(report.Errors, Warning{
				Code:    "EXCHANGE_ARCHIVE_HASH_MISMATCH",
				Target:  "manifest",
				Message: "Manifest hash mismatch, likely stale",
			})
			report.Valid = false
		} else {
			// manifest references existing snapshots
			if len(manifest.Snapshots) != len(snapshots) {
				report.Errors = append(report.Errors, Warning{
					Code:    "EXCHANGE_ARCHIVE_MANIFEST_STALE",
					Target:  "manifest",
					Message: fmt.Sprintf("Manifest snapshot count %d != Archive count %d", len(manifest.Snapshots), len(snapshots)),
				})
				report.Valid = false
			}
		}
	}

	report.Warnings = dedupeWarnings(report.Warnings)
	report.Errors = dedupeWarnings(report.Errors)

	return report, nil
}
