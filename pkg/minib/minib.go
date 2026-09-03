// Package minib provides a small browser-like HTTP session, HTML DOM, and
// persistent goja JavaScript runtime without a visual renderer.
package minib

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/cascadia"
	"github.com/dop251/goja"

	"wx_channel/pkg/clawreq"
	"wx_channel/pkg/cookies"
)

const default_timeout = 30 * time.Second

type request_header_modifier_context_key struct{}

func with_request_header_modifier(ctx context.Context, modifier func(*http.Request) error) context.Context {
	return context.WithValue(ctx, request_header_modifier_context_key{}, modifier)
}

func request_header_modifier_from_context(ctx context.Context) func(*http.Request) error {
	modifier, _ := ctx.Value(request_header_modifier_context_key{}).(func(*http.Request) error)
	return modifier
}

// MiniBrowser keeps cookies and JavaScript state across requests.
type MiniBrowser struct {
	http_client       *clawreq.Client
	cookie_jar        *cookiejar.Jar
	js_runtime        *goja.Runtime
	js_mutex          sync.Mutex
	page_runtime      *page_runtime
	resource_cache    *resource_cache
	request_scheduler request_scheduler
	timeout           time.Duration
	cookie_provider   *cookies.Reader
	lifecycle_ctx     context.Context
	lifecycle_cancel  context.CancelFunc
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
	lifecycle_ctx, lifecycle_cancel := context.WithCancel(context.Background())
	return &MiniBrowser{
		http_client:    http_client,
		cookie_jar:     cookie_jar,
		js_runtime:     goja.New(),
		resource_cache: new_resource_cache(DefaultResourceCacheLimits()),
		request_scheduler: &host_scheduler{
			per_host_concurrency: per_host_resource_concurrency,
			states:               make(map[string]*host_schedule_state),
		},
		timeout:          timeout,
		cookie_provider:  cookie_provider,
		lifecycle_ctx:    lifecycle_ctx,
		lifecycle_cancel: lifecycle_cancel,
	}, nil
}

// Request sends one request with exactly headers, plus cookies kept by this session.
func (b *MiniBrowser) Request(ctx context.Context, method, raw_url string, body io.Reader, headers http.Header) (*clawreq.Response, error) {
	if b == nil || b.http_client == nil {
		return nil, fmt.Errorf("minib: browser is closed")
	}
	release_request := func() {}
	if b.request_scheduler != nil {
		var err error
		release_request, err = b.request_scheduler.before_request(ctx, raw_url)
		if err != nil {
			return nil, err
		}
	}
	defer release_request()
	prepared_headers, err := b.prepare_request_headers(raw_url, headers)
	if err != nil {
		return nil, err
	}
	request_url, err := url.Parse(raw_url)
	if err != nil {
		return nil, err
	}
	if modifier := request_header_modifier_from_context(ctx); modifier != nil {
		request := &http.Request{Method: method, URL: request_url, Header: prepared_headers}
		if err := modifier(request); err != nil {
			return nil, fmt.Errorf("minib: modify request headers: %w", err)
		}
		prepared_headers = request.Header
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
	started_at := time.Now()
	response, err := b.http_client.Do(ctx, method, raw_url, body, clawreq.WithOnlyHeaders(request_headers))
	if recorder != nil {
		recorder.record_network(ctx, started_at, time.Since(started_at), method, raw_url, prepared_headers, request_body, response, err)
	}
	if err != nil {
		return nil, err
	}
	if b.request_scheduler != nil {
		b.request_scheduler.observe_response(raw_url, response.StatusCode, response.Header)
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

// SetResourceCacheLimits changes the in-memory cache bounds and immediately
// evicts least-recently-used entries if necessary. Zero disables an individual
// bound; negative values are rejected.
func (b *MiniBrowser) SetResourceCacheLimits(limits ResourceCacheLimits) error {
	if b == nil || b.resource_cache == nil {
		return fmt.Errorf("minib: browser is closed")
	}
	if limits.MaxEntries < 0 || limits.MaxBytes < 0 {
		return fmt.Errorf("minib: resource cache limits cannot be negative")
	}
	b.resource_cache.set_limits(limits)
	return nil
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

// Cookies returns copies of the in-memory session cookies visible to raw_url.
func (b *MiniBrowser) Cookies(raw_url string) ([]*http.Cookie, error) {
	if b == nil || b.cookie_jar == nil {
		return nil, fmt.Errorf("minib: browser is closed")
	}
	parsed_url, err := url.Parse(raw_url)
	if err != nil {
		return nil, err
	}
	if parsed_url.Scheme == "" || parsed_url.Host == "" {
		return nil, fmt.Errorf("minib: invalid cookie URL %q", raw_url)
	}
	cookies := b.cookie_jar.Cookies(parsed_url)
	result := make([]*http.Cookie, 0, len(cookies))
	for _, cookie := range cookies {
		copy_cookie := *cookie
		result = append(result, &copy_cookie)
	}
	return result, nil
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
func (b *MiniBrowser) ExecuteJS(ctx context.Context, expression string) (value goja.Value, err error) {
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
	if b.page_runtime != nil {
		b.page_runtime.ctx = ctx
		defer func() {
			b.page_runtime.ctx = b.lifecycle_ctx
		}()
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
	defer func() {
		close(finished)
		<-interrupt_done
		b.js_runtime.ClearInterrupt()
		if recovered := recover(); recovered != nil {
			value = nil
			err = javascript_panic_error(recovered)
		}
	}()
	value, err = b.js_runtime.RunString(expression)
	if err != nil {
		return nil, fmt.Errorf("minib: JavaScript failed: %w", err)
	}
	if b.page_runtime != nil && b.page_runtime.page != nil && b.page_runtime.page.Document != nil {
		b.page_runtime.pump_event_loop(ctx)
		if !b.page_runtime.use_custom_runtime {
			b.page_runtime.sync_named_elements()
			b.page_runtime.refresh_style_sheets()
			b.page_runtime.page.RenderedHTML = render_node(b.page_runtime.page.Document)
		}
	}
	return value, nil
}

// Click dispatches a browser-generated click event to the first element that
// matches selector. It does not perform layout, hit testing, or default actions.
func (b *MiniBrowser) Click(ctx context.Context, selector string) error {
	if b == nil || b.js_runtime == nil {
		return fmt.Errorf("minib: browser is closed")
	}
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return fmt.Errorf("minib: click selector cannot be empty")
	}
	if _, err := cascadia.ParseGroup(selector); err != nil {
		return fmt.Errorf("minib: invalid click selector %q: %w", selector, err)
	}
	if b.page_runtime == nil || b.page_runtime.page == nil {
		return fmt.Errorf("minib: click requires a loaded page")
	}
	if b.page_runtime.use_custom_runtime {
		return fmt.Errorf("minib: click is unavailable with a custom runtime")
	}
	current_page := b.page_runtime.page
	current_navigation_history := append([]string(nil), current_page.NavigationHistory...)
	navigation_headers := b.page_runtime.request_headers.Clone()
	encoded_selector, err := json.Marshal(selector)
	if err != nil {
		return fmt.Errorf("minib: encode click selector: %w", err)
	}
	clicked, err := b.ExecuteJS(ctx, "__minib_browser_click("+string(encoded_selector)+")")
	if err != nil {
		return fmt.Errorf("minib: click %q: %w", selector, err)
	}
	if !clicked.ToBoolean() {
		return fmt.Errorf("minib: no element matches click selector %q", selector)
	}
	if current_page.navigation_url == "" {
		return nil
	}
	current_navigation_requests := append([]string(nil), current_page.NavigationRequests...)
	navigation_headers.Set("Referer", current_page.URL)
	click_navigation_options := current_page.navigate_options
	click_navigation_options.WaitForSelector = ""
	click_navigation_options.WaitForContent = ""
	navigated_page, err := b.Navigate(ctx, current_page.navigation_url, navigation_headers, click_navigation_options)
	if err != nil {
		return fmt.Errorf("minib: follow click navigation to %q: %w", current_page.navigation_url, err)
	}
	navigated_page.NavigationHistory = append(current_navigation_history, navigated_page.NavigationHistory...)
	navigated_page.NavigationRequests = append(current_navigation_requests, navigated_page.NavigationRequests...)
	*current_page = *navigated_page
	b.page_runtime.page = current_page
	return nil
}

// Close releases the HTTP client and JavaScript context.
func (b *MiniBrowser) Close() {
	if b == nil {
		return
	}
	if b.http_client != nil {
		b.http_client.CloseIdleConnections()
	}
	if b.lifecycle_cancel != nil {
		b.lifecycle_cancel()
	}
	if b.page_runtime != nil {
		b.page_runtime.close_webassembly()
	}
	b.http_client = nil
	b.cookie_jar = nil
	b.js_runtime = nil
	b.page_runtime = nil
	b.resource_cache = nil
	b.request_scheduler = nil
	b.cookie_provider = nil
	b.lifecycle_ctx = nil
	b.lifecycle_cancel = nil
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
