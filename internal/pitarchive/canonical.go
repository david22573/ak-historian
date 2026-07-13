package pitarchive

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

func digestCanonical(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func canonicalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func validSHA256(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != len("sha256:")+sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil
}

type canonicalPartition struct {
	PartitionKey   string `json:"partition_key"`
	EventTimeStart string `json:"event_time_start"`
	EventTimeEnd   string `json:"event_time_end"`
}

type canonicalException struct {
	ExceptionID    string `json:"exception_id"`
	EventTimeStart string `json:"event_time_start"`
	EventTimeEnd   string `json:"event_time_end"`
	Reason         string `json:"reason"`
}

type canonicalSnapshot struct {
	SnapshotID     string `json:"snapshot_id"`
	PartitionKey   string `json:"partition_key"`
	RelativePath   string `json:"relative_path"`
	EventTimeStart string `json:"event_time_start"`
	EventTimeEnd   string `json:"event_time_end"`
	AvailableAt    string `json:"available_at"`
	IngestedAt     string `json:"ingested_at,omitempty"`
	ContentHash    string `json:"content_hash"`
	SchemaVersion  string `json:"schema_version"`
	ByteSize       int64  `json:"byte_size"`
}

func canonicalPartitions(values []PartitionRequirement) []canonicalPartition {
	result := make([]canonicalPartition, 0, len(values))
	for _, value := range values {
		result = append(result, canonicalPartition{
			PartitionKey: value.PartitionKey, EventTimeStart: canonicalTime(value.EventTimeStart), EventTimeEnd: canonicalTime(value.EventTimeEnd),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].EventTimeStart != result[j].EventTimeStart {
			return result[i].EventTimeStart < result[j].EventTimeStart
		}
		return result[i].PartitionKey < result[j].PartitionKey
	})
	return result
}

func canonicalExceptions(values []CoverageException) []canonicalException {
	result := make([]canonicalException, 0, len(values))
	for _, value := range values {
		result = append(result, canonicalException{
			ExceptionID: value.ExceptionID, EventTimeStart: canonicalTime(value.EventTimeStart), EventTimeEnd: canonicalTime(value.EventTimeEnd), Reason: value.Reason,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].EventTimeStart != result[j].EventTimeStart {
			return result[i].EventTimeStart < result[j].EventTimeStart
		}
		return result[i].ExceptionID < result[j].ExceptionID
	})
	return result
}

func canonicalSnapshots(values []SnapshotReference) []canonicalSnapshot {
	result := make([]canonicalSnapshot, 0, len(values))
	for _, value := range values {
		result = append(result, canonicalSnapshot{
			SnapshotID: value.SnapshotID, PartitionKey: value.PartitionKey, RelativePath: value.RelativePath,
			EventTimeStart: canonicalTime(value.EventTimeStart), EventTimeEnd: canonicalTime(value.EventTimeEnd),
			AvailableAt: canonicalTime(value.AvailableAt), IngestedAt: canonicalTime(value.IngestedAt),
			ContentHash: strings.ToLower(value.ContentHash), SchemaVersion: value.SchemaVersion, ByteSize: value.ByteSize,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].SnapshotID != result[j].SnapshotID {
			return result[i].SnapshotID < result[j].SnapshotID
		}
		return result[i].PartitionKey < result[j].PartitionKey
	})
	return result
}

func ComputeManifestHash(manifest SnapshotManifest) (string, error) {
	canonical := struct {
		SchemaVersion       string              `json:"schema_version"`
		ManifestID          string              `json:"manifest_id"`
		DatasetID           string              `json:"dataset_id"`
		DatasetVersion      string              `json:"dataset_version"`
		ResearchWindowStart string              `json:"research_window_start"`
		ResearchWindowEnd   string              `json:"research_window_end"`
		GeneratedAt         string              `json:"generated_at"`
		Source              string              `json:"source"`
		ArchiveID           string              `json:"archive_id"`
		CoveragePolicy      any                 `json:"coverage_policy"`
		AvailabilityPolicy  any                 `json:"availability_policy"`
		SnapshotCount       int                 `json:"snapshot_count"`
		Snapshots           []canonicalSnapshot `json:"snapshots"`
	}{
		SchemaVersion: manifest.SchemaVersion, ManifestID: manifest.ManifestID, DatasetID: manifest.DatasetID,
		DatasetVersion: manifest.DatasetVersion, ResearchWindowStart: canonicalTime(manifest.ResearchWindowStart),
		ResearchWindowEnd: canonicalTime(manifest.ResearchWindowEnd), GeneratedAt: canonicalTime(manifest.GeneratedAt),
		Source: manifest.Source, ArchiveID: manifest.ArchiveID, SnapshotCount: manifest.SnapshotCount,
		CoveragePolicy: struct {
			SchemaVersion     string               `json:"schema_version"`
			PolicyID          string               `json:"policy_id"`
			PartitionModel    string               `json:"partition_model"`
			MaximumGapSeconds *int64               `json:"maximum_gap_seconds"`
			Required          []canonicalPartition `json:"required_partitions"`
			Exceptions        []canonicalException `json:"exceptions"`
		}{manifest.CoveragePolicy.SchemaVersion, manifest.CoveragePolicy.PolicyID, manifest.CoveragePolicy.PartitionModel,
			manifest.CoveragePolicy.MaximumGapSeconds, canonicalPartitions(manifest.CoveragePolicy.Required), canonicalExceptions(manifest.CoveragePolicy.Exceptions)},
		AvailabilityPolicy: struct {
			SchemaVersion                   string `json:"schema_version"`
			PolicyID                        string `json:"policy_id"`
			RequiredPublicationDelaySeconds *int64 `json:"required_publication_delay_seconds"`
		}{manifest.AvailabilityPolicy.SchemaVersion, manifest.AvailabilityPolicy.PolicyID, manifest.AvailabilityPolicy.RequiredPublicationDelaySeconds},
		Snapshots: canonicalSnapshots(manifest.Snapshots),
	}
	return digestCanonical(canonical)
}

func ComputeSnapshotSetDigest(snapshots []SnapshotReference) (string, error) {
	identities := canonicalSnapshots(snapshots)
	for i := range identities {
		identities[i].RelativePath = ""
		identities[i].IngestedAt = ""
	}
	return digestCanonical(identities)
}

func compareDigest(expected, actual string) bool {
	if !validSHA256(expected) || !validSHA256(actual) {
		return false
	}
	expectedBytes, _ := hex.DecodeString(strings.TrimPrefix(strings.ToLower(expected), "sha256:"))
	actualBytes, _ := hex.DecodeString(strings.TrimPrefix(strings.ToLower(actual), "sha256:"))
	if len(expectedBytes) != len(actualBytes) {
		return false
	}
	return subtle.ConstantTimeCompare(expectedBytes, actualBytes) == 1
}

func evidenceID(manifestHash string, cutoff time.Time, snapshotSetDigest string) (string, error) {
	digest, err := digestCanonical(struct {
		ManifestHash      string `json:"manifest_hash"`
		EvaluationCutoff  string `json:"evaluation_cutoff"`
		SnapshotSetDigest string `json:"snapshot_set_digest"`
	}{manifestHash, canonicalTime(cutoff), snapshotSetDigest})
	if err != nil {
		return "", fmt.Errorf("compute evidence ID: %w", err)
	}
	return "pit-evidence:" + strings.TrimPrefix(digest, "sha256:"), nil
}
