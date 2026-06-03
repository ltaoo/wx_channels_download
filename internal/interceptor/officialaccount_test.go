package interceptor

import (
	"encoding/json"
	"testing"
)

func TestNewOfficialAccountArticleProfile(t *testing.T) {
	raw := json.RawMessage(`{
		"bizuin": "MzkzNDY0MzE1Nw==",
		"user_name": "gh_990683e05e57",
		"nick_name": "开智学堂",
		"round_head_img": "",
		"ori_head_img_url": "https://example.com/avatar/132",
		"hd_head_img": "https://example.com/avatar/0",
		"title": "阳志平：让 AI 替你自主干活的 12 个技巧",
		"link": "https://mp.weixin.qq.com/s/YGu-hw-DHKQx_lmeB3-ABA",
		"source_url": "https://j.youzan.com/JU1nMQ",
		"cdn_url": "https://example.com/cover.jpg",
		"mid": 2247554959,
		"idx": "1",
		"sn": "ec6c0e8f3b6b9bf2ead8761284c6ffaa"
	}`)

	profile, err := NewOfficialAccountArticleProfile(raw)
	if err != nil {
		t.Fatal(err)
	}
	if profile.UniqueMark != "MzkzNDY0MzE1Nw==_2247554959_1_ec6c0e8f3b6b9bf2ead8761284c6ffaa" {
		t.Fatalf("UniqueMark = %q", profile.UniqueMark)
	}
	if profile.AvatarURL != "https://example.com/avatar/132" {
		t.Fatalf("AvatarURL = %q", profile.AvatarURL)
	}
	if profile.Biz != "MzkzNDY0MzE1Nw==" || profile.Username != "gh_990683e05e57" {
		t.Fatalf("account = %#v", profile)
	}
	if string(profile.RawCgiDataNew) == "" {
		t.Fatal("RawCgiDataNew is empty")
	}
}

func TestNewOfficialAccountArticleProfileFillsIdsFromEscapedLink(t *testing.T) {
	raw := json.RawMessage(`{
		"user_name": "gh_4049dff7f346",
		"nick_name": "Sync-in",
		"title": "自托管文件同步与协作平台Sync-in",
		"link": "https://mp.weixin.qq.com/s?__biz=MzYyMjg1NjMxNw==&amp;mid=2247484054&amp;idx=1&amp;sn=2259da95d318c941aa1e4802fae4e0a3&amp;chksm=fed77f091eb9a487#rd"
	}`)

	profile, err := NewOfficialAccountArticleProfile(raw)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Biz != "MzYyMjg1NjMxNw==" {
		t.Fatalf("Biz = %q", profile.Biz)
	}
	if profile.UniqueMark != "MzYyMjg1NjMxNw==_2247484054_1_2259da95d318c941aa1e4802fae4e0a3" {
		t.Fatalf("UniqueMark = %q", profile.UniqueMark)
	}
	if profile.URL != "https://mp.weixin.qq.com/s?__biz=MzYyMjg1NjMxNw==&mid=2247484054&idx=1&sn=2259da95d318c941aa1e4802fae4e0a3&chksm=fed77f091eb9a487#rd" {
		t.Fatalf("URL = %q", profile.URL)
	}
}
