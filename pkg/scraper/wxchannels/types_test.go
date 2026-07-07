package wxchannels

import "testing"

func TestParseFeedURL(t *testing.T) {
	parts, err := ParseFeedURL("https://channels.weixin.qq.com/web/pages/feed?oid=z0Qii_kLCBA&nid=2-dNcmWxXdc&eid=eid123")
	if err != nil {
		t.Fatalf("ParseFeedURL: %v", err)
	}
	if parts.Oid == "" || parts.Oid == "z0Qii_kLCBA" {
		t.Fatalf("OID = %q", parts.Oid)
	}
	if parts.Nid == "" || parts.Nid == "2-dNcmWxXdc" {
		t.Fatalf("NID = %q", parts.Nid)
	}
	if parts.Eid != "eid123" {
		t.Fatalf("EID = %q", parts.Eid)
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

func TestChannelsObjectToChannelsFeedProfile(t *testing.T) {
	obj := &ChannelsObject{
		ID:            "feed123",
		ObjectNonceId: "nonce123",
		SourceURL:     "https://channels.weixin.qq.com/web/pages/feed?oid=feed123",
		Type:          "media",
		Contact: ChannelsContact{
			Username: "author",
			Nickname: "作者",
			HeadUrl:  "https://image.example.com/avatar.jpg",
		},
		ObjectDesc: ChannelsObjectDesc{
			Description: "测试视频",
			MediaType:   4,
			Media: []ChannelsMediaItem{
				{
					URL:          "https://video.example.com/video.mp4?",
					URLToken:     "encfilekey=filekey&token=token",
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
	profile, err := ChannelsObjectToChannelsFeedProfile(obj)
	if err != nil {
		t.Fatalf("ChannelsObjectToChannelsFeedProfile: %v", err)
	}
	if profile.ObjectId != "feed123" || profile.NonceId != "nonce123" {
		t.Fatalf("profile ids = %#v", profile)
	}
	if profile.URL != "https://video.example.com/video.mp4?encfilekey=filekey&token=token" {
		t.Fatalf("profile URL = %q", profile.URL)
	}
	if profile.CoverWidth != 1920 || profile.CoverHeight != 1080 {
		t.Fatalf("cover size = %dx%d", profile.CoverWidth, profile.CoverHeight)
	}
}
