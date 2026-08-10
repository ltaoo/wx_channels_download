package hermes

import (
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
)

const default_http_probe_size = 512

// defaultHTTPDriver is the dependency-free HTTP/HTTPS driver used by the
// high-level API. Applications may replace it with RegisterProtocol.
type default_http_driver struct {
	mu      sync.Mutex
	clients map[string]*http.Client
}

func new_default_http_driver() *default_http_driver {
	return &default_http_driver{clients: make(map[string]*http.Client)}
}

func (d *default_http_driver) Protocols() []string {
	return []string{"http", "https"}
}

func (d *default_http_driver) Prepare(ctx context.Context, endpoint Endpoint) (PreparedResource, error) {
	req, err := d.new_request(ctx, endpoint, ReadRequest{
		OffsetStart: 0,
		OffsetEnd:   default_http_probe_size - 1,
		UseRange:    true,
	})
	if err != nil {
		return PreparedResource{}, err
	}
	resp, err := d.do(req, endpoint.ProxyServer)
	if err != nil {
		return PreparedResource{}, err
	}
	defer resp.Body.Close()

	probe, err := io.ReadAll(io.LimitReader(resp.Body, default_http_probe_size))
	if err != nil {
		return PreparedResource{}, fmt.Errorf("read HTTP probe: %w", err)
	}
	prepared := PreparedResource{
		ContentType: resp.Header.Get("Content-Type"),
		ProbeData:   probe,
	}

	if resp.StatusCode == http.StatusPartialContent {
		start, _, total, ok := parse_default_http_content_range(resp.Header.Get("Content-Range"))
		if !ok || start != 0 {
			return PreparedResource{}, errors.New("server returned an invalid Content-Range")
		}
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
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return PreparedResource{}, fmt.Errorf("resource probe returned status code %d", resp.StatusCode)
	}
	if value := resp.Header.Get("Content-Length"); value != "" {
		if size, parse_err := strconv.ParseInt(value, 10, 64); parse_err == nil && size >= 0 {
			prepared.Size = size
		}
	}
	return prepared, nil
}

func (d *default_http_driver) Open(ctx context.Context, endpoint Endpoint, request ReadRequest) (io.ReadCloser, error) {
	req, err := d.new_request(ctx, endpoint, request)
	if err != nil {
		return nil, err
	}
	resp, err := d.do(req, endpoint.ProxyServer)
	if err != nil {
		return nil, err
	}

	if request.UseRange {
		// A 200 response is valid for a range that covers the complete object.
		// Accept it only when the response length proves that no partial-range
		// semantics are being lost.
		if full_range_response_matches_request(resp, request) {
			return resp.Body, nil
		}
		if resp.StatusCode != http.StatusPartialContent {
			resp.Body.Close()
			return nil, fmt.Errorf("server does not support the requested range, status code %d", resp.StatusCode)
		}
		start, end, _, ok := parse_default_http_content_range(resp.Header.Get("Content-Range"))
		if !ok || start != request.OffsetStart || end > request.OffsetEnd {
			resp.Body.Close()
			return nil, errors.New("server returned a Content-Range that does not match the request")
		}
		return resp.Body, nil
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		resp.Body.Close()
		return nil, fmt.Errorf("server returned error status code: %d", resp.StatusCode)
	}
	return resp.Body, nil
}

func full_range_response_matches_request(resp *http.Response, request ReadRequest) bool {
	if resp == nil || resp.StatusCode != http.StatusOK || request.OffsetStart != 0 || request.OffsetEnd < 0 {
		return false
	}
	return resp.ContentLength == request.OffsetEnd+1
}

func (d *default_http_driver) new_request(ctx context.Context, endpoint Endpoint, request ReadRequest) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Accept", "*/*")
	if request.UseRange {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", request.OffsetStart, request.OffsetEnd))
	}
	for key, value := range endpoint.Headers {
		req.Header.Set(key, value)
	}
	if endpoint.Cookies != "" {
		req.Header.Set("Cookie", endpoint.Cookies)
	}
	return req, nil
}

func (d *default_http_driver) do(req *http.Request, proxy_server ProxyServer) (*http.Response, error) {
	proxy_url, err := proxy_server.URL()
	if err != nil {
		return nil, err
	}
	client, err := d.client(proxy_url)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func (d *default_http_driver) client(raw_proxy_url string) (*http.Client, error) {
	proxy_url, parsed_proxy_url, err := normalize_default_http_proxy_url(raw_proxy_url)
	if err != nil {
		return nil, err
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if client := d.clients[proxy_url]; client != nil {
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
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	if parsed_proxy_url != nil {
		transport.Proxy = http.ProxyURL(parsed_proxy_url)
	}
	client := &http.Client{Transport: transport}
	d.clients[proxy_url] = client
	return client, nil
}

func normalize_default_http_proxy_url(raw_proxy_url string) (string, *url.URL, error) {
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

func parse_default_http_content_range(value string) (start, end, total int64, ok bool) {
	if _, err := fmt.Sscanf(strings.TrimSpace(value), "bytes %d-%d/%d", &start, &end, &total); err != nil {
		return 0, 0, 0, false
	}
	if start < 0 || end < start || total <= end {
		return 0, 0, 0, false
	}
	return start, end, total, true
}
