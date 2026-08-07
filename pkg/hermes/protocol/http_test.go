package protocol

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"wx_channel/pkg/hermes"
)

func TestNormalizeProxyURL(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		want       string
		wantScheme string
		wantErr    bool
	}{
		{name: "direct", input: "", want: ""},
		{name: "http", input: " HTTP://user:pass@127.0.0.1:8080 ", want: "http://user:pass@127.0.0.1:8080", wantScheme: "http"},
		{name: "https", input: "https://127.0.0.1:8443", want: "https://127.0.0.1:8443", wantScheme: "https"},
		{name: "socks5", input: "socks5://127.0.0.1:1080", want: "socks5://127.0.0.1:1080", wantScheme: "socks5"},
		{name: "missing scheme", input: "127.0.0.1:8080", wantErr: true},
		{name: "missing host", input: "http://", wantErr: true},
		{name: "unsupported scheme", input: "ftp://127.0.0.1:21", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, parsed, err := normalizeProxyURL(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("normalizeProxyURL() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got != tt.want {
				t.Fatalf("normalized proxy = %q, want %q", got, tt.want)
			}
			if tt.wantScheme == "" {
				if parsed != nil {
					t.Fatalf("parsed proxy = %v, want nil", parsed)
				}
				return
			}
			if parsed == nil || parsed.Scheme != tt.wantScheme {
				t.Fatalf("parsed proxy scheme = %v, want %q", parsed, tt.wantScheme)
			}
		})
	}
}

func TestHTTPDriverSeparatesClientsByProxy(t *testing.T) {
	driver := NewHTTPDriver()
	proxyURL := "http://127.0.0.1:18080"

	directClient, err := driver.standardHTTPClient("")
	if err != nil {
		t.Fatal(err)
	}
	proxiedClient, err := driver.standardHTTPClient(proxyURL)
	if err != nil {
		t.Fatal(err)
	}
	proxiedClientAgain, err := driver.standardHTTPClient("  " + proxyURL + "  ")
	if err != nil {
		t.Fatal(err)
	}
	if directClient == proxiedClient {
		t.Fatal("direct and proxied requests unexpectedly share one HTTP client")
	}
	if proxiedClient != proxiedClientAgain {
		t.Fatal("the same proxy did not reuse its HTTP client")
	}

	transport, ok := proxiedClient.Transport.(*http.Transport)
	if !ok || transport.Proxy == nil {
		t.Fatal("proxied HTTP client has no proxy function")
	}
	req, err := http.NewRequest(http.MethodGet, "https://download.example/file", nil)
	if err != nil {
		t.Fatal(err)
	}
	gotProxy, err := transport.Proxy(req)
	if err != nil {
		t.Fatal(err)
	}
	if gotProxy == nil || gotProxy.String() != proxyURL {
		t.Fatalf("transport proxy = %v, want %s", gotProxy, proxyURL)
	}

	directPool, err := driver.browserClientPool("")
	if err != nil {
		t.Fatal(err)
	}
	proxiedPool, err := driver.browserClientPool(proxyURL)
	if err != nil {
		t.Fatal(err)
	}
	proxiedPoolAgain, err := driver.browserClientPool(proxyURL)
	if err != nil {
		t.Fatal(err)
	}
	if directPool == proxiedPool {
		t.Fatal("direct and proxied requests unexpectedly share one browser client pool")
	}
	if proxiedPool != proxiedPoolAgain {
		t.Fatal("the same proxy did not reuse its browser client pool")
	}
}

func TestHTTPDriverRoutesRequestsThroughTaskProxy(t *testing.T) {
	content := []byte(strings.Repeat("hermes-proxy-", 100))
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		rangeHeader := r.Header.Get("Range")
		if rangeHeader == "" {
			w.Header().Set("Content-Length", strconv.Itoa(len(content)))
			_, _ = w.Write(content)
			return
		}

		var start, end int
		if _, err := fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end); err != nil || start < 0 || end < start || start >= len(content) {
			http.Error(w, "invalid range", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if end >= len(content) {
			end = len(content) - 1
		}
		w.Header().Set("Content-Length", strconv.Itoa(end-start+1))
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(content)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(content[start : end+1])
	}))
	defer target.Close()

	const proxyUsername = "proxy-user"
	const proxyPassword = "proxy-password"
	proxy, requestCount := newRecordingHTTPProxy(t, proxyUsername, proxyPassword)
	defer proxy.Close()

	driver := NewHTTPDriver()
	endpoint := hermes.Endpoint{
		URL: target.URL,
		ProxyServer: hermes.ProxyServer{
			Address:  proxy.URL,
			Username: proxyUsername,
			Password: proxyPassword,
		},
	}
	prepared, err := driver.Prepare(context.Background(), endpoint)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if prepared.Size != int64(len(content)) || !prepared.SupportsRange {
		t.Fatalf("prepared = %+v, want size %d with ranges", prepared, len(content))
	}

	full, err := driver.Open(context.Background(), endpoint, hermes.ReadRequest{})
	if err != nil {
		t.Fatalf("full Open() error = %v", err)
	}
	fullBody, err := io.ReadAll(full)
	_ = full.Close()
	if err != nil {
		t.Fatal(err)
	}
	if string(fullBody) != string(content) {
		t.Fatalf("full body length = %d, want %d", len(fullBody), len(content))
	}

	ranged, err := driver.Open(context.Background(), endpoint, hermes.ReadRequest{
		OffsetStart: 100,
		OffsetEnd:   199,
		UseRange:    true,
	})
	if err != nil {
		t.Fatalf("range Open() error = %v", err)
	}
	rangeBody, err := io.ReadAll(ranged)
	_ = ranged.Close()
	if err != nil {
		t.Fatal(err)
	}
	if string(rangeBody) != string(content[100:200]) {
		t.Fatalf("range body length = %d, want %d", len(rangeBody), 100)
	}
	if got := requestCount.Load(); got < 3 {
		t.Fatalf("proxy observed %d requests, want at least 3", got)
	}
}

func newRecordingHTTPProxy(t *testing.T, username, password string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var requestCount atomic.Int32
	directTransport := &http.Transport{
		DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
	}
	t.Cleanup(directTransport.CloseIdleConnections)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantAuthorization := "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
		if r.Header.Get("Proxy-Authorization") != wantAuthorization {
			w.Header().Set("Proxy-Authenticate", `Basic realm="hermes-test"`)
			http.Error(w, "proxy authentication required", http.StatusProxyAuthRequired)
			return
		}
		requestCount.Add(1)
		if r.Method == http.MethodConnect {
			destination, err := net.DialTimeout("tcp", r.Host, 5*time.Second)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				_ = destination.Close()
				http.Error(w, "hijacking unsupported", http.StatusInternalServerError)
				return
			}
			client, buffered, err := hijacker.Hijack()
			if err != nil {
				_ = destination.Close()
				return
			}
			_, _ = buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
			_ = buffered.Flush()
			go func() {
				_, _ = io.Copy(destination, client)
				_ = destination.Close()
			}()
			_, _ = io.Copy(client, destination)
			_ = client.Close()
			return
		}

		outbound := r.Clone(r.Context())
		outbound.RequestURI = ""
		outbound.Header.Del("Proxy-Connection")
		response, err := directTransport.RoundTrip(outbound)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer response.Body.Close()
		for key, values := range response.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(response.StatusCode)
		_, _ = io.Copy(w, response.Body)
	}))
	return server, &requestCount
}
