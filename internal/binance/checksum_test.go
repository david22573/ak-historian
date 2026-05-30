package binance

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadChecksum(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		want    string
		wantErr error
	}{
		{
			name:   "success",
			status: http.StatusOK,
			body:   "abc123sha256  BTCUSDT-1m-2024-01-01.zip",
			want:   "abc123sha256",
		},
		{
			name:    "not found",
			status:  http.StatusNotFound,
			wantErr: ErrChecksumNotFound,
		},
		{
			name:    "server error",
			status:  http.StatusInternalServerError,
			wantErr: errors.New("unexpected status code for checksum: 500"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			got, err := DownloadChecksum(context.Background(), http.DefaultClient, server.URL)
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("DownloadChecksum() expected error = %v, got nil", tt.wantErr)
					return
				}
				if err.Error() != tt.wantErr.Error() && !errors.Is(err, tt.wantErr) {
					t.Errorf("DownloadChecksum() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("DownloadChecksum() unexpected error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("DownloadChecksum() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVerifySHA256(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "checksum-test")
	defer os.RemoveAll(tmpDir)

	path := filepath.Join(tmpDir, "test.zip")
	content := "hello world"
	_ = os.WriteFile(path, []byte(content), 0644)

	// sha256 of "hello world"
	expected := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"

	err := VerifySHA256(path, expected)
	if err != nil {
		t.Errorf("VerifySHA256() unexpected error = %v", err)
	}

	err = VerifySHA256(path, "wrong")
	if err == nil {
		t.Error("VerifySHA256() expected error for mismatch, got nil")
	}
}
