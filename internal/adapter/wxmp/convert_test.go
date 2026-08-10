package wxmpadapter

import (
	"testing"

	"wx_channel/pkg/scraper/wxmp"
)

func TestFetchArticleLegacyDataConversion(t *testing.T) {
	legacy_data := &wxmp.CgiDataNew{
		UserName:        "account-id",
		NickName:        "测试公众号",
		Title:           "测试文章",
		ContentNoEncode: "<p>正文</p>",
		CdnUrl:          "https://example.com/cover.jpg",
		Link:            "https://mp.weixin.qq.com/s/example",
		SourceUrl:       "https://mp.weixin.qq.com/s/example",
		BizUin:          "biz-id",
		Mid:             wxmp.FlexibleInt(123),
		UserUin:         wxmp.FlexibleInt(456),
	}
	article := &wxmp.WechatOfficialArticle{PageJSON: legacy_data}
	adapter := NewOfficialAccountAdapter()

	content, err := adapter.ToContent(article)
	if err != nil {
		t.Fatalf("ToContent() error = %v", err)
	}
	if content.Title != legacy_data.Title || content.SourceURL != legacy_data.SourceUrl {
		t.Fatalf("ToContent() = %#v", content)
	}

	account, err := adapter.ToAccount(article)
	if err != nil {
		t.Fatalf("ToAccount() error = %v", err)
	}
	if account.ExternalId != legacy_data.UserName || account.Nickname != legacy_data.NickName {
		t.Fatalf("ToAccount() = %#v", account)
	}
}
