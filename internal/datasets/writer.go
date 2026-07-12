package datasets

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/david22573/ak-historian/internal/datasets/derivatives"
	"github.com/david22573/ak-historian/internal/datasets/sentiment"
)

func WriteSentimentRowsJSON(path string, rows []sentiment.Row) error {
	data, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sentiment rows: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write JSON file: %w", err)
	}
	return nil
}

func WriteSentimentRowsCSV(path string, rows []sentiment.Row) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create CSV file: %w", err)
	}
	defer file.Close()

	w := csv.NewWriter(file)
	defer w.Flush()

	header := []string{
		"source", "dataset", "scope", "symbol", "interval",
		"event_time_ms", "available_at_ms", "ingested_at_ms",
		"score", "label", "intensity",
		"mentions", "positive", "negative", "neutral",
		"source_version",
	}

	if err := w.Write(header); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	for _, r := range rows {
		record := []string{
			r.Source, r.Dataset, r.Scope, r.Symbol, r.Interval,
			strconv.FormatInt(r.EventTimeMS, 10),
			strconv.FormatInt(r.AvailableAtMS, 10),
			strconv.FormatInt(r.IngestedAtMS, 10),
			strconv.FormatFloat(r.Score, 'f', -1, 64),
			r.Label,
			strconv.FormatFloat(r.Intensity, 'f', -1, 64),
			strconv.FormatInt(r.Mentions, 10),
			strconv.FormatInt(r.Positive, 10),
			strconv.FormatInt(r.Negative, 10),
			strconv.FormatInt(r.Neutral, 10),
			r.SourceVersion,
		}
		if err := w.Write(record); err != nil {
			return fmt.Errorf("write record: %w", err)
		}
	}

	return nil
}

func WriteSentimentRowsParquet(csvPath, parquetPath string) error {
	_, err := exec.LookPath("duckdb")
	if err != nil {
		return fmt.Errorf("parquet output requires duckdb installed")
	}

	escapedCsv := strings.ReplaceAll(csvPath, "'", "''")
	escapedParquet := strings.ReplaceAll(parquetPath, "'", "''")

	query := fmt.Sprintf(
		"COPY (SELECT * FROM read_csv_auto('%s')) TO '%s' (FORMAT PARQUET, COMPRESSION ZSTD);",
		escapedCsv, escapedParquet,
	)

	cmd := exec.Command("duckdb", "-c", query)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("duckdb parquet conversion failed: %s: %w", string(output), err)
	}

	return nil
}

func WriteDerivativesRowsJSON(path string, rows []derivatives.Row) error {
	data, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal derivatives rows: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write JSON file: %w", err)
	}
	return nil
}

func WriteDerivativesRowsCSV(path string, rows []derivatives.Row) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create CSV file: %w", err)
	}
	defer file.Close()

	w := csv.NewWriter(file)
	defer w.Flush()

	header := []string{
		"source", "dataset", "market", "symbol", "interval",
		"event_time_ms", "available_at_ms", "ingested_at_ms",
		"value", "extra_1", "extra_2", "source_version",
	}
	if err := w.Write(header); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	for _, r := range rows {
		record := []string{
			r.Source, r.Dataset, r.Market, r.Symbol, r.Interval,
			strconv.FormatInt(r.EventTimeMS, 10),
			strconv.FormatInt(r.AvailableAtMS, 10),
			strconv.FormatInt(r.IngestedAtMS, 10),
			strconv.FormatFloat(r.Value, 'f', -1, 64),
			strconv.FormatFloat(r.Extra1, 'f', -1, 64),
			strconv.FormatFloat(r.Extra2, 'f', -1, 64),
			r.SourceVersion,
		}
		if err := w.Write(record); err != nil {
			return fmt.Errorf("write record: %w", err)
		}
	}

	return nil
}

func WriteDerivativesRowsParquet(csvPath, parquetPath string) error {
	_, err := exec.LookPath("duckdb")
	if err != nil {
		return fmt.Errorf("parquet output requires duckdb installed")
	}

	escapedCsv := strings.ReplaceAll(csvPath, "'", "''")
	escapedParquet := strings.ReplaceAll(parquetPath, "'", "''")

	query := fmt.Sprintf(
		"COPY (SELECT * FROM read_csv_auto('%s')) TO '%s' (FORMAT PARQUET, COMPRESSION ZSTD);",
		escapedCsv, escapedParquet,
	)

	cmd := exec.Command("duckdb", "-c", query)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("duckdb parquet conversion failed: %s: %w", string(output), err)
	}

	return nil
}
