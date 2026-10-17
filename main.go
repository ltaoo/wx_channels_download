package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	gorm_logger "gorm.io/gorm/logger"

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
	logger, log_writer, log_path := new_app_logger()
	log.SetOutput(logger)
	gin.DefaultWriter = logger
	gin.DefaultErrorWriter = logger
	gorm_logger.Default = gorm_logger.New(log.New(logger, "", 0), gorm_logger.Config{
		SlowThreshold: 200 * time.Millisecond,
		LogLevel:      gorm_logger.Warn,
	})
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

func new_app_logger() (*zerolog.Logger, *logfile.Writer, string) {
	// Configuration and command-line flags are loaded later. Keep all logging
	// disabled until debug.enabled has explicitly been resolved.
	zerolog.SetGlobalLevel(zerolog.Disabled)
	zerolog.TimeFieldFormat = time.RFC3339Nano

	// The desktop build keeps its log under the OS temp dir (transient by
	// design). Container images set WX_LOG_DIR so app.log lands on the mounted
	// volume next to the other logs and outlives a restart.
	log_dir := filepath.Join(os.TempDir(), "wx_channels_download")
	if configured_dir := strings.TrimSpace(os.Getenv("WX_LOG_DIR")); configured_dir != "" {
		log_dir = configured_dir
	}
	log_path := filepath.Join(log_dir, "app.log")
	log_writer := logfile.Open(log_path)

	logger := zerolog.New(log_writer).Hook(logtime.Hook{})
	return &logger, log_writer, log_path
}
