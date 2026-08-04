package wxmp

import (
	"testing"

	"wx_channel/internal/database/model"
	scraper "wx_channel/pkg/scraper/wxmp"
)

func TestArticleConversions(t *testing.T) {
	data := &scraper.ArticleCgiDataNew{
		Bizuin:          "biz-id",
		Mid:             123,
		Idx:             1,
		CommentID:       "comment-id",
		UserName:        "gh_example",
		NickName:        "示例公众号",
		Signature:       "示例签名",
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

	content, ext, err := ToContent(data)
	if err != nil {
		t.Fatalf("ToContent: %v", err)
	}
	_ = ext
	if content.Id != "wxmp:biz-id" || content.PlatformId != PlatformID {
		t.Fatalf("content identity = (%q, %q)", content.Id, content.PlatformId)
	}
	if content.Type != "article" || content.ExternalId2 != "123" {
		t.Fatalf("content = %#v", content)
	}

	account, err := ToAccount(data)
	if err != nil {
		t.Fatalf("ToAccount: %v", err)
	}
	if account.Id != "wxmp:gh_example" {
		t.Fatalf("account = %#v", account)
	}
	if account.Signature != "示例签名" {
		t.Fatalf("account signature = %q", account.Signature)
	}

	history, err := ArticleToHistory(data)
	if err != nil {
		t.Fatalf("ArticleToHistory: %v", err)
	}
	if history.PlatformId != PlatformID || history.ExternalId != "biz-id" {
		t.Fatalf("history = %#v", history)
	}

	article, err := ArticleToContentArticle(data)
	if err != nil {
		t.Fatalf("ArticleToContentArticle: %v", err)
	}
	if article.Id != content.Id || article.Type != model.ContentArticleTypeHTML || article.HTML != "<p>正文</p>" || article.WordCount != 2 {
		t.Fatalf("article = %#v", article)
	}
}

func TestArticleExternalIDRejectsMissingIdentity(t *testing.T) {
	if got := ArticleExternalID(&scraper.ArticleCgiDataNew{Bizuin: ""}); got != "" {
		t.Fatalf("ArticleExternalID = %q, want empty", got)
	}
	if _, _, err := ToContent(&scraper.ArticleCgiDataNew{Bizuin: ""}); err == nil {
		t.Fatal("ToContent accepted incomplete article identity")
	}
}
