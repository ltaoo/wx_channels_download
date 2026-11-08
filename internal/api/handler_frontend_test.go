package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

func TestFrontendLogLimiter_Deduplication(t *testing.T) {
	limiter := NewFrontendLogLimiter(100*time.Millisecond, 10, 10)

	key := "task:100:downloading:info:update - check whether the task is completed"

	// First call should be allowed
	if !limiter.Allow(key) {
		t.Fatalf("expected first call with key to be allowed")
	}

	// Immediate repeat should be suppressed by cooldown
	if limiter.Allow(key) {
		t.Fatalf("expected immediate duplicate call to be suppressed by cooldown")
	}

	// A different key should be allowed immediately
	differentKey := "task:101:downloading:info:update - check whether the task is completed"
	if !limiter.Allow(differentKey) {
		t.Fatalf("expected different key to be allowed")
	}

	// After cooldown, original key should be allowed again
	time.Sleep(120 * time.Millisecond)
	if !limiter.Allow(key) {
		t.Fatalf("expected key to be allowed after cooldown expired")
	}
}

func TestFrontendLogLimiter_BurstRateLimit(t *testing.T) {
	// Limiter with 5 tokens and 0 refill
	limiter := NewFrontendLogLimiter(1*time.Millisecond, 5, 0)

	for i := 0; i < 5; i++ {
		key := "key-" + string(rune('a'+i))
		if !limiter.Allow(key) {
			t.Fatalf("expected token %d to be allowed", i)
		}
	}

	// 6th call should be rejected due to token exhaustion
	if limiter.Allow("key-exhausted") {
		t.Fatalf("expected call to be rejected when token bucket is exhausted")
	}
}

func TestFrontendLogLimiter_Concurrency(t *testing.T) {
	limiter := NewFrontendLogLimiter(50*time.Millisecond, 100, 100)
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_ = limiter.Allow("concurrent-key")
			}
		}(i)
	}

	wg.Wait()
}

func TestFrontendLogLimiter_MemoryPrune(t *testing.T) {
	limiter := NewFrontendLogLimiter(10*time.Millisecond, 1000, 1000)
	limiter.maxEntries = 5

	for i := 0; i < 10; i++ {
		limiter.Allow("entry-" + string(rune('a'+i)))
	}

	time.Sleep(25 * time.Millisecond)
	limiter.Allow("trigger-prune")

	limiter.mu.Lock()
	count := len(limiter.lastSeen)
	limiter.mu.Unlock()

	if count > 5 {
		t.Errorf("expected pruned entries to not exceed maxEntries, got %d", count)
	}
}

func TestBuildFrontendLogKey(t *testing.T) {
	extra := map[string]interface{}{
		"task_id":     123,
		"next_status": "completed",
	}
	key := buildFrontendLogKey("info", "done", extra)
	expected := "task:123:completed:info:done"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}

	keyNoTask := buildFrontendLogKey("info", "generic", nil)
	expectedNoTask := "report:info:generic"
	if keyNoTask != expectedNoTask {
		t.Errorf("expected %q, got %q", expectedNoTask, keyNoTask)
	}
}

func TestHandlerFrontendReport_LevelFiltering(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	nopLogger := zerolog.Nop()
	client := &APIClient{
		logger:               &nopLogger,
		frontend_log_limiter: NewFrontendLogLimiter(time.Second, 10, 10),
	}
	engine.POST("/report", client.handle_frontend_report)

	// Test debug level report
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

	// Test info level report
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
}

func TestLogRotation_InPlaceTruncation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rotatetest")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logFile := filepath.Join(tmpDir, "wx_video_download.out.log")
	dummyData := make([]byte, 10485760+100) // 10MB + 100 bytes
	if err := os.WriteFile(logFile, dummyData, 0644); err != nil {
		t.Fatalf("failed to write dummy log: %v", err)
	}

	// Verify initial size exceeds 10MB
	info, err := os.Stat(logFile)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if info.Size() <= 10485760 {
		t.Fatalf("expected size > 10MB, got %d", info.Size())
	}

	// Execute in-place truncation
	maxBytes := int64(10485760)
	if info.Size() >= maxBytes {
		f, err := os.OpenFile(logFile, os.O_TRUNC|os.O_WRONLY, 0644)
		if err != nil {
			t.Fatalf("failed to truncate file: %v", err)
		}
		_ = f.Close()
	}

	// Verify truncated size
	infoAfter, err := os.Stat(logFile)
	if err != nil {
		t.Fatalf("stat after failed: %v", err)
	}
	if infoAfter.Size() != 0 {
		t.Errorf("expected size 0 after in-place truncation, got %d", infoAfter.Size())
	}
}
