package pitarchive

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	GapManifestSchemaVersion       = "ak-historian.pit-gap-manifest.v1"
	DefaultMaxGapBytes       int64 = 16 << 20
)

type GapBuildOptions struct {
	ArchiveRoot            string
	DatasetID              string
	ManifestID             string
	CandidateID            string
	CandidateVersion       string
	ImplementationHash     string
	Market                 string
	Interval               string
	WindowStart            time.Time
	WindowEnd              time.Time
	EvaluationCutoff       time.Time
	GeneratedAt            time.Time
	HistorianBuild         string
	EventSchemaVersion     string
	RequiredSymbols        []string
	RequiredContextSymbols []string
	SourceAvailability     map[string]time.Time
	MaxSnapshotBytes       int64
}

type GapSnapshot struct {
	SnapshotID        string       `json:"snapshot_id"`
	PartitionKey      string       `json:"partition_key"`
	RelativePath      string       `json:"relative_path"`
	EventTimeStart    time.Time    `json:"event_time_start"`
	EventTimeEnd      time.Time    `json:"event_time_end"`
	SourceAvailableAt *time.Time   `json:"source_available_at"`
	ContentHash       string       `json:"content_hash"`
	PartitionHash     string       `json:"partition_hash"`
	ByteSize          int64        `json:"byte_size"`
	EvidenceGaps      []ReasonCode `json:"evidence_gaps"`
}

type MissingPartitionEvidence struct {
	PartitionKey string       `json:"partition_key"`
	Reasons      []ReasonCode `json:"reasons"`
}

type PITCoverageClaim struct {
	Available bool       `json:"available"`
	Start     *time.Time `json:"start"`
	End       *time.Time `json:"end"`
}

type GapManifest struct {
	SchemaVersion             string                     `json:"schema_version"`
	Status                    Verdict                    `json:"status"`
	DatasetID                 string                     `json:"dataset_id"`
	DatasetVersion            string                     `json:"dataset_version"`
	ManifestID                string                     `json:"manifest_id"`
	ManifestHash              string                     `json:"manifest_hash"`
	CandidateID               string                     `json:"candidate_id"`
	CandidateVersion          string                     `json:"candidate_version"`
	ImplementationHash        string                     `json:"implementation_hash"`
	PhysicalCoverageStart     time.Time                  `json:"physical_coverage_start"`
	PhysicalCoverageEnd       time.Time                  `json:"physical_coverage_end"`
	ProvablePITCoverage       PITCoverageClaim           `json:"provable_pit_coverage"`
	EvaluationCutoff          time.Time                  `json:"evaluation_cutoff"`
	CoveragePolicyVersion     string                     `json:"coverage_policy_version"`
	AvailabilityPolicyVersion string                     `json:"availability_policy_version"`
	EventSchemaVersion        string                     `json:"event_schema_version"`
	RequiredSymbols           []string                   `json:"required_symbols"`
	RequiredContextSymbols    []string                   `json:"required_context_symbols"`
	ExpectedPartitions        []string                   `json:"expected_partitions"`
	Snapshots                 []GapSnapshot              `json:"snapshots"`
	MissingEvidence           []MissingPartitionEvidence `json:"missing_evidence"`
	SnapshotSetHash           string                     `json:"snapshot_set_hash"`
	ManifestCreatedAt         time.Time                  `json:"manifest_created_at"`
	HistorianBuild            string                     `json:"historian_build"`
}

func BuildGapManifest(options GapBuildOptions) (GapManifest, error) {
	if err := validateGapOptions(options); err != nil {
		return GapManifest{}, err
	}
	if options.MaxSnapshotBytes <= 0 {
		options.MaxSnapshotBytes = DefaultMaxSnapshotBytes
	}
	root, err := os.OpenRoot(options.ArchiveRoot)
	if err != nil {
		return GapManifest{}, fmt.Errorf("open archive root: %w", err)
	}
	defer root.Close()

	symbols := sortedUnique(append(append([]string{}, options.RequiredSymbols...), options.RequiredContextSymbols...))
	manifest := GapManifest{
		SchemaVersion: GapManifestSchemaVersion, Status: VerdictEvidenceIncomplete,
		DatasetID: options.DatasetID, ManifestID: options.ManifestID,
		CandidateID: options.CandidateID, CandidateVersion: options.CandidateVersion,
		ImplementationHash:    strings.ToLower(options.ImplementationHash),
		PhysicalCoverageStart: options.WindowStart.UTC(), PhysicalCoverageEnd: options.WindowEnd.UTC(),
		ProvablePITCoverage: PITCoverageClaim{Available: false}, EvaluationCutoff: options.EvaluationCutoff.UTC(),
		CoveragePolicyVersion: CoveragePolicySchemaVersion, AvailabilityPolicyVersion: AvailabilityPolicySchemaVersion,
		EventSchemaVersion: options.EventSchemaVersion, RequiredSymbols: sortedUnique(options.RequiredSymbols),
		RequiredContextSymbols: sortedUnique(options.RequiredContextSymbols), ManifestCreatedAt: options.GeneratedAt.UTC(),
		HistorianBuild:     options.HistorianBuild,
		ExpectedPartitions: []string{}, Snapshots: []GapSnapshot{}, MissingEvidence: []MissingPartitionEvidence{},
	}
	schemaAuthorityMissing := options.EventSchemaVersion == "" || strings.Contains(strings.ToLower(options.EventSchemaVersion), "unversioned")

	for month := monthFloor(options.WindowStart); month.Before(options.WindowEnd); month = month.AddDate(0, 1, 0) {
		end := month.AddDate(0, 1, 0)
		for _, symbol := range symbols {
			partition := fmt.Sprintf("%s/%s/%s/%s", options.Market, options.Interval, symbol, month.Format("2006-01"))
			relative := fmt.Sprintf("candles/%s/%s/symbol=%s/year=%s/month=%s/%s-%s-%s.parquet", options.Market, options.Interval, symbol, month.Format("2006"), month.Format("01"), symbol, options.Interval, month.Format("2006-01"))
			manifest.ExpectedPartitions = append(manifest.ExpectedPartitions, partition)
			snapshot, reasons, snapshotErr := hashGapSnapshot(root, relative, partition, month, end, options.SourceAvailability[partition], options.EvaluationCutoff, options.MaxSnapshotBytes)
			if snapshotErr != nil {
				return GapManifest{}, snapshotErr
			}
			if schemaAuthorityMissing {
				reasons = appendUniqueReason(reasons, ReasonSnapshotSchemaUnsupported)
				snapshot.EvidenceGaps = appendUniqueReason(snapshot.EvidenceGaps, ReasonSnapshotSchemaUnsupported)
			}
			if snapshot.ContentHash != "" {
				manifest.Snapshots = append(manifest.Snapshots, snapshot)
			}
			if len(reasons) > 0 {
				manifest.MissingEvidence = append(manifest.MissingEvidence, MissingPartitionEvidence{PartitionKey: partition, Reasons: reasons})
			}
		}
	}
	manifest.SnapshotSetHash, err = digestCanonical(manifest.Snapshots)
	if err != nil {
		return GapManifest{}, err
	}
	manifest.DatasetVersion = manifest.SnapshotSetHash
	manifest.ManifestHash, err = ComputeGapManifestHash(manifest)
	if err != nil {
		return GapManifest{}, err
	}
	return manifest, nil
}

func ComputeGapManifestHash(manifest GapManifest) (string, error) {
	copyManifest := manifest
	copyManifest.ManifestHash = ""
	copyManifest.ExpectedPartitions = append([]string{}, manifest.ExpectedPartitions...)
	copyManifest.RequiredSymbols = append([]string{}, manifest.RequiredSymbols...)
	copyManifest.RequiredContextSymbols = append([]string{}, manifest.RequiredContextSymbols...)
	copyManifest.Snapshots = append([]GapSnapshot{}, manifest.Snapshots...)
	copyManifest.MissingEvidence = append([]MissingPartitionEvidence{}, manifest.MissingEvidence...)
	sort.Strings(copyManifest.ExpectedPartitions)
	sort.Strings(copyManifest.RequiredSymbols)
	sort.Strings(copyManifest.RequiredContextSymbols)
	sort.Slice(copyManifest.Snapshots, func(i, j int) bool {
		return copyManifest.Snapshots[i].PartitionKey < copyManifest.Snapshots[j].PartitionKey
	})
	sort.Slice(copyManifest.MissingEvidence, func(i, j int) bool {
		return copyManifest.MissingEvidence[i].PartitionKey < copyManifest.MissingEvidence[j].PartitionKey
	})
	for i := range copyManifest.MissingEvidence {
		sort.Slice(copyManifest.MissingEvidence[i].Reasons, func(a, b int) bool {
			return copyManifest.MissingEvidence[i].Reasons[a] < copyManifest.MissingEvidence[i].Reasons[b]
		})
	}
	return digestCanonical(copyManifest)
}

func WriteGapManifest(output string, manifest GapManifest) error {
	expected, err := ComputeGapManifestHash(manifest)
	if err != nil || !compareDigest(expected, manifest.ManifestHash) {
		return errors.New("gap manifest hash does not match canonical contents")
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(output, append(data, '\n'), DefaultMaxGapBytes)
}

func validateGapOptions(options GapBuildOptions) error {
	for name, value := range map[string]string{"archive_root": options.ArchiveRoot, "dataset_id": options.DatasetID, "manifest_id": options.ManifestID, "candidate_id": options.CandidateID, "candidate_version": options.CandidateVersion, "market": options.Market, "interval": options.Interval, "historian_build": options.HistorianBuild} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	if strings.ContainsAny(options.DatasetID, `/\\`) || mutableAlias(options.DatasetID) || mutableAlias(options.ManifestID) {
		return errors.New("dataset and manifest identities must be stable non-path, non-mutable tokens")
	}
	if !validSHA256(options.ImplementationHash) {
		return errors.New("implementation_hash must be a SHA-256 digest")
	}
	if options.WindowStart.IsZero() || options.WindowEnd.IsZero() || !options.WindowStart.Before(options.WindowEnd) || !monthFloor(options.WindowStart).Equal(options.WindowStart.UTC()) || !monthFloor(options.WindowEnd).Equal(options.WindowEnd.UTC()) {
		return errors.New("window must be a nonempty UTC month-aligned half-open interval")
	}
	if options.EvaluationCutoff.IsZero() || options.EvaluationCutoff.Before(options.WindowEnd) || options.GeneratedAt.IsZero() || options.GeneratedAt.Before(options.EvaluationCutoff) {
		return errors.New("cutoff and creation time are required and ordered")
	}
	if len(options.RequiredSymbols) == 0 || len(options.RequiredContextSymbols) == 0 {
		return errors.New("required symbols and context symbols are required")
	}
	return nil
}

func hashGapSnapshot(root *os.Root, relative, partition string, start, end, available time.Time, cutoff time.Time, maxBytes int64) (GapSnapshot, []ReasonCode, error) {
	reasons := []ReasonCode{}
	snapshot := GapSnapshot{SnapshotID: "snapshot:" + partition, PartitionKey: partition, RelativePath: relative, EventTimeStart: start.UTC(), EventTimeEnd: end.UTC(), EvidenceGaps: []ReasonCode{}}
	if err := validateSnapshotPath(relative); err != nil {
		return GapSnapshot{}, nil, err
	}
	if err := rejectSymlinkComponents(root, relative); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return snapshot, []ReasonCode{ReasonSnapshotMissing, ReasonAvailabilityTimestampMissing}, nil
		}
		return GapSnapshot{}, nil, err
	}
	file, err := root.Open(relative)
	if err != nil {
		return GapSnapshot{}, nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return GapSnapshot{}, nil, errors.New("snapshot is not a regular file")
	}
	if info.Size() > maxBytes {
		return GapSnapshot{}, nil, errors.New("snapshot exceeds bounded read policy")
	}
	hash := sha256.New()
	count, err := io.Copy(hash, io.LimitReader(file, maxBytes+1))
	if err != nil || count != info.Size() {
		return GapSnapshot{}, nil, errors.New("snapshot could not be hashed completely")
	}
	snapshot.ContentHash = "sha256:" + hex.EncodeToString(hash.Sum(nil))
	snapshot.PartitionHash = snapshot.ContentHash
	snapshot.ByteSize = count
	if available.IsZero() {
		reasons = append(reasons, ReasonAvailabilityTimestampMissing)
	} else {
		value := available.UTC()
		snapshot.SourceAvailableAt = &value
		if value.After(cutoff) {
			reasons = append(reasons, ReasonAvailableAfterEvaluation)
		}
	}
	snapshot.EvidenceGaps = append(snapshot.EvidenceGaps, reasons...)
	return snapshot, reasons, nil
}

func monthFloor(value time.Time) time.Time {
	value = value.UTC()
	return time.Date(value.Year(), value.Month(), 1, 0, 0, 0, 0, time.UTC)
}
func sortedUnique(values []string) []string {
	seen := map[string]struct{}{}
	for _, value := range values {
		if value != "" {
			seen[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
func appendUniqueReason(values []ReasonCode, value ReasonCode) []ReasonCode {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
func mutableAlias(value string) bool {
	switch strings.ToLower(value) {
	case "latest", "current", "default", "production":
		return true
	}
	return false
}
