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
	nopLogger := zerolog.Nop()
	client := &APIClient{
		logger: &nopLogger,
		cfg:    &APIConfig{},
	}
	engine.POST("/report", client.handle_frontend_report)

	// When logging is disabled by default, debug log should not cause an error
	debugReport := map[string]interface{}{
		"level":   "debug",
		"message": "polling debug log",
		"task_id": 999,
	}
	body, _ := json.Marshal(debugReport)
	req := httptest.NewRequest(http.MethodPost, "/report", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	// Info report should also return 200 without outputting
	infoReport := map[string]interface{}{
		"level":   "info",
		"message": "task starting",
		"task_id": 1000,
	}
	bodyInfo, _ := json.Marshal(infoReport)
	reqInfo := httptest.NewRequest(http.MethodPost, "/report", bytes.NewReader(bodyInfo))
	reqInfo.Header.Set("Content-Type", "application/json")
	wInfo := httptest.NewRecorder()
	engine.ServeHTTP(wInfo, reqInfo)

	if wInfo.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", wInfo.Code)
	}

	// Error report is always handled
	errorReport := map[string]interface{}{
		"level":   "error",
		"message": "download failed",
		"task_id": 1001,
	}
	bodyErr, _ := json.Marshal(errorReport)
	reqErr := httptest.NewRequest(http.MethodPost, "/report", bytes.NewReader(bodyErr))
	reqErr.Header.Set("Content-Type", "application/json")
	wErr := httptest.NewRecorder()
	engine.ServeHTTP(wErr, reqErr)

	if wErr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", wErr.Code)
	}
}

func TestHandlerFrontendReport_LoggingEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	var buf bytes.Buffer
	testLogger := zerolog.New(&buf)
	baseCfg := config.New("test", "debug", &testLogger, nil, "/tmp/test.log")

	client := &APIClient{
		logger: &testLogger,
		cfg: &APIConfig{
			Original: baseCfg,
		},
	}
	engine.POST("/report", client.handle_frontend_report)

	if !client.isLoggingEnabled() {
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
	nopLogger := zerolog.Nop()
	client := &APIClient{
		logger: &nopLogger,
		cfg:    &APIConfig{},
	}
	engine.POST("/tip", client.handleFrontendTip)

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
	nopLogger := zerolog.Nop()
	client := &APIClient{
		logger: &nopLogger,
		cfg:    &APIConfig{},
	}
	engine.POST("/error", client.handleFrontendError)

	errTip := map[string]interface{}{
		"msg": "error detail",
	}
	body, _ := json.Marshal(errTip)
	req := httptest.NewRequest(http.MethodPost, "/error", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}
}

func TestLogRotation_InPlaceTruncation_50MB(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rotatetest50mb")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logFile := filepath.Join(tmpDir, "wx_video_download.out.log")
	// Simulate file reaching 50MB (52428800 bytes)
	const maxBytes = int64(52428800)
	dummyData := make([]byte, 1024)
	if err := os.WriteFile(logFile, dummyData, 0644); err != nil {
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
	if err := rotate(logFile, maxBytes); err != nil {
		t.Fatalf("rotate failed: %v", err)
	}
	info, _ := os.Stat(logFile)
	if info.Size() != 1024 {
		t.Fatalf("expected size 1024, got %d", info.Size())
	}

	// Set limit to 1024 to simulate exceeding 50MB
	if err := rotate(logFile, 1024); err != nil {
		t.Fatalf("rotate failed: %v", err)
	}
	infoAfter, _ := os.Stat(logFile)
	if infoAfter.Size() != 0 {
		t.Errorf("expected size 0 after in-place truncation, got %d", infoAfter.Size())
	}
}

type mockSizeLimitedWriter struct {
	file         *os.File
	maxBytes     int64
	currentBytes int64
	mu           sync.Mutex
}

func (w *mockSizeLimitedWriter) Write(p []byte) (n int, err error) {
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

func TestSizeLimitedWriter_InPlaceTruncation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sizelimitedtest")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "test.log")
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC|os.O_APPEND, 0666)
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	defer f.Close()

	writer := &mockSizeLimitedWriter{
		file:     f,
		maxBytes: 100,
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

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if info.Size() != 60 {
		t.Errorf("expected file size to be 60 after in-place truncation, got %d", info.Size())
	}
}
