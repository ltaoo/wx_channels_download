package x

import (
	"context"
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
	xpkg "wx_channel/pkg/scraper/x"
)

type fakeTimelineFetcher struct {
	gotURL  string
	gotOpts xpkg.TimelineOptions
	page    *xpkg.TimelinePage
}

func (f *fakeTimelineFetcher) FetchUserTimeline(ctx context.Context, rawURL string, opts xpkg.TimelineOptions) (*xpkg.TimelinePage, error) {
	f.gotURL = rawURL
	f.gotOpts = opts
	return f.page, nil
}

func TestMatch(t *testing.T) {
	h := New(&fakeTimelineFetcher{})
	if !h.Match("https://x.com/Barret_China") {
		t.Fatal("expected profile URL to match")
	}
	if h.Match("https://x.com/Barret_China/status/2067997733605331174") {
		t.Fatal("unexpected status URL match")
	}
}

func TestProbeAndResolve(t *testing.T) {
	fetcher := &fakeTimelineFetcher{page: &xpkg.TimelinePage{
		URL:       xpkg.ProfileURL{Username: "Barret_China", Canonical: "https://x.com/Barret_China"},
		SourceURL: "https://x.com/Barret_China",
		APIURL:    "https://x.com/i/api/graphql/op/UserTweets",
		Profile: xpkg.UserProfile{
			ID:             "272736093",
			Username:       "Barret_China",
			Name:           "Barret",
			Description:    "profile bio",
			AvatarURL:      "https://pbs.twimg.com/profile_images/avatar_normal.jpg",
			FollowersCount: 81688,
			FollowingCount: 403,
			StatusesCount:  2471,
			MediaCount:     827,
		},
		Posts: []xpkg.PostSummary{
			{
				ID:            "2067997733605331174",
				URL:           "https://x.com/Barret_China/status/2067997733605331174",
				Text:          "post text",
				CreatedAt:     "Fri Jun 19 15:47:35 +0000 2026",
				ImageURLs:     []string{"https://pbs.twimg.com/media/post.jpg"},
				CoverURL:      "https://pbs.twimg.com/media/post.jpg",
				FavoriteCount: 20,
			},
		},
		BottomCursor: "bottom",
	}}
	h := New(fetcher)
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{
		URL: "https://x.com/Barret_China",
		Extra: map[string]any{
			"count":       float64(12),
			"cookie":      "auth_token=test",
			"csrf_token":  "csrf-test",
			"guest_token": "guest-test",
		},
	})
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if fetcher.gotURL != "https://x.com/Barret_China" ||
		fetcher.gotOpts.Count != 12 ||
		fetcher.gotOpts.Cookie != "auth_token=test" ||
		fetcher.gotOpts.CSRFToken != "csrf-test" ||
		fetcher.gotOpts.GuestToken != "guest-test" {
		t.Fatalf("fetch input url=%q opts=%#v", fetcher.gotURL, fetcher.gotOpts)
	}
	if probe.Platform != PlatformID || probe.ContentID != "272736093" {
		t.Fatalf("probe = %#v", probe)
	}
	if contentdownload.ContentType(probe.Content) != xpkg.ContentTypeUserTimeline ||
		contentdownload.ContentTitle(probe.Content) != "Barret X timeline" ||
		contentdownload.ContentAuthorNickname(probe.Content) != "Barret" {
		t.Fatalf("summary = %#v", contentdownload.ContentSummaryOf(probe.Content))
	}
	output := contentdownload.ContentOutputOf(probe.Content)
	if output["post_count"] != 1 || output["content_count"] != 2471 || output["followers_count"] != 81688 {
		t.Fatalf("output = %#v", output)
	}
	posts, ok := output["posts"].([]xpkg.PostSummary)
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
		resolved.ContentID != "272736093" {
		t.Fatalf("resolved = %#v", resolved)
	}
	if resolved.Metadata["variant_id"] != "json" || resolved.Labels["content_type"] != xpkg.ContentTypeUserTimeline {
		t.Fatalf("resolved metadata=%#v labels=%#v", resolved.Metadata, resolved.Labels)
	}
}
