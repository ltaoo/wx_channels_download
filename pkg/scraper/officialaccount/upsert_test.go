package officialaccount

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	officialaccountdownload "wx_channel/pkg/officialaccount"
)

func TestArticleToProfile_NormalArticle(t *testing.T) {
	// article 级别的字段为空时，由 PageJSON 提供 fallback
	article := &WechatOfficialArticle{
		Type:           1,
		Content:        "<p>这是一篇测试文章的内容</p>",
		ContentLength:  1024,
		PublishTimeStr: "2025-01-01 10:00",
		Images:         []string{"https://mmbiz.qpic.cn/img1.jpg", "https://mmbiz.qpic.cn/img2.jpg"},
		PageJSON: &officialaccountdownload.CgiDataNew{
			UserName:      "gh_1234567890ab",
			NickName:      "官方昵称",
			RoundHeadImg:  "https://mmbiz.qpic.cn/round_head.jpg",
			OriHeadImgUrl: "https://mmbiz.qpic.cn/ori_head.jpg",
			Title:         "PageJSON标题",
			Desc:          "文章摘要",
			CdnUrl:        "https://mmbiz.qpic.cn/cover.jpg",
			OriCreateTime: 1704067200,
			HdHeadImg:     "https://mmbiz.qpic.cn/hd_head.jpg",
		},
	}

	sourceURL := "https://mp.weixin.qq.com/s?__biz=MzAwMDAwMA==&mid=1234567890&idx=1&sn=abcdef123456"

	got, err := ArticleToProfile(article, sourceURL)
	if err != nil {
		t.Fatalf("ArticleToProfile failed: %v", err)
	}

	// ArticleID 是从 sourceURL 动态提取的，只验证非空
	if got.ArticleID == "" {
		t.Error("ArticleID should not be empty")
	}

	want := &ArticleProfile{
		ArticleID:   got.ArticleID, // 动态值
		Title:       "PageJSON标题",  // article.Title 为空时 fallback 到 PageJSON.Title
		Description: "文章摘要",        // PageJSON.Desc
		SourceURL:   sourceURL,
		CoverURL:    "https://mmbiz.qpic.cn/cover.jpg", // PageJSON.CdnUrl 优先于 Images[0]
		ContentHTML: article.Content,
		ContentSize: 1024,
		PublishTime: 1704067200,
		Author: ArticleAuthor{
			ExternalId: "gh_1234567890ab",                       // PageJSON.UserName
			Nickname:   "官方昵称",                                   // PageJSON.NickName
			AvatarURL:  "https://mmbiz.qpic.cn/round_head.jpg",   // RoundHeadImg > OriHeadImgUrl > HdHeadImg
		},
	}
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("ArticleToProfile mismatch (-want +got):\n%s", diff)
	}
}

func TestArticleToProfile_NilArticle(t *testing.T) {
	_, err := ArticleToProfile(nil, "https://mp.weixin.qq.com/s/test")
	if err == nil {
		t.Fatal("expected error for nil article")
	}
}

func TestArticleToProfile_TitleFallback(t *testing.T) {
	// 没有 PageJSON 时，从 article.Title 获取标题
	article := &WechatOfficialArticle{
		Title:          "直接标题",
		AuthorNickname: "作者A",
		AuthorAvatar:   "https://example.com/avatar.jpg",
		AuthorID:       "gh_test",
		Images:         []string{"https://example.com/cover.jpg"},
	}
	got, err := ArticleToProfile(article, "https://mp.weixin.qq.com/s?__biz=MzTest&mid=1&idx=1&sn=test")
	if err != nil {
		t.Fatalf("ArticleToProfile failed: %v", err)
	}
	want := &ArticleProfile{
		ArticleID:   got.ArticleID,
		Title:       "直接标题",
		SourceURL:   "https://mp.weixin.qq.com/s?__biz=MzTest&mid=1&idx=1&sn=test",
		CoverURL:    "https://example.com/cover.jpg", // Images[0] 作为封面
		Author: ArticleAuthor{
			ExternalId: "gh_test",
			Nickname:   "作者A",
			AvatarURL:  "https://example.com/avatar.jpg",
		},
	}
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("title fallback mismatch (-want +got):\n%s", diff)
	}
}

func TestArticleToProfile_TitleFallbackToArticleID(t *testing.T) {
	// 没有 PageJSON 且没有 Title 时，使用 articleID 作为标题
	article := &WechatOfficialArticle{
		AuthorNickname: "作者B",
		Images:         []string{},
	}
	sourceURL := "https://mp.weixin.qq.com/s?__biz=MzTest&mid=1&idx=1&sn=test"
	got, err := ArticleToProfile(article, sourceURL)
	if err != nil {
		t.Fatalf("ArticleToProfile failed: %v", err)
	}
	if got.Title != got.ArticleID {
		t.Errorf("Title = %q, want %q (articleID)", got.Title, got.ArticleID)
	}
}

func TestArticleToProfile_AuthorFromPageJSON(t *testing.T) {
	// article 级别作者字段全部为空，PageJSON 提供所有作者信息
	article := &WechatOfficialArticle{
		Title:  "测试文章",
		Images: []string{},
		PageJSON: &officialaccountdownload.CgiDataNew{
			UserName:      "gh_pagejson_level",
			NickName:      "pagejson_nickname",
			RoundHeadImg:  "https://example.com/pagejson_round.jpg",
			OriHeadImgUrl: "https://example.com/pagejson_ori.jpg",
			HdHeadImg:     "https://example.com/pagejson_hd.jpg",
			Author:        "pagejson_author",
		},
	}
	got, err := ArticleToProfile(article, "https://mp.weixin.qq.com/s?__biz=MzTest&mid=2&idx=1&sn=test2")
	if err != nil {
		t.Fatalf("ArticleToProfile failed: %v", err)
	}
	want := &ArticleProfile{
		ArticleID: got.ArticleID,
		Title:     "测试文章",
		SourceURL: "https://mp.weixin.qq.com/s?__biz=MzTest&mid=2&idx=1&sn=test2",
		Author: ArticleAuthor{
			ExternalId: "gh_pagejson_level",                   // PageJSON.UserName
			Nickname:   "pagejson_nickname",                    // PageJSON.NickName
			AvatarURL:  "https://example.com/pagejson_round.jpg", // RoundHeadImg
		},
	}
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("author from PageJSON mismatch (-want +got):\n%s", diff)
	}
}

func TestArticleToProfile_CoverFallbackToImages(t *testing.T) {
	// PageJSON.CdnUrl 为空时，使用 Images[0] 作为封面
	article := &WechatOfficialArticle{
		Title:          "测试文章",
		AuthorNickname: "作者",
		AuthorID:       "gh_test",
		Images:         []string{"https://mmbiz.qpic.cn/img_first.jpg", "https://mmbiz.qpic.cn/img_second.jpg"},
		PageJSON:       &officialaccountdownload.CgiDataNew{CdnUrl: ""},
	}
	got, err := ArticleToProfile(article, "https://mp.weixin.qq.com/s?__biz=MzTest&mid=3&idx=1&sn=test3")
	if err != nil {
		t.Fatalf("ArticleToProfile failed: %v", err)
	}
	want := &ArticleProfile{
		ArticleID: got.ArticleID,
		Title:     "测试文章",
		SourceURL: "https://mp.weixin.qq.com/s?__biz=MzTest&mid=3&idx=1&sn=test3",
		CoverURL:  "https://mmbiz.qpic.cn/img_first.jpg",
		Author: ArticleAuthor{
			ExternalId: "gh_test",
			Nickname:   "作者",
		},
	}
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("cover fallback mismatch (-want +got):\n%s", diff)
	}
}

func TestArticleToProfile_ContentSizeFallback(t *testing.T) {
	// ContentLength 为 0 时，从 Content 字符串长度自动计算
	article := &WechatOfficialArticle{
		Title:          "测试",
		AuthorNickname: "作者",
		AuthorID:       "gh_test",
		Content:        "Hello, World! 这是一篇短文。",
		PageJSON: &officialaccountdownload.CgiDataNew{
			OriCreateTime: 1704067200,
		},
	}
	got, err := ArticleToProfile(article, "https://mp.weixin.qq.com/s?__biz=MzTest&mid=4&idx=1&sn=test4")
	if err != nil {
		t.Fatalf("ArticleToProfile failed: %v", err)
	}
	if got.ContentSize == 0 {
		t.Error("ContentSize should not be 0 when Content is not empty")
	}
}

func TestArticleToProfile_AuthorAvatarPriority(t *testing.T) {
	// 头像优先级: RoundHeadImg > OriHeadImgUrl > HdHeadImg
	article := &WechatOfficialArticle{
		Title:          "测试",
		AuthorNickname: "作者",
		AuthorID:       "gh_test",
		PageJSON: &officialaccountdownload.CgiDataNew{
			RoundHeadImg:  "",
			OriHeadImgUrl: "https://example.com/ori_head.jpg",
			HdHeadImg:     "https://example.com/hd_head.jpg",
		},
	}
	got, err := ArticleToProfile(article, "https://mp.weixin.qq.com/s?__biz=MzTest&mid=5&idx=1&sn=test5")
	if err != nil {
		t.Fatalf("ArticleToProfile failed: %v", err)
	}
	want := &ArticleProfile{
		ArticleID: got.ArticleID,
		Title:     "测试",
		SourceURL: "https://mp.weixin.qq.com/s?__biz=MzTest&mid=5&idx=1&sn=test5",
		Author: ArticleAuthor{
			ExternalId: "gh_test",
			Nickname:   "作者",
			AvatarURL:  "https://example.com/ori_head.jpg", // RoundHeadImg 为空，用 OriHeadImgUrl
		},
	}
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("avatar priority mismatch (-want +got):\n%s", diff)
	}
}

func TestArticleToProfile_EmptyAuthorID(t *testing.T) {
	// 只有 biz 参数，没有 author
	sourceURL := "https://mp.weixin.qq.com/s?__biz=Mzg3MDYyMTIyNQ==&mid=123&idx=1&sn=abc"
	article := &WechatOfficialArticle{
		Title:    "测试文章",
		PageJSON: &officialaccountdownload.CgiDataNew{},
	}
	got, err := ArticleToProfile(article, sourceURL)
	if err != nil {
		t.Fatalf("ArticleToProfile failed: %v", err)
	}
	if got.ArticleID == "" {
		t.Error("ArticleID should not be empty even without author")
	}
}

func TestArticleToProfile_PublishTimeZero(t *testing.T) {
	article := &WechatOfficialArticle{
		Title:          "测试",
		AuthorNickname: "作者",
		AuthorID:       "gh_test",
		PageJSON:       &officialaccountdownload.CgiDataNew{OriCreateTime: 0},
	}
	got, err := ArticleToProfile(article, "https://mp.weixin.qq.com/s?__biz=MzTest&mid=7&idx=1&sn=test7")
	if err != nil {
		t.Fatalf("ArticleToProfile failed: %v", err)
	}
	want := &ArticleProfile{
		ArticleID:   got.ArticleID,
		Title:       "测试",
		SourceURL:   "https://mp.weixin.qq.com/s?__biz=MzTest&mid=7&idx=1&sn=test7",
		PublishTime: 0,
		Author: ArticleAuthor{
			ExternalId: "gh_test",
			Nickname:   "作者",
		},
	}
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("publish time zero mismatch (-want +got):\n%s", diff)
	}
}

func TestPlatformIDOfficialAccount(t *testing.T) {
	if platformIDOfficialAccount != "wx_official_account" {
		t.Errorf("platformIDOfficialAccount = %q, want \"wx_official_account\"", platformIDOfficialAccount)
	}
}
