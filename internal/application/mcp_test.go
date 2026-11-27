package application

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"wx_channel/internal/api"
	"wx_channel/pkg/dm"
)

func TestInternalMCPClientUsesConfiguredAPICredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "owner" || password != "s:e@cret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"code":0,"data":{"version":"test"}}`))
	}))
	defer server.Close()
	address, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(address.Port())
	if err != nil {
		t.Fatal(err)
	}
	client, err := dm.NewClient(dm.ClientOptions{BaseURL: mcp_api_base_url(&api.APIConfig{Hostname: address.Hostname(), Port: port, Username: "owner", Password: "s:e@cret"})})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Status(context.Background()); err != nil {
		t.Fatal(err)
	}
}
