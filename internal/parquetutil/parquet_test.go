package parquetutil

import (
	"path/filepath"
	"testing"

	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/writer"
)

func TestReadOpenTimesAndStats(t *testing.T) {
	path := filepath.Join(t.TempDir(), "candles.parquet")
	fw, err := local.NewLocalFileWriter(path)
	if err != nil {
		t.Fatalf("NewLocalFileWriter: %v", err)
	}

	pw, err := writer.NewParquetWriter(fw, new(OpenTimeRow), 1)
	if err != nil {
		t.Fatalf("NewParquetWriter: %v", err)
	}

	rows := []OpenTimeRow{
		{OpenTimeMS: 1000},
		{OpenTimeMS: 2000},
		{OpenTimeMS: 3000},
	}
	for _, row := range rows {
		if err := pw.Write(row); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if err := pw.WriteStop(); err != nil {
		t.Fatalf("WriteStop: %v", err)
	}
	if err := fw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	openTimes, err := ReadOpenTimes([]string{path})
	if err != nil {
		t.Fatalf("ReadOpenTimes: %v", err)
	}
	if len(openTimes) != 3 || openTimes[0] != 1000 || openTimes[2] != 3000 {
		t.Fatalf("unexpected openTimes: %#v", openTimes)
	}

	stats, err := ReadStats(path)
	if err != nil {
		t.Fatalf("ReadStats: %v", err)
	}
	if stats.RowCount != 3 || stats.MinOpenTimeMS != 1000 || stats.MaxOpenTimeMS != 3000 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}
