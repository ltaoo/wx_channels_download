package protocol

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"wx_channel/pkg/clawreq"
	"wx_channel/pkg/hermes"
)

// HTTPDriver is the HTTP/HTTPS protocol driver that uses clawreq (CycleTLS) to
// provide Chrome 112 JA3/uTLS fingerprints, avoiding detection as a Go HTTP client.
// Uses a client pool to support concurrent HTTP Range requests, eliminating serial
// bottlenecks caused by single-client mutex locks.
type HTTPDriver struct {
	pool    chan *clawreq.Client
	stdHTTP *http.Client
}

const httpDriverPoolSize = 3

// NewHTTPDriver creates a new HTTP protocol driver instance.
func NewHTTPDriver() *HTTPDriver {
	pool := make(chan *clawreq.Client, httpDriverPoolSize)
	for i := 0; i < httpDriverPoolSize; i++ {
		client, _ := clawreq.New(clawreq.Config{
			Profile:         clawreq.ProfileChrome,
			FollowRedirects: true,
		})
		pool <- client
	}
	return &HTTPDriver{
		pool: pool,
		stdHTTP: &http.Client{
			Timeout: 300 * time.Second,
			Transport: &http.Transport{
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
			},
		},
	}
}

// Protocols returns the protocol identifiers supported by this driver.
func (d *HTTPDriver) Protocols() []string { return []string{"http", "https"} }

// prepareProbeSize is the number of bytes downloaded during probing, used for Range capability detection and magic bytes type detection.
const prepareProbeSize = 512

// Prepare probes the resource for size, Range capability, Content-Type, and first 512 bytes (for magic bytes detection).
func (d *HTTPDriver) Prepare(ctx context.Context, endpoint hermes.Endpoint) (hermes.PreparedResource, error) {
	resp, err := d.do(ctx, endpoint, clawreq.WithHeader("Range", fmt.Sprintf("bytes=0-%d", prepareProbeSize-1)))
	if err != nil {
		return hermes.PreparedResource{}, err
	}

	prepared := hermes.PreparedResource{
		ContentType: resp.Header.Get("Content-Type"),
		ProbeData:   resp.Body,
	}
	if resp.StatusCode == http.StatusPartialContent {
		start, end, total, ok := parseContentRange(resp.Header.Get("Content-Range"))
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
		if _, scanErr := fmt.Sscanf(strings.TrimSpace(resp.Header.Get("Content-Range")), "bytes */%d", &total); scanErr == nil && total == 0 {
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
	return hermes.PreparedResource{}, fmt.Errorf("resource probe returned status code %d", resp.StatusCode)
}

// Open downloads a resource according to ReadRequest and returns a readable data stream.
// For Range requests (segmented downloads), uses the net/http standard library directly to
// return resp.Body for streaming reads, enabling real-time progress events in the download loop.
// For non-Range requests, still uses clawreq to preserve browser fingerprint.
func (d *HTTPDriver) Open(ctx context.Context, endpoint hermes.Endpoint, request hermes.ReadRequest) (io.ReadCloser, error) {
	if request.UseRange {
		return d.openRange(ctx, endpoint, request)
	}
	return d.openFull(ctx, endpoint)
}

// openRange uses the net/http standard library for Range requests, returning streaming resp.Body.
// WeChat CDN Range requests do not require browser fingerprint masking; the standard library suffices for real-time progress.
func (d *HTTPDriver) openRange(ctx context.Context, endpoint hermes.Endpoint, request hermes.ReadRequest) (io.ReadCloser, error) {
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

	resp, err := d.stdHTTP.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusPartialContent {
		resp.Body.Close()
		return nil, fmt.Errorf("server does not support the requested range, status code %d", resp.StatusCode)
	}
	start, end, _, ok := parseContentRange(resp.Header.Get("Content-Range"))
	if !ok || start != request.OffsetStart || end > request.OffsetEnd {
		resp.Body.Close()
		return nil, errors.New("server returned a Content-Range that does not match the request")
	}

	return resp.Body, nil
}

// openFull downloads the full resource via clawreq (non-Range) to preserve browser fingerprint.
func (d *HTTPDriver) openFull(ctx context.Context, endpoint hermes.Endpoint) (io.ReadCloser, error) {
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
	allOpts := make([]clawreq.RequestOption, 0, len(opts)+len(endpoint.Headers)+3)
	allOpts = append(allOpts, clawreq.WithHeader("Accept-Encoding", "identity"))
	allOpts = append(allOpts, clawreq.WithHeader("Accept", "*/*"))
	for key, value := range endpoint.Headers {
		allOpts = append(allOpts, clawreq.WithHeader(key, value))
	}
	if endpoint.Cookies != "" {
		allOpts = append(allOpts, clawreq.WithCookie(endpoint.Cookies))
	}
	allOpts = append(allOpts, opts...)

	client := <-d.pool
	defer func() { d.pool <- client }()
	return client.Do(ctx, "GET", endpoint.URL, nil, allOpts...)
}

func parseContentRange(value string) (start, end, total int64, ok bool) {
	if _, err := fmt.Sscanf(strings.TrimSpace(value), "bytes %d-%d/%d", &start, &end, &total); err != nil {
		return 0, 0, 0, false
	}
	if start < 0 || end < start || total <= end {
		return 0, 0, 0, false
	}
	return start, end, total, true
}
