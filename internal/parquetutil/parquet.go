package parquetutil

import (
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"

	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/reader"
)

type OpenTimeRow struct {
	OpenTimeMS int64 `parquet:"name=open_time_ms, type=INT64"`
}

type Stats struct {
	RowCount      int64
	MinOpenTimeMS int64
	MaxOpenTimeMS int64
}

func ReadOpenTimes(paths []string) ([]int64, error) {
	if _, err := exec.LookPath("duckdb"); err == nil {
		return readOpenTimesDuckDB(paths)
	}

	return ReadOpenTimesStrict(paths)
}

func ReadOpenTimesStrict(paths []string) ([]int64, error) {
	openTimes := make([]int64, 0)
	for _, path := range paths {
		rows, err := readRows(path)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			openTimes = append(openTimes, row.OpenTimeMS)
		}
	}
	return openTimes, nil
}

func readOpenTimesDuckDB(paths []string) ([]int64, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	quoted := make([]string, 0, len(paths))
	for _, path := range paths {
		quoted = append(quoted, "'"+strings.ReplaceAll(path, "'", "''")+"'")
	}

	query := fmt.Sprintf(
		"COPY (SELECT open_time_ms FROM read_parquet([%s]) ORDER BY open_time_ms) TO STDOUT (FORMAT CSV, HEADER FALSE);",
		strings.Join(quoted, ", "),
	)

	cmd := exec.Command("duckdb", "-c", query)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("duckdb read open times failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	openTimes := make([]int64, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		v, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse duckdb open_time_ms %q: %w", line, err)
		}
		openTimes = append(openTimes, v)
	}
	return openTimes, nil
}

func ReadStats(path string) (Stats, error) {
	rows, err := readRows(path)
	if err != nil {
		return Stats{}, err
	}
	if len(rows) == 0 {
		return Stats{}, fmt.Errorf("parquet file is empty")
	}

	stats := Stats{
		RowCount:      int64(len(rows)),
		MinOpenTimeMS: math.MaxInt64,
		MaxOpenTimeMS: math.MinInt64,
	}
	for _, row := range rows {
		if row.OpenTimeMS < stats.MinOpenTimeMS {
			stats.MinOpenTimeMS = row.OpenTimeMS
		}
		if row.OpenTimeMS > stats.MaxOpenTimeMS {
			stats.MaxOpenTimeMS = row.OpenTimeMS
		}
	}
	return stats, nil
}

func readRows(path string) (rows []OpenTimeRow, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("read parquet rows %s: panic: %v", path, r)
		}
	}()

	fr, err := local.NewLocalFileReader(path)
	if err != nil {
		return nil, fmt.Errorf("open parquet file %s: %w", path, err)
	}
	defer fr.Close()

	pr, err := reader.NewParquetReader(fr, new(OpenTimeRow), 1)
	if err != nil {
		return nil, fmt.Errorf("create parquet reader %s: %w", path, err)
	}
	defer pr.ReadStop()

	total := int(pr.GetNumRows())
	rows = make([]OpenTimeRow, 0, total)
	batchSize := 1024
	for total > 0 {
		n := batchSize
		if total < n {
			n = total
		}
		batch := make([]OpenTimeRow, n)
		if err := pr.Read(&batch); err != nil {
			return nil, fmt.Errorf("read parquet rows %s: %w", path, err)
		}
		rows = append(rows, batch...)
		total -= n
	}

	return rows, nil
}
