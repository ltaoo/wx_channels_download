// Package minib provides a small browser-like HTTP session, HTML DOM, and
// persistent goja JavaScript runtime without a visual renderer.
package minib

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"

	"wx_channel/pkg/clawreq"
	"wx_channel/pkg/cookies"
)

const default_timeout = 30 * time.Second

// MiniBrowser keeps cookies and JavaScript state across requests.
type MiniBrowser struct {
	http_client     *clawreq.Client
	cookie_jar      *cookiejar.Jar
	js_runtime      *goja.Runtime
	js_mutex        sync.Mutex
	resource_cache  *resource_cache
	timeout         time.Duration
	cookie_provider *cookies.Reader
}

// NewMiniBrowser creates a Chrome-fingerprint session that does not follow
// redirects automatically.
func NewMiniBrowser(timeout time.Duration, cookie_providers ...*cookies.Reader) (*MiniBrowser, error) {
	if timeout <= 0 {
		timeout = default_timeout
	}
	http_client, err := clawreq.New(clawreq.Config{
		Profile: clawreq.ProfileChrome,
		Timeout: timeout,
	})
	if err != nil {
		return nil, err
	}
	cookie_jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	var cookie_provider *cookies.Reader
	if len(cookie_providers) > 0 {
		cookie_provider = cookie_providers[0]
	}
	return &MiniBrowser{
		http_client:     http_client,
		cookie_jar:      cookie_jar,
		js_runtime:      goja.New(),
		resource_cache:  new_resource_cache(),
		timeout:         timeout,
		cookie_provider: cookie_provider,
	}, nil
}

// Request sends one request with exactly headers, plus cookies kept by this session.
func (b *MiniBrowser) Request(ctx context.Context, method, raw_url string, body io.Reader, headers http.Header) (*clawreq.Response, error) {
	if b == nil || b.http_client == nil {
		return nil, fmt.Errorf("minib: browser is closed")
	}
	prepared_headers, err := b.prepare_request_headers(raw_url, headers)
	if err != nil {
		return nil, err
	}
	request_headers := make(map[string]string, len(prepared_headers))
	for name, values := range prepared_headers {
		request_headers[http.CanonicalHeaderKey(name)] = strings.Join(values, ", ")
	}
	recorder := har_recorder_from_context(ctx)
	request_body := []byte(nil)
	if recorder != nil && body != nil {
		request_body, err = io.ReadAll(body)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(request_body)
	}
	request_url, _ := url.Parse(raw_url)
	started_at := time.Now()
	response, err := b.http_client.Do(ctx, method, raw_url, body, clawreq.WithOnlyHeaders(request_headers))
	if recorder != nil {
		recorder.record_network(ctx, started_at, time.Since(started_at), method, raw_url, prepared_headers, request_body, response, err)
	}
	if err != nil {
		return nil, err
	}
	b.store_response_cookies(request_url, response)
	return response, nil
}

func (b *MiniBrowser) prepare_request_headers(raw_url string, headers http.Header) (http.Header, error) {
	request_url, err := url.Parse(raw_url)
	if err != nil {
		return nil, err
	}
	request_headers := headers.Clone()
	if request_headers == nil {
		request_headers = make(http.Header)
	}
	persistent_cookie_header, err := b.persistent_cookie_header(raw_url)
	if err != nil {
		return nil, err
	}
	if cookie_header := b.cookie_header(request_url, persistent_cookie_header, request_headers.Get("Cookie")); cookie_header != "" {
		request_headers.Set("Cookie", cookie_header)
	}
	return request_headers, nil
}

// SetCookieProvider configures cookies loaded from workdir/cookies.json for
// future requests. The provider reloads the file on every request.
func (b *MiniBrowser) SetCookieProvider(cookie_provider *cookies.Reader) {
	if b == nil {
		return
	}
	b.cookie_provider = cookie_provider
}

// Get sends a GET request.
func (b *MiniBrowser) Get(ctx context.Context, raw_url string, headers http.Header) (*clawreq.Response, error) {
	return b.Request(ctx, http.MethodGet, raw_url, nil, headers)
}

// SetCookie stores one cookie for subsequent requests.
func (b *MiniBrowser) SetCookie(raw_url string, cookie *http.Cookie) error {
	if b == nil || b.cookie_jar == nil {
		return fmt.Errorf("minib: browser is closed")
	}
	parsed_url, err := url.Parse(raw_url)
	if err != nil {
		return err
	}
	if parsed_url.Scheme == "" || parsed_url.Host == "" || cookie == nil {
		return fmt.Errorf("minib: invalid cookie URL or cookie")
	}
	b.cookie_jar.SetCookies(parsed_url, []*http.Cookie{cookie})
	return nil
}

// SetCookieHeader imports a Cookie request-header value into the session.
func (b *MiniBrowser) SetCookieHeader(raw_url, cookie_header string) error {
	request, err := http.NewRequest(http.MethodGet, raw_url, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Cookie", cookie_header)
	for _, cookie := range request.Cookies() {
		cookie.Path = "/"
		if err := b.SetCookie(raw_url, cookie); err != nil {
			return err
		}
	}
	return nil
}

// JavaScriptRuntime returns this browser's persistent goja runtime.
func (b *MiniBrowser) JavaScriptRuntime() *goja.Runtime {
	if b == nil {
		return nil
	}
	return b.js_runtime
}

// ExecuteJS evaluates JavaScript in the persistent goja runtime.
func (b *MiniBrowser) ExecuteJS(ctx context.Context, expression string) (goja.Value, error) {
	if b == nil || b.js_runtime == nil {
		return nil, fmt.Errorf("minib: browser is closed")
	}
	b.js_mutex.Lock()
	defer b.js_mutex.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}
	if _, has_deadline := ctx.Deadline(); !has_deadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, b.timeout)
		defer cancel()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	finished := make(chan struct{})
	interrupt_done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			b.js_runtime.Interrupt(ctx.Err())
		case <-finished:
		}
		close(interrupt_done)
	}()
	value, err := b.js_runtime.RunString(expression)
	close(finished)
	<-interrupt_done
	b.js_runtime.ClearInterrupt()
	if err != nil {
		return nil, fmt.Errorf("minib: JavaScript failed: %w", err)
	}
	return value, nil
}

// Close releases the HTTP client and JavaScript context.
func (b *MiniBrowser) Close() {
	if b == nil {
		return
	}
	if b.http_client != nil {
		b.http_client.CloseIdleConnections()
	}
	b.http_client = nil
	b.cookie_jar = nil
	b.js_runtime = nil
	b.resource_cache = nil
	b.cookie_provider = nil
}

func (b *MiniBrowser) cookie_header(request_url *url.URL, persistent_header string, explicit_header string) string {
	cookie_values := make(map[string]string)
	cookie_order := make([]string, 0)
	add_cookie := func(cookie *http.Cookie) {
		if cookie == nil || cookie.Name == "" {
			return
		}
		if _, exists := cookie_values[cookie.Name]; !exists {
			cookie_order = append(cookie_order, cookie.Name)
		}
		cookie_values[cookie.Name] = cookie.Value
	}
	add_cookie_header := func(cookie_header string) {
		request := &http.Request{Header: http.Header{"Cookie": []string{cookie_header}}}
		for _, cookie := range request.Cookies() {
			add_cookie(cookie)
		}
	}
	add_cookie_header(persistent_header)
	if b.cookie_jar != nil {
		for _, cookie := range b.cookie_jar.Cookies(request_url) {
			add_cookie(cookie)
		}
	}
	add_cookie_header(explicit_header)
	parts := make([]string, 0, len(cookie_order))
	for _, name := range cookie_order {
		parts = append(parts, name+"="+cookie_values[name])
	}
	return strings.Join(parts, "; ")
}

func (b *MiniBrowser) persistent_cookie_header(raw_url string) (string, error) {
	if b == nil || b.cookie_provider == nil {
		return "", nil
	}
	cookie_header, err := b.cookie_provider.HeaderForURL(raw_url)
	if errors.Is(err, cookies.ErrCookieNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("minib: read persistent cookies for %q: %w", raw_url, err)
	}
	return strings.TrimSpace(cookie_header), nil
}

func (b *MiniBrowser) store_response_cookies(request_url *url.URL, response *clawreq.Response) {
	if b.cookie_jar == nil || response == nil {
		return
	}
	response_url := request_url
	if parsed_url, err := url.Parse(response.FinalURL); err == nil && parsed_url.Host != "" {
		response_url = parsed_url
	}
	http_response := &http.Response{Header: response.Header}
	b.cookie_jar.SetCookies(response_url, http_response.Cookies())
}
