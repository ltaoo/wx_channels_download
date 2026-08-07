package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"wx_channel/pkg/configapi"
)

func TestSavePublishesRuntimeConfig(t *testing.T) {
	cfg := &Config{FullPath: filepath.Join(t.TempDir(), "config.yaml")}
	register_config_test_module(t, cfg, "test.save", configapi.Item{
		Key:     "channels.refreshInterval",
		Type:    configapi.TypeInt,
		Default: 0,
		Reload:  configapi.ReloadHot,
	})
	initial_revision := cfg.Revision()
	var received configapi.Snapshot
	unsubscribe := cfg.Subscribe("channels", func(snapshot configapi.Snapshot) {
		received = snapshot
	})
	defer unsubscribe()

	cfg.Update("channels.refreshInterval", 7)
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	var channels_config struct {
		RefreshInterval int `json:"refreshInterval"`
	}
	if err := received.Decode(&channels_config); err != nil {
		t.Fatalf("decode published config: %v", err)
	}
	if channels_config.RefreshInterval != 7 {
		t.Fatalf("refresh interval = %d, want 7", channels_config.RefreshInterval)
	}
	if received.Revision() != initial_revision+1 {
		t.Fatalf("revision = %d, want %d", received.Revision(), initial_revision+1)
	}
}

func TestLoadConfigBuildsManagerAndGUIView(t *testing.T) {
	root_dir := t.TempDir()
	cfg := &Config{
		RootDir:  root_dir,
		FullPath: filepath.Join(root_dir, "config.yaml"),
		Filename: "config.yaml",
	}
	if err := cfg.LoadConfig(); err != nil {
		t.Fatalf("load config: %v", err)
	}
	view := cfg.View(true)
	if view.Revision == 0 {
		t.Fatal("manager did not publish initial configuration")
	}
	if len(view.Items) == 0 {
		t.Fatal("GUI schema is empty")
	}
	if len(view.Sources) < 2 {
		t.Fatalf("sources = %+v", view.Sources)
	}
	for _, key := range []string{"pagespy.enabled", "debug.error"} {
		if !has_config_schema_item(view.Items, key) {
			t.Fatalf("common configuration item %s is missing", key)
		}
	}
	for _, key := range []string{"proxy.port", "download.dir", "api.port", "filehelper.enabled"} {
		if has_config_schema_item(view.Items, key) {
			t.Fatalf("module-owned configuration item %s was registered by LoadConfig", key)
		}
	}
}

func TestManagerApplyPersistsAndPublishesRuntimeConfig(t *testing.T) {
	cfg := &Config{FullPath: filepath.Join(t.TempDir(), "config.yaml")}
	register_config_test_module(t, cfg, "test.runtime_apply", configapi.Item{
		Key:     "runtime_test.enabled",
		Type:    configapi.TypeBool,
		Default: false,
		Reload:  configapi.ReloadHot,
	})
	manager := cfg.Manager()
	if manager == nil {
		t.Fatal("manager is nil")
	}
	var received configapi.Snapshot
	unsubscribe := cfg.Subscribe("runtime_test", func(snapshot configapi.Snapshot) {
		received = snapshot
	})
	defer unsubscribe()
	update, err := cfg.Apply(context.Background(), configapi.UpdateRequest{
		Values:           map[string]any{"runtime_test.enabled": true},
		ExpectedRevision: cfg.Revision(),
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if update.Revision == 0 || received.Revision() != update.Revision {
		t.Fatalf("update revision = %d, received = %d", update.Revision, received.Revision())
	}
	if enabled := received.Values()["enabled"]; enabled != true {
		t.Fatalf("published enabled = %v", enabled)
	}
	if _, err := os.Stat(cfg.FullPath); err != nil {
		t.Fatalf("persisted config: %v", err)
	}
}

func TestLoadConfigUsesFileAndExplicitValuesOverride(t *testing.T) {
	root_dir := t.TempDir()
	config_path := filepath.Join(root_dir, "config.yaml")
	if err := os.WriteFile(config_path, []byte("pagespy:\n  api: file.example:6752\nworkdir: ./runtime\n"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg := New(config_path, map[string]any{"pagespy.api": "values.example:6752"})
	if err := cfg.LoadConfig(); err != nil {
		t.Fatalf("load config: %v", err)
	}
	if api := cfg.GetString("pagespy.api"); api != "values.example:6752" {
		t.Fatalf("pagespy API = %q, want explicit values source", api)
	}
	api_entry := config_entry(t, cfg.View(false), "pagespy.api")
	if api_entry.Source != "values" || api_entry.Writable {
		t.Fatalf("values entry = %+v", api_entry)
	}
}

func TestNewSelectsExplicitExistingFile(t *testing.T) {
	root_dir := t.TempDir()
	config_path := filepath.Join(root_dir, "custom.yaml")
	if err := os.WriteFile(config_path, []byte("proxy:\n  port: 3030\n"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg := New(config_path, nil)
	if cfg.RootDir != root_dir || cfg.Filename != "custom.yaml" || cfg.FullPath != config_path || !cfg.Existing {
		t.Fatalf("config file metadata = %+v", cfg)
	}
	missing_cfg := New(filepath.Join(root_dir, "missing.yaml"), nil)
	if err := missing_cfg.LoadConfig(); err == nil {
		t.Fatal("missing explicit config file was accepted")
	}
}

func TestRefreshReloadsChangedConfigFile(t *testing.T) {
	config_path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(config_path, []byte("file_reload:\n  enabled: false\n"), 0600); err != nil {
		t.Fatalf("write initial config: %v", err)
	}
	cfg := &Config{FullPath: config_path, Existing: true}
	register_config_test_module(t, cfg, "test.file_reload", configapi.Item{
		Key:     "file_reload.enabled",
		Type:    configapi.TypeBool,
		Default: false,
		Reload:  configapi.ReloadHot,
	})
	if manager := cfg.Manager(); manager == nil {
		t.Fatal("manager is nil")
	}
	if err := os.WriteFile(config_path, []byte("file_reload:\n  enabled: true\n"), 0600); err != nil {
		t.Fatalf("write changed config: %v", err)
	}
	result, err := cfg.Refresh(context.Background())
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if result.Revision == 0 || !cfg.GetBool("file_reload.enabled") {
		t.Fatalf("file reload did not publish: %+v", result)
	}
}

func config_entry(t *testing.T, view configapi.View, key string) configapi.Entry {
	t.Helper()
	for _, entry := range view.Items {
		if entry.Item.Key == key {
			return entry
		}
	}
	t.Fatalf("configuration item %s not found", key)
	return configapi.Entry{}
}

func has_config_schema_item(entries []configapi.Entry, key string) bool {
	for _, entry := range entries {
		if entry.Item.Key == key {
			return true
		}
	}
	return false
}

func register_config_test_module(t *testing.T, cfg *Config, name string, items ...configapi.Item) {
	t.Helper()
	handle, err := cfg.RegisterModule(configapi.DeclareModule(name, items...))
	if err != nil {
		t.Fatalf("register test config module: %v", err)
	}
	t.Cleanup(handle.Unregister)
	if _, err := cfg.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh test config module: %v", err)
	}
}
