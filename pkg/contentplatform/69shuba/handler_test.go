package shuba69

import (
	"context"
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
	if resolved.Download.Protocol != "inline_html" || resolved.Pipeline == nil {
		t.Fatalf("resolved = %#v", resolved)
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
