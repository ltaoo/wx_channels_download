package wxchannels

import (
	"encoding/json"
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

func TestBuildBrowseRecord_MediaVideo(t *testing.T) {
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
	r := BuildBrowseRecord(profile)

	expected := model.BrowseHistory{
		Id:                "wx_channels:feed_001",
		PlatformId:        "wx_channels",
		VisitedTimes:      1,
		AccountExternalId: "test_user",
		AccountUsername:   "test_user",
		AccountNickname:   "测试用户",
		AccountAvatarURL:  "https://img.example.com/avatar.jpg",
		ContentType:       "media",
		ContentExternalId: "feed_001",
		ContentTitle:      "测试视频",
		ContentURL:        "https://video.example.com/v.mp4",
		ContentSourceURL:  "https://channels.weixin.qq.com/web/pages/feed?oid=feed_001",
		ContentCoverURL:   "https://img.example.com/cover.jpg",
		ExtraData:         `{"decode_key":"decode_key_001","id":"feed_001","nonce_id":"nonce_001"}`,
		Timestamps:        r.Timestamps,
	}

	assertBrowseHistoryEqual(t, expected, *r)
}

func TestBuildBrowseRecord_TrimWhitespace(t *testing.T) {
	profile := &scraper.MediaProfile{
		Id: "feed_002",
		Contact: scraper.InterceptorContact{
			Id:        "  user_with_spaces  ",
			Nickname:  "空格用户",
			AvatarURL: "https://img.example.com/avatar2.jpg",
		},
	}
	r := BuildBrowseRecord(profile)

	if r.AccountExternalId != "user_with_spaces" {
		t.Errorf("AccountExternalId = %q, want %q", r.AccountExternalId, "user_with_spaces")
	}
	if r.AccountUsername != "user_with_spaces" {
		t.Errorf("AccountUsername = %q, want %q", r.AccountUsername, "user_with_spaces")
	}
}

func TestBuildBrowseRecord_ExtraDataDefaults(t *testing.T) {
	profile := &scraper.MediaProfile{
		Id:      "feed_003",
		Contact: scraper.InterceptorContact{Id: "user_003"},
	}
	r := BuildBrowseRecord(profile)

	expected := model.BrowseHistory{
		Id:                "wx_channels:feed_003",
		PlatformId:        "wx_channels",
		VisitedTimes:      1,
		AccountExternalId: "user_003",
		AccountUsername:   "user_003",
		AccountNickname:   "",
		AccountAvatarURL:  "",
		ContentType:       "",
		ContentExternalId: "feed_003",
		ContentTitle:      "",
		ContentURL:        "",
		ContentSourceURL:  "https://channels.weixin.qq.com/web/pages/feed?username=user_003",
		ContentCoverURL:   "",
		ExtraData:         `{"decode_key":"","id":"feed_003","nonce_id":""}`,
		Timestamps:        r.Timestamps,
	}

	assertBrowseHistoryEqual(t, expected, *r)
}

func assertBrowseHistoryEqual(t *testing.T, expected, actual model.BrowseHistory) {
	t.Helper()
	// Zero out Timestamps before comparing (they are set to time.Now)
	expectedJSON, _ := json.MarshalIndent(expected, "", "  ")
	actualJSON, _ := json.MarshalIndent(actual, "", "  ")
	if string(expectedJSON) != string(actualJSON) {
		t.Errorf("BrowseHistory mismatch:\n--- expected\n+++ actual\n%s\n%s",
			string(expectedJSON), string(actualJSON))
	}
}
