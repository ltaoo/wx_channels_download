package wxchannels

import (
	"context"
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
	channelspkg "wx_channel/pkg/wxchannels"
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

func TestProbeFeedURLContentEnvelope(t *testing.T) {
	h := New(nil)
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://channels.weixin.qq.com/web/pages/feed?oid=oid123&nid=nid123&eid=eid123"})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	envelope, ok := probe.Content.(*FeedURLContentEnvelope)
	if !ok {
		t.Fatalf("probe content = %T, want *FeedURLContentEnvelope", probe.Content)
	}
	if envelope.Data.URL != probe.SourceURL {
		t.Fatalf("feed url data = %#v", envelope.Data)
	}
	if envelope.Metadata.OID != probe.ContentID || envelope.Metadata.NID == "" || envelope.Metadata.EID != "eid123" {
		t.Fatalf("feed url metadata = %#v", envelope.Metadata)
	}
}

func TestProbeAndResolveFeedContentEnvelope(t *testing.T) {
	h := New(fakeFeedFetcher{})
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://channels.weixin.qq.com/web/pages/feed?oid=oid123&nid=nid123&eid=eid123"})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	envelope, ok := probe.Content.(*FeedContentEnvelope)
	if !ok {
		t.Fatalf("probe content = %T, want *FeedContentEnvelope", probe.Content)
	}
	if envelope.Data.ID != "feed123" || envelope.Metadata.NonceID != "nonce123" {
		t.Fatalf("feed envelope = %#v", envelope)
	}
	data, ok := contentdownload.ContentDataOf(probe.Content).(channelspkg.ChannelsObject)
	if !ok || data.ID != "feed123" {
		t.Fatalf("content data = %#v, want ChannelsObject feed123", contentdownload.ContentDataOf(probe.Content))
	}

	resolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{
		URL:   probe.SourceURL,
		Probe: probe,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if _, ok := resolved.Content.(*FeedContentEnvelope); !ok {
		t.Fatalf("resolved content = %T, want *FeedContentEnvelope", resolved.Content)
	}
	wantURL := "https://video.example.com/video.mp4?encfilekey=filekey&token=token"
	if resolved.Download.URL != wantURL {
		t.Fatalf("Download.URL = %q, want %q", resolved.Download.URL, wantURL)
	}
	if resolved.Labels["key"] != "decode123" {
		t.Fatalf("labels = %#v", resolved.Labels)
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
	envelope, ok := probe.Content.(*SphContentEnvelope)
	if !ok {
		t.Fatalf("probe content = %T, want *SphContentEnvelope", probe.Content)
	}
	if envelope.Metadata.SphID != "AoPX5bEBDd" || envelope.Metadata.ExportID != "export123" {
		t.Fatalf("sph metadata = %#v", envelope.Metadata)
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
	if _, ok := resolved.Content.(*SphContentEnvelope); !ok {
		t.Fatalf("resolved content = %T, want *SphContentEnvelope", resolved.Content)
	}
}

type fakeFeedFetcher struct{}

func (fakeFeedFetcher) FetchChannelsFeedProfile(oid, nid, reqURL, eid string) (*channelspkg.ChannelsFeedProfileResp, error) {
	resp := &channelspkg.ChannelsFeedProfileResp{}
	resp.Data.Object = channelspkg.ChannelsObject{
		ID:            "feed123",
		ObjectNonceId: "nonce123",
		SourceURL:     reqURL,
		Type:          "media",
		Contact: channelspkg.ChannelsContact{
			Nickname: "作者",
			HeadUrl:  "https://image.example.com/avatar.jpg",
		},
		ObjectDesc: channelspkg.ChannelsObjectDesc{
			Description: "测试视频",
			MediaType:   4,
			Media: []channelspkg.ChannelsMediaItem{
				{
					URL:          "https://video.example.com/video.mp4?",
					URLToken:     "encfilekey=filekey&token=token&extra=drop",
					CoverUrl:     "https://image.example.com/cover.jpg",
					DecodeKey:    "decode123",
					VideoPlayLen: 5,
					FileSize:     100,
					Width:        1920,
					Height:       1080,
				},
			},
		},
	}
	return resp, nil
}

type fakeSphFetcher struct{}

func (fakeSphFetcher) FetchChannelsFeedProfile(oid, nid, reqURL, eid string) (*channelspkg.ChannelsFeedProfileResp, error) {
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
