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
	"wx_channel/pkg/flowengine"
)

func TestAutomationRunViewResponses(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "automation.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.UserFlowDefinition{}, &model.UserFlowInstance{}, &model.FlowSchedule{}, &model.FlowRunRecord{}, &model.FlowRunNodeLog{}); err != nil {
		t.Fatal(err)
	}
	sql_db, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sql_db.Close() })
	run := model.FlowRunRecord{ID: "run-view", FlowID: "flow-test", Status: "COMPLETED", Error: "启动校验失败: access_key 不能为空", TriggerType: "Event", TriggerKey: "downloadtask.finished",
		Definition: `{"id":"flow-test","nodes":{"start":{},"end":{}}}`,
		NodeLogs:   `[{"node_id":"start","outcome":"completed","input":{"large":"payload"}},{"node_id":"end","outcome":"completed","output":{"private":"other-node"}}]`}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	var entries []json.RawMessage
	if err := json.Unmarshal([]byte(run.NodeLogs), &entries); err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		var row struct {
			NodeID string `json:"node_id"`
		}
		_ = json.Unmarshal(entry, &row)
		if err := db.Create(&model.FlowRunNodeLog{RunID: run.ID, FlowID: run.FlowID, NodeID: row.NodeID, Entry: string(entry)}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Model(&run).Update("node_states", `{"flow-test":{"start":"COMPLETED","end":"COMPLETED"}}`).Error; err != nil {
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
	metadata := request("get_run", `{"id":"run-view","metadata_only":true}`)["data"].(map[string]interface{})
	if metadata["error"] != run.Error || metadata["trigger_key"] != run.TriggerKey || metadata["trigger_type"] != "Event" || metadata["node_logs"] != "" || metadata["definition"] != "" {
		t.Fatalf("metadata omitted startup failure or loaded payloads: %+v", metadata)
	}
	full := request("get_run", `{"id":"run-view"}`)["data"].(map[string]interface{})
	if full["definition"] != run.Definition || full["node_logs"] != run.NodeLogs {
		t.Fatal("full run response changed")
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

func TestAutomationFlowAndInstanceEndpointsAreSeparate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "entities.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.UserFlowDefinition{}, &model.UserFlowInstance{}, &model.FlowSchedule{}, &model.FlowRunRecord{}, &model.FlowRunNodeLog{}); err != nil {
		t.Fatal(err)
	}
	sql_db, _ := db.DB()
	t.Cleanup(func() { _ = sql_db.Close() })
	service := services.NewAutomationService(db, nil, flowengine.NewWorkflowEngine(), nil)
	t.Cleanup(service.Stop)
	router := gin.New()
	client := &APIClient{engine: router, service_automation: service}
	client.setup_automation_routes()
	request := func(endpoint string, body map[string]interface{}) map[string]interface{} {
		t.Helper()
		raw, _ := json.Marshal(body)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("POST", "/api/v1/automation/"+endpoint, strings.NewReader(string(raw))))
		var result map[string]interface{}
		if json.Unmarshal(response.Body.Bytes(), &result) != nil || result["code"] != float64(0) {
			t.Fatalf("%s failed: %s", endpoint, response.Body.String())
		}
		return result
	}
	flow := request("create_flow", map[string]interface{}{"name": "shared flow"})["data"].(map[string]interface{})
	instances := request("list_flow_instances", nil)["data"].(map[string]interface{})["list"].([]interface{})
	if len(instances) != 0 {
		t.Fatal("flow creation implicitly created an instance")
	}
	instance := request("create_flow_instance", map[string]interface{}{"flow_id": flow["id"], "name": "account A"})["data"].(map[string]interface{})
	if instance["id"] == flow["id"] || instance["flow_id"] != flow["id"] {
		t.Fatal("flow and instance identities were mixed")
	}
	request("create_flow_instance", map[string]interface{}{"flow_id": flow["id"], "name": "account B"})
	flows := request("list_flows", nil)["data"].(map[string]interface{})["list"].([]interface{})
	instances = request("list_flow_instances", nil)["data"].(map[string]interface{})["list"].([]interface{})
	if len(flows) != 1 || len(instances) != 2 || flows[0].(map[string]interface{})["instance_count"] != float64(2) {
		t.Fatalf("separate lists returned the wrong entities: flows=%d instances=%d", len(flows), len(instances))
	}
	if _, exists := instances[0].(map[string]interface{})["definition"]; exists {
		t.Fatal("instance list repeated a full graph")
	}
	request("update_flow", map[string]interface{}{"id": flow["id"], "triggers": []map[string]interface{}{{"id": "daily", "type": "Cron", "name": "daily", "cron_expr": "0 12 * * *", "enabled": true}}})
	bindings := request("list_schedules", map[string]interface{}{"instance_id": instance["id"]})["data"].(map[string]interface{})["list"].([]interface{})
	if len(bindings) != 1 || bindings[0].(map[string]interface{})["cron_expr"] != "0 12 * * *" {
		t.Fatalf("flow default did not reach an inheriting instance: %v", bindings)
	}
	custom := request("update_flow_instance", map[string]interface{}{"id": instance["id"], "trigger_overrides": map[string]interface{}{"daily": map[string]interface{}{"cron_expr": "0 9 * * *"}}})["data"].(map[string]interface{})
	if custom["inherit_triggers"] != true || custom["trigger_overrides"] != `{"daily":{"cron_expr":"0 9 * * *"}}` {
		t.Fatal("field override detached the rest of the defaults")
	}
	restored := request("update_flow_instance", map[string]interface{}{"id": instance["id"], "trigger_overrides": map[string]interface{}{}})["data"].(map[string]interface{})
	if restored["inherit_triggers"] != true || restored["trigger_overrides"] != "{}" {
		t.Fatal("instance could not restore trigger inheritance")
	}
}
