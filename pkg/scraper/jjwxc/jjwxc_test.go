package jjwxc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseBookProfileFixture(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "scraper_examples", "jjwxc", "260619", "book.html"))
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("jjwxc fixture not present")
		}
		t.Fatal(err)
	}
	profile, err := ParseBookProfile("https://m.jjwxc.net/book2/245452", body)
	if err != nil {
		t.Fatalf("ParseBookProfile: %v", err)
	}
	if profile.Title != "夜醉君度" || profile.Author.Name != "差劲" {
		t.Fatalf("profile title/author = %#v", profile)
	}
	if !strings.Contains(profile.Description, "BL小说") {
		t.Fatalf("description = %q", profile.Description)
	}
	if !strings.Contains(profile.Category, "原创-纯爱") || !strings.Contains(profile.Status, "完结") || profile.WordCount != "5181字" {
		t.Fatalf("category/status/words = %q %q %q", profile.Category, profile.Status, profile.WordCount)
	}
	if profile.ChapterCount != 5 || len(profile.Volumes) != 1 {
		t.Fatalf("chapters = %#v", profile.Volumes)
	}
	if !strings.Contains(profile.Volumes[0].Chapters[0].Title, "3月23日") {
		t.Fatalf("first chapter = %#v", profile.Volumes[0].Chapters[0])
	}
}

func TestParseURL(t *testing.T) {
	cases := map[string]string{
		"https://www.jjwxc.net/onebook.php?novelid=245452": "245452",
		"https://m.jjwxc.net/book2/245452":                 "245452",
		"245452":                                           "245452",
	}
	for rawURL, bookID := range cases {
		parts, ok := ParseURL(rawURL)
		if !ok || parts.BookID != bookID || parts.Canonical != "https://m.jjwxc.net/book2/"+bookID {
			t.Fatalf("ParseURL(%q) = %#v, %v", rawURL, parts, ok)
		}
	}
}
