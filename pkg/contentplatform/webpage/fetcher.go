package webpage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html/charset"
)

const (
	defaultFetchTimeout = 15 * time.Second
	defaultMaxHTMLBytes = 10 << 20
	defaultUserAgent    = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"
)

type PageFetcher interface {
	FetchPage(ctx context.Context, rawURL string) (*FetchedPage, error)
}

type HTTPFetcher struct {
	Client    *http.Client
	MaxBytes  int64
	UserAgent string
}

func (f *HTTPFetcher) FetchPage(ctx context.Context, rawURL string) (*FetchedPage, error) {
	parsed, err := parseHTTPURL(rawURL)
	if err != nil {
		return nil, err
	}
	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: defaultFetchTimeout}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html, application/xhtml+xml;q=0.9, */*;q=0.8")
	req.Header.Set("User-Agent", firstNonEmpty(f.UserAgent, defaultUserAgent))

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, fmt.Errorf("fetch webpage: status %d", resp.StatusCode)
	}

	maxBytes := f.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxHTMLBytes
	}
	limited := io.LimitReader(resp.Body, maxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("fetch webpage: html exceeds %d bytes", maxBytes)
	}

	contentType := resp.Header.Get("Content-Type")
	reader, err := charset.NewReader(bytes.NewReader(body), contentType)
	if err != nil {
		reader = bytes.NewReader(body)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	htmlText := string(decoded)
	if !looksLikeHTML(contentType, htmlText) {
		return nil, fmt.Errorf("fetch webpage: response is not html")
	}
	finalURL := parsed.String()
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	return &FetchedPage{
		URL:         finalURL,
		StatusCode:  resp.StatusCode,
		ContentType: contentType,
		HTML:        htmlText,
	}, nil
}

func parseHTTPURL(rawURL string) (*url.URL, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("empty url")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed == nil || parsed.Host == "" {
		return nil, fmt.Errorf("invalid url")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported url scheme %q", parsed.Scheme)
	}
	return parsed, nil
}

func looksLikeHTML(contentType string, body string) bool {
	ct := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if ct == "" || ct == "text/html" || ct == "application/xhtml+xml" {
		return true
	}
	head := strings.ToLower(body)
	if len(head) > 4096 {
		head = head[:4096]
	}
	return strings.Contains(head, "<html") ||
		strings.Contains(head, "<!doctype html") ||
		strings.Contains(head, "<article") ||
		strings.Contains(head, "<main")
}
