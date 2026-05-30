package binance

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type DownloadStatus string

const (
	Downloaded DownloadStatus = "downloaded"
	NotFound   DownloadStatus = "not_found"
	Reused     DownloadStatus = "reused"
)

type Downloader struct {
	Client *http.Client
}

func NewDownloader() *Downloader {
	return &Downloader{
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type HTTPStatusError struct {
	StatusCode int
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("unexpected status code: %d", e.StatusCode)
}

func (e *HTTPStatusError) IsRetryable() bool {
	return e.StatusCode == http.StatusTooManyRequests || (e.StatusCode >= 500 && e.StatusCode <= 599)
}

func (d *Downloader) DownloadChecksum(ctx context.Context, checksumURL string) (string, error) {
	return DownloadChecksum(ctx, d.Client, checksumURL)
}

func (d *Downloader) DownloadArchive(
	ctx context.Context,
	url string,
	destPath string,
	force bool,
) (DownloadStatus, error) {
	if !force {
		if _, err := os.Stat(destPath); err == nil {
			return Reused, nil
		}
	}

	err := os.MkdirAll(filepath.Dir(destPath), 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	tmpPath := destPath + ".tmp"

	// Retry loop
	var status DownloadStatus
	err = retry(ctx, 3, 1*time.Second, func() error {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", "ak-historian/1.0")

		resp, err := d.Client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			status = NotFound
			return nil
		}

		if resp.StatusCode != http.StatusOK {
			return &HTTPStatusError{StatusCode: resp.StatusCode}
		}

		out, err := os.Create(tmpPath)
		if err != nil {
			return err
		}
		defer out.Close()

		_, err = io.Copy(out, resp.Body)
		if err != nil {
			return err
		}

		status = Downloaded
		return nil
	})

	if err != nil {
		return "", err
	}

	if status == NotFound {
		return NotFound, nil
	}

	err = os.Rename(tmpPath, destPath)
	if err != nil {
		return "", fmt.Errorf("failed to rename tmp file: %w", err)
	}

	return Downloaded, nil
}

func retry(ctx context.Context, maxRetries int, baseDelay time.Duration, fn func() error) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		err = fn()
		if err == nil {
			return nil
		}

		// Don't retry if context is cancelled
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Check if error is retryable
		if statusErr, ok := err.(*HTTPStatusError); ok {
			if !statusErr.IsRetryable() {
				return err
			}
		}

		if i < maxRetries-1 {
			select {
			case <-time.After(baseDelay * time.Duration(1<<i)):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return err
}
