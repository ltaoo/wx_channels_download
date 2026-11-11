package services

import (
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

func TestFlowImportRejectsInstanceTriggerSettings(t *testing.T) {
	service := new_trigger_test_service(t)
	t.Cleanup(service.Stop)
	if _, err := service.ImportFlowDefinition(`{"name":"flow","start_node":"start","nodes":{"start":{"type":"StartNode"}},"triggers":[{"type":"Cron","name":"daily","cron_expr":"@daily"}]}`); err == nil {
		t.Fatal("a definition accepted instance-only trigger settings")
	}
	flows, err := service.ListFlowDefinitions()
	if err != nil || len(flows) != 0 {
		t.Fatalf("rejected import left a partial definition: %+v %v", flows, err)
	}
}
