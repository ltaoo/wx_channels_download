package wxchannels_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	adapter "wx_channel/internal/adapter/wxchannels"
)

func TestParseFeedURL(t *testing.T) {
	parts, err := adapter.ParseFeedURL("https://channels.weixin.qq.com/web/pages/feed?oid=z0Qii_kLCBA&nid=2-dNcmWxXdc&eid=eid123")
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
	parts, err := adapter.ParseSphShareURL("https://weixin.qq.com/sph/AoPX5bEBDd")
	if err != nil {
		t.Fatalf("ParseSphShareURL weixin: %v", err)
	}
	if parts.ID != "AoPX5bEBDd" {
		t.Fatalf("ID = %q", parts.ID)
	}

	parts, err = adapter.ParseSphShareURL("https://channels.weixin.qq.com/finder-preview/pages/sph?id=AoPX5bEBDd")
	if err != nil {
		t.Fatalf("ParseSphShareURL finder-preview: %v", err)
	}
	if parts.ID != "AoPX5bEBDd" {
		t.Fatalf("ID = %q", parts.ID)
	}
}

func makeVideoObject() *adapter.ChannelsObject {
	return &adapter.ChannelsObject{
		ID:            "feed123",
		ObjectNonceId: "nonce123",
		SourceURL:     "https://channels.weixin.qq.com/web/pages/feed?oid=feed123",
		Type:          "media",
		Contact: adapter.ChannelsContact{
			Username:  "author",
			Nickname:  "作者",
			HeadUrl:   "https://image.example.com/avatar.jpg",
			Signature: "作者签名",
		},
		ObjectDesc: adapter.ChannelsObjectDesc{
			Description: "测试视频",
			MediaType:   adapter.MediaTypeVideo,
			Media: []adapter.ChannelsMediaItem{
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
}

func TestPickSpec_NoSpec(t *testing.T) {
	obj := makeVideoObject()
	got := adapter.PickSpec(obj)
	if got != "" {
		t.Errorf("PickSpec(no spec) = %q, want \"\"", got)
	}
}

func TestPickSpec_FromMedia(t *testing.T) {
	obj := makeVideoObject()
	obj.ObjectDesc.Media[0].Spec = []adapter.ChannelsMediaSpec{
		{FileFormat: "h264", Width: 1920, Height: 1080},
	}
	got := adapter.PickSpec(obj)
	if got != "h264" {
		t.Errorf("PickSpec(media spec) = %q, want \"h264\"", got)
	}
}

func TestPickSpec_FromObject(t *testing.T) {
	obj := makeVideoObject()
	obj.Spec = []adapter.ChannelsMediaSpec{
		{FileFormat: "mp4", Width: 1920, Height: 1080},
	}
	got := adapter.PickSpec(obj)
	if got != "mp4" {
		t.Errorf("PickSpec(object spec) = %q, want \"mp4\"", got)
	}
}

func TestDecryptKeyInt(t *testing.T) {
	obj := makeVideoObject()
	got := adapter.DecryptKeyInt(obj)
	if got != 0 {
		t.Errorf("DecryptKeyInt(non-numeric) = %d, want 0", got)
	}

	obj.ObjectDesc.Media[0].DecodeKey = "42"
	got = adapter.DecryptKeyInt(obj)
	if got != 42 {
		t.Errorf("DecryptKeyInt(42) = %d, want 42", got)
	}
}

func TestDecryptKeyInt_NoMedia(t *testing.T) {
	obj := &adapter.ChannelsObject{ID: "test"}
	got := adapter.DecryptKeyInt(obj)
	if got != 0 {
		t.Errorf("DecryptKeyInt(no media) = %d, want 0", got)
	}
}

func TestObjectTitle_WithDescription(t *testing.T) {
	obj := makeVideoObject()
	got := adapter.ObjectTitle(obj)
	if got != "测试视频" {
		t.Errorf("ObjectTitle = %q, want \"测试视频\"", got)
	}
}

func TestObjectTitle_EmptyDescription(t *testing.T) {
	obj := makeVideoObject()
	obj.ObjectDesc.Description = ""
	got := adapter.ObjectTitle(obj)
	if got != "feed123" {
		t.Errorf("ObjectTitle(no desc) = %q, want \"feed123\"", got)
	}
}

func TestObjectTitle_EmptyDescriptionAndID(t *testing.T) {
	obj := makeVideoObject()
	obj.ObjectDesc.Description = ""
	obj.ID = ""
	got := adapter.ObjectTitle(obj)
	if got == "" {
		t.Error("ObjectTitle should not be empty (should fallback to timestamp)")
	}
}

func TestObjectTitle_Live(t *testing.T) {
	obj := makeVideoObject()
	obj.LiveInfo = &adapter.ChannelsLiveInfo{AnchorStatusFlag: "live"}
	got := adapter.ObjectTitle(obj)
	if got != "直播" {
		t.Errorf("ObjectTitle(live) = %q, want \"直播\"", got)
	}
}

func TestObjectURL_Video(t *testing.T) {
	obj := makeVideoObject()
	got := adapter.ObjectURL(obj)
	if got != "https://video.example.com/video.mp4?encfilekey=filekey&token=token" {
		t.Errorf("ObjectURL = %q", got)
	}
}

func TestObjectURL_Live(t *testing.T) {
	obj := makeVideoObject()
	obj.LiveInfo = &adapter.ChannelsLiveInfo{AnchorStatusFlag: "live"}
	got := adapter.ObjectURL(obj)
	if got != "" {
		t.Errorf("ObjectURL(live) = %q, want \"\"", got)
	}
}

func TestObjectURL_Picture(t *testing.T) {
	obj := makeVideoObject()
	obj.Type = "picture"
	got := adapter.ObjectURL(obj)
	if got != "" {
		t.Errorf("ObjectURL(picture) = %q, want \"\"", got)
	}
}

func TestBuildJumpURLFromParts_WithSourceURL(t *testing.T) {
	got := adapter.BuildJumpURLFromParts("oid123", "nid456", "https://channels.weixin.qq.com/web/pages/feed?oid=abc", "user")
	if got != "https://channels.weixin.qq.com/web/pages/feed?oid=abc" {
		t.Errorf("BuildJumpURLFromParts(sourceURL) = %q", got)
	}
}

func TestBuildJumpURLFromParts_WithUsername(t *testing.T) {
	got := adapter.BuildJumpURLFromParts("feed_jump_001", "", "", "test_user")
	if got != "https://channels.weixin.qq.com/web/pages/feed?username=test_user" {
		t.Errorf("BuildJumpURLFromParts(username) = %q", got)
	}
}

func TestBuildJumpURLFromParts_NilLike(t *testing.T) {
	got := adapter.BuildJumpURLFromParts("", "", "", "")
	if got != "https://channels.weixin.qq.com/web/pages/feed" {
		t.Errorf("BuildJumpURLFromParts(empty) = %q", got)
	}
}

func TestToContent_Video(t *testing.T) {
	obj := makeVideoObject()
	got, _, err := adapter.ToContent(obj)
	if err != nil {
		t.Fatalf("ToContent: %v", err)
	}
	if got.ExternalId != "feed123" {
		t.Errorf("ExternalId = %q", got.ExternalId)
	}
	if got.Type != "video" {
		t.Errorf("ContentType = %q", got.Type)
	}
	if got.URL != "https://video.example.com/video.mp4?encfilekey=filekey&token=token" {
		t.Errorf("URL = %q", got.URL)
	}
	if got.CoverWidth != "1920" {
		t.Errorf("CoverWidth = %q, want %q", got.CoverWidth, "1920")
	}
	if got.CoverHeight != "1080" {
		t.Errorf("CoverHeight = %q, want %q", got.CoverHeight, "1080")
	}
}

func TestToContent_Nil(t *testing.T) {
	_, _, err := adapter.ToContent(nil)
	if err == nil {
		t.Fatal("expected error for nil object")
	}
}

func TestToContent_EmptyID(t *testing.T) {
	_, _, err := adapter.ToContent(&adapter.ChannelsObject{})
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestToContent_MediaType9(t *testing.T) {
	obj := makeVideoObject()
	obj.ObjectDesc.MediaType = adapter.MediaTypeLive
	_, _, err := adapter.ToContent(obj)
	if err == nil {
		t.Fatal("expected error for mediaType=9")
	}
}

func TestToContent_Live(t *testing.T) {
	obj := makeVideoObject()
	obj.LiveInfo = &adapter.ChannelsLiveInfo{AnchorStatusFlag: "live"}
	got, _, err := adapter.ToContent(obj)
	if err != nil {
		t.Fatalf("ToContent(live): %v", err)
	}
	if got.Type != "live" {
		t.Errorf("ContentType = %q, want \"live\"", got.Type)
	}
	if got.Title != "直播" {
		t.Errorf("Title = %q, want \"直播\"", got.Title)
	}
}

func TestToContent_Picture(t *testing.T) {
	obj := &adapter.ChannelsObject{
		ID:            "pic_feed_001",
		ObjectNonceId: "pic_nonce_001",
		SourceURL:     "https://channels.weixin.qq.com/web/pages/feed?oid=pic_feed_001",
		Type:          "picture",
		CreateTime:    1700000000,
		Contact: adapter.ChannelsContact{
			Username: "pic_author",
			Nickname: "图片作者",
			HeadUrl:  "https://example.com/pic_avatar.jpg",
		},
		Files: []adapter.ChannelsMediaItem{
			{CoverUrl: "https://example.com/pic.jpg", Width: 1280, Height: 720},
		},
		ObjectDesc: adapter.ChannelsObjectDesc{
			Description: "一组美图",
			MediaType:   adapter.MediaTypePicture,
		},
	}
	got, _, err := adapter.ToContent(obj)
	if err != nil {
		t.Fatalf("ToContent(picture): %v", err)
	}
	if got.Type != "picture" {
		t.Errorf("ContentType = %q, want \"picture\"", got.Type)
	}
	if got.CoverURL != "https://example.com/pic.jpg" {
		t.Errorf("CoverURL = %q", got.CoverURL)
	}
	if got.CoverWidth != "1280" {
		t.Errorf("CoverWidth = %q, want %q", got.CoverWidth, "1280")
	}
	if got.CoverHeight != "720" {
		t.Errorf("CoverHeight = %q, want %q", got.CoverHeight, "720")
	}
}

func TestToContent_LiveAnchorContactFallback(t *testing.T) {
	obj := &adapter.ChannelsObject{
		ID:            "live_no_anchor",
		ObjectNonceId: "nonce_live_no_anchor",
		SourceURL:     "https://channels.weixin.qq.com/web/pages/feed?oid=live_no_anchor",
		Contact: adapter.ChannelsContact{
			Username: "fallback_user",
			Nickname: "回退主播",
			HeadUrl:  "https://example.com/fallback_avatar.jpg",
		},
		ObjectDesc: adapter.ChannelsObjectDesc{
			Description: "直播回退测试",
			Media:       []adapter.ChannelsMediaItem{{CoverUrl: "https://example.com/media.jpg"}},
		},
		LiveInfo: &adapter.ChannelsLiveInfo{AnchorStatusFlag: "live"},
	}
	content, _, err := adapter.ToContent(obj)
	if err != nil {
		t.Fatalf("ToContent(live fallback): %v", err)
	}
	if content.CoverURL != "https://example.com/media.jpg" {
		t.Errorf("CoverURL = %q", content.CoverURL)
	}
	account, err := adapter.ToAccount(obj)
	if err != nil {
		t.Fatalf("ToAccount(live fallback): %v", err)
	}
	if account.ExternalId != "fallback_user" {
		t.Errorf("Account ExternalId = %q", account.ExternalId)
	}
}

func TestToContent_FullConversion(t *testing.T) {
	obj := &adapter.ChannelsObject{
		ID:            "14885057406549363320",
		ObjectNonceId: "nonce_full_test",
		SourceURL:     "https://channels.weixin.qq.com/web/pages/feed?oid=14885057406549363320&nid=nonce_full_test",
		Type:          "media",
		CreateTime:    1701234567,
		Contact: adapter.ChannelsContact{
			Username: "v2_060000231003b20f@finder",
			Nickname: "测试视频号主",
			HeadUrl:  "https://example.com/real_avatar.jpg",
		},
		ObjectDesc: adapter.ChannelsObjectDesc{
			Description: "这是一条真实的测试视频",
			MediaType:   adapter.MediaTypeVideo,
			Media: []adapter.ChannelsMediaItem{
				{
					URL:          "https://finder.video.qq.com/251/20302/stodownload?encfilekey=actual_key",
					URLToken:     "&token=actual_token",
					CoverUrl:     "https://finder.video.qq.com/251/cover.jpg",
					DecodeKey:    "actual_decode_key",
					VideoPlayLen: 125,
					FileSize:     8388608,
					Width:        1920,
					Height:       1080,
					Spec: []adapter.ChannelsMediaSpec{
						{FileFormat: "mp4", Width: 1920, Height: 1080, DurationMs: 125000},
						{FileFormat: "mp4", Width: 1280, Height: 720, DurationMs: 125000},
					},
				},
			},
		},
	}

	got, _, err := adapter.ToContent(obj)
	if err != nil {
		t.Fatalf("ToContent(full): %v", err)
	}

	wantExternalId := "14885057406549363320"
	wantURL := "https://finder.video.qq.com/251/20302/stodownload?encfilekey=actual_key&token=actual_token"
	if got.ExternalId != wantExternalId {
		t.Errorf("ExternalId = %q, want %q", got.ExternalId, wantExternalId)
	}
	if got.URL != wantURL {
		t.Errorf("ContentURL = %q, want %q", got.URL, wantURL)
	}
	if got.CoverWidth != "1920" {
		t.Errorf("CoverWidth = %q, want %q", got.CoverWidth, "1920")
	}
	if got.CoverHeight != "1080" {
		t.Errorf("CoverHeight = %q, want %q", got.CoverHeight, "1080")
	}

	// Verify ID building
	if id := adapter.BuildContentID(got.ExternalId); id != "wxchannels:14885057406549363320" {
		t.Errorf("BuildContentID = %q", id)
	}
}

func TestToAccount(t *testing.T) {
	obj := makeVideoObject()
	got, err := adapter.ToAccount(obj)
	if err != nil {
		t.Fatalf("ToAccount: %v", err)
	}
	if got.ExternalId != "author" {
		t.Errorf("ExternalId = %q, want \"author\"", got.ExternalId)
	}
	if got.Nickname != "作者" {
		t.Errorf("Nickname = %q, want \"作者\"", got.Nickname)
	}
	if got.Signature != "作者签名" {
		t.Errorf("Signature = %q, want \"作者签名\"", got.Signature)
	}
	if id := adapter.BuildAccountID(got.ExternalId); id != "wxchannels:author" {
		t.Errorf("BuildAccountID = %q, want \"wxchannels:author\"", id)
	}
}

func TestBuildContentID(t *testing.T) {
	tests := []struct {
		externalID string
		want       string
	}{
		{"feed123", "wxchannels:feed123"},
		{"14885057406549363320", "wxchannels:14885057406549363320"},
		{"", "wxchannels:"},
	}
	for _, tt := range tests {
		got := adapter.BuildContentID(tt.externalID)
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
		{"v2_060000231003b20faec8c7e48a10cbd2c804ee34b07796d7c188c77d0735f563ef0156a0425e@finder", "wxchannels:v2_060000231003b20faec8c7e48a10cbd2c804ee34b07796d7c188c77d0735f563ef0156a0425e@finder"},
		{"simple_user", "wxchannels:simple_user"},
	}
	for _, tt := range tests {
		got := adapter.BuildAccountID(tt.externalID)
		if got != tt.want {
			t.Errorf("BuildAccountID(%q) = %q, want %q", tt.externalID, got, tt.want)
		}
	}
}

func TestPlatformID(t *testing.T) {
	if adapter.PlatformID != "wxchannels" {
		t.Errorf("PlatformID = %q, want \"wx_channels\"", adapter.PlatformID)
	}
}

func TestObjectTitle_LiveAnchorContactFallback(t *testing.T) {
	obj := &adapter.ChannelsObject{
		ID:            "live_no_anchor",
		ObjectNonceId: "nonce_live_no_anchor",
		LiveInfo:      &adapter.ChannelsLiveInfo{AnchorStatusFlag: "live"},
	}
	got := adapter.ObjectTitle(obj)
	if got != "直播" {
		t.Errorf("ObjectTitle(live fallback) = %q, want \"直播\"", got)
	}
}

func TestPickSpec_MediaSpecsVsObjectSpecs(t *testing.T) {
	obj := makeVideoObject()
	obj.ObjectDesc.Media[0].Spec = []adapter.ChannelsMediaSpec{
		{FileFormat: "h264", Width: 1920},
	}
	obj.Spec = []adapter.ChannelsMediaSpec{
		{FileFormat: "h265", Width: 3840},
	}
	// Media specs should take precedence
	got := adapter.PickSpec(obj)
	if got != "h264" {
		t.Errorf("PickSpec = %q, want \"h264\" (media spec takes priority)", got)
	}
}

func TestObjectURL_NoMedia(t *testing.T) {
	obj := &adapter.ChannelsObject{ID: "test", Type: "media"}
	got := adapter.ObjectURL(obj)
	if got != "" {
		t.Errorf("ObjectURL(no media) = %q, want \"\"", got)
	}
}

func TestBuildJumpURLFromParts_WithObjectIdAndNonceId(t *testing.T) {
	tests := []struct {
		name      string
		objectId  string
		nonceId   string
		sourceURL string
		username  string
		want      string
	}{
		{
			name:     "numeric objectId",
			objectId: "14962486294771997060",
			nonceId:  "",
			want:     "https://channels.weixin.qq.com/web/pages/feed?oid=z6VuAqyJGYQ",
		},
		{
			name:     "numeric objectId with username",
			objectId: "14962486294771997060",
			username: "test_user",
			want:     "https://channels.weixin.qq.com/web/pages/feed?username=test_user&oid=z6VuAqyJGYQ",
		},
		{
			name:     "underscore-separated nonceId uses first segment",
			objectId: "14962486294771997060",
			nonceId:  "4390481592474233535_0_146_0_0",
			username: "test_user",
			want:     "https://channels.weixin.qq.com/web/pages/feed?username=test_user&oid=z6VuAqyJGYQ&nid=PO4fvyBRar8",
		},
		{
			name:     "non-numeric objectId ignored",
			objectId: "feed_001",
			username: "test_user",
			want:     "https://channels.weixin.qq.com/web/pages/feed?username=test_user",
		},
		{
			name:      "sourceURL overrides",
			objectId:  "oid123",
			sourceURL: "https://channels.weixin.qq.com/web/pages/feed?oid=abc",
			want:      "https://channels.weixin.qq.com/web/pages/feed?oid=abc",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adapter.BuildJumpURLFromParts(tt.objectId, tt.nonceId, tt.sourceURL, tt.username)
			if got != tt.want {
				t.Errorf("got  %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestToContent_PictureNoFiles(t *testing.T) {
	obj := &adapter.ChannelsObject{
		ID:            "pic_no_files",
		ObjectNonceId: "nonce_no_files",
		Type:          "picture",
		ObjectDesc:    adapter.ChannelsObjectDesc{MediaType: adapter.MediaTypePicture},
	}
	_, _, err := adapter.ToContent(obj)
	if err == nil {
		t.Fatal("expected error for picture without files")
	}
}

func TestToContent_MediaNoMedia(t *testing.T) {
	obj := &adapter.ChannelsObject{
		ID:            "media_no_data",
		ObjectNonceId: "nonce_no_data",
		Type:          "media",
		ObjectDesc:    adapter.ChannelsObjectDesc{MediaType: adapter.MediaTypeVideo},
	}
	_, _, err := adapter.ToContent(obj)
	if err == nil {
		t.Fatal("expected error for media without media data")
	}
}

func TestToAccount_Nil(t *testing.T) {
	_, err := adapter.ToAccount(nil)
	if err == nil {
		t.Fatal("expected error for nil object")
	}
}

func TestObjectTitle_EmptyDescriptionWithID(t *testing.T) {
	obj := &adapter.ChannelsObject{
		ID: "only_id_123",
		ObjectDesc: adapter.ChannelsObjectDesc{
			Description: "   ",
		},
	}
	got := adapter.ObjectTitle(obj)
	if got != "only_id_123" {
		t.Errorf("ObjectTitle = %q, want \"only_id_123\"", got)
	}
}

func TestPickSpec_NoMediaSpecFallsBackToObjectSpec(t *testing.T) {
	obj := makeVideoObject()
	obj.Spec = []adapter.ChannelsMediaSpec{
		{FileFormat: "h265", Width: 3840},
	}
	got := adapter.PickSpec(obj)
	if got != "h265" {
		t.Errorf("PickSpec(fallback to object spec) = %q, want \"h265\"", got)
	}
}

// Ensure imports are used
var _ = cmp.Diff
