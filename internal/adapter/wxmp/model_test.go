package wxmp

import (
	"testing"

	scraper "wx_channel/pkg/scraper/wxmp"
)

func TestArticleConversions(t *testing.T) {
	data := &scraper.ArticleCgiData{
		Bizuin:          "biz-id",
		Mid:             123,
		Idx:             1,
		CommentID:       "comment-id",
		UserName:        "gh_example",
		NickName:        "示例公众号",
		RoundHeadImg:    "https://example.com/avatar.jpg",
		Title:           "文章标题",
		Desc:            "文章摘要",
		Link:            "https://mp.weixin.qq.com/s?__biz=biz-id",
		SourceURL:       "https://mp.weixin.qq.com/s?__biz=biz-id",
		CdnURL:          "https://example.com/cover.jpg",
		OriCreateTime:   1700000000,
		ContentNoencode: "<p>正文</p>",
		Author:          "作者",
	}

	content, err := ToContent(data)
	if err != nil {
		t.Fatalf("ToContent: %v", err)
	}
	if content.Id != "wxmp:biz-id_123_1" || content.PlatformId != PlatformID {
		t.Fatalf("content identity = (%q, %q)", content.Id, content.PlatformId)
	}
	if content.ContentType != "article" || content.ExternalId2 != "comment-id" {
		t.Fatalf("content = %#v", content)
	}

	account, err := ToAccount(data)
	if err != nil {
		t.Fatalf("ToAccount: %v", err)
	}
	if account.Id != "wxmp:biz-id" || account.Username != "gh_example" {
		t.Fatalf("account = %#v", account)
	}

	history, err := ArticleToHistory(data)
	if err != nil {
		t.Fatalf("ArticleToHistory: %v", err)
	}
	if history.PlatformId != PlatformID || history.ContentExternalId != "biz-id_123_1" {
		t.Fatalf("history = %#v", history)
	}

	article, err := ArticleToContentArticle(data)
	if err != nil {
		t.Fatalf("ArticleToContentArticle: %v", err)
	}
	if article.ContentId != content.Id || article.AuthorName != "作者" {
		t.Fatalf("article = %#v", article)
	}
}

func TestArticleExternalIDRejectsMissingIdentity(t *testing.T) {
	if got := ArticleExternalID(&scraper.ArticleCgiData{Bizuin: "biz", Mid: 1}); got != "" {
		t.Fatalf("ArticleExternalID = %q, want empty", got)
	}
	if _, err := ToContent(&scraper.ArticleCgiData{Bizuin: "biz", Mid: 1}); err == nil {
		t.Fatal("ToContent accepted incomplete article identity")
	}
}
