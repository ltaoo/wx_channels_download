package cmd

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"wx_channel/pkg/hermes"
)

func TestDownloadUpdateAssetWithHermes(t *testing.T) {
	payload := []byte(strings.Repeat("hermes update payload", 64))
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", "application/octet-stream")
		if r.Header.Get("Range") == "" {
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			_, _ = w.Write(payload)
			return
		}

		var start, end int
		if _, err := fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end); err != nil {
			http.Error(w, "invalid range", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if end >= len(payload) {
			end = len(payload) - 1
		}
		w.Header().Set("Content-Length", strconv.Itoa(end-start+1))
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(payload)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(payload[start : end+1])
	}))
	defer server.Close()

	var progressCount atomic.Int32
	assetPath, cleanup, err := download_update_asset_with_hermes(
		server.URL+"/release.bin",
		"release.bin",
		hermes.ProxyServer{},
		func(*hermes.TaskProgress) { progressCount.Add(1) },
	)
	if err != nil {
		t.Fatalf("download_update_asset_with_hermes() error = %v", err)
	}
	defer cleanup()

	got, err := os.ReadFile(assetPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("downloaded payload length = %d, want %d", len(got), len(payload))
	}
	if requestCount.Load() < 2 {
		t.Fatalf("request count = %d, want at least probe and download requests", requestCount.Load())
	}
	if progressCount.Load() == 0 {
		t.Fatal("Hermes did not emit update download progress")
	}
	tempDir := filepath.Dir(assetPath)
	cleanup()
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Fatalf("temporary update directory still exists after cleanup: %v", err)
	}
}

func TestApplyDownloadedUpdate(t *testing.T) {
	const updatedContents = "updated executable"
	executableName := "wx_channels_download"
	if runtime.GOOS == "windows" {
		executableName += ".exe"
	}

	tests := []struct {
		name     string
		filename string
		archive  func(*testing.T, string, []byte) io.Reader
	}{
		{
			name:     "binary",
			filename: executableName,
			archive: func(_ *testing.T, _ string, contents []byte) io.Reader {
				return bytes.NewReader(contents)
			},
		},
		{
			name:     "zip",
			filename: "release.ZIP",
			archive:  zipUpdate,
		},
		{
			name:     "tar gzip",
			filename: "release.TAR.GZ",
			archive:  tarGzipUpdate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetPath := filepath.Join(t.TempDir(), executableName)
			if err := os.WriteFile(targetPath, []byte("old executable"), 0o755); err != nil {
				t.Fatal(err)
			}

			source := tt.archive(t, executableName, []byte(updatedContents))
			if err := apply_downloaded_update(source, tt.filename, targetPath); err != nil {
				t.Fatalf("apply_downloaded_update() error = %v", err)
			}

			got, err := os.ReadFile(targetPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != updatedContents {
				t.Fatalf("updated contents = %q, want %q", got, updatedContents)
			}
		})
	}
}

func TestApplyDownloadedUpdateRejectsArchiveWithoutExecutable(t *testing.T) {
	targetPath := filepath.Join(t.TempDir(), "wx_channels_download")
	if err := os.WriteFile(targetPath, []byte("old executable"), 0o755); err != nil {
		t.Fatal(err)
	}

	source := zipUpdate(t, "README.md", []byte("not an executable"))
	err := apply_downloaded_update(source, "release.zip", targetPath)
	if err == nil || !strings.Contains(err.Error(), "executable not found") {
		t.Fatalf("apply_downloaded_update() error = %v, want executable-not-found error", err)
	}
}

func zipUpdate(t *testing.T, filename string, contents []byte) io.Reader {
	t.Helper()

	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	file, err := zw.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(contents); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(archive.Bytes())
}

func tarGzipUpdate(t *testing.T, filename string, contents []byte) io.Reader {
	t.Helper()

	var archive bytes.Buffer
	gw := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gw)
	if err := tw.WriteHeader(&tar.Header{Name: filename, Mode: 0o755, Size: int64(len(contents))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(contents); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(archive.Bytes())
}
