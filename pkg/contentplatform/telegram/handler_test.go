package telegram

import (
	"context"
	"strings"
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
	telegrampkg "wx_channel/pkg/scraper/telegram"
)

type fakePageFetcher struct{}

func (fakePageFetcher) FetchPage(ctx context.Context, rawURL string) (*telegrampkg.Page, error) {
	return &telegrampkg.Page{
		URL:          telegrampkg.TelegramURL{Username: "testchannel", MessageID: 12, Canonical: "https://t.me/testchannel/12", WebURL: "https://t.me/s/testchannel/12"},
		SourceURL:    rawURL,
		CanonicalURL: "https://t.me/testchannel/12",
		Channel: telegrampkg.Channel{
			Username:  "testchannel",
			Title:     "Test Channel",
			URL:       "https://t.me/testchannel",
			WebURL:    "https://t.me/s/testchannel",
			AvatarURL: "https://cdn.example.com/avatar.jpg",
		},
		Messages: []telegrampkg.Message{
			{
				ChannelUsername: "testchannel",
				ID:              12,
				URL:             "https://t.me/testchannel/12",
				WebURL:          "https://t.me/s/testchannel/12",
				ContentHTML:     "<p>Telegram body</p>",
				ContentText:     "Telegram body",
				MediaType:       "Text",
				ViewCount:       1200,
				PublishedAt:     "2026-06-02T10:00:00+00:00",
			},
		},
	}, nil
}

func TestMatchProbeResolve(t *testing.T) {
	h := New(fakePageFetcher{})
	if !h.Match("https://t.me/testchannel/12") {
		t.Fatal("expected telegram match")
	}
	if h.Match("https://t.me/c/12345/12") {
		t.Fatal("private t.me/c links should not match public website fetcher")
	}

	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://t.me/testchannel/12"})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if probe.Platform != PlatformID || probe.ContentID != "testchannel_12" {
		t.Fatalf("probe = %#v", probe)
	}
	if contentdownload.ContentType(probe.Content) != telegrampkg.ContentTypeMessage {
		t.Fatalf("content type = %q", contentdownload.ContentType(probe.Content))
	}
	body, _ := contentdownload.ContentOutputOf(probe.Content)["body_html"].(string)
	if !strings.Contains(body, "Telegram body") {
		t.Fatalf("body_html = %q", body)
	}
	foundJSON := false
	for _, variant := range probe.Variants {
		if variant.ID == "json" && variant.Suffix == ".json" {
			foundJSON = true
		}
	}
	if !foundJSON {
		t.Fatalf("expected json variant: %#v", probe.Variants)
	}

	resolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{URL: probe.SourceURL, Probe: probe})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.Download.Protocol != "inline_html" || resolved.Labels["content_type"] != telegrampkg.ContentTypeMessage {
		t.Fatalf("resolved = %#v", resolved)
	}
	if resolved.Suffix != ".html" || resolved.Filename == "" {
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
