package zip

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/GopeedLab/gopeed/internal/controller"
	"github.com/GopeedLab/gopeed/pkg/base"
)

func TestZipFetcher(t *testing.T) {
	// 1. Setup mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			switch r.URL.Path {
			case "/file1.txt":
				w.Header().Set("Content-Length", "8") // "content1" length
				w.WriteHeader(http.StatusOK)
				return
			case "/file2.txt":
				w.Header().Set("Content-Length", "8") // "content2" length
				w.WriteHeader(http.StatusOK)
				return
			default:
				http.NotFound(w, r)
				return
			}
		}

		switch r.URL.Path {
		case "/file1.txt":
			fmt.Fprint(w, "content1")
		case "/file2.txt":
			fmt.Fprint(w, "content2")
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	// 2. Prepare request
	files := []FileItem{
		{Url: ts.URL + "/file1.txt", Filename: "file1.txt"},
		{Url: ts.URL + "/file2.txt", Filename: "file2.txt"},
	}
	filesJson, _ := json.Marshal(files)
	reqUrl := fmt.Sprintf("zip://example.com?files=%s", url.QueryEscape(string(filesJson)))

	req := &base.Request{
		URL: reqUrl,
	}

	// 3. Create Fetcher
	fm := &FetcherManager{}
	fetcher := fm.Build()
	ctl := controller.NewController()
	fetcher.Setup(ctl)

	// 4. Resolve
	err := fetcher.Resolve(req)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if fetcher.Meta().Res.Size != 16 {
		t.Errorf("Expected total size 16, got %d", fetcher.Meta().Res.Size)
	}

	// 5. Setup temporary download directory
	tmpDir, err := os.MkdirTemp("", "gopeed-zip-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	fetcher.Meta().Opts = &base.Options{
		Path: tmpDir,
	}

	// 6. Start
	err = fetcher.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// 7. Wait
	err = fetcher.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	// 8. Verify
	zipPath := filepath.Join(tmpDir, "example.com.zip")
	if _, err := os.Stat(zipPath); os.IsNotExist(err) {
		t.Fatalf("Zip file not found at %s", zipPath)
	}

	// Check zip content
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("Failed to open zip: %v", err)
	}
	defer r.Close()

	if len(r.File) != 2 {
		t.Errorf("Expected 2 files in zip, got %d", len(r.File))
	}

	verifyFile := func(name, content string) {
		found := false
		for _, f := range r.File {
			if f.Name == name {
				found = true
				rc, err := f.Open()
				if err != nil {
					t.Fatalf("Failed to open %s in zip: %v", name, err)
				}
				b, _ := io.ReadAll(rc)
				rc.Close()
				if string(b) != content {
					t.Errorf("File %s content mismatch. Want %s, got %s", name, content, string(b))
				}
				break
			}
		}
		if !found {
			t.Errorf("File %s not found in zip", name)
		}
	}

	verifyFile("file1.txt", "content1")
	verifyFile("file2.txt", "content2")
}

func TestZipFetcher_FallbackParsing(t *testing.T) {
	// Test URL with unencoded files parameter (simulating what might happen with raw concatenation)
	filesJson := `[{"url":"http://example.com/1","filename":"1"}]`
	// Construct URL where & might be present in JSON if we are not careful, but here just test simple case
	// first without query encoding
	reqUrl := "zip://example.com?files=" + filesJson

	req := &base.Request{URL: reqUrl}

	fm := &FetcherManager{}
	fetcher := fm.Build()
	fetcher.Setup(controller.NewController())

	// Simulate pre-existing Opts
	originalMeta := fetcher.Meta() // Should be non-nil after Setup
	if originalMeta == nil {
		t.Fatal("Meta should be initialized after Setup")
	}

	// Set a dummy field in Meta
	originalMeta.Opts = &base.Options{Path: "/tmp"}

	err := fetcher.Resolve(req)
	if err != nil {
		t.Fatalf("Resolve failed with unencoded JSON: %v", err)
	}

	if fetcher.Meta() != originalMeta {
		t.Error("Meta pointer changed after Resolve")
	}

	if fetcher.Meta().Opts == nil || fetcher.Meta().Opts.Path != "/tmp" {
		t.Error("Meta.Opts was lost after Resolve")
	}

	// Check internal files slice (need to cast interface)
	f := fetcher.(*Fetcher)
	if len(f.files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(f.files))
	}
}
