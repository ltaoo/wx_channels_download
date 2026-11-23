package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"

	"wx_channel/internal/logfile"
	"wx_channel/internal/services"
)

func TestAppLoggerRelativeDirectoryAndGORMLevels(t *testing.T) {
	original_level := zerolog.GlobalLevel()
	original_time_format := zerolog.TimeFieldFormat
	t.Cleanup(func() {
		zerolog.SetGlobalLevel(original_level)
		zerolog.TimeFieldFormat = original_time_format
	})
	current_dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	log_dir := filepath.Join(t.TempDir(), "logs")
	relative_dir, err := filepath.Rel(current_dir, log_dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("WX_LOG_DIR", relative_dir)
	logger, log_writer, log_path, err := new_app_logger()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = log_writer.Close() })
	if !filepath.IsAbs(log_path) || log_path != filepath.Join(log_dir, "app.log") || log_writer.Filename != log_path {
		t.Fatalf("inconsistent log paths: config=%s, writer=%s", log_path, log_writer.Filename)
	}
	logger.Error().Msg("logging is disabled")
	if _, err := os.Stat(log_dir); !os.IsNotExist(err) {
		t.Fatalf("disabled logging touched the log directory: %v", err)
	}
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	db_logger := logfile.NewGORMLogger(logger)
	db_logger.Error(context.Background(), "database %s", "failed")
	db_logger.Warn(context.Background(), "database warning")
	service := services.NewDataQueryService(services.DataQueryServiceConfig{
		LogPath: log_path,
		WorkDir: t.TempDir(),
	})
	for _, level := range []string{"error", "warn"} {
		result, err := service.ListLogs(context.Background(), services.LogListQuery{Levels: []string{level}})
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		var page struct {
			Total   int `json:"total"`
			Entries []struct {
				Level string `json:"level"`
			} `json:"entries"`
		}
		if err := json.Unmarshal(data, &page); err != nil {
			t.Fatal(err)
		}
		if page.Total != 1 || len(page.Entries) != 1 || page.Entries[0].Level != level {
			t.Fatalf("%s filter did not find its GORM entry: %s", level, data)
		}
	}
}
