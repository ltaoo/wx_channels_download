package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"wx_channel/frontend"
	"wx_channel/pkg/testui/assert"
	"wx_channel/pkg/testui/require"
)

func TestHandleContentPageRendersInjectedContentHTML(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousAssets := frontend.Assets
	if frontend.Assets.InjectDir != "(embedded)" {
		frontend.Assets = frontend.NewChannelInjectedFiles("../../frontend")
	}
	t.Cleanup(func() {
		frontend.Assets = previousAssets
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/content", nil)
	client := &APIClient{cfg: &APIConfig{Version: "test-version"}}
	client.handleContentPage(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "text/html; charset=utf-8", recorder.Header().Get("Content-Type"))
	assert.True(t, strings.Contains(recorder.Body.String(), `<div id="app"></div>`))
	assert.True(t, strings.Contains(recorder.Body.String(), `/__assets/inject/content/style.css`))
	assert.True(t, strings.Contains(recorder.Body.String(), `/__assets/inject/content/core.js`))
	assert.True(t, strings.Contains(recorder.Body.String(), `/__assets/inject/content/index.js`))
	assert.False(t, strings.Contains(recorder.Body.String(), `/__assets/inject/download/index.js`))
	assert.True(t, strings.Contains(recorder.Body.String(), `window.__wx_channels_config__ = {"Version":"test-version"`))
	assert.False(t, strings.Contains(recorder.Body.String(), "window.{"))
	assert.False(t, strings.Contains(recorder.Body.String(), "__WX_DOWNLOAD_CONFIG_JSON__"))
	assert.False(t, strings.Contains(recorder.Body.String(), "__WX_DOWNLOAD_VERSION__"))

	coreJS, err := frontend.Assets.ReadInject("content/core.js")
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(coreJS), `content_request.get("/api/content/list", params)`))
	indexJS, err := frontend.Assets.ReadInject("content/index.js")
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(indexJS), "ContentLibraryModel()"))
	assert.True(t, strings.Contains(string(indexJS), "Timeless.Select({"))
	assert.True(t, strings.Contains(string(coreJS), "new Timeless.ui.SelectCore({"))
}

func TestShouldServeContentPageByAPI(t *testing.T) {
	assert.True(t, shouldServeByAPI("/content"))
}
