package configapi

import (
	"context"
	"errors"
	"testing"
)

func test_manager(t *testing.T) (*Manager, *MemorySource) {
	t.Helper()
	manager := NewManager()
	_, err := manager.RegisterModule(DeclareModule("core",
		Item{Key: "proxy.port", Type: TypeInt, Default: 2023, Min: float_pointer(1), Max: float_pointer(65535), Reload: ReloadComponent},
		Item{Key: "proxy.system", Type: TypeBool, Default: true, Reload: ReloadHot},
		Item{Key: "secret.token", Type: TypeString, Default: "default-secret", Sensitive: true, Reload: ReloadHot},
	))
	if err != nil {
		t.Fatalf("register module: %v", err)
	}
	runtime_source := MustMemorySource("runtime", PriorityRuntime, map[string]any{})
	if err := manager.AddSource(runtime_source); err != nil {
		t.Fatalf("add runtime source: %v", err)
	}
	if err := manager.SetDefaultWriteSource("runtime"); err != nil {
		t.Fatalf("set write source: %v", err)
	}
	if _, err := manager.Refresh(context.Background()); err != nil {
		t.Fatalf("initial refresh: %v", err)
	}
	return manager, runtime_source
}

func TestManagerMergesSourcesAndTracksProvenance(t *testing.T) {
	manager, _ := test_manager(t)
	cli_source, err := NewCLISource(map[string]any{"proxy.port": 4040})
	if err != nil {
		t.Fatalf("new CLI source: %v", err)
	}
	if err := manager.AddSource(cli_source); err != nil {
		t.Fatalf("add CLI source: %v", err)
	}
	result, err := manager.Refresh(context.Background())
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if result.Revision == 0 {
		t.Fatal("revision was not published")
	}
	var proxy_config struct {
		Port   int  `json:"port"`
		System bool `json:"system"`
	}
	if err := manager.Snapshot("proxy").Decode(&proxy_config); err != nil {
		t.Fatalf("decode proxy config: %v", err)
	}
	if proxy_config.Port != 4040 || !proxy_config.System {
		t.Fatalf("proxy config = %+v", proxy_config)
	}
	view := manager.View(false)
	port_entry := find_entry(t, view, "proxy.port")
	if port_entry.Source != "cli" || port_entry.Writable {
		t.Fatalf("port entry = %+v", port_entry)
	}
}

func TestManagerApplyValidatesBeforeWriting(t *testing.T) {
	manager, runtime_source := test_manager(t)
	initial_revision := manager.Revision()
	_, err := manager.Apply(context.Background(), UpdateRequest{
		Values:           map[string]any{"proxy.port": 70000},
		ExpectedRevision: initial_revision,
	})
	if err == nil {
		t.Fatal("invalid update succeeded")
	}
	var validation_error *ValidationError
	if !errors.As(err, &validation_error) {
		t.Fatalf("error = %T %v, want ValidationError", err, err)
	}
	if manager.Revision() != initial_revision {
		t.Fatalf("revision changed from %d to %d", initial_revision, manager.Revision())
	}
	layer, err := runtime_source.Load(context.Background())
	if err != nil {
		t.Fatalf("load runtime source: %v", err)
	}
	if _, exists := lookup_path(layer, "proxy.port"); exists {
		t.Fatalf("invalid value was written: %v", layer)
	}
}

func TestManagerApplyPublishesTypedValueAndDetectsRevisionConflict(t *testing.T) {
	manager, _ := test_manager(t)
	initial_revision := manager.Revision()
	result, err := manager.Apply(context.Background(), UpdateRequest{
		Values:           map[string]any{"proxy.port": "3030", "secret.token": "top-secret"},
		ExpectedRevision: initial_revision,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if result.Revision <= initial_revision {
		t.Fatalf("revision = %d, initial = %d", result.Revision, initial_revision)
	}
	var proxy_config struct {
		Port int `json:"port"`
	}
	if err := manager.Snapshot("proxy").Decode(&proxy_config); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if proxy_config.Port != 3030 {
		t.Fatalf("port = %d", proxy_config.Port)
	}
	secret_entry := find_entry(t, manager.View(true), "secret.token")
	if secret_entry.Value != "********" {
		t.Fatalf("redacted secret = %v", secret_entry.Value)
	}
	if secret_entry.Item.Default != "********" {
		t.Fatalf("redacted secret default = %v", secret_entry.Item.Default)
	}
	for _, change := range result.Changes {
		if change.Key == "secret.token" && (change.OldValue != "********" || change.NewValue != "********") {
			t.Fatalf("secret change was not redacted: %+v", change)
		}
	}
	if _, err := manager.Apply(context.Background(), UpdateRequest{
		Values:           map[string]any{"proxy.port": 3031},
		ExpectedRevision: initial_revision,
	}); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("revision conflict error = %v", err)
	}
}

func TestManagerCoalescesEffectiveNoopUnderHigherPrioritySource(t *testing.T) {
	manager, runtime_source := test_manager(t)
	cli_source, _ := NewCLISource(map[string]any{"proxy.port": 4040})
	if err := manager.AddSource(cli_source); err != nil {
		t.Fatalf("add CLI source: %v", err)
	}
	if _, err := manager.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	revision := manager.Revision()
	result, err := manager.Apply(context.Background(), UpdateRequest{Values: map[string]any{"proxy.port": 3030}})
	if err != nil {
		t.Fatalf("apply lower priority value: %v", err)
	}
	if result.Revision != revision {
		t.Fatalf("no-op effective update changed revision to %d", result.Revision)
	}
	layer, _ := runtime_source.Load(context.Background())
	value, _ := lookup_path(layer, "proxy.port")
	if value != float64(3030) {
		t.Fatalf("stored lower-priority value = %v", value)
	}
}

func TestModuleUnregisterRemovesSchemaButPreservesSourceValue(t *testing.T) {
	manager := NewManager()
	handle, err := manager.RegisterModule(DeclareModule("adapter", Item{Key: "adapter.enabled", Type: TypeBool, Default: false}))
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	source := MustMemorySource("database", PriorityDatabase, map[string]any{"adapter": map[string]any{"enabled": true}})
	if err := manager.AddSource(source); err != nil {
		t.Fatalf("add source: %v", err)
	}
	if _, err := manager.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	handle.Unregister()
	if len(manager.Schema()) != 0 {
		t.Fatalf("schema remains after unregister: %+v", manager.Schema())
	}
	if got := manager.Snapshot("adapter").Values()["enabled"]; got != true {
		t.Fatalf("source value after unregister = %v", got)
	}
}

func find_entry(t *testing.T, view View, key string) Entry {
	t.Helper()
	for _, entry := range view.Items {
		if entry.Item.Key == key {
			return entry
		}
	}
	t.Fatalf("entry %s not found", key)
	return Entry{}
}

func float_pointer(value float64) *float64 { return &value }
