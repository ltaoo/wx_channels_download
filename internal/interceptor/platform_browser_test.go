package interceptor

import "testing"

func TestNewPlatformBrowserProfile(t *testing.T) {
	raw := []byte(`{
		"platform_id": "zhihu",
		"platform_name": "知乎",
		"content_type": "article",
		"content_title": "示例文章",
		"content_url": "https://www.zhihu.com/question/1/answer/2#hash",
		"content_source_url": "https://www.zhihu.com/question/1/answer/2?utm=1",
		"content_cover_url": "https://example.com/cover.jpg",
		"account_external_id": "https://www.zhihu.com/people/demo",
		"account_nickname": "知乎用户"
	}`)

	profile, err := NewPlatformBrowserProfile(raw)
	if err != nil {
		t.Fatal(err)
	}
	if profile.PlatformId != "zhihu" {
		t.Fatalf("PlatformId = %q", profile.PlatformId)
	}
	if profile.ContentExternalId != "https://www.zhihu.com/question/1/answer/2#hash" {
		t.Fatalf("ContentExternalId = %q", profile.ContentExternalId)
	}
	if profile.AccountExternalId != "https://www.zhihu.com/people/demo" {
		t.Fatalf("AccountExternalId = %q", profile.AccountExternalId)
	}
	if string(profile.Raw) == "" {
		t.Fatal("Raw is empty")
	}
}
