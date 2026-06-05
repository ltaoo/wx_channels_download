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

func TestMatchProbeResolveNovel(t *testing.T) {
	h := New(fakeFetcher{})
	if !h.Match("https://www.69shuba.com/book/34567/") {
		t.Fatal("expected book url match")
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
