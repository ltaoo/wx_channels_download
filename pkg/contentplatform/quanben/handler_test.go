package quanben

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
		BookID: "42",
		Chapters: []Chapter{
			{Index: 1, Title: "chapter 1", URL: "https://www.quanben.io/n/book/1.html"},
		},
	}, nil
}

func (fakeFetcher) FetchChapterContent(url string) (*ChapterContent, error) {
	return &ChapterContent{Title: "chapter 1", Content: "body line"}, nil
}

func TestMatchProbeResolveNovel(t *testing.T) {
	h := New(fakeFetcher{})
	if !h.Match("https://www.quanben.io/n/book/") {
		t.Fatal("expected quanben novel match")
	}
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://www.quanben.io/n/book/"})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if contentdownload.ContentType(probe.Content) != "novel" || probe.ContentID != "42" {
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
	if resolved.Download.Protocol != "inline_html" {
		t.Fatalf("protocol = %s", resolved.Download.Protocol)
	}
}

func TestProbeChapter(t *testing.T) {
	h := New(fakeFetcher{})
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://www.quanben.io/n/book/1.html"})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if contentdownload.ContentType(probe.Content) != "chapter" || probe.ContentID != "1" {
		t.Fatalf("probe = %#v", probe)
	}
}
