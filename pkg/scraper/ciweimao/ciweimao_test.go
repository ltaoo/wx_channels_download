package ciweimao

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseBookProfileFixture(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "scraper_examples", "ciweimao", "260619", "book.html"))
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("ciweimao fixture not present")
		}
		t.Fatal(err)
	}
	profile, err := ParseBookProfile("https://www.ciweimao.com/book/100337734", body)
	if err != nil {
		t.Fatalf("ParseBookProfile: %v", err)
	}
	if profile.Title != "人在璃月，开局被刻晴关进大牢" || profile.Author.Name != "已断开连接" {
		t.Fatalf("profile title/author = %#v", profile)
	}
	if profile.Category != "游戏竞技" || profile.WordCount != "105805" || !strings.Contains(profile.Status, "连载中") {
		t.Fatalf("category/words/status = %q %q %q", profile.Category, profile.WordCount, profile.Status)
	}
	if !strings.Contains(profile.Description, "刻晴") {
		t.Fatalf("description = %q", profile.Description)
	}
	if profile.ChapterCount < 20 || len(profile.Volumes) != 1 || profile.Volumes[0].Title != "第一卷" {
		t.Fatalf("volumes = %#v", profile.Volumes)
	}
	if !strings.Contains(profile.LatestChapter.Title, "第五十章") {
		t.Fatalf("latest chapter = %#v", profile.LatestChapter)
	}
}

func TestParseURL(t *testing.T) {
	for _, rawURL := range []string{"https://www.ciweimao.com/book/100337734", "https://wap.ciweimao.com/book/100337734", "100337734"} {
		parts, ok := ParseURL(rawURL)
		if !ok || parts.BookID != "100337734" || parts.Canonical != "https://www.ciweimao.com/book/100337734" {
			t.Fatalf("ParseURL(%q) = %#v, %v", rawURL, parts, ok)
		}
	}
}
