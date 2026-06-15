package qidian

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
	qidianpkg "wx_channel/pkg/qidian"
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

func TestProbePageContextJSONVariant(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "qidian_260614.html"))
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("qidian_260614.html fixture not present")
		}
		t.Fatal(err)
	}
	profile, err := qidianpkg.ParseBookProfile("https://www.qidian.com/book/1035420986/", body)
	if err != nil {
		t.Fatalf("ParseBookProfile: %v", err)
	}
	h := New(staticFetcher{profile: profile})
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://www.qidian.com/book/1035420986/"})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if probe.ContentID != "1035420986" || contentdownload.ContentTitle(probe.Content) != "玄鉴仙族" {
		t.Fatalf("probe content = %#v", probe.Content)
	}
	found := false
	for _, variant := range probe.Variants {
		if variant.ID == "page_context_json" && variant.Type == "json" && variant.Suffix == ".json" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing page_context_json variant: %#v", probe.Variants)
	}
	if raw := pageContextJSONFromProbe(probe); len(raw) == 0 || !json.Valid(raw) {
		t.Fatalf("page context json = %q", raw)
	}
	resolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{
		URL:     "https://www.qidian.com/book/1035420986/",
		Probe:   probe,
		Options: contentdownload.Options{VariantID: "page_context_json"},
	})
	if err != nil {
		t.Fatalf("Resolve page context json: %v", err)
	}
	if resolved.Download.Protocol != "inline_json" || resolved.Suffix != ".json" {
		t.Fatalf("resolved = %#v", resolved)
	}
	raw, ok := resolved.Metadata["json"].(json.RawMessage)
	if !ok || !json.Valid(raw) || !strings.Contains(string(raw), "玄鉴仙族") {
		t.Fatalf("resolved json = %#v", resolved.Metadata["json"])
	}
}

type staticFetcher struct {
	profile *BookProfile
}

func (f staticFetcher) FetchBookProfile(id string) (*BookProfile, error) {
	return f.profile, nil
}
