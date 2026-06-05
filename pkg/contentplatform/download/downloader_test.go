package download

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
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
