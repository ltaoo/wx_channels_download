package zhihu

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"wx_channel/pkg/cache"
)

const test_article_id = "1992203318399898641"
const test_article_url = "https://zhuanlan.zhihu.com/p/1992203318399898641"
const test_har_article_id = "1984716911095861438"
const test_har_article_url = "https://zhuanlan.zhihu.com/p/1984716911095861438"

func TestPCWebHasArticleRejectsTruncatedAppViewContent(t *testing.T) {
	truncated_document := test_appview_article_document(t, true, `<p>我们采取了人类和猴子都能玩的双人游</p>`)
	if pcweb_has_article(truncated_document, test_article_id) {
		t.Fatal("truncated AppView article must not be accepted as a complete document")
	}

	initial_data, err := ParseInitialData(truncated_document)
	if err != nil {
		t.Fatal(err)
	}
	article, ok := article_from_initial_data(initial_data, test_article_id)
	if !ok || !article.ContentNeedTruncated || !article.ForceLoginWhenClickReadMore {
		t.Fatalf("article truncation flags = %#v", article)
	}

	if _, err := parse_article_page(truncated_document, ArticleURL{ArticleID: test_article_id, Canonical: test_article_url}); err == nil {
		t.Fatal("parse_article_page accepted truncated article content")
	}
}

func TestFetchArticlePageUsesHARCanonicalDocumentBefore403Fallbacks(t *testing.T) {
	full_content := `<p>HAR 返回的完整正文</p><h2>正文结尾</h2>`
	full_document := test_article_document(t, test_har_article_id, false, full_content)
	canonical_requests := 0

	client := NewClient(nil, nil)
	client.http_client = &http.Client{Transport: round_trip_func(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != test_har_article_url {
			t.Fatalf("unexpected fallback request: %s %s", req.Method, req.URL.String())
		}
		canonical_requests++
		if req.Method != http.MethodGet ||
			req.Header.Get("Referer") != test_har_article_url ||
			req.Header.Get("Sec-Fetch-Site") != "same-origin" ||
			req.Header.Get("Sec-Fetch-Dest") != "document" ||
			req.Header.Get("Sec-Fetch-Mode") != "navigate" ||
			req.Header.Get("Sec-Fetch-User") != "" ||
			req.Header.Get("Upgrade-Insecure-Requests") != "1" ||
			req.Header.Get("User-Agent") != pcweb_desktop_user_agent {
			t.Fatalf("canonical Article request headers = %#v", req.Header)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(string(full_document))),
			Request:    req,
		}, nil
	})}

	page, err := client.FetchArticlePage(test_har_article_url)
	if err != nil {
		t.Fatal(err)
	}
	if canonical_requests != 1 {
		t.Fatalf("canonical requests = %d, want 1", canonical_requests)
	}
	if page.Article.Content != full_content || page.Article.ContentNeedTruncated {
		t.Fatalf("article = %#v", page.Article)
	}
}

func TestFetchArticlePageReplacesTruncatedCacheWithArticleAPIContent(t *testing.T) {
	truncated_document := test_appview_article_document(t, true, `<p>我们采取了人类和猴子都能玩的双人游</p>`)
	full_content := `<p>我们采取了人类和猴子都能玩的双人游戏。</p>` +
		`<a class="video-box" data-lens-id="1992214443061424967" href="https://www.zhihu.com/video/1992214443061424967">视频</a>` +
		`<h2>完整正文结尾</h2>`

	registry, err := cache.NewProviderRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	file_cache, err := registry.Namespace("zhihu")
	if err != nil {
		t.Fatal(err)
	}

	client := NewClient(nil, nil)
	client.SetPersistentCache(file_cache)
	if err := client.write_cached_html(test_article_url, truncated_document); err != nil {
		t.Fatal(err)
	}

	api_requests := 0
	client.http_client = &http.Client{Transport: round_trip_func(func(req *http.Request) (*http.Response, error) {
		var status_code = http.StatusOK
		var response_body string
		switch {
		case req.URL.Path == "/explore":
			response_body = "<!doctype html><html></html>"
		case req.URL.Path == "/appview/p/"+test_article_id ||
			(req.URL.Host == "zhuanlan.zhihu.com" && req.URL.Path == "/p/"+test_article_id):
			response_body = string(truncated_document)
		case req.URL.Path == "/api/v4/articles/"+test_article_id:
			api_requests++
			payload := map[string]any{
				"id":      test_article_id,
				"title":   "《自然》：为什么百闻不如一见？",
				"content": full_content,
				"author":  map[string]any{"name": "测试作者", "urlToken": "author"},
			}
			payload_data, marshal_err := json.Marshal(payload)
			if marshal_err != nil {
				t.Fatal(marshal_err)
			}
			response_body = string(payload_data)
		default:
			status_code = http.StatusNotFound
			response_body = "not found"
		}
		return &http.Response{
			StatusCode: status_code,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(response_body)),
			Request:    req,
		}, nil
	})}

	page, err := client.FetchArticlePage(test_article_url)
	if err != nil {
		t.Fatal(err)
	}
	if api_requests != 1 {
		t.Fatalf("Article API requests = %d, want 1", api_requests)
	}
	if page.Article.Content != full_content || page.Article.ContentNeedTruncated {
		t.Fatalf("article = %#v", page.Article)
	}

	cached_document, cached, err := client.read_cached_html(test_article_url)
	if err != nil {
		t.Fatal(err)
	}
	if !cached || !pcweb_has_article(cached_document, test_article_id) {
		t.Fatal("complete Article API document did not replace the truncated cache")
	}
	if strings.Contains(string(cached_document), `contentNeedTruncated":true`) {
		t.Fatal("cached document still contains the truncated article marker")
	}
}

func TestDecodePCWebArticleAPIRejectsTruncatedContent(t *testing.T) {
	payload := fmt.Sprintf(`{"id":"%s","content":"<p>截断正文</p>","content_need_truncated":true}`, test_article_id)
	if _, err := decode_pcweb_article_api([]byte(payload), test_article_id); err == nil {
		t.Fatal("Article API decoder accepted truncated content")
	}
}

func test_appview_article_document(t *testing.T, truncated bool, content string) []byte {
	return test_article_document(t, test_article_id, truncated, content)
}

func test_article_document(t *testing.T, article_id string, truncated bool, content string) []byte {
	t.Helper()
	initial_data := map[string]any{
		"initialState": map[string]any{
			"post": map[string]any{
				article_id: map[string]any{
					"id":                          article_id,
					"title":                       "《自然》：为什么百闻不如一见？",
					"content":                     content,
					"contentNeedTruncated":        truncated,
					"forceLoginWhenClickReadMore": truncated,
					"author":                      map[string]any{"name": "测试作者", "urlToken": "author"},
				},
			},
		},
	}
	payload, err := json.Marshal(initial_data)
	if err != nil {
		t.Fatal(err)
	}
	return []byte(`<!doctype html><html><body><script id="js-initialData" type="text/json">` + string(payload) + `</script></body></html>`)
}
