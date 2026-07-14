package archiveauthority

import (
	"strings"
	"testing"
)

func TestSourceSchemaAuthorityValidationAndIdentity(t *testing.T) {
	fields := CanonicalSourceFields()
	fingerprint, err := SourceSchemaFingerprint(fields)
	if err != nil {
		t.Fatal(err)
	}
	observation := SourceSchemaObservation{Fields: fields, RowCount: 10}
	failures, observed, err := ValidateSchemaObservation(fields, observation)
	if err != nil || len(failures) != 0 || observed != fingerprint {
		t.Fatalf("valid schema failed: %v %v", failures, err)
	}

	missing := observation
	missing.Fields = append([]SourceField{}, fields[:len(fields)-1]...)
	if failures, _, _ := ValidateSchemaObservation(fields, missing); !slicesContain(failures, "SOURCE_SCHEMA_FINGERPRINT_MISMATCH") {
		t.Fatal("missing required source field passed")
	}

	validation := SnapshotSchemaValidation{ManifestRelativeIdentity: "a.parquet", PartitionKey: "p", ContentHash: testDigest('a'), SchemaVersion: SourceCandleSchemaVersion, ObservedSchemaFingerprint: fingerprint, Status: "SCHEMA_VALIDATED", Failures: []string{}}
	parser := ParserAuthority{ImplementationPath: "internal/converter/duckdb.go", SourceCommit: strings.Repeat("a", 40), ImplementationHash: testDigest('b')}
	authority, err := NewSourceSchemaAuthority(parser, []SnapshotSchemaValidation{validation})
	if err != nil {
		t.Fatal(err)
	}
	if authority.ValidatedCount != 1 || authority.FailedCount != 0 || authority.MixedSchemaVersions {
		t.Fatal("valid snapshot was not identified")
	}

	variant := validation
	variant.ManifestRelativeIdentity, variant.ObservedSchemaFingerprint, variant.Status = "b.parquet", testDigest('c'), "SCHEMA_INVALID"
	mixed, err := NewSourceSchemaAuthority(parser, []SnapshotSchemaValidation{validation, variant})
	if err != nil {
		t.Fatal(err)
	}
	if !mixed.MixedSchemaVersions || mixed.FailedCount != 1 {
		t.Fatal("mixed schema versions were not detected")
	}

	mutatedParser := parser
	mutatedParser.ImplementationHash = testDigest('d')
	changed, err := NewSourceSchemaAuthority(mutatedParser, []SnapshotSchemaValidation{validation})
	if err != nil {
		t.Fatal(err)
	}
	if changed.Parser.AuthorityHash == authority.Parser.AuthorityHash || changed.AuthorityHash == authority.AuthorityHash {
		t.Fatal("parser mutation did not change authority identity")
	}

	if authority.TimestampSemantics["open_time_ms"] == "" || authority.TimestampSemantics["availability_time"] == "" {
		t.Fatal("timestamp semantics are not explicit")
	}
	if authority.SourceSchemaVersion == "ak.engine.retained-event.downtrend-midvol-relief.v1" {
		t.Fatal("source schema confused with retained-event schema")
	}
}

func TestSourceStructuralFailures(t *testing.T) {
	observation := SourceSchemaObservation{Fields: CanonicalSourceFields(), RowCount: 1, MissingRequiredRows: 1, OrderingViolations: 1, DuplicateTimestampRows: 1, IntervalBoundaryViolations: 1, PartitionMetadataViolations: 1}
	failures, _, err := ValidateSchemaObservation(CanonicalSourceFields(), observation)
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"SOURCE_REQUIRED_VALUE_MISSING", "SOURCE_TIMESTAMP_ORDER_INVALID", "SOURCE_DUPLICATE_TIMESTAMP", "SOURCE_INTERVAL_BOUNDARY_INVALID", "SOURCE_PARTITION_METADATA_MISMATCH"} {
		if !slicesContain(failures, code) {
			t.Fatalf("missing failure %s", code)
		}
	}
}

func slicesContain(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
