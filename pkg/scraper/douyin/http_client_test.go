package douyin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDouyinBuiltInClientsDialIPv4(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	for name, create_client := range map[string]func() *http.Client{
		"web and mobile": new_douyin_web_http_client,
		"PC":             new_douyin_pc_http_client,
	} {
		t.Run(name, func(t *testing.T) {
			client := create_client()
			defer client.CloseIdleConnections()
			transport := client.Transport.(*http.Transport)
			if transport.Proxy != nil {
				t.Fatal("built-in client inherited environment proxy")
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			// Override the requested family even when a caller supplies tcp6.
			connection, err := transport.DialContext(ctx, "tcp6", server.Listener.Addr().String())
			if err != nil {
				t.Fatalf("did not dial the IPv4 listener: %v", err)
			}
			connection.Close()
			response, err := client.Get(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusNoContent {
				t.Fatalf("unexpected status: %d", response.StatusCode)
			}
			if name == "PC" && client.Jar == nil {
				t.Fatal("PC cookie jar was lost")
			}
		})
	}
}
