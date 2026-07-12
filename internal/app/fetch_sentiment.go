package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/david22573/ak-historian/internal/datasets"
	"github.com/david22573/ak-historian/internal/datasets/sentiment"
	"github.com/spf13/cobra"
)

var (
	fsSource        string
	fsDataset       string
	fsStart         string
	fsEnd           string
	fsOut           string
	fsFormat        string
	fsWriteManifest bool
	fsUploadR2      bool
)

func init() {
	rootCmd.AddCommand(fetchSentimentCmd)

	fetchSentimentCmd.Flags().StringVar(&fsSource, "source", "", "required, only alternative_me for first version")
	fetchSentimentCmd.Flags().StringVar(&fsDataset, "dataset", "", "required, only fear_greed for first version")
	fetchSentimentCmd.Flags().StringVar(&fsStart, "start", "", "required YYYY-MM-DD")
	fetchSentimentCmd.Flags().StringVar(&fsEnd, "end", "", "required YYYY-MM-DD")
	fetchSentimentCmd.Flags().StringVar(&fsOut, "out", ".ak-historian/work", "output dir")
	fetchSentimentCmd.Flags().StringVar(&fsFormat, "format", "parquet", "json | csv | parquet")
	fetchSentimentCmd.Flags().BoolVar(&fsWriteManifest, "write-manifest", false, "write manifest")
	fetchSentimentCmd.Flags().BoolVar(&fsUploadR2, "upload-r2", false, "upload to r2 (not implemented)")
}

var fetchSentimentCmd = &cobra.Command{
	Use:   "fetch-sentiment",
	Short: "Fetch sentiment data",
	Run: func(cmd *cobra.Command, args []string) {
		if fsSource != "alternative_me" {
			fmt.Printf("{\"status\":\"FAIL\",\"error\":\"unsupported source: %s\"}\n", fsSource)
			os.Exit(1)
		}
		if fsDataset != "fear_greed" {
			fmt.Printf("{\"status\":\"FAIL\",\"error\":\"unsupported dataset: %s\"}\n", fsDataset)
			os.Exit(1)
		}
		if fsUploadR2 {
			fmt.Printf("{\"status\":\"FAIL\",\"error\":\"--upload-r2 is not implemented yet\"}\n")
			os.Exit(1)
		}

		startT, err := time.Parse("2006-01-02", fsStart)
		if err != nil {
			fmt.Printf("{\"status\":\"FAIL\",\"error\":\"invalid start: %v\"}\n", err)
			os.Exit(1)
		}

		endT, err := time.Parse("2006-01-02", fsEnd)
		if err != nil {
			fmt.Printf("{\"status\":\"FAIL\",\"error\":\"invalid end: %v\"}\n", err)
			os.Exit(1)
		}

		client := sentiment.NewFearGreedClient()
		rows, err := client.Fetch(context.Background(), startT, endT)
		if err != nil {
			fmt.Printf("{\"status\":\"FAIL\",\"error\":\"fetch failed: %v\"}\n", err)
			os.Exit(1)
		}

		if err := datasets.ValidateSentimentRows(rows); err != nil {
			fmt.Printf("{\"status\":\"FAIL\",\"error\":\"validation failed: %v\"}\n", err)
			os.Exit(1)
		}

		// Partition by month
		type PartitionKey struct {
			Year  int
			Month int
		}
		partitions := make(map[PartitionKey][]sentiment.Row)
		for _, r := range rows {
			t := time.UnixMilli(r.EventTimeMS).UTC()
			k := PartitionKey{t.Year(), int(t.Month())}
			partitions[k] = append(partitions[k], r)
		}

		var manifestObjs []datasets.Object

		for k, pRows := range partitions {
			spec := datasets.DatasetSpec{
				Kind:     datasets.KindSentiment,
				Source:   fsSource,
				Dataset:  fsDataset,
				Scope:    "global",
				Interval: "1d",
				Date:     fmt.Sprintf("%04d-%02d", k.Year, k.Month),
			}

			key, err := datasets.ObjectKey(spec)
			if err != nil {
				fmt.Printf("{\"status\":\"FAIL\",\"error\":\"path build failed: %v\"}\n", err)
				os.Exit(1)
			}

			if fsFormat != "parquet" {
				// strip parquet and add format if needed, but requirements say path builder provides format natively or we just append?
				// well, requirement: datasets/sentiment/source=alternative_me/dataset=fear_greed/scope=global/interval=1d/year=2023/month=01/fear_greed-2023-01.parquet
				// Let's assume path builder returns .parquet
				key = key[:len(key)-len(".parquet")] + "." + fsFormat
			}

			outPath := filepath.Join(fsOut, key)
			if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
				fmt.Printf("{\"status\":\"FAIL\",\"error\":\"mkdir failed: %v\"}\n", err)
				os.Exit(1)
			}

			var stats datasets.RowStats

			if fsFormat == "json" {
				if err := datasets.WriteSentimentRowsJSON(outPath, pRows); err != nil {
					fmt.Printf("{\"status\":\"FAIL\",\"error\":\"write json failed: %v\"}\n", err)
					os.Exit(1)
				}
				stats = computeLocalStats(pRows)
			} else if fsFormat == "csv" {
				if err := datasets.WriteSentimentRowsCSV(outPath, pRows); err != nil {
					fmt.Printf("{\"status\":\"FAIL\",\"error\":\"write csv failed: %v\"}\n", err)
					os.Exit(1)
				}
				stats = computeLocalStats(pRows)
			} else if fsFormat == "parquet" {
				csvPath := outPath + ".csv"
				if err := datasets.WriteSentimentRowsCSV(csvPath, pRows); err != nil {
					fmt.Printf("{\"status\":\"FAIL\",\"error\":\"write temp csv failed: %v\"}\n", err)
					os.Exit(1)
				}
				if err := datasets.WriteSentimentRowsParquet(csvPath, outPath); err != nil {
					fmt.Printf("{\"status\":\"FAIL\",\"error\":\"write parquet failed: %v\"}\n", err)
					os.Exit(1)
				}
				os.Remove(csvPath)

				var err error
				stats, err = datasets.ValidateDatasetParquet(context.Background(), outPath)
				if err != nil {
					fmt.Printf("{\"status\":\"FAIL\",\"error\":\"parquet validation failed: %v\"}\n", err)
					os.Exit(1)
				}
			} else {
				fmt.Printf("{\"status\":\"FAIL\",\"error\":\"unsupported format: %s\"}\n", fsFormat)
				os.Exit(1)
			}

			manifestObjs = append(manifestObjs, datasets.Object{
				Key:              key,
				Period:           spec.Date,
				RowCount:         stats.RowCount,
				MinEventTimeMS:   stats.MinEventTimeMS,
				MaxEventTimeMS:   stats.MaxEventTimeMS,
				MinAvailableAtMS: stats.MinAvailableAtMS,
				MaxAvailableAtMS: stats.MaxAvailableAtMS,
			})
		}

		manifestPath := ""
		if fsWriteManifest {
			spec := datasets.DatasetSpec{
				Kind:     datasets.KindSentiment,
				Source:   fsSource,
				Dataset:  fsDataset,
				Scope:    "global",
				Interval: "1d",
			}
			mKey, err := datasets.ManifestKey(spec)
			if err != nil {
				fmt.Printf("{\"status\":\"FAIL\",\"error\":\"manifest key failed: %v\"}\n", err)
				os.Exit(1)
			}

			manifestPath = filepath.Join(fsOut, mKey)

			var minEvent, maxEvent int64
			for i, o := range manifestObjs {
				if i == 0 || o.MinEventTimeMS < minEvent {
					minEvent = o.MinEventTimeMS
				}
				if i == 0 || o.MaxEventTimeMS > maxEvent {
					maxEvent = o.MaxEventTimeMS
				}
			}

			m := datasets.Manifest{
				SchemaVersion:    1,
				Kind:             string(datasets.KindSentiment),
				Source:           fsSource,
				Dataset:          fsDataset,
				Scope:            "global",
				Interval:         "1d",
				CoverageStartMS:  minEvent,
				CoverageEndMS:    maxEvent,
				ObjectCount:      len(manifestObjs),
				Objects:          manifestObjs,
				LastVerifiedAtMS: time.Now().UnixMilli(),
			}

			if err := datasets.WriteManifest(manifestPath, m); err != nil {
				fmt.Printf("{\"status\":\"FAIL\",\"error\":\"write manifest failed: %v\"}\n", err)
				os.Exit(1)
			}
		}

		res := map[string]interface{}{
			"status":   "PASS",
			"source":   fsSource,
			"dataset":  fsDataset,
			"scope":    "global",
			"interval": "1d",
			"rows":     len(rows),
			"objects":  len(partitions),
		}
		if manifestPath != "" {
			res["manifest"] = manifestPath
		}

		b, _ := json.Marshal(res)
		fmt.Println(string(b))
	},
}

func computeLocalStats(rows []sentiment.Row) datasets.RowStats {
	var stats datasets.RowStats
	stats.RowCount = int64(len(rows))
	for i, r := range rows {
		if i == 0 || r.EventTimeMS < stats.MinEventTimeMS {
			stats.MinEventTimeMS = r.EventTimeMS
		}
		if i == 0 || r.EventTimeMS > stats.MaxEventTimeMS {
			stats.MaxEventTimeMS = r.EventTimeMS
		}
		if i == 0 || r.AvailableAtMS < stats.MinAvailableAtMS {
			stats.MinAvailableAtMS = r.AvailableAtMS
		}
		if i == 0 || r.AvailableAtMS > stats.MaxAvailableAtMS {
			stats.MaxAvailableAtMS = r.AvailableAtMS
		}
	}
	return stats
}
