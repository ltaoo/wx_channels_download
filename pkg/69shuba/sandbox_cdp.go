package shuba69

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

const defaultSandboxCDPApplyTTL = 300

// SandboxCDPFetcher talks to the webarchive-style sandbox browser API. It asks
// the sandbox service for a CDP WebSocket ticket, then uses the same CDP fetch
// path as a direct browser endpoint.
type SandboxCDPFetcher struct {
	APIBaseURL    string
	SandboxID     string
	Timeout       time.Duration
	WaitAfterLoad time.Duration
	HTTPClient    HTTPClient
}

func NewSandboxCDPFetcher(apiBaseURL string, sandboxID string) *SandboxCDPFetcher {
	return &SandboxCDPFetcher{
		APIBaseURL:    apiBaseURL,
		SandboxID:     sandboxID,
		Timeout:       defaultCDPTimeout,
		WaitAfterLoad: defaultCDPWaitAfterLoad,
	}
}

func (f *SandboxCDPFetcher) FetchHTML(rawURL string, referer string, headers http.Header) (string, error) {
	if f == nil {
		return "", fmt.Errorf("69shuba sandbox CDP fetcher is nil")
	}
	timeout := f.Timeout
	if timeout <= 0 {
		timeout = defaultCDPTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	wsURL, err := f.cdpWebSocketURL(ctx)
	if err != nil {
		return "", err
	}
	cdpFetcher := &CDPFetcher{
		Endpoint:      wsURL,
		Timeout:       timeout,
		WaitAfterLoad: f.WaitAfterLoad,
		HTTPClient:    f.HTTPClient,
	}
	return cdpFetcher.FetchHTML(rawURL, referer, headers)
}

func (f *SandboxCDPFetcher) cdpWebSocketURL(ctx context.Context) (string, error) {
	apiBaseURL := strings.TrimSpace(f.APIBaseURL)
	if apiBaseURL == "" {
		return "", fmt.Errorf("69shuba sandbox API base URL not configured")
	}
	sandboxID := strings.TrimSpace(f.SandboxID)
	if sandboxID == "" {
		return "", fmt.Errorf("69shuba sandbox ID not configured")
	}
	base, err := url.Parse(apiBaseURL)
	if err != nil {
		return "", fmt.Errorf("parse 69shuba sandbox API base URL: %w", err)
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		return "", fmt.Errorf("69shuba sandbox API base URL must be http or https")
	}

	applyURL := *base
	applyURL.Path = path.Join(base.Path, "sandboxes", sandboxID, "cdp", "apply")
	body, _ := json.Marshal(map[string]any{
		"mode":    "page",
		"ttl_sec": defaultSandboxCDPApplyTTL,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, applyURL.String(), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("apply 69shuba sandbox CDP ticket: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("apply 69shuba sandbox CDP ticket HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var applyResp struct {
		Ticket string `json:"ticket"`
	}
	if err := json.Unmarshal(respBody, &applyResp); err != nil {
		return "", fmt.Errorf("decode 69shuba sandbox CDP ticket: %w", err)
	}
	if strings.TrimSpace(applyResp.Ticket) == "" {
		return "", fmt.Errorf("69shuba sandbox CDP ticket is empty")
	}

	wsURL := *base
	if wsURL.Scheme == "https" {
		wsURL.Scheme = "wss"
	} else {
		wsURL.Scheme = "ws"
	}
	wsURL.Path = path.Join(base.Path, "sandboxes", sandboxID, "cdp", "browser")
	query := wsURL.Query()
	query.Set("ticket", applyResp.Ticket)
	wsURL.RawQuery = query.Encode()
	return wsURL.String(), nil
}

func (f *SandboxCDPFetcher) httpClient() HTTPClient {
	if f != nil && f.HTTPClient != nil {
		return f.HTTPClient
	}
	return http.DefaultClient
}
