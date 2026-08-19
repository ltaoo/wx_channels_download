package worker

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type round_trip_func func(*http.Request) (*http.Response, error)

func (round_trip round_trip_func) RoundTrip(request *http.Request) (*http.Response, error) {
	return round_trip(request)
}

func TestGetSubdomain(t *testing.T) {
	http_client := &http.Client{Transport: round_trip_func(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet {
			t.Fatalf("unexpected method %s", request.Method)
		}
		if request.URL.String() != "https://api.example.test/accounts/account/workers/subdomain" {
			t.Fatalf("unexpected URL %s", request.URL)
		}
		if authorization := request.Header.Get("Authorization"); authorization != "Bearer api-token" {
			t.Fatalf("unexpected authorization header %q", authorization)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(
				`{"success":true,"errors":[],"result":{"subdomain":"account-subdomain"}}`,
			)),
		}, nil
	})}
	api_client := NewClient(ClientOptions{
		BaseURL:    "https://api.example.test",
		HTTPClient: http_client,
	})

	subdomain, err := api_client.GetSubdomain(
		context.Background(),
		"account",
		"api-token",
	)
	if err != nil {
		t.Fatalf("GetSubdomain returned error: %v", err)
	}
	if subdomain != "account-subdomain" {
		t.Fatalf("unexpected subdomain %q", subdomain)
	}
}
