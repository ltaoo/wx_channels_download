package testui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- Fast unit tests ---

func TestServerServesHTML(t *testing.T) {
	modDir := ResolveModDir(".")
	srv := NewServer(modDir, "127.0.0.1:0")
	defer srv.Close()
	handler := srv.Handler()

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET / returned %d, want 200", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "单测管理") {
		t.Error("HTML page should include title '单测管理'")
	}
	if !strings.Contains(body, "refreshTests") {
		t.Error("HTML page should contain JS function 'refreshTests'")
	}
}

func TestNotFound(t *testing.T) {
	modDir := ResolveModDir(".")
	srv := NewServer(modDir, "127.0.0.1:0")
	defer srv.Close()
	handler := srv.Handler()

	req := httptest.NewRequest("GET", "/api/tests/run/nonexistent", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	var resp struct {
		Code int `json:"code"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 404 {
		t.Errorf("expected code 404 for unknown run, got %d", resp.Code)
	}
}

func TestMissingPkg(t *testing.T) {
	modDir := ResolveModDir(".")
	srv := NewServer(modDir, "127.0.0.1:0")
	defer srv.Close()
	handler := srv.Handler()

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest("POST", "/api/tests/run", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	var resp struct {
		Code int `json:"code"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 400 {
		t.Errorf("expected code 400 for missing pkg, got %d", resp.Code)
	}
}

func TestHandlerCORS(t *testing.T) {
	modDir := ResolveModDir(".")
	srv := NewServer(modDir, "127.0.0.1:0")
	defer srv.Close()
	handler := srv.Handler()

	// Test OPTIONS preflight
	req := httptest.NewRequest("OPTIONS", "/api/tests", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("OPTIONS returned %d, want 204", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("CORS header missing")
	}
}

// --- Slow integration test (skipped with -short) ---

func TestIntegration_FullFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full integration test in short mode")
	}

	modDir := ResolveModDir(".")
	srv := NewServer(modDir, "127.0.0.1:0")
	defer srv.Close()
	handler := srv.Handler()

	// Step 1: List tests (this triggers discovery which may take a while)
	listReq := httptest.NewRequest("GET", "/api/tests", nil)

	// Poll for discovery completion (up to 90 seconds)
	var discoveredPkgs []PackageInfo
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		listW := httptest.NewRecorder()
		handler.ServeHTTP(listW, listReq)

		var listResp struct {
			Code int `json:"code"`
			Data struct {
				Packages []PackageInfo `json:"packages"`
			} `json:"data"`
		}
		if err := json.Unmarshal(listW.Body.Bytes(), &listResp); err != nil {
			t.Fatalf("JSON decode: %v", err)
		}
		if listResp.Code != 0 {
			t.Fatalf("list API code=%d", listResp.Code)
		}
		if len(listResp.Data.Packages) > 0 {
			discoveredPkgs = listResp.Data.Packages
			break
		}
		time.Sleep(2 * time.Second)
	}

	if len(discoveredPkgs) == 0 {
		t.Fatal("no tests discovered within deadline")
	}

	// Find first package with tests
	var targetPkg, targetTest string
	for _, pkg := range discoveredPkgs {
		if len(pkg.Tests) > 0 {
			targetPkg = pkg.ImportPath
			targetTest = pkg.Tests[0].Name
			break
		}
	}
	if targetPkg == "" {
		t.Fatal("no packages with tests found")
	}

	t.Logf("target: %s / %s", targetPkg, targetTest)

	// Step 2: Run the test
	runBody := strings.NewReader(`{"pkg":"` + targetPkg + `","test":"` + targetTest + `"}`)
	runReq := httptest.NewRequest("POST", "/api/tests/run", runBody)
	runReq.Header.Set("Content-Type", "application/json")
	runW := httptest.NewRecorder()
	handler.ServeHTTP(runW, runReq)

	if runW.Code != http.StatusOK {
		t.Fatalf("POST /api/tests/run returned %d, want 200", runW.Code)
	}

	var runResp struct {
		Code int `json:"code"`
		Data struct {
			RunID string `json:"run_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(runW.Body.Bytes(), &runResp); err != nil {
		t.Fatalf("JSON decode: %v", err)
	}
	if runResp.Code != 0 || runResp.Data.RunID == "" {
		t.Fatalf("run response: %+v", runResp)
	}

	t.Logf("run_id = %s", runResp.Data.RunID)

	// Step 3: Poll for result
	pollDeadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(pollDeadline) {
		pollReq := httptest.NewRequest("GET", "/api/tests/run/"+runResp.Data.RunID, nil)
		pollW := httptest.NewRecorder()
		handler.ServeHTTP(pollW, pollReq)

		var pollResp struct {
			Code int        `json:"code"`
			Data *RunResult `json:"data"`
		}
		if err := json.Unmarshal(pollW.Body.Bytes(), &pollResp); err != nil {
			t.Fatalf("JSON decode: %v", err)
		}

		if pollResp.Data == nil {
			t.Fatalf("run result not found for %s", runResp.Data.RunID)
		}

		if pollResp.Data.Status != StatusRunning {
			t.Logf("test %s: status=%s duration=%dms output=%d chars",
				pollResp.Data.Test, pollResp.Data.Status, pollResp.Data.DurationMs, len(pollResp.Data.Output))
			if pollResp.Data.Status != StatusPassed {
				// Log output for debugging, but don't fail - the test might have real failures
				t.Logf("test status is %s (may be expected)", pollResp.Data.Status)
				if len(pollResp.Data.Output) > 0 {
					t.Logf("output (first 300 chars): %s", pollResp.Data.Output[:minInt(len(pollResp.Data.Output), 300)])
				}
			}
			return
		}
		time.Sleep(500 * time.Millisecond)
	}

	t.Error("test did not complete within poll deadline")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
