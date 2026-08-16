package douyin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// HttpClient is the web-side HTTP client.
type HttpClient struct {
	req       *http.Request
	build_err error
}

// HttpResponse holds the HTTP response.
type HttpResponse struct {
	body             []byte
	status_code      int
	request_url      string
	content_type     string
	content_encoding string
	content_length   int64
}

// NewHttpClient creates a new HTTP client.
func NewHttpClient(method string, u string, body any, headers any) *HttpClient {
	values := url.Values{}
	switch typed_body := body.(type) {
	case map[string]string:
		for key, value := range typed_body {
			values.Add(key, value)
		}
	}

	request_headers := make(map[string]string)
	switch typed_headers := headers.(type) {
	case map[string]interface{}:
		for key, value := range typed_headers {
			if string_value, ok := value.(string); ok {
				request_headers[key] = string_value
			}
		}
	case map[string]string:
		for key, value := range typed_headers {
			request_headers[key] = value
		}
	}

	req, err := http.NewRequest(method, u, strings.NewReader(values.Encode()))
	if err != nil {
		return &HttpClient{build_err: err}
	}
	for key, value := range request_headers {
		req.Header.Set(key, value)
	}
	return &HttpClient{req: req}
}

// Request sends the HTTP request.
func (c *HttpClient) Request() (*HttpResponse, error) {
	return c.RequestWithClient(nil)
}

// RequestWithClient sends the HTTP request with client. A nil client keeps the
// legacy behavior and uses a default net/http client.
func (c *HttpClient) RequestWithClient(client *http.Client) (*HttpResponse, error) {
	if c == nil {
		return nil, fmt.Errorf("HTTP client is nil")
	}
	if c.build_err != nil {
		return nil, c.build_err
	}
	if c.req == nil {
		return nil, fmt.Errorf("HTTP request is nil")
	}

	if client == nil {
		client = &http.Client{}
	}
	resp, err := client.Do(c.req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	response_body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &HttpResponse{
		body:             response_body,
		status_code:      resp.StatusCode,
		request_url:      resp.Request.URL.String(),
		content_type:     resp.Header.Get("Content-Type"),
		content_encoding: resp.Header.Get("Content-Encoding"),
		content_length:   resp.ContentLength,
	}, nil
}

// ToJSON parses the response body as JSON.
func (c *HttpResponse) ToJSON(v any) error {
	if c == nil {
		return fmt.Errorf("decode JSON response: response is nil")
	}
	if err := json.Unmarshal(c.body, v); err != nil {
		return fmt.Errorf(
			"decode JSON response: http_status=%d content_type=%q content_encoding=%q content_length=%d body_bytes=%d body_preview=%q: %w",
			c.status_code,
			c.content_type,
			c.content_encoding,
			c.content_length,
			len(c.body),
			log_body_preview(c.body),
			err,
		)
	}
	return nil
}
