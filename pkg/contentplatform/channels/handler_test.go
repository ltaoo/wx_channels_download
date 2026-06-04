package channels

import (
	"context"
	"testing"

	apitypes "wx_channel/internal/api/types"
	contentdownload "wx_channel/pkg/contentplatform/download"
)

func TestParseFeedURL(t *testing.T) {
	parts, err := ParseFeedURL("https://channels.weixin.qq.com/web/pages/feed?oid=z0Qii_kLCBA&nid=2-dNcmWxXdc&context_id=x")
	if err != nil {
		t.Fatalf("ParseFeedURL: %v", err)
	}
	if parts.Oid == "" || parts.Oid == "z0Qii_kLCBA" {
		t.Fatalf("oid = %q", parts.Oid)
	}
	if parts.Nid == "" || parts.Nid == "2-dNcmWxXdc" {
		t.Fatalf("nid = %q", parts.Nid)
	}
}

func TestMatch(t *testing.T) {
	h := New(nil)
	if !h.Match("https://channels.weixin.qq.com/web/pages/feed?oid=x") {
		t.Fatal("expected feed url to match")
	}
	if !h.Match("https://weixin.qq.com/sph/AoPX5bEBDd") {
		t.Fatal("expected sph share url to match")
	}
	if h.Match("https://channels.weixin.qq.com/web/pages/profile?oid=x") {
		t.Fatal("profile url should not match")
	}
}

func TestParseSphShareURL(t *testing.T) {
	parts, err := ParseSphShareURL("https://weixin.qq.com/sph/AoPX5bEBDd")
	if err != nil {
		t.Fatalf("ParseSphShareURL weixin: %v", err)
	}
	if parts.ID != "AoPX5bEBDd" {
		t.Fatalf("ID = %q", parts.ID)
	}

	parts, err = ParseSphShareURL("https://channels.weixin.qq.com/finder-preview/pages/sph?id=AoPX5bEBDd")
	if err != nil {
		t.Fatalf("ParseSphShareURL finder-preview: %v", err)
	}
	if parts.ID != "AoPX5bEBDd" {
		t.Fatalf("ID = %q", parts.ID)
	}
}

func TestProbeAndResolveSphShareURL(t *testing.T) {
	h := New(fakeSphFetcher{})
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://weixin.qq.com/sph/AoPX5bEBDd"})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if probe.ContentID != "export123" {
		t.Fatalf("ContentID = %q", probe.ContentID)
	}
	if len(probe.Variants) != 3 {
		t.Fatalf("variants len = %d", len(probe.Variants))
	}

	resolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{
		URL:   "https://weixin.qq.com/sph/AoPX5bEBDd",
		Probe: probe,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	wantURL := "https://video.example.com/path?encfilekey=filekey&token=token"
	if resolved.Download.URL != wantURL {
		t.Fatalf("Download.URL = %q, want %q", resolved.Download.URL, wantURL)
	}
	if resolved.Suffix != ".mp4" {
		t.Fatalf("Suffix = %q", resolved.Suffix)
	}
}

type fakeSphFetcher struct{}

func (fakeSphFetcher) FetchChannelsFeedProfile(oid, nid, reqURL, eid string) (*apitypes.ChannelsFeedProfileResp, error) {
	return nil, nil
}

func (fakeSphFetcher) FetchChannelsSphProfile(reqURL string) (*SphProfile, error) {
	return &SphProfile{
		ShareURL:        reqURL,
		SphID:           "AoPX5bEBDd",
		ExportID:        "export123",
		VideoURL:        "https://video.example.com/path?encfilekey=filekey&token=token&extra=drop",
		Description:     "测试视频",
		CoverURL:        "https://image.example.com/cover.jpg",
		AuthorNickname:  "作者",
		AuthorAvatarURL: "https://image.example.com/avatar.jpg",
	}, nil
}
