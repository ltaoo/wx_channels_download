package interceptor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"

	"wx_channel/internal/config"
)

func TestNewInterceptorSettingsPrefersConfiguredGlobalScript(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	rootDir := t.TempDir()
	defaultScriptPath := filepath.Join(rootDir, "global.js")
	configuredScriptPath := filepath.Join(rootDir, "configured.js")

	if err := os.WriteFile(defaultScriptPath, []byte("default script"), 0644); err != nil {
		t.Fatalf("write default global script: %v", err)
	}
	if err := os.WriteFile(configuredScriptPath, []byte("configured script"), 0644); err != nil {
		t.Fatalf("write configured global script: %v", err)
	}
	configPath := filepath.Join(rootDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("inject:\n  globalScript: configured.js\n"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	viper.SetConfigFile(configPath)
	cfg := &config.Config{
		RootDir:  rootDir,
		Filename: "config.yaml",
		FullPath: configPath,
		Existing: true,
		Version:  "test",
	}
	if err := cfg.LoadConfig(); err != nil {
		t.Fatalf("load config: %v", err)
	}

	settings := NewInterceptorSettings(cfg)

	if settings.InjectGlobalScript != "configured script" {
		t.Fatalf("InjectGlobalScript = %q, want configured script", settings.InjectGlobalScript)
	}
	if settings.InjectGlobalScriptFilepath != configuredScriptPath {
		t.Fatalf("InjectGlobalScriptFilepath = %q, want %q", settings.InjectGlobalScriptFilepath, configuredScriptPath)
	}
}

func TestNewInterceptorSettingsSeparatesEchoLogFromDebugError(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	rootDir := t.TempDir()
	configPath := filepath.Join(rootDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("debug:\n  error: true\n  echolog: false\n"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	viper.SetConfigFile(configPath)
	cfg := &config.Config{
		RootDir:  rootDir,
		Filename: "config.yaml",
		FullPath: configPath,
		Existing: true,
		Version:  "test",
	}
	if err := cfg.LoadConfig(); err != nil {
		t.Fatalf("load config: %v", err)
	}

	settings := NewInterceptorSettings(cfg)

	if !settings.DebugShowError {
		t.Fatal("DebugShowError = false, want true")
	}
	if settings.EchoLogEnabled {
		t.Fatal("EchoLogEnabled = true, want false")
	}
}
