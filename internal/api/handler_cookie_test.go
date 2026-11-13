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
		count  int
	}{
		{path: "/api/cookies/update", body: `{"cookies":[{"name":"sid","value":"plain","domain":".example.com"}]}`, status: http.StatusOK, count: 1},
		{path: "/api/cookie/update", body: `{"uuid":"device","crypto_type":"aes-128-cbc-fixed","encrypted":"yNCP7ZYYy9cD9I/SLB6wRcCcBALpUdGVT2VBRiQbLIj1I0Axi62BvDorUaDTKk7HIzqtWO1j7W16mbuHY4mGm2mVfkw+wKJI7DGCZtC5jdQ="}`, status: http.StatusOK, count: 1},
		{path: "/api/cookies/update", body: `{"data":"{\"cookies\":[{\"name\":\"sid\",\"value\":\"plain\",\"domain\":\".example.com\"}]}"}`, status: http.StatusOK, count: 1},
		{path: "/api/cookies/update", body: `{"crypto_type":"none","data":"[{\"domain\":\".qidian.com\",\"expirationDate\":2000000000.5,\"name\":\"sid\",\"value\":\"plain\"}]","update_time":"2026-10-19T07:27:06.611Z"}`, status: http.StatusOK, count: 1},
		{path: "/api/cookies/update", body: `{"uuid":"device","crypto_type":"aes-128-cbc-fixed","data":"yNCP7ZYYy9cD9I/SLB6wRcCcBALpUdGVT2VBRiQbLIj1I0Axi62BvDorUaDTKk7HIzqtWO1j7W16mbuHY4mGm2mVfkw+wKJI7DGCZtC5jdQ="}`, status: http.StatusOK, count: 1},
		{path: "/api/cookies/update", body: `{"uuid":"device","crypto_type":"aes-256-cbc","data":"U2FsdGVkX18xMjM0NTY3OOLzETzn4kr7wHTt2ZCV8+pAPblACCh83mVLS6xsqySPULtrwlDYu6vMDkXx0ulFOmQUx02YwzQ+mrC8XQNuuBTvsT0rqiRS6Ni1Q15B8A6d"}`, status: http.StatusOK, count: 1},
		{path: "/api/cookies/update", body: `{"uuid":"device","crypto_type":"legacy","data":"U2FsdGVkX18xMjM0NTY3OOLzETzn4kr7wHTt2ZCV8+pAPblACCh83mVLS6xsqySPULtrwlDYu6vMDkXx0ulFOmQUx02YwzQ+mrC8XQNuuBTvsT0rqiRS6Ni1Q15B8A6d"}`, status: http.StatusOK, count: 1},
		{path: "/api/cookie/update", body: `{"cookies":null}`, status: http.StatusBadRequest},
		{path: "/api/cookie/update", body: `{"crypto_type":"none","cookie_data":{}}`, status: http.StatusBadRequest},
		{path: "/api/cookie/update", body: `{"uuid":"wrong","crypto_type":"aes-128-cbc-fixed","encrypted":"ignored"}`, status: http.StatusForbidden},
		{path: "/api/cookies/update", body: `{"data":"{\"cookie_data\":{}}"}`, status: http.StatusBadRequest},
		{path: "/api/cookies/update", body: `{"data":"{\"cookies\":[{\"name\":\"\",\"domain\":\".example.com\"}]}"}`, status: http.StatusBadRequest},
		{path: "/api/cookies/update", body: `{"uuid":"device","crypto_type":"aes-256-cbc","data":"invalid!"}`, status: http.StatusBadRequest},
		{path: "/api/cookies/update", body: `{"uuid":"wrong","crypto_type":"aes-256-cbc","data":"ignored"}`, status: http.StatusForbidden},
		{path: "/api/cookies/update", body: `{"crypto_type":"none","data":"[]","update_time":"2026-10-19T07:27:06.611Z"}`, status: http.StatusOK},
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
		if err := json.Unmarshal(saved, &stored); err != nil || stored == nil || len(stored) != test_case.count {
			t.Fatalf("saved cookies=%s err=%v", saved, err)
		}
		if len(stored) > 0 && stored[0].Name != "sid" {
			t.Fatalf("unexpected saved cookie: %+v", stored[0])
		}
		if strings.Contains(test_case.body, ".qidian.com") && (stored[0].Domain != ".qidian.com" || stored[0].Expires != 2000000000) {
			t.Fatalf("array cookie attributes were not saved: %+v", stored[0])
		}
		var result struct {
			Action string `json:"action"`
			Code   int    `json:"code"`
			Data   struct {
				Count int `json:"count"`
			} `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || result.Action != "done" || result.Code != 0 || result.Data.Count != test_case.count {
			t.Fatalf("unexpected response: %s err=%v", response.Body.String(), err)
		}
	}
}
