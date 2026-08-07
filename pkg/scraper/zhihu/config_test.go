package zhihu

import (
	"testing"

	"wx_channel/pkg/configapi"
)

func TestClientReadsDeclaredCookieAtRuntime(t *testing.T) {
	store := configapi.NewStore()
	if err := store.Publish(map[string]any{
		"zhihu": map[string]any{"cookie": "cookie-one"},
	}); err != nil {
		t.Fatalf("publish initial config: %v", err)
	}
	client := NewClientWithConfig(ClientConfig{ConfigProvider: store})
	if cookie := client.cookie(); cookie != "cookie-one" {
		t.Fatalf("initial cookie = %q", cookie)
	}

	if err := store.Publish(map[string]any{
		"zhihu": map[string]any{"cookie": "cookie-two"},
	}); err != nil {
		t.Fatalf("publish updated config: %v", err)
	}
	if cookie := client.cookie(); cookie != "cookie-two" {
		t.Fatalf("updated cookie = %q", cookie)
	}
}
