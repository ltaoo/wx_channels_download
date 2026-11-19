package services

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/flowengine/engine"
)

func TestFlowAndInstanceHaveIndependentLifecycles(t *testing.T) {
	service := new_trigger_test_service(t)
	t.Cleanup(service.Stop)
	flow, err := service.CreateFlowDefinition(CreateUserFlowInput{Name: "standalone flow"})
	if err != nil {
		t.Fatal(err)
	}
	instances, err := service.ListUserFlowInstances()
	if err != nil || len(instances) != 0 {
		t.Fatalf("creating a flow implicitly created instances: %+v %v", instances, err)
	}
	if _, err := service.TriggerFlowInstance(flow.ID, nil); err == nil {
		t.Fatal("a flow was executed without an instance")
	}
	first, err := service.CreateUserFlowInstance(flow.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateUserFlowInstance(flow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID || first.ID == flow.ID || first.FlowID != flow.ID || second.FlowID != flow.ID {
		t.Fatal("flow and instance identities were mixed")
	}
	instances, err = service.ListUserFlowInstances()
	if err != nil || len(instances) != 2 || instances[0].Definition != "" {
		t.Fatalf("instance list should omit repeated shared graphs: %+v %v", instances, err)
	}
	loaded, err := service.GetFlowDefinition(flow.ID)
	if err != nil || loaded.InstanceCount != 2 {
		t.Fatalf("wrong flow instance count: %+v %v", loaded, err)
	}
	if err := service.DeleteFlowDefinition(flow.ID); err == nil {
		t.Fatal("deleting a referenced flow orphaned its instances")
	}
	if err := service.DeleteUserFlowInstance(first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetUserFlowInstance(second.ID); err != nil {
		t.Fatal("deleting one instance affected another", err)
	}
	run, err := service.TriggerFlowInstance(second.ID, nil)
	if err != nil || run.InstanceID != second.ID || run.FlowID != flow.ID {
		t.Fatalf("execution lost its flow/instance relation: %+v %v", run, err)
	}
	service.wg.Wait()
}

func TestTwoInstancesOfOneFlowKeepIndependentWaitingExecutions(t *testing.T) {
	service := new_trigger_test_service(t)
	t.Cleanup(service.Stop)
	flow, err := service.ImportFlowDefinition(`{"name":"shared confirm","start_node":"start","nodes":{"start":{"type":"StartNode","next_node_ids":["confirm"]},"confirm":{"type":"ManualNode","input_schema":[{"key":"approved","type":"boolean","required":true}],"next_node_ids":["result"]},"result":{"type":"JSCodeNode","config":{"code":"config.value + ':' + output.confirm.response.approved","output_key":"result"},"config_schema":[{"key":"value","type":"string","required":true}]}}}`)
	if err != nil {
		t.Fatal(err)
	}
	instances := make([]*model.UserFlowInstance, 2)
	runs := make([]*model.FlowRunRecord, 2)
	for index, value := range []string{"A", "B"} {
		instances[index], err = service.CreateUserFlowInstance(flow.ID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.UpdateUserFlowInstance(instances[index].ID, UpdateUserFlowInput{NodeConfigs: map[string]map[string]interface{}{"result": {"value": value}}}); err != nil {
			t.Fatal(err)
		}
		runs[index], err = service.TriggerFlowInstance(instances[index].ID, nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	service.wg.Wait()
	for index := range runs {
		saved, err := service.GetRunMetadata(runs[index].ID)
		if err != nil || saved.Status != "WAITING" || saved.InstanceID != instances[index].ID {
			t.Fatalf("shared flow merged execution state: %+v %v", saved, err)
		}
		if _, err := service.ResumeRun(saved.ID, "confirm", map[string]interface{}{"approved": index == 0}); err != nil {
			t.Fatal(err)
		}
	}
	service.wg.Wait()
	for index, result := range []string{"A:true", "B:false"} {
		saved, err := service.GetRun(runs[index].ID)
		if err != nil || saved.Status != "COMPLETED" || !strings.Contains(saved.NodeOutputs, result) {
			t.Fatalf("instance inputs crossed executions: %+v %v", saved, err)
		}
	}
}

func TestUserFlowDefinitionNameIsIndependentOfInstanceNames(t *testing.T) {
	service := new_trigger_test_service(t)
	t.Cleanup(service.Stop)
	source := trigger_test_flow(t, service)
	instance, err := service.CreateUserFlowInstance(source.FlowID)
	if err != nil {
		t.Fatal(err)
	}
	name := "account A"
	instance, err = service.UpdateUserFlowInstance(instance.ID, UpdateUserFlowInput{
		Name: &name, Nodes: []UserFlowNodeInput{{ID: "start", Type: "StartNode"}},
	})
	if err != nil || instance.Name != name || instance.DefinitionName != source.Name {
		t.Fatalf("instance editing renamed the shared flow: %+v %v", instance, err)
	}
	flow_name := "shared sync"
	if _, err := service.UpdateFlowDefinition(source.FlowID, UpdateUserFlowInput{Name: &flow_name}); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteUserFlowInstance(source.ID); err != nil {
		t.Fatal(err)
	}
	instance, err = service.GetUserFlowInstance(instance.ID)
	if err != nil || instance.Name != name || instance.DefinitionName != flow_name {
		t.Fatalf("shared flow name was lost with the original entry: %+v %v", instance, err)
	}
}

func TestUserFlowInstanceConfigurationInheritsLiveDefaults(t *testing.T) {
	service := new_trigger_test_service(t)
	t.Cleanup(service.Stop)
	source, err := service.ImportUserFlowWithInstance(`{"name":"defaults","start_node":"start","nodes":{"start":{"type":"StartNode","next_node_ids":["result"]},"result":{"type":"JSCodeNode","config":{"code":"config.value","output_key":"result"},"config_schema":[{"key":"value","type":"string","required":true},{"key":"enabled","type":"boolean","required":true},{"key":"count","type":"integer","required":true},{"key":"options","type":"object"}]}}}`)
	if err != nil {
		t.Fatal(err)
	}
	defaults := map[string]map[string]interface{}{"result": {"value": "v1", "enabled": true, "count": 9, "options": map[string]interface{}{"a": 1, "b": 2}}}
	if _, err := service.UpdateFlowDefinition(source.FlowID, UpdateUserFlowInput{DefaultNodeConfigs: defaults}); err != nil {
		t.Fatal(err)
	}
	instance, err := service.CreateUserFlowInstance(source.FlowID)
	if err != nil {
		t.Fatal(err)
	}
	overrides := map[string]map[string]interface{}{"result": {"enabled": false, "count": 0, "options": map[string]interface{}{"a": 3}}}
	if _, err := service.UpdateUserFlowInstance(instance.ID, UpdateUserFlowInput{NodeConfigs: overrides}); err != nil {
		t.Fatal(err)
	}
	execute := func() (*model.FlowRunRecord, map[string]interface{}) {
		t.Helper()
		run, err := service.TriggerFlowInstance(instance.ID, nil)
		if err != nil {
			t.Fatal(err)
		}
		service.wg.Wait()
		saved, err := service.GetRun(run.ID)
		if err != nil || saved.Status != "COMPLETED" {
			t.Fatalf("inherited configuration did not execute: %+v %v", saved, err)
		}
		definition, err := decode_user_flow_definition(saved.Definition)
		if err != nil {
			t.Fatal(err)
		}
		return saved, definition.Nodes["result"].ConfigValues
	}
	original, values := execute()
	if values["value"] != "v1" || values["enabled"] != false || values["count"] != float64(0) || !reflect.DeepEqual(values["options"], map[string]interface{}{"a": float64(3)}) {
		t.Fatalf("defaults/overrides were merged incorrectly: %+v", values)
	}
	defaults["result"]["value"], defaults["result"]["count"] = "v2", 10
	if _, err := service.UpdateFlowDefinition(source.FlowID, UpdateUserFlowInput{DefaultNodeConfigs: defaults}); err != nil {
		t.Fatal(err)
	}
	stale, _ := decode_user_flow_definition(instance.Definition)
	stale_node := stale.Nodes["result"]
	stale_node.Config["code"] = "'stale cached graph'"
	stale.Nodes["result"] = stale_node
	service.register_user_flow(*stale)
	updated, values := execute()
	var outputs map[string]interface{}
	if err := json.Unmarshal([]byte(updated.NodeOutputs), &outputs); err != nil || outputs["result"] != "v2" || values["enabled"] != false || values["count"] != float64(0) {
		t.Fatalf("instance did not inherit the latest defaults: %+v %+v %v", values, outputs, err)
	}
	previous, _ := service.GetRun(original.ID)
	if previous.Definition != original.Definition {
		t.Fatal("changing defaults rewrote the previous execution snapshot")
	}
	if _, err := service.UpdateUserFlowInstance(instance.ID, UpdateUserFlowInput{NodeConfigs: map[string]map[string]interface{}{}}); err != nil {
		t.Fatal(err)
	}
	_, values = execute()
	if values["enabled"] != true || values["count"] != float64(10) {
		t.Fatalf("removing overrides did not restore inheritance: %+v", values)
	}
	defaults["result"]["value"] = "v3"
	if _, err := service.UpdateUserFlowInstance(instance.ID, UpdateUserFlowInput{DefaultNodeConfigs: defaults}); err != nil {
		t.Fatal(err)
	}
	_, values = execute()
	instance, _ = service.GetUserFlowInstance(instance.ID)
	if values["value"] != "v3" || instance.NodeConfigs != "{}" {
		t.Fatal("updating shared defaults created instance overrides")
	}
}

func TestUserFlowInstancesShareGraphAndIsolateSettings(t *testing.T) {
	service := new_trigger_test_service(t)
	t.Cleanup(service.Stop)
	source, err := service.ImportUserFlowWithInstance(`{"name":"shared","start_node":"start","nodes":{"start":{"type":"StartNode","next_node_ids":["result"]},"result":{"type":"JSCodeNode","config":{"code":"config.value","output_key":"result"},"config_schema":[{"key":"value","type":"string","required":true}]}}}`)
	if err != nil {
		t.Fatal(err)
	}
	source, err = service.UpdateUserFlowInstance(source.ID, UpdateUserFlowInput{
		DefaultNodeConfigs: map[string]map[string]interface{}{"result": {"value": "first"}},
		Triggers: []UserFlowTriggerInput{
			{Type: "Cron", Name: "daily", CronExpr: "@daily", Enabled: true},
			{Type: "Webhook", Name: "hook"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := service.CreateUserFlowInstance(source.FlowID)
	if err != nil {
		t.Fatal(err)
	}
	if instance.ID == source.ID || instance.FlowID != source.FlowID || instance.NodeConfigs != "{}" || instance.DefaultNodeConfigs != source.DefaultNodeConfigs {
		t.Fatalf("incorrect instance: %+v", instance)
	}
	var count int64
	if err := service.db.Model(&model.UserFlowDefinition{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("instance duplicated graph: %d, %v", count, err)
	}
	for _, rule := range trigger_test_rules(t, service, instance.ID) {
		if rule.Enabled || automation_schedule_metadata(rule).Active {
			t.Fatal("new instance must allow configuration before automatic execution")
		}
	}
	if _, err := service.UpdateUserFlowInstance(instance.ID, UpdateUserFlowInput{NodeConfigs: map[string]map[string]interface{}{"result": {"value": "second"}}}); err != nil {
		t.Fatal(err)
	}
	// Updating either instance updates the one shared graph.
	if _, err := service.UpdateUserFlowInstance(source.ID, UpdateUserFlowInput{
		StartNodeID: "entry",
		Nodes: []UserFlowNodeInput{
			{ID: "entry", Type: "StartNode", NextIDs: []string{"result"}},
			{ID: "result", Type: "JSCodeNode", Config: map[string]interface{}{"code": "'updated:' + config.value", "output_key": "result"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ id, value string }{{source.ID, "first"}, {instance.ID, "second"}} {
		loaded, err := service.GetUserFlowInstance(item.id)
		if err != nil || !strings.Contains(loaded.Definition, "updated:") || merge_node_configs(decode_node_configs(loaded.DefaultNodeConfigs), decode_node_configs(loaded.NodeConfigs))["result"]["value"] != item.value {
			t.Fatalf("graph/config not composed correctly: %+v, %v", loaded, err)
		}
		run, err := service.TriggerFlowInstance(item.id, nil)
		if err != nil {
			t.Fatal(err)
		}
		service.wg.Wait()
		saved, err := service.GetRun(run.ID)
		if err != nil || saved.Status != "COMPLETED" || !strings.Contains(saved.NodeOutputs, "updated:"+item.value) {
			t.Fatalf("wrong instance execution: %+v, %v", saved, err)
		}
	}
	// Existing trigger metadata may still name the old start node.
	rule, err := service.CreateSchedule(CreateScheduleInput{Name: "instance rule", InstanceID: instance.ID, CronExpr: "@daily"})
	if err != nil {
		t.Fatal(err)
	}
	if run, err := service.TriggerSchedule(rule.ID); err != nil || run.Status != "COMPLETED" {
		t.Fatalf("shared start node change broke an instance rule: %+v %v", run, err)
	}
	// Changing declarations prunes incompatible values in every instance.
	if _, err := service.UpdateUserFlowInstance(source.ID, UpdateUserFlowInput{Nodes: []UserFlowNodeInput{
		{ID: "entry", Type: "StartNode", NextIDs: []string{"result"}},
		{ID: "result", Type: "JSCodeNode", Config: map[string]interface{}{"code": "config.value", "output_key": "result"}, ConfigSchema: []engine.NodeConfigField{{Key: "value", Type: "integer", Required: true}}},
	}}); err != nil {
		t.Fatal(err)
	}
	instance, _ = service.GetUserFlowInstance(instance.ID)
	if instance.NodeConfigs != "{}" {
		t.Fatalf("incompatible config survived graph update: %s", instance.NodeConfigs)
	}
	if err := service.DeleteUserFlowInstance(source.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetUserFlowInstance(instance.ID); err != nil {
		t.Fatalf("deleting an instance removed the shared graph: %v", err)
	}
}

func TestUserFlowSharedGraphSaveRollsBackWithInvalidTrigger(t *testing.T) {
	service := new_trigger_test_service(t)
	t.Cleanup(service.Stop)
	source := trigger_test_flow(t, service)
	instance, err := service.CreateUserFlowInstance(source.FlowID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.UpdateUserFlowInstance(source.ID, UpdateUserFlowInput{
		Nodes:    []UserFlowNodeInput{{ID: "start", Name: "changed", Type: "StartNode"}},
		Triggers: []UserFlowTriggerInput{{Type: "Cron", Name: "invalid", CronExpr: "invalid", Enabled: true}},
	})
	if err == nil {
		t.Fatal("invalid trigger was accepted")
	}
	loaded, err := service.GetUserFlowInstance(instance.ID)
	if err != nil || loaded.Definition != instance.Definition {
		t.Fatalf("failed save changed another instance: %+v %v", loaded, err)
	}
}

func TestUserFlowInstancesHaveIndependentRunClaims(t *testing.T) {
	service := new_trigger_test_service(t)
	t.Cleanup(service.Stop)
	source := trigger_test_flow(t, service)
	instance, err := service.CreateUserFlowInstance(source.FlowID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.prepare_run(model.FlowSchedule{InstanceID: source.ID}, "Manual", "", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := service.prepare_run(model.FlowSchedule{InstanceID: instance.ID}, "Manual", "", "", ""); err != nil {
		t.Fatalf("different configuration was blocked: %v", err)
	}
	if _, err := service.prepare_run(model.FlowSchedule{InstanceID: instance.ID}, "Manual", "", "", ""); err == nil {
		t.Fatal("same configuration allowed duplicate execution")
	}
}
