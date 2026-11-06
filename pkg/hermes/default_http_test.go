package hermes

import (
	"crypto/x509"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type retry_round_tripper func(*http.Request) (*http.Response, error)

func (f retry_round_tripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestDefaultHTTPIPv4Retry(t *testing.T) {
	certificate_error := x509.UnknownAuthorityError{}
	network_error := errors.New("network unavailable")
	for _, test_case := range []struct {
		name        string
		proxy       ProxyServer
		initial_err error
		retry_err   error
		want_retry  bool
	}{
		{name: "certificate failure", initial_err: certificate_error, want_retry: true},
		{name: "retry fails", initial_err: certificate_error, retry_err: network_error, want_retry: true},
		{name: "network failure", initial_err: network_error},
		{name: "proxy failure", proxy: ProxyServer{Address: "http://proxy.example:8080"}, initial_err: certificate_error},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "https://media.example/video", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Range", "bytes=128-255")
			req.Header.Set("Cookie", "session=example")
			proxy_url, err := test_case.proxy.URL()
			if err != nil {
				t.Fatal(err)
			}
			retries := 0
			driver := new_default_http_driver()
			driver.clients[proxy_url] = &http.Client{Transport: retry_round_tripper(func(*http.Request) (*http.Response, error) {
				return nil, test_case.initial_err
			})}
			driver.clients[proxy_url+"|tcp4"] = &http.Client{Transport: retry_round_tripper(func(retry_req *http.Request) (*http.Response, error) {
				retries++
				if retry_req == req || retry_req.Context() != req.Context() || retry_req.Header.Get("Range") != "bytes=128-255" || retry_req.Header.Get("Cookie") != "session=example" {
					t.Fatal("retry did not preserve the request context and download headers in a clone")
				}
				if test_case.retry_err != nil {
					return nil, test_case.retry_err
				}
				return &http.Response{StatusCode: http.StatusPartialContent, Body: io.NopCloser(strings.NewReader("video")), Header: make(http.Header)}, nil
			})}
			response, err := driver.do(req, test_case.proxy)
			if (retries == 1) != test_case.want_retry {
				t.Fatalf("unexpected IPv4 retries: %d", retries)
			}
			if !test_case.want_retry || test_case.retry_err != nil {
				if err == nil || !errors.Is(err, test_case.initial_err) {
					t.Fatalf("original error lost: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil || response.StatusCode != http.StatusPartialContent || string(body) != "video" {
				t.Fatalf("download response lost: response=%+v body=%q err=%v", response, body, err)
			}
		})
	}
}
