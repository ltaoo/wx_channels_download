package qidian

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
	qidianpkg "wx_channel/pkg/scraper/qidian"
)

type fakeFetcher struct{}

func (fakeFetcher) FetchBookProfile(id string) (*BookProfile, error) {
	return &BookProfile{
		URL:          "https://www.qidian.com/book/" + id + "/",
		Title:        "book",
		Description:  "desc",
		Slogan:       "slogan",
		Author:       Author{Name: "author", Avatar: "https://ccportrait.yuewen.com/apimg/349573/p_16465333704674701/100"},
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
	if description := contentdownload.ContentDescription(probe.Content); description != "desc" {
		t.Fatalf("summary description = %q", description)
	}
	body, _ := contentdownload.ContentOutputOf(probe.Content)["body_html"].(string)
	if !strings.Contains(body, "chapter 1") {
		t.Fatalf("body_html = %q", body)
	}
	output := contentdownload.ContentOutputOf(probe.Content)
	if output["description"] != "desc" {
		t.Fatalf("description output = %#v", output["description"])
	}
	if got := contentdownload.ContentAuthorAvatarURL(probe.Content); got != "https://ccportrait.yuewen.com/apimg/349573/p_16465333704674701/100" {
		t.Fatalf("summary author avatar = %q", got)
	}
	if output["author_avatar_url"] != "https://ccportrait.yuewen.com/apimg/349573/p_16465333704674701/100" {
		t.Fatalf("author avatar output = %#v", output["author_avatar_url"])
	}
	if metadata := contentdownload.ContentMetadataOf(probe.Content); metadata["author_avatar_url"] != "https://ccportrait.yuewen.com/apimg/349573/p_16465333704674701/100" {
		t.Fatalf("author avatar metadata = %#v", metadata["author_avatar_url"])
	}
	volumes, ok := output["volumes"].([]BookVolume)
	if !ok || len(volumes) != 1 || len(volumes[0].Chapters) != 1 {
		t.Fatalf("volumes output = %#v", output["volumes"])
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
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "scraper_examples", "qidian", "260619", "book.html"))
	if err != nil {
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
	if description := contentdownload.ContentDescription(probe.Content); !strings.Contains(description, "陆江仙熬夜猝死") || strings.Contains(description, "修仙觅长生") {
		t.Fatalf("summary description = %q", description)
	}
	output := contentdownload.ContentOutputOf(probe.Content)
	if !strings.Contains(output["description"].(string), "陆江仙熬夜猝死") {
		t.Fatalf("description output = %#v", output["description"])
	}
	if volumes, ok := output["volumes"].([]BookVolume); !ok || len(volumes) < 2 || volumes[1].Chapters[0].Title != "第一章 初入" {
		t.Fatalf("volumes output = %#v", output["volumes"])
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
	if !ok || !json.Valid(raw) || !strings.Contains(string(raw), `"bookId":1035420986`) || !strings.Contains(string(raw), "季越人") {
		t.Fatalf("resolved json = %#v", resolved.Metadata["json"])
	}
}

type staticFetcher struct {
	profile *BookProfile
}

func (f staticFetcher) FetchBookProfile(id string) (*BookProfile, error) {
	return f.profile, nil
}
