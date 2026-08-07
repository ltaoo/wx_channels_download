package application

import (
	"os"
	"path/filepath"
	"testing"

	"wx_channel/internal/config"
	"wx_channel/pkg/configapi"
)

func TestStartConfigOwnsAndResolvesBootParameters(t *testing.T) {
	root_dir := t.TempDir()
	config_path := filepath.Join(root_dir, "config.yaml")
	global_script_path := filepath.Join(root_dir, "global.js")
	if err := os.WriteFile(global_script_path, []byte("window.test = true"), 0600); err != nil {
		t.Fatalf("write global script: %v", err)
	}
	config_content := []byte("workdir: ./runtime\ninject:\n  globalScript: global.js\ndb:\n  type: sqlite\n  filepath: '%CWD%/application.db'\nproxy:\n  port: 3131\ndownload:\n  dir: '%CWD%/downloads'\n")
	if err := os.WriteFile(config_path, config_content, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	provider := config.New(config_path, map[string]any{
		"version": "test-version",
		"mode":    "test",
	})
	if err := provider.LoadConfig(); err != nil {
		t.Fatalf("load config: %v", err)
	}
	start_cfg, err := NewStartConfig(provider)
	if err != nil {
		t.Fatalf("new start config: %v", err)
	}

	want_workdir := filepath.Join(root_dir, "runtime")
	if start_cfg.WorkDir != want_workdir {
		t.Fatalf("workdir = %q, want %q", start_cfg.WorkDir, want_workdir)
	}
	if start_cfg.Database.Type != "sqlite" || start_cfg.Database.Path != filepath.Join(want_workdir, "application.db") {
		t.Fatalf("database config = %+v", start_cfg.Database)
	}
	if start_cfg.Version != "test-version" || start_cfg.Mode != "test" {
		t.Fatalf("build metadata = version %q mode %q", start_cfg.Version, start_cfg.Mode)
	}
	version := application_config_entry(t, provider.View(false), "version")
	if version.Item.Reload != configapi.ReloadBootOnly || version.Source != "values" || version.Writable {
		t.Fatalf("version config = %+v", version)
	}
	if start_cfg.GlobalScriptPath != global_script_path || start_cfg.GlobalScriptContent != "window.test = true" {
		t.Fatalf("global script = %q %q", start_cfg.GlobalScriptPath, start_cfg.GlobalScriptContent)
	}
	var proxy_config struct {
		Port int `json:"port"`
	}
	if err := provider.Snapshot("proxy").Decode(&proxy_config); err != nil || proxy_config.Port != 3131 {
		t.Fatalf("proxy config = %+v, err = %v", proxy_config, err)
	}
	proxy_port := application_config_entry(t, provider.View(false), "proxy.port")
	if proxy_port.Item.Reload != configapi.ReloadProcess || proxy_port.Source != "user-file" {
		t.Fatalf("proxy port schema = %+v", proxy_port)
	}
	download_dir := application_config_entry(t, provider.View(false), "download.dir")
	if download_dir.Source != "user-file" {
		t.Fatalf("download directory schema = %+v", download_dir)
	}

	db_password := application_config_entry(t, provider.View(false), "db.password")
	if db_password.Item.Reload != configapi.ReloadBootOnly || !db_password.Item.Sensitive {
		t.Fatalf("database password schema = %+v", db_password.Item)
	}
}

func TestStartConfigAppliesDatabasePortDefaults(t *testing.T) {
	root_dir := t.TempDir()
	config_path := filepath.Join(root_dir, "config.yaml")
	if err := os.WriteFile(config_path, []byte("db:\n  type: postgres\n"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	provider := config.New(config_path, nil)
	if err := provider.LoadConfig(); err != nil {
		t.Fatalf("load config: %v", err)
	}

	start_cfg, err := NewStartConfig(provider)
	if err != nil {
		t.Fatalf("new start config: %v", err)
	}
	if start_cfg.Database.Port != 5432 {
		t.Fatalf("postgres port = %d, want 5432", start_cfg.Database.Port)
	}
}

func application_config_entry(t *testing.T, view configapi.View, key string) configapi.Entry {
	t.Helper()
	for _, entry := range view.Items {
		if entry.Item.Key == key {
			return entry
		}
	}
	t.Fatalf("configuration item %s not found", key)
	return configapi.Entry{}
}
