package clawreq

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestClientFollowsCrossHostRedirectWithFreshHost(t *testing.T) {
	received_host := make(chan string, 1)
	target_server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		received_host <- request.Host
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer target_server.Close()

	redirect_url := strings.Replace(target_server.URL, "127.0.0.1", "localhost", 1)
	source_server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, redirect_url, http.StatusFound)
	}))
	defer source_server.Close()

	client, err := New(Config{FollowRedirects: true})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	response, err := client.Get(context.Background(), source_server.URL)
	if err != nil {
		t.Fatalf("follow redirect: %v", err)
	}
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}
	if response.FinalURL != redirect_url {
		t.Fatalf("final URL = %q, want %q", response.FinalURL, redirect_url)
	}

	want_host := strings.TrimPrefix(redirect_url, "http://")
	if host := <-received_host; host != want_host {
		t.Fatalf("redirect Host = %q, want %q", host, want_host)
	}
}

func TestClientResolvesRelativeRedirect(t *testing.T) {
	test_server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/start" {
			http.Redirect(writer, request, "/final?source=redirect", http.StatusFound)
			return
		}
		if request.URL.Path != "/final" || request.URL.Query().Get("source") != "redirect" {
			http.Error(writer, "unexpected redirect target", http.StatusBadRequest)
			return
		}
		writer.Header().Set("Content-Type", "text/plain")
		_, _ = writer.Write([]byte("done"))
	}))
	defer test_server.Close()

	client, err := New(Config{FollowRedirects: true})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	response, err := client.Get(context.Background(), test_server.URL+"/start")
	if err != nil {
		t.Fatalf("follow relative redirect: %v", err)
	}
	if response.StatusCode != http.StatusOK || string(response.Body) != "done" {
		t.Fatalf("response = status %d body %q", response.StatusCode, response.Body)
	}
	if response.FinalURL != test_server.URL+"/final?source=redirect" {
		t.Fatalf("final URL = %q", response.FinalURL)
	}
}

func TestClientDoesNotFollowRedirectWhenDisabled(t *testing.T) {
	var target_hits atomic.Int32
	test_server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/target" {
			target_hits.Add(1)
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		http.Redirect(writer, request, "/target", http.StatusFound)
	}))
	defer test_server.Close()

	client, err := New(Config{FollowRedirects: false})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	response, err := client.Get(context.Background(), test_server.URL)
	if err != nil {
		t.Fatalf("request redirect: %v", err)
	}
	if response.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusFound)
	}
	if target_hits.Load() != 0 {
		t.Fatalf("redirect target received %d requests", target_hits.Load())
	}
}

func TestClientStripsSensitiveHeadersOnCrossHostRedirect(t *testing.T) {
	type received_headers struct {
		authorization string
		cookie        string
	}
	received := make(chan received_headers, 1)
	target_server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		received <- received_headers{
			authorization: request.Header.Get("Authorization"),
			cookie:        request.Header.Get("Cookie"),
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer target_server.Close()

	redirect_url := strings.Replace(target_server.URL, "127.0.0.1", "localhost", 1)
	source_server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.SetCookie(writer, &http.Cookie{Name: "source_cookie", Value: "source_value", Path: "/"})
		http.Redirect(writer, request, redirect_url, http.StatusFound)
	}))
	defer source_server.Close()

	client, err := New(Config{FollowRedirects: true})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	response, err := client.Get(
		context.Background(),
		source_server.URL,
		WithHeader("Authorization", "Bearer secret"),
		WithCookie("explicit_cookie=secret"),
	)
	if err != nil {
		t.Fatalf("follow redirect: %v", err)
	}
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}
	redirected_headers := <-received
	if redirected_headers.authorization != "" {
		t.Fatalf("Authorization leaked to redirect target: %q", redirected_headers.authorization)
	}
	if redirected_headers.cookie != "" {
		t.Fatalf("Cookie leaked to redirect target: %q", redirected_headers.cookie)
	}
}

func TestClientStoresRedirectCookiesForSameHost(t *testing.T) {
	received_cookie := make(chan string, 1)
	test_server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/start" {
			http.SetCookie(writer, &http.Cookie{Name: "redirect_cookie", Value: "cookie_value", Path: "/"})
			http.Redirect(writer, request, "/target", http.StatusFound)
			return
		}
		received_cookie <- request.Header.Get("Cookie")
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer test_server.Close()

	client, err := New(Config{FollowRedirects: true})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	response, err := client.Get(context.Background(), test_server.URL+"/start")
	if err != nil {
		t.Fatalf("follow redirect: %v", err)
	}
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}
	if cookie := <-received_cookie; !strings.Contains(cookie, "redirect_cookie=cookie_value") {
		t.Fatalf("redirect Cookie = %q", cookie)
	}
}

func TestClientStopsAfterRedirectLimit(t *testing.T) {
	var request_count atomic.Int32
	test_server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		request_count.Add(1)
		http.Redirect(writer, request, "/loop", http.StatusFound)
	}))
	defer test_server.Close()

	client, err := New(Config{FollowRedirects: true})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	response, err := client.Get(context.Background(), test_server.URL+"/loop")
	if err == nil || !strings.Contains(err.Error(), "stopped after 10 redirects") {
		t.Fatalf("error = %v, want redirect limit error", err)
	}
	if response == nil || response.StatusCode != http.StatusFound {
		t.Fatalf("response = %#v, want final redirect response", response)
	}
	if request_count.Load() != max_redirect_count+1 {
		t.Fatalf("request count = %d, want %d", request_count.Load(), max_redirect_count+1)
	}
}

func TestRedirectRequestMethodSemantics(t *testing.T) {
	body := []byte("payload")
	method, redirected_body := redirect_request(http.MethodPost, body, http.StatusFound)
	if method != http.MethodGet || redirected_body != nil {
		t.Fatalf("302 redirect = method %q body %q", method, redirected_body)
	}

	method, redirected_body = redirect_request(http.MethodPost, body, http.StatusTemporaryRedirect)
	if method != http.MethodPost || string(redirected_body) != string(body) {
		t.Fatalf("307 redirect = method %q body %q", method, redirected_body)
	}
}
