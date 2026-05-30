package binance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestDownloader_DownloadArchive(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/found.zip" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("zip-content"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	d := NewDownloader()
	tmpDir, _ := os.MkdirTemp("", "downloader-test")
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		url      string
		destPath string
		force    bool
		want     DownloadStatus
		wantErr  bool
	}{
		{
			name:     "download success",
			url:      ts.URL + "/found.zip",
			destPath: filepath.Join(tmpDir, "found.zip"),
			force:    false,
			want:     Downloaded,
		},
		{
			name:     "download 404",
			url:      ts.URL + "/notfound.zip",
			destPath: filepath.Join(tmpDir, "notfound.zip"),
			force:    false,
			want:     NotFound,
		},
		{
			name:     "reuse existing",
			url:      ts.URL + "/found.zip",
			destPath: filepath.Join(tmpDir, "found.zip"),
			force:    false,
			want:     Reused,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := d.DownloadArchive(context.Background(), tt.url, tt.destPath, tt.force)
			if (err != nil) != tt.wantErr {
				t.Errorf("DownloadArchive() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("DownloadArchive() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDownloader_Retry(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "retry-test")
	defer os.RemoveAll(tmpDir)

	d := NewDownloader()

	t.Run("retry 429 then success", func(t *testing.T) {
		var calls int32
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if atomic.AddInt32(&calls, 1) < 2 {
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}))
		defer ts.Close()

		status, err := d.DownloadArchive(context.Background(), ts.URL, filepath.Join(tmpDir, "retry429.zip"), true)
		if err != nil {
			t.Errorf("Expected success, got %v", err)
		}
		if status != Downloaded {
			t.Errorf("Expected Downloaded, got %v", status)
		}
		if atomic.LoadInt32(&calls) != 2 {
			t.Errorf("Expected 2 calls, got %d", calls)
		}
	})

	t.Run("no retry 403", func(t *testing.T) {
		var calls int32
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&calls, 1)
			w.WriteHeader(http.StatusForbidden)
		}))
		defer ts.Close()

		_, err := d.DownloadArchive(context.Background(), ts.URL, filepath.Join(tmpDir, "fail403.zip"), true)
		if err == nil {
			t.Error("Expected error for 403, got nil")
		}
		if atomic.LoadInt32(&calls) != 1 {
			t.Errorf("Expected 1 call, got %d", calls)
		}
	})

	t.Run("context cancel stops retry sleep", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer ts.Close()

		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()

		start := time.Now()
		_, err := d.DownloadArchive(ctx, ts.URL, filepath.Join(tmpDir, "cancel.zip"), true)
		duration := time.Since(start)

		if err == nil || (err != context.Canceled && err.Error() != "context canceled") {
			t.Errorf("Expected context canceled error, got %v", err)
		}
		// Base delay is 1s, so if we cancelled in 100ms, it should be much less than 1s
		if duration > 500*time.Millisecond {
			t.Errorf("Retry took too long (%v), context cancellation might not be respected in sleep", duration)
		}
	})
}
