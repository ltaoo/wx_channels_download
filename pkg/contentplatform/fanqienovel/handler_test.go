package fanqienovel

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
)

type fakeFetcher struct{}

func (fakeFetcher) FetchBookProfile(id string) (*BookProfile, error) {
	return &BookProfile{
		URL:          "https://fanqienovel.com/page/" + id,
		Title:        "book",
		Description:  "desc",
		Author:       Author{Name: "author"},
		ChapterCount: 1,
		Volumes: []BookVolume{
			{Idx: 1, Title: "volume", Chapters: []Chapter{{Idx: 1, Title: "chapter 1", URL: "https://fanqienovel.com/reader/1"}}},
		},
	}, nil
}

func (fakeFetcher) FetchBookChapterProfile(chapterID string) (*ChapterContent, error) {
	return &ChapterContent{Title: "chapter 1", Content: "body line"}, nil
}

func TestMatchProbeResolveNovel(t *testing.T) {
	h := New(fakeFetcher{})
	if !h.Match("https://fanqienovel.com/page/12345") {
		t.Fatal("expected fanqie novel match")
	}
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://fanqienovel.com/page/12345"})
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
	if resolved.Download.Protocol != "inline_html" {
		t.Fatalf("protocol = %s", resolved.Download.Protocol)
	}
}

func TestProbeChapter(t *testing.T) {
	h := New(fakeFetcher{})
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://fanqienovel.com/reader/67890"})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if contentdownload.ContentType(probe.Content) != "chapter" || probe.ContentID != "67890" {
		t.Fatalf("probe = %#v", probe)
	}
}

func TestClientParseBookProfileInitialStateFixture(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "fanqienovel_260614.html"))
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("fanqienovel_260614.html fixture not present")
		}
		t.Fatal(err)
	}
	profile, err := NewClient(nil).parseBookProfile("https://fanqienovel.com/page/7069948840148732967", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("parseBookProfile: %v", err)
	}
	if profile.Title != "部族荣光" || profile.Author.Name != "丧狐" || profile.ChapterCount != 351 {
		t.Fatalf("profile = %#v", profile)
	}
	if len(profile.InitialStateJSON) == 0 {
		t.Fatal("expected initial state json")
	}
	if len(profile.Volumes) == 0 || len(profile.Volumes[0].Chapters) == 0 {
		t.Fatalf("volumes = %#v", profile.Volumes)
	}
	if !strings.Contains(profile.Volumes[0].Chapters[0].Title, "第351章") {
		t.Fatalf("first chapter = %#v", profile.Volumes[0].Chapters[0])
	}
}
