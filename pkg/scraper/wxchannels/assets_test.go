package wxchannels

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"wx_channel/internal/webassets"
)

func TestRegisterStaticAssets(t *testing.T) {
	if Assets.InjectFS == nil {
		t.Skip("wxchannels assets are not embedded in this build")
	}
	registry := webassets.NewRegistry()
	if err := RegisterStaticAssets(registry); err != nil {
		t.Fatalf("register assets: %v", err)
	}

	for _, requestPath := range []string{
		StaticAssetsPath + "/channels.ws.js",
		"/__assets/inject/channels.ws.js",
	} {
		response := httptest.NewRecorder()
		registry.ServeHTTP(response, httptest.NewRequest(http.MethodGet, requestPath, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want %d", requestPath, response.Code, http.StatusOK)
		}
	}
}
