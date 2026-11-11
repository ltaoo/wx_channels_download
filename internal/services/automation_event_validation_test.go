package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"wx_channel/internal/database/model"
	"wx_channel/internal/events"
	"wx_channel/pkg/flowengine"
)

func TestAutomationEventValidationFailuresRemainVisible(t *testing.T) {
	for _, scenario := range []string{"missing-config", "missing-subflow"} {
		t.Run(scenario, func(t *testing.T) {
			service := new_trigger_test_service(t)
			t.Cleanup(service.Stop)
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				requests.Add(1)
				response.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()
			node := runtime_test_node("action", "APICallNode", map[string]interface{}{"url": server.URL})
			node.Name = "上传到七牛云"
			node.ConfigSchema = []flowengine.NodeConfigField{{Key: "access_key", Type: "string", Required: true, Secret: true}}
			expected_error := "access_key"
			if scenario == "missing-subflow" {
				node = runtime_test_node("action", "WorkflowNode", map[string]interface{}{"flow_id": "flow-u-missing-child"})
				expected_error = "flow-u-missing-child"
			}
			flow := import_runtime_flow(t, service, map[string]flowengine.NodeDefinition{
				"start": runtime_test_node("start", "StartNode", nil, "action"), "action": node,
			}, flowengine.FieldSchema{Key: "event", Type: "object"})
			if _, err := service.UpdateUserFlow(flow.ID, UpdateUserFlowInput{Triggers: []UserFlowTriggerInput{{Type: "Event", Name: "下载完成", EventKey: events.TypeDownloadTaskFinished}}}); err != nil {
				t.Fatal(err)
			}
			rule := trigger_test_rules(t, service, flow.ID)[0]
			bus := events.NewBus()
			failed_events := make(chan events.AutomationRunFailed, 4)
			bus.Subscribe(events.TypeAutomationRunFailed, func(event events.Event) { failed_events <- event.(events.AutomationRunFailed) })
			service.event_bus = bus
			if unmatched, err := service.DispatchEvent("unmatched", nil); err != nil || len(unmatched) != 0 {
				t.Fatal("unmatched event created a run")
			}
			runs, err := service.DispatchEvent(events.TypeDownloadTaskFinished, map[string]interface{}{"task_id": 42})
			if err != nil || len(runs) != 1 || runs[0].ID == "" || runs[0].Status != model.FlowRunStatusFailed {
				t.Fatalf("validation failure omitted from event response: %+v %v", runs, err)
			}
			// Exercise the background application-event path as well as explicit dispatch.
			service.HandleEvent(events.DownloadTaskFinished{TaskID: 43})
			service.wg.Wait()
			var saved []model.FlowRunRecord
			if err := service.db.Where("flow_id = ?", flow.ID).Order("created_at ASC").Find(&saved).Error; err != nil || len(saved) != 2 {
				t.Fatalf("event receipts missing from history: %+v %v", saved, err)
			}
			for _, run := range saved {
				if run.Status != model.FlowRunStatusFailed || !strings.Contains(run.Error, "启动校验失败") || !strings.Contains(run.Error, expected_error) || run.TriggerType != model.FlowRunTriggerEvent || run.TriggerKey != events.TypeDownloadTaskFinished || run.ScheduleID != rule.ID || run.StartedAt == nil || run.CompletedAt == nil {
					t.Fatalf("incomplete failed run: %+v", run)
				}
				if run.CurrentNode != "" || run.NodeLogs != "" || run.NodeAttempts != "" || requests.Load() != 0 {
					t.Fatalf("validation failure executed nodes: %+v requests=%d", run, requests.Load())
				}
				metadata, err := service.GetRunMetadata(run.ID)
				if err != nil || metadata.Error != run.Error || metadata.TriggerKey != run.TriggerKey || metadata.InitialData != "" || metadata.Definition != "" {
					t.Fatalf("failure metadata is not available without snapshots: %+v %v", metadata, err)
				}
				var input map[string]interface{}
				if json.Unmarshal([]byte(run.InitialData), &input) != nil || input["event"].(map[string]interface{})["type"] != events.TypeDownloadTaskFinished || !strings.Contains(run.Definition, "action") {
					t.Fatalf("event payload or graph missing: %+v", run)
				}
			}
			history, total, err := service.ListRuns(ListRunsInput{FlowID: flow.ID, Summary: true})
			if err != nil || total != 2 || len(history) != 2 || history[0].Status != model.FlowRunStatusFailed {
				t.Fatalf("failed receipt missing from history summary: %+v total=%d error=%v", history, total, err)
			}
			if len(failed_events) != 2 {
				t.Fatalf("failure notifications missing: %d", len(failed_events))
			}
			for len(failed_events) > 0 {
				notice := <-failed_events
				if notice.RunID == "" || notice.FlowID != flow.ID || notice.Status != model.FlowRunStatusFailed || !strings.Contains(notice.Error, expected_error) {
					t.Fatalf("invalid failure notification: %+v", notice)
				}
			}
			latest, err := service.GetSchedule(rule.ID)
			if err != nil || latest.LastRunStatus != model.FlowRunStatusFailed || latest.LastRunID == "" {
				t.Fatalf("trigger did not retain validation outcome: %+v %v", latest, err)
			}
			if scenario == "missing-config" {
				if _, err := service.UpdateUserFlow(flow.ID, UpdateUserFlowInput{NodeConfigs: map[string]map[string]interface{}{"action": {"access_key": "test-key"}}}); err != nil {
					t.Fatal(err)
				}
				retry, err := service.RetryRun(saved[0].ID, true)
				if err != nil {
					t.Fatal(err)
				}
				service.wg.Wait()
				completed, _ := service.GetRun(retry.ID)
				if completed.Status != model.FlowRunStatusCompleted || completed.RetryOf != saved[0].ID || requests.Load() != 1 {
					t.Fatalf("fixed configuration cannot be retried: %+v requests=%d", completed, requests.Load())
				}
			}
		})
	}
}
