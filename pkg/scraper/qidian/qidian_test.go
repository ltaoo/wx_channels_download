package qidian

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseURL(t *testing.T) {
	got, ok := ParseURL("https://www.qidian.com/book/1035420986/")
	if !ok {
		t.Fatal("ParseURL returned false")
	}
	if got.BookID != "1035420986" || got.Canonical != "https://www.qidian.com/book/1035420986/" {
		t.Fatalf("ParseURL = %#v", got)
	}
	if _, ok := ParseURL("https://m.qidian.com/book/1035420986/"); ok {
		t.Fatal("desktop qidian parser should not match m.qidian.com")
	}
}

func TestParseBookProfileFixture(t *testing.T) {
	body := readFixture(t)
	profile, err := ParseBookProfile("https://www.qidian.com/book/1035420986/", body)
	if err != nil {
		t.Fatalf("ParseBookProfile: %v", err)
	}
	if profile.Title != "玄鉴仙族" || profile.Author.Name != "季越人" {
		t.Fatalf("profile = %#v", profile)
	}
	if profile.Author.ID != "430784443" {
		t.Fatalf("author id = %q", profile.Author.ID)
	}
	if profile.Category != "仙侠" || profile.SubCategory != "修真文明" || profile.Status != "连载" {
		t.Fatalf("category/status = %q/%q/%q", profile.Category, profile.SubCategory, profile.Status)
	}
	if profile.ChapterCount != 1581 {
		t.Fatalf("chapter count = %d", profile.ChapterCount)
	}
	if len(profile.Volumes) == 0 || profile.Volumes[0].Title != "作品相关" {
		t.Fatalf("volumes = %#v", profile.Volumes)
	}
	if len(profile.Volumes) < 2 || len(profile.Volumes[1].Chapters) == 0 || profile.Volumes[1].Chapters[0].Title != "第一章 初入" {
		t.Fatalf("first main chapter = %#v", profile.Volumes)
	}
	if profile.LatestChapter.Title != "第一千五百一十四章 神库（1+1/2）Raincheck黄金盟加更5" {
		t.Fatalf("latest chapter = %#v", profile.LatestChapter)
	}
	expectedDescription := "陆江仙熬夜猝死，残魂却附在了一面满是裂痕的青灰色铜镜上，飘落到了浩瀚无垠的修仙世界。\n凶险难测的大黎山，眉尺河旁小小的村落，一个小家族拾到了这枚镜子，于是传仙道授仙法，开启波澜壮阔的新时代。\n(家族修仙，不圣母，种田，无系统，群像文)"
	if profile.Description != expectedDescription {
		t.Fatalf("description = %q", profile.Description)
	}
	if profile.Volumes[0].Idx != 1 || profile.Volumes[0].Chapters[0].Idx != 1 {
		t.Fatalf("volume/chapter indexes = %#v", profile.Volumes[0])
	}
	if profile.Volumes[0].Chapters[0].URL != "https://www.qidian.com/chapter/1035420986/753321428/" {
		t.Fatalf("first chapter url = %q", profile.Volumes[0].Chapters[0].URL)
	}
	if profile.Volumes[1].Title != "溪里青玄" {
		t.Fatalf("second volume title = %q", profile.Volumes[1].Title)
	}
	if profile.DisplayWordCount != "588.63万" || profile.WordCount != 5886300 {
		t.Fatalf("word count = %q/%d", profile.DisplayWordCount, profile.WordCount)
	}
	if profile.PageHTML == "" || len(profile.PageContextJSON) == 0 || !json.Valid(profile.PageContextJSON) {
		t.Fatal("expected page html and valid g_data.pageJson")
	}
}

func TestExtractPageJSONFixture(t *testing.T) {
	raw, err := ExtractPageJSON(readFixture(t))
	if err != nil {
		t.Fatalf("ExtractPageJSON: %v", err)
	}
	if !json.Valid(raw) || !strings.Contains(string(raw), `"bookId":1035420986`) {
		t.Fatalf("page json = %s", raw)
	}
}

func TestFetchBookProfileFallsBackToMobile(t *testing.T) {
	fixture := readFixture(t)
	client := NewClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "www.qidian.com":
			return stringResponse(http.StatusAccepted, `<!DOCTYPE html><script src="/C2WF946J0/probe.js"></script>`), nil
		case "m.qidian.com":
			return bytesResponse(http.StatusOK, fixture), nil
		default:
			t.Fatalf("unexpected host: %s", req.URL.Host)
			return nil, nil
		}
	})})
	profile, err := client.FetchBookProfileContext(context.Background(), "1035420986")
	if err != nil {
		t.Fatalf("FetchBookProfileContext: %v", err)
	}
	if profile.Title != "玄鉴仙族" || profile.URL != "https://www.qidian.com/book/1035420986/" {
		t.Fatalf("profile = %#v", profile)
	}
}

func TestFetchBookProfileSupplementsMobileCatalog(t *testing.T) {
	mobileRecent := []byte(`<script id="vite-plugin-ssr_pageContext" type="application/json">{"pageContext":{"pageProps":{"pageData":{"bookInfo":{"bookId":1035420986,"bookName":"玄鉴仙族","desc":"简介","authorName":"季越人","updChapterName":"第一章 初入","updChapterUrl":"//m.qidian.com/chapter/1035420986/730944635/","bookStatus":"连载","wordsCnt":2039,"showWordsCnt":"2039"},"cTCnt":2,"recentChapters":[{"id":730944635,"cN":"第一章 初入","uT":"2022-10-07 10:22:55","cnt":2039}],"authorInfo":{"authorId":430784443,"authorName":"季越人"}}},"urlPathname":"/book/1035420986/"}}</script>`)
	mobileCatalog := []byte(`<script id="vite-plugin-ssr_pageContext" type="application/json">{"pageContext":{"pageProps":{"pageData":{"bookId":1035420986,"bookName":"玄鉴仙族","chapterTotalCnt":2,"bookStatus":"连载","actionStatus":"连载中","updateTime":"2022-10-07 10:22:55","authorInfo":{"authorId":430784443,"authorName":"季越人"},"vs":[{"vN":"作品相关","cCnt":1,"cs":[{"sS":1,"cnt":475,"cN":"感谢大佬们","id":753321428,"uuid":379,"uT":"2023-05-13 21:36:25"}]},{"vN":"溪里青玄","cCnt":1,"cs":[{"sS":0,"cnt":2039,"cN":"第一章 初入","id":730944635,"uuid":1,"uT":"2022-10-07 10:22:55"}]}]}},"urlPathname":"/book/1035420986/catalog/"}}</script>`)
	client := NewClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Host == "www.qidian.com":
			return stringResponse(http.StatusAccepted, `<!DOCTYPE html><script src="/C2WF946J0/probe.js"></script>`), nil
		case req.URL.Host == "m.qidian.com" && req.URL.Path == "/book/1035420986/":
			return bytesResponse(http.StatusOK, mobileRecent), nil
		case req.URL.Host == "m.qidian.com" && req.URL.Path == "/book/1035420986/catalog/":
			return bytesResponse(http.StatusOK, mobileCatalog), nil
		default:
			t.Fatalf("unexpected request: %s", req.URL.String())
			return nil, nil
		}
	})})
	profile, err := client.FetchBookProfileContext(context.Background(), "1035420986")
	if err != nil {
		t.Fatalf("FetchBookProfileContext: %v", err)
	}
	if profile.ChapterCount != 2 || len(profile.Volumes) != 2 {
		t.Fatalf("profile catalog = %#v", profile)
	}
	if profile.Volumes[0].Title == "最近更新" {
		t.Fatalf("recent chapters should not be exposed as full catalog: %#v", profile.Volumes)
	}
	if profile.Volumes[0].Title != "作品相关" || profile.Volumes[1].Chapters[0].Title != "第一章 初入" {
		t.Fatalf("volumes = %#v", profile.Volumes)
	}
	if profile.Volumes[1].Chapters[0].URL != "https://www.qidian.com/chapter/1035420986/730944635/" {
		t.Fatalf("desktop chapter url = %q", profile.Volumes[1].Chapters[0].URL)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func stringResponse(status int, body string) *http.Response {
	return bytesResponse(status, []byte(body))
}

func bytesResponse(status int, body []byte) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(string(body))),
	}
}

func readFixture(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "scraper_examples", "qidian", "260619", "book.html"))
	if err != nil {
		t.Fatal(err)
	}
	return body
}
