package pages

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const default_api_base_url = "https://api.cloudflare.com/client/v4"

// ClientOptions controls the Cloudflare API transport. BaseURL and HTTPClient
// are primarily useful for tests and private API gateways.
type ClientOptions struct {
	BaseURL    string
	HTTPClient *http.Client
}

// Client calls the Cloudflare Pages and Pages Assets REST APIs.
type Client struct {
	base_url    string
	http_client *http.Client
}

type api_message struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type api_response struct {
	Success bool            `json:"success"`
	Errors  []api_message   `json:"errors"`
	Result  json.RawMessage `json:"result"`
}

// NewClient creates a Cloudflare Pages API client.
func NewClient(options ClientOptions) *Client {
	base_url := strings.TrimRight(strings.TrimSpace(options.BaseURL), "/")
	if base_url == "" {
		base_url = default_api_base_url
	}
	http_client := options.HTTPClient
	if http_client == nil {
		http_client = http.DefaultClient
	}
	return &Client{base_url: base_url, http_client: http_client}
}

func (c *Client) request(
	request_context context.Context,
	method string,
	endpoint string,
	auth_token string,
	content_type string,
	body io.Reader,
) ([]byte, int, error) {
	request, err := http.NewRequestWithContext(
		request_context,
		method,
		c.base_url+endpoint,
		body,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("创建 Cloudflare Pages 请求失败: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+auth_token)
	if content_type != "" {
		request.Header.Set("Content-Type", content_type)
	}
	response, err := c.http_client.Do(request)
	if err != nil {
		return nil, 0, fmt.Errorf("调用 Cloudflare Pages API 失败: %w", err)
	}
	defer response.Body.Close()
	response_body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, response.StatusCode, fmt.Errorf("读取 Cloudflare Pages 响应失败: %w", err)
	}
	return response_body, response.StatusCode, nil
}

func (c *Client) request_json(
	request_context context.Context,
	method string,
	endpoint string,
	auth_token string,
	payload any,
	result any,
) (int, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return 0, fmt.Errorf("编码 Cloudflare Pages 请求失败: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	response_body, status_code, err := c.request(
		request_context,
		method,
		endpoint,
		auth_token,
		"application/json",
		body,
	)
	if err != nil {
		return status_code, err
	}
	if status_code < http.StatusOK || status_code >= http.StatusMultipleChoices {
		return status_code, response_error(status_code, response_body)
	}
	var response api_response
	if err := json.Unmarshal(response_body, &response); err != nil {
		return status_code, fmt.Errorf("解析 Cloudflare Pages 响应失败: %w", err)
	}
	if !response.Success {
		return status_code, response_error(status_code, response_body)
	}
	if result != nil && len(response.Result) > 0 && string(response.Result) != "null" {
		if err := json.Unmarshal(response.Result, result); err != nil {
			return status_code, fmt.Errorf("解析 Cloudflare Pages result 失败: %w", err)
		}
	}
	return status_code, nil
}

func response_error(status_code int, response_body []byte) error {
	var response api_response
	if err := json.Unmarshal(response_body, &response); err == nil {
		messages := make([]string, 0, len(response.Errors))
		for _, api_error := range response.Errors {
			if api_error.Message != "" {
				messages = append(messages, api_error.Message)
			}
		}
		if len(messages) > 0 {
			return fmt.Errorf("Cloudflare Pages API 返回 %d: %s", status_code, strings.Join(messages, "; "))
		}
	}
	message := strings.TrimSpace(string(response_body))
	if message == "" {
		message = http.StatusText(status_code)
	}
	return fmt.Errorf("Cloudflare Pages API 返回 %d: %s", status_code, message)
}
