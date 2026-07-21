package wxmp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"wx_channel/internal/webassets"
)

func TestRegisterStaticAssets(t *testing.T) {
	if Assets.InjectFS == nil {
		t.Skip("wxmp assets are not embedded in this build")
	}
	registry := webassets.NewRegistry()
	if err := RegisterStaticAssets(registry); err != nil {
		t.Fatalf("register assets: %v", err)
	}

	for _, requestPath := range []string{
		StaticAssetsPath + "/mp.ws.js",
		"/__assets/inject/mp.ws.js",
	} {
		response := httptest.NewRecorder()
		registry.ServeHTTP(response, httptest.NewRequest(http.MethodGet, requestPath, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want %d", requestPath, response.Code, http.StatusOK)
		}
	}
}
