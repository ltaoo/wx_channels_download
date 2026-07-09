package shuba69

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
)

type fakeFetcher struct{}

func (fakeFetcher) FetchNovelChapters(url string) (*Novel, error) {
	return &Novel{
		Title:  "book",
		URL:    url,
		Author: "author",
		BookID: "34567",
		Chapters: []Chapter{
			{Index: 1, Title: "chapter 1", URL: "https://www.69shuba.com/txt/34567/1001"},
		},
	}, nil
}

func (fakeFetcher) FetchChapterContent(url string) (*ChapterContent, error) {
	return &ChapterContent{Title: "chapter 1", Content: "body line"}, nil
}

type fakeNovelPipelineFetcher struct{}

func (fakeNovelPipelineFetcher) FetchNovelChapters(url string) (*Novel, error) {
	return fakeFetcher{}.FetchNovelChapters(url)
}

func (fakeNovelPipelineFetcher) FetchNovel(url string) (*NovelFetchResult, error) {
	return &NovelFetchResult{
		Novel: &Novel{
			Title:  "book",
			URL:    url,
			Author: "author",
			BookID: "34567",
			Chapters: []Chapter{
				{Index: 1, Title: "chapter 1", URL: "https://www.69shuba.com/txt/34567/1001"},
				{Index: 2, Title: "chapter 2", URL: "https://www.69shuba.com/txt/34567/1002"},
			},
		},
		SourceURL:        "https://www.69shuba.com/book/34567.htm",
		SourceHTML:       "<html>source</html>",
		SourceNovel:      &Novel{Title: "book", URL: url, BookID: "34567", Chapters: []Chapter{{Index: 1, Title: "chapter 1", URL: "https://www.69shuba.com/txt/34567/1001"}}},
		SourceParsedHTML: "<html>source parsed</html>",
		FullCatalogURL:   "https://www.69shuba.com/book/34567/",
		FullCatalogHTML:  "<html>full</html>",
		FullCatalogNovel: &Novel{
			Title:  "book",
			URL:    "https://www.69shuba.com/book/34567/",
			BookID: "34567",
			Chapters: []Chapter{
				{Index: 1, Title: "chapter 1", URL: "https://www.69shuba.com/txt/34567/1001"},
				{Index: 2, Title: "chapter 2", URL: "https://www.69shuba.com/txt/34567/1002"},
			},
		},
		FullCatalogParsedHTML: "<html>full catalog parsed chapter 2</html>",
	}, nil
}

func (fakeNovelPipelineFetcher) FetchChapterContent(url string) (*ChapterContent, error) {
	return fakeFetcher{}.FetchChapterContent(url)
}

func TestMatchProbeResolveNovel(t *testing.T) {
	h := New(fakeFetcher{})
	if !h.Match("https://www.69shuba.com/book/34567/") {
		t.Fatal("expected book url match")
	}
	if !h.Match("https://www.69shuba.com/book/34567.htm") {
		t.Fatal("expected book html url match")
	}
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://www.69shuba.com/book/34567/"})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if contentdownload.ContentType(probe.Content) != "novel" || probe.ContentID != "34567" {
		t.Fatalf("probe content = %#v", probe)
	}
	body, _ := contentdownload.ContentOutputOf(probe.Content)["body_html"].(string)
	if !strings.Contains(body, "chapter 1") {
		t.Fatalf("body_html = %q", body)
	}
	resolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{URL: probe.SourceURL, Probe: probe})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.Download.Protocol != ArchiveProtocol || resolved.Suffix != "" || resolved.Pipeline == nil {
		t.Fatalf("resolved = %#v", resolved)
	}
	if contentdownload.FileNodesCount(resolved.Files) != 3 {
		t.Fatalf("resolved files = %#v", resolved.Files)
	}
	if got := platformNodeType(resolved.Pipeline, "download"); got != "download_69shuba_archive" {
		t.Fatalf("download node type = %q", got)
	}
}

func TestProbeNovelIncludesFullCatalogPipeline(t *testing.T) {
	h := New(fakeNovelPipelineFetcher{})
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://www.69shuba.com/book/34567.htm"})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	nodes, ok := probe.Internal["probe_pipeline"].([]map[string]any)
	if !ok {
		t.Fatalf("probe_pipeline = %#v", probe.Internal["probe_pipeline"])
	}
	if len(nodes) != 2 {
		t.Fatalf("nodes len = %d", len(nodes))
	}
	output, ok := nodes[1]["output"].(map[string]any)
	if !ok {
		t.Fatalf("second output = %#v", nodes[1]["output"])
	}
	if output["url"] != "https://www.69shuba.com/book/34567/" {
		t.Fatalf("full catalog url = %#v", output["url"])
	}
	if body, _ := output["body_html"].(string); !strings.Contains(body, "chapter 2") {
		t.Fatalf("full catalog body_html = %q", body)
	}
}

func TestProbeChapter(t *testing.T) {
	h := New(fakeFetcher{})
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://www.69shuba.com/txt/34567/1001"})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if contentdownload.ContentType(probe.Content) != "chapter" || probe.ContentID != "34567_1001" {
		t.Fatalf("probe = %#v", probe)
	}
}

func TestArchiveExecutorWritesDirectory(t *testing.T) {
	seed := &NovelFetchResult{
		Novel: &Novel{
			Title:  "book",
			URL:    "https://www.69shuba.com/book/34567.htm",
			BookID: "34567",
			Chapters: []Chapter{
				{Index: 1, Title: "chapter 1", URL: "https://www.69shuba.com/txt/34567/1001"},
				{Index: 2, Title: "chapter 2", URL: "https://www.69shuba.com/txt/34567/1002"},
			},
		},
		SourceHTML:      "<html>book source</html>",
		FullCatalogHTML: "<html>full catalog</html>",
	}
	fetcher := &fakeArchiveFetcher{}
	dest := filepath.Join(t.TempDir(), "book")
	var files []contentdownload.FileNode
	sawChapterFileAfterDone := false
	err := NewExecutor(fetcher).Execute(context.Background(), contentdownload.ExecuteRequest{
		Resolved: &contentdownload.ResolvedRequest{
			Platform:     PlatformID,
			CanonicalURL: "https://www.69shuba.com/book/34567.htm",
			Internal: map[string]any{
				metadataNovelFetchResult: seed,
			},
		},
		Source:   contentdownload.DownloadSpec{URL: "69shuba-archive://34567", Protocol: ArchiveProtocol, Connections: 5},
		DestPath: dest,
		OnFiles: func(next []contentdownload.FileNode) {
			files = contentdownload.CloneFileNodes(next)
			node := findFileNode(files, "chapters/chapter 1.html")
			if node != nil && node.Status == contentdownload.FileNodeStatusDone {
				if _, err := os.Stat(filepath.Join(dest, "chapters", "chapter 1.html")); err == nil {
					sawChapterFileAfterDone = true
				}
			}
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if fetcher.seed != seed {
		t.Fatal("expected executor to pass resolved novel fetch seed")
	}
	if fetcher.options.Concurrency != 5 {
		t.Fatalf("concurrency = %d", fetcher.options.Concurrency)
	}
	for name, want := range map[string]string{
		"source/book.html":         "book source",
		"source/full_catalog.html": "full catalog",
		"chapters/chapter 1.html":  "chapter one",
		"chapters/chapter 2.html":  "chapter two",
	} {
		data, err := os.ReadFile(filepath.Join(dest, filepath.FromSlash(name)))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if !strings.Contains(string(data), want) {
			t.Fatalf("%s = %q, want contains %q", name, string(data), want)
		}
	}
	if contentdownload.FileNodesCount(files) != 4 {
		t.Fatalf("reported files = %#v", files)
	}
	if !sawChapterFileAfterDone {
		t.Fatal("chapter file was not present when file node became done")
	}
}

func TestArchiveExecutorWritesFullCatalogWhenSourceIsCatalogPage(t *testing.T) {
	seed := &NovelFetchResult{
		Novel: &Novel{
			Title:          "book",
			URL:            "https://www.69shuba.com/book/34567/",
			BookID:         "34567",
			FullCatalogURL: "https://www.69shuba.com/book/34567/",
			Chapters: []Chapter{
				{Index: 1, Title: "chapter 1", URL: "https://www.69shuba.com/txt/34567/1001"},
			},
		},
		SourceURL:  "https://www.69shuba.com/book/34567/",
		SourceHTML: "<html>full catalog source</html>",
	}
	fetcher := &fakeArchiveFetcher{}
	dest := filepath.Join(t.TempDir(), "book")
	err := NewExecutor(fetcher).Execute(context.Background(), contentdownload.ExecuteRequest{
		Resolved: &contentdownload.ResolvedRequest{
			Platform:     PlatformID,
			CanonicalURL: "https://www.69shuba.com/book/34567/",
			Internal: map[string]any{
				metadataNovelFetchResult: seed,
			},
		},
		Source:   contentdownload.DownloadSpec{URL: "69shuba-archive://34567", Protocol: ArchiveProtocol, Connections: 1},
		DestPath: dest,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dest, "source", "full_catalog.html"))
	if err != nil {
		t.Fatalf("read full catalog: %v", err)
	}
	if !strings.Contains(string(data), "full catalog source") {
		t.Fatalf("full catalog html = %q", string(data))
	}
}

func TestArchiveExecutorTargetedRetrySkipsCompletedChapters(t *testing.T) {
	seed := &NovelFetchResult{
		Novel: &Novel{
			Title:  "book",
			URL:    "https://www.69shuba.com/book/34567.htm",
			BookID: "34567",
			Chapters: []Chapter{
				{Index: 1, Title: "chapter 1", URL: "https://www.69shuba.com/txt/34567/1001"},
				{Index: 2, Title: "chapter 2", URL: "https://www.69shuba.com/txt/34567/1002"},
			},
		},
		SourceHTML:      "<html>book source</html>",
		FullCatalogHTML: "<html>full catalog</html>",
	}
	files := shubaArchiveFilesFromSeed(seed, contentdownload.FileNodeStatusDone)
	failed := findFileNode(files, "chapters/chapter 2.html")
	if failed == nil {
		t.Fatal("missing chapter 2 node")
	}
	failed.Status = contentdownload.FileNodeStatusError
	failed.Error = "blocked"

	dest := filepath.Join(t.TempDir(), "book")
	if err := os.MkdirAll(filepath.Join(dest, "chapters"), 0o755); err != nil {
		t.Fatal(err)
	}
	chapter1Path := filepath.Join(dest, "chapters", "chapter 1.html")
	if err := os.WriteFile(chapter1Path, []byte("existing chapter one"), 0o644); err != nil {
		t.Fatal(err)
	}

	fetcher := &fakeArchiveFetcher{}
	var reported []contentdownload.FileNode
	err := NewExecutor(fetcher).Execute(context.Background(), contentdownload.ExecuteRequest{
		Resolved: &contentdownload.ResolvedRequest{
			Platform:     PlatformID,
			CanonicalURL: "https://www.69shuba.com/book/34567.htm",
			Internal: map[string]any{
				metadataNovelFetchResult: seed,
			},
			Metadata: map[string]any{
				retryFilePathsMetadataKey: []string{"chapters/chapter 2.html"},
			},
			Files: files,
		},
		Source:   contentdownload.DownloadSpec{URL: "69shuba-archive://34567", Protocol: ArchiveProtocol, Connections: 5},
		DestPath: dest,
		OnFiles: func(next []contentdownload.FileNode) {
			reported = contentdownload.CloneFileNodes(next)
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if fetcher.seed == nil || fetcher.seed.Novel == nil || len(fetcher.seed.Novel.Chapters) != 1 || fetcher.seed.Novel.Chapters[0].Index != 2 {
		t.Fatalf("targeted seed chapters = %#v", fetcher.seed)
	}
	data, err := os.ReadFile(chapter1Path)
	if err != nil {
		t.Fatalf("read chapter 1: %v", err)
	}
	if string(data) != "existing chapter one" {
		t.Fatalf("chapter 1 was overwritten: %q", string(data))
	}
	if _, err := os.Stat(filepath.Join(dest, "chapters", "chapter 2.html")); err != nil {
		t.Fatalf("expected retried chapter 2 file: %v", err)
	}
	chapter1 := findFileNode(reported, "chapters/chapter 1.html")
	if chapter1 == nil || chapter1.Status != contentdownload.FileNodeStatusDone {
		t.Fatalf("chapter 1 node = %#v", chapter1)
	}
	chapter2 := findFileNode(reported, "chapters/chapter 2.html")
	if chapter2 == nil || chapter2.Status != contentdownload.FileNodeStatusDone {
		t.Fatalf("chapter 2 node = %#v", chapter2)
	}
}

func TestArchiveChapterFilePathsUseNativeDuplicateSuffix(t *testing.T) {
	paths := archiveChapterFilePaths([]Chapter{
		{Index: 1, Title: "same", URL: "https://www.69shuba.com/txt/1/1"},
		{Index: 2, Title: "same", URL: "https://www.69shuba.com/txt/1/2"},
		{Index: 3, Title: "same (1)", URL: "https://www.69shuba.com/txt/1/3"},
		{Index: 4, Title: "same", URL: "https://www.69shuba.com/txt/1/4"},
	})
	want := []string{
		"chapters/same.html",
		"chapters/same (1).html",
		"chapters/same (1) (1).html",
		"chapters/same (2).html",
	}
	if len(paths) != len(want) {
		t.Fatalf("paths = %#v, want %#v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("paths = %#v, want %#v", paths, want)
		}
	}
}

func TestArchiveExecutorKeepsTaskSuccessfulWhenChapterFails(t *testing.T) {
	seed := &NovelFetchResult{
		Novel: &Novel{
			Title:  "book",
			URL:    "https://www.69shuba.com/book/34567.htm",
			BookID: "34567",
			Chapters: []Chapter{
				{Index: 1, Title: "chapter 1", URL: "https://www.69shuba.com/txt/34567/1001"},
				{Index: 2, Title: "chapter 2", URL: "https://www.69shuba.com/txt/34567/1002"},
			},
		},
		SourceHTML:      "<html>book source</html>",
		FullCatalogHTML: "<html>full catalog</html>",
	}
	fetcher := &fakePartialArchiveFetcher{}
	dest := filepath.Join(t.TempDir(), "book")
	var files []contentdownload.FileNode
	err := NewExecutor(fetcher).Execute(context.Background(), contentdownload.ExecuteRequest{
		Resolved: &contentdownload.ResolvedRequest{
			Platform:     PlatformID,
			CanonicalURL: "https://www.69shuba.com/book/34567.htm",
			Internal: map[string]any{
				metadataNovelFetchResult: seed,
			},
			Files: shubaArchiveFilesFromSeed(seed, contentdownload.FileNodeStatusPending),
		},
		Source:   contentdownload.DownloadSpec{URL: "69shuba-archive://34567", Protocol: ArchiveProtocol, Connections: 5},
		DestPath: dest,
		OnFiles: func(next []contentdownload.FileNode) {
			files = contentdownload.CloneFileNodes(next)
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "chapters", "chapter 1.html")); err != nil {
		t.Fatalf("expected successful chapter file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "chapters", "chapter 2.html")); !os.IsNotExist(err) {
		t.Fatalf("failed chapter file should not exist, stat err = %v", err)
	}
	failed := findFileNode(files, "chapters/chapter 2.html")
	if failed == nil || failed.Status != contentdownload.FileNodeStatusError || !strings.Contains(failed.Error, "blocked") {
		t.Fatalf("failed file node = %#v in %#v", failed, files)
	}
}

type fakeArchiveFetcher struct {
	seed    *NovelFetchResult
	options NovelArchiveOptions
}

func (f *fakeArchiveFetcher) FetchNovelArchive(rawURL string, seed *NovelFetchResult, options NovelArchiveOptions) (*NovelArchiveResult, error) {
	f.seed = seed
	f.options = options
	chapters := []ChapterFetchResult{
		{
			Chapter:    Chapter{Index: 1, Title: "chapter 1", URL: "https://www.69shuba.com/txt/34567/1001"},
			URL:        "https://www.69shuba.com/txt/34567/1001",
			ParsedHTML: "<html>chapter one</html>",
		},
		{
			Chapter:    Chapter{Index: 2, Title: "chapter 2", URL: "https://www.69shuba.com/txt/34567/1002"},
			URL:        "https://www.69shuba.com/txt/34567/1002",
			ParsedHTML: "<html>chapter two</html>",
		},
	}
	for i, chapter := range chapters {
		if options.OnChapter != nil {
			options.OnChapter(i+1, len(chapters), chapter)
		}
	}
	return &NovelArchiveResult{Novel: seed.Novel, Fetch: seed, Chapters: chapters}, nil
}

type fakePartialArchiveFetcher struct{}

func (f *fakePartialArchiveFetcher) FetchNovelArchive(rawURL string, seed *NovelFetchResult, options NovelArchiveOptions) (*NovelArchiveResult, error) {
	if !options.AllowPartial {
		return nil, errors.New("expected partial mode")
	}
	chapters := []ChapterFetchResult{
		{
			Chapter:    seed.Novel.Chapters[0],
			URL:        seed.Novel.Chapters[0].URL,
			ParsedHTML: "<html>chapter one</html>",
		},
		{
			Chapter: seed.Novel.Chapters[1],
			URL:     seed.Novel.Chapters[1].URL,
			Error:   "blocked by remote",
		},
	}
	for i, chapter := range chapters {
		if options.OnChapter != nil {
			options.OnChapter(i+1, len(chapters), chapter)
		}
	}
	return &NovelArchiveResult{Novel: seed.Novel, Fetch: seed, Chapters: chapters}, nil
}

func findFileNode(files []contentdownload.FileNode, path string) *contentdownload.FileNode {
	for i := range files {
		if files[i].Path == path {
			return &files[i]
		}
		if found := findFileNode(files[i].Children, path); found != nil {
			return found
		}
	}
	return nil
}

func platformNodeType(plan *contentdownload.PipelinePlan, id string) string {
	if plan == nil {
		return ""
	}
	for _, node := range plan.Nodes {
		if node.ID == id {
			return node.Type
		}
	}
	return ""
}
