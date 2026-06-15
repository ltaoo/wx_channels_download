package v2ex

import (
	"context"
	"strings"
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
	v2expkg "wx_channel/pkg/v2ex"
)

type fakeTopicFetcher struct{}

func (fakeTopicFetcher) FetchTopicPage(rawURL string) (*v2expkg.TopicPage, error) {
	return &v2expkg.TopicPage{
		ID:           "1219463",
		URL:          rawURL,
		CanonicalURL: "https://www.v2ex.com/t/1219463",
		Title:        "topic title",
		Description:  "topic description",
		NodeName:     "分享发现",
		NodeURL:      "https://www.v2ex.com/go/share",
		Author:       v2expkg.Author{Name: "author", URL: "https://www.v2ex.com/member/author", AvatarURL: "https://example.com/avatar.jpg"},
		ContentHTML:  "<p>topic body</p>",
		ContentText:  "topic body",
		ReplyCount:   1,
		Replies: []v2expkg.Reply{
			{ID: "1", No: 1, Author: v2expkg.Author{Name: "replyer"}, ContentHTML: "<p>reply body</p>"},
		},
	}, nil
}

func TestMatchProbeResolve(t *testing.T) {
	h := New(fakeTopicFetcher{})
	if !h.Match("https://www.v2ex.com/t/1219463") {
		t.Fatal("expected v2ex topic match")
	}
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://www.v2ex.com/t/1219463"})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if probe.Platform != PlatformID || probe.ContentID != "1219463" {
		t.Fatalf("probe = %#v", probe)
	}
	if contentdownload.ContentType(probe.Content) != v2expkg.ContentTypeTopic {
		t.Fatalf("content type = %q", contentdownload.ContentType(probe.Content))
	}
	foundJSON := false
	for _, variant := range probe.Variants {
		if variant.ID == "json" && variant.Type == "json" && variant.Suffix == ".json" {
			foundJSON = true
		}
	}
	if !foundJSON {
		t.Fatalf("expected json variant: %#v", probe.Variants)
	}
	body, _ := contentdownload.ContentOutputOf(probe.Content)["body_html"].(string)
	if !strings.Contains(body, "topic body") || !strings.Contains(body, "reply body") {
		t.Fatalf("body_html = %q", body)
	}
	resolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{URL: probe.SourceURL, Probe: probe})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.Download.Protocol != "inline_html" || resolved.Labels["content_type"] != v2expkg.ContentTypeTopic {
		t.Fatalf("resolved = %#v", resolved)
	}
	if resolved.Suffix != ".html" || resolved.Filename != "topic title" {
		t.Fatalf("resolved name fields = %#v", resolved)
	}

	jsonResolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{
		URL:     probe.SourceURL,
		Probe:   probe,
		Options: contentdownload.Options{VariantID: "json"},
	})
	if err != nil {
		t.Fatalf("Resolve JSON: %v", err)
	}
	if jsonResolved.Download.Protocol != "inline_json" || jsonResolved.Suffix != ".json" {
		t.Fatalf("json resolved = %#v", jsonResolved)
	}
	if jsonResolved.Metadata["json"] == nil {
		t.Fatalf("json metadata missing payload: %#v", jsonResolved.Metadata)
	}
}
