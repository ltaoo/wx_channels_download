// Package singlefile captures the HTML DOM after minib navigation completes.
package singlefile

import (
	"context"
	"fmt"
	"mime"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"wx_channel/pkg/cookies"
	"wx_channel/pkg/minib"
)

const PlatformID = "singlefile"

type Page struct {
	URL          string `json:"url"`
	RequestedURL string `json:"requested_url"`
	Title        string `json:"title"`
	HTML         string `json:"html"`
}

type Client struct {
	cookie_provider *cookies.Reader
}

func NewClient(cookie_provider *cookies.Reader) *Client {
	return &Client{cookie_provider: cookie_provider}
}

func (c *Client) Fetch(raw_url string) (*Page, error) {
	return c.FetchContext(context.Background(), raw_url)
}

func (c *Client) FetchContext(ctx context.Context, raw_url string) (*Page, error) {
	request_url, err := ParseURL(raw_url)
	if err != nil {
		return nil, err
	}
	browser, err := minib.NewMiniBrowser(30*time.Second, c.cookie_provider)
	if err != nil {
		return nil, err
	}
	defer browser.Close()
	page, err := browser.Navigate(ctx, request_url.String(), nil, minib.NavigateOptions{
		WaitUntil:         minib.WaitUntilLoad,
		ResourceTimeout:   5 * time.Second,
		JavaScriptTimeout: 2 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("singlefile navigation: %w", err)
	}
	if page.StatusCode < 200 || page.StatusCode >= 300 {
		return nil, fmt.Errorf("singlefile: HTTP %d", page.StatusCode)
	}
	if page.ContentType != "" {
		media_type, _, err := mime.ParseMediaType(page.ContentType)
		if err != nil || (media_type != "text/html" && media_type != "application/xhtml+xml") {
			return nil, fmt.Errorf("singlefile: expected HTML, got %q", page.ContentType)
		}
	}
	document := goquery.NewDocumentFromNode(page.Document)
	title := strings.TrimSpace(document.Find("title").First().Text())
	if title == "" {
		title = request_url.Hostname()
	}
	return &Page{URL: page.URL, RequestedURL: request_url.String(), Title: title, HTML: page.RenderedHTML}, nil
}

// ParseURL validates page and asset URLs at the network boundary.
func ParseURL(raw_url string) (*url.URL, error) {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil || parsed_url.Hostname() == "" || (parsed_url.Scheme != "http" && parsed_url.Scheme != "https") {
		return nil, fmt.Errorf("singlefile: invalid HTTP/HTTPS URL %q", raw_url)
	}
	return parsed_url, nil
}
