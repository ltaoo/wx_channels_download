package douyin

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// HttpClient is the web-side HTTP client.
type HttpClient struct {
	req *http.Request
}

// HttpResponse holds the HTTP response.
type HttpResponse struct {
	body []byte
}

// NewHttpClient creates a new HTTP client.
func NewHttpClient(method string, u string, body any, headers any) *HttpClient {
	values := url.Values{}
	switch tt := body.(type) {
	case map[string]string:
		for k, v := range tt {
			values.Add(k, v)
		}
	}

	_headers := make(map[string]string)
	switch tt := headers.(type) {
	case map[string]interface{}:
		for k, v := range tt {
			if s, ok := v.(string); ok {
				_headers[k] = s
			}
		}
	case map[string]string:
		for k, v := range tt {
			_headers[k] = v
		}
	}

	req, err := http.NewRequest(method, u, strings.NewReader(values.Encode()))
	if err != nil {
		return nil
	}
	for key, value := range _headers {
		req.Header.Set(key, value)
	}
	return &HttpClient{req}
}

// Request sends the HTTP request.
func (c *HttpClient) Request() (*HttpResponse, error) {
	client := &http.Client{}
	resp, err := client.Do(c.req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &HttpResponse{body: respBytes}, nil
}

// ToJSON parses the response body as JSON.
func (c *HttpResponse) ToJSON(v any) error {
	return json.Unmarshal(c.body, v)
}
