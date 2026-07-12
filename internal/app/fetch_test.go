package app

import (
	"context"
	"fmt"
	"testing"

	"github.com/david22573/ak-historian/internal/binance"
)

type fakeObjectStore struct {
	exists func(string) (bool, error)
	upload func(string, string) error
}

func (f *fakeObjectStore) ObjectExists(ctx context.Context, key string) (bool, error) {
	if f.exists != nil {
		return f.exists(key)
	}
	return false, nil
}

func (f *fakeObjectStore) UploadFile(ctx context.Context, localPath string, objectKey string) error {
	if f.upload != nil {
		return f.upload(localPath, objectKey)
	}
	return nil
}

type fakeDownloader struct {
	download func(string) (binance.DownloadStatus, error)
	checksum func(string) (string, error)
}

func (f *fakeDownloader) DownloadArchive(ctx context.Context, url string, destPath string, force bool) (binance.DownloadStatus, error) {
	if f.download != nil {
		return f.download(url)
	}
	return binance.Downloaded, nil
}

func (f *fakeDownloader) DownloadChecksum(ctx context.Context, url string) (string, error) {
	if f.checksum != nil {
		return f.checksum(url)
	}
	return "fake-checksum", nil
}

func TestRunFetch_Orchestration(t *testing.T) {
	opts := FetchOptions{
		Market:      "spot",
		Symbols:     []string{"BTCUSDT"},
		Interval:    "1m",
		Period:      "daily",
		Start:       "2024-01-01",
		End:         "2024-01-01",
		WorkDir:     "tmp-work",
		Concurrency: 1,
		DryRun:      true,
	}

	t.Run("dry run plans only", func(t *testing.T) {
		r2 := &fakeObjectStore{}
		dl := &fakeDownloader{}

		err := runFetch(context.Background(), opts, r2, dl)
		if err != nil {
			t.Fatalf("runFetch failed: %v", err)
		}
	})

	t.Run("failed item does not stop others", func(t *testing.T) {
		optsMulti := opts
		optsMulti.Symbols = []string{"FAIL", "OK"}
		optsMulti.DryRun = false

		r2 := &fakeObjectStore{}
		dl := &fakeDownloader{
			download: func(url string) (binance.DownloadStatus, error) {
				if url == "https://data.binance.vision/data/spot/daily/klines/FAIL/1m/FAIL-1m-2024-01-01.zip" {
					return "", fmt.Errorf("forced failure")
				}
				return binance.Downloaded, nil
			},
		}

		err := runFetch(context.Background(), optsMulti, r2, dl)
		if err == nil {
			t.Error("Expected overall error due to failures, got nil")
		}
	})
}
