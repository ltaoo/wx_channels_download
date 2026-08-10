package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func TestLoadConfigResolvesRuntimeFilesFromWorkDir(t *testing.T) {
	reset_config_test_state(t)

	root_dir := t.TempDir()
	work_dir := filepath.Join(root_dir, "runtime")
	if err := os.MkdirAll(work_dir, 0755); err != nil {
		t.Fatalf("create workdir: %v", err)
	}
	for _, filename := range []string{"global.js", "hooks.js"} {
		if err := os.WriteFile(filepath.Join(work_dir, filename), []byte("// test"), 0644); err != nil {
			t.Fatalf("create %s: %v", filename, err)
		}
	}

	config_path := filepath.Join(root_dir, "config.yaml")
	config_data := []byte("workdir: ./runtime\n")
	if err := os.WriteFile(config_path, config_data, 0644); err != nil {
		t.Fatalf("create config: %v", err)
	}
	viper.SetConfigFile(config_path)
	logger := zerolog.Nop()
	cfg := &Config{
		RootDir:  root_dir,
		Filename: filepath.Base(config_path),
		FullPath: config_path,
		Existing: true,
		logger:   &logger,
	}

	if err := cfg.LoadConfig(); err != nil {
		t.Fatalf("load config: %v", err)
	}

	assert_config_path(t, "workdir", cfg.WorkDir, work_dir)
	assert_config_path(t, "database", cfg.DBPath, filepath.Join(work_dir, "data.db"))
	assert_config_path(t, "global script", cfg.GlobalScriptPath, filepath.Join(work_dir, "global.js"))
	assert_config_path(t, "hooks script", cfg.HookScriptPath, filepath.Join(work_dir, "hooks.js"))
}

func TestLoadConfigCreatesWorkDir(t *testing.T) {
	reset_config_test_state(t)

	root_dir := t.TempDir()
	config_path := filepath.Join(root_dir, "config.yaml")
	if err := os.WriteFile(config_path, []byte("workdir: ./runtime/nested\n"), 0644); err != nil {
		t.Fatalf("create config: %v", err)
	}
	viper.SetConfigFile(config_path)
	logger := zerolog.Nop()
	cfg := &Config{
		RootDir:  root_dir,
		Filename: filepath.Base(config_path),
		FullPath: config_path,
		Existing: true,
		logger:   &logger,
	}

	if err := cfg.LoadConfig(); err != nil {
		t.Fatalf("load config: %v", err)
	}
	if info, err := os.Stat(cfg.WorkDir); err != nil {
		t.Fatalf("stat workdir: %v", err)
	} else if !info.IsDir() {
		t.Fatalf("workdir is not a directory: %s", cfg.WorkDir)
	}
}

func reset_config_test_state(t *testing.T) {
	t.Helper()
	previous_registry := Registry
	Registry = nil
	viper.Reset()
	t.Cleanup(func() {
		Registry = previous_registry
		viper.Reset()
	})
}

func assert_config_path(t *testing.T, name string, actual string, expected string) {
	t.Helper()
	if actual != expected {
		t.Fatalf("%s path = %q, want %q", name, actual, expected)
	}
}
