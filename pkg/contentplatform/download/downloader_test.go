package download

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeHandler struct {
	rawURL string
	source DownloadSpec
}

func (h fakeHandler) Platform() string { return "fake" }
func (h fakeHandler) Match(rawURL string) bool {
	return rawURL == h.rawURL
}
func (h fakeHandler) Probe(ctx context.Context, input ProbeInput) (*Probe, error) {
	return &Probe{Platform: "fake", SourceURL: input.URL, ContentID: "fake1"}, nil
}
func (h fakeHandler) Resolve(ctx context.Context, input ResolveInput) (*ResolvedRequest, error) {
	return &ResolvedRequest{
		Platform:  "fake",
		SourceURL: input.URL,
		ContentID: "fake1",
		Title:     "fake",
		Filename:  "fake",
		Suffix:    ".txt",
		Download:  h.source,
	}, nil
}
func (h fakeHandler) Plan(ctx context.Context, resolved *ResolvedRequest) (*PipelinePlan, error) {
	return &PipelinePlan{Platform: "fake", Nodes: []PipelineNode{{ID: "download", Type: "download_asset"}}}, nil
}

func TestDownloaderCreateAndStartHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	}))
	defer server.Close()

	router := NewRouter(fakeHandler{
		rawURL: "fake://item",
		source: DownloadSpec{
			URL:      server.URL,
			Protocol: "http",
		},
	})
	dir := t.TempDir()
	d := NewDownloader(router, dir)
	task, err := d.CreateAndStart(context.Background(), ResolveInput{URL: "fake://item"})
	if err != nil {
		t.Fatalf("CreateAndStart: %v", err)
	}
	if task.Status != TaskStatusDone {
		t.Fatalf("status = %s", task.Status)
	}
	data, err := os.ReadFile(filepath.Join(dir, "fake.txt"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("file = %q", string(data))
	}
}

func TestHTTPExecutorResumesPartFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Range"); got != "bytes=6-" {
			t.Fatalf("Range = %q", got)
		}
		w.WriteHeader(http.StatusPartialContent)
		w.Write([]byte("world"))
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "resume.txt")
	if err := os.WriteFile(dest+".part", []byte("hello "), 0o644); err != nil {
		t.Fatal(err)
	}
	err := NewHTTPExecutor(server.Client()).Execute(context.Background(), ExecuteRequest{
		Source:   DownloadSpec{URL: server.URL, Protocol: "http"},
		DestPath: dest,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "hello world" {
		t.Fatalf("file = %q", string(data))
	}
	if _, err := os.Stat(dest + ".part"); !os.IsNotExist(err) {
		t.Fatalf("part file still exists: %v", err)
	}
}

func TestHTTPExecutorDownloadsChunks(t *testing.T) {
	body := "abcdefghijklmnopqrstuvwxyz"
	var ranges []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")
		ranges = append(ranges, rangeHeader)
		var start, end int
		if _, err := fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end); err != nil {
			t.Fatalf("Range = %q", rangeHeader)
		}
		if end >= len(body) {
			end = len(body) - 1
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(body)))
		w.WriteHeader(http.StatusPartialContent)
		w.Write([]byte(body[start : end+1]))
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "chunked.txt")
	err := NewHTTPExecutor(server.Client()).Execute(context.Background(), ExecuteRequest{
		Source:   DownloadSpec{URL: server.URL, Protocol: "http", ChunkSize: 10},
		DestPath: dest,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != body {
		t.Fatalf("file = %q", string(data))
	}
	wantRanges := []string{"bytes=0-9", "bytes=10-19", "bytes=20-29"}
	if strings.Join(ranges, ",") != strings.Join(wantRanges, ",") {
		t.Fatalf("ranges = %#v", ranges)
	}
}

func TestHTTPExecutorRetriesInterruptedChunk(t *testing.T) {
	body := "abcdefghijklmnopqrstuvwxyz"
	var ranges []string
	interrupted := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")
		ranges = append(ranges, rangeHeader)
		var start, end int
		if _, err := fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end); err != nil {
			t.Fatalf("Range = %q", rangeHeader)
		}
		if end >= len(body) {
			end = len(body) - 1
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(body)))
		if !interrupted {
			interrupted = true
			w.Header().Set("Content-Length", "10")
			w.WriteHeader(http.StatusPartialContent)
			w.Write([]byte(body[start : start+5]))
			return
		}
		w.WriteHeader(http.StatusPartialContent)
		w.Write([]byte(body[start : end+1]))
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "retry-chunk.txt")
	err := NewHTTPExecutor(server.Client()).Execute(context.Background(), ExecuteRequest{
		Source:   DownloadSpec{URL: server.URL, Protocol: "http", ChunkSize: 10},
		DestPath: dest,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != body {
		t.Fatalf("file = %q", string(data))
	}
	wantRanges := []string{"bytes=0-9", "bytes=5-14", "bytes=15-24", "bytes=25-34"}
	if strings.Join(ranges, ",") != strings.Join(wantRanges, ",") {
		t.Fatalf("ranges = %#v", ranges)
	}
}

func TestMultiHTTPExecutorDownloadsAndMergesSources(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/video":
			w.Write([]byte("video"))
		case "/audio":
			w.Write([]byte("audio"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "merged.mp4")
	executor := NewMultiHTTPExecutor(server.Client())
	executor.runCommand = func(ctx context.Context, name string, args ...string) error {
		if name != "ffmpeg" {
			t.Fatalf("command = %q", name)
		}
		output := args[len(args)-1]
		return os.WriteFile(output, []byte(strings.Join(args, "\n")), 0o644)
	}
	err := executor.Execute(context.Background(), ExecuteRequest{
		Resolved: &ResolvedRequest{
			Metadata: map[string]any{
				"sources": []MultiSourceSpec{
					{ID: "137", URL: server.URL + "/video", Ext: "mp4", HasVideo: true, Size: 5},
					{ID: "140", URL: server.URL + "/audio", Ext: "m4a", HasAudio: true, Size: 5},
				},
			},
		},
		Source:   DownloadSpec{URL: "multi-http://test", Protocol: ProtocolMultiHTTP},
		DestPath: dest,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	args := string(data)
	for _, want := range []string{"-i", "-map\n0:v:0", "-map\n1:a:0", "-c\ncopy", "merged.mp4"} {
		if !strings.Contains(args, want) {
			t.Fatalf("ffmpeg args missing %q in:\n%s", want, args)
		}
	}
}

func TestZipExecutor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/1.txt":
			w.Write([]byte("one"))
		case "/2.txt":
			w.Write([]byte("two"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	files := `[{"url":"` + server.URL + `/1.txt","filename":"1.txt"},{"url":"` + server.URL + `/2.txt","filename":"2.txt"}]`
	source := DownloadSpec{
		URL:      "zip://example.com?files=" + url.QueryEscape(files),
		Protocol: "zip",
	}
	dest := filepath.Join(t.TempDir(), "archive.zip")
	err := NewZipExecutor(server.Client()).Execute(context.Background(), ExecuteRequest{
		Source:   source,
		DestPath: dest,
	})
	if err != nil {
		t.Fatalf("zip execute: %v", err)
	}
	if info, err := os.Stat(dest); err != nil || info.Size() == 0 {
		t.Fatalf("zip file not created: info=%v err=%v", info, err)
	}
}

func TestInlineHTMLExecutor(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "page.html")
	err := NewInlineHTMLExecutor().Execute(context.Background(), ExecuteRequest{
		Resolved: &ResolvedRequest{
			Metadata: map[string]any{"body_html": "<html>hello</html>"},
		},
		Source:   DownloadSpec{URL: "inline-html://test/1", Protocol: "inline_html"},
		DestPath: dest,
	})
	if err != nil {
		t.Fatalf("inline html execute: %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "<html>hello</html>" {
		t.Fatalf("file = %q", string(data))
	}
}
