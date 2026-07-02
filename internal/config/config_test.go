package config

import (
	"os"
	"path/filepath"
	"strings"
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

func TestIsMPEnabledPrefersEnabled(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()
	loadTestConfig(t, "mp:\n  enabled: true\n  disabled: true\n")

	if !IsMPEnabled() {
		t.Fatal("IsMPEnabled() = false, want true when mp.enabled is true")
	}
}

func TestIsMPEnabledPrefersEnabledFalse(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()
	loadTestConfig(t, "mp:\n  enabled: false\n  disabled: false\n")

	if IsMPEnabled() {
		t.Fatal("IsMPEnabled() = true, want false when mp.enabled is false")
	}
}

func TestIsMPEnabledFallsBackToDisabled(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()
	loadTestConfig(t, "mp:\n  disabled: false\n")

	if !IsMPEnabled() {
		t.Fatal("IsMPEnabled() = false, want true when mp.disabled is false")
	}
}

func TestIsMPEnabledDefaultDisabled(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()

	cfg := &Config{Existing: false}
	if err := cfg.LoadConfig(); err != nil {
		t.Fatalf("load config: %v", err)
	}

	if IsMPEnabled() {
		t.Fatal("IsMPEnabled() = true, want false by default")
	}
}

func TestSaveDoesNotPersistMPEnabledDefault(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()
	cfg := loadTestConfig(t, "mp:\n  disabled: false\n")

	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	saved, err := os.ReadFile(cfg.FullPath)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if yamlSectionHasKey(string(saved), "mp", "enabled") {
		t.Fatalf("saved config unexpectedly contains mp.enabled:\n%s", string(saved))
	}
}

func TestMPDisabledConfigItemIsDeprecated(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()

	cfg := &Config{Existing: false}
	if err := cfg.LoadConfig(); err != nil {
		t.Fatalf("load config: %v", err)
	}

	for _, item := range GetSchema() {
		if item.Key == "mp.disabled" {
			if !item.Deprecated {
				t.Fatal("mp.disabled Deprecated = false, want true")
			}
			return
		}
	}
	t.Fatal("mp.disabled config item not found")
}
func loadTestConfig(t *testing.T, data string) *Config {
	t.Helper()

	rootDir := t.TempDir()
	configPath := filepath.Join(rootDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(data), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	viper.SetConfigFile(configPath)
	cfg := &Config{
		RootDir:  rootDir,
		Filename: "config.yaml",
		FullPath: configPath,
		Existing: true,
	}
	if err := cfg.LoadConfig(); err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}

func yamlSectionHasKey(data, section, key string) bool {
	inSection := false
	for _, line := range strings.Split(data, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if indent == 0 {
			inSection = strings.TrimSuffix(trimmed, ":") == section
			continue
		}
		if inSection && strings.HasPrefix(trimmed, key+":") {
			return true
		}
	}
	return false
}
