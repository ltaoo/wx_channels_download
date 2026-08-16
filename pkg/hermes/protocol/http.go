package protocol

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"wx_channel/pkg/clawreq"
	"wx_channel/pkg/hermes"
)

// HTTPDriver is the HTTP/HTTPS protocol driver that uses clawreq (CycleTLS) to
// provide Chrome 112 JA3/uTLS fingerprints, avoiding detection as a Go HTTP client.
// Uses a client pool to support concurrent HTTP Range requests, eliminating serial
// bottlenecks caused by single-client mutex locks.
type HTTPDriver struct {
	mu        sync.Mutex
	pools     map[string]chan *clawreq.Client
	std_https map[string]*http.Client
}

const http_driver_pool_size = 3

// NewHTTPDriver creates a new HTTP protocol driver instance.
func NewHTTPDriver() *HTTPDriver {
	return &HTTPDriver{
		pools:     make(map[string]chan *clawreq.Client),
		std_https: make(map[string]*http.Client),
	}
}

// Protocols returns the protocol identifiers supported by this driver.
func (d *HTTPDriver) Protocols() []string { return []string{"http", "https"} }

// prepareProbeSize is the number of bytes downloaded during probing, used for Range capability detection and magic bytes type detection.
const prepare_probe_size = 512

// Prepare probes the resource for size, Range capability, Content-Type, and first 512 bytes (for magic bytes detection).
func (d *HTTPDriver) Prepare(ctx context.Context, endpoint hermes.Endpoint) (hermes.PreparedResource, error) {
	resp, err := d.do(ctx, endpoint, clawreq.WithHeader("Range", fmt.Sprintf("bytes=0-%d", prepare_probe_size-1)))
	if err != nil {
		return hermes.PreparedResource{}, err
	}

	prepared := hermes.PreparedResource{
		ContentType: resp.Header.Get("Content-Type"),
		ProbeData:   resp.Body,
	}
	if resp.StatusCode == http.StatusPartialContent {
		start, end, total, ok := parse_content_range(resp.Header.Get("Content-Range"))
		if !ok || start != 0 {
			return hermes.PreparedResource{}, errors.New("server returned an invalid Content-Range")
		}
		_ = end // end depends on actual bytes returned, valid range 0..prepareProbeSize-1
		prepared.Size = total
		prepared.SupportsRange = true
		return prepared, nil
	}
	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		var total int64
		if _, scan_err := fmt.Sscanf(strings.TrimSpace(resp.Header.Get("Content-Range")), "bytes */%d", &total); scan_err == nil && total == 0 {
			return prepared, nil
		}
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if cl := resp.Header.Get("Content-Length"); cl != "" {
			if size, err := strconv.ParseInt(cl, 10, 64); err == nil && size >= 0 {
				prepared.Size = size
			}
		}
		return prepared, nil
	}
	return hermes.PreparedResource{}, probe_response_status_error(resp.StatusCode, resp.Header, len(resp.Body))
}

func probe_response_status_error(status_code int, headers http.Header, body_bytes int) error {
	detail := fmt.Sprintf(
		"response content-type=%q content-length=%q content-range=%q server=%q body-bytes=%d",
		headers.Get("Content-Type"),
		headers.Get("Content-Length"),
		headers.Get("Content-Range"),
		headers.Get("Server"),
		body_bytes,
	)
	if status_code == http.StatusUnauthorized || status_code == http.StatusForbidden {
		return fmt.Errorf("resource probe rejected with HTTP status %d: endpoint authorization, signed URL, access token, or request challenge may be missing, invalid, or expired (%s)", status_code, detail)
	}
	return fmt.Errorf("resource probe returned HTTP status %d (%s)", status_code, detail)
}

// Open downloads a resource according to ReadRequest and returns a readable data stream.
// For Range requests (segmented downloads), uses the net/http standard library directly to
// return resp.Body for streaming reads, enabling real-time progress events in the download loop.
// For non-Range requests, still uses clawreq to preserve browser fingerprint.
func (d *HTTPDriver) Open(ctx context.Context, endpoint hermes.Endpoint, request hermes.ReadRequest) (io.ReadCloser, error) {
	if request.UseRange {
		return d.open_range(ctx, endpoint, request)
	}
	return d.open_full(ctx, endpoint)
}

// openRange uses the net/http standard library for Range requests, returning streaming resp.Body.
// WeChat CDN Range requests do not require browser fingerprint masking; the standard library suffices for real-time progress.
func (d *HTTPDriver) open_range(ctx context.Context, endpoint hermes.Endpoint, request hermes.ReadRequest) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", request.OffsetStart, request.OffsetEnd))
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Accept", "*/*")
	for key, value := range endpoint.Headers {
		req.Header.Set(key, value)
	}
	if endpoint.Cookies != "" {
		req.Header.Set("Cookie", endpoint.Cookies)
	}

	proxy_url, err := endpoint.ProxyServer.URL()
	if err != nil {
		return nil, err
	}
	client, err := d.standard_http_client(proxy_url)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	// Some CDNs return 200 instead of 206 when the requested range starts at
	// zero and spans the complete representation. The body is still safe to use
	// as the initial streaming download when its length matches the request.
	if full_range_response_matches_request(resp, request) {
		return resp.Body, nil
	}
	if resp.StatusCode != http.StatusPartialContent {
		resp.Body.Close()
		return nil, range_response_status_error(resp.StatusCode)
	}
	start, end, _, ok := parse_content_range(resp.Header.Get("Content-Range"))
	if !ok || start != request.OffsetStart || end > request.OffsetEnd {
		resp.Body.Close()
		return nil, errors.New("server returned a Content-Range that does not match the request")
	}

	return resp.Body, nil
}

func range_response_status_error(status_code int) error {
	switch status_code {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("range request rejected with HTTP status %d: endpoint authorization, signed URL, or access token may be missing, invalid, or expired", status_code)
	case http.StatusRequestedRangeNotSatisfiable:
		return fmt.Errorf("range request rejected with HTTP status %d: requested byte range is not satisfiable", status_code)
	case http.StatusOK:
		return errors.New("server ignored the requested byte range and returned HTTP status 200")
	default:
		return fmt.Errorf("range request returned unexpected HTTP status %d", status_code)
	}
}

func full_range_response_matches_request(resp *http.Response, request hermes.ReadRequest) bool {
	if resp == nil || resp.StatusCode != http.StatusOK || request.OffsetStart != 0 || request.OffsetEnd < 0 {
		return false
	}
	return resp.ContentLength == request.OffsetEnd+1
}

// openFull downloads the full resource via clawreq (non-Range) to preserve browser fingerprint.
func (d *HTTPDriver) open_full(ctx context.Context, endpoint hermes.Endpoint) (io.ReadCloser, error) {
	resp, err := d.do(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("server returned error status code: %d", resp.StatusCode)
	}
	return io.NopCloser(bytes.NewReader(resp.Body)), nil
}

// do sends a GET request. Overrides the default Accept header (removes image/webp)
// to prevent servers like WeChat CDN from transcoding images to webp format.
// Uses a client pool for concurrent HTTP requests; each goroutine gets an independent client from the pool.
func (d *HTTPDriver) do(ctx context.Context, endpoint hermes.Endpoint, opts ...clawreq.RequestOption) (*clawreq.Response, error) {
	all_opts := make([]clawreq.RequestOption, 0, len(opts)+len(endpoint.Headers)+3)
	all_opts = append(all_opts, clawreq.WithHeader("Accept-Encoding", "identity"))
	all_opts = append(all_opts, clawreq.WithHeader("Accept", "*/*"))
	for key, value := range endpoint.Headers {
		all_opts = append(all_opts, clawreq.WithHeader(key, value))
	}
	if endpoint.Cookies != "" {
		all_opts = append(all_opts, clawreq.WithCookie(endpoint.Cookies))
	}
	all_opts = append(all_opts, opts...)

	proxy_url, err := endpoint.ProxyServer.URL()
	if err != nil {
		return nil, err
	}
	pool, err := d.browser_client_pool(proxy_url)
	if err != nil {
		return nil, err
	}
	client := <-pool
	defer func() { pool <- client }()
	return client.Do(ctx, "GET", endpoint.URL, nil, all_opts...)
}

func (d *HTTPDriver) browser_client_pool(raw_proxy_url string) (chan *clawreq.Client, error) {
	proxy_url, _, err := normalize_proxy_url(raw_proxy_url)
	if err != nil {
		return nil, err
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if pool := d.pools[proxy_url]; pool != nil {
		return pool, nil
	}

	pool := make(chan *clawreq.Client, http_driver_pool_size)
	for i := 0; i < http_driver_pool_size; i++ {
		client, err := clawreq.New(clawreq.Config{
			Profile:         clawreq.ProfileChrome,
			ProxyURL:        proxy_url,
			FollowRedirects: true,
		})
		if err != nil {
			return nil, fmt.Errorf("create HTTP client: %w", err)
		}
		pool <- client
	}
	d.pools[proxy_url] = pool
	return pool, nil
}

func (d *HTTPDriver) standard_http_client(raw_proxy_url string) (*http.Client, error) {
	proxy_url, parsed_proxy_url, err := normalize_proxy_url(raw_proxy_url)
	if err != nil {
		return nil, err
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if client := d.std_https[proxy_url]; client != nil {
		return client, nil
	}

	transport := &http.Transport{
		ForceAttemptHTTP2: true,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 32,
		MaxConnsPerHost:     0, // no limit, let MaxIdleConnsPerHost govern reuse
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	if parsed_proxy_url != nil {
		transport.Proxy = http.ProxyURL(parsed_proxy_url)
	}
	client := &http.Client{Timeout: 300 * time.Second, Transport: transport}
	d.std_https[proxy_url] = client
	return client, nil
}

func normalize_proxy_url(raw_proxy_url string) (string, *url.URL, error) {
	raw_proxy_url = strings.TrimSpace(raw_proxy_url)
	if raw_proxy_url == "" {
		return "", nil, nil
	}

	proxy_url, err := url.Parse(raw_proxy_url)
	if err != nil || proxy_url.Host == "" || proxy_url.Hostname() == "" {
		return "", nil, errors.New("invalid proxy URL")
	}
	proxy_url.Scheme = strings.ToLower(proxy_url.Scheme)
	switch proxy_url.Scheme {
	case "http", "https", "socks5":
	default:
		return "", nil, fmt.Errorf("unsupported proxy scheme %q", proxy_url.Scheme)
	}
	return proxy_url.String(), proxy_url, nil
}

func parse_content_range(value string) (start, end, total int64, ok bool) {
	if _, err := fmt.Sscanf(strings.TrimSpace(value), "bytes %d-%d/%d", &start, &end, &total); err != nil {
		return 0, 0, 0, false
	}
	if start < 0 || end < start || total <= end {
		return 0, 0, 0, false
	}
	return start, end, total, true
}
