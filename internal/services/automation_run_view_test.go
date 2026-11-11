package services

import (
	"bytes"
	"encoding/json"
	"log"
	"strings"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/flowengine"
)

func TestAutomationRunStatesAndNodeLogs(t *testing.T) {
	service := new_trigger_test_service(t)
	run := model.FlowRunRecord{ID: "run-view", FlowID: "root", InstanceID: "instance", Status: "COMPLETED", Definition: strings.Repeat("large snapshot", 10000), NodeStates: `{"root":{"start":"COMPLETED","end":"PENDING"},"child":{"start":"FAILED"}}`}
	if err := service.db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	// Repeated loop invocations can all have attempt=1 and the same timestamp.
	for index := 0; index < 3; index++ {
		service.persist_node_log(flowengine.NodeExecutionLog{RunID: run.ID, FlowID: "root", NodeID: "start", Attempt: 1, Outcome: "completed", Output: map[string]interface{}{"iteration": index}, Success: true})
	}
	service.persist_node_log(flowengine.NodeExecutionLog{RunID: run.ID, FlowID: "root", NodeID: "other", Output: map[string]interface{}{"large": strings.Repeat("payload", 10000)}})
	service.persist_node_log(flowengine.NodeExecutionLog{RunID: run.ID, FlowID: "child", NodeID: "start", Outcome: "failed"})
	var trace bytes.Buffer
	query_service := &AutomationService{db: service.db.Session(&gorm.Session{Logger: logger.New(log.New(&trace, "", 0), logger.Config{LogLevel: logger.Info})})}
	states, err := query_service.GetRunNodeStates(run.ID)
	if err != nil || len(states) != 2 || states["start"] != "COMPLETED" || states["end"] != "PENDING" {
		t.Fatalf("incorrect simple states: %+v %v", states, err)
	}
	for _, column := range []string{"definition", "node_logs", "node_outputs", "initial_data", "checkpoint"} {
		if strings.Contains(trace.String(), "`"+column+"`") {
			t.Fatalf("state query loaded %s: %s", column, trace.String())
		}
	}
	trace.Reset()
	logs, err := query_service.GetRunNodeLogs(run.ID, "start")
	if err != nil || len(logs) != 3 {
		t.Fatalf("node attempts were collapsed or mixed: %d %v", len(logs), err)
	}
	last_id := int64(0)
	for index, raw := range logs {
		var entry struct {
			ID      int64                  `json:"log_id"`
			Attempt int                    `json:"attempt"`
			Output  map[string]interface{} `json:"output"`
		}
		if json.Unmarshal(raw, &entry) != nil || entry.ID <= last_id || entry.Attempt != 1 || entry.Output["iteration"] != float64(index) {
			t.Fatalf("attempt identity/order lost: %s", raw)
		}
		last_id = entry.ID
	}
	if strings.Contains(trace.String(), "`definition`") || strings.Contains(trace.String(), "SELECT * FROM `flow_run_record`") {
		t.Fatalf("node click read the full run: %s", trace.String())
	}
	metadata, err := service.GetRunMetadata(run.ID)
	if err != nil || metadata.Definition != "" || metadata.NodeLogs != "" || metadata.NodeStates != "" {
		t.Fatalf("metadata read heavy payloads: %+v %v", metadata, err)
	}
	child, err := service.GetRunNodeLogs(run.ID, "start", "child")
	if err != nil || len(child) != 1 {
		t.Fatalf("subflow logs leaked across scopes: %d %v", len(child), err)
	}
}

func TestAutomationSubflowResetPreservesEveryInvocationLog(t *testing.T) {
	service := new_trigger_test_service(t)
	run := model.FlowRunRecord{ID: "run-reset", FlowID: "root", Status: "RUNNING", Definition: `{"id":"root","nodes":{"child":{"config":{"workflow":{"id":"child","nodes":{"start":{}}}}}}}`, NodeStates: `{"root":{"child":"RUNNING"},"child":{"start":"COMPLETED"}}`}
	if err := service.db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	service.persist_node_log(flowengine.NodeExecutionLog{RunID: run.ID, FlowID: "child", NodeID: "start", Attempt: 1, Outcome: "completed"})
	service.persist_node_status(flowengine.NodeExecutionStatus{RunID: run.ID, FlowID: "child", NodeStates: map[string]flowengine.NodeState{"start": flowengine.StatePending}})
	service.persist_node_log(flowengine.NodeExecutionLog{RunID: run.ID, FlowID: "child", NodeID: "start", Attempt: 1, Outcome: "failed"})
	logs, err := service.GetRunNodeLogs(run.ID, "start", "child")
	if err != nil || len(logs) != 2 {
		t.Fatalf("reset removed previous invocation logs: %d %v", len(logs), err)
	}
	// Querying states still only projects the current lightweight status.
	states, err := service.GetRunNodeStates(run.ID, "child")
	if err != nil || states["start"] != "PENDING" {
		t.Fatalf("incorrect reset status: %+v %v", states, err)
	}
}

func TestRepeatedNodeAttemptsUseBoundedCursorPages(t *testing.T) {
	service := new_trigger_test_service(t)
	run := model.FlowRunRecord{ID: "run-many-attempts", FlowID: "root", InstanceID: "instance", Status: "COMPLETED"}
	if err := service.db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	rows := make([]model.FlowRunNodeLog, 1205)
	for index := range rows {
		rows[index] = model.FlowRunNodeLog{RunID: run.ID, FlowID: run.FlowID, NodeID: "repeat", Entry: `{"node_id":"repeat","attempt":1,"outcome":"completed"}`}
	}
	if err := service.db.CreateInBatches(&rows, 100).Error; err != nil {
		t.Fatal(err)
	}
	seen := map[int64]bool{}
	before_id := int64(0)
	for {
		page, err := service.GetRunNodeLogPage(run.ID, "repeat", "", before_id, 128)
		if err != nil || len(page.List) > 128 {
			t.Fatalf("unbounded page: %d %v", len(page.List), err)
		}
		last_id := int64(0)
		for _, raw := range page.List {
			var entry struct {
				ID      int64 `json:"log_id"`
				Attempt int   `json:"attempt"`
			}
			if json.Unmarshal(raw, &entry) != nil || entry.ID <= last_id || seen[entry.ID] || entry.Attempt != 1 {
				t.Fatalf("duplicate or missing attempt identity: %s", raw)
			}
			seen[entry.ID], last_id = true, entry.ID
		}
		if !page.HasMore {
			break
		}
		if page.NextBeforeID <= 0 || (before_id > 0 && page.NextBeforeID >= before_id) {
			t.Fatal("cursor did not advance")
		}
		before_id = page.NextBeforeID
	}
	if len(seen) != 1205 {
		t.Fatalf("only %d repeated attempts retained", len(seen))
	}
	var plan struct{ Detail string }
	if err := service.db.Raw(`EXPLAIN QUERY PLAN SELECT entry FROM flow_run_node_log WHERE run_id=? AND flow_id=? AND node_id=? AND id<? ORDER BY id DESC LIMIT 129`, run.ID, "root", "repeat", 1000).Scan(&plan).Error; err != nil || !strings.Contains(plan.Detail, "idx_flow_run_node_log_node") {
		t.Fatalf("node log cursor did not use its index: %s %v", plan.Detail, err)
	}
}
