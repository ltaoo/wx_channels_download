package wxchannels_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	wxchannels "wx_channel/pkg/scraper/wxchannels"
)

func TestParseFeedURL(t *testing.T) {
	parts, err := wxchannels.ParseFeedURL("https://channels.weixin.qq.com/web/pages/feed?oid=z0Qii_kLCBA&nid=2-dNcmWxXdc&eid=eid123")
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
	parts, err := wxchannels.ParseSphShareURL("https://weixin.qq.com/sph/AoPX5bEBDd")
	if err != nil {
		t.Fatalf("ParseSphShareURL weixin: %v", err)
	}
	if parts.ID != "AoPX5bEBDd" {
		t.Fatalf("ID = %q", parts.ID)
	}

	parts, err = wxchannels.ParseSphShareURL("https://channels.weixin.qq.com/finder-preview/pages/sph?id=AoPX5bEBDd")
	if err != nil {
		t.Fatalf("ParseSphShareURL finder-preview: %v", err)
	}
	if parts.ID != "AoPX5bEBDd" {
		t.Fatalf("ID = %q", parts.ID)
	}
}

func TestChannelsObjectToChannelsFeedProfile(t *testing.T) {
	obj := &wxchannels.ChannelsObject{
		ID:            "feed123",
		ObjectNonceId: "nonce123",
		SourceURL:     "https://channels.weixin.qq.com/web/pages/feed?oid=feed123",
		Type:          "media",
		Contact: wxchannels.ChannelsContact{
			Username: "author",
			Nickname: "作者",
			HeadUrl:  "https://image.example.com/avatar.jpg",
		},
		ObjectDesc: wxchannels.ChannelsObjectDesc{
			Description: "测试视频",
			MediaType:   4,
			Media: []wxchannels.ChannelsMediaItem{
				{
					URL:          "https://video.example.com/video.mp4?encfilekey=filekey",
					URLToken:     "&token=token",
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
	got, err := wxchannels.ChannelsObjectToChannelsFeedProfile(obj)
	if err != nil {
		t.Fatalf("ChannelsObjectToChannelsFeedProfile: %v", err)
	}
	want := &wxchannels.ChannelsFeedProfile{
		ObjectId:    "feed123",
		NonceId:     "nonce123",
		SourceURL:   "https://channels.weixin.qq.com/web/pages/feed?oid=feed123",
		URL:         "https://video.example.com/video.mp4?encfilekey=filekey&token=token",
		Title:       "测试视频",
		DecryptKey:  "decode123",
		CoverURL:    "https://image.example.com/cover.jpg",
		CoverWidth:  1920,
		CoverHeight: 1080,
		Duration:    5,
		FileSize:    100,
		Contact: wxchannels.ChannelsFeedAccount{
			Username:  "author",
			Nickname:  "作者",
			AvatarURL: "https://image.example.com/avatar.jpg",
		},
	}
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("ChannelsObjectToChannelsFeedProfile mismatch (-want +got):\n%s", diff)
	}
}

func TestChannelsObjectToChannelsFeedProfile_Live(t *testing.T) {
	obj := &wxchannels.ChannelsObject{
		ID:            "live_feed_001",
		ObjectNonceId: "live_nonce_001",
		SourceURL:     "https://channels.weixin.qq.com/web/pages/feed?oid=live_feed_001",
		Type:          "video",
		CreateTime:    1711234567,
		Contact: wxchannels.ChannelsContact{
			Username: "streamer",
			Nickname: "主播昵称",
			HeadUrl:  "https://example.com/streamer_avatar.jpg",
		},
		AnchorContact: &wxchannels.ChannelsContact{
			Username:    "anchor_user",
			Nickname:    "主播",
			HeadUrl:     "https://example.com/anchor_avatar.jpg",
			CoverImgUrl: "https://example.com/live_cover.jpg",
		},
		LiveInfo: &wxchannels.ChannelsLiveInfo{
			AnchorStatusFlag: "live",
			SwitchFlag:       1,
			SourceType:       1,
		},
	}
	got, err := wxchannels.ChannelsObjectToChannelsFeedProfile(obj)
	if err != nil {
		t.Fatalf("ChannelsObjectToChannelsFeedProfile live: %v", err)
	}
	want := &wxchannels.ChannelsFeedProfile{
		ObjectId:  "live_feed_001",
		NonceId:   "live_nonce_001",
		SourceURL: "https://channels.weixin.qq.com/web/pages/feed?oid=live_feed_001",
		Title:     "直播",
		CoverURL:  "https://example.com/live_cover.jpg",
		CreatedAt: 1711234567,
		Contact: wxchannels.ChannelsFeedAccount{
			Username:  "anchor_user",
			Nickname:  "主播",
			AvatarURL: "https://example.com/anchor_avatar.jpg",
		},
	}
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("live profile mismatch (-want +got):\n%s", diff)
	}
}

func TestChannelsObjectToChannelsFeedProfile_Picture(t *testing.T) {
	obj := &wxchannels.ChannelsObject{
		ID:            "pic_feed_001",
		ObjectNonceId: "pic_nonce_001",
		SourceURL:     "https://channels.weixin.qq.com/web/pages/feed?oid=pic_feed_001",
		Type:          "picture",
		CreateTime:    1700000000,
		Contact: wxchannels.ChannelsContact{
			Username: "pic_author",
			Nickname: "图片作者",
			HeadUrl:  "https://example.com/pic_avatar.jpg",
		},
		Files: []wxchannels.ChannelsMediaItem{
			{CoverUrl: "https://example.com/pic_cover.jpg", Width: 1280, Height: 720},
		},
		ObjectDesc: wxchannels.ChannelsObjectDesc{
			Description: "一组美图",
			MediaType:   2,
		},
	}
	got, err := wxchannels.ChannelsObjectToChannelsFeedProfile(obj)
	if err != nil {
		t.Fatalf("ChannelsObjectToChannelsFeedProfile picture: %v", err)
	}
	want := &wxchannels.ChannelsFeedProfile{
		ObjectId:  "pic_feed_001",
		NonceId:   "pic_nonce_001",
		SourceURL: "https://channels.weixin.qq.com/web/pages/feed?oid=pic_feed_001",
		Title:     "一组美图",
		CoverURL:  "https://example.com/pic_cover.jpg",
		CreatedAt: 1700000000,
		Contact: wxchannels.ChannelsFeedAccount{
			Username:  "pic_author",
			Nickname:  "图片作者",
			AvatarURL: "https://example.com/pic_avatar.jpg",
		},
	}
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("picture profile mismatch (-want +got):\n%s", diff)
	}
}

func TestChannelsObjectToChannelsFeedProfile_WithoutType(t *testing.T) {
	obj := &wxchannels.ChannelsObject{
		ID:            "feed_no_type",
		ObjectNonceId: "nonce_no_type",
		SourceURL:     "https://channels.weixin.qq.com/web/pages/feed?oid=feed_no_type",
		Contact: wxchannels.ChannelsContact{
			Username: "author3",
			Nickname: "作者三",
			HeadUrl:  "https://example.com/avatar3.jpg",
		},
		ObjectDesc: wxchannels.ChannelsObjectDesc{
			Description: "默认类型视频",
			MediaType:   0,
			Media: []wxchannels.ChannelsMediaItem{
				{
					URL:          "https://video.example.com/v3.mp4",
					URLToken:     "&token=v3token",
					CoverUrl:     "https://example.com/cover3.jpg",
					DecodeKey:    "key3",
					VideoPlayLen: 60,
					FileSize:     5000,
					Width:        640,
					Height:       480,
				},
			},
		},
	}
	got, err := wxchannels.ChannelsObjectToChannelsFeedProfile(obj)
	if err != nil {
		t.Fatalf("ChannelsObjectToChannelsFeedProfile without type: %v", err)
	}
	want := &wxchannels.ChannelsFeedProfile{
		ObjectId:    "feed_no_type",
		NonceId:     "nonce_no_type",
		SourceURL:   "https://channels.weixin.qq.com/web/pages/feed?oid=feed_no_type",
		URL:         "https://video.example.com/v3.mp4?token=v3token",
		Title:       "默认类型视频",
		DecryptKey:  "key3",
		CoverURL:    "https://example.com/cover3.jpg",
		CoverWidth:  640,
		CoverHeight: 480,
		Duration:    60,
		FileSize:    5000,
		Contact: wxchannels.ChannelsFeedAccount{
			Username:  "author3",
			Nickname:  "作者三",
			AvatarURL: "https://example.com/avatar3.jpg",
		},
	}
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("no-type profile mismatch (-want +got):\n%s", diff)
	}
}

func TestChannelsObjectToChannelsFeedProfile_NilObject(t *testing.T) {
	_, err := wxchannels.ChannelsObjectToChannelsFeedProfile(nil)
	if err == nil {
		t.Fatal("expected error for nil object, got nil")
	}
}

func TestChannelsObjectToChannelsFeedProfile_EmptyID(t *testing.T) {
	obj := &wxchannels.ChannelsObject{ID: "  "}
	_, err := wxchannels.ChannelsObjectToChannelsFeedProfile(obj)
	if err == nil {
		t.Fatal("expected error for empty ID, got nil")
	}
}

func TestChannelsObjectToChannelsFeedProfile_MediaType9(t *testing.T) {
	obj := &wxchannels.ChannelsObject{
		ID:            "feed_replay",
		ObjectNonceId: "nonce_replay",
		Type:          "video",
		ObjectDesc: wxchannels.ChannelsObjectDesc{
			MediaType: 9,
			Media:     []wxchannels.ChannelsMediaItem{{URL: "https://example.com/replay.mp4"}},
		},
	}
	_, err := wxchannels.ChannelsObjectToChannelsFeedProfile(obj)
	if err == nil {
		t.Fatal("expected error for mediaType=9, got nil")
	}
	if err.Error() != "不支持直播回放（mediaType=9）" {
		t.Fatalf("error message = %q", err.Error())
	}
}

func TestChannelsObjectToChannelsFeedProfile_PictureNoFiles(t *testing.T) {
	obj := &wxchannels.ChannelsObject{
		ID:            "pic_no_files",
		ObjectNonceId: "nonce_no_files",
		Type:          "picture",
		ObjectDesc:    wxchannels.ChannelsObjectDesc{MediaType: 2},
	}
	_, err := wxchannels.ChannelsObjectToChannelsFeedProfile(obj)
	if err == nil {
		t.Fatal("expected error for picture without files, got nil")
	}
}

func TestChannelsObjectToChannelsFeedProfile_MediaNoMedia(t *testing.T) {
	obj := &wxchannels.ChannelsObject{
		ID:            "media_no_data",
		ObjectNonceId: "nonce_no_data",
		Type:          "media",
		ObjectDesc:    wxchannels.ChannelsObjectDesc{MediaType: 4},
	}
	_, err := wxchannels.ChannelsObjectToChannelsFeedProfile(obj)
	if err == nil {
		t.Fatal("expected error for media without media data, got nil")
	}
}

func TestChannelsObjectToChannelsFeedProfile_SpecFromObject(t *testing.T) {
	obj := &wxchannels.ChannelsObject{
		ID:            "feed_with_spec",
		ObjectNonceId: "nonce_spec",
		SourceURL:     "https://channels.weixin.qq.com/web/pages/feed?oid=feed_with_spec",
		Type:          "media",
		Spec: []wxchannels.ChannelsMediaSpec{
			{FileFormat: "mp4", Width: 1920, Height: 1080, DurationMs: 30000},
		},
		Contact: wxchannels.ChannelsContact{
			Username: "spec_author",
			Nickname: "规格作者",
			HeadUrl:  "https://example.com/spec_avatar.jpg",
		},
		ObjectDesc: wxchannels.ChannelsObjectDesc{
			Description: "使用对象级 spec",
			MediaType:   4,
			Media: []wxchannels.ChannelsMediaItem{
				{
					URL:          "https://video.example.com/spec.mp4",
					URLToken:     "&token=spec_token",
					CoverUrl:     "https://example.com/spec_cover.jpg",
					DecodeKey:    "spec_key",
					VideoPlayLen: 120,
					FileSize:     10000,
					Width:        0,
					Height:       0,
				},
			},
		},
	}
	got, err := wxchannels.ChannelsObjectToChannelsFeedProfile(obj)
	if err != nil {
		t.Fatalf("ChannelsObjectToChannelsFeedProfile spec: %v", err)
	}
	if len(got.Spec) != 1 {
		t.Fatalf("spec length = %d, want 1", len(got.Spec))
	}
	wantSpec := wxchannels.ChannelsMediaSpec{FileFormat: "mp4", Width: 1920, Height: 1080, DurationMs: 30000}
	if diff := cmp.Diff(wantSpec, got.Spec[0]); diff != "" {
		t.Errorf("spec mismatch (-want +got):\n%s", diff)
	}
}

func TestChannelsObjectToChannelsFeedProfile_LiveAnchorContactFallback(t *testing.T) {
	obj := &wxchannels.ChannelsObject{
		ID:            "live_no_anchor",
		ObjectNonceId: "nonce_live_no_anchor",
		SourceURL:     "https://channels.weixin.qq.com/web/pages/feed?oid=live_no_anchor",
		Contact: wxchannels.ChannelsContact{
			Username: "fallback_user",
			Nickname: "回退主播",
			HeadUrl:  "https://example.com/fallback_avatar.jpg",
		},
		ObjectDesc: wxchannels.ChannelsObjectDesc{
			Description: "直播回退测试",
			Media:       []wxchannels.ChannelsMediaItem{{CoverUrl: "https://example.com/media_cover.jpg"}},
		},
		LiveInfo: &wxchannels.ChannelsLiveInfo{AnchorStatusFlag: "live"},
	}
	got, err := wxchannels.ChannelsObjectToChannelsFeedProfile(obj)
	if err != nil {
		t.Fatalf("ChannelsObjectToChannelsFeedProfile live fallback: %v", err)
	}
	want := &wxchannels.ChannelsFeedProfile{
		ObjectId:  "live_no_anchor",
		NonceId:   "nonce_live_no_anchor",
		SourceURL: "https://channels.weixin.qq.com/web/pages/feed?oid=live_no_anchor",
		Title:     "直播",
		CoverURL:  "https://example.com/media_cover.jpg",
		Contact: wxchannels.ChannelsFeedAccount{
			Username:  "fallback_user",
			Nickname:  "回退主播",
			AvatarURL: "https://example.com/fallback_avatar.jpg",
		},
	}
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("live fallback profile mismatch (-want +got):\n%s", diff)
	}
}

func TestBuildContentID(t *testing.T) {
	tests := []struct {
		externalID string
		want       string
	}{
		{"feed123", "wx_channels:feed123"},
		{"14885057406549363320", "wx_channels:14885057406549363320"},
		{"", "wx_channels:"},
	}
	for _, tt := range tests {
		got := wxchannels.BuildContentID(tt.externalID)
		if got != tt.want {
			t.Errorf("BuildContentID(%q) = %q, want %q", tt.externalID, got, tt.want)
		}
	}
}

func TestBuildAccountID(t *testing.T) {
	tests := []struct {
		externalID string
		want       string
	}{
		{"v2_060000231003b20faec8c7e48a10cbd2c804ee34b07796d7c188c77d0735f563ef0156a0425e@finder", "wx_channels:v2_060000231003b20faec8c7e48a10cbd2c804ee34b07796d7c188c77d0735f563ef0156a0425e@finder"},
		{"simple_user", "wx_channels:simple_user"},
	}
	for _, tt := range tests {
		got := wxchannels.BuildAccountID(tt.externalID)
		if got != tt.want {
			t.Errorf("BuildAccountID(%q) = %q, want %q", tt.externalID, got, tt.want)
		}
	}
}

func TestBuildJumpURL(t *testing.T) {
	tests := []struct {
		name string
		feed *wxchannels.ChannelsFeedProfile
		want string
	}{
		{
			name: "with source_url",
			feed: &wxchannels.ChannelsFeedProfile{
				SourceURL: "https://channels.weixin.qq.com/web/pages/feed?oid=abc&nid=def",
			},
			want: "https://channels.weixin.qq.com/web/pages/feed?oid=abc&nid=def",
		},
		{
			name: "with username",
			feed: &wxchannels.ChannelsFeedProfile{
				ObjectId: "feed_jump_001",
				Contact:  wxchannels.ChannelsFeedAccount{Username: "test_user"},
			},
			want: "https://channels.weixin.qq.com/web/pages/feed?username=test_user",
		},
		{
			name: "nil feed",
			feed: nil,
			want: "https://channels.weixin.qq.com/web/pages/feed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wxchannels.BuildJumpURL(tt.feed)
			if got != tt.want {
				t.Errorf("BuildJumpURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPlatformID(t *testing.T) {
	if wxchannels.PlatformID != "wx_channels" {
		t.Errorf("PlatformID = %q, want \"wx_channels\"", wxchannels.PlatformID)
	}
}

// TestChannelsObjectToChannelsFeedProfile_FullConversion 模拟完整的 JSON → Profile → ID 生成流程
func TestChannelsObjectToChannelsFeedProfile_FullConversion(t *testing.T) {
	obj := &wxchannels.ChannelsObject{
		ID:            "14885057406549363320",
		ObjectNonceId: "nonce_full_test",
		SourceURL:     "https://channels.weixin.qq.com/web/pages/feed?oid=14885057406549363320&nid=nonce_full_test",
		Type:          "media",
		CreateTime:    1701234567,
		Contact: wxchannels.ChannelsContact{
			Username: "v2_060000231003b20f@finder",
			Nickname: "测试视频号主",
			HeadUrl:  "https://example.com/real_avatar.jpg",
		},
		ObjectDesc: wxchannels.ChannelsObjectDesc{
			Description: "这是一条真实的测试视频",
			MediaType:   4,
			Media: []wxchannels.ChannelsMediaItem{
				{
					URL:          "https://finder.video.qq.com/251/20302/stodownload?encfilekey=actual_key",
					URLToken:     "&token=actual_token",
					CoverUrl:     "https://finder.video.qq.com/251/cover.jpg",
					DecodeKey:    "actual_decode_key",
					VideoPlayLen: 125,
					FileSize:     8388608,
					Width:        1920,
					Height:       1080,
					Spec: []wxchannels.ChannelsMediaSpec{
						{FileFormat: "mp4", Width: 1920, Height: 1080, DurationMs: 125000},
						{FileFormat: "mp4", Width: 1280, Height: 720, DurationMs: 125000},
					},
				},
			},
		},
	}

	got, err := wxchannels.ChannelsObjectToChannelsFeedProfile(obj)
	if err != nil {
		t.Fatalf("conversion failed: %v", err)
	}

	want := &wxchannels.ChannelsFeedProfile{
		ObjectId:    "14885057406549363320",
		NonceId:     "nonce_full_test",
		SourceURL:   "https://channels.weixin.qq.com/web/pages/feed?oid=14885057406549363320&nid=nonce_full_test",
		URL:         "https://finder.video.qq.com/251/20302/stodownload?encfilekey=actual_key&token=actual_token",
		Title:       "这是一条真实的测试视频",
		DecryptKey:  "actual_decode_key",
		CoverURL:    "https://finder.video.qq.com/251/cover.jpg",
		CoverWidth:  1920,
		CoverHeight: 1080,
		Duration:    125,
		FileSize:    8388608,
		CreatedAt:   1701234567,
		Spec: []wxchannels.ChannelsMediaSpec{
			{FileFormat: "mp4", Width: 1920, Height: 1080, DurationMs: 125000},
			{FileFormat: "mp4", Width: 1280, Height: 720, DurationMs: 125000},
		},
		Contact: wxchannels.ChannelsFeedAccount{
			Username:  "v2_060000231003b20f@finder",
			Nickname:  "测试视频号主",
			AvatarURL: "https://example.com/real_avatar.jpg",
		},
	}
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("full conversion profile mismatch (-want +got):\n%s", diff)
	}

	// 验证 ID 构建
	if id := wxchannels.BuildContentID(got.ObjectId); id != "wx_channels:14885057406549363320" {
		t.Errorf("BuildContentID = %q", id)
	}
	if id := wxchannels.BuildAccountID(got.Contact.Username); id != "wx_channels:v2_060000231003b20f@finder" {
		t.Errorf("BuildAccountID = %q", id)
	}
}
