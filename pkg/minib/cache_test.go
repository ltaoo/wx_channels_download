package minib

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNavigateCachesResourcesAndCanDisableCache(t *testing.T) {
	request_counts := make(map[string]int)
	disabled_requests := make(map[string]int)
	conditional_requests := 0
	var request_mutex sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		request_mutex.Lock()
		request_counts[request.URL.Path]++
		if request.Header.Get("Cache-Control") == "no-cache" && request.Header.Get("Pragma") == "no-cache" {
			disabled_requests[request.URL.Path]++
		}
		request_mutex.Unlock()
		switch request.URL.Path {
		case "/":
			variant := "one"
			if cookie, err := request.Cookie("variant"); err == nil && cookie.Value == "one" {
				variant = "two"
			}
			http.SetCookie(response_writer, &http.Cookie{Name: "variant", Value: variant, Path: "/"})
			response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprint(response_writer, `<!doctype html><html><head>
<link rel="stylesheet" href="/fresh.css">
<link rel="preload" as="font" href="/font.woff2">
<script src="/fresh.js"></script>
<script src="/vary.js"></script>
<script src="/revalidate.js"></script>
<script src="/no-store.js"></script>
</head><body><img src="/fresh.png"><script>
var harXhr = new XMLHttpRequest();
harXhr.open('POST', '/api?source=har');
harXhr.setRequestHeader('Content-Type', 'application/json');
harXhr.send('{"ok":true}');
</script></body></html>`)
		case "/fresh.js":
			response_writer.Header().Set("Content-Type", "application/javascript")
			response_writer.Header().Set("Cache-Control", "public, max-age=3600")
			_, _ = fmt.Fprint(response_writer, `window.fresh_loaded = true;`)
		case "/fresh.css":
			response_writer.Header().Set("Content-Type", "text/css")
			response_writer.Header().Set("Expires", time.Now().Add(time.Hour).UTC().Format(http.TimeFormat))
			_, _ = fmt.Fprint(response_writer, `body { color: green; }`)
		case "/fresh.png":
			response_writer.Header().Set("Content-Type", "image/png")
			response_writer.Header().Set("Cache-Control", "max-age=3600")
			_, _ = response_writer.Write([]byte("png"))
		case "/font.woff2":
			response_writer.Header().Set("Content-Type", "font/woff2")
			response_writer.Header().Set("Cache-Control", "max-age=3600")
			_, _ = response_writer.Write([]byte("font"))
		case "/revalidate.js":
			response_writer.Header().Set("Content-Type", "application/javascript")
			response_writer.Header().Set("Cache-Control", "no-cache")
			response_writer.Header().Set("ETag", `"script-v1"`)
			if request.Header.Get("If-None-Match") == `"script-v1"` {
				request_mutex.Lock()
				conditional_requests++
				request_mutex.Unlock()
				response_writer.WriteHeader(http.StatusNotModified)
				return
			}
			_, _ = fmt.Fprint(response_writer, `window.revalidated_loaded = true;`)
		case "/vary.js":
			response_writer.Header().Set("Content-Type", "application/javascript")
			response_writer.Header().Set("Cache-Control", "max-age=3600")
			response_writer.Header().Set("Vary", "Cookie")
			_, _ = fmt.Fprint(response_writer, `window.vary_loaded = true;`)
		case "/no-store.js":
			response_writer.Header().Set("Content-Type", "application/javascript")
			response_writer.Header().Set("Cache-Control", "no-store")
			_, _ = fmt.Fprint(response_writer, `window.no_store_loaded = true;`)
		case "/api":
			_, _ = io.ReadAll(request.Body)
			response_writer.Header().Set("Content-Type", "application/json")
			response_writer.Header().Set("Cache-Control", "no-store")
			_, _ = fmt.Fprint(response_writer, `{"saved":true}`)
		default:
			http.NotFound(response_writer, request)
		}
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(10 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()

	first_page, err := browser.Navigate(context.Background(), server.URL+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first_page.HAR(); err == nil {
		t.Fatal("HAR was captured without CaptureHAR")
	}
	second_page, err := browser.Navigate(context.Background(), server.URL+"/", nil, NavigateOptions{CaptureHAR: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, resource_path := range []string{"/fresh.js", "/fresh.css", "/fresh.png", "/font.woff2"} {
		if request_count(request_counts, &request_mutex, resource_path) != 1 {
			t.Fatalf("request count for %s = %d, want 1", resource_path, request_count(request_counts, &request_mutex, resource_path))
		}
		resource := find_page_resource(second_page, resource_path)
		if resource == nil || !resource.FromCache {
			t.Fatalf("resource %s was not served from cache: %+v", resource_path, resource)
		}
	}
	if resource := find_page_resource(second_page, "/font.woff2"); resource == nil || resource.Kind != FontResource {
		t.Fatalf("font resource = %+v", resource)
	}
	if request_count(request_counts, &request_mutex, "/revalidate.js") != 2 || request_count_value(&request_mutex, &conditional_requests) != 1 {
		t.Fatalf("revalidation requests=%d conditional=%d", request_count(request_counts, &request_mutex, "/revalidate.js"), request_count_value(&request_mutex, &conditional_requests))
	}
	if resource := find_page_resource(second_page, "/revalidate.js"); resource == nil || !resource.FromCache || len(resource.Body) == 0 {
		t.Fatalf("revalidated resource = %+v", resource)
	}
	if request_count(request_counts, &request_mutex, "/no-store.js") != 2 {
		t.Fatalf("no-store requests=%d", request_count(request_counts, &request_mutex, "/no-store.js"))
	}
	vary_resource := find_page_resource(second_page, "/vary.js")
	if request_count(request_counts, &request_mutex, "/vary.js") != 2 || vary_resource == nil || vary_resource.FromCache {
		t.Fatalf("vary cookie resource was incorrectly reused: %+v", vary_resource)
	}

	disabled_page, err := browser.Navigate(context.Background(), server.URL+"/", nil, NavigateOptions{DisableCache: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, resource := range disabled_page.Resources {
		if resource.FromCache {
			t.Fatalf("disabled navigation used cached resource %s", resource.URL)
		}
	}
	for _, resource_path := range []string{"/", "/fresh.js", "/fresh.css", "/fresh.png", "/font.woff2", "/vary.js", "/revalidate.js", "/no-store.js", "/api"} {
		if request_count(disabled_requests, &request_mutex, resource_path) != 1 {
			t.Fatalf("disabled cache headers for %s = %d, want 1", resource_path, request_count(disabled_requests, &request_mutex, resource_path))
		}
	}
	if request_count_value(&request_mutex, &conditional_requests) != 1 {
		t.Fatalf("disabled navigation sent a conditional request")
	}
	har_data, err := second_page.HAR()
	if err != nil {
		t.Fatal(err)
	}
	var har_file struct {
		Log struct {
			Version string `json:"version"`
			Pages   []struct {
				Title string `json:"title"`
			} `json:"pages"`
			Entries []struct {
				ResourceType string `json:"_resourceType"`
				FromCache    string `json:"_fromCache"`
				Request      struct {
					Method      string           `json:"method"`
					URL         string           `json:"url"`
					QueryString []har_name_value `json:"queryString"`
					PostData    *har_post_data   `json:"postData"`
				} `json:"request"`
				Response struct {
					Status  int `json:"status"`
					Content struct {
						Text     string `json:"text"`
						Encoding string `json:"encoding"`
					} `json:"content"`
				} `json:"response"`
			} `json:"entries"`
		} `json:"log"`
	}
	if err := json.Unmarshal(har_data, &har_file); err != nil {
		t.Fatal(err)
	}
	if har_file.Log.Version != "1.2" || len(har_file.Log.Pages) != 1 || len(har_file.Log.Entries) != 9 {
		t.Fatalf("invalid HAR summary: version=%q pages=%d entries=%d", har_file.Log.Version, len(har_file.Log.Pages), len(har_file.Log.Entries))
	}
	fresh_har_found := false
	revalidated_har_found := false
	image_har_found := false
	xhr_har_found := false
	for _, entry := range har_file.Log.Entries {
		switch {
		case strings.HasSuffix(entry.Request.URL, "/fresh.js"):
			fresh_har_found = entry.FromCache == "memory" && entry.ResourceType == "script" && entry.Response.Status == http.StatusOK
		case strings.HasSuffix(entry.Request.URL, "/revalidate.js"):
			revalidated_har_found = entry.Response.Status == http.StatusNotModified
		case strings.HasSuffix(entry.Request.URL, "/fresh.png"):
			image_har_found = entry.FromCache == "memory" && entry.Response.Content.Encoding == "base64" && entry.Response.Content.Text != ""
		case strings.Contains(entry.Request.URL, "/api?"):
			xhr_har_found = entry.ResourceType == "xhr" && entry.Request.Method == http.MethodPost && entry.Request.PostData != nil && entry.Request.PostData.Text == `{"ok":true}` && len(entry.Request.QueryString) == 1
		}
	}
	if !fresh_har_found || !revalidated_har_found || !image_har_found || !xhr_har_found {
		t.Fatalf("HAR entries missing: fresh=%t revalidated=%t image=%t xhr=%t", fresh_har_found, revalidated_har_found, image_har_found, xhr_har_found)
	}
	har_path := filepath.Join(t.TempDir(), "navigation.har")
	if err := os.WriteFile(har_path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(har_path, 0644); err != nil {
		t.Fatal(err)
	}
	if err := second_page.SaveHAR(har_path); err != nil {
		t.Fatal(err)
	}
	if file_info, err := os.Stat(har_path); err != nil || file_info.Mode().Perm() != 0600 {
		t.Fatalf("HAR file mode: info=%v err=%v", file_info, err)
	}
	html_path := filepath.Join(t.TempDir(), "navigation.html")
	if err := second_page.SaveHTML(html_path); err != nil {
		t.Fatal(err)
	}
	html_data, err := os.ReadFile(html_path)
	if err != nil {
		t.Fatal(err)
	}
	if string(html_data) != second_page.RenderedHTML {
		t.Fatal("saved HTML does not match the post-JavaScript DOM")
	}
	if file_info, err := os.Stat(html_path); err != nil || file_info.Mode().Perm() != 0600 {
		t.Fatalf("HTML file mode: info=%v err=%v", file_info, err)
	}
	if len(first_page.ScriptFailures) != 0 || len(second_page.ScriptFailures) != 0 || len(disabled_page.ScriptFailures) != 0 {
		t.Fatalf("script failures: first=%+v second=%+v disabled=%+v", first_page.ScriptFailures, second_page.ScriptFailures, disabled_page.ScriptFailures)
	}
}

func TestResourceCacheEvictsLeastRecentlyUsedEntriesWithinLimits(t *testing.T) {
	cache := new_resource_cache(ResourceCacheLimits{MaxEntries: 2, MaxBytes: 1 << 20})
	headers := http.Header{"Cache-Control": []string{"max-age=3600"}}
	store := func(raw_url string, body string) {
		cache.store(raw_url, nil, headers, Resource{URL: raw_url, StatusCode: http.StatusOK, Body: []byte(body)}, time.Now())
	}
	store("https://example.test/one", "1")
	store("https://example.test/two", "2")
	if _, found := cache.lookup("https://example.test/one", nil); !found {
		t.Fatal("first entry was not cached")
	}
	store("https://example.test/three", "3")
	if _, found := cache.lookup("https://example.test/two", nil); found {
		t.Fatal("least-recently-used entry was not evicted")
	}
	if _, found := cache.lookup("https://example.test/one", nil); !found {
		t.Fatal("recently used entry was evicted")
	}
	if _, found := cache.lookup("https://example.test/three", nil); !found {
		t.Fatal("new entry was evicted")
	}

	byte_limit := cache.current_bytes - 1
	cache.set_limits(ResourceCacheLimits{MaxBytes: byte_limit})
	if cache.entry_count != 1 || cache.current_bytes > byte_limit {
		t.Fatalf("byte limit not applied: entries=%d bytes=%d", cache.entry_count, cache.current_bytes)
	}
}

func find_page_resource(page *Page, suffix string) *Resource {
	if page == nil {
		return nil
	}
	for index := range page.Resources {
		if strings.HasSuffix(page.Resources[index].URL, suffix) {
			return &page.Resources[index]
		}
	}
	return nil
}

func request_count(counts map[string]int, mutex *sync.Mutex, path string) int {
	mutex.Lock()
	defer mutex.Unlock()
	return counts[path]
}

func request_count_value(mutex *sync.Mutex, value *int) int {
	mutex.Lock()
	defer mutex.Unlock()
	return *value
}
