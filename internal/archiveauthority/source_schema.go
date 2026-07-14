package archiveauthority

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

const SourceSchemaAuthoritySchemaVersion = "ak-historian.source-schema-authority.v1"
const SourceCandleSchemaVersion = "ak-historian.source-candle.binance-futures-um-1m.v1"

type SourceField struct {
	Ordinal             int    `json:"ordinal"`
	Name                string `json:"name"`
	PhysicalType        string `json:"physical_type"`
	PhysicalNullability string `json:"physical_nullability"`
	SemanticType        string `json:"semantic_type"`
	Unit                string `json:"unit"`
	Required            bool   `json:"required"`
}

type ParserAuthority struct {
	ImplementationPath string `json:"implementation_path"`
	SourceCommit       string `json:"source_commit"`
	ImplementationHash string `json:"implementation_hash"`
	AuthorityHash      string `json:"authority_hash"`
}

type SourceSchemaObservation struct {
	Fields                      []SourceField `json:"fields"`
	RowCount                    int64         `json:"row_count"`
	MissingRequiredRows         int64         `json:"missing_required_rows"`
	OrderingViolations          int64         `json:"ordering_violations"`
	DuplicateTimestampRows      int64         `json:"duplicate_timestamp_rows"`
	IntervalBoundaryViolations  int64         `json:"interval_boundary_violations"`
	PartitionMetadataViolations int64         `json:"partition_metadata_violations"`
}

type SnapshotSchemaValidation struct {
	ManifestRelativeIdentity  string    `json:"manifest_relative_identity"`
	PartitionKey              string    `json:"partition_key"`
	ContentHash               string    `json:"content_hash"`
	ClaimedSourcePeriodStart  time.Time `json:"claimed_source_period_start"`
	ClaimedSourcePeriodEnd    time.Time `json:"claimed_source_period_end"`
	SchemaVersion             string    `json:"schema_version"`
	ObservedSchemaFingerprint string    `json:"observed_schema_fingerprint"`
	Status                    string    `json:"status"`
	Failures                  []string  `json:"failures"`
}

type SourceSchemaAuthority struct {
	SchemaVersion             string                     `json:"schema_version"`
	SourceSchemaVersion       string                     `json:"source_schema_version"`
	SourceFormat              string                     `json:"source_format"`
	ClosedSchema              bool                       `json:"closed_schema"`
	Fields                    []SourceField              `json:"fields"`
	TimestampSemantics        map[string]string          `json:"timestamp_semantics"`
	OrderingRequirement       string                     `json:"ordering_requirement"`
	DuplicateSemantics        string                     `json:"duplicate_semantics"`
	MissingValuePolicy        string                     `json:"missing_value_policy"`
	PriceUnits                string                     `json:"price_units"`
	VolumeUnits               map[string]string          `json:"volume_units"`
	IntervalBoundarySemantics string                     `json:"interval_boundary_semantics"`
	SchemaEvolution           string                     `json:"schema_evolution"`
	Parser                    ParserAuthority            `json:"parser_authority"`
	ValidationRules           []string                   `json:"validation_rules"`
	SchemaFingerprint         string                     `json:"schema_fingerprint"`
	SnapshotValidations       []SnapshotSchemaValidation `json:"snapshot_validations"`
	ValidatedCount            int                        `json:"validated_count"`
	FailedCount               int                        `json:"failed_count"`
	MixedSchemaVersions       bool                       `json:"mixed_schema_versions"`
	AuthorityHash             string                     `json:"authority_hash"`
}

func CanonicalSourceFields() []SourceField {
	names := []struct{ name, physical, semantic, unit string }{
		{"market", "VARCHAR", "market identifier", "identifier"},
		{"symbol", "VARCHAR", "instrument symbol", "identifier"},
		{"interval", "VARCHAR", "candle interval", "duration label"},
		{"period", "VARCHAR", "source partition cadence", "cadence label"},
		{"source_date", "VARCHAR", "covered UTC calendar month", "YYYY-MM"},
		{"open_time_ms", "BIGINT", "inclusive candle open time", "Unix milliseconds UTC"},
		{"open", "DOUBLE", "opening base-asset price", "quote asset per base asset"},
		{"high", "DOUBLE", "maximum base-asset price", "quote asset per base asset"},
		{"low", "DOUBLE", "minimum base-asset price", "quote asset per base asset"},
		{"close", "DOUBLE", "closing base-asset price", "quote asset per base asset"},
		{"volume", "DOUBLE", "traded base-asset volume", "base asset"},
		{"close_time_ms", "BIGINT", "inclusive candle close time", "Unix milliseconds UTC"},
		{"quote_asset_volume", "DOUBLE", "traded quote-asset volume", "quote asset"},
		{"number_of_trades", "BIGINT", "provider trade count", "count"},
		{"taker_buy_base_volume", "DOUBLE", "taker-buy base-asset volume", "base asset"},
		{"taker_buy_quote_volume", "DOUBLE", "taker-buy quote-asset volume", "quote asset"},
	}
	fields := make([]SourceField, len(names))
	for index, field := range names {
		fields[index] = SourceField{Ordinal: index + 1, Name: field.name, PhysicalType: field.physical, PhysicalNullability: "OPTIONAL_IN_PARQUET_ENCODING", SemanticType: field.semantic, Unit: field.unit, Required: true}
	}
	return fields
}

func SourceSchemaFingerprint(fields []SourceField) (string, error) {
	return canonicalHash(struct {
		Version string        `json:"version"`
		Fields  []SourceField `json:"fields"`
	}{SourceCandleSchemaVersion, fields})
}

func SealParserAuthority(parser ParserAuthority) (ParserAuthority, error) {
	parser.AuthorityHash = ""
	if strings.TrimSpace(parser.ImplementationPath) == "" || len(parser.SourceCommit) != 40 || !validDigest(parser.ImplementationHash) {
		return ParserAuthority{}, errors.New("parser authority is incomplete")
	}
	hash, err := canonicalHash(parser)
	if err != nil {
		return ParserAuthority{}, err
	}
	parser.AuthorityHash = hash
	return parser, nil
}

func NewSourceSchemaAuthority(parser ParserAuthority, validations []SnapshotSchemaValidation) (SourceSchemaAuthority, error) {
	sealedParser, err := SealParserAuthority(parser)
	if err != nil {
		return SourceSchemaAuthority{}, err
	}
	fields := CanonicalSourceFields()
	fingerprint, err := SourceSchemaFingerprint(fields)
	if err != nil {
		return SourceSchemaAuthority{}, err
	}
	validations = append([]SnapshotSchemaValidation{}, validations...)
	sort.Slice(validations, func(i, j int) bool {
		return validations[i].ManifestRelativeIdentity < validations[j].ManifestRelativeIdentity
	})
	validated, failed := 0, 0
	versions := map[string]struct{}{}
	for _, validation := range validations {
		if validation.Status == "SCHEMA_VALIDATED" && validation.SchemaVersion == SourceCandleSchemaVersion && validation.ObservedSchemaFingerprint == fingerprint && len(validation.Failures) == 0 {
			validated++
		} else {
			failed++
		}
		versions[validation.ObservedSchemaFingerprint] = struct{}{}
	}
	authority := SourceSchemaAuthority{
		SchemaVersion: SourceSchemaAuthoritySchemaVersion, SourceSchemaVersion: SourceCandleSchemaVersion,
		SourceFormat: "Apache Parquet generated from Binance futures-um monthly kline CSV", ClosedSchema: true, Fields: fields,
		TimestampSemantics:        map[string]string{"open_time_ms": "inclusive event/open timestamp; not source availability", "close_time_ms": "inclusive final millisecond of the one-minute interval; not source availability", "availability_time": "not present in source candle rows and must come from separate acquisition authority"},
		OrderingRequirement:       "strictly increasing open_time_ms in physical row order",
		DuplicateSemantics:        "duplicate open_time_ms is invalid; no implicit last-write-wins normalization",
		MissingValuePolicy:        "all sixteen logical fields are required; any null fails structural validation even though the physical Parquet encoding is optional",
		PriceUnits:                "quote asset per one base asset; no scaling or normalization",
		VolumeUnits:               map[string]string{"volume": "base asset", "quote_asset_volume": "quote asset", "taker_buy_base_volume": "base asset", "taker_buy_quote_volume": "quote asset", "number_of_trades": "count"},
		IntervalBoundarySemantics: "half-open event interval [open_time_ms, open_time_ms+60000); stored close_time_ms equals open_time_ms+59999",
		SchemaEvolution:           "no archive variant is silently normalized; any differing field set, order, type, or fingerprint is a distinct/unknown schema and fails this v1 authority",
		Parser:                    sealedParser,
		ValidationRules:           []string{"exact closed field names, order, and physical types", "no missing required field values", "strict open-time ordering", "no duplicate open timestamps", "one-minute close boundary equality", "partition market/symbol/interval/period/source_date metadata matches manifest identity", "mixed fingerprints fail closed"},
		SchemaFingerprint:         fingerprint, SnapshotValidations: validations, ValidatedCount: validated, FailedCount: failed, MixedSchemaVersions: len(versions) > 1,
	}
	authority.AuthorityHash = ""
	hash, err := canonicalHash(authority)
	if err != nil {
		return SourceSchemaAuthority{}, err
	}
	authority.AuthorityHash = hash
	return authority, nil
}

func ValidateSchemaObservation(authorityFields []SourceField, observation SourceSchemaObservation) ([]string, string, error) {
	wantFingerprint, err := SourceSchemaFingerprint(authorityFields)
	if err != nil {
		return nil, "", err
	}
	observedFingerprint, err := SourceSchemaFingerprint(observation.Fields)
	if err != nil {
		return nil, "", err
	}
	failures := []string{}
	if wantFingerprint != observedFingerprint {
		failures = append(failures, "SOURCE_SCHEMA_FINGERPRINT_MISMATCH")
	}
	if observation.RowCount <= 0 {
		failures = append(failures, "SOURCE_SNAPSHOT_EMPTY")
	}
	if observation.MissingRequiredRows != 0 {
		failures = append(failures, "SOURCE_REQUIRED_VALUE_MISSING")
	}
	if observation.OrderingViolations != 0 {
		failures = append(failures, "SOURCE_TIMESTAMP_ORDER_INVALID")
	}
	if observation.DuplicateTimestampRows != 0 {
		failures = append(failures, "SOURCE_DUPLICATE_TIMESTAMP")
	}
	if observation.IntervalBoundaryViolations != 0 {
		failures = append(failures, "SOURCE_INTERVAL_BOUNDARY_INVALID")
	}
	if observation.PartitionMetadataViolations != 0 {
		failures = append(failures, "SOURCE_PARTITION_METADATA_MISMATCH")
	}
	return failures, observedFingerprint, nil
}

type duckDBDescription struct {
	ColumnName string `json:"column_name"`
	ColumnType string `json:"column_type"`
	Null       string `json:"null"`
}

type duckDBStats struct {
	RowCount                    int64 `json:"row_count"`
	MissingRequiredRows         int64 `json:"missing_required_rows"`
	OrderingViolations          int64 `json:"ordering_violations"`
	DuplicateTimestampRows      int64 `json:"duplicate_timestamp_rows"`
	IntervalBoundaryViolations  int64 `json:"interval_boundary_violations"`
	PartitionMetadataViolations int64 `json:"partition_metadata_violations"`
}

func InspectParquetWithDuckDB(path, market, symbol, interval, period, sourceDate string) (SourceSchemaObservation, error) {
	if _, err := exec.LookPath("duckdb"); err != nil {
		return SourceSchemaObservation{}, errors.New("duckdb is required for archive structural validation")
	}
	escape := func(value string) string { return strings.ReplaceAll(value, "'", "''") }
	quotedPath := escape(path)
	describe := fmt.Sprintf("DESCRIBE SELECT * FROM read_parquet('%s', hive_partitioning=false);", quotedPath)
	output, err := exec.Command("duckdb", "-json", "-c", describe).CombinedOutput()
	if err != nil {
		return SourceSchemaObservation{}, fmt.Errorf("describe parquet: %w: %s", err, output)
	}
	var descriptions []duckDBDescription
	if err := json.Unmarshal(output, &descriptions); err != nil {
		return SourceSchemaObservation{}, fmt.Errorf("decode parquet schema: %w", err)
	}
	fields := make([]SourceField, len(descriptions))
	canonical := CanonicalSourceFields()
	for index, description := range descriptions {
		semantic, unit := "unknown", "unknown"
		if index < len(canonical) && canonical[index].Name == description.ColumnName {
			semantic, unit = canonical[index].SemanticType, canonical[index].Unit
		}
		fields[index] = SourceField{Ordinal: index + 1, Name: description.ColumnName, PhysicalType: description.ColumnType, PhysicalNullability: map[string]string{"YES": "OPTIONAL_IN_PARQUET_ENCODING", "NO": "REQUIRED_IN_PARQUET_ENCODING"}[description.Null], SemanticType: semantic, Unit: unit, Required: true}
	}
	query := fmt.Sprintf(`WITH ordered AS (
  SELECT *, lag(open_time_ms) OVER () AS prior_open_time_ms
  FROM read_parquet('%s', hive_partitioning=false)
)
SELECT
  count(*)::BIGINT AS row_count,
  coalesce(sum(CASE WHEN market IS NULL OR symbol IS NULL OR interval IS NULL OR period IS NULL OR source_date IS NULL OR open_time_ms IS NULL OR open IS NULL OR high IS NULL OR low IS NULL OR close IS NULL OR volume IS NULL OR close_time_ms IS NULL OR quote_asset_volume IS NULL OR number_of_trades IS NULL OR taker_buy_base_volume IS NULL OR taker_buy_quote_volume IS NULL THEN 1 ELSE 0 END),0)::BIGINT AS missing_required_rows,
  coalesce(sum(CASE WHEN prior_open_time_ms IS NOT NULL AND open_time_ms <= prior_open_time_ms THEN 1 ELSE 0 END),0)::BIGINT AS ordering_violations,
  (count(*) - count(DISTINCT open_time_ms))::BIGINT AS duplicate_timestamp_rows,
  coalesce(sum(CASE WHEN close_time_ms <> open_time_ms + 59999 THEN 1 ELSE 0 END),0)::BIGINT AS interval_boundary_violations,
  coalesce(sum(CASE WHEN market <> '%s' OR symbol <> '%s' OR interval <> '%s' OR period <> '%s' OR source_date <> '%s' THEN 1 ELSE 0 END),0)::BIGINT AS partition_metadata_violations
FROM ordered;`, quotedPath, escape(market), escape(symbol), escape(interval), escape(period), escape(sourceDate))
	output, err = exec.Command("duckdb", "-json", "-c", query).CombinedOutput()
	if err != nil {
		return SourceSchemaObservation{}, fmt.Errorf("validate parquet structure: %w: %s", err, output)
	}
	var stats []duckDBStats
	if err := json.Unmarshal(output, &stats); err != nil || len(stats) != 1 {
		return SourceSchemaObservation{}, fmt.Errorf("decode parquet validation statistics: %w", err)
	}
	return SourceSchemaObservation{Fields: fields, RowCount: stats[0].RowCount, MissingRequiredRows: stats[0].MissingRequiredRows, OrderingViolations: stats[0].OrderingViolations, DuplicateTimestampRows: stats[0].DuplicateTimestampRows, IntervalBoundaryViolations: stats[0].IntervalBoundaryViolations, PartitionMetadataViolations: stats[0].PartitionMetadataViolations}, nil
}
