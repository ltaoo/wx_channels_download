package douyin

import (
	"context"
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
	douyinpkg "wx_channel/pkg/scraper/douyin"
)

type fakeParser struct{}

func (fakeParser) Parse(ctx context.Context, rawURL string) (*douyinpkg.VideoInfo, error) {
	return &douyinpkg.VideoInfo{
		URL:            "https://example.com/video.mp4",
		Title:          "demo",
		VideoID:        "123",
		UserAgent:      "ua",
		AuthorNickname: "author",
	}, nil
}

func (fakeParser) FetchUserProfile(ctx context.Context, rawURL string, opts douyinpkg.ProfileOptions) (*douyinpkg.ProfilePage, error) {
	secUID := "MS4wLjABAAAAUnitTestSecUserID1234567890"
	return &douyinpkg.ProfilePage{
		URL: douyinpkg.ProfileURL{
			SecUserID: secUID,
			Canonical: "https://www.douyin.com/user/" + secUID,
		},
		APIURL: "https://www.douyin.com/aweme/v1/web/aweme/post/?sec_user_id=" + secUID,
		User: douyinpkg.UserProfile{
			UID:           "uid1",
			SecUID:        secUID,
			UniqueID:      "author_unique",
			Nickname:      "作者",
			Signature:     "签名",
			AvatarURL:     "https://example.com/avatar.jpg",
			FollowerCount: 10,
			AwemeCount:    2,
		},
		Posts: []douyinpkg.AwemeSummary{
			{ID: "1", ContentType: "video", Description: "hello", CoverURL: "https://example.com/cover.jpg"},
		},
		MaxCursor: 123,
		HasMore:   true,
	}, nil
}

func TestResolve(t *testing.T) {
	h := New(fakeParser{})
	resolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{URL: "https://www.douyin.com/video/123"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.Platform != PlatformID {
		t.Fatalf("platform = %s", resolved.Platform)
	}
	if resolved.Download.URL != "https://example.com/video.mp4" {
		t.Fatalf("download url = %s", resolved.Download.URL)
	}
	if resolved.Pipeline == nil || len(resolved.Pipeline.Nodes) == 0 {
		t.Fatal("expected pipeline plan")
	}
}

func TestProbeProfile(t *testing.T) {
	h := New(fakeParser{})
	secUID := "MS4wLjABAAAAUnitTestSecUserID1234567890"
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://www.douyin.com/user/" + secUID})
	if err != nil {
		t.Fatalf("Probe profile: %v", err)
	}
	if probe.ContentID != "uid1" {
		t.Fatalf("content id = %q", probe.ContentID)
	}
	if contentdownload.ContentType(probe.Content) != douyinpkg.ContentTypeUserProfile {
		t.Fatalf("content type = %q", contentdownload.ContentType(probe.Content))
	}
	if contentdownload.ContentAuthorNickname(probe.Content) != "作者" {
		t.Fatalf("author = %q", contentdownload.ContentAuthorNickname(probe.Content))
	}
	resolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{Probe: probe})
	if err != nil {
		t.Fatalf("Resolve profile: %v", err)
	}
	if resolved.Download.Protocol != "inline_json" || resolved.Suffix != ".json" {
		t.Fatalf("resolved download = protocol:%q suffix:%q", resolved.Download.Protocol, resolved.Suffix)
	}
}
