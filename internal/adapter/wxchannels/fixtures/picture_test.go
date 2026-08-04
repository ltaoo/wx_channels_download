package wxchannels_test

import (
	"encoding/json"
	"testing"

	wxchannels "wx_channel/internal/adapter/wxchannels"
	example "wx_channel/scraper_examples"
)

func TestToAccount_FromPictureFeedJSON(t *testing.T) {
	raw, err := example.Load("wxchannels/260604/picture.json")
	if err != nil {
		t.Fatalf("load picture fixture: %v", err)
	}

	var obj wxchannels.ChannelsObject
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	account, err := wxchannels.ToAccount(&obj)
	if err != nil {
		t.Fatalf("ToAccount: %v", err)
	}

	if account.ExternalId != "v2_060000231003b20faec8c5eb8c1dc3d5cc02ec34b0777fde89138921d31325e189f6ac106494@finder" {
		t.Errorf("ExternalId = %q", account.ExternalId)
	}
	if account.Id != "wxchannels:v2_060000231003b20faec8c5eb8c1dc3d5cc02ec34b0777fde89138921d31325e189f6ac106494@finder" {
		t.Errorf("Id = %q", account.Id)
	}
	if account.Nickname != "锦妆阁汉服旅拍" {
		t.Errorf("Nickname = %q, want %q", account.Nickname, "锦妆阁汉服旅拍")
	}
	if account.Signature != "📍杭州西湖国贸中心424 锦妆阁\n️19560433888\n🫘锦妆阁官方号（杭州西湖店）\n🍠锦妆阁汉服旅拍（杭州西湖店）\n预约可加V\n其他暂未入驻～" {
		t.Errorf("Signature = %q", account.Signature)
	}
	if account.PlatformId != "wxchannels" {
		t.Errorf("PlatformId = %q", account.PlatformId)
	}
}

func TestToContent_FromPictureFeedJSON(t *testing.T) {
	raw, err := example.Load("wxchannels/260604/picture.json")
	if err != nil {
		t.Fatalf("load picture fixture: %v", err)
	}

	var obj wxchannels.ChannelsObject
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	content, _, err := wxchannels.ToContent(&obj)
	if err != nil {
		t.Fatalf("ToContent: %v", err)
	}

	if content.Type != "picture" {
		t.Errorf("ContentType = %q, want %q", content.Type, "picture")
	}
	if content.Title != "锦鲤上岸，事事皆如意！#锦鲤#锦妆阁汉服旅拍#杭州#锦妆阁#汉服" {
		t.Errorf("Title = %q", content.Title)
	}
	if content.Description != "锦鲤上岸，事事皆如意！#锦鲤#锦妆阁汉服旅拍#杭州#锦妆阁#汉服" {
		t.Errorf("Description = %q", content.Description)
	}
	if content.ExternalId != "14936583787284727824" {
		t.Errorf("ExternalId = %q", content.ExternalId)
	}
	if content.ExternalId2 != "8189655580179332358_0_146_0_0" {
		t.Errorf("ExternalId2 = %q, want %q", content.ExternalId2, "8189655580179332358_0_146_0_0")
	}
	if content.ExternalId3 != "" {
		t.Errorf("ExternalId3 = %q, want empty", content.ExternalId3)
	}
	if content.Id != "wxchannels:14936583787284727824" {
		t.Errorf("Id = %q", content.Id)
	}
	if content.URL != "" {
		t.Errorf("URL = %q, want empty", content.URL)
	}
	if content.CoverURL != "" {
		t.Errorf("CoverURL = %q, want empty (picture media has empty coverUrl)", content.CoverURL)
	}
	if content.CoverWidth != "954" {
		t.Errorf("CoverWidth = %q, want %q", content.CoverWidth, "954")
	}
	if content.CoverHeight != "636" {
		t.Errorf("CoverHeight = %q, want %q", content.CoverHeight, "636")
	}
	if content.SourceURL != "" {
		t.Errorf("SourceURL = %q, want empty", content.SourceURL)
	}
	if content.PublishTime == nil || *content.PublishTime != 1780579541 {
		t.Errorf("PublishTime = %v, want ptr to 1780579541", content.PublishTime)
	}
}
