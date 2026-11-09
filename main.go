package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"wx_channel/cmd"
	"wx_channel/internal/application"
	"wx_channel/internal/config"
	"wx_channel/internal/logtime"
)

var AppVer = "261106"
var Mode = "debug"

const defaultLogMaxBytes = 50 * 1024 * 1024 // 50MB

type sizeLimitedWriter struct {
	file         *os.File
	maxBytes     int64
	currentBytes int64
	mu           sync.Mutex
}

func (w *sizeLimitedWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.currentBytes+int64(len(p)) > w.maxBytes {
		_ = w.file.Truncate(0)
		_, _ = w.file.Seek(0, io.SeekStart)
		w.currentBytes = 0
	}
	n, err = w.file.Write(p)
	w.currentBytes += int64(n)
	return n, err
}

func is_log_enabled() bool {
	val := strings.ToLower(strings.TrimSpace(os.Getenv("ENABLE_LOG")))
	if val == "true" || val == "1" {
		return true
	}
	wxVal := strings.ToLower(strings.TrimSpace(os.Getenv("WX_ENABLE_LOG")))
	if wxVal == "true" || wxVal == "1" {
		return true
	}
	for _, arg := range os.Args[1:] {
		if arg == "--enable-log" || strings.HasPrefix(arg, "--enable-log=") {
			return true
		}
	}
	return false
}

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
	if log_file != nil {
		_ = log_file.Close()
	}
	if run_err != nil {
		fmt.Printf("Failed to run: %v\n", run_err.Error())
		return
	}
	if err := application.RestartIfRequested(); err != nil {
		fmt.Printf("Failed to restart: %v\n", err.Error())
	}
}

func new_app_logger() (*zerolog.Logger, *os.File, string, error) {
	if !is_log_enabled() {
		nop := zerolog.Nop()
		return &nop, nil, "", nil
	}

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	zerolog.TimeFieldFormat = time.RFC3339Nano

	log_dir := filepath.Join(os.TempDir(), "wx_channels_download")
	if err := os.MkdirAll(log_dir, 0755); err != nil {
		return nil, nil, "", err
	}
	log_path := filepath.Join(log_dir, "app.log")
	log_file, err := os.OpenFile(log_path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC|os.O_APPEND, 0666)
	if err != nil {
		return nil, nil, "", err
	}

	writer := &sizeLimitedWriter{
		file:     log_file,
		maxBytes: defaultLogMaxBytes,
	}
	logger := zerolog.New(writer).Hook(logtime.Hook{})
	return &logger, log_file, log_path, nil
}
