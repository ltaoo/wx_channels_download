package services

import (
	"reflect"
	"testing"

	"wx_channel/internal/database/model"
)

func TestCopyUserFlowIncludesConfigurationAndTriggers(t *testing.T) {
	service := new_trigger_test_service(t)
	source, err := service.ImportUserFlowWithInstance(`{"name":"creator","start_node":"start","nodes":{"start":{"id":"start","type":"StartNode","next_node_ids":["validate"]},"validate":{"id":"validate","type":"JSCodeNode","config":{"code":"config.username","output_key":"creator"},"config_schema":[{"key":"username","type":"string"},{"key":"token","type":"string","secret":true}]}}}`)
	if err != nil {
		t.Fatal(err)
	}
	description := "creator sync"
	active := false
	source, err = service.UpdateUserFlowInstance(source.ID, UpdateUserFlowInput{
		Description: &description,
		NodeConfigs: map[string]map[string]interface{}{"validate": {"username": "creator@finder", "token": "saved-token"}},
		Triggers: []UserFlowTriggerInput{
			{Type: "Cron", Name: "daily", CronExpr: "0 12 * * *", Enabled: true, InitialData: map[string]interface{}{"limit": 10}},
			{Type: "Webhook", Name: "hook", Active: &active},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	original_rules := trigger_test_rules(t, service, source.ID)
	if err := service.db.Model(&model.FlowSchedule{}).Where("instance_id = ?", source.ID).Updates(map[string]interface{}{"last_run_id": "old-run", "last_run_status": "COMPLETED"}).Error; err != nil {
		t.Fatal(err)
	}
	copied, err := service.CopyUserFlowInstance(source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if copied.ID == source.ID || copied.Name != source.Name+"（副本）" || copied.Description != source.Description || copied.NodeConfigs != source.NodeConfigs || copied.TriggerType != source.TriggerType || copied.EventKey != source.EventKey {
		t.Fatalf("flow fields were not copied: %+v", copied)
	}
	original_definition, _ := decode_user_flow_definition(source.Definition)
	copied_definition, err := decode_user_flow_definition(copied.Definition)
	if err != nil {
		t.Fatal(err)
	}
	if copied_definition.ID != source.FlowID || copied.FlowID != source.FlowID {
		t.Fatal("copied definition kept source identity")
	}
	copied_definition.ID, copied_definition.Name = original_definition.ID, original_definition.Name
	if !reflect.DeepEqual(original_definition, copied_definition) {
		t.Fatal("copied graph differs from source")
	}
	copied_rules := trigger_test_rules(t, service, copied.ID)
	if len(copied_rules) != len(original_rules) {
		t.Fatal("trigger count differs")
	}
	for _, copied_rule := range copied_rules {
		var original model.FlowSchedule
		for _, rule := range original_rules {
			if rule.Name == copied_rule.Name {
				original = rule
			}
		}
		if copied_rule.ID == original.ID || copied_rule.InstanceID != copied.ID || copied_rule.CronExpr != original.CronExpr || copied_rule.Enabled != original.Enabled || copied_rule.TimeoutSec != original.TimeoutSec || copied_rule.LastRunID != "" || copied_rule.LastRunStatus != "" {
			t.Fatalf("incorrect copied trigger: %+v", copied_rule)
		}
		original_data, copied_data := decode_initial_data(original.InitialData), decode_initial_data(copied_rule.InitialData)
		if automation_schedule_metadata(original).Type == "Webhook" {
			original_metadata := original_data[automation_metadata_key].(map[string]interface{})
			copied_metadata := copied_data[automation_metadata_key].(map[string]interface{})
			if copied_metadata["webhook_token"] == "" || copied_metadata["webhook_token"] == original_metadata["webhook_token"] {
				t.Fatal("webhook token was not regenerated")
			}
			delete(original_metadata, "webhook_token")
			delete(copied_metadata, "webhook_token")
		}
		if !reflect.DeepEqual(original_data, copied_data) {
			t.Fatal("trigger parameters differ")
		}
	}
	if _, err := service.UpdateUserFlowInstance(copied.ID, UpdateUserFlowInput{NodeConfigs: map[string]map[string]interface{}{"validate": {"username": "other@finder"}}}); err != nil {
		t.Fatal(err)
	}
	unchanged, err := service.GetUserFlowInstance(source.ID)
	if err != nil || unchanged.NodeConfigs != source.NodeConfigs || unchanged.Definition != source.Definition || unchanged.Name != source.Name {
		t.Fatalf("copy modified source: %v", err)
	}
}

func TestCopyUserFlowRollbackAndMissingSource(t *testing.T) {
	service := new_trigger_test_service(t)
	source := trigger_test_flow(t, service)
	if _, err := service.UpdateUserFlowInstance(source.ID, UpdateUserFlowInput{Triggers: []UserFlowTriggerInput{{Type: "Cron", Name: "daily", CronExpr: "0 12 * * *", Enabled: true}}}); err != nil {
		t.Fatal(err)
	}
	if err := service.db.Model(&model.FlowSchedule{}).Where("instance_id = ?", source.ID).Update("cron_expr", "invalid").Error; err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{source.ID, "missing", ""} {
		if _, err := service.CopyUserFlowInstance(id); err == nil {
			t.Fatalf("copied invalid source %q", id)
		}
	}
	flows, err := service.ListUserFlowInstances()
	if err != nil || len(flows) != 1 || len(service.flow_engine.FlowDefinitions) != 1 {
		t.Fatalf("failed copy left partial data: %v", err)
	}
	if rules := trigger_test_rules(t, service, ""); len(rules) != 1 {
		t.Fatal("failed copy left partial triggers")
	}
}
