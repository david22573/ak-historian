package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/david22573/ak-historian/internal/parquetutil"
	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/writer"
)

func main() {
	path := "testdata/parquet_small/candles/futures-um/1m/symbol=BTCUSDT/year=2024/month=01/BTCUSDT-1m-2024-01-01.parquet"

	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		panic(err)
	}

	fw, err := local.NewLocalFileWriter(path)
	if err != nil {
		panic(err)
	}
	defer fw.Close()

	pw, err := writer.NewParquetWriter(fw, new(parquetutil.OpenTimeRow), 4)
	if err != nil {
		panic(err)
	}
	defer pw.WriteStop()

	// write 10 rows
	for i := 0; i < 10; i++ {
		pw.Write(parquetutil.OpenTimeRow{OpenTimeMS: int64(1704067200000 + i*60000)})
	}
	fmt.Println("wrote", path)
}
