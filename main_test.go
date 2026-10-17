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

func TestAppLoggingRequiresExplicitDebug(t *testing.T) {
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
		existing_log bool
		enabled      bool
	}{
		{name: "default"},
		{name: "explicitly_disabled", settings: "debug:\n  enabled: false\n"},
		{name: "frontend_and_echo_options", settings: "debug:\n  error: true\n  echolog: true\n"},
		{name: "preserves_existing_log", existing_log: true},
		{name: "enabled", settings: "debug:\n  enabled: true\n", enabled: true},
		{name: "enabled_truncates_existing_log", settings: "debug:\n  enabled: true\n", existing_log: true, enabled: true},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			viper.Reset()
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
			if cfg.Debug != test_case.enabled {
				t.Fatalf("debug = %v, want %v", cfg.Debug, test_case.enabled)
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
				if !strings.Contains(string(contents), "after configuration: "+level) {
					t.Fatalf("missing %s log: %s", level, contents)
				}
			}
		})
	}
}
