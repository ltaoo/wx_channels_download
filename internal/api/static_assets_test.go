package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"

	"wx_channel/internal/webassets"
)

func TestPlatformStaticAssetRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	assets := webassets.NewRegistry()
	if err := assets.Register("/__assets/platform/test", fstest.MapFS{
		"asset.js": &fstest.MapFile{Data: []byte("window.test = true;")},
	}); err != nil {
		t.Fatalf("register assets: %v", err)
	}

	client := &APIClient{engine: gin.New(), staticAssets: assets}
	client.setupStaticAssetRoutes()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/__assets/platform/test/asset.js", nil)
	client.engine.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Body.String(); got != "window.test = true;" {
		t.Fatalf("body = %q", got)
	}
}
