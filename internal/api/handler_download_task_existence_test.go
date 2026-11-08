package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"wx_channel/internal/services"
	"wx_channel/pkg/dm"
)

func TestDownloadTaskContentCheckHTTPAndSDK(t *testing.T) {
	client := new_sync_test_client(t)
	client.service_download_task = services.NewDownloadTaskService(client.db, nil, nil, nil, "", "", nil)
	seed_sync_test_task(t, client)
	engine := gin.New()
	engine.POST("/api/v1/download_task/check_contents", client.handle_check_download_task_contents)
	server := httptest.NewServer(engine)
	defer server.Close()
	sdk, err := dm.NewClient(dm.ClientOptions{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := sdk.CheckDownloadTaskContents(context.Background(), []string{"local-content", "missing", "local-content"})
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		List []struct {
			ContentID string `json:"content_id"`
			Exists    bool   `json:"exists"`
			TaskCount int64  `json:"task_count"`
		} `json:"list"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 || len(result.List) != 2 || result.List[0].ContentID != "local-content" || !result.List[0].Exists || result.List[0].TaskCount != 1 || result.List[1].Exists {
		t.Fatalf("wrong batch result: %s", raw)
	}
	raw, err = sdk.CheckDownloadTaskContents(context.Background(), []string{})
	if err != nil || !strings.Contains(string(raw), `"list":[]`) {
		t.Fatalf("empty array failed: %s %v", raw, err)
	}
	for _, body := range []string{`{}`, `{"content_ids":null}`, `{"content_ids":[123]}`, `{"content_ids":[""]}`} {
		request := httptest.NewRequest("POST", "/api/v1/download_task/check_contents", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)
		var envelope struct {
			Code int `json:"code"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil || envelope.Code == 0 {
			t.Fatalf("invalid request accepted: %s %s", body, recorder.Body.String())
		}
	}
}
