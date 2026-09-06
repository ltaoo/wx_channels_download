// Package homepage provides rendered-page and persistent-cache plumbing for
// platform scrapers that need authenticated account-home requests.
package homepage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"wx_channel/pkg/cache"
	"wx_channel/pkg/cookies"
	"wx_channel/pkg/minib"
)

const request_timeout = 90 * time.Second

// FetchOptions controls one rendered page request.
type FetchOptions struct {
	URL                 string
	Scope               string
	AccountID           string
	WaitForSelector     string
	DisableSubresources bool
	DisableJavaScript   bool
	Headers             http.Header
	CookieReader        *cookies.Reader
	Cache               *cache.CacheProvider
}

// Fetch renders a page with persistent cookies and caches its HTML.
func Fetch(fetch_context context.Context, options FetchOptions) (string, error) {
	page_url := strings.TrimSpace(options.URL)
	if page_url == "" {
		return "", fmt.Errorf("account home URL is empty")
	}
	if fetch_context == nil {
		fetch_context = context.Background()
	}
	fetch_context, cancel_fetch := context.WithTimeout(fetch_context, request_timeout)
	defer cancel_fetch()
	browser, err := minib.NewMiniBrowser(request_timeout, options.CookieReader)
	if err != nil {
		return "", fmt.Errorf("create account home browser: %w", err)
	}
	defer browser.Close()

	page, err := browser.Navigate(fetch_context, page_url, options.Headers, minib.NavigateOptions{
		DisableSubresources: options.DisableSubresources,
		DisableImages:       true,
		DisableMedia:        true,
		DisableJavaScript:   options.DisableJavaScript,
		ResourceTimeout:     20 * time.Second,
		WaitForSelector:     strings.TrimSpace(options.WaitForSelector),
	})
	if err != nil {
		if cached_html, cache_err := read_cache(options.Cache, html_cache_path(page_url, options.Scope, options.AccountID)); cache_err == nil && len(cached_html) > 0 {
			return string(cached_html), nil
		}
		return "", fmt.Errorf("render account home: %w", err)
	}
	if page.StatusCode < http.StatusOK || page.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("account home returned HTTP %d", page.StatusCode)
	}
	if login_page(page.URL, page.RenderedHTML) {
		return "", fmt.Errorf("cookies.json 中的登录凭证无效或已过期")
	}
	document_html := page.RenderedHTML
	if strings.TrimSpace(document_html) == "" {
		document_html = page.HTML
	}
	if strings.TrimSpace(document_html) == "" {
		return "", fmt.Errorf("account home returned empty HTML")
	}
	if err := write_cache(options.Cache, html_cache_path(page_url, options.Scope, options.AccountID), []byte(document_html)); err != nil {
		return "", fmt.Errorf("cache account home: %w", err)
	}
	return document_html, nil
}

// RequestOptions controls one authenticated API request.
type RequestOptions struct {
	URL          string
	Method       string
	Body         []byte
	Scope        string
	AccountID    string
	Headers      http.Header
	CookieReader *cookies.Reader
	Cache        *cache.CacheProvider
}

// Request sends a browser-fingerprinted request and caches its response body.
func Request(fetch_context context.Context, options RequestOptions) ([]byte, error) {
	request_url := strings.TrimSpace(options.URL)
	if request_url == "" {
		return nil, fmt.Errorf("account home API URL is empty")
	}
	if fetch_context == nil {
		fetch_context = context.Background()
	}
	fetch_context, cancel_fetch := context.WithTimeout(fetch_context, request_timeout)
	defer cancel_fetch()
	browser, err := minib.NewMiniBrowser(request_timeout, options.CookieReader)
	if err != nil {
		return nil, fmt.Errorf("create account home API client: %w", err)
	}
	defer browser.Close()
	method := strings.ToUpper(strings.TrimSpace(options.Method))
	if method == "" {
		method = http.MethodGet
	}
	response, err := browser.Request(fetch_context, method, request_url, bytes.NewReader(options.Body), options.Headers)
	cache_key := api_cache_path(request_url, options.Scope, options.AccountID, options.Body)
	if err != nil {
		if cached_data, cache_err := read_cache(options.Cache, cache_key); cache_err == nil && len(cached_data) > 0 {
			return cached_data, nil
		}
		return nil, fmt.Errorf("request account home API: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("account home API returned HTTP %d", response.StatusCode)
	}
	if err := write_cache(options.Cache, cache_key, response.Body); err != nil {
		return nil, fmt.Errorf("cache account home API: %w", err)
	}
	return response.Body, nil
}

func html_cache_path(page_url string, scope string, account_id string) string {
	identity := strings.TrimSpace(account_id) + "\x00" + strings.TrimSpace(scope) + "\x00" + strings.TrimSpace(page_url)
	return cache_path(identity, ".html")
}

func api_cache_path(request_url string, scope string, account_id string, body []byte) string {
	identity := strings.TrimSpace(account_id) + "\x00" + strings.TrimSpace(scope) + "\x00" + strings.TrimSpace(request_url) + "\x00" + string(body)
	return cache_path(identity, ".json")
}

func cache_path(identity string, extension string) string {
	digest := sha256.Sum256([]byte(identity))
	return "home/" + hex.EncodeToString(digest[:]) + extension
}

func write_cache(file_cache *cache.CacheProvider, path string, data []byte) error {
	if file_cache == nil || !file_cache.Enabled() {
		return nil
	}
	return file_cache.Write(path, data)
}

func read_cache(file_cache *cache.CacheProvider, path string) ([]byte, error) {
	if file_cache == nil || !file_cache.Enabled() {
		return nil, nil
	}
	return file_cache.Read(path)
}

func login_page(final_url string, document_html string) bool {
	lower_url := strings.ToLower(final_url)
	if strings.Contains(lower_url, "passport.") || strings.Contains(lower_url, "/login") || strings.Contains(lower_url, "/signin") {
		return true
	}
	lower_html := strings.ToLower(document_html)
	return strings.Contains(lower_html, "<title>登录 - 微博</title>") ||
		strings.Contains(lower_html, "<title>login - weibo</title>")
}
