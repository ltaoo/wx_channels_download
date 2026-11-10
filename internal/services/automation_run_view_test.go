package services

import (
	"encoding/json"
	"strings"
	"testing"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/flowengine"
)

func TestAutomationRunStatesAndNodeLogs(t *testing.T) {
	service := new_trigger_test_service(t)
	payload := strings.Repeat("large-payload", 10000)
	run := model.FlowRunRecord{ID: "run-view", FlowID: "target", Status: model.FlowRunStatusFailed,
		Definition:  `{"id":"original","nodes":{"fetch":{"type":"ServiceNode"}}}`,
		InitialData: payload, NodeOutputs: payload, Checkpoint: payload, NodeAttempts: `{"fetch":2}`,
		NodeLogs: marshal_json([]map[string]interface{}{
			{"flow_id": "target", "node_id": "fetch", "attempt": 1, "outcome": "retrying", "error": "timeout", "input": payload},
			{"flow_id": "original", "node_id": "fetch", "attempt": 2, "outcome": "failed", "output": payload},
			{"flow_id": "target:child", "node_id": "fetch", "outcome": "completed", "output": "child"},
			{"flow_id": "target", "node_id": "other", "outcome": "completed", "output": "other"},
		})}
	if err := service.db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	states, err := service.GetRunNodeStates(" run-view ")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(states)
	if err != nil || len(encoded) > 2048 || strings.Contains(string(encoded), "large-payload") {
		t.Fatalf("summary contains large payloads: size=%d, err=%v", len(encoded), err)
	}
	if states["fetch"] != "FAILED" || states["other"] != "COMPLETED" || len(states) != 2 {
		t.Fatalf("incorrect node state map: %+v", states)
	}
	listed, _, err := service.ListRuns(ListRunsInput{FlowID: "target", Summary: true})
	if err != nil || len(listed) != 1 || listed[0].ID != run.ID || listed[0].Status != run.Status || listed[0].Definition != "" || listed[0].NodeLogs != "" {
		t.Fatalf("list loaded more than run summaries: %+v, %v", listed, err)
	}
	logs, err := service.GetRunNodeLogs("run-view", " fetch ")
	if err != nil || len(logs) != 2 || !strings.Contains(string(logs[1]), payload) {
		t.Fatalf("node payloads missing or child included: count=%d, err=%v", len(logs), err)
	}
	full, err := service.GetRun(run.ID)
	if err != nil || full.NodeLogs != run.NodeLogs || full.NodeOutputs != payload {
		t.Fatal("existing full-detail callers or saved data changed")
	}
	if _, err := service.GetRunNodeLogs(run.ID, ""); err == nil {
		t.Fatal("empty node id accepted")
	}
	if _, err := service.GetRunNodeStates("missing"); err == nil {
		t.Fatal("missing run accepted")
	}
	if _, err := service.ClearRuns("target"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetRunNodeLogs(run.ID, "fetch"); err == nil {
		t.Fatal("deleted run logs accessible")
	}
	if _, err := service.GetRunNodeStates(run.ID); err == nil {
		t.Fatal("deleted summary accessible")
	}
}

func TestAutomationResetStartsNewNodeStateAndLogProjection(t *testing.T) {
	service := new_trigger_test_service(t)
	run := model.FlowRunRecord{ID: "run-reset", FlowID: "target", Status: model.FlowRunStatusRunning,
		Definition: `{"id":"target","nodes":{"first":{},"second":{}}}`, NodeAttempts: `{"first":1}`,
		NodeLogs: `[{"flow_id":"target","node_id":"first","outcome":"failed","error":"previous invocation","output":{"old":true}}]`}
	if err := service.db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	service.publish_node_execution_status(flowengine.NodeExecutionStatus{RunID: run.ID, FlowID: run.FlowID,
		NodeStates: map[string]flowengine.NodeState{"first": flowengine.StatePending, "second": flowengine.StatePending}})
	states, err := service.GetRunNodeStates(run.ID)
	if err != nil || states["first"] != "PENDING" || states["second"] != "PENDING" {
		t.Fatalf("old state restored after reset: %v, %v", states, err)
	}
	if logs, err := service.GetRunNodeLogs(run.ID, "first"); err != nil || len(logs) != 0 {
		t.Fatalf("previous invocation logs returned: %v, %v", logs, err)
	}
	service.persist_node_log(flowengine.NodeExecutionLog{RunID: run.ID, FlowID: run.FlowID, NodeID: "second", Outcome: "completed", Output: map[string]interface{}{"current": true}})
	service.publish_node_execution_status(flowengine.NodeExecutionStatus{RunID: run.ID, FlowID: "another-child", NodeStates: map[string]flowengine.NodeState{"second": flowengine.StatePending}})
	states, err = service.GetRunNodeStates(run.ID)
	if err != nil || states["first"] != "PENDING" || states["second"] != "COMPLETED" {
		t.Fatalf("another scope affected current states: %v, %v", states, err)
	}
	if logs, err := service.GetRunNodeLogs(run.ID, "second"); err != nil || len(logs) != 1 || !strings.Contains(string(logs[0]), "current") {
		t.Fatalf("current invocation logs missing: %v, %v", logs, err)
	}
}

func TestAutomationRunViewLegacyLogs(t *testing.T) {
	service := new_trigger_test_service(t)
	if err := service.db.Create(&model.FlowRunRecord{ID: "legacy", FlowID: "target", Status: model.FlowRunStatusFailed, NodeLogs: "malformed"}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetRunNodeStates("legacy"); err != nil {
		t.Fatal(err)
	}
	if logs, err := service.GetRunNodeLogs("legacy", "fetch"); err != nil || len(logs) != 0 {
		t.Fatalf("legacy logs unreadable: %v, %v", logs, err)
	}
}
