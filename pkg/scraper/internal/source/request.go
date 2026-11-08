// Adapted from the local dramadl project; see ../../NOTICE and ../../LICENSE.
package source

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"wx_channel/pkg/scraper"
)

type RequestClient struct {
	UserAgent   string
	Profile     *scraper.Profile
	PlaylistURL string
	HTTPClient  *http.Client
	Policy      Policy
	Cookie      string
	Referer     string
}

func (client *RequestClient) Request(ctx context.Context, address string, body []byte, metadata bool, byte_range string) (*http.Response, error) {
	if err := client.Policy.check_address(address, metadata); err != nil {
		return nil, err
	}
	method := http.MethodGet
	if body != nil {
		method = http.MethodPost
	}
	req, err := http.NewRequestWithContext(ctx, method, address, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("视频请求地址无效")
	}
	req.Header.Set("User-Agent", FirstText(client.UserAgent, UserAgent))
	req.Header.Set("Referer", client.Referer)
	if !metadata && byte_range != "" {
		req.Header.Set("Range", byte_range)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	send_cookie := metadata
	for _, host := range client.Policy.AnonymousHosts {
		if strings.EqualFold(req.URL.Hostname(), host) {
			send_cookie = false
			break
		}
	}
	if send_cookie {
		req.Header.Set("Cookie", client.Cookie)
	}
	http_client := *client.HTTPClient
	if !send_cookie {
		http_client.Jar = nil
	}
	var redirect_error error
	http_client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			redirect_error = errors.New("视频服务跳转次数过多")
			return redirect_error
		}
		redirect_error = client.Policy.check_address(next.URL.String(), metadata)
		if !metadata || next.URL.Hostname() != req.URL.Hostname() {
			next.Header.Del("Cookie")
		}
		return redirect_error
	}
	response, err := http_client.Do(req)
	if err != nil {
		if redirect_error != nil {
			return nil, redirect_error
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("视频服务网络连接失败")
	}
	if response.StatusCode != http.StatusOK && (metadata || response.StatusCode != http.StatusPartialContent) {
		response.Body.Close()
		return nil, fmt.Errorf("视频服务返回 HTTP %d", response.StatusCode)
	}
	return response, nil
}

func (client *RequestClient) Response(ctx context.Context, address string, metadata bool) (*http.Response, error) {
	return client.Request(ctx, address, nil, metadata, "")
}

func ReadVideoResponse(response *http.Response) (string, error) {
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, (8<<20)+1))
	if err != nil || len(data) > 8<<20 {
		return "", errors.New("视频响应读取失败或超过 8 MiB")
	}
	return string(data), nil
}

func (client *RequestClient) LoadHLS(ctx context.Context, address string, depth int) (string, error) {
	video, audio, err := client.LoadHLSTracks(ctx, address, depth)
	if err == nil && audio != "" {
		return "", errors.New("该片源需要独立音轨处理")
	}
	return video, err
}
