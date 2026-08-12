package researchidentity

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/david22573/ak-historian/internal/atomicfile"
	"github.com/david22573/ak-historian/internal/canonicalcontract"
	"github.com/david22573/ak-historian/internal/parquetutil"
)

type Builder struct{}

type timestampSeries struct {
	symbol   string
	interval string
	times    []int64
}

func (Builder) Build(opts BuildOptions) (Manifest, error) {
	if err := validateBuildOptions(opts); err != nil {
		return Manifest{}, err
	}
	start, err := parseCanonicalUTC(opts.DatasetStartUTC)
	if err != nil {
		return Manifest{}, fmt.Errorf("dataset start: %w", err)
	}
	end, err := parseCanonicalUTC(opts.DatasetEndUTC)
	if err != nil {
		return Manifest{}, fmt.Errorf("dataset end: %w", err)
	}
	cutoff, err := parseCanonicalUTC(opts.PointInTimeCutoffUTC)
	if err != nil {
		return Manifest{}, fmt.Errorf("point-in-time cutoff: %w", err)
	}
	if !start.Before(end) {
		return Manifest{}, fmt.Errorf("dataset start must be strictly before end")
	}
	if cutoff.Before(end) {
		return Manifest{}, fmt.Errorf("point-in-time cutoff precedes dataset end")
	}
	now := opts.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if start.After(now) || end.After(now) || cutoff.After(now) {
		return Manifest{}, fmt.Errorf("future dataset or cutoff timestamp")
	}

	availability, availabilitySource, err := loadAvailabilityPolicy(opts.AvailabilityPolicyPath)
	if err != nil {
		return Manifest{}, err
	}
	coveragePolicy, coverageSource, err := loadCoveragePolicy(opts.CoveragePolicyPath)
	if err != nil {
		return Manifest{}, err
	}
	availabilitySource.RelativePath, err = safeRelativePath(opts.EvidenceRoot, opts.AvailabilityPolicyPath)
	if err != nil {
		return Manifest{}, fmt.Errorf("availability policy locator: %w", err)
	}
	coverageSource.RelativePath, err = safeRelativePath(opts.EvidenceRoot, opts.CoveragePolicyPath)
	if err != nil {
		return Manifest{}, fmt.Errorf("coverage policy locator: %w", err)
	}
	if availability.AvailabilityDelayNS%int64(time.Millisecond) != 0 || coveragePolicy.IntervalNS%int64(time.Millisecond) != 0 {
		return Manifest{}, fmt.Errorf("current Parquet evidence requires millisecond-aligned policy durations")
	}
	availabilityDelayMS := availability.AvailabilityDelayNS / int64(time.Millisecond)
	coverageIntervalMS := coveragePolicy.IntervalNS / int64(time.Millisecond)

	archiveRawHash, archiveSize, err := hashFile(opts.SourceArchivePath, "source_archive")
	if err != nil {
		return Manifest{}, fmt.Errorf("source archive: %w", err)
	}
	archiveRelative, err := safeRelativePath(opts.EvidenceRoot, opts.SourceArchivePath)
	if err != nil {
		return Manifest{}, fmt.Errorf("source archive locator: %w", err)
	}

	objects, series, symbols, err := buildObjectInventory(opts.DataRoot, start, end, availabilityDelayMS)
	if err != nil {
		return Manifest{}, err
	}
	coverage, earliestAvailable, latestAvailable, err := validateStrictCoverage(series, start, end, cutoff, availabilityDelayMS, coverageIntervalMS)
	if err != nil {
		return Manifest{}, err
	}

	archive := ArchiveIdentity{
		Contract:      canonicalcontract.NewHeader(ArchiveSchemaName, ManifestSchemaVersion, ArchiveArtifactRole),
		ArchiveID:     opts.SourceArchiveID,
		MediaType:     "application/octet-stream",
		RawObjectHash: archiveRawHash,
		RelativePath:  archiveRelative,
		SizeBytes:     archiveSize,
	}
	archive.ArtifactHash, err = computeArchiveHash(archive)
	if err != nil {
		return Manifest{}, err
	}
	dataset := DatasetIdentity{
		Contract:             canonicalcontract.NewHeader(DatasetSchemaName, ManifestSchemaVersion, DatasetArtifactRole),
		DatasetID:            opts.DatasetID,
		DatasetVersion:       opts.DatasetVersion,
		InstrumentUniverseID: opts.InstrumentUniverseID,
		Symbols:              symbols,
		DatasetStartUTC:      opts.DatasetStartUTC,
		DatasetEndUTC:        opts.DatasetEndUTC,
		Objects:              objects,
	}
	dataset.ArtifactHash, err = computeDatasetHash(dataset)
	if err != nil {
		return Manifest{}, err
	}
	coverage.Contract = canonicalcontract.NewHeader(CoverageSchemaName, ManifestSchemaVersion, CoverageArtifactRole)
	coverage.ArtifactHash, err = computeCoverageHash(coverage)
	if err != nil {
		return Manifest{}, err
	}
	pit := PITEvidence{
		Contract:                canonicalcontract.NewHeader(PITSchemaName, ManifestSchemaVersion, PITArtifactRole),
		EvidenceID:              opts.PITEvidenceID,
		EvidenceVersion:         opts.PITEvidenceVersion,
		Status:                  EvidenceStatusPass,
		DatasetID:               dataset.DatasetID,
		DatasetVersion:          dataset.DatasetVersion,
		DatasetHash:             dataset.ArtifactHash,
		SourceArchiveHash:       archive.ArtifactHash,
		AvailabilityPolicyHash:  availability.ArtifactHash,
		CoveragePolicyHash:      coveragePolicy.ArtifactHash,
		EvaluationCutoffUTC:     opts.PointInTimeCutoffUTC,
		EarliestEventUTC:        coverage.EarliestEventUTC,
		LatestEventUTC:          coverage.LatestEventUTC,
		EarliestAvailableUTC:    formatUTC(earliestAvailable),
		LatestAvailableUTC:      formatUTC(latestAvailable),
		FullWindowCoverage:      true,
		AvailabilityDelayNS:     availability.AvailabilityDelayNS,
		GapCount:                coverage.GapCount,
		DuplicateTimestampCount: coverage.DuplicateTimestampCount,
		OutOfOrderCount:         coverage.OutOfOrderCount,
	}
	pit.ArtifactHash, err = computePITEvidenceHash(pit)
	if err != nil {
		return Manifest{}, err
	}
	manifest := Manifest{
		Contract:                 canonicalcontract.NewHeader(ManifestSchemaName, ManifestSchemaVersion, ManifestArtifactRole),
		ManifestID:               opts.ManifestID,
		ManifestVersion:          opts.ManifestVersion,
		DatasetStartUTC:          opts.DatasetStartUTC,
		DatasetEndUTC:            opts.DatasetEndUTC,
		PointInTimeCutoffUTC:     opts.PointInTimeCutoffUTC,
		Dataset:                  dataset,
		SourceArchive:            archive,
		AvailabilityPolicy:       availability,
		AvailabilityPolicySource: availabilitySource,
		CoveragePolicy:           coveragePolicy,
		CoveragePolicySource:     coverageSource,
		CoverageEvidence:         coverage,
		PITEvidence:              pit,
	}
	manifest.ArtifactHash, err = computeManifestHash(manifest)
	if err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func WriteManifest(path string, manifest Manifest) error {
	data, err := canonicalcontract.CanonicalizeValue(manifest)
	if err != nil {
		return fmt.Errorf("marshal research identity manifest: %w", err)
	}
	if _, err := canonicalcontract.ValidateArtifact(data, true); err != nil {
		return fmt.Errorf("self-validate research identity manifest: %w", err)
	}
	if err := atomicfile.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write research identity manifest: %w", err)
	}
	return nil
}

func validateBuildOptions(opts BuildOptions) error {
	for name, value := range map[string]string{
		"data root":                opts.DataRoot,
		"evidence root":            opts.EvidenceRoot,
		"manifest id":              opts.ManifestID,
		"manifest version":         opts.ManifestVersion,
		"dataset id":               opts.DatasetID,
		"dataset version":          opts.DatasetVersion,
		"source archive id":        opts.SourceArchiveID,
		"source archive path":      opts.SourceArchivePath,
		"instrument universe id":   opts.InstrumentUniverseID,
		"dataset start":            opts.DatasetStartUTC,
		"dataset end":              opts.DatasetEndUTC,
		"point-in-time cutoff":     opts.PointInTimeCutoffUTC,
		"availability policy path": opts.AvailabilityPolicyPath,
		"coverage policy path":     opts.CoveragePolicyPath,
		"PIT evidence id":          opts.PITEvidenceID,
		"PIT evidence version":     opts.PITEvidenceVersion,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	for name, value := range map[string]string{
		"manifest id":            opts.ManifestID,
		"manifest version":       opts.ManifestVersion,
		"dataset id":             opts.DatasetID,
		"dataset version":        opts.DatasetVersion,
		"source archive id":      opts.SourceArchiveID,
		"instrument universe id": opts.InstrumentUniverseID,
		"PIT evidence id":        opts.PITEvidenceID,
		"PIT evidence version":   opts.PITEvidenceVersion,
	} {
		if err := validateIdentityText(name, value); err != nil {
			return err
		}
	}
	return nil
}

func validateIdentityText(name, value string) error {
	if value == "" || len(value) > 127 {
		return fmt.Errorf("%s is empty or too long", name)
	}
	for _, r := range value {
		if r > 127 || !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && !strings.ContainsRune("._:/-", r) {
			return fmt.Errorf("%s contains invalid character %q", name, r)
		}
	}
	return nil
}

func buildObjectInventory(root string, start, end time.Time, delayMS int64) ([]DatasetObjectIdentity, []timestampSeries, []string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, nil, err
	}
	var paths []string
	err = filepath.WalkDir(rootAbs, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink is not permitted in dataset root: %s", path)
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("non-regular dataset object: %s", path)
		}
		if strings.EqualFold(filepath.Ext(path), ".parquet") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, nil, nil, err
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return nil, nil, nil, fmt.Errorf("dataset contains no parquet objects")
	}

	seriesByKey := map[string]*timestampSeries{}
	symbolSet := map[string]struct{}{}
	objects := make([]DatasetObjectIdentity, 0, len(paths))
	for _, path := range paths {
		relative, err := safeRelativePath(rootAbs, path)
		if err != nil {
			return nil, nil, nil, err
		}
		symbol, interval := inferSymbolInterval(relative)
		if symbol == "" || interval == "" {
			return nil, nil, nil, fmt.Errorf("cannot infer symbol/interval from %s", relative)
		}
		times, hash, size, err := readConsistentParquetObject(path, func(snapshotPath string) ([]int64, error) {
			return parquetutil.ReadOpenTimesStrict([]string{snapshotPath})
		})
		if err != nil {
			return nil, nil, nil, err
		}
		if len(times) == 0 {
			return nil, nil, nil, fmt.Errorf("empty parquet object: %s", relative)
		}
		var windowTimes []int64
		for _, value := range times {
			t := time.UnixMilli(value).UTC()
			if !t.Before(start) && !t.After(end) {
				windowTimes = append(windowTimes, value)
			}
		}
		if len(windowTimes) == 0 {
			continue
		}
		if len(windowTimes) != len(times) {
			return nil, nil, nil, fmt.Errorf("dataset object contains rows outside requested window: %s", relative)
		}
		objects = append(objects, DatasetObjectIdentity{
			RelativePath:       relative,
			Symbol:             symbol,
			Interval:           interval,
			SizeBytes:          size,
			RawObjectHash:      hash,
			RowCount:           int64(len(times)),
			WindowRowCount:     int64(len(windowTimes)),
			EarliestEventUTC:   formatUTC(time.UnixMilli(windowTimes[0]).UTC()),
			LatestEventUTC:     formatUTC(time.UnixMilli(windowTimes[len(windowTimes)-1]).UTC()),
			LatestAvailableUTC: formatUTC(time.UnixMilli(windowTimes[len(windowTimes)-1] + delayMS).UTC()),
		})
		key := symbol + "\x00" + interval
		series := seriesByKey[key]
		if series == nil {
			series = &timestampSeries{symbol: symbol, interval: interval}
			seriesByKey[key] = series
		}
		series.times = append(series.times, windowTimes...)
		symbolSet[symbol] = struct{}{}
	}
	if len(objects) == 0 {
		return nil, nil, nil, fmt.Errorf("dataset has no rows in requested window")
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].RelativePath < objects[j].RelativePath })
	keys := make([]string, 0, len(seriesByKey))
	for key := range seriesByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	series := make([]timestampSeries, 0, len(keys))
	for _, key := range keys {
		series = append(series, *seriesByKey[key])
	}
	symbols := make([]string, 0, len(symbolSet))
	for symbol := range symbolSet {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)
	return objects, series, symbols, nil
}

func readConsistentParquetObject(path string, readTimes func(string) ([]int64, error)) ([]int64, string, int64, error) {
	beforeHash, beforeSize, err := hashFile(path, "dataset_object")
	if err != nil {
		return nil, "", 0, err
	}
	source, err := os.Open(path)
	if err != nil {
		return nil, "", 0, err
	}
	defer source.Close()
	snapshot, err := os.CreateTemp("", "ak-historian-identity-*.parquet")
	if err != nil {
		return nil, "", 0, err
	}
	snapshotPath := snapshot.Name()
	defer os.Remove(snapshotPath)
	if _, err := io.Copy(snapshot, source); err != nil {
		snapshot.Close()
		return nil, "", 0, err
	}
	if err := snapshot.Sync(); err != nil {
		snapshot.Close()
		return nil, "", 0, err
	}
	if err := snapshot.Close(); err != nil {
		return nil, "", 0, err
	}
	snapshotHash, snapshotSize, err := hashFile(snapshotPath, "dataset_object")
	if err != nil {
		return nil, "", 0, err
	}
	if snapshotHash != beforeHash || snapshotSize != beforeSize {
		return nil, "", 0, fmt.Errorf("dataset object changed while snapshotting identity input: %s", path)
	}
	times, err := readTimes(snapshotPath)
	if err != nil {
		return nil, "", 0, err
	}
	afterHash, afterSize, err := hashFile(path, "dataset_object")
	if err != nil {
		return nil, "", 0, err
	}
	if snapshotHash != afterHash || snapshotSize != afterSize {
		return nil, "", 0, fmt.Errorf("dataset object changed during identity construction: %s", path)
	}
	return times, snapshotHash, snapshotSize, nil
}

func validateStrictCoverage(series []timestampSeries, start, end, cutoff time.Time, delayMS, intervalMS int64) (CoverageEvidence, time.Time, time.Time, error) {
	if len(series) == 0 {
		return CoverageEvidence{}, time.Time{}, time.Time{}, fmt.Errorf("coverage has zero series")
	}
	if (end.UnixMilli()-start.UnixMilli())%intervalMS != 0 {
		return CoverageEvidence{}, time.Time{}, time.Time{}, fmt.Errorf("window is not aligned to coverage interval")
	}
	expectedPerSeries := (end.UnixMilli()-start.UnixMilli())/intervalMS + 1
	evidence := CoverageEvidence{
		Status:            EvidenceStatusPass,
		FullWindow:        true,
		RequestedStartUTC: formatUTC(start),
		RequestedEndUTC:   formatUTC(end),
		ExpectedRowCount:  expectedPerSeries * int64(len(series)),
		SeriesCount:       len(series),
	}
	var earliestEvent, latestEvent time.Time
	for _, values := range series {
		if len(values.times) == 0 {
			return CoverageEvidence{}, time.Time{}, time.Time{}, fmt.Errorf("empty coverage series %s/%s", values.symbol, values.interval)
		}
		for i, value := range values.times {
			if i > 0 {
				delta := value - values.times[i-1]
				switch {
				case delta < 0:
					evidence.OutOfOrderCount++
				case delta == 0:
					evidence.DuplicateTimestampCount++
				case delta != intervalMS:
					evidence.GapCount++
				}
			}
		}
		first := time.UnixMilli(values.times[0]).UTC()
		last := time.UnixMilli(values.times[len(values.times)-1]).UTC()
		if !first.Equal(start) || !last.Equal(end) || int64(len(values.times)) != expectedPerSeries {
			evidence.FullWindow = false
		}
		if earliestEvent.IsZero() || first.Before(earliestEvent) {
			earliestEvent = first
		}
		if latestEvent.IsZero() || last.After(latestEvent) {
			latestEvent = last
		}
		evidence.RowCount += int64(len(values.times))
	}
	evidence.EarliestEventUTC = formatUTC(earliestEvent)
	evidence.LatestEventUTC = formatUTC(latestEvent)
	if evidence.GapCount != 0 || evidence.DuplicateTimestampCount != 0 || evidence.OutOfOrderCount != 0 {
		return evidence, time.Time{}, time.Time{}, fmt.Errorf("coverage defects: gaps=%d duplicates=%d out_of_order=%d", evidence.GapCount, evidence.DuplicateTimestampCount, evidence.OutOfOrderCount)
	}
	if !evidence.FullWindow || evidence.RowCount != evidence.ExpectedRowCount {
		return evidence, time.Time{}, time.Time{}, fmt.Errorf("partial-window coverage")
	}
	earliestAvailable := earliestEvent.Add(time.Duration(delayMS) * time.Millisecond)
	latestAvailable := latestEvent.Add(time.Duration(delayMS) * time.Millisecond)
	if latestAvailable.After(cutoff) {
		return evidence, earliestAvailable, latestAvailable, fmt.Errorf("data availability exceeds evaluation cutoff")
	}
	return evidence, earliestAvailable, latestAvailable, nil
}

func safeRelativePath(root, path string) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return "", err
	}
	if relative == "." || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes root")
	}
	relative = filepath.ToSlash(filepath.Clean(relative))
	if strings.Contains(relative, "\\") || strings.HasPrefix(relative, "/") {
		return "", fmt.Errorf("invalid relative path")
	}
	return relative, nil
}

func inferSymbolInterval(path string) (string, string) {
	var symbol, interval string
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		switch {
		case strings.HasPrefix(part, "symbol="):
			symbol = strings.ToUpper(strings.TrimPrefix(part, "symbol="))
		case strings.HasPrefix(part, "interval="):
			interval = strings.TrimPrefix(part, "interval=")
		case interval == "" && isInterval(part):
			interval = part
		}
	}
	return symbol, interval
}

func isInterval(value string) bool {
	if len(value) < 2 {
		return false
	}
	unit := value[len(value)-1]
	if !strings.ContainsRune("mhdw", rune(unit)) {
		return false
	}
	for _, r := range value[:len(value)-1] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func parseCanonicalUTC(value string) (time.Time, error) {
	if err := canonicalcontract.ValidateTimestamp(value); err != nil {
		return time.Time{}, err
	}
	return time.Parse("2006-01-02T15:04:05.000000000Z", value)
}

func formatUTC(value time.Time) string {
	formatted, err := canonicalcontract.FormatTimestamp(value)
	if err != nil {
		panic(err)
	}
	return formatted
}
