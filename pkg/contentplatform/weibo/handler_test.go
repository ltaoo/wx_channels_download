package weibo

import (
	"context"
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
	weibopkg "wx_channel/pkg/scraper/weibo"
)

type fakeTimelineFetcher struct {
	gotURL  string
	gotOpts weibopkg.TimelineOptions
	page    *weibopkg.TimelinePage
}

func (f *fakeTimelineFetcher) FetchUserTimeline(ctx context.Context, rawURL string, opts weibopkg.TimelineOptions) (*weibopkg.TimelinePage, error) {
	f.gotURL = rawURL
	f.gotOpts = opts
	return f.page, nil
}

func TestMatch(t *testing.T) {
	h := New(&fakeTimelineFetcher{})
	if !h.Match("https://weibo.com/u/1926245291") {
		t.Fatal("expected user URL to match")
	}
	if h.Match("https://weibo.com/tv/show/123") {
		t.Fatal("unexpected non-user URL match")
	}
}

func TestProbeAndResolve(t *testing.T) {
	fetcher := &fakeTimelineFetcher{page: &weibopkg.TimelinePage{
		URL:       weibopkg.UserURL{UID: "1926245291", Canonical: "https://weibo.com/u/1926245291"},
		SourceURL: "https://weibo.com/u/1926245291",
		Request:   weibopkg.TimelineOptions{Page: 2},
		User: weibopkg.User{
			IDStr:           "1926245291",
			ScreenName:      "Krenz",
			ProfileImageURL: "https://example.com/avatar.jpg",
			StatusesCount:   1365,
		},
		Posts: []weibopkg.PostSummary{
			{
				ID:             "5311280362561538",
				MblogID:        "R4Js8aKn8",
				URL:            "https://weibo.com/1926245291/R4Js8aKn8",
				Text:           "習作",
				CreatedAt:      "Thu Jun 18 22:03:00 +0800 2026",
				PicURLs:        []string{"https://wx2.sinaimg.cn/orj1080/pic1.jpg"},
				CoverURL:       "https://wx2.sinaimg.cn/orj1080/pic1.jpg",
				AttitudesCount: 161,
			},
		},
		Total:   1365,
		SinceID: "next",
	}}
	h := New(fetcher)
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{
		URL:   "https://weibo.com/u/1926245291",
		Extra: map[string]any{"page": float64(2), "cookie": "SUB=test"},
	})
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if fetcher.gotURL != "https://weibo.com/u/1926245291" ||
		fetcher.gotOpts.Page != 2 ||
		fetcher.gotOpts.Cookie != "SUB=test" {
		t.Fatalf("fetch input url=%q opts=%#v", fetcher.gotURL, fetcher.gotOpts)
	}
	if probe.Platform != PlatformID || probe.ContentID != "1926245291" {
		t.Fatalf("probe = %#v", probe)
	}
	if contentdownload.ContentType(probe.Content) != weibopkg.ContentTypeUserTimeline ||
		contentdownload.ContentTitle(probe.Content) != "Krenz 的微博列表" ||
		contentdownload.ContentAuthorNickname(probe.Content) != "Krenz" {
		t.Fatalf("content summary = %#v", contentdownload.ContentSummaryOf(probe.Content))
	}
	output := contentdownload.ContentOutputOf(probe.Content)
	if output["post_count"] != 1 || output["total"] != 1365 || output["since_id"] != "next" {
		t.Fatalf("output = %#v", output)
	}
	posts, ok := output["posts"].([]weibopkg.PostSummary)
	if !ok || len(posts) != 1 || posts[0].CoverURL == "" {
		t.Fatalf("posts output = %#v", output["posts"])
	}

	resolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{
		URL:   probe.SourceURL,
		Probe: probe,
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.Platform != PlatformID ||
		resolved.Download.Protocol != "inline_json" ||
		resolved.Suffix != ".json" ||
		resolved.ContentID != "1926245291" {
		t.Fatalf("resolved = %#v", resolved)
	}
	if resolved.Metadata["variant_id"] != "json" || resolved.Labels["content_type"] != "account" {
		t.Fatalf("resolved metadata=%#v labels=%#v", resolved.Metadata, resolved.Labels)
	}
}
