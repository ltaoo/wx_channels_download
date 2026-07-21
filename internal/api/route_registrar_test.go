package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAPIClientRegisterGET(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &APIClient{engine: gin.New()}
	client.RegisterGET("/adapter-route", func(ctx *gin.Context) {
		ctx.Status(http.StatusNoContent)
	})

	response := httptest.NewRecorder()
	client.engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/adapter-route", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
}
