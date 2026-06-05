package qidian

import (
	"context"
	"strings"
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
)

type fakeFetcher struct{}

func (fakeFetcher) FetchBookProfile(id string) (*BookProfile, error) {
	return &BookProfile{
		URL:          "https://www.qidian.com/book/" + id + "/",
		Title:        "book",
		Description:  "desc",
		Author:       Author{Name: "author"},
		ChapterCount: 1,
		Volumes: []BookVolume{
			{Idx: 1, Title: "volume", Chapters: []Chapter{{Idx: 1, Title: "chapter 1", URL: "https://www.qidian.com/chapter/1"}}},
		},
	}, nil
}

func TestMatchProbeResolve(t *testing.T) {
	h := New(fakeFetcher{})
	if !h.Match("https://www.qidian.com/book/12345/") {
		t.Fatal("expected qidian book match")
	}
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://www.qidian.com/book/12345/"})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if contentdownload.ContentType(probe.Content) != "novel" || probe.ContentID != "12345" {
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
	if resolved.Download.Protocol != "inline_html" || resolved.Labels["content_type"] != "novel" {
		t.Fatalf("resolved = %#v", resolved)
	}
}
