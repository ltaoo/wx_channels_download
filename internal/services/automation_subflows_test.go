package services

import (
	"encoding/json"
	"os"
	"testing"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/flowengine"
)

func TestReusableSubflowsImportSaveAndFreeze(t *testing.T) {
	for _, filename := range []string{"node_status_demo.json", "01_creator_video_sync_service.json"} {
		t.Run(filename, func(t *testing.T) {
			service := new_trigger_test_service(t)
			t.Cleanup(service.Stop)
			raw, err := os.ReadFile("../../examples/pipelines/" + filename)
			if err != nil {
				t.Fatal(err)
			}
			flow, err := service.ImportUserFlow(string(raw))
			if err != nil {
				t.Fatal(err)
			}
			var definition flowengine.FlowDefinition
			if err := json.Unmarshal([]byte(flow.Definition), &definition); err != nil {
				t.Fatal(err)
			}
			if len(definition.Subflows) == 0 {
				t.Fatal("named subflows lost on import")
			}
			values := map[string]map[string]interface{}{}
			if filename == "01_creator_video_sync_service.json" {
				values["validate"] = map[string]interface{}{"username": "creator"}
			}
			updated, err := service.UpdateUserFlow(flow.ID, UpdateUserFlowInput{Subflows: definition.Subflows, NodeConfigs: values})
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(updated.Definition), &definition); err != nil {
				t.Fatal(err)
			}
			node_id := "child"
			if filename == "01_creator_video_sync_service.json" {
				node_id = "fetch_page"
			}
			if definition.Nodes[node_id].Config["flow_id"] == nil || definition.Nodes[node_id].Config["workflow"] != nil {
				t.Fatal("reusable declaration was replaced with an inline copy in the saved template")
			}
			run, err := service.prepare_run(model.FlowSchedule{FlowID: flow.ID}, model.FlowRunTriggerManual, "", "", "")
			if err != nil {
				t.Fatal(err)
			}
			var snapshot flowengine.FlowDefinition
			if err := json.Unmarshal([]byte(run.Definition), &snapshot); err != nil {
				t.Fatal(err)
			}
			if snapshot.Nodes[node_id].Config["flow_id"] != nil || snapshot.Nodes[node_id].Config["workflow"] == nil {
				t.Fatal("run did not freeze its reusable child definition")
			}
		})
	}
}
