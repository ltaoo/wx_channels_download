package shuba69

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"wx_channel/pkg/clawreq"
)

// ClawreqFetcher fetches 69shuba HTML through clawreq so requests use a
// browser-like TLS and header fingerprint instead of Go's standard transport.
type ClawreqFetcher struct {
	Client *clawreq.Client
	Config clawreq.Config

	mu sync.Mutex
}

func NewClawreqFetcher() *ClawreqFetcher {
	return &ClawreqFetcher{
		Config: clawreq.Config{
			Profile:         clawreq.ProfileChrome,
			FollowRedirects: true,
		},
	}
}

func (f *ClawreqFetcher) FetchHTML(rawURL string, referer string, headers http.Header) (string, error) {
	if f == nil {
		return "", fmt.Errorf("69shuba clawreq fetcher is nil")
	}
	client, err := f.client()
	if err != nil {
		return "", err
	}
	cookie := headers.Get("Cookie")
	requestReferer := firstNonEmpty(referer, headers.Get("Referer"))
	requestHeaders := clawreqHeaderMap(headers)
	options := []clawreq.RequestOption{clawreq.WithHeaders(requestHeaders)}
	if cookie != "" {
		options = append(options, clawreq.WithCookie(cookie))
	}
	if requestReferer != "" {
		options = append(options, clawreq.WithReferer(requestReferer))
	}
	resp, err := client.Get(context.Background(), rawURL, options...)
	if err != nil {
		return "", fmt.Errorf("clawreq fetch 69shuba page: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("69shuba clawreq HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(resp.Body)))
	}
	text, err := resp.Text()
	if err != nil {
		return "", fmt.Errorf("decode 69shuba clawreq response: %w", err)
	}
	return text, nil
}

func (f *ClawreqFetcher) client() (*clawreq.Client, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Client != nil {
		return f.Client, nil
	}
	config := f.Config
	if config.Profile == "" {
		config.Profile = clawreq.ProfileChrome
	}
	config.FollowRedirects = true
	client, err := clawreq.New(config)
	if err != nil {
		return nil, fmt.Errorf("create 69shuba clawreq client: %w", err)
	}
	f.Client = client
	return client, nil
}

func clawreqHeaderMap(headers http.Header) map[string]string {
	mapped := make(map[string]string, len(headers))
	for name, values := range headers {
		if len(values) == 0 || strings.EqualFold(name, "Cookie") || strings.EqualFold(name, "Referer") {
			continue
		}
		mapped[name] = strings.Join(values, ", ")
	}
	return mapped
}
