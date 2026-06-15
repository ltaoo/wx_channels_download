package shuba69

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseURL(t *testing.T) {
	bookHTML, ok := ParseURL("https://www.69shuba.com/book/34567.htm")
	if !ok {
		t.Fatal("expected book html url match")
	}
	if bookHTML.Kind != ContentTypeNovel || bookHTML.BookID != "34567" || bookHTML.Canonical != "https://www.69shuba.com/book/34567.htm" {
		t.Fatalf("book html url = %#v", bookHTML)
	}

	bookCatalog, ok := ParseURL("https://www.69shuba.com/book/34567/")
	if !ok {
		t.Fatal("expected book catalog url match")
	}
	if bookCatalog.Kind != ContentTypeNovel || bookCatalog.BookID != "34567" || bookCatalog.Canonical != "https://www.69shuba.com/book/34567/" {
		t.Fatalf("book catalog url = %#v", bookCatalog)
	}

	chapter, ok := ParseURL("https://www.69shuba.com/txt/34567/41064442")
	if !ok {
		t.Fatal("expected chapter url match")
	}
	if chapter.Kind != ContentTypeChapter || chapter.BookID != "34567" || chapter.ChapterID != "41064442" {
		t.Fatalf("chapter url = %#v", chapter)
	}
}

func TestParseNovelFixture(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "69shuba_260614.html"))
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("69shuba_260614.html fixture not present")
		}
		t.Fatal(err)
	}
	novel, err := ParseNovelHTML("https://www.69shuba.com/book/34567.htm", string(body))
	if err != nil {
		t.Fatalf("ParseNovelHTML: %v", err)
	}
	if novel.BookID != "34567" || novel.Title != "坐忘长生" {
		t.Fatalf("novel identity = %#v", novel)
	}
	if novel.Author != "飞翔的黎哥" || novel.Category != "修真武侠" || novel.Status != "连载" {
		t.Fatalf("novel metadata = %#v", novel)
	}
	if novel.WordCount != "465.17万字" || novel.ChapterCount != 2016 {
		t.Fatalf("count metadata word=%q chapter=%d", novel.WordCount, novel.ChapterCount)
	}
	if novel.FullCatalogURL != "https://www.69shuba.com/book/34567/" {
		t.Fatalf("full catalog url = %q", novel.FullCatalogURL)
	}
	if len(novel.Chapters) != 5 {
		t.Fatalf("chapters len = %d", len(novel.Chapters))
	}
	if novel.Chapters[0].Title != "第2006章 元神化象" || novel.Chapters[0].URL != "https://www.69shuba.com/txt/34567/41064442" {
		t.Fatalf("first chapter = %#v", novel.Chapters[0])
	}
	if len(novel.Tags) == 0 || novel.Tags[0] != "炼丹" {
		t.Fatalf("tags = %#v", novel.Tags)
	}
	out := BuildNovelHTML(novel)
	for _, want := range []string{"<!doctype html>", "坐忘长生", "第2006章 元神化象", "小小少年柳清欢"} {
		if !strings.Contains(out, want) {
			t.Fatalf("rendered html missing %q: %s", want, out)
		}
	}
}

func TestFetchNovelChaptersLoadsFullCatalog(t *testing.T) {
	client := NewClient(fakeHTTPClient(func(req *http.Request) string {
		switch req.URL.Path {
		case "/book/1.htm":
			return `<!doctype html><html><head><meta charset="utf-8"><meta property="og:novel:book_name" content="book"><meta property="og:novel:author" content="author"></head><body><a class="more-btn" href="https://www.69shuba.com/book/1/">完整目录</a><div class="qustime"><ul><li><a href="/txt/1/10"><span>chapter 10</span></a></li></ul></div></body></html>`
		case "/book/1/":
			return `<!doctype html><html><body><h1>book</h1><div class="qustime"><ul><li><a href="/txt/1/1"><span>chapter 1</span></a></li><li><a href="/txt/1/2"><span>chapter 2</span></a></li></ul></div></body></html>`
		default:
			t.Fatalf("unexpected request path %s", req.URL.Path)
			return ""
		}
	}))

	result, err := client.FetchNovel("https://www.69shuba.com/book/1.htm")
	if err != nil {
		t.Fatalf("FetchNovel: %v", err)
	}
	novel := result.Novel
	if len(novel.Chapters) != 2 || novel.Chapters[0].Title != "chapter 1" || novel.Chapters[1].Title != "chapter 2" {
		t.Fatalf("chapters = %#v", novel.Chapters)
	}
	if result.SourceNovel == nil || len(result.SourceNovel.Chapters) != 1 {
		t.Fatalf("source novel = %#v", result.SourceNovel)
	}
	if result.FullCatalogURL != "https://www.69shuba.com/book/1/" || result.FullCatalogNovel == nil {
		t.Fatalf("full catalog result = %#v", result)
	}
	if !strings.Contains(result.FullCatalogParsedHTML, "chapter 2") {
		t.Fatalf("full catalog parsed html = %q", result.FullCatalogParsedHTML)
	}
}

func TestFetchNovelUsesBrowserHeadersAndCookie(t *testing.T) {
	var requests []*http.Request
	client := NewClientWithOptions(fakeHTTPClient(func(req *http.Request) string {
		requests = append(requests, req)
		switch req.URL.Path {
		case "/book/1.htm":
			return `<!doctype html><html><head><meta charset="utf-8"><meta property="og:novel:book_name" content="book"></head><body><a class="more-btn" href="https://www.69shuba.com/book/1/">完整目录</a><div class="qustime"><ul><li><a href="/txt/1/1"><span>chapter 1</span></a></li></ul></div></body></html>`
		case "/book/1/":
			return `<!doctype html><html><body><h1>book</h1><div class="qustime"><ul><li><a href="/txt/1/1"><span>chapter 1</span></a></li></ul></div></body></html>`
		default:
			t.Fatalf("unexpected request path %s", req.URL.Path)
			return ""
		}
	}), "cf_clearance=token; zh_choose=s", "")

	if _, err := client.FetchNovel("https://www.69shuba.com/book/1.htm"); err != nil {
		t.Fatalf("FetchNovel: %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("requests len = %d", len(requests))
	}
	first := requests[0]
	if got := first.Header.Get("Cookie"); got != "cf_clearance=token; zh_choose=s" {
		t.Fatalf("cookie header = %q", got)
	}
	if got := first.Header.Get("Referer"); got != "https://www.69shuba.com/book/1/" {
		t.Fatalf("referer = %q", got)
	}
	for key, want := range map[string]string{
		"Accept":             "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		"Cache-Control":      "no-cache",
		"Pragma":             "no-cache",
		"Sec-Ch-Ua-Platform": `"macOS"`,
		"Sec-Fetch-Dest":     "document",
		"Sec-Fetch-Mode":     "navigate",
		"Sec-Fetch-Site":     "same-origin",
		"Sec-Fetch-User":     "?1",
	} {
		if got := first.Header.Get(key); got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
	if got := first.Header.Get("User-Agent"); !strings.Contains(got, "Chrome/149.0.0.0") {
		t.Fatalf("user-agent = %q", got)
	}
}

func TestFetchNovelUsesHTMLFetcher(t *testing.T) {
	fetcher := &fakeHTMLFetcher{pages: map[string]string{
		"https://www.69shuba.com/book/1.htm": `<!doctype html><html><head><meta charset="utf-8"><meta property="og:novel:book_name" content="book"></head><body><a class="more-btn" href="https://www.69shuba.com/book/1/">完整目录</a><div class="qustime"><ul><li><a href="/txt/1/10"><span>chapter 10</span></a></li></ul></div></body></html>`,
		"https://www.69shuba.com/book/1/":    `<!doctype html><html><body><h1>book</h1><div class="qustime"><ul><li><a href="/txt/1/1"><span>chapter 1</span></a></li><li><a href="/txt/1/2"><span>chapter 2</span></a></li></ul></div></body></html>`,
	}}
	client := NewClientWithHTMLFetcher(fetcher, "cf_clearance=token", "")
	client.HTTPClient = fakeHTTPClient(func(req *http.Request) string {
		t.Fatalf("HTTP client should not be called for %s", req.URL.String())
		return ""
	})

	result, err := client.FetchNovel("https://www.69shuba.com/book/1.htm")
	if err != nil {
		t.Fatalf("FetchNovel: %v", err)
	}
	if len(fetcher.calls) != 2 {
		t.Fatalf("fetcher calls = %#v", fetcher.calls)
	}
	if got := fetcher.headers[0].Get("Cookie"); got != "cf_clearance=token" {
		t.Fatalf("cookie header = %q", got)
	}
	if got := fetcher.referers[0]; got != "https://www.69shuba.com/book/1/" {
		t.Fatalf("first referer = %q", got)
	}
	if got := fetcher.referers[1]; got != "https://www.69shuba.com/book/1.htm" {
		t.Fatalf("catalog referer = %q", got)
	}
	if len(result.Novel.Chapters) != 2 || result.FullCatalogNovel == nil {
		t.Fatalf("result = %#v", result)
	}
}

func TestFetchNovelUsesSingleHTMLFetchSession(t *testing.T) {
	fetcher := &fakeHTMLFetcher{pages: map[string]string{
		"https://www.69shuba.com/book/1.htm": `<!doctype html><html><head><meta charset="utf-8"><meta property="og:novel:book_name" content="book"></head><body><a class="more-btn" href="https://www.69shuba.com/book/1/">完整目录</a><div class="qustime"><ul><li><a href="/txt/1/10"><span>chapter 10</span></a></li></ul></div></body></html>`,
		"https://www.69shuba.com/book/1/":    `<!doctype html><html><body><h1>book</h1><div class="qustime"><ul><li><a href="/txt/1/1"><span>chapter 1</span></a></li><li><a href="/txt/1/2"><span>chapter 2</span></a></li></ul></div></body></html>`,
	}}
	session := &fakeSessionFetcher{fetcher: fetcher}
	client := NewClientWithHTMLFetcher(session, "", "")
	if _, err := client.FetchNovel("https://www.69shuba.com/book/1.htm"); err != nil {
		t.Fatal(err)
	}
	if session.beginCount != 1 || session.doneCount != 1 || session.doneErr != nil {
		t.Fatalf("session begin=%d done=%d err=%v", session.beginCount, session.doneCount, session.doneErr)
	}
	if len(fetcher.calls) != 2 {
		t.Fatalf("fetch calls = %#v", fetcher.calls)
	}
}

func TestCDPCookiePairs(t *testing.T) {
	pairs := cdpCookiePairs("cf_clearance=a=b; zh_choose=s; empty; spaced = value ")
	if len(pairs) != 3 {
		t.Fatalf("pairs = %#v", pairs)
	}
	if pairs[0].name != "cf_clearance" || pairs[0].value != "a=b" {
		t.Fatalf("first pair = %#v", pairs[0])
	}
	if pairs[2].name != "spaced" || pairs[2].value != "value" {
		t.Fatalf("third pair = %#v", pairs[2])
	}
}

func TestCDPFetcherChecksEndpointBeforeCreatingTarget(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls = append(calls, req.Method+" "+req.URL.Path)
		switch req.URL.Path {
		case "/json/version":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"Browser":"Chrome/Test"}`))
		case "/json/new":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"target-1","webSocketDebuggerUrl":"ws://127.0.0.1:9222/devtools/page/target-1"}`))
		default:
			http.NotFound(w, req)
		}
	}))
	defer server.Close()

	fetcher := NewCDPFetcher(server.URL)
	wsURL, targetID, err := fetcher.openTarget(context.Background())
	if err != nil {
		t.Fatalf("openTarget: %v", err)
	}
	if wsURL != "ws://127.0.0.1:9222/devtools/page/target-1" || targetID != "target-1" {
		t.Fatalf("target = %q %q", wsURL, targetID)
	}
	if len(calls) < 2 || calls[0] != "GET /json/version" || calls[1] != "PUT /json/new" {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestCDPFetcherUnavailableErrorMentionsManualDockerCDP(t *testing.T) {
	fetcher := NewCDPFetcher("http://127.0.0.1:9222")
	fetcher.HTTPClient = errHTTPClient{err: errors.New("connection refused")}
	_, _, err := fetcher.openTarget(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{"CDP endpoint http://127.0.0.1:9222 is not available", "docker run", "wx-69shuba-cdp", "39000:3000", "lscr.io/linuxserver/chromium:latest"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q does not contain %q", msg, want)
		}
	}
}

func TestSandboxCDPFetcherBuildsProxyWebSocketURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost || req.URL.Path != "/api/v1/sandboxes/sb1/cdp/apply" {
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["mode"] != "page" {
			t.Fatalf("mode = %#v", body["mode"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ticket":"ticket-1","expires_at":123}`))
	}))
	defer server.Close()

	fetcher := NewSandboxCDPFetcher(server.URL+"/api/v1", "sb1")
	wsURL, err := fetcher.cdpWebSocketURL(context.Background())
	if err != nil {
		t.Fatalf("cdpWebSocketURL: %v", err)
	}
	wantPrefix := strings.Replace(server.URL, "http://", "ws://", 1) + "/api/v1/sandboxes/sb1/cdp/browser?"
	if !strings.HasPrefix(wsURL, wantPrefix) || !strings.Contains(wsURL, "ticket=ticket-1") {
		t.Fatalf("wsURL = %q, want prefix %q with ticket", wsURL, wantPrefix)
	}
}

func TestParseChapterHTML(t *testing.T) {
	htmlText := `<!doctype html><html><body><div class="txtnav"><h1>第1章 开始</h1><div id="htmlContent">第一行<br />第二行<br />请收藏本站 69书吧</div></div></body></html>`
	chapter, err := ParseChapterHTML(htmlText)
	if err != nil {
		t.Fatalf("ParseChapterHTML: %v", err)
	}
	if chapter.Title != "第1章 开始" || chapter.Content != "第一行\n第二行" {
		t.Fatalf("chapter = %#v", chapter)
	}
}

type fakeHTTPClient func(req *http.Request) string

func (f fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	body := f(req)
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

type errHTTPClient struct {
	err error
}

func (c errHTTPClient) Do(*http.Request) (*http.Response, error) {
	return nil, c.err
}

type fakeHTMLFetcher struct {
	pages    map[string]string
	calls    []string
	referers []string
	headers  []http.Header
}

func (f *fakeHTMLFetcher) FetchHTML(rawURL string, referer string, headers http.Header) (string, error) {
	f.calls = append(f.calls, rawURL)
	f.referers = append(f.referers, referer)
	f.headers = append(f.headers, headers.Clone())
	return f.pages[rawURL], nil
}

type fakeSessionFetcher struct {
	fetcher    *fakeHTMLFetcher
	beginCount int
	doneCount  int
	doneErr    error
}

func (f *fakeSessionFetcher) BeginHTMLFetchSession() (HTMLFetcher, func(error), error) {
	f.beginCount++
	return f.fetcher, func(err error) {
		f.doneCount++
		f.doneErr = err
	}, nil
}

func (f *fakeSessionFetcher) FetchHTML(rawURL string, referer string, headers http.Header) (string, error) {
	return f.fetcher.FetchHTML(rawURL, referer, headers)
}
