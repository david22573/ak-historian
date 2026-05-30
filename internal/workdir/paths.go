package workdir

import (
	"path/filepath"
	"strings"

	"github.com/davidmiguel22573/ak-historian/internal/binance"
)

type Paths struct {
	Root        string
	ItemDir     string
	ZipPath     string
	CSVPath     string
	ParquetPath string
}

func BuildPaths(root string, spec binance.ArchiveSpec) (Paths, error) {
	symbol := strings.ToUpper(spec.Symbol)
	market := strings.ToLower(spec.Market)

	// .ak-historian/work/{market}/{interval}/{symbol}/{period}/{date}/
	itemDir := filepath.Join(root, market, spec.Interval, symbol, spec.Period, spec.Date)

	baseName := binance.BaseName(spec)

	return Paths{
		Root:        root,
		ItemDir:     itemDir,
		ZipPath:     filepath.Join(itemDir, baseName+".zip"),
		CSVPath:     filepath.Join(itemDir, baseName+".csv"),
		ParquetPath: filepath.Join(itemDir, baseName+".parquet"),
	}, nil
}
