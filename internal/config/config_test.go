package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestNewUsesConfigPathFromEnv(t *testing.T) {
	t.Cleanup(viper.Reset)

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("proxy:\n  hostname: 0.0.0.0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvConfigPath, configPath)

	cfg := New("test", "")

	if cfg.FullPath != configPath {
		t.Fatalf("FullPath = %q, want %q", cfg.FullPath, configPath)
	}
	if cfg.RootDir != filepath.Dir(configPath) {
		t.Fatalf("RootDir = %q, want %q", cfg.RootDir, filepath.Dir(configPath))
	}
	if cfg.Filename != filepath.Base(configPath) {
		t.Fatalf("Filename = %q, want %q", cfg.Filename, filepath.Base(configPath))
	}
	if !cfg.Existing {
		t.Fatal("Existing = false, want true")
	}
}
