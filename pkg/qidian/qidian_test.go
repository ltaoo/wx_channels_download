package qidian

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePageContextFixture(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "qidian_260614.html"))
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("qidian_260614.html fixture not present")
		}
		t.Fatal(err)
	}
	raw, err := ExtractPageContextJSON(body)
	if err != nil {
		t.Fatalf("ExtractPageContextJSON: %v", err)
	}
	if !json.Valid(raw) {
		t.Fatal("page context json is invalid")
	}
	root, err := ParsePageContext(body)
	if err != nil {
		t.Fatalf("ParsePageContext: %v", err)
	}
	bookInfo := root.PageContext.PageProps.PageData.BookInfo
	if bookInfo.BookID != 1035420986 || bookInfo.BookName != "玄鉴仙族" {
		t.Fatalf("bookInfo = %#v", bookInfo)
	}
	if bookInfo.AuthorName != "季越人" {
		t.Fatalf("author = %q", bookInfo.AuthorName)
	}
}

func TestParseBookProfileFromPageContextFixture(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "qidian_260614.html"))
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("qidian_260614.html fixture not present")
		}
		t.Fatal(err)
	}
	profile, err := ParseBookProfile("https://www.qidian.com/book/1035420986/", body)
	if err != nil {
		t.Fatalf("ParseBookProfile: %v", err)
	}
	if profile.Title != "玄鉴仙族" || profile.Author.Name != "季越人" {
		t.Fatalf("profile = %#v", profile)
	}
	if profile.ChapterCount != 1578 {
		t.Fatalf("chapter count = %d", profile.ChapterCount)
	}
	if profile.LatestChapter.Title != "第一千五百一十一章退走（1+1/2） 潜龙勿用加更52/115" {
		t.Fatalf("latest chapter = %#v", profile.LatestChapter)
	}
	if len(profile.Volumes) == 0 || len(profile.Volumes[0].Chapters) == 0 {
		t.Fatalf("volumes = %#v", profile.Volumes)
	}
	if !strings.Contains(profile.Description, "陆江仙熬夜猝死") {
		t.Fatalf("description = %q", profile.Description)
	}
	if len(profile.PageContextJSON) == 0 || profile.PageHTML == "" {
		t.Fatal("expected raw page context and html")
	}
}

func TestParseURL(t *testing.T) {
	got, ok := ParseURL("https://m.qidian.com/book/1035420986/")
	if !ok {
		t.Fatal("ParseURL returned false")
	}
	if got.BookID != "1035420986" || got.Canonical != "https://www.qidian.com/book/1035420986/" {
		t.Fatalf("ParseURL = %#v", got)
	}
}
