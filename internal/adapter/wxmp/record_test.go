package wxmp

import (
	"testing"

	scraper "wx_channel/pkg/scraper/wxmp"
)

func TestBuildBrowseRecord(t *testing.T) {
	record := BuildBrowseRecord(&scraper.OfficialAccountArticleProfile{
		UniqueMark: "biz_123_1_signature",
		Biz:        "biz",
		Username:   "gh_example",
		Nickname:   "示例公众号",
		Title:      "文章标题",
		URL:        "https://mp.weixin.qq.com/s?mid=123",
	})
	if record == nil {
		t.Fatal("BuildBrowseRecord returned nil")
	}
	if record.PlatformId != PlatformID || record.ContentExternalId != "biz_123_1_signature" {
		t.Fatalf("record = %#v", record)
	}
	if record.AccountExternalId != "biz" || record.CreatedAt == 0 {
		t.Fatalf("record = %#v", record)
	}
}

func TestBuildBrowseRecordRejectsEmptyProfile(t *testing.T) {
	if got := BuildBrowseRecord(nil); got != nil {
		t.Fatalf("BuildBrowseRecord(nil) = %#v", got)
	}
}
