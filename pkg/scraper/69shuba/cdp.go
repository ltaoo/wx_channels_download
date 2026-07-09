package shuba69

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	defaultCDPEndpoint      = "http://127.0.0.1:9222"
	defaultCDPTimeout       = 30 * time.Second
	defaultCDPWaitAfterLoad = 3 * time.Second
)

// CDPFetcher fetches HTML through a running Chrome/Chromium remote debugging
// endpoint. It is optional and leaves the default HTTP client path unchanged.
type CDPFetcher struct {
	Endpoint      string
	Timeout       time.Duration
	WaitAfterLoad time.Duration
	HTTPClient    HTTPClient
}

func NewCDPFetcher(endpoint string) *CDPFetcher {
	return &CDPFetcher{
		Endpoint:      endpoint,
		Timeout:       defaultCDPTimeout,
		WaitAfterLoad: defaultCDPWaitAfterLoad,
	}
}

func (f *CDPFetcher) FetchHTML(rawURL string, referer string, headers http.Header) (string, error) {
	if f == nil {
		return "", fmt.Errorf("69shuba CDP fetcher is nil")
	}
	timeout := f.Timeout
	if timeout <= 0 {
		timeout = defaultCDPTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	wsURL, targetID, err := f.openTarget(ctx)
	if err != nil {
		return "", err
	}
	if targetID != "" {
		defer f.closeTarget(targetID)
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return "", fmt.Errorf("connect 69shuba CDP target: %w", err)
	}
	defer conn.Close()

	cdp := &cdpConn{conn: conn}
	if _, err := cdp.call(ctx, "Network.enable", nil); err != nil {
		return "", fmt.Errorf("enable CDP network: %w", err)
	}
	if _, err := cdp.call(ctx, "Page.enable", nil); err != nil {
		return "", fmt.Errorf("enable CDP page: %w", err)
	}
	if _, err := cdp.call(ctx, "Runtime.enable", nil); err != nil {
		return "", fmt.Errorf("enable CDP runtime: %w", err)
	}
	if ua := strings.TrimSpace(headers.Get("User-Agent")); ua != "" {
		if _, err := cdp.call(ctx, "Network.setUserAgentOverride", map[string]any{"userAgent": ua}); err != nil {
			return "", fmt.Errorf("set CDP user agent: %w", err)
		}
	}
	if extra := cdpExtraHeaders(headers); len(extra) > 0 {
		if _, err := cdp.call(ctx, "Network.setExtraHTTPHeaders", map[string]any{"headers": extra}); err != nil {
			return "", fmt.Errorf("set CDP headers: %w", err)
		}
	}
	for _, pair := range cdpCookiePairs(headers.Get("Cookie")) {
		if _, err := cdp.call(ctx, "Network.setCookie", map[string]any{
			"name":  pair.name,
			"value": pair.value,
			"url":   rawURL,
		}); err != nil {
			return "", fmt.Errorf("set CDP cookie %s: %w", pair.name, err)
		}
	}

	params := map[string]any{"url": rawURL}
	if strings.TrimSpace(referer) != "" {
		params["referrer"] = referer
	}
	if _, err := cdp.call(ctx, "Page.navigate", params); err != nil {
		return "", fmt.Errorf("navigate CDP target: %w", err)
	}
	_ = cdp.waitDocumentReady(ctx)
	if wait := f.waitAfterLoad(); wait > 0 {
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", ctx.Err()
		case <-timer.C:
		}
	}

	result, err := cdp.call(ctx, "Runtime.evaluate", map[string]any{
		"expression":    "document.documentElement ? document.documentElement.outerHTML : document.body.outerHTML",
		"returnByValue": true,
	})
	if err != nil {
		return "", fmt.Errorf("read CDP html: %w", err)
	}
	htmlText, ok := runtimeValue(result).(string)
	if !ok || strings.TrimSpace(htmlText) == "" {
		return "", fmt.Errorf("read CDP html: empty document")
	}
	return htmlText, nil
}

func (f *CDPFetcher) waitAfterLoad() time.Duration {
	if f == nil {
		return defaultCDPWaitAfterLoad
	}
	if f.WaitAfterLoad < 0 {
		return 0
	}
	if f.WaitAfterLoad == 0 {
		return defaultCDPWaitAfterLoad
	}
	return f.WaitAfterLoad
}

func (f *CDPFetcher) openTarget(ctx context.Context) (string, string, error) {
	endpoint := strings.TrimSpace(f.Endpoint)
	if endpoint == "" {
		endpoint = defaultCDPEndpoint
	}
	if strings.HasPrefix(endpoint, "ws://") || strings.HasPrefix(endpoint, "wss://") {
		return endpoint, "", nil
	}
	base, err := url.Parse(endpoint)
	if err != nil {
		return "", "", fmt.Errorf("parse 69shuba CDP endpoint: %w", err)
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		return "", "", fmt.Errorf("69shuba CDP endpoint must be http, https, ws, or wss")
	}
	if err := f.checkEndpoint(ctx, endpoint); err != nil {
		return "", "", err
	}
	targetURL := strings.TrimRight(endpoint, "/") + "/json/new?" + url.QueryEscape("about:blank")
	var lastErr error
	for _, method := range []string{http.MethodPut, http.MethodGet} {
		target, err := f.createTarget(ctx, method, targetURL)
		if err == nil {
			if target.WebSocketDebuggerURL == "" {
				return "", "", fmt.Errorf("69shuba CDP target missing websocket url")
			}
			return target.WebSocketDebuggerURL, target.ID, nil
		}
		lastErr = err
	}
	return "", "", lastErr
}

func (f *CDPFetcher) checkEndpoint(ctx context.Context, endpoint string) error {
	versionURL := strings.TrimRight(endpoint, "/") + "/json/version"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, versionURL, nil)
	if err != nil {
		return err
	}
	resp, err := f.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", cdpUnavailableMessage(endpoint), err)
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s; /json/version HTTP %d: %s", cdpUnavailableMessage(endpoint), resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (f *CDPFetcher) createTarget(ctx context.Context, method string, targetURL string) (cdpTarget, error) {
	req, err := http.NewRequestWithContext(ctx, method, targetURL, nil)
	if err != nil {
		return cdpTarget{}, err
	}
	resp, err := f.httpClient().Do(req)
	if err != nil {
		return cdpTarget{}, fmt.Errorf("create 69shuba CDP target: %w", err)
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return cdpTarget{}, readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return cdpTarget{}, fmt.Errorf("create 69shuba CDP target HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var target cdpTarget
	if err := json.Unmarshal(body, &target); err != nil {
		return cdpTarget{}, fmt.Errorf("decode 69shuba CDP target: %w", err)
	}
	return target, nil
}

func (f *CDPFetcher) closeTarget(targetID string) {
	if strings.TrimSpace(targetID) == "" {
		return
	}
	endpoint := strings.TrimRight(firstNonEmpty(strings.TrimSpace(f.Endpoint), defaultCDPEndpoint), "/")
	req, err := http.NewRequest(http.MethodGet, endpoint+"/json/close/"+url.PathEscape(targetID), nil)
	if err != nil {
		return
	}
	resp, err := f.httpClient().Do(req)
	if err == nil && resp != nil {
		resp.Body.Close()
	}
}

func (f *CDPFetcher) httpClient() HTTPClient {
	if f != nil && f.HTTPClient != nil {
		return f.HTTPClient
	}
	return http.DefaultClient
}

func cdpUnavailableMessage(endpoint string) string {
	return fmt.Sprintf("69shuba CDP endpoint %s is not available; create a browser sandbox before fetching. Docker webtop example: docker run -d --name wx-69shuba-cdp -p 39000:3000 -p 9222:9222 --shm-size=1g -e PUID=1000 -e PGID=1000 -e TZ=Asia/Shanghai -e RESOLUTION=1920x1080x24 -e 'CHROME_CLI=--no-sandbox --disable-dev-shm-usage --remote-debugging-address=0.0.0.0 --remote-debugging-port=9222 --user-data-dir=/config/wx-browser-profile' lscr.io/linuxserver/chromium:latest; desktop: http://127.0.0.1:39000", endpoint)
}

type cdpTarget struct {
	ID                   string `json:"id"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

type cdpConn struct {
	conn   *websocket.Conn
	nextID int
}

type cdpResponse struct {
	ID     int             `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *cdpError       `json:"error,omitempty"`
}

type cdpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (c *cdpConn) call(ctx context.Context, method string, params any) (map[string]any, error) {
	c.nextID++
	id := c.nextID
	msg := map[string]any{
		"id":     id,
		"method": method,
	}
	if params != nil {
		msg["params"] = params
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = c.conn.SetWriteDeadline(deadline)
	}
	if err := c.conn.WriteJSON(msg); err != nil {
		return nil, err
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if deadline, ok := ctx.Deadline(); ok {
			_ = c.conn.SetReadDeadline(deadline)
		}
		_, payload, err := c.conn.ReadMessage()
		if err != nil {
			return nil, err
		}
		var resp cdpResponse
		if err := json.Unmarshal(payload, &resp); err != nil {
			return nil, err
		}
		if resp.ID != id {
			continue
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("CDP %s failed: %s (%d)", method, resp.Error.Message, resp.Error.Code)
		}
		var result map[string]any
		if len(resp.Result) > 0 {
			if err := json.Unmarshal(resp.Result, &result); err != nil {
				return nil, err
			}
		}
		return result, nil
	}
}

func (c *cdpConn) waitDocumentReady(ctx context.Context) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		result, err := c.call(ctx, "Runtime.evaluate", map[string]any{
			"expression":    `["interactive","complete"].includes(document.readyState)`,
			"returnByValue": true,
		})
		if err != nil {
			return err
		}
		if ready, _ := runtimeValue(result).(bool); ready {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func runtimeValue(result map[string]any) any {
	obj, _ := result["result"].(map[string]any)
	return obj["value"]
}

func cdpExtraHeaders(headers http.Header) map[string]string {
	extra := make(map[string]string)
	for key, values := range headers {
		switch strings.ToLower(key) {
		case "cookie", "user-agent", "referer":
			continue
		}
		value := strings.TrimSpace(strings.Join(values, ", "))
		if value != "" {
			extra[key] = value
		}
	}
	return extra
}

type cdpCookiePair struct {
	name  string
	value string
}

func cdpCookiePairs(cookieHeader string) []cdpCookiePair {
	parts := strings.Split(cookieHeader, ";")
	out := make([]cdpCookiePair, 0, len(parts))
	for _, part := range parts {
		name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			continue
		}
		out = append(out, cdpCookiePair{name: name, value: strings.TrimSpace(value)})
	}
	return out
}
