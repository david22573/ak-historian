package archiveauthority

import (
	"errors"
	"sort"
	"strings"
	"time"
)

const PhysicalArchiveIdentitySchemaVersion = "ak-historian.physical-archive-identity.v1"

type PhysicalArchiveIdentity struct {
	SchemaVersion             string    `json:"schema_version"`
	Classification            string    `json:"classification"`
	DatasetID                 string    `json:"dataset_id"`
	DatasetVersion            string    `json:"dataset_version"`
	ManifestID                string    `json:"manifest_id"`
	ManifestHash              string    `json:"manifest_hash"`
	PhysicalCoverageStart     time.Time `json:"physical_coverage_start"`
	PhysicalCoverageEnd       time.Time `json:"physical_coverage_end"`
	SnapshotCount             int       `json:"snapshot_count"`
	SourceSchemaVersion       string    `json:"source_schema_version"`
	SourceSchemaAuthorityHash string    `json:"source_schema_authority_hash"`
	AvailabilityAuthorityHash string    `json:"availability_authority_hash"`
	ProvablePITCoverage       bool      `json:"provable_pit_coverage"`
	IdentityHash              string    `json:"identity_hash"`
}

func SealPhysicalArchiveIdentity(identity PhysicalArchiveIdentity) (PhysicalArchiveIdentity, error) {
	identity.SchemaVersion = PhysicalArchiveIdentitySchemaVersion
	identity.IdentityHash = ""
	if identity.Classification != "IMMUTABLE_PHYSICAL_ARCHIVE_GAP_IDENTITY" || strings.TrimSpace(identity.DatasetID) == "" || !validDigest(identity.DatasetVersion) || strings.TrimSpace(identity.ManifestID) == "" || !validDigest(identity.ManifestHash) || !identity.PhysicalCoverageStart.Before(identity.PhysicalCoverageEnd) || identity.SnapshotCount <= 0 || identity.SourceSchemaVersion != SourceCandleSchemaVersion || !validDigest(identity.SourceSchemaAuthorityHash) || !validDigest(identity.AvailabilityAuthorityHash) || identity.ProvablePITCoverage {
		return PhysicalArchiveIdentity{}, errors.New("physical archive gap identity is invalid")
	}
	hash, err := canonicalHash(identity)
	if err != nil {
		return PhysicalArchiveIdentity{}, err
	}
	identity.IdentityHash = hash
	return identity, nil
}

func ProspectiveReceiptJSONSchema() map[string]any {
	required := []string{"schema_version", "dataset_id", "dataset_version", "source_schema_version", "acquisition_timestamp", "acquisition_evidence_type", "acquisition_evidence_hash", "content_hash", "manifest_relative_identity", "partition_key", "symbol", "covered_period_start", "covered_period_end", "expected_partition", "evaluation_cutoff", "coverage_policy_version", "availability_policy_version", "registration_sequence", "previous_receipt_hash", "receipt_hash"}
	sort.Strings(required)
	return map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema", "$id": ProspectiveReceiptSchemaVersion,
		"title": "Prospective immutable ingestion receipt", "type": "object", "additionalProperties": false,
		"required": required,
		"allOf":    []map[string]any{{"anyOf": []map[string]any{{"required": []string{"source_availability_timestamp"}}, {"required": []string{"source_availability_reference"}}}}},
		"properties": map[string]any{
			"schema_version":                map[string]any{"const": ProspectiveReceiptSchemaVersion},
			"dataset_id":                    map[string]any{"type": "string", "minLength": 1, "pattern": "^[^/\\\\]+$"},
			"dataset_version":               map[string]any{"type": "string", "pattern": "^sha256:[0-9a-f]{64}$"},
			"source_schema_version":         map[string]any{"type": "string", "minLength": 1},
			"acquisition_timestamp":         map[string]any{"type": "string", "format": "date-time"},
			"source_availability_timestamp": map[string]any{"type": "string", "format": "date-time"},
			"source_availability_reference": map[string]any{"type": "string", "minLength": 1},
			"acquisition_evidence_type":     map[string]any{"type": "string", "minLength": 1},
			"acquisition_evidence_hash":     map[string]any{"type": "string", "pattern": "^sha256:[0-9a-f]{64}$"},
			"content_hash":                  map[string]any{"type": "string", "pattern": "^sha256:[0-9a-f]{64}$"},
			"manifest_relative_identity":    map[string]any{"type": "string", "minLength": 1, "pattern": "^(?!/)(?!.*\\.\\.)[^\\\\]+$"},
			"partition_key":                 map[string]any{"type": "string", "minLength": 1}, "symbol": map[string]any{"type": "string", "minLength": 1},
			"covered_period_start": map[string]any{"type": "string", "format": "date-time"}, "covered_period_end": map[string]any{"type": "string", "format": "date-time"},
			"expected_partition": map[string]any{"const": true}, "evaluation_cutoff": map[string]any{"type": "string", "format": "date-time"},
			"coverage_policy_version": map[string]any{"type": "string", "minLength": 1}, "availability_policy_version": map[string]any{"type": "string", "minLength": 1},
			"registration_sequence": map[string]any{"type": "integer", "minimum": 1}, "previous_receipt_hash": map[string]any{"type": "string", "pattern": "^sha256:[0-9a-f]{64}$"}, "receipt_hash": map[string]any{"type": "string", "pattern": "^sha256:[0-9a-f]{64}$"},
		},
	}
}

func ProspectiveManifestContract() map[string]any {
	return map[string]any{
		"schema_version":          "ak-historian.prospective-manifest-contract.v1",
		"manifest_schema_version": ProspectiveManifestSchemaVersion,
		"collector_contract": map[string]any{
			"acquire_once": true, "hash_before_registration": true, "capture_source_availability_at_birth": true,
			"append_only_registration": true, "mutable_aliases_forbidden": true, "local_paths_excluded_from_identity": true,
			"real_pr4b0_r1_collection_authorized": false, "canary_scope": "synthetic or clearly non-research only",
		},
		"validation_command": "ak-historian validate-prospective-manifest --manifest <manifest.json>",
		"recovery_behavior":  "recover only from the last verified receipt hash; orphaned content is quarantined and never auto-registered",
		"duplicate_behavior": "byte-identical receipt/content at the same manifest identity is idempotent",
		"conflict_behavior":  "same manifest identity with different content or receipt hash fails closed and quarantines both claims",
		"required_authority": []string{"dataset ID and version", "source-schema version", "acquisition timestamp", "source availability timestamp or authoritative reference", "acquisition evidence type and hash", "content hash", "manifest-relative identity", "required primary/context coverage", "expected partitions", "evaluation cutoff", "coverage/availability policy versions", "append-only receipt chain"},
	}
}
