package services

import (
	"encoding/json"
	"strings"
	"testing"
	"wx_channel/internal/database/model"
)

func TestFlowImportCreatesNoImplicitInstancesOrTriggers(t *testing.T) {
	service := new_trigger_test_service(t)
	t.Cleanup(service.Stop)
	raw := `{"name":"creator sync","start_node":"start","nodes":{"start":{"type":"StartNode"}}}`
	first, err := service.ImportFlowDefinition(raw)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.ImportFlowDefinition(raw)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID {
		t.Fatal("separate imports were collapsed")
	}
	flows, err := service.ListFlowDefinitions()
	if err != nil || len(flows) != 2 || flows[0].InstanceCount != 0 {
		t.Fatalf("flow list is not a direct definition list: %+v %v", flows, err)
	}
	instances, err := service.ListUserFlowInstances()
	if err != nil || len(instances) != 0 {
		t.Fatalf("import created implicit instances: %+v %v", instances, err)
	}
	var count int64
	if err := service.db.Model(&model.FlowSchedule{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("flow import created instance trigger rules")
	}
	instance, err := service.CreateUserFlowInstance(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpdateUserFlowInstance(instance.ID, UpdateUserFlowInput{Triggers: []UserFlowTriggerInput{{Name: "daily", Type: "Cron", CronExpr: "@daily", Enabled: true}}}); err != nil {
		t.Fatal(err)
	}
	rules := trigger_test_rules(t, service, instance.ID)
	if len(rules) != 1 || rules[0].InstanceID != instance.ID || rules[0].FlowID != first.ID {
		t.Fatalf("rule did not belong to the instance: %+v", rules)
	}
}

func TestFlowImportValidatesDefaultTriggersWithoutInstances(t *testing.T) {
	service := new_trigger_test_service(t)
	t.Cleanup(service.Stop)
	if _, err := service.ImportFlowDefinition(`{"name":"flow","start_node":"start","nodes":{"start":{"type":"StartNode"}},"triggers":[{"type":"Cron","name":"daily","cron_expr":"invalid"}]}`); err == nil {
		t.Fatal("a definition accepted an invalid default Cron")
	}
	flows, err := service.ListFlowDefinitions()
	if err != nil || len(flows) != 0 {
		t.Fatalf("rejected import left a partial definition: %+v %v", flows, err)
	}
}

func TestFlowDefaultTriggersInheritOverrideAndRestore(t *testing.T) {
	service := new_trigger_test_service(t)
	t.Cleanup(service.Stop)
	flow, err := service.ImportFlowDefinition(`{"name":"defaults","start_node":"start","nodes":{"start":{"type":"StartNode"}},"triggers":[{"id":"daily","type":"Cron","name":"daily","cron_expr":"0 12 * * *","enabled":true},{"id":"finished","type":"Event","name":"finished","event_key":"downloadtask.finished"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	instances, err := service.ListUserFlowInstances()
	if err != nil || len(instances) != 0 {
		t.Fatalf("import implicitly created instances: %v %v", instances, err)
	}
	first, err := service.CreateUserFlowInstance(flow.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateUserFlowInstance(flow.ID)
	if err != nil {
		t.Fatal(err)
	}
	first_rules := trigger_test_rules(t, service, first.ID)
	second_rules := trigger_test_rules(t, service, second.ID)
	if !first.InheritTriggers || len(first_rules) != 2 || len(second_rules) != 2 || first_rules[0].ID == second_rules[0].ID {
		t.Fatalf("instances did not inherit independent bindings: %v %v", first_rules, second_rules)
	}
	if _, err := service.UpdateUserFlowInstance(first.ID, UpdateUserFlowInput{Triggers: []UserFlowTriggerInput{{Type: "Cron", Name: "custom", CronExpr: "0 9 * * *", Enabled: false}}}); err != nil {
		t.Fatal(err)
	}
	var defaults []UserFlowTriggerInput
	if err := json.Unmarshal([]byte(flow.DefaultTriggers), &defaults); err != nil {
		t.Fatal(err)
	}
	var daily model.FlowSchedule
	for _, rule := range second_rules {
		if rule.Name == "daily" {
			daily = rule
		}
	}
	data := flow_initial_data(daily)
	data["runtime_cursor"] = "keep"
	data[automation_metadata_key] = decode_initial_data(daily.InitialData)[automation_metadata_key]
	if err := service.db.Model(&daily).Updates(map[string]interface{}{"initial_data": marshal_json(data), "last_run_id": "history"}).Error; err != nil {
		t.Fatal(err)
	}
	defaults[0].CronExpr = "0 18 * * *"
	if _, err := service.UpdateFlowDefinition(flow.ID, UpdateUserFlowInput{Triggers: defaults}); err != nil {
		t.Fatal(err)
	}
	first_rules = trigger_test_rules(t, service, first.ID)
	if len(first_rules) != 1 || first_rules[0].CronExpr != "0 9 * * *" || first_rules[0].Enabled {
		t.Fatalf("flow update overwrote instance overrides: %v", first_rules)
	}
	updated, err := service.GetSchedule(daily.ID)
	if err != nil || updated.CronExpr != "0 18 * * *" || updated.LastRunID != "history" || flow_initial_data(*updated)["runtime_cursor"] != "keep" {
		t.Fatalf("inheritance lost binding identity, history or runtime state: %+v %v", updated, err)
	}
	inherit := true
	if _, err := service.UpdateUserFlowInstance(first.ID, UpdateUserFlowInput{InheritTriggers: &inherit}); err != nil {
		t.Fatal(err)
	}
	if rules := trigger_test_rules(t, service, first.ID); len(rules) != 2 {
		t.Fatalf("restoring inheritance did not restore defaults: %v", rules)
	}
	if _, err := service.UpdateFlowDefinition(flow.ID, UpdateUserFlowInput{Triggers: []UserFlowTriggerInput{}}); err != nil {
		t.Fatal(err)
	}
	if rules := trigger_test_rules(t, service, first.ID); len(rules) != 0 {
		t.Fatalf("deleted defaults left inherited rules active: %v", rules)
	}
}

func TestInheritedScheduleEditsAndCopiesPreserveOwnership(t *testing.T) {
	service := new_trigger_test_service(t)
	t.Cleanup(service.Stop)
	flow, err := service.ImportFlowDefinition(`{"name":"daily","start_node":"start","nodes":{"start":{"type":"StartNode"}},"triggers":[{"id":"daily","type":"Cron","name":"daily","cron_expr":"0 12 * * *","enabled":true}]}`)
	if err != nil {
		t.Fatal(err)
	}
	instance, err := service.CreateUserFlowInstance(flow.ID)
	if err != nil {
		t.Fatal(err)
	}
	copy, err := service.CopyUserFlowInstance(instance.ID)
	if err != nil || !copy.InheritTriggers {
		t.Fatalf("copy lost inheritance: %+v %v", copy, err)
	}
	rule := trigger_test_rules(t, service, instance.ID)[0]
	data := decode_initial_data(rule.InitialData)
	data["cursor"] = "runtime-progress"
	if _, err := service.UpdateSchedule(rule.ID, UpdateScheduleInput{InitialData: data}); err != nil {
		t.Fatal(err)
	}
	loaded, err := service.GetUserFlowInstance(instance.ID)
	if err != nil || !loaded.InheritTriggers {
		t.Fatalf("runtime input update detached inheritance: %+v %v", loaded, err)
	}
	if _, err := service.ToggleSchedule(rule.ID); err != nil {
		t.Fatal(err)
	}
	loaded, _ = service.GetUserFlowInstance(instance.ID)
	if !loaded.InheritTriggers || !strings.Contains(loaded.TriggerOverrides, `"enabled":false`) {
		t.Fatal("direct schedule edit did not record a field override")
	}
	if _, err := service.UpdateFlowDefinition(flow.ID, UpdateUserFlowInput{Triggers: []UserFlowTriggerInput{{ID: "daily", Type: "Cron", Name: "daily", CronExpr: "0 18 * * *", Enabled: true}}}); err != nil {
		t.Fatal(err)
	}
	if rules := trigger_test_rules(t, service, instance.ID); rules[0].Enabled || rules[0].CronExpr != "0 18 * * *" {
		t.Fatalf("flow update overwrote a direct schedule override: %v", rules)
	}
	if rules := trigger_test_rules(t, service, copy.ID); rules[0].CronExpr != "0 18 * * *" || rules[0].ID == rule.ID {
		t.Fatalf("inherited copy did not receive defaults independently: %v", rules)
	}
}

func TestTriggerFieldOverridesKeepOtherDefaultsLive(t *testing.T) {
	service := new_trigger_test_service(t)
	t.Cleanup(service.Stop)
	flow, err := service.ImportFlowDefinition(`{"name":"fields","start_node":"start","nodes":{"start":{"type":"StartNode"}},"triggers":[{"id":"daily","name":"daily","type":"Cron","cron_expr":"0 12 * * *","enabled":true,"timeout_sec":600},{"id":"finished","name":"finished","type":"Event","event_key":"downloadtask.finished","active":true,"timeout_sec":600}]}`)
	if err != nil {
		t.Fatal(err)
	}
	instance, err := service.CreateUserFlowInstance(flow.ID)
	if err != nil {
		t.Fatal(err)
	}
	overrides := map[string]map[string]interface{}{"daily": {"cron_expr": "0 9 * * *"}, "finished": {"active": false}}
	if _, err := service.UpdateUserFlowInstance(instance.ID, UpdateUserFlowInput{TriggerOverrides: overrides}); err != nil {
		t.Fatal(err)
	}
	var defaults []UserFlowTriggerInput
	if err := json.Unmarshal([]byte(flow.DefaultTriggers), &defaults); err != nil {
		t.Fatal(err)
	}
	timeout_sec := 900
	defaults[0].CronExpr, defaults[0].Enabled, defaults[0].TimeoutSec = "0 18 * * *", false, &timeout_sec
	defaults[1].EventKey, defaults[1].TimeoutSec = "downloadtask.updated", &timeout_sec
	before := trigger_test_rules(t, service, instance.ID)
	if _, err := service.UpdateFlowDefinition(flow.ID, UpdateUserFlowInput{Triggers: defaults}); err != nil {
		t.Fatal(err)
	}
	for _, previous := range before {
		rule, err := service.GetSchedule(previous.ID)
		if err != nil || rule.TimeoutSec != 900 {
			t.Fatalf("unoverridden timeout stopped inheriting or rule ID changed: %+v %v", rule, err)
		}
		metadata := automation_schedule_metadata(*rule)
		if metadata.Type == "Cron" && (rule.CronExpr != "0 9 * * *" || rule.Enabled) {
			t.Fatalf("Cron override or inherited enabled state lost: %+v", rule)
		}
		if metadata.Type == "Event" && (metadata.Active || metadata.EventKey != "downloadtask.updated") {
			t.Fatalf("event pause override or inherited key lost: %+v", metadata)
		}
	}
	loaded, _ := service.GetUserFlowInstance(instance.ID)
	if !loaded.InheritTriggers || loaded.TriggerOverrides != marshal_json(overrides) {
		t.Fatalf("instance stored more than explicit field overrides: %+v", loaded)
	}
	copied, err := service.CopyUserFlowInstance(instance.ID)
	if err != nil || copied.TriggerOverrides != loaded.TriggerOverrides || !copied.InheritTriggers {
		t.Fatalf("copy lost field overrides or inheritance: %+v %v", copied, err)
	}
	if _, err := service.UpdateUserFlowInstance(instance.ID, UpdateUserFlowInput{TriggerOverrides: map[string]map[string]interface{}{"finished": {"active": false}}}); err != nil {
		t.Fatal(err)
	}
	for _, rule := range trigger_test_rules(t, service, instance.ID) {
		if rule.CronExpr != "@daily" && rule.CronExpr != "0 18 * * *" {
			t.Fatalf("restoring one field did not use the newest default: %+v", rule)
		}
	}
	for _, invalid := range []map[string]map[string]interface{}{
		{"missing": {"enabled": false}}, {"daily": {"id": "foreign"}},
		{"daily": {"timeout_sec": 0}}, {"daily": {"enabled": "false"}},
		{"daily": {"cron_expr": "invalid"}}, {"finished": {"event_key": ""}},
	} {
		if _, err := service.UpdateUserFlowInstance(instance.ID, UpdateUserFlowInput{TriggerOverrides: invalid}); err == nil {
			t.Fatalf("accepted invalid trigger field overrides: %v", invalid)
		}
	}
	loaded, _ = service.GetUserFlowInstance(instance.ID)
	if loaded.TriggerOverrides != `{"finished":{"active":false}}` {
		t.Fatal("invalid patch changed persisted overrides")
	}
	if _, err := service.UpdateUserFlowInstance(instance.ID, UpdateUserFlowInput{TriggerOverrides: map[string]map[string]interface{}{}}); err != nil {
		t.Fatal(err)
	}
	loaded, _ = service.GetUserFlowInstance(instance.ID)
	if loaded.TriggerOverrides != "{}" {
		t.Fatal("restoring all defaults left overrides behind")
	}
}

func TestTriggerInputOverrideReplacesConfiguredKeysAndPreservesProgress(t *testing.T) {
	service := new_trigger_test_service(t)
	t.Cleanup(service.Stop)
	flow, err := service.ImportFlowDefinition(`{"name":"input","start_node":"start","nodes":{"start":{"type":"StartNode"}},"triggers":[{"id":"finished","name":"finished","type":"Event","event_key":"test.finished","initial_data":{"old":1}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	instance, err := service.CreateUserFlowInstance(flow.ID)
	if err != nil {
		t.Fatal(err)
	}
	rule := trigger_test_rules(t, service, instance.ID)[0]
	data := decode_initial_data(rule.InitialData)
	data["cursor"] = "keep"
	if _, err := service.UpdateSchedule(rule.ID, UpdateScheduleInput{InitialData: data}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpdateUserFlowInstance(instance.ID, UpdateUserFlowInput{TriggerOverrides: map[string]map[string]interface{}{"finished": {"initial_data": map[string]interface{}{"new": 2}}}}); err != nil {
		t.Fatal(err)
	}
	updated, _ := service.GetSchedule(rule.ID)
	values := flow_initial_data(*updated)
	if _, old := values["old"]; old || values["new"] != float64(2) || values["cursor"] != "keep" {
		t.Fatalf("object override merged old configured keys or lost runtime progress: %v", values)
	}
	if _, err := service.UpdateFlowDefinition(flow.ID, UpdateUserFlowInput{Triggers: []UserFlowTriggerInput{{ID: "finished", Name: "finished", Type: "Event", EventKey: "test.finished", InitialData: map[string]interface{}{"latest": 3}}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpdateUserFlowInstance(instance.ID, UpdateUserFlowInput{TriggerOverrides: map[string]map[string]interface{}{}}); err != nil {
		t.Fatal(err)
	}
	updated, _ = service.GetSchedule(rule.ID)
	values = flow_initial_data(*updated)
	if _, old := values["new"]; old || values["latest"] != float64(3) || values["cursor"] != "keep" {
		t.Fatalf("restoring the object field did not use current defaults: %v", values)
	}
}

func TestInheritedEventTriggerRespondsForEachInstance(t *testing.T) {
	service := new_trigger_test_service(t)
	t.Cleanup(service.Stop)
	flow, err := service.ImportFlowDefinition(`{"name":"event","start_node":"event","context_schema":[{"key":"event","type":"object","required":true}],"nodes":{"event":{"type":"JSCodeNode","config":{"code":"input.event.data.value","output_key":"received"}}},"triggers":[{"type":"Event","name":"finished","event_key":"test.finished"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	for count := 0; count < 2; count++ {
		if _, err := service.CreateUserFlowInstance(flow.ID); err != nil {
			t.Fatal(err)
		}
	}
	runs, err := service.DispatchEvent("test.finished", map[string]interface{}{"value": "received"})
	if err != nil || len(runs) != 2 {
		t.Fatalf("inherited event did not dispatch both instances: %v %v", runs, err)
	}
	service.wg.Wait()
	for _, run := range runs {
		loaded, err := service.GetRun(run.ID)
		if err != nil || loaded.Status != "COMPLETED" {
			t.Fatalf("inherited event did not execute: %+v %v", loaded, err)
		}
	}
}
