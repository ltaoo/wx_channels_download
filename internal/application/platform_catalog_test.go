package application

import (
	"context"
	"os"
	"testing"

	"wx_channel/internal/services"
)

func TestPlatformCatalogCoversScraperImplementations(t *testing.T) {
	entries, err := os.ReadDir("../../pkg/scraper")
	if err != nil {
		t.Fatal(err)
	}
	sources := make(map[string]bool)
	for _, platform := range services.ScraperPlatforms() {
		if platform.ID == "" || platform.Name == "" || !platform.Detail {
			t.Fatalf("invalid platform: %+v", platform)
		}
		sources[platform.Source] = true
	}
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != "internal" && !sources[entry.Name()] {
			t.Errorf("platform %s is missing from the catalog", entry.Name())
		}
	}
}

func TestPlatformFetchRejectsInvalidOperationsBeforeNetwork(t *testing.T) {
	for _, test_case := range []services.ScraperPlatformFetchRequest{
		{Platform: "unknown", Operation: "detail", URL: "https://www.douyin.com/video/123"},
		{Platform: "douyin", Operation: "unknown"},
		{Platform: "douyin", Operation: "detail"},
		{Platform: "douyin", Operation: "detail", URL: "https://www.youtube.com/watch?v=123"},
		{Platform: "douyin", Operation: "list"},
		{Platform: "x", Operation: "list", Profile: "username"},
		{Platform: "cctv", Operation: "list", Profile: "owner"},
	} {
		if _, err := services.FetchScraperPlatform(context.Background(), test_case); err == nil {
			t.Errorf("expected validation failure for %+v", test_case)
		}
	}
}

func TestNewlyEnabledNovelURLsSelectTheirImplementations(t *testing.T) {
	for _, test_case := range []struct {
		url      string
		platform string
	}{
		{"https://www.69shuba.com/book/34567.htm", "69shuba"},
		{"https://fanqienovel.com/page/123456", "fanqienovel"},
	} {
		resolution, err := services.ResolveScraperPlatform(test_case.url)
		if err != nil || resolution.Platform != test_case.platform {
			t.Fatalf("URL %s: resolution=%+v err=%v", test_case.url, resolution, err)
		}
	}
}
