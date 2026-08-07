package api

import (
	"path/filepath"
	"testing"

	"wx_channel/pkg/configapi"
)

func TestNewAPIConfigUsesDeclaredSnapshots(t *testing.T) {
	work_dir := t.TempDir()
	manager := configapi.NewManager()
	source, err := configapi.NewMemorySource("test", configapi.PriorityRuntime, map[string]any{
		"api": map[string]any{
			"protocol": "http",
			"hostname": "127.0.0.1",
			"port":     3030,
		},
		"download": map[string]any{
			"dir":              "%CWD%/downloads",
			"maxRunning":       7,
			"filenameTemplate": "{{filename}}",
		},
		"cloudflare": map[string]any{"sphCookie": "cookie-value"},
	})
	if err != nil {
		t.Fatalf("new source: %v", err)
	}
	if err := manager.AddSource(source); err != nil {
		t.Fatalf("add source: %v", err)
	}

	cfg, err := NewAPIConfig(APIConfigSource{
		Provider: manager,
		Runtime: configapi.Runtime{
			Version: "test-version",
			Mode:    "test",
			RootDir: work_dir,
			WorkDir: work_dir,
		},
	})
	if err != nil {
		t.Fatalf("new API config: %v", err)
	}
	if cfg.Port != 3030 {
		t.Fatalf("port = %d, want 3030", cfg.Port)
	}
	if cfg.MaxRunning != 7 {
		t.Fatalf("max running = %d, want 7", cfg.MaxRunning)
	}
	if cfg.DownloadDir != filepath.Join(work_dir, "downloads") {
		t.Fatalf("download dir = %q", cfg.DownloadDir)
	}
	if cfg.CloudflareSphCookie != "cookie-value" {
		t.Fatalf("cloudflare cookie = %q", cfg.CloudflareSphCookie)
	}
	if !has_api_schema_item(manager.Schema(), "api.port") || !has_api_schema_item(manager.Schema(), "filehelper.enabled") {
		t.Fatalf("API module schema was not registered: %+v", manager.Schema())
	}
}

func has_api_schema_item(items []configapi.Item, key string) bool {
	for _, item := range items {
		if item.Key == key {
			return true
		}
	}
	return false
}
