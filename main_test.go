package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"

	"wx_channel/internal/config"
)

func TestAppLoggingRequiresExplicitOptIn(t *testing.T) {
	t.Setenv("ENABLE_LOG", "")
	t.Setenv("WX_ENABLE_LOG", "")
	original_level := zerolog.GlobalLevel()
	original_time_format := zerolog.TimeFieldFormat
	t.Cleanup(func() {
		zerolog.SetGlobalLevel(original_level)
		zerolog.TimeFieldFormat = original_time_format
		viper.Reset()
	})
	for _, test_case := range []struct {
		name         string
		settings     string
		env_name     string
		env_value    string
		existing_log bool
		enabled      bool
		log_only     bool
	}{
		{name: "default"},
		{name: "explicitly_disabled", settings: "debug:\n  enabled: false\n"},
		{name: "frontend_and_echo_options", settings: "debug:\n  error: true\n  echolog: true\n"},
		{name: "preserves_existing_log", existing_log: true},
		{name: "enabled", settings: "debug:\n  enabled: true\n", enabled: true},
		{name: "enabled_truncates_existing_log", settings: "debug:\n  enabled: true\n", existing_log: true, enabled: true},
		{name: "log_enabled", settings: "log:\n  enabled: true\n", enabled: true, log_only: true},
		{name: "enable_log_env", env_name: "ENABLE_LOG", env_value: "true", enabled: true, log_only: true},
		{name: "wx_enable_log_env", env_name: "WX_ENABLE_LOG", env_value: "1", enabled: true, log_only: true},
		{name: "disabled_log_env", env_name: "ENABLE_LOG", env_value: "false"},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			viper.Reset()
			if test_case.env_name != "" {
				t.Setenv(test_case.env_name, test_case.env_value)
			}
			test_dir := t.TempDir()
			log_dir := filepath.Join(test_dir, "logs")
			log_path := filepath.Join(log_dir, "app.log")
			t.Setenv("WX_LOG_DIR", log_dir)
			config_path := filepath.Join(test_dir, "config.yaml")
			t.Setenv(config.EnvConfigPath, config_path)
			settings := fmt.Sprintf("workdir: %q\n%s", filepath.Join(test_dir, "data"), test_case.settings)
			if err := os.WriteFile(config_path, []byte(settings), 0600); err != nil {
				t.Fatal(err)
			}
			const previous_log = "previous session\n"
			if test_case.existing_log {
				if err := os.MkdirAll(log_dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(log_path, []byte(previous_log), 0600); err != nil {
					t.Fatal(err)
				}
			}
			logger, log_writer, resolved_path := new_app_logger()
			t.Cleanup(func() { _ = log_writer.Close() })
			logger.Error().Msg("before configuration")
			cfg := config.New("test", "debug", logger, log_writer, resolved_path)
			if err := cfg.LoadConfig(); err != nil {
				t.Fatal(err)
			}
			if cfg.Debug != (test_case.enabled && !test_case.log_only) {
				t.Fatalf("debug = %v, enabled = %v, log_only = %v", cfg.Debug, test_case.enabled, test_case.log_only)
			}
			child_logger := logger.With().Str("component", "test").Logger()
			for _, level := range []zerolog.Level{zerolog.DebugLevel, zerolog.InfoLevel, zerolog.WarnLevel, zerolog.ErrorLevel, zerolog.NoLevel} {
				child_logger.WithLevel(level).Msg("after configuration: " + level.String())
			}
			if !test_case.enabled && !test_case.existing_log {
				if _, err := os.Stat(log_dir); !os.IsNotExist(err) {
					t.Fatalf("disabled logging touched log directory: %v", err)
				}
				return
			}
			contents, err := os.ReadFile(log_path)
			if err != nil {
				t.Fatal(err)
			}
			if !test_case.enabled {
				if string(contents) != previous_log {
					t.Fatalf("disabled logging changed existing log: %s", contents)
				}
				return
			}
			for _, message := range []string{"previous session", "before configuration"} {
				if strings.Contains(string(contents), message) {
					t.Fatalf("unexpected log message %q: %s", message, contents)
				}
			}
			for _, level := range []string{"debug", "info", "warn", "error"} {
				if level == "debug" && test_case.log_only {
					if strings.Contains(string(contents), "after configuration: debug") {
						t.Fatalf("log-only mode recorded debug output: %s", contents)
					}
					continue
				}
				if !strings.Contains(string(contents), "after configuration: "+level) {
					t.Fatalf("missing %s log: %s", level, contents)
				}
			}
		})
	}
}
