package configapi

import "testing"

func TestDeclarationRejectsUndeclaredNamespace(t *testing.T) {
	store := NewStore()
	if err := store.Publish(map[string]any{"api": map[string]any{"port": 2022}}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	declaration := Declare("api")

	var api_config struct {
		Port int `json:"port"`
	}
	if err := declaration.Decode(store, "api", &api_config); err != nil {
		t.Fatalf("decode declared namespace: %v", err)
	}
	if api_config.Port != 2022 {
		t.Fatalf("port = %d, want 2022", api_config.Port)
	}
	if err := declaration.Decode(store, "proxy", &struct{}{}); err == nil {
		t.Fatal("decode undeclared namespace succeeded")
	}
}

func TestDeclarationSubscriptionCoalescesPublication(t *testing.T) {
	store := NewStore()
	declaration := Declare("api", "download")
	callback_count := 0
	unsubscribe, err := declaration.Subscribe(store, func(uint64) {
		callback_count++
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer unsubscribe()

	if err := store.Publish(map[string]any{
		"api":      map[string]any{"port": 2022},
		"download": map[string]any{"dir": "/tmp/downloads"},
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if callback_count != 1 {
		t.Fatalf("callback count = %d, want 1", callback_count)
	}
}
