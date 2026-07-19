package wxchannels_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	scraper "wx_channel/pkg/scraper/wxchannels"
	webcontent "wx_channel/internal/webcontent/wxchannels"
)

func TestParseFeedURL(t *testing.T) {
	parts, err := scraper.ParseFeedURL("https://channels.weixin.qq.com/web/pages/feed?oid=z0Qii_kLCBA&nid=2-dNcmWxXdc&eid=eid123")
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
	parts, err := scraper.ParseSphShareURL("https://weixin.qq.com/sph/AoPX5bEBDd")
	if err != nil {
		t.Fatalf("ParseSphShareURL weixin: %v", err)
	}
	if parts.ID != "AoPX5bEBDd" {
		t.Fatalf("ID = %q", parts.ID)
	}

	parts, err = scraper.ParseSphShareURL("https://channels.weixin.qq.com/finder-preview/pages/sph?id=AoPX5bEBDd")
	if err != nil {
		t.Fatalf("ParseSphShareURL finder-preview: %v", err)
	}
	if parts.ID != "AoPX5bEBDd" {
		t.Fatalf("ID = %q", parts.ID)
	}
}

func makeVideoObject() *scraper.ChannelsObject {
	return &scraper.ChannelsObject{
		ID:            "feed123",
		ObjectNonceId: "nonce123",
		SourceURL:     "https://channels.weixin.qq.com/web/pages/feed?oid=feed123",
		Type:          "media",
		Contact: scraper.ChannelsContact{
			Username: "author",
			Nickname: "作者",
			HeadUrl:  "https://image.example.com/avatar.jpg",
		},
		ObjectDesc: scraper.ChannelsObjectDesc{
			Description: "测试视频",
			MediaType:   4,
			Media: []scraper.ChannelsMediaItem{
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
	got := webcontent.PickSpec(obj)
	if got != "original" {
		t.Errorf("PickSpec(no spec) = %q, want \"original\"", got)
	}
}

func TestPickSpec_FromMedia(t *testing.T) {
	obj := makeVideoObject()
	obj.ObjectDesc.Media[0].Spec = []scraper.ChannelsMediaSpec{
		{FileFormat: "h264", Width: 1920, Height: 1080},
	}
	got := webcontent.PickSpec(obj)
	if got != "h264" {
		t.Errorf("PickSpec(media spec) = %q, want \"h264\"", got)
	}
}

func TestPickSpec_FromObject(t *testing.T) {
	obj := makeVideoObject()
	obj.Spec = []scraper.ChannelsMediaSpec{
		{FileFormat: "mp4", Width: 1920, Height: 1080},
	}
	got := webcontent.PickSpec(obj)
	if got != "mp4" {
		t.Errorf("PickSpec(object spec) = %q, want \"mp4\"", got)
	}
}

func TestDecryptKeyInt(t *testing.T) {
	obj := makeVideoObject()
	got := webcontent.DecryptKeyInt(obj)
	if got != 0 {
		t.Errorf("DecryptKeyInt(non-numeric) = %d, want 0", got)
	}

	obj.ObjectDesc.Media[0].DecodeKey = "42"
	got = webcontent.DecryptKeyInt(obj)
	if got != 42 {
		t.Errorf("DecryptKeyInt(42) = %d, want 42", got)
	}
}

func TestDecryptKeyInt_NoMedia(t *testing.T) {
	obj := &scraper.ChannelsObject{ID: "test"}
	got := webcontent.DecryptKeyInt(obj)
	if got != 0 {
		t.Errorf("DecryptKeyInt(no media) = %d, want 0", got)
	}
}

func TestObjectTitle_WithDescription(t *testing.T) {
	obj := makeVideoObject()
	got := webcontent.ObjectTitle(obj)
	if got != "测试视频" {
		t.Errorf("ObjectTitle = %q, want \"测试视频\"", got)
	}
}

func TestObjectTitle_EmptyDescription(t *testing.T) {
	obj := makeVideoObject()
	obj.ObjectDesc.Description = ""
	got := webcontent.ObjectTitle(obj)
	if got != "feed123" {
		t.Errorf("ObjectTitle(no desc) = %q, want \"feed123\"", got)
	}
}

func TestObjectTitle_EmptyDescriptionAndID(t *testing.T) {
	obj := makeVideoObject()
	obj.ObjectDesc.Description = ""
	obj.ID = ""
	got := webcontent.ObjectTitle(obj)
	if got == "" {
		t.Error("ObjectTitle should not be empty (should fallback to timestamp)")
	}
}

func TestObjectTitle_Live(t *testing.T) {
	obj := makeVideoObject()
	obj.LiveInfo = &scraper.ChannelsLiveInfo{AnchorStatusFlag: "live"}
	got := webcontent.ObjectTitle(obj)
	if got != "直播" {
		t.Errorf("ObjectTitle(live) = %q, want \"直播\"", got)
	}
}

func TestObjectURL_Video(t *testing.T) {
	obj := makeVideoObject()
	got := webcontent.ObjectURL(obj)
	if got != "https://video.example.com/video.mp4?encfilekey=filekey&token=token" {
		t.Errorf("ObjectURL = %q", got)
	}
}

func TestObjectURL_Live(t *testing.T) {
	obj := makeVideoObject()
	obj.LiveInfo = &scraper.ChannelsLiveInfo{AnchorStatusFlag: "live"}
	got := webcontent.ObjectURL(obj)
	if got != "" {
		t.Errorf("ObjectURL(live) = %q, want \"\"", got)
	}
}

func TestObjectURL_Picture(t *testing.T) {
	obj := makeVideoObject()
	obj.Type = "picture"
	got := webcontent.ObjectURL(obj)
	if got != "" {
		t.Errorf("ObjectURL(picture) = %q, want \"\"", got)
	}
}

func TestBuildJumpURLFromParts_WithSourceURL(t *testing.T) {
	got := webcontent.BuildJumpURLFromParts("oid123", "nid456", "https://channels.weixin.qq.com/web/pages/feed?oid=abc", "user")
	if got != "https://channels.weixin.qq.com/web/pages/feed?oid=abc" {
		t.Errorf("BuildJumpURLFromParts(sourceURL) = %q", got)
	}
}

func TestBuildJumpURLFromParts_WithUsername(t *testing.T) {
	got := webcontent.BuildJumpURLFromParts("feed_jump_001", "", "", "test_user")
	if got != "https://channels.weixin.qq.com/web/pages/feed?username=test_user" {
		t.Errorf("BuildJumpURLFromParts(username) = %q", got)
	}
}

func TestBuildJumpURLFromParts_NilLike(t *testing.T) {
	got := webcontent.BuildJumpURLFromParts("", "", "", "")
	if got != "https://channels.weixin.qq.com/web/pages/feed" {
		t.Errorf("BuildJumpURLFromParts(empty) = %q", got)
	}
}

func TestToContent_Video(t *testing.T) {
	obj := makeVideoObject()
	got, err := webcontent.ToContent(obj)
	if err != nil {
		t.Fatalf("ToContent: %v", err)
	}
	if got.ExternalId != "feed123" {
		t.Errorf("ExternalId = %q", got.ExternalId)
	}
	if got.ContentType != "video" {
		t.Errorf("ContentType = %q", got.ContentType)
	}
	if got.ContentURL != "https://video.example.com/video.mp4?encfilekey=filekey&token=token" {
		t.Errorf("ContentURL = %q", got.ContentURL)
	}
	if got.Duration != 5 {
		t.Errorf("Duration = %d", got.Duration)
	}
	if got.Size != 100 {
		t.Errorf("Size = %d", got.Size)
	}
}

func TestToContent_Nil(t *testing.T) {
	_, err := webcontent.ToContent(nil)
	if err == nil {
		t.Fatal("expected error for nil object")
	}
}

func TestToContent_EmptyID(t *testing.T) {
	_, err := webcontent.ToContent(&scraper.ChannelsObject{})
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestToContent_MediaType9(t *testing.T) {
	obj := makeVideoObject()
	obj.ObjectDesc.MediaType = 9
	_, err := webcontent.ToContent(obj)
	if err == nil {
		t.Fatal("expected error for mediaType=9")
	}
}

func TestToContent_Live(t *testing.T) {
	obj := makeVideoObject()
	obj.LiveInfo = &scraper.ChannelsLiveInfo{AnchorStatusFlag: "live"}
	got, err := webcontent.ToContent(obj)
	if err != nil {
		t.Fatalf("ToContent(live): %v", err)
	}
	if got.ContentType != "live" {
		t.Errorf("ContentType = %q, want \"live\"", got.ContentType)
	}
	if got.Title != "直播" {
		t.Errorf("Title = %q, want \"直播\"", got.Title)
	}
}

func TestToContent_Picture(t *testing.T) {
	obj := &scraper.ChannelsObject{
		ID:            "pic_feed_001",
		ObjectNonceId: "pic_nonce_001",
		SourceURL:     "https://channels.weixin.qq.com/web/pages/feed?oid=pic_feed_001",
		Type:          "picture",
		CreateTime:    1700000000,
		Contact: scraper.ChannelsContact{
			Username: "pic_author",
			Nickname: "图片作者",
			HeadUrl:  "https://example.com/pic_avatar.jpg",
		},
		Files: []scraper.ChannelsMediaItem{
			{CoverUrl: "https://example.com/pic_cover.jpg", Width: 1280, Height: 720},
		},
		ObjectDesc: scraper.ChannelsObjectDesc{
			Description: "一组美图",
			MediaType:   2,
		},
	}
	got, err := webcontent.ToContent(obj)
	if err != nil {
		t.Fatalf("ToContent(picture): %v", err)
	}
	if got.ContentType != "picture" {
		t.Errorf("ContentType = %q, want \"picture\"", got.ContentType)
	}
	if got.CoverURL != "https://example.com/pic_cover.jpg" {
		t.Errorf("CoverURL = %q", got.CoverURL)
	}
}

func TestToContent_LiveAnchorContactFallback(t *testing.T) {
	obj := &scraper.ChannelsObject{
		ID:            "live_no_anchor",
		ObjectNonceId: "nonce_live_no_anchor",
		SourceURL:     "https://channels.weixin.qq.com/web/pages/feed?oid=live_no_anchor",
		Contact: scraper.ChannelsContact{
			Username: "fallback_user",
			Nickname: "回退主播",
			HeadUrl:  "https://example.com/fallback_avatar.jpg",
		},
		ObjectDesc: scraper.ChannelsObjectDesc{
			Description: "直播回退测试",
			Media:       []scraper.ChannelsMediaItem{{CoverUrl: "https://example.com/media_cover.jpg"}},
		},
		LiveInfo: &scraper.ChannelsLiveInfo{AnchorStatusFlag: "live"},
	}
	content, err := webcontent.ToContent(obj)
	if err != nil {
		t.Fatalf("ToContent(live fallback): %v", err)
	}
	if content.CoverURL != "https://example.com/media_cover.jpg" {
		t.Errorf("CoverURL = %q", content.CoverURL)
	}
	account, err := webcontent.ToAccount(obj)
	if err != nil {
		t.Fatalf("ToAccount(live fallback): %v", err)
	}
	if account.ExternalId != "fallback_user" {
		t.Errorf("Account ExternalId = %q", account.ExternalId)
	}
}

func TestToContent_FullConversion(t *testing.T) {
	obj := &scraper.ChannelsObject{
		ID:            "14885057406549363320",
		ObjectNonceId: "nonce_full_test",
		SourceURL:     "https://channels.weixin.qq.com/web/pages/feed?oid=14885057406549363320&nid=nonce_full_test",
		Type:          "media",
		CreateTime:    1701234567,
		Contact: scraper.ChannelsContact{
			Username: "v2_060000231003b20f@finder",
			Nickname: "测试视频号主",
			HeadUrl:  "https://example.com/real_avatar.jpg",
		},
		ObjectDesc: scraper.ChannelsObjectDesc{
			Description: "这是一条真实的测试视频",
			MediaType:   4,
			Media: []scraper.ChannelsMediaItem{
				{
					URL:          "https://finder.video.qq.com/251/20302/stodownload?encfilekey=actual_key",
					URLToken:     "&token=actual_token",
					CoverUrl:     "https://finder.video.qq.com/251/cover.jpg",
					DecodeKey:    "actual_decode_key",
					VideoPlayLen: 125,
					FileSize:     8388608,
					Width:        1920,
					Height:       1080,
					Spec: []scraper.ChannelsMediaSpec{
						{FileFormat: "mp4", Width: 1920, Height: 1080, DurationMs: 125000},
						{FileFormat: "mp4", Width: 1280, Height: 720, DurationMs: 125000},
					},
				},
			},
		},
	}

	got, err := webcontent.ToContent(obj)
	if err != nil {
		t.Fatalf("ToContent(full): %v", err)
	}

	wantExternalId := "14885057406549363320"
	wantURL := "https://finder.video.qq.com/251/20302/stodownload?encfilekey=actual_key&token=actual_token"
	if got.ExternalId != wantExternalId {
		t.Errorf("ExternalId = %q, want %q", got.ExternalId, wantExternalId)
	}
	if got.ContentURL != wantURL {
		t.Errorf("ContentURL = %q, want %q", got.ContentURL, wantURL)
	}
	if got.Duration != 125 {
		t.Errorf("Duration = %d, want 125", got.Duration)
	}
	if got.Size != 8388608 {
		t.Errorf("Size = %d, want 8388608", got.Size)
	}

	// Verify ID building
	if id := webcontent.BuildContentID(got.ExternalId); id != "wx_channels:14885057406549363320" {
		t.Errorf("BuildContentID = %q", id)
	}
}

func TestToAccount(t *testing.T) {
	obj := makeVideoObject()
	got, err := webcontent.ToAccount(obj)
	if err != nil {
		t.Fatalf("ToAccount: %v", err)
	}
	if got.ExternalId != "author" {
		t.Errorf("ExternalId = %q, want \"author\"", got.ExternalId)
	}
	if got.Nickname != "作者" {
		t.Errorf("Nickname = %q, want \"作者\"", got.Nickname)
	}
	if id := webcontent.BuildAccountID(got.ExternalId); id != "wx_channels:author" {
		t.Errorf("BuildAccountID = %q, want \"wx_channels:author\"", id)
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
		got := webcontent.BuildContentID(tt.externalID)
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
		got := webcontent.BuildAccountID(tt.externalID)
		if got != tt.want {
			t.Errorf("BuildAccountID(%q) = %q, want %q", tt.externalID, got, tt.want)
		}
	}
}

func TestPlatformID(t *testing.T) {
	if webcontent.PlatformID != "wx_channels" {
		t.Errorf("PlatformID = %q, want \"wx_channels\"", webcontent.PlatformID)
	}
}

func TestObjectTitle_LiveAnchorContactFallback(t *testing.T) {
	obj := &scraper.ChannelsObject{
		ID:            "live_no_anchor",
		ObjectNonceId: "nonce_live_no_anchor",
		LiveInfo:      &scraper.ChannelsLiveInfo{AnchorStatusFlag: "live"},
	}
	got := webcontent.ObjectTitle(obj)
	if got != "直播" {
		t.Errorf("ObjectTitle(live fallback) = %q, want \"直播\"", got)
	}
}

func TestPickSpec_MediaSpecsVsObjectSpecs(t *testing.T) {
	obj := makeVideoObject()
	obj.ObjectDesc.Media[0].Spec = []scraper.ChannelsMediaSpec{
		{FileFormat: "h264", Width: 1920},
	}
	obj.Spec = []scraper.ChannelsMediaSpec{
		{FileFormat: "h265", Width: 3840},
	}
	// Media specs should take precedence
	got := webcontent.PickSpec(obj)
	if got != "h264" {
		t.Errorf("PickSpec = %q, want \"h264\" (media spec takes priority)", got)
	}
}

func TestObjectURL_NoMedia(t *testing.T) {
	obj := &scraper.ChannelsObject{ID: "test", Type: "media"}
	got := webcontent.ObjectURL(obj)
	if got != "" {
		t.Errorf("ObjectURL(no media) = %q, want \"\"", got)
	}
}

func TestBuildJumpURLFromParts_WithObjectIdAndNonceId(t *testing.T) {
	tests := []struct {
		name     string
		objectId string
		nonceId  string
		sourceURL string
		username string
		want     string
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
			name: "sourceURL overrides",
			objectId: "oid123",
			sourceURL: "https://channels.weixin.qq.com/web/pages/feed?oid=abc",
			want:     "https://channels.weixin.qq.com/web/pages/feed?oid=abc",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := webcontent.BuildJumpURLFromParts(tt.objectId, tt.nonceId, tt.sourceURL, tt.username)
			if got != tt.want {
				t.Errorf("got  %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestToContent_PictureNoFiles(t *testing.T) {
	obj := &scraper.ChannelsObject{
		ID:            "pic_no_files",
		ObjectNonceId: "nonce_no_files",
		Type:          "picture",
		ObjectDesc:    scraper.ChannelsObjectDesc{MediaType: 2},
	}
	_, err := webcontent.ToContent(obj)
	if err == nil {
		t.Fatal("expected error for picture without files")
	}
}

func TestToContent_MediaNoMedia(t *testing.T) {
	obj := &scraper.ChannelsObject{
		ID:            "media_no_data",
		ObjectNonceId: "nonce_no_data",
		Type:          "media",
		ObjectDesc:    scraper.ChannelsObjectDesc{MediaType: 4},
	}
	_, err := webcontent.ToContent(obj)
	if err == nil {
		t.Fatal("expected error for media without media data")
	}
}

func TestToAccount_Nil(t *testing.T) {
	_, err := webcontent.ToAccount(nil)
	if err == nil {
		t.Fatal("expected error for nil object")
	}
}

func TestObjectTitle_EmptyDescriptionWithID(t *testing.T) {
	obj := &scraper.ChannelsObject{
		ID: "only_id_123",
		ObjectDesc: scraper.ChannelsObjectDesc{
			Description: "   ",
		},
	}
	got := webcontent.ObjectTitle(obj)
	if got != "only_id_123" {
		t.Errorf("ObjectTitle = %q, want \"only_id_123\"", got)
	}
}

func TestPickSpec_NoMediaSpecFallsBackToObjectSpec(t *testing.T) {
	obj := makeVideoObject()
	obj.Spec = []scraper.ChannelsMediaSpec{
		{FileFormat: "h265", Width: 3840},
	}
	got := webcontent.PickSpec(obj)
	if got != "h265" {
		t.Errorf("PickSpec(fallback to object spec) = %q, want \"h265\"", got)
	}
}

// Ensure imports are used
var _ = cmp.Diff
