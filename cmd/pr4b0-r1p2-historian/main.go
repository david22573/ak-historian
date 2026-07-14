package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/david22573/ak-historian/internal/archiveauthority"
	"github.com/david22573/ak-historian/internal/pitarchive"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var gapPath, archiveRoot, parserPath, outputDir string
	flag.StringVar(&gapPath, "gap-manifest", "evidence/pr4b0_r1p_historian_identity_gap.json", "accepted physical archive gap manifest")
	flag.StringVar(&archiveRoot, "archive-root", "", "physical archive root used only for structural validation")
	flag.StringVar(&parserPath, "parser-source", "internal/converter/duckdb.go", "bound source parser implementation")
	flag.StringVar(&outputDir, "out-dir", "authority", "repository-authoritative output directory")
	flag.Parse()
	if archiveRoot == "" {
		return errors.New("--archive-root is required")
	}

	gap, err := readGap(gapPath)
	if err != nil {
		return err
	}
	if err := pitarchive.VerifyGapManifest(gap); err != nil {
		return fmt.Errorf("verify accepted gap manifest: %w", err)
	}

	searchArtifacts := []archiveauthority.SearchArtifact{
		{ArtifactID: "archive:manifests/local_source_manifest.json", SHA256: "sha256:80ed307f181a16026b0b20970dfe4cfaeb8b1acdf1c9f0298181ecb2bb9ff329", EvidenceType: "LATER_LOCAL_ARCHIVE_VERIFICATION", Disposition: "REJECTED_AS_AVAILABILITY_AUTHORITY", Reason: "last_verified_at and current archive location postdate acquisition and do not preserve source availability"},
		{ArtifactID: "repository:runs/backfill/core_futures_1m_2026-05-29_232247.log", SHA256: "sha256:f691956afb50a15fa85a29c657b725d145198d3085c0ac57eff22194c60d212f", EvidenceType: "FAILED_LATER_BACKFILL_LOG", Disposition: "REJECTED_AS_AVAILABILITY_AUTHORITY", Reason: "2026-05 object checks failed authorization and contain no original acquisition event"},
		{ArtifactID: "repository:runs/backfill/expansion_futures_1m_2026-05-29_232419.log", SHA256: "sha256:b11ea5a6514a0907271582839e5f4fe1a593d91104c1ba37a0d694e940875590", EvidenceType: "FAILED_LATER_BACKFILL_LOG", Disposition: "REJECTED_AS_AVAILABILITY_AUTHORITY", Reason: "2026-05 object checks failed authorization and contain no original acquisition event"},
		{ArtifactID: "repository:runs/reports/r2_restore_candles.json", SHA256: "sha256:fe941652d78e8726db0ab493f8e2cd30205e569032be4bb7378939e97b4d100f", EvidenceType: "PRESENT_DAY_MIGRATION_RESTORE_REPORT", Disposition: "REJECTED_AS_AVAILABILITY_AUTHORITY", Reason: "restore/copy timing after migration cannot establish historical source availability"},
	}
	searchHash, err := archiveauthority.SearchArtifactSetHash(searchArtifacts)
	if err != nil {
		return err
	}
	records := make([]archiveauthority.AvailabilityRecord, 0, len(gap.Snapshots))
	validations := make([]archiveauthority.SnapshotSchemaValidation, 0, len(gap.Snapshots))
	fields := archiveauthority.CanonicalSourceFields()
	for _, snapshot := range gap.Snapshots {
		parts := strings.Split(snapshot.PartitionKey, "/")
		if len(parts) != 4 {
			return fmt.Errorf("invalid partition key %s", snapshot.PartitionKey)
		}
		records = append(records, archiveauthority.AvailabilityRecord{
			ManifestRelativeIdentity: snapshot.RelativePath, PartitionKey: snapshot.PartitionKey, Symbol: parts[2], CoveredMonth: parts[3], ContentHash: snapshot.ContentHash,
			ClaimedSourcePeriodStart: snapshot.EventTimeStart, ClaimedSourcePeriodEnd: snapshot.EventTimeEnd,
			EvidenceType: "NO_AUTHORITATIVE_ACQUISITION_EVIDENCE_FOUND", EvidenceArtifactHash: searchHash,
			Status: archiveauthority.AvailabilityMissing, Reason: "canonical repository, branches/tags/history, archive metadata, manifests, logs, reports, and retained acquisition records contain no authoritative original source-availability evidence",
		})
		observation, inspectErr := archiveauthority.InspectParquetWithDuckDB(filepath.Join(archiveRoot, filepath.FromSlash(snapshot.RelativePath)), parts[0], parts[2], parts[1], "monthly", parts[3])
		validation := archiveauthority.SnapshotSchemaValidation{ManifestRelativeIdentity: snapshot.RelativePath, PartitionKey: snapshot.PartitionKey, ContentHash: snapshot.ContentHash, ClaimedSourcePeriodStart: snapshot.EventTimeStart, ClaimedSourcePeriodEnd: snapshot.EventTimeEnd, Failures: []string{}}
		if inspectErr != nil {
			validation.Status, validation.SchemaVersion, validation.ObservedSchemaFingerprint, validation.Failures = "SCHEMA_VALIDATION_FAILED", "UNKNOWN", "", []string{inspectErr.Error()}
		} else {
			failures, fingerprint, fingerprintErr := archiveauthority.ValidateSchemaObservation(fields, observation)
			if fingerprintErr != nil {
				return fingerprintErr
			}
			validation.ObservedSchemaFingerprint, validation.Failures = fingerprint, failures
			if len(failures) == 0 {
				validation.Status, validation.SchemaVersion = "SCHEMA_VALIDATED", archiveauthority.SourceCandleSchemaVersion
			} else {
				validation.Status, validation.SchemaVersion = "SCHEMA_INVALID", "UNKNOWN"
			}
		}
		validations = append(validations, validation)
	}
	availability, err := archiveauthority.SealAvailabilityAuthority(archiveauthority.AvailabilityAuthority{
		PhysicalArchiveClassification: "IMMUTABLE_PHYSICAL_ARCHIVE_GAP_IDENTITY", DatasetID: gap.DatasetID, DatasetVersion: gap.DatasetVersion, ManifestID: gap.ManifestID, ManifestHash: gap.ManifestHash,
		SearchScope:       []string{"canonical Historian repository tracked files", "all local branches, tags, and Git history", "archive manifests and retained object metadata", "ingestion/acquisition/download journals and receipts", "historical reports and references", "provider-response/publication metadata retained locally"},
		SearchedArtifacts: searchArtifacts, Records: records,
	})
	if err != nil {
		return err
	}
	availabilityGap, err := archiveauthority.BuildAvailabilityGap(availability)
	if err != nil {
		return err
	}
	parserBytes, err := os.ReadFile(parserPath)
	if err != nil {
		return err
	}
	parser := archiveauthority.ParserAuthority{ImplementationPath: "internal/converter/duckdb.go", SourceCommit: "71edbf30c23bf830906be8944ccc0521f2dcc20f", ImplementationHash: bytesHash(parserBytes)}
	sourceSchema, err := archiveauthority.NewSourceSchemaAuthority(parser, validations)
	if err != nil {
		return err
	}
	physical, err := archiveauthority.SealPhysicalArchiveIdentity(archiveauthority.PhysicalArchiveIdentity{
		Classification: "IMMUTABLE_PHYSICAL_ARCHIVE_GAP_IDENTITY", DatasetID: gap.DatasetID, DatasetVersion: gap.DatasetVersion, ManifestID: gap.ManifestID, ManifestHash: gap.ManifestHash,
		PhysicalCoverageStart: gap.PhysicalCoverageStart, PhysicalCoverageEnd: gap.PhysicalCoverageEnd, SnapshotCount: len(gap.Snapshots), SourceSchemaVersion: sourceSchema.SourceSchemaVersion,
		SourceSchemaAuthorityHash: sourceSchema.AuthorityHash, AvailabilityAuthorityHash: availability.AuthorityHash, ProvablePITCoverage: false,
	})
	if err != nil {
		return err
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	artifacts := []struct {
		name     string
		value    any
		markdown string
	}{
		{"source_availability_authority", availability, availabilityMarkdown(availability)},
		{"source_availability_gap_manifest", availabilityGap, availabilityGapMarkdown(availabilityGap)},
		{"source_schema_authority", sourceSchema, sourceSchemaMarkdown(sourceSchema)},
		{"physical_archive_identity", physical, physicalMarkdown(physical)},
		{"prospective_ingestion_receipt_schema", archiveauthority.ProspectiveReceiptJSONSchema(), prospectiveReceiptMarkdown()},
		{"prospective_manifest_contract", archiveauthority.ProspectiveManifestContract(), prospectiveManifestMarkdown()},
	}
	for _, artifact := range artifacts {
		if err := writeJSON(filepath.Join(outputDir, artifact.name+".json"), artifact.value); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(outputDir, artifact.name+".md"), []byte(artifact.markdown), 0o644); err != nil {
			return err
		}
	}
	if sourceSchema.ValidatedCount != len(gap.Snapshots) || sourceSchema.FailedCount != 0 || sourceSchema.MixedSchemaVersions {
		return fmt.Errorf("source schema validation incomplete: validated=%d failed=%d mixed=%t", sourceSchema.ValidatedCount, sourceSchema.FailedCount, sourceSchema.MixedSchemaVersions)
	}
	return nil
}

func readGap(path string) (pitarchive.GapManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return pitarchive.GapManifest{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var gap pitarchive.GapManifest
	if err := decoder.Decode(&gap); err != nil {
		return pitarchive.GapManifest{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return pitarchive.GapManifest{}, errors.New("gap manifest contains trailing JSON")
		}
		return pitarchive.GapManifest{}, err
	}
	return gap, nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func bytesHash(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func availabilityMarkdown(value archiveauthority.AvailabilityAuthority) string {
	return fmt.Sprintf("# Source availability authority\n\nAuthority hash: `%s`\n\nAll %d physical snapshots are `AVAILABILITY_AUTHORITY_MISSING`. Filesystem, verification, migration, restore, and later object timestamps were rejected as substitutes. No availability timestamp was synthesized.\n", value.AuthorityHash, len(value.Records))
}
func availabilityGapMarkdown(value archiveauthority.AvailabilityGapManifest) string {
	return fmt.Sprintf("# Source availability gap manifest\n\nGap hash: `%s`\n\nThis canonical remediation manifest enumerates %d non-verified snapshots.\n", value.GapHash, len(value.Records))
}
func sourceSchemaMarkdown(value archiveauthority.SourceSchemaAuthority) string {
	return fmt.Sprintf("# Source schema authority\n\nVersion: `%s`\n\nFingerprint: `%s`\n\nAuthority hash: `%s`\n\nAll %d snapshots validated structurally against the closed 16-field Parquet schema. This source-candle identity is separate from Engine feature, retained-event, and cluster schemas.\n", value.SourceSchemaVersion, value.SchemaFingerprint, value.AuthorityHash, value.ValidatedCount)
}
func physicalMarkdown(value archiveauthority.PhysicalArchiveIdentity) string {
	return fmt.Sprintf("# Physical archive identity\n\nClassification: **%s**\n\nIdentity hash: `%s`\n\nThe bytes and schema are structurally identified. Provable PIT-valid coverage remains empty because historical source-availability authority is missing.\n", value.Classification, value.IdentityHash)
}
func prospectiveReceiptMarkdown() string {
	return "# Prospective ingestion receipt schema\n\nClosed JSON Schema for immutable append-only acquisition receipts. Every future snapshot must carry source availability authority at birth; local paths and mutable aliases are excluded from identity.\n"
}
func prospectiveManifestMarkdown() string {
	return "# Prospective manifest contract\n\nDesign only; real PR4B0-R1 collection is not authorized in this phase. Validate with `ak-historian validate-prospective-manifest --manifest <manifest.json>`. Identical duplicates are idempotent; conflicting duplicates fail closed.\n"
}
