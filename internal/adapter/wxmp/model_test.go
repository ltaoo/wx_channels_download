package wxmpadapter

import (
	"encoding/json"
	"testing"

	"wx_channel/pkg/scraper/wxmp"
)

func TestArticleExternalID(t *testing.T) {
	data := &wxmp.ArticleCgiDataNew{
		Bizuin: "MzExample==",
		Mid:    2247506133,
		Idx:    2,
	}

	got := ArticleExternalID(data)
	want := "MzExample==_2247506133_2"
	if got != want {
		t.Fatalf("ArticleExternalID() = %q, want %q", got, want)
	}
}

func TestArticleExternalIDRequiresArticleCoordinates(t *testing.T) {
	tests := []struct {
		name string
		data *wxmp.ArticleCgiDataNew
	}{
		{name: "nil", data: nil},
		{name: "missing biz", data: &wxmp.ArticleCgiDataNew{Mid: 1, Idx: 1}},
		{name: "missing mid", data: &wxmp.ArticleCgiDataNew{Bizuin: "biz", Idx: 1}},
		{name: "missing idx", data: &wxmp.ArticleCgiDataNew{Bizuin: "biz", Mid: 1}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ArticleExternalID(test.data); got != "" {
				t.Fatalf("ArticleExternalID() = %q, want empty string", got)
			}
		})
	}
}

func TestBuildDownloadTaskAcceptsNumericUserUin(t *testing.T) {
	content := json.RawMessage(`{
		"user_name": "biz_user",
		"user_uin": 1234567890,
		"nick_name": "公众号作者",
		"title": "公众号标题",
		"desc": "公众号摘要",
		"content_noencode": "<p>正文内容</p>",
		"cdn_url": "https://mmbiz.qpic.cn/cover.jpg",
		"link": "https://mp.weixin.qq.com/s/k_F-1KYn-EPy27W9VoKZng",
		"ori_create_time": 1700000000,
		"bizuin": "239001",
		"mid": 2247483666,
		"idx": 1
	}`)

	info, err := NewOfficialAccountAdapter().BuildDownloadTask(content, json.RawMessage(`{"download_dir":"/tmp","filename":"article.html"}`))
	if err != nil {
		t.Fatalf("BuildDownloadTask() error = %v", err)
	}
	if info == nil || info.Task == nil || info.Content == nil {
		t.Fatalf("BuildDownloadTask() returned incomplete info: %+v", info)
	}
	if info.Task.PlatformId != PlatformID || info.Content.ExternalId != "239001_2247483666_1" {
		t.Fatalf("unexpected task/content: task=%+v content=%+v", info.Task, info.Content)
	}
}
