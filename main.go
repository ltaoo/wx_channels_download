package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"wx_channel/cmd"
	"wx_channel/internal/application"
	"wx_channel/internal/config"
)

var AppVer = "260823"
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
	logger, log_file, log_path, err := new_app_logger()
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		return
	}
	cfg := config.New(AppVer, Mode, logger, log_file, log_path)
	run_err := cmd.Execute(cfg)
	_ = log_file.Close()
	if run_err != nil {
		fmt.Printf("Failed to run: %v\n", run_err.Error())
		return
	}
	if err := application.RestartIfRequested(); err != nil {
		fmt.Printf("Failed to restart: %v\n", err.Error())
	}
}

func new_app_logger() (*zerolog.Logger, *os.File, string, error) {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	zerolog.TimeFieldFormat = time.RFC3339Nano

	log_dir := filepath.Join(os.TempDir(), "wx_channels_download")
	if err := os.MkdirAll(log_dir, 0755); err != nil {
		return nil, nil, "", err
	}
	log_path := filepath.Join(log_dir, "app.log")
	log_file, err := os.OpenFile(log_path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return nil, nil, "", err
	}

	logger := zerolog.New(log_file).With().
		Timestamp().
		Logger()
	return &logger, log_file, log_path, nil
}
