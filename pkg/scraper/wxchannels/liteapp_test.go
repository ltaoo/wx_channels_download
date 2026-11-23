package wxchannels

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLiteAppBootstrapAndStatus(t *testing.T) {
	plugins := NewInterceptorPlugins(InterceptorConfig{Version: "test"}, nil)
	request := func(method, path, body string) (int, map[string]string, string) {
		t.Helper()
		ctx := scriptContext(t, method, "https://res.wx.qq.com"+liteapp_asset_path+path, body)
		ctx.Req.Header.Set("Origin", "https://liteapp.weixin.qq.com")
		plugins[1].OnRequest(ctx)
		response := ctx.GetMockResponse()
		if response == nil {
			t.Fatal("runtime request was not intercepted")
		}
		text, ok := response.Body.(string)
		if !ok {
			t.Fatalf("unexpected response body: %T", response.Body)
		}
		return response.StatusCode, response.Headers, text
	}
	status, headers, body := request(http.MethodGet, "bootstrap", "")
	if status != http.StatusOK || headers["Access-Control-Allow-Origin"] != "https://liteapp.weixin.qq.com" || headers["Cache-Control"] != "no-store" {
		t.Fatalf("bootstrap response: %d, %v", status, headers)
	}
	var result map[string]string
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatal(err)
	}
	for _, asset := range []string{"eventbus.js", "channels.events.js", "channels.utils.js", "channels.ws.js", "channels.feed.js", `"assets_base_url":"https://res.wx.qq.com/__assets"`} {
		if !strings.Contains(result["bootstrap"], asset) {
			t.Errorf("bootstrap is missing %s", asset)
		}
	}
	if len(result) != 1 {
		t.Fatalf("bootstrap still contains compiled module metadata: %v", result)
	}
	for _, path := range []string{"compile", "modules/old.js"} {
		status, _, _ := request(http.MethodPost, path, `{"source":"unused"}`)
		if status != http.StatusNotFound {
			t.Errorf("obsolete endpoint %s returned %d", path, status)
		}
	}
	posted := `{"stage":"ready","revision":"direct-wrap-v1","methods":12,"init_events":1,"detail_events":1,"app_preserved":true}`
	status, _, body = request(http.MethodPost, "status", posted)
	if status != http.StatusOK {
		t.Fatalf("status POST returned %d", status)
	}
	_, _, stored := request(http.MethodGet, "status", "")
	if body != stored || !strings.Contains(stored, `"app_preserved":true`) {
		t.Fatalf("status was not retained: %s", stored)
	}
}

func TestLiteAppGlobalScriptInjection(t *testing.T) {
	script_path := filepath.Join(t.TempDir(), "custom-global.js")
	const source = `WXU.unshiftMenuItems({label:"test alert",onClick(){alert("click")}});`
	if err := os.WriteFile(script_path, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	plugins := NewInterceptorPlugins(InterceptorConfig{GlobalScriptPath: script_path}, nil)
	ctx := scriptContext(t, http.MethodGet, "https://res.wx.qq.com"+liteapp_asset_path+"bootstrap", "")
	plugins[1].OnRequest(ctx)
	var result map[string]string
	if err := json.Unmarshal([]byte(ctx.GetMockResponse().Body.(string)), &result); err != nil {
		t.Fatal(err)
	}
	html := result["bootstrap"]
	asset_path := "/__assets/user/custom-global.js"
	global_index := strings.Index(html, `src="`+asset_path+`"`)
	utils_index := strings.Index(html, "channels.utils.js")
	feed_index := strings.Index(html, "channels.feed.js")
	if global_index < 0 || utils_index < 0 || feed_index < 0 || global_index < utils_index || global_index > feed_index {
		t.Fatal("global script must load after menu extensions and before the feed UI")
	}
	for _, content := range []string{source, source + "\n// edited without restarting"} {
		if err := os.WriteFile(script_path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		ctx := scriptContext(t, http.MethodGet, "https://res.wx.qq.com"+asset_path, "")
		ctx.Req.Header.Set("If-None-Match", "cached")
		plugins[1].OnRequest(ctx)
		response := ctx.GetMockResponse()
		if response == nil || response.StatusCode != http.StatusOK || response.Body != content {
			t.Fatalf("global script did not return current disk content: %#v", response)
		}
		if response.Headers["Cache-Control"] != "no-store" || response.Headers["ETag"] != "" || response.Headers["Access-Control-Allow-Origin"] != "https://liteapp.weixin.qq.com" {
			t.Fatalf("unexpected global script headers: %v", response.Headers)
		}
	}
}

func TestLiteAppSDKInjection(t *testing.T) {
	for _, mode := range []string{"", "dev", "release", "prod"} {
		t.Run("mode="+mode, func(t *testing.T) {
			plugins := NewInterceptorPlugins(InterceptorConfig{Mode: mode}, nil)
			ctx := scriptContext(t, http.MethodGet, "https://res.wx.qq.com/t/wx_fed/cdn_libs/res/libpag/4.0/libpag.min.js", "window.originalSDK = true;")
			ctx.Res.Header.Set("ETag", "cached")
			ctx.Res.Header.Set("Last-Modified", "yesterday")
			plugins[1].OnResponse(ctx)
			body, err := io.ReadAll(ctx.Res.Body)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(body), "window.originalSDK = true;\n;") {
				t.Fatal("original SDK was not preserved")
			}
			inline := mode == "release" || mode == "prod"
			if strings.Contains(string(body), liteapp_runtime_js) != inline {
				t.Fatal("runtime injection did not follow the configured mode")
			}
			if ctx.Res.Header.Get("Cache-Control") != "no-store" || ctx.Res.Header.Get("ETag") != "" || ctx.Res.Header.Get("Last-Modified") != "" {
				t.Fatalf("SDK cache headers were not reset: %v", ctx.Res.Header)
			}
			if !inline {
				assertScriptBehavior(t, string(body), "runtime-loader")
			} else {
				req := scriptContext(t, http.MethodGet, "https://res.wx.qq.com"+liteapp_asset_path+"runtime.js", "")
				plugins[1].OnRequest(req)
				if req.GetMockResponse().StatusCode != http.StatusNotFound {
					t.Fatal("production runtime must not depend on a source file endpoint")
				}
			}
		})
	}
}

func TestLiteAppRuntimeReadsDevelopmentEdits(t *testing.T) {
	// A temporary source tree lets us edit the runtime without touching project files.
	dir := t.TempDir()
	source_path := filepath.Join(dir, "pkg", "scraper", "wxchannels", "liteapp_runtime.js")
	if err := os.MkdirAll(filepath.Dir(source_path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source_path, []byte("window.revision = 1;"), 0644); err != nil {
		t.Fatal(err)
	}
	original_dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(original_dir); err != nil {
			t.Error(err)
		}
	})
	plugins := NewInterceptorPlugins(InterceptorConfig{Mode: "dev"}, nil)
	request := func(method string) (int, map[string]string, string) {
		t.Helper()
		ctx := scriptContext(t, method, "https://res.wx.qq.com"+liteapp_asset_path+"runtime.js?t=1", "")
		ctx.Req.Header.Set("Origin", "https://liteapp.weixin.qq.com")
		ctx.Req.Header.Set("If-None-Match", "old-version")
		plugins[1].OnRequest(ctx)
		response := ctx.GetMockResponse()
		return response.StatusCode, response.Headers, response.Body.(string)
	}
	for _, source := range []string{"window.revision = 1;", "window.revision = 2;"} {
		if err := os.WriteFile(source_path, []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
		status, headers, body := request(http.MethodGet)
		if status != http.StatusOK || body != source {
			t.Fatalf("runtime did not pick up the disk edit: %d %q", status, body)
		}
		if headers["Cache-Control"] != "no-store" || !strings.Contains(headers["Content-Type"], "javascript") || headers["Access-Control-Allow-Origin"] != "https://liteapp.weixin.qq.com" {
			t.Fatalf("unexpected runtime headers: %v", headers)
		}
	}
	status, _, body := request(http.MethodHead)
	if status != http.StatusOK || body != "" {
		t.Fatalf("HEAD returned a body: %d %q", status, body)
	}
	if err := os.Remove(source_path); err != nil {
		t.Fatal(err)
	}
	status, _, _ = request(http.MethodGet)
	if status != http.StatusInternalServerError {
		t.Fatal("missing development source silently fell back to embedded runtime")
	}
}
