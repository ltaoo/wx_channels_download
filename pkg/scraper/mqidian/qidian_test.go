package mqidian

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

func TestParseCatalogPageContextVolumes(t *testing.T) {
	body := []byte(`<script id="vite-plugin-ssr_pageContext" type="application/json">{"pageContext":{"pageProps":{"pageData":{"bookId":1035420986,"bookName":"玄鉴仙族","chapterTotalCnt":2,"bookStatus":"连载","actionStatus":"连载中","updateTime":"2026-06-18 20:06:58","authorInfo":{"authorId":430784443,"authorName":"季越人"},"vs":[{"vN":"作品相关","cCnt":1,"cs":[{"sS":1,"cnt":475,"cN":"感谢大佬们","id":753321428,"uuid":379,"uT":"2023-05-13 21:36:25"}]},{"vN":"溪里青玄","cCnt":1,"cs":[{"sS":0,"cnt":2039,"cN":"第一章 初入","id":730944635,"uuid":1,"uT":"2022-10-07 10:22:55"}]}]}},"urlPathname":"/book/1035420986/catalog/"}}</script>`)
	profile, err := ParseBookProfile("https://m.qidian.com/book/1035420986/catalog/", body)
	if err != nil {
		t.Fatalf("ParseBookProfile: %v", err)
	}
	if profile.ChapterCount != 2 || len(profile.Volumes) != 2 {
		t.Fatalf("catalog = %#v", profile)
	}
	if profile.Volumes[0].Title != "作品相关" || profile.Volumes[1].Chapters[0].Title != "第一章 初入" {
		t.Fatalf("volumes = %#v", profile.Volumes)
	}
	if profile.Volumes[1].Chapters[0].URL != "https://m.qidian.com/chapter/1035420986/730944635/" {
		t.Fatalf("chapter url = %q", profile.Volumes[1].Chapters[0].URL)
	}
	if !profile.Volumes[1].Chapters[0].Locked || profile.Volumes[1].Chapters[0].WordCount != 2039 {
		t.Fatalf("chapter metadata = %#v", profile.Volumes[1].Chapters[0])
	}
	if profile.LatestChapter.Title != "第一章 初入" {
		t.Fatalf("latest chapter = %#v", profile.LatestChapter)
	}
}

func TestParseCatalogFixture(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "scraper_examples", "qidian", "260619", "catalog.html"))
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("qidian catalog fixture not present")
		}
		t.Fatal(err)
	}
	profile, err := ParseBookProfile("https://m.qidian.com/book/1035420986/catalog/", body)
	if err != nil {
		t.Fatalf("ParseBookProfile: %v", err)
	}
	if profile.ChapterCount != 1581 || len(profile.Volumes) < 2 {
		t.Fatalf("catalog profile = chapters:%d volumes:%d", profile.ChapterCount, len(profile.Volumes))
	}
	if profile.Volumes[0].Title != "作品相关" || profile.Volumes[1].Chapters[0].Title != "第一章 初入" {
		t.Fatalf("volumes = %#v", profile.Volumes[:2])
	}
}

func TestParseURL(t *testing.T) {
	got, ok := ParseURL("https://m.qidian.com/book/1035420986/")
	if !ok {
		t.Fatal("ParseURL returned false")
	}
	if got.BookID != "1035420986" || got.Canonical != "https://m.qidian.com/book/1035420986/" {
		t.Fatalf("ParseURL = %#v", got)
	}
}
