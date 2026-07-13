package pitarchive

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type codedError struct {
	code ReasonCode
	err  error
}

func (e *codedError) Error() string { return e.err.Error() }
func (e *codedError) Unwrap() error { return e.err }

func failureFromError(err error) Failure {
	var coded *codedError
	if errors.As(err, &coded) {
		return Failure{Code: coded.code, Message: coded.err.Error()}
	}
	return Failure{Code: ReasonManifestUnreadable, Message: err.Error()}
}

func LoadManifest(path string, maxBytes int64) (SnapshotManifest, error) {
	if strings.TrimSpace(path) == "" {
		return SnapshotManifest{}, &codedError{ReasonManifestMissing, errors.New("snapshot manifest path is required")}
	}
	base := filepath.Base(path)
	if strings.HasPrefix(base, ".") || strings.Contains(base, ".tmp-") || strings.HasSuffix(base, ".tmp") {
		return SnapshotManifest{}, &codedError{ReasonManifestUnreadable, errors.New("temporary files are not accepted as snapshot manifests")}
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxManifestBytes
	}
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return SnapshotManifest{}, &codedError{ReasonManifestMissing, errors.New("snapshot manifest does not exist")}
		}
		return SnapshotManifest{}, &codedError{ReasonManifestUnreadable, fmt.Errorf("open snapshot manifest: %w", err)}
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return SnapshotManifest{}, &codedError{ReasonManifestUnreadable, fmt.Errorf("read snapshot manifest: %w", err)}
	}
	if int64(len(data)) > maxBytes {
		return SnapshotManifest{}, &codedError{ReasonManifestUnreadable, fmt.Errorf("snapshot manifest exceeds %d-byte limit", maxBytes)}
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return SnapshotManifest{}, &codedError{ReasonManifestEmpty, errors.New("snapshot manifest is empty")}
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest SnapshotManifest
	if err := decoder.Decode(&manifest); err != nil {
		return SnapshotManifest{}, &codedError{ReasonManifestMalformed, fmt.Errorf("decode snapshot manifest: %w", err)}
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return SnapshotManifest{}, &codedError{ReasonManifestMalformed, errors.New("snapshot manifest contains trailing JSON data")}
	}
	return manifest, nil
}

func WriteManifest(path string, manifest SnapshotManifest) error {
	failures := validateManifestStructure(manifest)
	if len(failures) > 0 {
		return fmt.Errorf("snapshot manifest is invalid: %s", failures[0].Message)
	}
	expected, err := ComputeManifestHash(manifest)
	if err != nil {
		return fmt.Errorf("compute snapshot manifest hash: %w", err)
	}
	if !compareDigest(manifest.ManifestHash, expected) {
		return fmt.Errorf("snapshot manifest hash does not match canonical contents")
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot manifest: %w", err)
	}
	data = append(data, '\n')
	return writeAtomic(path, data, DefaultMaxManifestBytes)
}

func validateManifestStructure(manifest SnapshotManifest) []Failure {
	var failures []Failure
	missing := func(field string) {
		failures = append(failures, Failure{Code: ReasonManifestFieldMissing, Message: field + " is required"})
	}
	if manifest.SchemaVersion == "" {
		missing("schema_version")
	} else if manifest.SchemaVersion != SnapshotManifestSchemaVersion {
		failures = append(failures, Failure{Code: ReasonManifestSchemaUnsupported, Message: "unsupported snapshot manifest schema"})
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"manifest_id", manifest.ManifestID}, {"dataset_id", manifest.DatasetID}, {"dataset_version", manifest.DatasetVersion},
		{"source", manifest.Source}, {"archive_id", manifest.ArchiveID},
	} {
		if strings.TrimSpace(field.value) == "" {
			missing(field.name)
		}
	}
	if manifest.ResearchWindowStart.IsZero() {
		missing("research_window_start")
	}
	if manifest.ResearchWindowEnd.IsZero() {
		missing("research_window_end")
	}
	if !manifest.ResearchWindowStart.IsZero() && !manifest.ResearchWindowEnd.After(manifest.ResearchWindowStart) {
		failures = append(failures, Failure{Code: ReasonManifestWindowMismatch, Message: "research window end must be after start"})
	}
	if manifest.GeneratedAt.IsZero() {
		missing("generated_at")
	}
	if manifest.SnapshotCount <= 0 || len(manifest.Snapshots) == 0 {
		failures = append(failures, Failure{Code: ReasonManifestSnapshotCountMismatch, Message: "snapshot manifest must contain at least one snapshot"})
	} else if manifest.SnapshotCount != len(manifest.Snapshots) {
		failures = append(failures, Failure{Code: ReasonManifestSnapshotCountMismatch, Message: "snapshot_count does not match snapshots length"})
	}

	seenIDs := make(map[string]struct{}, len(manifest.Snapshots))
	for _, snapshot := range manifest.Snapshots {
		failure := Failure{SnapshotID: snapshot.SnapshotID, PartitionKey: snapshot.PartitionKey}
		if strings.TrimSpace(snapshot.SnapshotID) == "" {
			failure.Code, failure.Message = ReasonManifestFieldMissing, "snapshot_id is required"
			failures = append(failures, failure)
		} else if _, exists := seenIDs[snapshot.SnapshotID]; exists {
			failure.Code, failure.Message = ReasonSnapshotDuplicateID, "duplicate snapshot identity"
			failures = append(failures, failure)
		}
		seenIDs[snapshot.SnapshotID] = struct{}{}
		if strings.TrimSpace(snapshot.PartitionKey) == "" {
			failure.Code, failure.Message = ReasonManifestFieldMissing, "snapshot partition_key is required"
			failures = append(failures, failure)
		}
		if strings.TrimSpace(snapshot.RelativePath) == "" {
			failure.Code, failure.Message = ReasonSnapshotPathInvalid, "snapshot relative_path is required"
			failures = append(failures, failure)
		} else if err := validateSnapshotPath(snapshot.RelativePath); err != nil {
			failure.Code, failure.Message = ReasonSnapshotPathInvalid, err.Error()
			failures = append(failures, failure)
		}
		if snapshot.EventTimeStart.IsZero() || snapshot.EventTimeEnd.IsZero() || !snapshot.EventTimeEnd.After(snapshot.EventTimeStart) {
			failure.Code, failure.Message = ReasonManifestFieldMissing, "snapshot event-time bounds are required and must be ordered"
			failures = append(failures, failure)
		}
		if snapshot.AvailableAt.IsZero() {
			failure.Code, failure.Message = ReasonAvailabilityTimestampMissing, "snapshot available_at is required"
			failures = append(failures, failure)
		}
		if snapshot.SchemaVersion == "" {
			failure.Code, failure.Message = ReasonManifestFieldMissing, "snapshot schema_version is required"
			failures = append(failures, failure)
		} else if snapshot.SchemaVersion != SnapshotSchemaVersion {
			failure.Code, failure.Message = ReasonSnapshotSchemaUnsupported, "unsupported snapshot schema"
			failures = append(failures, failure)
		}
		if !validSHA256(snapshot.ContentHash) {
			failure.Code, failure.Message = ReasonManifestFieldMissing, "snapshot content_hash must be a SHA-256 digest"
			failures = append(failures, failure)
		}
		if snapshot.ByteSize <= 0 {
			failure.Code, failure.Message = ReasonManifestFieldMissing, "snapshot byte_size must be positive"
			failures = append(failures, failure)
		}
	}

	failures = append(failures, validateCoveragePolicy(manifest)...)
	if manifest.AvailabilityPolicy.SchemaVersion == "" {
		missing("availability_policy.schema_version")
	} else if manifest.AvailabilityPolicy.SchemaVersion != AvailabilityPolicySchemaVersion {
		failures = append(failures, Failure{Code: ReasonManifestSchemaUnsupported, Message: "unsupported availability policy schema"})
	}
	if strings.TrimSpace(manifest.AvailabilityPolicy.PolicyID) == "" {
		missing("availability_policy.policy_id")
	}
	if manifest.AvailabilityPolicy.RequiredPublicationDelaySeconds == nil {
		missing("availability_policy.required_publication_delay_seconds")
	} else if *manifest.AvailabilityPolicy.RequiredPublicationDelaySeconds < 0 || *manifest.AvailabilityPolicy.RequiredPublicationDelaySeconds > math.MaxInt64/int64(time.Second) {
		failures = append(failures, Failure{Code: ReasonManifestFieldMissing, Message: "publication delay is outside the supported duration range"})
	}
	if !validSHA256(manifest.ManifestHash) {
		missing("manifest_hash")
	} else if expected, err := ComputeManifestHash(manifest); err != nil || !compareDigest(manifest.ManifestHash, expected) {
		failures = append(failures, Failure{Code: ReasonManifestHashMismatch, Message: "manifest hash does not match canonical contents"})
	}
	return failures
}

func validateManifestExpectations(manifest SnapshotManifest, options EvaluateOptions) []Failure {
	var failures []Failure
	if manifest.DatasetID != options.DatasetID || manifest.DatasetVersion != options.DatasetVersion {
		failures = append(failures, Failure{Code: ReasonManifestDatasetMismatch, Message: "manifest dataset identity does not match evaluation request"})
	}
	if !manifest.ResearchWindowStart.Equal(options.ResearchWindowStart) || !manifest.ResearchWindowEnd.Equal(options.ResearchWindowEnd) {
		failures = append(failures, Failure{Code: ReasonManifestWindowMismatch, Message: "manifest research window does not match evaluation request"})
	}
	return failures
}

func requiredTime(value time.Time) bool { return !value.IsZero() }
