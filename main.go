package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"wx_channel/cmd"
	"wx_channel/internal/application"
	"wx_channel/internal/config"
	"wx_channel/internal/logfile"
	"wx_channel/internal/logtime"
)

var AppVer = "26092804"
var Mode = "debug"

func main() {
	if handled, err := application.RunApplicationUpdateHelperIfRequested(); handled {
		if err != nil {
			fmt.Printf("Failed to apply staged update: %v\n", err)
		}
		return
	}
	if err := application.CleanupApplicationUpdateHelperIfRequested(); err != nil {
		fmt.Printf("Failed to clean up update helper: %v\n", err)
	}
	if Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	logger, log_writer, log_path, err := new_app_logger()
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		return
	}
	cfg := config.New(AppVer, Mode, logger, log_writer, log_path)
	run_err := cmd.Execute(cfg)
	_ = log_writer.Close()
	if run_err != nil {
		fmt.Printf("Failed to run: %v\n", run_err.Error())
		return
	}
	if err := application.RestartIfRequested(); err != nil {
		fmt.Printf("Failed to restart: %v\n", err.Error())
	}
}

func new_app_logger() (*zerolog.Logger, *logfile.Writer, string, error) {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	zerolog.TimeFieldFormat = time.RFC3339Nano

	// The desktop build keeps its log under the OS temp dir (transient by
	// design). Container images set WX_LOG_DIR so app.log lands on the mounted
	// volume next to the other logs and outlives a restart.
	log_dir := filepath.Join(os.TempDir(), "wx_channels_download")
	if configured_dir := strings.TrimSpace(os.Getenv("WX_LOG_DIR")); configured_dir != "" {
		log_dir = configured_dir
	}
	if err := os.MkdirAll(log_dir, 0755); err != nil {
		return nil, nil, "", err
	}
	log_path := filepath.Join(log_dir, "app.log")
	log_writer := logfile.Open(log_path)
	// Keep the historical truncate-on-start behaviour; rotation bounds the file
	// from here on.
	if err := log_writer.Truncate(); err != nil {
		return nil, nil, "", err
	}

	logger := zerolog.New(log_writer).Hook(logtime.Hook{})
	return &logger, log_writer, log_path, nil
}
