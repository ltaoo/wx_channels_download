package services

import "testing"

func TestResolveScraperPlatformMatchesFeishuWikiAndDocx(t *testing.T) {
	cases := map[string]string{
		"https://mayfairtech.larkenterprise.com/wiki/LxaDwNLkVizzU0k6hQtc8FI5nZc": scraper_platform_feishu,
		"https://example.feishu.cn/docx/ABC123":                                   scraper_platform_feishu,
		"https://mayfairtech.larkenterprise.com/drive/home/":                      scraper_platform_singlefile,
	}
	for raw_url, want_platform := range cases {
		resolution, err := ResolveScraperPlatform(raw_url)
		if err != nil {
			t.Fatalf("resolve %s: %v", raw_url, err)
		}
		if resolution.Platform != want_platform {
			t.Fatalf("resolve %s = %s, want %s", raw_url, resolution.Platform, want_platform)
		}
	}
}
