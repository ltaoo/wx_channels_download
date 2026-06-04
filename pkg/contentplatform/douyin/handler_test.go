package douyin

import (
	"context"
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
	douyinpkg "wx_channel/pkg/douyin"
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
