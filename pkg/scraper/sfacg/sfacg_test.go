package sfacg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseBookProfileFixture(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "scraper_examples", "sfacg", "260619", "book.html"))
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("sfacg fixture not present")
		}
		t.Fatal(err)
	}
	profile, err := ParseBookProfile("https://book.sfacg.com/Novel/672419/", body)
	if err != nil {
		t.Fatalf("ParseBookProfile: %v", err)
	}
	if profile.Title != "被货车撞成了女孩子" || profile.Author.Name != "若是能重来" {
		t.Fatalf("profile title/author = %#v", profile)
	}
	if profile.Category != "都市" || profile.WordCount != "8175字" || profile.Status != "连载中" {
		t.Fatalf("category/words/status = %q %q %q", profile.Category, profile.WordCount, profile.Status)
	}
	if !strings.Contains(profile.Description, "美少女") || len(profile.Tags) == 0 {
		t.Fatalf("description/tags = %q %#v", profile.Description, profile.Tags)
	}
	if profile.ChapterCount != 1 || !strings.Contains(profile.LatestChapter.Title, "第四章") {
		t.Fatalf("chapter = %#v count=%d", profile.LatestChapter, profile.ChapterCount)
	}
}

func TestParseURL(t *testing.T) {
	for _, rawURL := range []string{"https://book.sfacg.com/Novel/672419/", "672419"} {
		parts, ok := ParseURL(rawURL)
		if !ok || parts.BookID != "672419" || parts.Canonical != "https://book.sfacg.com/Novel/672419/" {
			t.Fatalf("ParseURL(%q) = %#v, %v", rawURL, parts, ok)
		}
	}
}
