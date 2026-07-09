package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigReadsRepoDotEnv(t *testing.T) {
	repoRoot := newAssetSyncTestRepo(t)
	home := filepath.Join(repoRoot, "home")
	unsetEnv(t, "ASSET_SYNC_STORAGE_ROOT")
	t.Setenv("HOME", home)
	withCommandGlobals(t, repoRoot)

	if err := os.WriteFile(filepath.Join(repoRoot, ".env"), []byte(`ASSET_SYNC_STORAGE_ROOT="$HOME/SynologyDrive/asset-sync-store"`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, cfg, _, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "SynologyDrive", "asset-sync-store")
	if cfg.Storage.LocalPath != want {
		t.Fatalf("unexpected local path: got %q want %q", cfg.Storage.LocalPath, want)
	}
}

func TestLoadConfigDoesNotOverrideExistingEnvironment(t *testing.T) {
	repoRoot := newAssetSyncTestRepo(t)
	withCommandGlobals(t, repoRoot)
	t.Setenv("ASSET_SYNC_STORAGE_ROOT", "/from/shell")

	if err := os.WriteFile(filepath.Join(repoRoot, ".env"), []byte("ASSET_SYNC_STORAGE_ROOT=/from/dotenv\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, cfg, _, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Storage.LocalPath != "/from/shell" {
		t.Fatalf("expected existing env to win, got %q", cfg.Storage.LocalPath)
	}
}

func TestParseEnvLine(t *testing.T) {
	t.Setenv("ASSET_SYNC_TEST_HOME", "/tmp/home")

	tests := []struct {
		name  string
		line  string
		key   string
		value string
		ok    bool
	}{
		{name: "blank", line: " ", ok: false},
		{name: "comment", line: "# comment", ok: false},
		{name: "export", line: "export FOO=bar", key: "FOO", value: "bar", ok: true},
		{name: "double quoted expands", line: `ROOT="$ASSET_SYNC_TEST_HOME/store"`, key: "ROOT", value: "/tmp/home/store", ok: true},
		{name: "single quoted literal", line: `ROOT='$ASSET_SYNC_TEST_HOME/store'`, key: "ROOT", value: "$ASSET_SYNC_TEST_HOME/store", ok: true},
		{name: "inline comment", line: "ROOT=/tmp/store # local path", key: "ROOT", value: "/tmp/store", ok: true},
		{name: "hash in value", line: "TOKEN=abc#123", key: "TOKEN", value: "abc#123", ok: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, value, ok, err := parseEnvLine(tt.line)
			if err != nil {
				t.Fatal(err)
			}
			if ok != tt.ok || key != tt.key || value != tt.value {
				t.Fatalf("got key=%q value=%q ok=%v", key, value, ok)
			}
		})
	}
}

func newAssetSyncTestRepo(t *testing.T) string {
	t.Helper()
	repoRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoRoot, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	config := []byte(`version: 1
manifest: .asset-sync.lock.json
roots:
  - path: scraper_examples
storage:
  type: local
  local_path: ${ASSET_SYNC_STORAGE_ROOT}
  prefix: wx_channels_download
`)
	if err := os.WriteFile(filepath.Join(repoRoot, ".asset-sync.yaml"), config, 0644); err != nil {
		t.Fatal(err)
	}
	return repoRoot
}

func withCommandGlobals(t *testing.T, repoRoot string) {
	t.Helper()
	oldRepoFlag := repoFlag
	oldConfigFlag := configFlag
	oldDeviceFlag := deviceFlag
	repoFlag = repoRoot
	configFlag = ""
	deviceFlag = ""
	t.Cleanup(func() {
		repoFlag = oldRepoFlag
		configFlag = oldConfigFlag
		deviceFlag = oldDeviceFlag
	})
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	oldValue, oldExists := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if oldExists {
			_ = os.Setenv(key, oldValue)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}
