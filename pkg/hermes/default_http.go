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

const defaultHTTPProbeSize = 512

// defaultHTTPDriver is the dependency-free HTTP/HTTPS driver used by the
// high-level API. Applications may replace it with RegisterProtocol.
type defaultHTTPDriver struct {
	mu      sync.Mutex
	clients map[string]*http.Client
}

func newDefaultHTTPDriver() *defaultHTTPDriver {
	return &defaultHTTPDriver{clients: make(map[string]*http.Client)}
}

func (d *defaultHTTPDriver) Protocols() []string {
	return []string{"http", "https"}
}

func (d *defaultHTTPDriver) Prepare(ctx context.Context, endpoint Endpoint) (PreparedResource, error) {
	req, err := d.newRequest(ctx, endpoint, ReadRequest{
		OffsetStart: 0,
		OffsetEnd:   defaultHTTPProbeSize - 1,
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

	probe, err := io.ReadAll(io.LimitReader(resp.Body, defaultHTTPProbeSize))
	if err != nil {
		return PreparedResource{}, fmt.Errorf("read HTTP probe: %w", err)
	}
	prepared := PreparedResource{
		ContentType: resp.Header.Get("Content-Type"),
		ProbeData:   probe,
	}

	if resp.StatusCode == http.StatusPartialContent {
		start, _, total, ok := parseDefaultHTTPContentRange(resp.Header.Get("Content-Range"))
		if !ok || start != 0 {
			return PreparedResource{}, errors.New("server returned an invalid Content-Range")
		}
		prepared.Size = total
		prepared.SupportsRange = true
		return prepared, nil
	}
	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		var total int64
		if _, scanErr := fmt.Sscanf(strings.TrimSpace(resp.Header.Get("Content-Range")), "bytes */%d", &total); scanErr == nil && total == 0 {
			return prepared, nil
		}
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return PreparedResource{}, fmt.Errorf("resource probe returned status code %d", resp.StatusCode)
	}
	if value := resp.Header.Get("Content-Length"); value != "" {
		if size, parseErr := strconv.ParseInt(value, 10, 64); parseErr == nil && size >= 0 {
			prepared.Size = size
		}
	}
	return prepared, nil
}

func (d *defaultHTTPDriver) Open(ctx context.Context, endpoint Endpoint, request ReadRequest) (io.ReadCloser, error) {
	req, err := d.newRequest(ctx, endpoint, request)
	if err != nil {
		return nil, err
	}
	resp, err := d.do(req, endpoint.ProxyServer)
	if err != nil {
		return nil, err
	}

	if request.UseRange {
		if resp.StatusCode != http.StatusPartialContent {
			resp.Body.Close()
			return nil, fmt.Errorf("server does not support the requested range, status code %d", resp.StatusCode)
		}
		start, end, _, ok := parseDefaultHTTPContentRange(resp.Header.Get("Content-Range"))
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

func (d *defaultHTTPDriver) newRequest(ctx context.Context, endpoint Endpoint, request ReadRequest) (*http.Request, error) {
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

func (d *defaultHTTPDriver) do(req *http.Request, proxyServer ProxyServer) (*http.Response, error) {
	proxyURL, err := proxyServer.URL()
	if err != nil {
		return nil, err
	}
	client, err := d.client(proxyURL)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func (d *defaultHTTPDriver) client(rawProxyURL string) (*http.Client, error) {
	proxyURL, parsedProxyURL, err := normalizeDefaultHTTPProxyURL(rawProxyURL)
	if err != nil {
		return nil, err
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if client := d.clients[proxyURL]; client != nil {
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
	if parsedProxyURL != nil {
		transport.Proxy = http.ProxyURL(parsedProxyURL)
	}
	client := &http.Client{Transport: transport}
	d.clients[proxyURL] = client
	return client, nil
}

func normalizeDefaultHTTPProxyURL(rawProxyURL string) (string, *url.URL, error) {
	rawProxyURL = strings.TrimSpace(rawProxyURL)
	if rawProxyURL == "" {
		return "", nil, nil
	}
	proxyURL, err := url.Parse(rawProxyURL)
	if err != nil || proxyURL.Host == "" || proxyURL.Hostname() == "" {
		return "", nil, errors.New("invalid proxy URL")
	}
	proxyURL.Scheme = strings.ToLower(proxyURL.Scheme)
	switch proxyURL.Scheme {
	case "http", "https", "socks5":
	default:
		return "", nil, fmt.Errorf("unsupported proxy scheme %q", proxyURL.Scheme)
	}
	return proxyURL.String(), proxyURL, nil
}

func parseDefaultHTTPContentRange(value string) (start, end, total int64, ok bool) {
	if _, err := fmt.Sscanf(strings.TrimSpace(value), "bytes %d-%d/%d", &start, &end, &total); err != nil {
		return 0, 0, 0, false
	}
	if start < 0 || end < start || total <= end {
		return 0, 0, 0, false
	}
	return start, end, total, true
}
