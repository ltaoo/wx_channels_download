// Package clawreq provides a small browser-fingerprint HTTP client.
//
// It uses CycleTLS for JA3/uTLS, HTTP/2, pseudo-header order, and request
// header order so page fetches do not expose Go's standard HTTP fingerprint.
package clawreq

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/transform"
)

const defaultTimeout = 30 * time.Second

const max_redirect_count = 10

const chrome112JA3 = "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513-21,29-23-24,0"

// Profile names the browser fingerprint to use for a client.
type Profile string

const (
	ProfileChrome    Profile = "chrome"
	ProfileFirefox   Profile = "firefox"
	ProfileSafari    Profile = "safari"
	ProfileSafariIOS Profile = "safari-ios"
	ProfileRandom    Profile = "random"
)

// Config controls how a Client is built.
type Config struct {
	// Profile defaults to chrome.
	//
	// The captured Chrome 151 profile under profiles/151.0.7922.109 is
	// intentionally not selectable yet. Enabling it requires upgrading this
	// module to Go 1.24.1 and replacing the current CycleTLS backend with a
	// modern tls-client version that supports Chrome 151's TLS features.
	Profile Profile
	// Timeout defaults to 30 seconds.
	Timeout time.Duration
	// ProxyURL is optional. Format: http://user:pass@host:port or socks5://host:port.
	ProxyURL string
	// FollowRedirects must be set to true to follow redirects.
	FollowRedirects bool
	// RandomTLSExtensionOrder is kept for API compatibility. CycleTLS uses the
	// selected JA3 profile order exactly.
	RandomTLSExtensionOrder bool
	// DisableIPv6 is kept for API compatibility and is not currently used by the
	// CycleTLS backend.
	DisableIPv6 bool
}

// Client is safe to reuse across requests. Cookies are kept in the jar between
// calls; each request gets its own CycleTLS transport.
type Client struct {
	cycle           cycletls.CycleTLS
	profile         Profile
	timeout         time.Duration
	proxyURL        string
	followRedirects bool
	jar             *cookiejar.Jar
}

// Response contains the fully buffered response body plus basic metadata.
type Response struct {
	StatusCode int
	Status     string
	Header     http.Header
	Body       []byte
	FinalURL   string
}

// RequestOption customizes a single request.
type RequestOption func(*requestOptions)

type requestOptions struct {
	headers http.Header
	cookie  string
	referer string
}

type browserSpec struct {
	ja3         string
	userAgent   string
	headerOrder []string
}

// New builds a browser-fingerprint HTTP client.
func New(config Config) (*Client, error) {
	profile := normalizeProfile(config.Profile)
	if profile == ProfileRandom {
		profile = randomProfile()
	}
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	return &Client{
		cycle:           cycletls.Init(),
		profile:         profile,
		timeout:         timeout,
		proxyURL:        strings.TrimSpace(config.ProxyURL),
		followRedirects: config.FollowRedirects,
		jar:             jar,
	}, nil
}

// Get sends a browser-like GET request and buffers the response body.
func (c *Client) Get(ctx context.Context, rawURL string, opts ...RequestOption) (*Response, error) {
	return c.Do(ctx, http.MethodGet, rawURL, nil, opts...)
}

// Do sends a request and buffers the response body.
func (c *Client) Do(ctx context.Context, method string, rawURL string, body io.Reader, opts ...RequestOption) (*Response, error) {
	if c == nil {
		return nil, fmt.Errorf("clawreq: nil client")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	request_url, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	request_opts := requestOptions{headers: DefaultHeaders(c.profile)}
	for _, opt := range opts {
		if opt != nil {
			opt(&request_opts)
		}
	}
	if request_opts.referer != "" {
		request_opts.headers.Set("Referer", request_opts.referer)
	}
	if request_opts.cookie != "" {
		request_opts.headers.Set("Cookie", request_opts.cookie)
	}

	body_bytes, err := readAllBody(body)
	if err != nil {
		return nil, err
	}

	request_ctx, cancel_request := context.WithTimeout(ctx, c.timeout)
	defer cancel_request()

	current_url := request_url
	current_method := strings.ToUpper(method)
	current_body := body_bytes
	current_headers := request_opts.headers.Clone()
	redirect_count := 0

	for {
		cycle_options := c.cycle_options(request_ctx, current_url, current_headers, current_body)
		cycle_response, request_err := c.doCycle(request_ctx, current_url.String(), cycle_options, current_method)
		if request_err != nil {
			return nil, request_err
		}
		if c.jar != nil && len(cycle_response.Cookies) > 0 {
			c.jar.SetCookies(current_url, cycle_response.Cookies)
		}

		response_headers := responseHeaders(cycle_response.Headers)
		response := &Response{
			StatusCode: cycle_response.Status,
			Status:     responseStatus(cycle_response.Status),
			Header:     response_headers,
			Body:       decodeBody(cycle_response.Body, response_headers),
			FinalURL:   current_url.String(),
		}
		if !c.followRedirects || !is_redirect_status(response.StatusCode) {
			return response, nil
		}

		location := strings.TrimSpace(response.Header.Get("Location"))
		if location == "" {
			return response, nil
		}
		if redirect_count >= max_redirect_count {
			return response, fmt.Errorf("clawreq: stopped after %d redirects", max_redirect_count)
		}
		next_url, resolve_err := current_url.Parse(location)
		if resolve_err != nil {
			return response, fmt.Errorf("clawreq: invalid redirect location %q: %w", location, resolve_err)
		}
		if next_url.Scheme != "http" && next_url.Scheme != "https" {
			return response, fmt.Errorf("clawreq: unsupported redirect scheme %q", next_url.Scheme)
		}

		current_headers = redirect_headers(current_headers, current_url, next_url)
		current_method, current_body = redirect_request(current_method, current_body, response.StatusCode)
		current_url = next_url
		redirect_count++
	}
}

func (c *Client) cycle_options(ctx context.Context, request_url *url.URL, headers http.Header, body []byte) cycletls.Options {
	browser_spec := resolveBrowserSpec(c.profile)
	user_agent := browser_spec.userAgent
	if header_user_agent := strings.TrimSpace(headers.Get("User-Agent")); header_user_agent != "" {
		user_agent = header_user_agent
	}
	return cycletls.Options{
		Ja3:       browser_spec.ja3,
		UserAgent: user_agent,
		Headers:   headerMap(headers),
		Body:      string(body),
		Proxy:     c.proxyURL,
		Cookies:   c.cookiesFor(request_url, headers.Get("Cookie") == ""),
		Timeout:   timeoutSeconds(timeoutForContext(ctx, c.timeout)),
		// CycleTLS stores Host as a regular header, so its automatic redirect
		// path can reuse the previous host on a cross-host redirect. Always stop
		// after one hop and let Client.Do rebuild the next request instead.
		DisableRedirect: true,
		HeaderOrder:     browser_spec.headerOrder,
	}
}

func is_redirect_status(status_code int) bool {
	switch status_code {
	case http.StatusMovedPermanently,
		http.StatusFound,
		http.StatusSeeOther,
		http.StatusTemporaryRedirect,
		http.StatusPermanentRedirect:
		return true
	default:
		return false
	}
}

func redirect_request(method string, body []byte, status_code int) (string, []byte) {
	switch status_code {
	case http.StatusMovedPermanently, http.StatusFound:
		if method != http.MethodGet && method != http.MethodHead {
			return http.MethodGet, nil
		}
	case http.StatusSeeOther:
		if method != http.MethodHead {
			return http.MethodGet, nil
		}
	}
	return method, body
}

func redirect_headers(headers http.Header, from_url *url.URL, to_url *url.URL) http.Header {
	redirected_headers := headers.Clone()
	if should_forward_sensitive_headers(from_url, to_url) {
		return redirected_headers
	}
	redirected_headers.Del("Authorization")
	redirected_headers.Del("Www-Authenticate")
	redirected_headers.Del("Cookie")
	return redirected_headers
}

func should_forward_sensitive_headers(from_url *url.URL, to_url *url.URL) bool {
	if from_url == nil || to_url == nil {
		return false
	}
	from_host := strings.TrimSuffix(strings.ToLower(from_url.Hostname()), ".")
	to_host := strings.TrimSuffix(strings.ToLower(to_url.Hostname()), ".")
	if from_host == "" || to_host == "" {
		return false
	}
	return to_host == from_host || strings.HasSuffix(to_host, "."+from_host)
}

// CloseIdleConnections is present for API symmetry. CycleTLS creates and closes
// its fhttp transport per request in Do.
func (c *Client) CloseIdleConnections() {}

// WithHeader sets or replaces a request header.
func WithHeader(name string, value string) RequestOption {
	return func(opts *requestOptions) {
		if opts.headers == nil {
			opts.headers = http.Header{}
		}
		opts.headers.Set(name, value)
	}
}

// WithHeaders sets or replaces request headers.
func WithHeaders(headers map[string]string) RequestOption {
	return func(opts *requestOptions) {
		for name, value := range headers {
			WithHeader(name, value)(opts)
		}
	}
}

// WithOnlyHeaders replaces the default browser headers with exactly the
// caller-provided headers.
func WithOnlyHeaders(headers map[string]string) RequestOption {
	return func(opts *requestOptions) {
		opts.headers = http.Header{}
		for name, value := range headers {
			opts.headers.Set(name, value)
		}
	}
}

// WithCookie sets the Cookie header for a single request.
func WithCookie(cookie string) RequestOption {
	return func(opts *requestOptions) {
		opts.cookie = strings.TrimSpace(cookie)
	}
}

// WithReferer sets the Referer header for a single request.
func WithReferer(referer string) RequestOption {
	return func(opts *requestOptions) {
		opts.referer = strings.TrimSpace(referer)
	}
}

// DecodeText decodes an HTML/text response using the response Content-Type and
// in-document charset hints. It handles pages like 69shuba that declare GBK in
// a <meta charset> tag.
func DecodeText(body []byte, content_type string) (string, error) {
	if utf8.Valid(body) {
		return string(body), nil
	}
	encoding, _, _ := charset.DetermineEncoding(body, content_type)
	reader := transform.NewReader(bytes.NewReader(body), encoding.NewDecoder())
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

// ContentType returns the response Content-Type header.
func (r *Response) ContentType() string {
	if r == nil {
		return ""
	}
	return r.Header.Get("Content-Type")
}

// Text decodes the response body as text.
func (r *Response) Text() (string, error) {
	if r == nil {
		return "", fmt.Errorf("clawreq: nil response")
	}
	return DecodeText(r.Body, r.ContentType())
}

// DefaultHeaders returns browser navigation headers.
func DefaultHeaders(profile Profile) http.Header {
	spec := resolveBrowserSpec(profile)
	switch profile {
	case ProfileFirefox:
		return http.Header{
			"User-Agent":                {spec.userAgent},
			"Accept":                    {"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"},
			"Accept-Language":           {"en-US,en;q=0.5"},
			"Accept-Encoding":           {"gzip, deflate, br"},
			"Upgrade-Insecure-Requests": {"1"},
			"Sec-Fetch-Dest":            {"document"},
			"Sec-Fetch-Mode":            {"navigate"},
			"Sec-Fetch-Site":            {"none"},
			"Sec-Fetch-User":            {"?1"},
		}
	case ProfileSafari, ProfileSafariIOS:
		return http.Header{
			"Accept":                    {"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"},
			"Accept-Language":           {"en-US,en;q=0.9"},
			"Accept-Encoding":           {"gzip, deflate, br"},
			"User-Agent":                {spec.userAgent},
			"Upgrade-Insecure-Requests": {"1"},
		}
	default:
		return http.Header{
			"Sec-Ch-Ua":                 {`"Chromium";v="112", "Google Chrome";v="112", "Not:A-Brand";v="99"`},
			"Sec-Ch-Ua-Mobile":          {"?0"},
			"Sec-Ch-Ua-Platform":        {`"macOS"`},
			"Upgrade-Insecure-Requests": {"1"},
			"User-Agent":                {spec.userAgent},
			"Accept":                    {"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7"},
			"Sec-Fetch-Site":            {"none"},
			"Sec-Fetch-Mode":            {"navigate"},
			"Sec-Fetch-User":            {"?1"},
			"Sec-Fetch-Dest":            {"document"},
			"Accept-Encoding":           {"gzip, deflate, br"},
			"Accept-Language":           {"en-US,en;q=0.9"},
		}
	}
}

func (c *Client) doCycle(ctx context.Context, rawURL string, options cycletls.Options, method string) (cycletls.Response, error) {
	type result struct {
		resp cycletls.Response
		err  error
	}
	resultCh := make(chan result, 1)
	go func() {
		resp, err := c.cycle.Do(rawURL, options, method)
		resultCh <- result{resp: resp, err: err}
	}()

	select {
	case <-ctx.Done():
		return cycletls.Response{}, ctx.Err()
	case result := <-resultCh:
		return result.resp, result.err
	}
}

func (c *Client) cookiesFor(requestURL *url.URL, includeJar bool) []cycletls.Cookie {
	if c == nil || c.jar == nil || requestURL == nil || !includeJar {
		return nil
	}
	cookies := c.jar.Cookies(requestURL)
	converted := make([]cycletls.Cookie, 0, len(cookies))
	for _, cookie := range cookies {
		converted = append(converted, cycletls.Cookie{
			Name:     cookie.Name,
			Value:    cookie.Value,
			Path:     cookie.Path,
			Domain:   cookie.Domain,
			Expires:  cookie.Expires,
			MaxAge:   cookie.MaxAge,
			Secure:   cookie.Secure,
			HTTPOnly: cookie.HttpOnly,
			SameSite: cookie.SameSite,
			Raw:      cookie.Raw,
			Unparsed: cookie.Unparsed,
		})
	}
	return converted
}

func resolveBrowserSpec(profile Profile) browserSpec {
	switch profile {
	case ProfileFirefox:
		return browserSpec{
			ja3:       chrome112JA3,
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:110.0) Gecko/20100101 Firefox/110.0",
			headerOrder: []string{
				"user-agent",
				"accept",
				"accept-language",
				"accept-encoding",
				"upgrade-insecure-requests",
				"sec-fetch-dest",
				"sec-fetch-mode",
				"sec-fetch-site",
				"sec-fetch-user",
			},
		}
	case ProfileSafari:
		return browserSpec{
			ja3:       chrome112JA3,
			userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Safari/605.1.15",
			headerOrder: []string{
				"accept",
				"accept-language",
				"accept-encoding",
				"user-agent",
				"upgrade-insecure-requests",
			},
		}
	case ProfileSafariIOS:
		return browserSpec{
			ja3:       chrome112JA3,
			userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1",
			headerOrder: []string{
				"accept",
				"accept-language",
				"accept-encoding",
				"user-agent",
				"upgrade-insecure-requests",
			},
		}
	default:
		return browserSpec{
			ja3:       chrome112JA3,
			userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36",
			headerOrder: []string{
				"sec-ch-ua",
				"sec-ch-ua-mobile",
				"sec-ch-ua-platform",
				"upgrade-insecure-requests",
				"user-agent",
				"accept",
				"sec-fetch-site",
				"sec-fetch-mode",
				"sec-fetch-user",
				"sec-fetch-dest",
				"accept-encoding",
				"accept-language",
				"cookie",
			},
		}
	}
}

func headerMap(headers http.Header) map[string]string {
	mapped := make(map[string]string, len(headers))
	for name, values := range headers {
		if len(values) == 0 {
			continue
		}
		mapped[strings.ToLower(name)] = strings.Join(values, ", ")
	}
	return mapped
}

func responseHeaders(headers map[string]string) http.Header {
	converted := http.Header{}
	for name, value := range headers {
		if strings.EqualFold(name, "Set-Cookie") {
			for _, cookie := range strings.Split(value, "/,/") {
				converted.Add(name, cookie)
			}
			continue
		}
		converted.Set(name, value)
	}
	return converted
}

// decodeBody decodes the CycleTLS response body. CycleTLS base64-encodes
// binary response bodies (image, video, audio, etc.) because its Body field
// is a string. For text responses the string is used as-is.
func decodeBody(body string, headers http.Header) []byte {
	if body == "" {
		return nil
	}
	contentType := headers.Get("Content-Type")
	if !isBinaryContentType(contentType) {
		return []byte(body)
	}
	trimmed := strings.TrimSpace(body)
	decoded, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		// Not valid base64 — return raw (shouldn't happen for binary, but be safe)
		return []byte(body)
	}
	return decoded
}

// isBinaryContentType returns true for content types that are not text-based.
func isBinaryContentType(contentType string) bool {
	if contentType == "" {
		return false
	}
	mediaType, _, _ := strings.Cut(contentType, ";")
	mediaType = strings.TrimSpace(mediaType)
	// text/* is always text
	if strings.HasPrefix(mediaType, "text/") {
		return false
	}
	// Common text-based application types
	switch mediaType {
	case "application/json", "application/xml", "application/javascript",
		"application/x-www-form-urlencoded", "application/xhtml+xml":
		return false
	}
	// Everything else (image/*, video/*, audio/*, application/octet-stream, etc.)
	return true
}

func readAllBody(body io.Reader) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	return io.ReadAll(body)
}

func responseStatus(statusCode int) string {
	if statusCode <= 0 {
		return ""
	}
	statusText := http.StatusText(statusCode)
	if statusText == "" {
		return fmt.Sprintf("%d", statusCode)
	}
	return fmt.Sprintf("%d %s", statusCode, statusText)
}

func timeoutForContext(ctx context.Context, fallback time.Duration) time.Duration {
	if ctx == nil {
		return fallback
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return fallback
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return time.Millisecond
	}
	if fallback <= 0 || remaining < fallback {
		return remaining
	}
	return fallback
}

func timeoutSeconds(timeout time.Duration) int {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	seconds := int((timeout + time.Second - 1) / time.Second)
	if seconds < 1 {
		return 1
	}
	return seconds
}

func normalizeProfile(profile Profile) Profile {
	switch profile {
	case ProfileFirefox, ProfileSafari, ProfileSafariIOS, ProfileRandom:
		return profile
	case "":
		return ProfileChrome
	default:
		return ProfileChrome
	}
}

func randomProfile() Profile {
	profiles := []Profile{ProfileChrome, ProfileFirefox, ProfileSafari}
	return profiles[rand.New(rand.NewSource(time.Now().UnixNano())).Intn(len(profiles))]
}
