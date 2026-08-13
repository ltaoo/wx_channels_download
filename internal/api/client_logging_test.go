package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAccessLoggerOmitsQueryString(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	engine := new_api_engine(&output)
	engine.GET("/safe", func(ctx *gin.Context) { ctx.Status(http.StatusNoContent) })

	request := httptest.NewRequest(http.MethodGet, "/safe?pass_ticket=sentinel-secret", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	logOutput := output.String()
	if !strings.Contains(logOutput, `"/safe"`) {
		t.Fatalf("access log omitted request path: %q", logOutput)
	}
	for _, forbidden := range []string{"pass_ticket", "sentinel-secret", "/safe?"} {
		if strings.Contains(logOutput, forbidden) {
			t.Fatalf("access log contains query data %q: %q", forbidden, logOutput)
		}
	}
}
