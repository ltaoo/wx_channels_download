package douyinadapter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDouyinListRejectsInvalidParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/list", new_routes(&handler{}).handle_fetch_list)
	for _, query := range []string{"", "sec_uid=%20", "sec_uid=https%3A%2F%2Fwww.douyin.com%2Fuser%2Ftest", "sec_uid=test&next_marker=invalid", "sec_uid=test&next_marker=0"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/list?"+query, nil))
		var body struct {
			Code int `json:"code"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Code != 400 {
			t.Fatalf("query %q: expected business error 400, got %s (%v)", query, response.Body.String(), err)
		}
	}
}
