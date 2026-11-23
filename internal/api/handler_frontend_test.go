package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"wx_channel/internal/config"
)

func TestHandlerFrontendReport_DefaultDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	nop_logger := zerolog.Nop()
	client := &APIClient{
		logger: &nop_logger,
		cfg:    &APIConfig{},
	}
	engine.POST("/report", client.handle_frontend_report)

	// When logging is disabled by default, debug log should not cause an error
	debug_report := map[string]interface{}{
		"level":   "debug",
		"message": "polling debug log",
		"task_id": 999,
	}
	body, _ := json.Marshal(debug_report)
	req := httptest.NewRequest(http.MethodPost, "/report", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	// Info report should also return 200 without outputting
	info_report := map[string]interface{}{
		"level":   "info",
		"message": "task starting",
		"task_id": 1000,
	}
	body_info, _ := json.Marshal(info_report)
	req_info := httptest.NewRequest(http.MethodPost, "/report", bytes.NewReader(body_info))
	req_info.Header.Set("Content-Type", "application/json")
	w_info := httptest.NewRecorder()
	engine.ServeHTTP(w_info, req_info)

	if w_info.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w_info.Code)
	}

	// Error report is always handled
	error_report := map[string]interface{}{
		"level":   "error",
		"message": "download failed",
		"task_id": 1001,
	}
	body_err, _ := json.Marshal(error_report)
	req_err := httptest.NewRequest(http.MethodPost, "/report", bytes.NewReader(body_err))
	req_err.Header.Set("Content-Type", "application/json")
	w_err := httptest.NewRecorder()
	engine.ServeHTTP(w_err, req_err)

	if w_err.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w_err.Code)
	}
}

func TestHandlerFrontendReport_LoggingEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	var buf bytes.Buffer
	test_logger := zerolog.New(&buf)
	base_cfg := config.New("test", "debug", &test_logger, nil, "/tmp/test.log")

	client := &APIClient{
		logger: &test_logger,
		cfg: &APIConfig{
			Original: base_cfg,
		},
	}
	engine.POST("/report", client.handle_frontend_report)

	if !client.is_logging_enabled() {
		t.Fatalf("expected logging to be enabled when LogPath is non-empty")
	}

	report := map[string]interface{}{
		"level":   "info",
		"message": "enabled task completed",
		"task_id": 1002,
	}
	body, _ := json.Marshal(report)
	req := httptest.NewRequest(http.MethodPost, "/report", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	if !bytes.Contains(buf.Bytes(), []byte("enabled task completed")) {
		t.Errorf("expected log buffer to contain message, got: %s", buf.String())
	}
}

func TestHandlerFrontendTip_Disabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	nop_logger := zerolog.Nop()
	client := &APIClient{
		logger: &nop_logger,
		cfg:    &APIConfig{},
	}
	engine.POST("/tip", client.handle_frontend_tip)

	tip := map[string]interface{}{
		"msg": "test tip",
	}
	body, _ := json.Marshal(tip)
	req := httptest.NewRequest(http.MethodPost, "/tip", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}
}

func TestHandlerFrontendError_AlwaysHandled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	nop_logger := zerolog.Nop()
	client := &APIClient{
		logger: &nop_logger,
		cfg:    &APIConfig{},
	}
	engine.POST("/error", client.handle_frontend_error)

	err_tip := map[string]interface{}{
		"msg": "error detail",
	}
	body, _ := json.Marshal(err_tip)
	req := httptest.NewRequest(http.MethodPost, "/error", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}
}

func TestLogRotation_InPlaceTruncation_50MB(t *testing.T) {
	tmp_dir, err := os.MkdirTemp("", "rotatetest50mb")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmp_dir)

	log_file := filepath.Join(tmp_dir, "wx_video_download.out.log")
	// Simulate file reaching 50MB (52428800 bytes)
	const max_bytes = int64(52428800)
	dummy_data := make([]byte, 1024)
	if err := os.WriteFile(log_file, dummy_data, 0644); err != nil {
		t.Fatalf("failed to write dummy log: %v", err)
	}

	// Helper function mimicking the shell script log rotation
	rotate := func(path string, limit int64) error {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if info.Size() >= limit {
			f, err := os.OpenFile(path, os.O_TRUNC|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
			return f.Close()
		}
		return nil
	}

	// Below limit, should not truncate
	if err := rotate(log_file, max_bytes); err != nil {
		t.Fatalf("rotate failed: %v", err)
	}
	info, _ := os.Stat(log_file)
	if info.Size() != 1024 {
		t.Fatalf("expected size 1024, got %d", info.Size())
	}

	// Set limit to 1024 to simulate exceeding 50MB
	if err := rotate(log_file, 1024); err != nil {
		t.Fatalf("rotate failed: %v", err)
	}
	info_after, _ := os.Stat(log_file)
	if info_after.Size() != 0 {
		t.Errorf("expected size 0 after in-place truncation, got %d", info_after.Size())
	}
}

type mock_size_limited_writer struct {
	file          *os.File
	max_bytes     int64
	current_bytes int64
	mu            sync.Mutex
}

func (w *mock_size_limited_writer) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.current_bytes+int64(len(p)) > w.max_bytes {
		_ = w.file.Truncate(0)
		_, _ = w.file.Seek(0, io.SeekStart)
		w.current_bytes = 0
	}
	n, err = w.file.Write(p)
	w.current_bytes += int64(n)
	return n, err
}

func TestSizeLimitedWriter_InPlaceTruncation(t *testing.T) {
	tmp_dir, err := os.MkdirTemp("", "sizelimitedtest")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmp_dir)

	file_path := filepath.Join(tmp_dir, "test.log")
	f, err := os.OpenFile(file_path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC|os.O_APPEND, 0666)
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	defer f.Close()

	writer := &mock_size_limited_writer{
		file:      f,
		max_bytes: 100,
	}

	data := make([]byte, 60)
	n, err := writer.Write(data)
	if err != nil || n != 60 {
		t.Fatalf("write failed: %v", err)
	}

	n, err = writer.Write(data)
	if err != nil || n != 60 {
		t.Fatalf("write after truncate failed: %v", err)
	}

	info, err := os.Stat(file_path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if info.Size() != 60 {
		t.Errorf("expected file size to be 60 after in-place truncation, got %d", info.Size())
	}
}
