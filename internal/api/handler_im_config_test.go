package api

import (
	"testing"

	"wx_channel/pkg/configapi"
)

func TestFileHelperHandlerReloadsDeclaredConfig(t *testing.T) {
	store := configapi.NewStore()
	if err := store.Publish(map[string]any{
		"filehelper": map[string]any{"enabled": true, "callbackUrl": "https://example.test/one"},
	}); err != nil {
		t.Fatalf("publish initial config: %v", err)
	}
	handler := NewFileHelperHandler(store)
	if cfg := handler.current_config(); !cfg.Enabled || cfg.CallbackURL != "https://example.test/one" {
		t.Fatalf("initial config = %+v", cfg)
	}

	if err := store.Publish(map[string]any{
		"filehelper": map[string]any{"enabled": false, "callbackUrl": "https://example.test/two"},
	}); err != nil {
		t.Fatalf("publish updated config: %v", err)
	}
	if cfg := handler.current_config(); cfg.Enabled || cfg.CallbackURL != "https://example.test/two" {
		t.Fatalf("updated config = %+v", cfg)
	}

	handler.Close()
	if err := store.Publish(map[string]any{
		"filehelper": map[string]any{"enabled": true, "callbackUrl": "https://example.test/three"},
	}); err != nil {
		t.Fatalf("publish after close: %v", err)
	}
	if cfg := handler.current_config(); cfg.Enabled || cfg.CallbackURL != "https://example.test/two" {
		t.Fatalf("config changed after close = %+v", cfg)
	}
}
