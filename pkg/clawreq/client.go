// Package clawreq provides a small browser-fingerprint HTTP client.
//
// It uses CycleTLS for JA3/uTLS, HTTP/2, pseudo-header order, and request
// header order so page fetches do not expose Go's standard HTTP fingerprint.
package clawreq

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/transform"
)

const defaultTimeout = 30 * time.Second

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

	requestURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	requestOpts := requestOptions{headers: DefaultHeaders(c.profile)}
	for _, opt := range opts {
		if opt != nil {
			opt(&requestOpts)
		}
	}
	if requestOpts.referer != "" {
		requestOpts.headers.Set("Referer", requestOpts.referer)
	}
	if requestOpts.cookie != "" {
		requestOpts.headers.Set("Cookie", requestOpts.cookie)
	}

	bodyBytes, err := readAllBody(body)
	if err != nil {
		return nil, err
	}
	spec := resolveBrowserSpec(c.profile)
	userAgent := spec.userAgent
	if headerUserAgent := strings.TrimSpace(requestOpts.headers.Get("User-Agent")); headerUserAgent != "" {
		userAgent = headerUserAgent
	}
	options := cycletls.Options{
		Ja3:             spec.ja3,
		UserAgent:       userAgent,
		Headers:         headerMap(requestOpts.headers),
		Body:            string(bodyBytes),
		Proxy:           c.proxyURL,
		Cookies:         c.cookiesFor(requestURL, requestOpts.cookie == ""),
		Timeout:         timeoutSeconds(timeoutForContext(ctx, c.timeout)),
		DisableRedirect: !c.followRedirects,
		HeaderOrder:     spec.headerOrder,
	}

	cycleResp, err := c.doCycle(ctx, rawURL, options, method)
	if err != nil {
		return nil, err
	}
	if c.jar != nil && len(cycleResp.Cookies) > 0 {
		c.jar.SetCookies(requestURL, cycleResp.Cookies)
	}

	return &Response{
		StatusCode: cycleResp.Status,
		Status:     responseStatus(cycleResp.Status),
		Header:     responseHeaders(cycleResp.Headers),
		Body:       []byte(cycleResp.Body),
		FinalURL:   cycleResp.FinalUrl,
	}, nil
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
func DecodeText(body []byte, contentType string) (string, error) {
	encoding, _, _ := charset.DetermineEncoding(body, contentType)
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
