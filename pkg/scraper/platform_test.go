package scraper

import (
	"strings"
	"testing"
)

func TestLookupDefaultAuthorAndHomepage(t *testing.T) {
	platform, ok := Lookup("YOUKU")
	if !ok {
		t.Fatal("expected youku platform")
	}
	if platform.ID != PlatformIDYouku || DefaultAuthor("youku") != "youku" {
		t.Fatalf("unexpected youku platform: %#v", platform)
	}
	if HomepageURL("youku") != "https://www.youku.com/" {
		t.Fatalf("youku homepage = %q", HomepageURL("youku"))
	}
}

func TestLookupAlias(t *testing.T) {
	platform, ok := Lookup("xhs")
	if !ok {
		t.Fatal("expected xhs alias")
	}
	if platform.ID != PlatformIDXiaohongshu || DisplayName("rednote") != "小红书" {
		t.Fatalf("unexpected xiaohongshu alias: %#v", platform)
	}
	platform, ok = Lookup("wx_official_account")
	if !ok {
		t.Fatal("expected wx_official_account alias")
	}
	if platform.ID != PlatformIDOfficialAccount || DisplayName("official_account") != "公众号" {
		t.Fatalf("unexpected official account alias: %#v", platform)
	}
}

func TestDisplayNamePreservesTikTok(t *testing.T) {
	if DisplayName("tiktok") != "TikTok" {
		t.Fatalf("tiktok display name = %q", DisplayName("tiktok"))
	}
}

func TestUnknownPlatformDefaultsToInput(t *testing.T) {
	if DefaultAuthor(" custom ") != "custom" {
		t.Fatalf("default author = %q", DefaultAuthor(" custom "))
	}
	if DisplayName(" custom ") != "custom" {
		t.Fatalf("display name = %q", DisplayName(" custom "))
	}
	if HomepageURL("custom") != "" {
		t.Fatalf("homepage = %q", HomepageURL("custom"))
	}
}

func TestFaviconDataURL(t *testing.T) {
	if FaviconBase64("zhihu") == "" {
		t.Fatal("expected zhihu favicon")
	}
	if got := FaviconDataURL("wx_official_account"); got == "" || !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Fatalf("official account favicon data URL = %q", got)
	}
	if got := FaviconDataURL("fanqienovel"); got == "" || !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Fatalf("fanqienovel favicon data URL = %q", got)
	}
	if FaviconDataURL("custom") != "" {
		t.Fatalf("custom favicon = %q", FaviconDataURL("custom"))
	}
}
