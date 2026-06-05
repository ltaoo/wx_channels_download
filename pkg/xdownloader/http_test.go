package xdownloader

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRegistrySelectsHTTPDownloader(t *testing.T) {
	registry := NewDefaultRegistry(nil)
	downloader, err := registry.Select(Request{URL: "https://example.com/file.mp4"})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if downloader.Name() != "http" {
		t.Fatalf("downloader = %q", downloader.Name())
	}
}

func TestHTTPDownloaderDownload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "5")
		_, _ = w.Write([]byte("hello"))
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "hello.txt")
	var finished bool
	result, err := NewHTTPDownloader(server.Client()).Download(context.Background(), Request{
		URL:      server.URL,
		DestPath: dest,
		Progress: func(progress Progress) {
			if progress.Status == StatusFinished {
				finished = true
			}
		},
	})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if result.Path != dest || result.Bytes != 5 {
		t.Fatalf("result = %+v", result)
	}
	if !finished {
		t.Fatal("finished progress was not reported")
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("file = %q", string(data))
	}
}

func TestHTTPDownloaderResume(t *testing.T) {
	const body = "hello world"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "bytes=6-" {
			t.Fatalf("Range = %q", r.Header.Get("Range"))
		}
		w.Header().Set("Content-Range", "bytes 6-10/11")
		w.Header().Set("Content-Length", "5")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte(body[6:]))
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "hello.txt")
	if err := os.WriteFile(dest+".part", []byte(body[:6]), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result, err := NewHTTPDownloader(server.Client()).Download(context.Background(), Request{
		URL:      server.URL,
		DestPath: dest,
		Resume:   true,
	})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if !result.Resumed || result.Bytes != int64(len(body)) {
		t.Fatalf("result = %+v", result)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != body {
		t.Fatalf("file = %q", string(data))
	}
}
