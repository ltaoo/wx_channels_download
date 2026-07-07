package ttk

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
)

const sampleNovelURL = "https://ttks.tw/novel/chapters/congzhanyaochumokaishichangshengbusi/index.html"

type fakeFetcher struct{}

func (fakeFetcher) FetchNovelChapters(url string) (*Novel, error) {
	return fakeNovel(url), nil
}

func (fakeFetcher) FetchNovel(url string) (*NovelFetchResult, error) {
	novel := fakeNovel(url)
	return &NovelFetchResult{
		Novel:                 novel,
		SourceURL:             url,
		SourceHTML:            "<html>book source</html>",
		SourceNovel:           cloneNovel(novel),
		SourceParsedHTML:      BuildNovelHTML(novel),
		FullCatalogURL:        url,
		FullCatalogHTML:       "<html>full catalog</html>",
		FullCatalogNovel:      cloneNovel(novel),
		FullCatalogParsedHTML: BuildNovelHTML(novel),
	}, nil
}

func (fakeFetcher) FetchChapterContent(url string) (*ChapterContent, error) {
	return &ChapterContent{Title: "chapter 1", Content: "body line"}, nil
}

func fakeNovel(url string) *Novel {
	return &Novel{
		Title:          "book",
		URL:            url,
		Author:         "author",
		BookID:         "congzhanyaochumokaishichangshengbusi",
		FullCatalogURL: url,
		Chapters: []Chapter{
			{Index: 1, Title: "chapter 1", URL: "https://ttks.tw/novel/chapters/congzhanyaochumokaishichangshengbusi/1.html"},
			{Index: 2, Title: "chapter 2", URL: "https://ttks.tw/novel/chapters/congzhanyaochumokaishichangshengbusi/2.html"},
		},
	}
}

func TestParseURL(t *testing.T) {
	novel, ok := ParseURL(sampleNovelURL)
	if !ok || novel.Kind != ContentTypeNovel || novel.BookID != "congzhanyaochumokaishichangshengbusi" {
		t.Fatalf("novel parse = %#v, %v", novel, ok)
	}
	chapter, ok := ParseURL("https://ttks.tw/novel/chapters/congzhanyaochumokaishichangshengbusi/1.html")
	if !ok || chapter.Kind != ContentTypeChapter || chapter.ChapterID != "1" || chapter.BookID != novel.BookID {
		t.Fatalf("chapter parse = %#v, %v", chapter, ok)
	}
}

func TestParseNovelFixtureChapters(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "scraper_examples", "ttk", "260618", "chapters.html"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	novel, err := ParseNovelHTML(sampleNovelURL, string(data))
	if err != nil {
		t.Fatalf("ParseNovelHTML: %v", err)
	}
	if novel.Title != "從斬妖除魔開始長生不死" || novel.Author != "佚名" || novel.Category != "玄幻奇幻" || novel.Status != "全本" {
		t.Fatalf("novel metadata = %#v", novel)
	}
	if novel.BookID != "59608" {
		t.Fatalf("book id = %q", novel.BookID)
	}
	if len(novel.Chapters) != 852 {
		t.Fatalf("chapter count = %d", len(novel.Chapters))
	}
	if got := novel.Chapters[0]; got.Index != 1 || got.Title != "第1章 妖魔亂世" || !strings.HasSuffix(got.URL, "/1.html") {
		t.Fatalf("first chapter = %#v", got)
	}
	last := novel.Chapters[len(novel.Chapters)-1]
	if last.Title != "新書《從效法萬妖開始成就真仙》已發布！" || !strings.HasSuffix(last.URL, "/885.html") {
		t.Fatalf("last chapter = %#v", last)
	}
}

func TestMatchProbeResolveNovel(t *testing.T) {
	h := New(fakeFetcher{})
	if !h.Match(sampleNovelURL) {
		t.Fatal("expected ttk novel match")
	}
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: sampleNovelURL})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if contentdownload.ContentType(probe.Content) != ContentTypeNovel || probe.ContentID != "congzhanyaochumokaishichangshengbusi" {
		t.Fatalf("probe = %#v", probe)
	}
	body, _ := contentdownload.ContentOutputOf(probe.Content)["body_html"].(string)
	if !strings.Contains(body, "chapter 1") {
		t.Fatalf("body_html = %q", body)
	}
	resolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{URL: probe.SourceURL, Probe: probe})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.Download.Protocol != ArchiveProtocol || resolved.Download.Connections != archiveConcurrency || resolved.Pipeline == nil {
		t.Fatalf("resolved = %#v", resolved)
	}
	if contentdownload.FileNodesCount(resolved.Files) != 4 {
		t.Fatalf("resolved files = %#v", resolved.Files)
	}
	if got := platformNodeType(resolved.Pipeline, "download"); got != "download_ttk_archive" {
		t.Fatalf("download node type = %q", got)
	}
}

func TestProbeChapter(t *testing.T) {
	h := New(fakeFetcher{})
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://ttks.tw/novel/chapters/congzhanyaochumokaishichangshengbusi/1.html"})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if contentdownload.ContentType(probe.Content) != ContentTypeChapter || probe.ContentID != "congzhanyaochumokaishichangshengbusi_1" {
		t.Fatalf("probe = %#v", probe)
	}
}

func TestArchiveExecutorWritesDirectory(t *testing.T) {
	seed := &NovelFetchResult{
		Novel: &Novel{
			Title:  "book",
			URL:    sampleNovelURL,
			BookID: "congzhanyaochumokaishichangshengbusi",
			Chapters: []Chapter{
				{Index: 1, Title: "chapter 1", URL: "https://ttks.tw/novel/chapters/congzhanyaochumokaishichangshengbusi/1.html"},
				{Index: 2, Title: "chapter 2", URL: "https://ttks.tw/novel/chapters/congzhanyaochumokaishichangshengbusi/2.html"},
			},
		},
		SourceHTML:      "<html>book source</html>",
		FullCatalogHTML: "<html>full catalog</html>",
	}
	fetcher := &fakeArchiveFetcher{}
	dest := filepath.Join(t.TempDir(), "book")
	var files []contentdownload.FileNode
	err := NewExecutor(fetcher).Execute(context.Background(), contentdownload.ExecuteRequest{
		Resolved: &contentdownload.ResolvedRequest{
			Platform:     PlatformID,
			CanonicalURL: sampleNovelURL,
			Internal: map[string]any{
				metadataNovelFetchResult: seed,
			},
			Files: ttkArchiveFilesFromSeed(seed, contentdownload.FileNodeStatusPending),
		},
		Source:   contentdownload.DownloadSpec{URL: "ttk-archive://congzhanyaochumokaishichangshengbusi", Protocol: ArchiveProtocol, Connections: 5},
		DestPath: dest,
		OnFiles: func(next []contentdownload.FileNode) {
			files = contentdownload.CloneFileNodes(next)
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
			Chapter:    seed.Novel.Chapters[0],
			URL:        seed.Novel.Chapters[0].URL,
			ParsedHTML: "<html>chapter one</html>",
		},
		{
			Chapter:    seed.Novel.Chapters[1],
			URL:        seed.Novel.Chapters[1].URL,
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
