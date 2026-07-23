package exchange_meta

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type IntakeOptions struct {
	InputFiles               []string
	InputDir                 string
	ArchiveRoot              string
	Exchange                 string
	MarketType               string
	QuoteAssetFilter         string
	SourceType               string
	SourceName               string
	SourceURI                string
	TrustLevel               string
	ObservedTime             string
	ObservedTimeFromFilename bool
	FilenameTimeLayout       string
	RefreshManifest          bool
	VerifyArchive            bool
	DryRun                   bool
}

type IntakeInputFile struct {
	InputPathRelative      string  `json:"input_path_relative"`
	InputSHA256            string  `json:"input_sha256"`
	SourceType             string  `json:"source_type"`
	SourceName             string  `json:"source_name"`
	SourceURIOrPath        string  `json:"source_uri_or_path"`
	SourceObservedTimeUTC  *string `json:"source_observed_time_utc"`
	SourceCollectedTimeUTC *string `json:"source_collected_time_utc"`
	TrustLevel             string  `json:"trust_level"`
	Notes                  string  `json:"notes"`
}

type IntakeSnapshot struct {
	SnapshotID            string    `json:"snapshot_id"`
	SnapshotHash          string    `json:"snapshot_hash"`
	SymbolCount           int       `json:"symbol_count"`
	ArchivePath           string    `json:"archive_path"`
	SourceObservedTimeUTC *string   `json:"source_observed_time_utc"`
	TrustLevel            string    `json:"trust_level"`
	Warnings              []Warning `json:"warnings"`
}

type IntakeSkippedFile struct {
	InputPathRelative string `json:"input_path_relative"`
	Reason            string `json:"reason"`
}

type IntakeDuplicateSnapshot struct {
	SnapshotID   string `json:"snapshot_id"`
	SnapshotHash string `json:"snapshot_hash"`
	ArchivePath  string `json:"archive_path"`
}

type IntakeHashes struct {
	IntakeHash              string `json:"intake_hash"`
	SourceBatchHash         string `json:"source_batch_hash"`
	ImportedSnapshotSetHash string `json:"imported_snapshot_set_hash"`
}

type IntakeReport struct {
	SchemaVersion      string                    `json:"schema_version"`
	IntakeID           string                    `json:"intake_id"`
	GeneratedAtUTC     string                    `json:"generated_at_utc"`
	SourceBatchName    string                    `json:"source_batch_name"`
	Exchange           string                    `json:"exchange"`
	MarketType         string                    `json:"market_type"`
	QuoteAssetFilter   string                    `json:"quote_asset_filter"`
	InputFiles         []IntakeInputFile         `json:"input_files"`
	ImportedSnapshots  []IntakeSnapshot          `json:"imported_snapshots"`
	SkippedFiles       []IntakeSkippedFile       `json:"skipped_files"`
	DuplicateSnapshots []IntakeDuplicateSnapshot `json:"duplicate_snapshots"`
	Validation         Validation                `json:"validation"`
	Warnings           []Warning                 `json:"warnings"`
	Hashes             IntakeHashes              `json:"hashes"`
}

func PerformBackfillIntake(opts IntakeOptions) (*IntakeReport, error) {
	if opts.ArchiveRoot == "" {
		return nil, fmt.Errorf("archive root is required")
	}

	report := &IntakeReport{
		SchemaVersion:      "1.0.0",
		GeneratedAtUTC:     time.Now().UTC().Format(time.RFC3339),
		Exchange:           normalizeDefault(opts.Exchange, "binance"),
		MarketType:         normalizeDefault(opts.MarketType, "futures_um"),
		QuoteAssetFilter:   opts.QuoteAssetFilter,
		InputFiles:         []IntakeInputFile{},
		ImportedSnapshots:  []IntakeSnapshot{},
		SkippedFiles:       []IntakeSkippedFile{},
		DuplicateSnapshots: []IntakeDuplicateSnapshot{},
		Validation: Validation{
			IsValid:      true,
			Status:       "VALID",
			WarningCodes: []string{},
		},
		Warnings: []Warning{},
	}

	filesToProcess := append([]string{}, opts.InputFiles...)
	if opts.InputDir != "" {
		err := filepath.WalkDir(opts.InputDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".json") {
				filesToProcess = append(filesToProcess, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to walk input dir: %w", err)
		}
	}

	if len(filesToProcess) == 0 {
		report.Validation.IsValid = false
		report.Validation.Status = "INVALID"
		report.Warnings = append(report.Warnings, Warning{Code: CodeBackfillInputEmpty, Message: "No input files provided or found"})
		report.Validation.WarningCodes = append(report.Validation.WarningCodes, CodeBackfillInputEmpty)
		return report, nil
	}

	// deduplicate input paths
	uniqueFiles := map[string]bool{}
	var filteredFiles []string
	for _, f := range filesToProcess {
		if !uniqueFiles[f] {
			uniqueFiles[f] = true
			filteredFiles = append(filteredFiles, f)
		}
	}
	sort.Strings(filteredFiles) // Initial sort to ensure deterministic processing

	// Check trust level warning
	trustLevel := normalizeDefault(opts.TrustLevel, TrustLevelUserProvidedUnverified)
	if trustLevel == TrustLevelUserProvidedUnverified {
		report.Warnings = append(report.Warnings, Warning{Code: CodeBackfillUserProvidedUnverified, Message: "User provided unverified evidence cannot reduce survivorship risk to LOW"})
		report.Validation.WarningCodes = append(report.Validation.WarningCodes, CodeBackfillUserProvidedUnverified)
	} else if trustLevel == TrustLevelUnknown {
		report.Warnings = append(report.Warnings, Warning{Code: CodeBackfillTrustLevelUnknown, Message: "Trust level is unknown"})
		report.Validation.WarningCodes = append(report.Validation.WarningCodes, CodeBackfillTrustLevelUnknown)
	}

	// Load existing manifest to check for duplicates
	baseDir := filepath.Join(opts.ArchiveRoot, report.Exchange, report.MarketType)
	manifestsDir := filepath.Join(baseDir, "manifests")
	manifestPath := filepath.Join(manifestsDir, "exchange_metadata_snapshot_manifest.json")
	var existingManifest *SnapshotManifest
	if data, err := os.ReadFile(manifestPath); err == nil {
		var m SnapshotManifest
		if json.Unmarshal(data, &m) == nil {
			existingManifest = &m
		}
	}
	existingHashes := map[string]bool{}
	if existingManifest != nil {
		for _, s := range existingManifest.Snapshots {
			existingHashes[s.SnapshotHash] = true
		}
	}

	// In-memory set for this batch's hashes to avoid cross-overwrites in same batch
	batchHashes := map[string]bool{}

	gitSHA := "unknown" // In real CLI we would get this from getGitSHA() if available, but for backfill it's less critical. Let's just use unknown.

	for _, path := range filteredFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			report.SkippedFiles = append(report.SkippedFiles, IntakeSkippedFile{InputPathRelative: path, Reason: err.Error()})
			continue
		}

		sum := sha256.Sum256(data)
		inputSHA := hex.EncodeToString(sum[:])

		var observedTime *string

		if opts.ObservedTimeFromFilename && opts.FilenameTimeLayout != "" {
			baseName := filepath.Base(path)
			// Remove extension
			ext := filepath.Ext(baseName)
			nameNoExt := strings.TrimSuffix(baseName, ext)

			// Attempt to parse time from filename
			t, err := time.Parse(opts.FilenameTimeLayout, nameNoExt)
			if err == nil {
				val := t.UTC().Format(time.RFC3339)
				observedTime = &val
			}
		}

		if observedTime == nil && opts.ObservedTime != "" {
			val := normalizeTimestampOrNow(opts.ObservedTime)
			observedTime = &val
		}

		sourceURI := opts.SourceURI
		if sourceURI == "" {
			sourceURI = filepath.ToSlash(path)
		}

		snapOpts := SnapshotOptions{
			Exchange:              report.Exchange,
			MarketType:            report.MarketType,
			QuoteAssetFilter:      opts.QuoteAssetFilter,
			SourceType:            normalizeDefault(opts.SourceType, "file_import_historical"),
			SourceName:            normalizeDefault(opts.SourceName, "historical_backfill"),
			SourceURI:             sourceURI,
			CollectedAtUTC:        report.GeneratedAtUTC,
			SourceObservedTimeUTC: "",
			TrustLevel:            trustLevel,
			CollectorGitSHA:       gitSHA,
			RawPayload:            data,
		}

		if observedTime != nil {
			snapOpts.SourceObservedTimeUTC = *observedTime
		}

		snapshot, err := BuildSnapshot(snapOpts)
		if err != nil || len(snapshot.Symbols) == 0 {
			msg := "No valid symbols extracted"
			if err != nil {
				msg = err.Error()
			}
			report.SkippedFiles = append(report.SkippedFiles, IntakeSkippedFile{InputPathRelative: path, Reason: msg})
			report.Warnings = append(report.Warnings, Warning{Code: CodeBackfillInputParseFailed, Target: path, Message: "Failed to parse input file: " + msg})
			report.Validation.WarningCodes = append(report.Validation.WarningCodes, CodeBackfillInputParseFailed)
			continue
		}

		if snapshot.SourceObservedTimeUTC == nil {
			report.Warnings = append(report.Warnings, Warning{Code: CodeBackfillObservedTimeMissing, Target: path, Message: "Observed time missing for snapshot"})
			report.Validation.WarningCodes = append(report.Validation.WarningCodes, CodeBackfillObservedTimeMissing)
		}

		inputFile := IntakeInputFile{
			InputPathRelative:      filepath.ToSlash(path),
			InputSHA256:            inputSHA,
			SourceType:             snapshot.SourceType,
			SourceName:             snapshot.SourceName,
			SourceURIOrPath:        snapshot.SourceURI,
			SourceObservedTimeUTC:  snapshot.SourceObservedTimeUTC,
			SourceCollectedTimeUTC: nil,
			TrustLevel:             snapshot.TrustLevel,
			Notes:                  "",
		}
		report.InputFiles = append(report.InputFiles, inputFile)

		year := report.GeneratedAtUTC[0:4]
		month := report.GeneratedAtUTC[5:7]
		day := report.GeneratedAtUTC[8:10]

		if snapshot.SourceObservedTimeUTC != nil {
			year = (*snapshot.SourceObservedTimeUTC)[0:4]
			month = (*snapshot.SourceObservedTimeUTC)[5:7]
			day = (*snapshot.SourceObservedTimeUTC)[8:10]
		}

		snapshotsDir := filepath.Join(baseDir, "snapshots", year, month, day)
		snapshotFileName := fmt.Sprintf("exchange_metadata_snapshot_%s.json", snapshot.SnapshotID)
		snapshotPath := filepath.Join(snapshotsDir, snapshotFileName)
		relSnapPath := filepath.ToSlash(filepath.Join("snapshots", year, month, day, snapshotFileName))

		if existingHashes[snapshot.Hashes.SnapshotHash] || batchHashes[snapshot.Hashes.SnapshotHash] {
			report.DuplicateSnapshots = append(report.DuplicateSnapshots, IntakeDuplicateSnapshot{
				SnapshotID:   snapshot.SnapshotID,
				SnapshotHash: snapshot.Hashes.SnapshotHash,
				ArchivePath:  relSnapPath,
			})
			report.Warnings = append(report.Warnings, Warning{Code: CodeBackfillDuplicateSnapshot, Target: snapshot.SnapshotID, Message: "Duplicate snapshot hash detected"})
			report.Validation.WarningCodes = append(report.Validation.WarningCodes, CodeBackfillDuplicateSnapshot)
			continue
		}

		batchHashes[snapshot.Hashes.SnapshotHash] = true

		if !opts.DryRun {
			if err := WriteSnapshot(snapshotPath, snapshot); err != nil {
				return nil, fmt.Errorf("write snapshot: %w", err)
			}

			// Write raw payload
			timestamp := strings.NewReplacer("-", "", ":", "", "T", "_", "Z", "").Replace(snapshot.CollectedAtUTC)
			rawFileName := fmt.Sprintf("exchangeInfo_%s_%s.json", timestamp, snapshot.RawPayloadSHA256)
			rawDir := filepath.Join(baseDir, "raw", year, month, day)
			rawPath := filepath.Join(rawDir, rawFileName)

			if err := os.MkdirAll(rawDir, 0755); err != nil {
				return nil, fmt.Errorf("mkdir raw dir: %w", err)
			}
			if err := os.WriteFile(rawPath, data, 0644); err != nil {
				return nil, fmt.Errorf("write raw payload: %w", err)
			}
		}

		imported := IntakeSnapshot{
			SnapshotID:            snapshot.SnapshotID,
			SnapshotHash:          snapshot.Hashes.SnapshotHash,
			SymbolCount:           len(snapshot.Symbols),
			ArchivePath:           relSnapPath,
			SourceObservedTimeUTC: snapshot.SourceObservedTimeUTC,
			TrustLevel:            snapshot.TrustLevel,
			Warnings:              snapshot.Warnings,
		}
		report.ImportedSnapshots = append(report.ImportedSnapshots, imported)
	}

	computeIntakeHashes(report)

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
	}

	return report, nil
}

func computeIntakeHashes(report *IntakeReport) {
	// Source batch hash
	sort.Slice(report.InputFiles, func(i, j int) bool {
		return report.InputFiles[i].InputPathRelative < report.InputFiles[j].InputPathRelative
	})

	batchData := []string{}
	for _, f := range report.InputFiles {
		obs := "null"
		if f.SourceObservedTimeUTC != nil {
			obs = *f.SourceObservedTimeUTC
		}
		batchData = append(batchData, fmt.Sprintf("%s|%s|%s|%s", filepath.Base(f.InputPathRelative), f.InputSHA256, obs, f.TrustLevel))
	}
	batchSum := sha256.Sum256([]byte(strings.Join(batchData, ",")))
	report.Hashes.SourceBatchHash = hex.EncodeToString(batchSum[:])

	// Imported snapshot set hash
	sort.SliceStable(report.ImportedSnapshots, func(i, j int) bool {
		obsI := ""
		if report.ImportedSnapshots[i].SourceObservedTimeUTC != nil {
			obsI = *report.ImportedSnapshots[i].SourceObservedTimeUTC
		}
		obsJ := ""
		if report.ImportedSnapshots[j].SourceObservedTimeUTC != nil {
			obsJ = *report.ImportedSnapshots[j].SourceObservedTimeUTC
		}
		if obsI != obsJ {
			return obsI < obsJ
		}
		return report.ImportedSnapshots[i].SnapshotHash < report.ImportedSnapshots[j].SnapshotHash
	})

	snapData := []string{}
	for _, s := range report.ImportedSnapshots {
		snapData = append(snapData, s.SnapshotHash)
	}
	snapSum := sha256.Sum256([]byte(strings.Join(snapData, ",")))
	report.Hashes.ImportedSnapshotSetHash = hex.EncodeToString(snapSum[:])

	// Intake Hash
	// To make intake hash deterministic, we omit GeneratedAtUTC
	clone := *report
	clone.GeneratedAtUTC = ""
	clone.Hashes.IntakeHash = ""
	b, _ := json.Marshal(clone)
	intakeSum := sha256.Sum256(b)
	report.Hashes.IntakeHash = hex.EncodeToString(intakeSum[:])

	// Create a short intake ID
	report.IntakeID = fmt.Sprintf("intake_%s_%s_%s", report.Exchange, report.MarketType, report.Hashes.IntakeHash[:12])
}
