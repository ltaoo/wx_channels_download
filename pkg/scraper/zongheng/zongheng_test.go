package zongheng

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseBookProfileFixture(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "scraper_examples", "zongheng", "260619", "book.html"))
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("zongheng fixture not present")
		}
		t.Fatal(err)
	}
	profile, err := ParseBookProfile("https://www.zongheng.com/detail/672340", body)
	if err != nil {
		t.Fatalf("ParseBookProfile: %v", err)
	}
	if profile.Title != "剑来" || profile.Author.Name != "烽火戏诸侯" {
		t.Fatalf("profile title/author = %#v", profile)
	}
	if !strings.Contains(profile.Description, "陈平安") {
		t.Fatalf("description = %q", profile.Description)
	}
	if !strings.Contains(profile.LatestChapter.Title, "百花深处") {
		t.Fatalf("latest chapter = %#v", profile.LatestChapter)
	}
	if profile.ChapterCount == 0 || len(profile.Volumes) == 0 {
		t.Fatalf("chapters = %#v", profile.Volumes)
	}
}

func TestParseURL(t *testing.T) {
	for _, rawURL := range []string{
		"https://book.zongheng.com/book/672340.html",
		"https://www.zongheng.com/detail/672340",
		"https://m.zongheng.com/book/672340",
		"672340",
	} {
		parts, ok := ParseURL(rawURL)
		if !ok || parts.BookID != "672340" || parts.Canonical != "https://www.zongheng.com/detail/672340" {
			t.Fatalf("ParseURL(%q) = %#v, %v", rawURL, parts, ok)
		}
	}
}
