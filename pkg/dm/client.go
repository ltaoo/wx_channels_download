// Package dm is the public SDK for the downloader's REST API.
//
// It imports only the standard library so any caller — the MCP stdio host, the
// CLI, or an external program outside this module — can share one transport
// implementation. The downloader answers every endpoint with a
// {"code","msg","data"} envelope; a non-zero code becomes an *APIError and the
// caller decides how to surface it.
package dm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultPollInterval is the wait between progress polls when ClientOptions
// leaves PollInterval unset.
const DefaultPollInterval = 500 * time.Millisecond

const max_response_bytes = 64 * 1024 * 1024

// ClientOptions configures NewClient.
type ClientOptions struct {
	// BaseURL is the downloader root, for example http://127.0.0.1:2022.
	BaseURL string
	// HTTPClient overrides the transport; nil means a default client.
	HTTPClient *http.Client
	// PollInterval overrides DefaultPollInterval for wait loops.
	PollInterval time.Duration
}

// Client talks to the downloader REST API.
type Client struct {
	base_url      string
	http_client   *http.Client
	poll_interval time.Duration
}

// NewClient validates the base URL and builds a Client.
func NewClient(options ClientOptions) (*Client, error) {
	raw_base_url := strings.TrimSpace(options.BaseURL)
	if raw_base_url == "" {
		return nil, fmt.Errorf("API 地址不能为空")
	}
	parsed_url, err := url.Parse(raw_base_url)
	if err != nil || parsed_url.Host == "" || (parsed_url.Scheme != "http" && parsed_url.Scheme != "https") {
		return nil, fmt.Errorf("无效的 API 地址: %s", raw_base_url)
	}
	if parsed_url.RawQuery != "" || parsed_url.Fragment != "" {
		return nil, fmt.Errorf("API 地址不能包含 query 或 fragment")
	}
	http_client := options.HTTPClient
	if http_client == nil {
		http_client = &http.Client{}
	}
	poll_interval := options.PollInterval
	if poll_interval <= 0 {
		poll_interval = DefaultPollInterval
	}
	return &Client{
		base_url:      strings.TrimRight(parsed_url.String(), "/"),
		http_client:   http_client,
		poll_interval: poll_interval,
	}, nil
}

// PollInterval returns the effective wait between progress polls. Callers that
// merge the downloader client with an in-process backend need this fallback.
func (c *Client) PollInterval() time.Duration {
	if c == nil || c.poll_interval <= 0 {
		return DefaultPollInterval
	}
	return c.poll_interval
}

// APIError is a non-zero envelope code returned by the downloader. Data is the
// envelope payload as sent; callers render it or pass it through.
type APIError struct {
	Code int
	Msg  string
	Data json.RawMessage
}

func (e *APIError) Error() string { return e.Msg }

type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// get issues a GET without a body.
func (c *Client) get(ctx context.Context, path string, query url.Values) (json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, path, query, nil)
}

// do performs one request and unwraps the response envelope. Every transport
// failure keeps the downloader-specific wording the CLI and MCP already show.
func (c *Client) do(ctx context.Context, method string, path string, query url.Values, body any) (json.RawMessage, error) {
	request_url := c.base_url + path
	if len(query) > 0 {
		request_url += "?" + query.Encode()
	}
	var request_body io.Reader
	if body != nil {
		encoded_body, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("编码下载器请求失败: %w", err)
		}
		request_body = bytes.NewReader(encoded_body)
	}
	request, err := http.NewRequestWithContext(ctx, method, request_url, request_body)
	if err != nil {
		return nil, fmt.Errorf("创建下载器请求失败: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http_client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("调用下载器服务失败（请确认主服务已启动）: %w", err)
	}
	defer response.Body.Close()
	response_data, err := io.ReadAll(io.LimitReader(response.Body, max_response_bytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取下载器响应失败: %w", err)
	}
	if len(response_data) > max_response_bytes {
		return nil, fmt.Errorf("下载器响应超过 64 MB")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("下载器服务返回状态码 %d: %s", response.StatusCode, strings.TrimSpace(string(response_data)))
	}
	var envelope_result envelope
	if err := json.Unmarshal(response_data, &envelope_result); err != nil {
		return nil, fmt.Errorf("解析下载器响应失败: %w", err)
	}
	if envelope_result.Code != 0 {
		return nil, &APIError{
			Code: envelope_result.Code,
			Msg:  value_or_default(envelope_result.Msg, fmt.Sprintf("下载器返回错误码 %d", envelope_result.Code)),
			Data: envelope_result.Data,
		}
	}
	if !has_json_value(envelope_result.Data) {
		return json.RawMessage("{}"), nil
	}
	return envelope_result.Data, nil
}

func has_json_value(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null"))
}

func value_or_default(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}
