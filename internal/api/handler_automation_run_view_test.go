package api

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	"wx_channel/internal/services"
)

func TestAutomationRunViewResponses(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "automation.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.UserFlow{}, &model.FlowSchedule{}, &model.FlowRunRecord{}); err != nil {
		t.Fatal(err)
	}
	sql_db, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sql_db.Close() })
	run := model.FlowRunRecord{ID: "run-view", FlowID: "flow-test", Status: "COMPLETED",
		Definition: `{"id":"flow-test","nodes":{"start":{},"end":{}}}`,
		NodeLogs:   `[{"node_id":"start","outcome":"completed","input":{"large":"payload"}},{"node_id":"end","outcome":"completed","output":{"private":"other-node"}}]`}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	client := &APIClient{engine: router, service_automation: services.NewAutomationService(db, nil, nil, nil)}
	client.setup_automation_routes()
	request := func(endpoint, body string) map[string]interface{} {
		t.Helper()
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("POST", "/api/v1/automation/"+endpoint, strings.NewReader(body)))
		var result map[string]interface{}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || result["code"] != float64(0) {
			t.Fatalf("request failed: %s, %v", response.Body.String(), err)
		}
		return result
	}
	listed := request("list_runs", `{"flow_id":"flow-test","summary":true}`)["data"].(map[string]interface{})["list"].([]interface{})
	summary := listed[0].(map[string]interface{})
	if len(summary) != 3 || summary["id"] != run.ID || summary["status"] != run.Status || summary["created_at"] == nil {
		t.Fatalf("list must contain only time, id and result: %+v", summary)
	}
	states := request("get_run_node_states", `{"id":"run-view"}`)["data"].(map[string]interface{})
	if len(states) != 2 || states["start"] != "COMPLETED" || states["end"] != "COMPLETED" {
		t.Fatalf("expected a node-to-status map: %+v", states)
	}
	logs := request("get_run_node_logs", `{"id":"run-view","node_id":"start"}`)["data"].(map[string]interface{})["list"].([]interface{})
	if len(logs) != 1 || logs[0].(map[string]interface{})["input"] == nil || logs[0].(map[string]interface{})["node_id"] != "start" {
		t.Fatalf("expected only the requested node's detailed logs: %+v", logs)
	}
}
