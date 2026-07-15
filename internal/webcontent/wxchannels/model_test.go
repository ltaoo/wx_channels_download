package wxchannels

import (
	"testing"

	"wx_channel/internal/database/model"
	scraper "wx_channel/pkg/scraper/wxchannels"
)

func TestToAccount_Media(t *testing.T) {
	obj := &scraper.ChannelsObject{
		ID:            "feed123",
		ObjectNonceId: "nonce123",
		Type:          "media",
		CreateTime:    1700000000,
		Contact: scraper.ChannelsContact{
			Username: "test_user",
			Nickname: "测试用户",
			HeadUrl:  "https://example.com/avatar.jpg",
		},
		ObjectDesc: scraper.ChannelsObjectDesc{
			Description: "测试视频",
			MediaType:   4,
			Media: []scraper.ChannelsMediaItem{
				{
					URL:          "https://video.example.com/v.mp4",
					URLToken:     "&token=t",
					CoverUrl:     "https://example.com/cover.jpg",
					DecodeKey:    "key123",
					VideoPlayLen: 10,
					FileSize:     100,
					Width:        1920,
					Height:       1080,
				},
			},
		},
	}
	got, err := ToAccount(obj)
	if err != nil {
		t.Fatalf("ToAccount: %v", err)
	}
	want := &model.Account{
		PlatformId: "wx_channels",
		ExternalId: "test_user",
		Username:   "test_user",
		Nickname:   "测试用户",
		AvatarURL:  "https://example.com/avatar.jpg",
	}
	if got.PlatformId != want.PlatformId {
		t.Errorf("PlatformId = %q, want %q", got.PlatformId, want.PlatformId)
	}
	if got.ExternalId != want.ExternalId {
		t.Errorf("ExternalId = %q, want %q", got.ExternalId, want.ExternalId)
	}
	if got.Username != want.Username {
		t.Errorf("Username = %q, want %q", got.Username, want.Username)
	}
	if got.Nickname != want.Nickname {
		t.Errorf("Nickname = %q, want %q", got.Nickname, want.Nickname)
	}
	if got.AvatarURL != want.AvatarURL {
		t.Errorf("AvatarURL = %q, want %q", got.AvatarURL, want.AvatarURL)
	}
	if got.CreatedAt == 0 {
		t.Error("CreatedAt should not be 0")
	}
	if got.UpdatedAt == 0 {
		t.Error("UpdatedAt should not be 0")
	}
}

func TestToAccount_Live(t *testing.T) {
	obj := &scraper.ChannelsObject{
		ID:            "live_feed",
		ObjectNonceId: "live_nonce",
		Type:          "video",
		Contact: scraper.ChannelsContact{
			Username: "streamer",
			Nickname: "主播昵称",
			HeadUrl:  "https://example.com/streamer_avatar.jpg",
		},
		AnchorContact: &scraper.ChannelsContact{
			Username: "anchor_user",
			Nickname: "主播",
			HeadUrl:  "https://example.com/anchor_avatar.jpg",
		},
		LiveInfo: &scraper.ChannelsLiveInfo{
			AnchorStatusFlag: "live",
		},
	}
	got, err := ToAccount(obj)
	if err != nil {
		t.Fatalf("ToAccount live: %v", err)
	}
	if got.ExternalId != "anchor_user" {
		t.Errorf("ExternalId = %q, want %q", got.ExternalId, "anchor_user")
	}
	if got.Nickname != "主播" {
		t.Errorf("Nickname = %q, want %q", got.Nickname, "主播")
	}
}

func TestToAccount_NilObject(t *testing.T) {
	_, err := ToAccount(nil)
	if err == nil {
		t.Fatal("expected error for nil object, got nil")
	}
}

func TestToContent_Media(t *testing.T) {
	obj := &scraper.ChannelsObject{
		ID:            "feed123",
		ObjectNonceId: "nonce123",
		SourceURL:     "https://channels.weixin.qq.com/web/pages/feed?oid=feed123",
		Type:          "media",
		CreateTime:    1701234567,
		Contact: scraper.ChannelsContact{
			Username: "test_user",
			Nickname: "测试用户",
			HeadUrl:  "https://example.com/avatar.jpg",
		},
		ObjectDesc: scraper.ChannelsObjectDesc{
			Description: "测试视频",
			MediaType:   4,
			Media: []scraper.ChannelsMediaItem{
				{
					URL:          "https://video.example.com/v.mp4",
					URLToken:     "&token=t",
					CoverUrl:     "https://example.com/cover.jpg",
					DecodeKey:    "key123",
					VideoPlayLen: 10,
					FileSize:     100,
					Width:        1920,
					Height:       1080,
				},
			},
		},
	}
	got, err := ToContent(obj)
	if err != nil {
		t.Fatalf("ToContent: %v", err)
	}
	if got.PlatformId != "wx_channels" {
		t.Errorf("PlatformId = %q", got.PlatformId)
	}
	if got.ContentType != "video" {
		t.Errorf("ContentType = %q, want %q", got.ContentType, "video")
	}
	if got.Title != "测试视频" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.Description != "测试视频" {
		t.Errorf("Description = %q", got.Description)
	}
	if got.ExternalId != "feed123" {
		t.Errorf("ExternalId = %q", got.ExternalId)
	}
	if got.ExternalId2 != "nonce123" {
		t.Errorf("ExternalId2 = %q", got.ExternalId2)
	}
	if got.ExternalId3 != "key123" {
		t.Errorf("ExternalId3 = %q", got.ExternalId3)
	}
	if got.SourceURL != "https://channels.weixin.qq.com/web/pages/feed?oid=feed123" {
		t.Errorf("SourceURL = %q", got.SourceURL)
	}
	if got.ContentURL != "https://video.example.com/v.mp4&token=t" {
		t.Errorf("ContentURL = %q", got.ContentURL)
	}
	if got.URL != "https://video.example.com/v.mp4&token=t" {
		t.Errorf("URL = %q", got.URL)
	}
	if got.CoverURL != "https://example.com/cover.jpg" {
		t.Errorf("CoverURL = %q", got.CoverURL)
	}
	if got.CoverWidth != "1920" {
		t.Errorf("CoverWidth = %q", got.CoverWidth)
	}
	if got.CoverHeight != "1080" {
		t.Errorf("CoverHeight = %q", got.CoverHeight)
	}
	if got.Duration != 10 {
		t.Errorf("Duration = %d", got.Duration)
	}
	if got.Size != 100 {
		t.Errorf("Size = %d", got.Size)
	}
	if got.PublishTime == nil || *got.PublishTime != 1701234567 {
		t.Errorf("PublishTime = %v", got.PublishTime)
	}
	if got.Metadata != `{"key":"key123"}` {
		t.Errorf("Metadata = %q", got.Metadata)
	}
	if got.CreatedAt == 0 {
		t.Error("CreatedAt should not be 0")
	}
	if got.UpdatedAt == 0 {
		t.Error("UpdatedAt should not be 0")
	}
}

func TestToContent_Live(t *testing.T) {
	obj := &scraper.ChannelsObject{
		ID:            "live_feed",
		ObjectNonceId: "live_nonce",
		Type:          "video",
		CreateTime:    1711234567,
		Contact: scraper.ChannelsContact{
			Username: "streamer",
			Nickname: "主播昵称",
			HeadUrl:  "https://example.com/streamer_avatar.jpg",
		},
		AnchorContact: &scraper.ChannelsContact{
			Username:    "anchor_user",
			Nickname:    "主播",
			HeadUrl:     "https://example.com/anchor_avatar.jpg",
			CoverImgUrl: "https://example.com/live_cover.jpg",
		},
		LiveInfo: &scraper.ChannelsLiveInfo{AnchorStatusFlag: "live"},
	}
	got, err := ToContent(obj)
	if err != nil {
		t.Fatalf("ToContent live: %v", err)
	}
	if got.ContentType != "live" {
		t.Errorf("ContentType = %q, want %q", got.ContentType, "live")
	}
	if got.Title != "直播" {
		t.Errorf("Title = %q, want %q", got.Title, "直播")
	}
	if got.CoverURL != "https://example.com/live_cover.jpg" {
		t.Errorf("CoverURL = %q", got.CoverURL)
	}
	if got.ContentURL != "" {
		t.Errorf("ContentURL = %q, want empty", got.ContentURL)
	}
	if got.URL != "" {
		t.Errorf("URL = %q, want empty", got.URL)
	}
}

func TestToContent_Picture(t *testing.T) {
	obj := &scraper.ChannelsObject{
		ID:            "pic_feed",
		ObjectNonceId: "pic_nonce",
		SourceURL:     "https://channels.weixin.qq.com/web/pages/feed?oid=pic_feed",
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
	got, err := ToContent(obj)
	if err != nil {
		t.Fatalf("ToContent picture: %v", err)
	}
	if got.ContentType != "picture" {
		t.Errorf("ContentType = %q, want %q", got.ContentType, "picture")
	}
	if got.ContentURL != "" {
		t.Errorf("ContentURL = %q, want empty", got.ContentURL)
	}
	if got.URL != "" {
		t.Errorf("URL = %q, want empty", got.URL)
	}
	if got.CoverURL != "https://example.com/pic_cover.jpg" {
		t.Errorf("CoverURL = %q", got.CoverURL)
	}
}

func TestToContent_MediaType9(t *testing.T) {
	obj := &scraper.ChannelsObject{
		ID:            "feed_replay",
		ObjectNonceId: "nonce_replay",
		Type:          "video",
		ObjectDesc: scraper.ChannelsObjectDesc{
			MediaType: 9,
			Media:     []scraper.ChannelsMediaItem{{URL: "https://example.com/replay.mp4"}},
		},
	}
	_, err := ToContent(obj)
	if err == nil {
		t.Fatal("expected error for mediaType=9, got nil")
	}
}

func TestCreateBrowseRecord_MediaVideo(t *testing.T) {
	profile := &scraper.MediaProfile{
		Type:    "media",
		Id:      "feed_001",
		NonceId: "nonce_001",
		Title:   "测试视频",
		URL:     "https://video.example.com/v.mp4",
		Key:     "decode_key_001",
		CoverURL: "https://img.example.com/cover.jpg",
		Pageurl:  "https://channels.weixin.qq.com/web/pages/feed?oid=feed_001",
		Contact: scraper.InterceptorContact{
			Id:        "test_user",
			Nickname:  "测试用户",
			AvatarURL: "https://img.example.com/avatar.jpg",
		},
	}
	uniqueMark, info := CreateBrowseRecord(profile)

	if uniqueMark != "feed_001" {
		t.Errorf("uniqueMark = %q, want %q", uniqueMark, "feed_001")
	}
	if info.PlatformId != "wx_channels" {
		t.Errorf("PlatformId = %q, want %q", info.PlatformId, "wx_channels")
	}
	if info.AccountExternalId != "test_user" {
		t.Errorf("AccountExternalId = %q, want %q", info.AccountExternalId, "test_user")
	}
	if info.AccountUsername != "test_user" {
		t.Errorf("AccountUsername = %q, want %q", info.AccountUsername, "test_user")
	}
	if info.AccountNickname != "测试用户" {
		t.Errorf("AccountNickname = %q, want %q", info.AccountNickname, "测试用户")
	}
	if info.AccountAvatarURL != "https://img.example.com/avatar.jpg" {
		t.Errorf("AccountAvatarURL = %q", info.AccountAvatarURL)
	}
	if info.ContentType != "media" {
		t.Errorf("ContentType = %q, want %q", info.ContentType, "media")
	}
	if info.ContentTitle != "测试视频" {
		t.Errorf("ContentTitle = %q", info.ContentTitle)
	}
	if info.ContentURL != "https://video.example.com/v.mp4" {
		t.Errorf("ContentURL = %q", info.ContentURL)
	}
	if info.ContentSourceURL != "https://channels.weixin.qq.com/web/pages/feed?oid=feed_001" {
		t.Errorf("ContentSourceURL = %q", info.ContentSourceURL)
	}
	if info.ContentCoverURL != "https://img.example.com/cover.jpg" {
		t.Errorf("ContentCoverURL = %q", info.ContentCoverURL)
	}
	if info.ExtraData["id"] != "feed_001" {
		t.Errorf("ExtraData[id] = %v", info.ExtraData["id"])
	}
	if info.ExtraData["nonce_id"] != "nonce_001" {
		t.Errorf("ExtraData[nonce_id] = %v", info.ExtraData["nonce_id"])
	}
	if info.ExtraData["decode_key"] != "decode_key_001" {
		t.Errorf("ExtraData[decode_key] = %v", info.ExtraData["decode_key"])
	}
}

func TestCreateBrowseRecord_TrimWhitespace(t *testing.T) {
	profile := &scraper.MediaProfile{
		Id: "feed_002",
		Contact: scraper.InterceptorContact{
			Id:        "  user_with_spaces  ",
			Nickname:  "空格用户",
			AvatarURL: "https://img.example.com/avatar2.jpg",
		},
	}
	_, info := CreateBrowseRecord(profile)

	if info.AccountExternalId != "user_with_spaces" {
		t.Errorf("AccountExternalId = %q, want %q", info.AccountExternalId, "user_with_spaces")
	}
	if info.AccountUsername != "user_with_spaces" {
		t.Errorf("AccountUsername = %q, want %q", info.AccountUsername, "user_with_spaces")
	}
}

func TestCreateBrowseRecord_ExtraDataDefaults(t *testing.T) {
	profile := &scraper.MediaProfile{
		Id:      "feed_003",
		Contact: scraper.InterceptorContact{Id: "user_003"},
	}
	uniqueMark, info := CreateBrowseRecord(profile)

	if uniqueMark != "feed_003" {
		t.Errorf("uniqueMark = %q", uniqueMark)
	}
	if info.PlatformId != "wx_channels" {
		t.Errorf("PlatformId = %q", info.PlatformId)
	}
	// Default zero values should be present
	if info.ContentType != "" {
		t.Errorf("ContentType = %q, want empty", info.ContentType)
	}
	if info.ExtraData["nonce_id"] != "" {
		t.Errorf("ExtraData[nonce_id] = %v, want empty", info.ExtraData["nonce_id"])
	}
	if info.ExtraData["decode_key"] != "" {
		t.Errorf("ExtraData[decode_key] = %v, want empty", info.ExtraData["decode_key"])
	}
}
