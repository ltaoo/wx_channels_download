package instagram

import (
	"context"
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
	instagrampkg "wx_channel/pkg/scraper/instagram"
)

type fakeProfileFetcher struct {
	gotURL  string
	gotOpts instagrampkg.ProfileOptions
	page    *instagrampkg.ProfilePage
}

func (f *fakeProfileFetcher) FetchUserProfile(ctx context.Context, rawURL string, opts instagrampkg.ProfileOptions) (*instagrampkg.ProfilePage, error) {
	f.gotURL = rawURL
	f.gotOpts = opts
	return f.page, nil
}

func TestMatch(t *testing.T) {
	h := New(&fakeProfileFetcher{})
	if !h.Match("https://www.instagram.com/r_ap82_/") {
		t.Fatal("expected profile URL to match")
	}
	if h.Match("https://www.instagram.com/p/ABC123/") {
		t.Fatal("unexpected post URL match")
	}
}

func TestProbeAndResolve(t *testing.T) {
	fetcher := &fakeProfileFetcher{page: &instagrampkg.ProfilePage{
		URL: instagrampkg.ProfileURL{
			Username:  "r_ap82_",
			Canonical: "https://www.instagram.com/r_ap82_/",
		},
		SourceURL: "https://www.instagram.com/r_ap82_/",
		APIURL:    "https://www.instagram.com/api/v1/users/web_profile_info/?username=r_ap82_",
		Profile: instagrampkg.UserProfile{
			ID:              "11599648301",
			Username:        "r_ap82_",
			FullName:        "Marina Amatsu",
			Biography:       "bio",
			ProfilePicURLHD: "https://cdn.example.com/avatar.jpg",
			FollowersCount:  185123,
			FollowingCount:  1,
			MediaCount:      702,
		},
		Posts: []instagrampkg.PostSummary{
			{
				ID:               "1",
				Shortcode:        "ABC123",
				URL:              "https://www.instagram.com/p/ABC123/",
				Caption:          "caption",
				ThumbnailURL:     "https://cdn.example.com/thumb.jpg",
				DisplayURL:       "https://cdn.example.com/post.jpg",
				TakenAtTimestamp: 1781796018,
				LikeCount:        10,
				CommentCount:     2,
			},
		},
	}}
	h := New(fetcher)
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{
		URL:   "https://www.instagram.com/r_ap82_/",
		Extra: map[string]any{"count": float64(12), "cookie": "csrftoken=test"},
	})
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if fetcher.gotURL != "https://www.instagram.com/r_ap82_/" ||
		fetcher.gotOpts.Count != 12 ||
		fetcher.gotOpts.Cookie != "csrftoken=test" {
		t.Fatalf("fetch input url=%q opts=%#v", fetcher.gotURL, fetcher.gotOpts)
	}
	if probe.Platform != PlatformID || probe.ContentID != "11599648301" {
		t.Fatalf("probe = %#v", probe)
	}
	if contentdownload.ContentType(probe.Content) != instagrampkg.ContentTypeUserProfile ||
		contentdownload.ContentTitle(probe.Content) != "Marina Amatsu Instagram profile" ||
		contentdownload.ContentAuthorNickname(probe.Content) != "Marina Amatsu" {
		t.Fatalf("summary = %#v", contentdownload.ContentSummaryOf(probe.Content))
	}
	output := contentdownload.ContentOutputOf(probe.Content)
	if output["post_count"] != 1 || output["content_count"] != 702 || output["followers_count"] != 185123 {
		t.Fatalf("output = %#v", output)
	}
	posts, ok := output["posts"].([]instagrampkg.PostSummary)
	if !ok || len(posts) != 1 || posts[0].Shortcode != "ABC123" {
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
		resolved.ContentID != "11599648301" {
		t.Fatalf("resolved = %#v", resolved)
	}
	if resolved.Metadata["variant_id"] != "json" || resolved.Labels["content_type"] != instagrampkg.ContentTypeUserProfile {
		t.Fatalf("resolved metadata=%#v labels=%#v", resolved.Metadata, resolved.Labels)
	}
}
