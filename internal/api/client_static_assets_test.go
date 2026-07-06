package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"wx_channel/internal/interceptor"
)

func TestAPIClientServesChannelLibAsset(t *testing.T) {
	withTestChannelAssets(t, map[string]string{
		"lib/mitt.umd.js": "window.mitt = {};",
	})
	client := newStaticAssetTestClient()

	resp := performStaticAssetRequest(client, http.MethodGet, "/__wx_channels_assets/lib/mitt.umd.js?v=test", nil)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
	if got := resp.Header().Get("Cache-Control"); got != interceptor.ChannelLibAssetCacheControl {
		t.Fatalf("Cache-Control = %q, want %q", got, interceptor.ChannelLibAssetCacheControl)
	}
	if got := resp.Body.String(); got != "window.mitt = {};" {
		t.Fatalf("body = %q, want lib asset content", got)
	}
}

func TestAPIClientServesChannelSrcAssetWithETag(t *testing.T) {
	body := "console.log('src');"
	withTestChannelAssets(t, map[string]string{
		"src/utils.js": body,
	})
	client := newStaticAssetTestClient()

	resp := performStaticAssetRequest(client, http.MethodGet, "/__wx_channels_assets/src/utils.js", nil)
	etag := interceptor.ChannelStaticAssetETag([]byte(body))

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
	if got := resp.Header().Get("Cache-Control"); got != interceptor.ChannelSrcAssetCacheControl {
		t.Fatalf("Cache-Control = %q, want %q", got, interceptor.ChannelSrcAssetCacheControl)
	}
	if got := resp.Header().Get("ETag"); got != etag {
		t.Fatalf("ETag = %q, want %q", got, etag)
	}

	headers := http.Header{"If-None-Match": []string{etag}}
	cachedResp := performStaticAssetRequest(client, http.MethodGet, "/__wx_channels_assets/src/utils.js", headers)
	if cachedResp.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want %d", cachedResp.Code, http.StatusNotModified)
	}
	if cachedResp.Body.Len() != 0 {
		t.Fatalf("body length = %d, want 0", cachedResp.Body.Len())
	}
}

func TestAPIClientServesDownloadPageTemplate(t *testing.T) {
	withTestChannelAssets(t, map[string]string{
		"index.html": `<!doctype html><link rel="stylesheet" href="/__wx_channels_assets/src/components.css"><script>window.__wx_channels_config__ = __WX_DOWNLOAD_CONFIG_JSON__; window.__wx_channels_version__ = "__WX_DOWNLOAD_VERSION__";</script><script src="/__wx_channels_assets/src/download/index.js"></script>`,
	})
	gin.SetMode(gin.TestMode)
	client := &APIClient{
		cfg:    &APIConfig{Protocol: "http", Hostname: "127.0.0.1", Port: 2022, RemoteServerEnabled: true},
		engine: gin.New(),
	}
	client.engine.GET("/download", client.handleDownloadPage)

	resp := performStaticAssetRequest(client, http.MethodGet, "/download", nil)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
	if got := resp.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want text/html; charset=utf-8", got)
	}
	body := resp.Body.String()
	if !strings.Contains(body, `"Protocol":"http"`) {
		t.Fatalf("body = %q, want rendered API config", body)
	}
	if !strings.Contains(body, `"remoteServerEnabled":true`) {
		t.Fatalf("body = %q, want remoteServerEnabled rendered for frontend config", body)
	}
	if !strings.Contains(body, `/__wx_channels_assets/src/download/index.js`) {
		t.Fatalf("body = %q, want download/index.js script", body)
	}
	if !strings.Contains(body, `/__wx_channels_assets/src/components.css`) {
		t.Fatalf("body = %q, want components.css stylesheet", body)
	}
	if strings.Contains(body, "__WX_DOWNLOAD_CONFIG_JSON__") {
		t.Fatalf("body = %q, want config placeholder replaced", body)
	}
}

func TestAPIClientServesWaterfallPreviewPageTemplate(t *testing.T) {
	withTestChannelAssets(t, map[string]string{
		"preview.html": `<!doctype html><link rel="stylesheet" href="/__wx_channels_assets/src/components.css"><script>window.__wx_channels_config__ = __WX_DOWNLOAD_CONFIG_JSON__; window.__wx_channels_version__ = "__WX_DOWNLOAD_VERSION__";</script><script src="/__wx_channels_assets/src/virtual-list-view.js"></script>`,
	})
	gin.SetMode(gin.TestMode)
	client := &APIClient{
		cfg:    &APIConfig{Protocol: "http", Hostname: "127.0.0.1", Port: 2022, RemoteServerEnabled: true},
		engine: gin.New(),
	}
	client.engine.GET("/waterfall", client.handleWaterfallPreviewPage)

	resp := performStaticAssetRequest(client, http.MethodGet, "/waterfall", nil)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
	if got := resp.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want text/html; charset=utf-8", got)
	}
	body := resp.Body.String()
	if !strings.Contains(body, `"Protocol":"http"`) {
		t.Fatalf("body = %q, want rendered API config", body)
	}
	if !strings.Contains(body, `/__wx_channels_assets/src/virtual-list-view.js`) {
		t.Fatalf("body = %q, want virtual-list-view.js script", body)
	}
	if strings.Contains(body, "__WX_DOWNLOAD_CONFIG_JSON__") {
		t.Fatalf("body = %q, want config placeholder replaced", body)
	}
}

func TestAPIClientServesDownloadPageAtRoot(t *testing.T) {
	withTestChannelAssets(t, map[string]string{
		"index.html": `<!doctype html><script>window.__wx_channels_config__ = __WX_DOWNLOAD_CONFIG_JSON__;</script>`,
	})
	gin.SetMode(gin.TestMode)
	client := &APIClient{
		cfg:    &APIConfig{Protocol: "http", Hostname: "127.0.0.1", Port: 2022},
		engine: gin.New(),
	}
	client.engine.GET("/", client.handleIndex)

	resp := performStaticAssetRequest(client, http.MethodGet, "/", nil)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
	body := resp.Body.String()
	if !strings.Contains(body, `"Hostname":"127.0.0.1"`) {
		t.Fatalf("body = %q, want rendered API config", body)
	}
	if strings.Contains(body, "__WX_DOWNLOAD_CONFIG_JSON__") {
		t.Fatalf("body = %q, want config placeholder replaced", body)
	}
}
func withTestChannelAssets(t *testing.T, assets map[string]string) {
	t.Helper()

	injectDir := t.TempDir()
	for rel, content := range assets {
		fullPath := filepath.Join(injectDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(fullPath), err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", fullPath, err)
		}
	}

	oldAssets := interceptor.Assets
	interceptor.Assets = interceptor.NewChannelInjectedFiles(injectDir)
	t.Cleanup(func() {
		interceptor.Assets = oldAssets
	})
}

func newStaticAssetTestClient() *APIClient {
	gin.SetMode(gin.TestMode)
	client := &APIClient{engine: gin.New()}
	client.setupStaticAssetRoutes()
	return client
}

func performStaticAssetRequest(client *APIClient, method string, target string, headers http.Header) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	resp := httptest.NewRecorder()
	client.ServeHTTP(resp, req)
	return resp
}
