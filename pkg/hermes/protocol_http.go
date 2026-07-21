package hermes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

// httpProtocolDriver 是 HTTP/HTTPS 插件。它强制 identity 编码，避免压缩响应
// 的 Content-Length 与 Range 偏移不一致。HTTPS 请求使用 tls-client 的 Chrome
// 144 profile；该 profile 是当前库中距 Chrome 143 最近的公开 Chrome profile。
type httpProtocolDriver struct {
	mu      sync.Mutex
	clients map[string]tls_client.HttpClient
}

func newHTTPProtocolDriver() *httpProtocolDriver {
	return &httpProtocolDriver{clients: make(map[string]tls_client.HttpClient)}
}

func (d *httpProtocolDriver) Protocols() []string { return []string{"http", "https"} }

func (d *httpProtocolDriver) Prepare(ctx context.Context, endpoint Endpoint) (PreparedResource, error) {
	// 直接用最小 Range GET 同时探测资源大小和 Range 能力。部分下载服务
	// 不实现 HEAD 或会让 HEAD 长时间挂起，先发 HEAD 会无谓阻塞任务启动。
	probeReq, err := d.newRequest(ctx, http.MethodGet, endpoint, ReadRequest{OffsetStart: 0, OffsetEnd: 0, UseRange: true})
	if err != nil {
		return PreparedResource{}, err
	}
	client, err := d.clientFor(endpoint)
	if err != nil {
		return PreparedResource{}, err
	}
	response, err := client.Do(probeReq)
	if err != nil {
		return PreparedResource{}, err
	}
	defer response.Body.Close()
	prepared := PreparedResource{ContentType: response.Header.Get("Content-Type")}
	if response.StatusCode == http.StatusPartialContent {
		start, end, total, ok := parseContentRange(response.Header.Get("Content-Range"))
		if !ok || start != 0 || end != 0 {
			return PreparedResource{}, errors.New("服务器返回了无效的 Content-Range")
		}
		prepared.Size = total
		prepared.SupportsRange = true
		return prepared, nil
	}
	if response.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		var total int64
		if _, scanErr := fmt.Sscanf(strings.TrimSpace(response.Header.Get("Content-Range")), "bytes */%d", &total); scanErr == nil && total == 0 {
			return prepared, nil
		}
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		prepared.Size = normalizedSize(response.ContentLength)
		return prepared, nil
	}
	return PreparedResource{}, fmt.Errorf("资源探测返回状态码 %d", response.StatusCode)
}

func (d *httpProtocolDriver) Open(ctx context.Context, endpoint Endpoint, request ReadRequest) (io.ReadCloser, error) {
	req, err := d.newRequest(ctx, http.MethodGet, endpoint, request)
	if err != nil {
		return nil, err
	}
	client, err := d.clientFor(endpoint)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if request.UseRange {
		if response.StatusCode != http.StatusPartialContent {
			response.Body.Close()
			return nil, fmt.Errorf("服务器不支持请求范围，状态码 %d", response.StatusCode)
		}
		start, end, _, ok := parseContentRange(response.Header.Get("Content-Range"))
		if !ok || start != request.OffsetStart || end > request.OffsetEnd {
			response.Body.Close()
			return nil, errors.New("服务器返回的 Content-Range 与请求不匹配")
		}
	} else if response.StatusCode < 200 || response.StatusCode >= 300 {
		response.Body.Close()
		return nil, fmt.Errorf("服务器返回错误状态码: %d", response.StatusCode)
	}
	return response.Body, nil
}

func (d *httpProtocolDriver) newRequest(ctx context.Context, method string, endpoint Endpoint, read ReadRequest) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint.URL, nil)
	if err != nil {
		return nil, err
	}
	for key, value := range endpoint.Headers {
		req.Header.Set(key, value)
	}
	if endpoint.Cookies != "" && req.Header.Get("Cookie") == "" {
		req.Header.Set("Cookie", endpoint.Cookies)
	}
	req.Header.Set("Accept-Encoding", "identity")
	if read.UseRange {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", read.OffsetStart, read.OffsetEnd))
	}
	return req, nil
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

func normalizedSize(size int64) int64 {
	if size < 0 {
		return 0
	}
	return size
}

func (d *httpProtocolDriver) clientFor(endpoint Endpoint) (tls_client.HttpClient, error) {
	probe, err := http.NewRequest(http.MethodGet, endpoint.URL, nil)
	if err != nil {
		return nil, err
	}
	proxyURL, err := http.ProxyFromEnvironment(probe)
	if err != nil {
		return nil, err
	}

	proxy := ""
	if proxyURL != nil {
		proxy = proxyURL.String()
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if client := d.clients[proxy]; client != nil {
		return client, nil
	}
	client, err := newDownloadClient(proxy)
	if err != nil {
		return nil, err
	}
	d.clients[proxy] = client
	return client, nil
}

// newDownloadClient 创建可供多个任务/分片并发复用的 Chrome 指纹 HTTP 客户端。
// 传入的代理地址来自 ProxyFromEnvironment，因此仍遵守 HTTP(S)_PROXY 与 NO_PROXY。
func newDownloadClient(proxyURL string) (tls_client.HttpClient, error) {
	idleTimeout := 90 * time.Second
	options := []tls_client.HttpClientOption{
		tls_client.WithClientProfile(profiles.Chrome_144),
		tls_client.WithTimeoutMilliseconds(0),
		tls_client.WithDialer(net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}),
		tls_client.WithTransportOptions(&tls_client.TransportOptions{
			IdleConnTimeout:     &idleTimeout,
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: defaultSegmentCount,
		}),
	}
	if proxyURL != "" {
		options = append(options, tls_client.WithProxyUrl(proxyURL))
	}
	return tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
}
