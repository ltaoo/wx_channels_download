package wxchannels

import (
	"testing"

	"wx_channel/internal/database/model"
)

func TestToAccount_Media(t *testing.T) {
	obj := &ChannelsObject{
		ID:            "feed123",
		ObjectNonceId: "nonce123",
		Type:          "media",
		CreateTime:    1700000000,
		Contact: ChannelsContact{
			Username: "test_user",
			Nickname: "测试用户",
			HeadUrl:  "https://example.com/avatar.jpg",
		},
		ObjectDesc: ChannelsObjectDesc{
			Description: "测试视频",
			MediaType:   4,
			Media: []ChannelsMediaItem{
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
	got, err := obj.ToAccount()
	if err != nil {
		t.Fatalf("ToAccount: %v", err)
	}
	want := &model.Account{
		Id:         "wx_channels:test_user",
		PlatformId: "wx_channels",
		ExternalId: "test_user",
		Username:   "test_user",
		Nickname:   "测试用户",
		AvatarURL:  "https://example.com/avatar.jpg",
	}
	if got.Id != want.Id {
		t.Errorf("Id = %q, want %q", got.Id, want.Id)
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
	obj := &ChannelsObject{
		ID:            "live_feed",
		ObjectNonceId: "live_nonce",
		Type:          "video",
		Contact: ChannelsContact{
			Username: "streamer",
			Nickname: "主播昵称",
			HeadUrl:  "https://example.com/streamer_avatar.jpg",
		},
		AnchorContact: &ChannelsContact{
			Username: "anchor_user",
			Nickname: "主播",
			HeadUrl:  "https://example.com/anchor_avatar.jpg",
		},
		LiveInfo: &ChannelsLiveInfo{
			AnchorStatusFlag: "live",
		},
	}
	got, err := obj.ToAccount()
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
	var obj *ChannelsObject
	_, err := obj.ToAccount()
	if err == nil {
		t.Fatal("expected error for nil object, got nil")
	}
}

func TestToContent_Media(t *testing.T) {
	obj := &ChannelsObject{
		ID:            "feed123",
		ObjectNonceId: "nonce123",
		SourceURL:     "https://channels.weixin.qq.com/web/pages/feed?oid=feed123",
		Type:          "media",
		CreateTime:    1701234567,
		Contact: ChannelsContact{
			Username: "test_user",
			Nickname: "测试用户",
			HeadUrl:  "https://example.com/avatar.jpg",
		},
		ObjectDesc: ChannelsObjectDesc{
			Description: "测试视频",
			MediaType:   4,
			Media: []ChannelsMediaItem{
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
	got, err := obj.ToContent()
	if err != nil {
		t.Fatalf("ToContent: %v", err)
	}
	if got.Id != "wx_channels:feed123" {
		t.Errorf("Id = %q, want %q", got.Id, "wx_channels:feed123")
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
	if got.ContentURL != "https://video.example.com/v.mp4?token=t" {
		t.Errorf("ContentURL = %q", got.ContentURL)
	}
	if got.URL != "https://video.example.com/v.mp4?token=t" {
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
	obj := &ChannelsObject{
		ID:            "live_feed",
		ObjectNonceId: "live_nonce",
		Type:          "video",
		CreateTime:    1711234567,
		Contact: ChannelsContact{
			Username: "streamer",
			Nickname: "主播昵称",
			HeadUrl:  "https://example.com/streamer_avatar.jpg",
		},
		AnchorContact: &ChannelsContact{
			Username:    "anchor_user",
			Nickname:    "主播",
			HeadUrl:     "https://example.com/anchor_avatar.jpg",
			CoverImgUrl: "https://example.com/live_cover.jpg",
		},
		LiveInfo: &ChannelsLiveInfo{AnchorStatusFlag: "live"},
	}
	got, err := obj.ToContent()
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
	obj := &ChannelsObject{
		ID:            "pic_feed",
		ObjectNonceId: "pic_nonce",
		SourceURL:     "https://channels.weixin.qq.com/web/pages/feed?oid=pic_feed",
		Type:          "picture",
		CreateTime:    1700000000,
		Contact: ChannelsContact{
			Username: "pic_author",
			Nickname: "图片作者",
			HeadUrl:  "https://example.com/pic_avatar.jpg",
		},
		Files: []ChannelsMediaItem{
			{CoverUrl: "https://example.com/pic_cover.jpg", Width: 1280, Height: 720},
		},
		ObjectDesc: ChannelsObjectDesc{
			Description: "一组美图",
			MediaType:   2,
		},
	}
	got, err := obj.ToContent()
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
	obj := &ChannelsObject{
		ID:            "feed_replay",
		ObjectNonceId: "nonce_replay",
		Type:          "video",
		ObjectDesc: ChannelsObjectDesc{
			MediaType: 9,
			Media:     []ChannelsMediaItem{{URL: "https://example.com/replay.mp4"}},
		},
	}
	_, err := obj.ToContent()
	if err == nil {
		t.Fatal("expected error for mediaType=9, got nil")
	}
}
