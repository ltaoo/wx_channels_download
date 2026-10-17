package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"wx_channel/pkg/cookies"
)

func TestCookieUpdateRoutes(t *testing.T) {
	logger := zerolog.Nop()
	client := &APIClient{
		cfg:         &APIConfig{WorkDir: t.TempDir(), CookieUUID: "device", CookieKey: "0123456789abcdef"},
		engine:      gin.New(),
		logger:      &logger,
		file_helper: &FileHelperHandler{},
	}
	client.SetupRoutes()
	cases := []struct {
		path   string
		body   string
		status int
	}{
		{path: "/api/cookies/update", body: `{"cookies":[{"name":"sid","value":"plain","domain":".example.com"}]}`, status: http.StatusOK},
		{path: "/api/cookie/update", body: `{"uuid":"device","crypto_type":"aes-128-cbc-fixed","encrypted":"yNCP7ZYYy9cD9I/SLB6wRcCcBALpUdGVT2VBRiQbLIj1I0Axi62BvDorUaDTKk7HIzqtWO1j7W16mbuHY4mGm2mVfkw+wKJI7DGCZtC5jdQ="}`, status: http.StatusOK},
		{path: "/api/cookie/update", body: `{"cookies":null}`, status: http.StatusBadRequest},
		{path: "/api/cookie/update", body: `{"crypto_type":"none","cookie_data":{}}`, status: http.StatusBadRequest},
		{path: "/api/cookie/update", body: `{"uuid":"wrong","crypto_type":"aes-128-cbc-fixed","encrypted":"ignored"}`, status: http.StatusForbidden},
	}
	cookie_path := filepath.Join(client.cfg.WorkDir, "cookies.json")
	for _, test_case := range cases {
		previous, _ := os.ReadFile(cookie_path)
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, test_case.path, strings.NewReader(test_case.body))
		request.Header.Set("Content-Type", "application/json")
		client.engine.ServeHTTP(response, request)
		if response.Code != test_case.status {
			t.Fatalf("%s: status=%d body=%s", test_case.path, response.Code, response.Body.String())
		}
		saved, err := os.ReadFile(cookie_path)
		if err != nil {
			t.Fatal(err)
		}
		if test_case.status != http.StatusOK {
			if string(saved) != string(previous) {
				t.Fatal("rejected request changed the saved cookies")
			}
			continue
		}
		var stored []cookies.Cookie
		if err := json.Unmarshal(saved, &stored); err != nil || len(stored) != 1 || stored[0].Name != "sid" {
			t.Fatalf("saved cookies=%s err=%v", saved, err)
		}
		var result struct {
			Action string `json:"action"`
			Code   int    `json:"code"`
			Data   struct {
				Count int `json:"count"`
			} `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || result.Action != "done" || result.Code != 0 || result.Data.Count != 1 {
			t.Fatalf("unexpected response: %s err=%v", response.Body.String(), err)
		}
	}
}
