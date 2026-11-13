package iqiyi

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type round_trip_func func(*http.Request) (*http.Response, error)

func (handler round_trip_func) RoundTrip(req *http.Request) (*http.Response, error) {
	return handler(req)
}

type test_api struct{ HTTPClient *http.Client }

func mock_client(t *testing.T, handler func(*http.Request) (string, int)) *test_api {
	t.Helper()
	return &test_api{HTTPClient: &http.Client{Transport: round_trip_func(func(req *http.Request) (*http.Response, error) {
		body, status := handler(req)
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body)), Header: make(http.Header), Request: req}, nil
	})}}
}
func new_test_backend(client *backend) *backend {
	client.device_id = "synthetic"
	client.Policy = source_policy
	return client
}
