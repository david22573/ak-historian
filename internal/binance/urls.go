package binance

import (
	"fmt"
	"strings"
)

type ArchiveSpec struct {
	Market   string
	Symbol   string
	Interval string
	Period   string
	Date     string
}

func BuildArchiveURL(spec ArchiveSpec) (string, error) {
	baseURL := "https://data.binance.vision"
	marketPath := ""

	switch spec.Market {
	case "futures-um":
		marketPath = "data/futures/um"
	case "futures-cm":
		marketPath = "data/futures/cm"
	case "spot":
		marketPath = "data/spot"
	default:
		return "", fmt.Errorf("invalid market: %s", spec.Market)
	}

	symbol := strings.ToUpper(spec.Symbol)

	// Format: {baseURL}/{marketPath}/{period}/klines/{SYMBOL}/{INTERVAL}/{SYMBOL}-{INTERVAL}-{DATE}.zip
	url := fmt.Sprintf("%s/%s/%s/klines/%s/%s/%s-%s-%s.zip",
		baseURL, marketPath, spec.Period, symbol, spec.Interval, symbol, spec.Interval, spec.Date)

	return url, nil
}

func BuildChecksumURL(archiveURL string) string {
	return archiveURL + ".CHECKSUM"
}

func ArchiveFileName(spec ArchiveSpec) string {
	return fmt.Sprintf("%s-%s-%s.zip", strings.ToUpper(spec.Symbol), spec.Interval, spec.Date)
}

func CSVFileName(spec ArchiveSpec) string {
	return fmt.Sprintf("%s-%s-%s.csv", strings.ToUpper(spec.Symbol), spec.Interval, spec.Date)
}

func BaseName(spec ArchiveSpec) string {
	return fmt.Sprintf("%s-%s-%s", strings.ToUpper(spec.Symbol), spec.Interval, spec.Date)
}

func ObjectKey(spec ArchiveSpec) (string, error) {
	// candles/{market}/{interval}/symbol={SYMBOL}/year={YYYY}/month={MM}/{SYMBOL}-{INTERVAL}-{DATE}.parquet
	symbol := strings.ToUpper(spec.Symbol)
	market := strings.ToLower(spec.Market)

	if len(spec.Date) < 7 {
		return "", fmt.Errorf("invalid date format: %s", spec.Date)
	}

	year := spec.Date[0:4]
	month := spec.Date[5:7]

	key := fmt.Sprintf("candles/%s/%s/symbol=%s/year=%s/month=%s/%s-%s-%s.parquet",
		market, spec.Interval, symbol, year, month, symbol, spec.Interval, spec.Date)

	return key, nil
}
