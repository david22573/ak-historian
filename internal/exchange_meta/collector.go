package exchange_meta

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CollectOptions struct {
	Exchange         string
	MarketType       string
	QuoteAssetFilter string
	ArchiveRoot      string
	SourceType       string
	SourceName       string
	SourceURI        string
	DryRun           bool
	RefreshManifest  bool
	WriteRaw         bool
	AllowNetwork     bool
	RawJSONPath      string
	BaseURL          string
}

type CollectReport struct {
	CollectedAtUTC            string   `json:"collected_at_utc"`
	Exchange                  string   `json:"exchange"`
	MarketType                string   `json:"market_type"`
	RawPayloadSHA256          string   `json:"raw_payload_sha256"`
	SnapshotHash              string   `json:"snapshot_hash"`
	SymbolCount               int      `json:"symbol_count"`
	DuplicateSnapshotDetected bool     `json:"duplicate_snapshot_detected"`
	FilesWritten              []string `json:"files_written"`
	ManifestRefreshed         bool     `json:"manifest_refreshed"`
	Warnings                  []string `json:"warnings"`
}

func CollectArchive(ctx context.Context, opts CollectOptions) (*CollectReport, error) {
	if opts.ArchiveRoot == "" {
		return nil, fmt.Errorf("missing archive root")
	}

	report := &CollectReport{
		Exchange:     normalizeDefault(strings.ToLower(opts.Exchange), "binance"),
		MarketType:   normalizeDefault(strings.ToLower(opts.MarketType), "futures_um"),
		FilesWritten: []string{},
		Warnings:     []string{},
	}

	var raw []byte
	var err error

	sourceType := normalizeDefault(opts.SourceType, "public_endpoint_current")
	sourceName := normalizeDefault(opts.SourceName, "binance_futures_exchangeInfo_v1")
	sourceURI := opts.SourceURI

	if opts.RawJSONPath != "" {
		raw, err = os.ReadFile(opts.RawJSONPath)
		if err != nil {
			return nil, fmt.Errorf("read raw json: %w", err)
		}
		sourceType = "file_import_current"
		if sourceURI == "" {
			sourceURI = filepath.ToSlash(opts.RawJSONPath)
		}
	} else if opts.AllowNetwork {
		raw, sourceURI, err = FetchBinanceFuturesExchangeInfo(ctx, nil, opts.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("fetch network: %w", err)
		}
	} else {
		return nil, fmt.Errorf("network not allowed and no raw json provided")
	}

	if len(raw) > 0 {
		sum := sha256.Sum256(raw)
		report.RawPayloadSHA256 = hex.EncodeToString(sum[:])
	}

	collectedAt := time.Now().UTC()
	collectedAtStr := collectedAt.Format(time.RFC3339)
	report.CollectedAtUTC = collectedAtStr

	snapshot, err := BuildSnapshot(SnapshotOptions{
		Exchange:         report.Exchange,
		MarketType:       report.MarketType,
		QuoteAssetFilter: opts.QuoteAssetFilter,
		SourceType:       sourceType,
		SourceName:       sourceName,
		SourceURI:        sourceURI,
		CollectedAtUTC:   collectedAtStr,
		RawPayload:       raw,
	})
	if err != nil {
		return nil, fmt.Errorf("build snapshot: %w", err)
	}

	report.SnapshotHash = snapshot.Hashes.SnapshotHash
	report.SymbolCount = len(snapshot.Symbols)

	// Archive Layout Paths
	year := collectedAt.Format("2006")
	month := collectedAt.Format("01")
	day := collectedAt.Format("02")

	baseDir := filepath.Join(opts.ArchiveRoot, report.Exchange, report.MarketType)
	snapshotsDir := filepath.Join(baseDir, "snapshots", year, month, day)
	rawDir := filepath.Join(baseDir, "raw", year, month, day)
	manifestsDir := filepath.Join(baseDir, "manifests")
	latestDir := filepath.Join(baseDir, "latest")

	snapshotFileName := fmt.Sprintf("exchange_metadata_snapshot_%s.json", snapshot.SnapshotID)
	snapshotPath := filepath.Join(snapshotsDir, snapshotFileName)

	var rawPath string
	if report.RawPayloadSHA256 != "" {
		timestamp := strings.NewReplacer("-", "", ":", "", "T", "_", "Z", "").Replace(collectedAtStr)
		rawFileName := fmt.Sprintf("exchangeInfo_%s_%s.json", timestamp, report.RawPayloadSHA256)
		rawPath = filepath.Join(rawDir, rawFileName)
	}

	// Dedupe check
	// Check if a snapshot with the exact same snapshot_hash exists anywhere in the archive
	// Wait, we can scan the archive, or just rely on the existing manifest if there is one.
	// Actually, an easier dedupe check without full scan is: if there is an existing manifest, check its Snapshots.
	manifestPath := filepath.Join(manifestsDir, "exchange_metadata_snapshot_manifest.json")
	var existingManifest *SnapshotManifest
	if data, err := os.ReadFile(manifestPath); err == nil {
		var m SnapshotManifest
		if json.Unmarshal(data, &m) == nil {
			existingManifest = &m
		}
	}

	if existingManifest != nil {
		for _, s := range existingManifest.Snapshots {
			if s.SnapshotHash == report.SnapshotHash {
				report.DuplicateSnapshotDetected = true
				break
			}
		}
	} else {
		// If no manifest exists, check if the exact file exists at the expected path (not perfectly reliable across days, but okay for same-day dedupe)
		if _, err := os.Stat(snapshotPath); err == nil {
			// Actually we'd need to parse it to be 100% sure it's the same hash, but the filename has the snapshot ID which includes a truncated hash.
			report.DuplicateSnapshotDetected = true
		}
	}

	if !report.DuplicateSnapshotDetected {
		if !opts.DryRun {
			if err := WriteSnapshot(snapshotPath, snapshot); err != nil {
				return nil, fmt.Errorf("write snapshot: %w", err)
			}
			report.FilesWritten = append(report.FilesWritten, snapshotPath)

			if opts.WriteRaw && rawPath != "" {
				if err := os.MkdirAll(filepath.Dir(rawPath), 0755); err != nil {
					return nil, err
				}
				if err := os.WriteFile(rawPath, raw, 0644); err != nil {
					return nil, fmt.Errorf("write raw payload: %w", err)
				}
				report.FilesWritten = append(report.FilesWritten, rawPath)
			}

			// update latest copy
			if err := os.MkdirAll(latestDir, 0755); err == nil {
				latestSnapPath := filepath.Join(latestDir, "latest_snapshot.json")
				os.WriteFile(latestSnapPath, raw, 0644) // Actually write snapshot here
				WriteSnapshot(latestSnapPath, snapshot)
			}
		} else {
			report.FilesWritten = append(report.FilesWritten, "(dry-run) "+snapshotPath)
			if opts.WriteRaw && rawPath != "" {
				report.FilesWritten = append(report.FilesWritten, "(dry-run) "+rawPath)
			}
		}
	} else {
		report.Warnings = append(report.Warnings, "EXCHANGE_ARCHIVE_DUPLICATE_SNAPSHOT")
	}

	if opts.RefreshManifest && !opts.DryRun {
		mOpts := ManifestOptions{
			SnapshotDir: filepath.Join(baseDir, "snapshots"),
			BaseDir:     filepath.Join(baseDir, "manifests"),
			ArchiveID:   "default_exchange_metadata_archive",
			Exchange:    report.Exchange,
			MarketType:  report.MarketType,
		}
		m, err := BuildSnapshotManifest(mOpts)
		if err != nil {
			return nil, fmt.Errorf("build manifest: %w", err)
		}
		if err := WriteSnapshotManifest(manifestPath, m); err != nil {
			return nil, fmt.Errorf("write manifest: %w", err)
		}
		report.FilesWritten = append(report.FilesWritten, manifestPath)
		report.ManifestRefreshed = true

		// update latest manifest copy
		if err := os.MkdirAll(latestDir, 0755); err == nil {
			latestManPath := filepath.Join(latestDir, "latest_manifest.json")
			WriteSnapshotManifest(latestManPath, m)
		}
	} else if opts.RefreshManifest && opts.DryRun {
		report.ManifestRefreshed = true
		report.FilesWritten = append(report.FilesWritten, "(dry-run) "+manifestPath)
	}

	return report, nil
}
