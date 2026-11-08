package source

import (
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
)

func TestExplicitPoliciesIsolateMetadataCredentialsAndRedirects(t *testing.T) {
	policy := Policy{Metadata: []string{"api.example"}, Media: []string{"cdn.example"}, AnonymousHosts: []string{"anonymous.api.example"}, RequireHTTPS: true}
	jar, _ := cookiejar.New(nil)
	for _, host := range []string{"cdn.example", "anonymous.api.example"} {
		address, _ := url.Parse("https://" + host)
		jar.SetCookies(address, []*http.Cookie{{Name: "jar_account", Value: "private"}})
	}
	requests := 0
	client := &RequestClient{Policy: policy, Cookie: "account=private", HTTPClient: &http.Client{Jar: jar, Transport: round_trip_func(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.URL.Hostname() == "api.example" {
			if req.Header.Get("Cookie") != "account=private" {
				t.Errorf("metadata credentials missing: %v", req.Header)
			}
		} else if req.Header.Get("Cookie") != "" {
			t.Errorf("credentials escaped metadata request: %s %v", req.URL, req.Header)
		}
		response := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), ContentLength: 2, Body: io.NopCloser(strings.NewReader("ok")), Request: req}
		if req.URL.Path == "/redirect" {
			response.StatusCode = http.StatusFound
			response.Header.Set("Location", "https://evil.example/resource")
		}
		return response, nil
	})}}
	for _, request := range []struct {
		address  string
		metadata bool
	}{
		{"https://api.example/metadata", true},
		{"https://anonymous.api.example/metadata", true},
		{"https://cdn.example/playlist.m3u8", false},
	} {
		response, err := client.Response(context.Background(), request.address, request.metadata)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
	}
	if _, err := client.Response(context.Background(), "https://cdn.example/redirect", false); err == nil {
		t.Fatal("followed redirect outside source policy")
	}
	if requests != 4 {
		t.Fatalf("unexpected redirected network request: %d", requests)
	}
	other := Policy{Media: []string{"other.example"}}
	if err := other.CheckMediaAddress("https://cdn.example/resource"); err == nil {
		t.Fatal("independent policies shared a global domain configuration")
	}
	if err := policy.CheckMediaAddress("https://cdn.example/resource"); err != nil {
		t.Fatal(err)
	}
}
