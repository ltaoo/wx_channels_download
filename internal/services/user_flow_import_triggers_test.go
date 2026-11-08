package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"wx_channel/internal/database/model"
)

func TestImportCreatorVideoSyncSchedule(t *testing.T) {
	service := new_trigger_test_service(t)
	raw, err := os.ReadFile(filepath.Join("..", "..", "examples", "pipelines", "01_creator_video_sync.json"))
	if err != nil {
		t.Fatal(err)
	}
	flow, err := service.ImportUserFlow(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	rules := trigger_test_rules(t, service, flow.ID)
	if len(rules) != 1 {
		t.Fatalf("expected one imported trigger, got %d", len(rules))
	}
	rule := rules[0]
	metadata := automation_schedule_metadata(rule)
	if rule.CronExpr != "0 12,18 * * *" || !rule.Enabled || rule.NextRunAt == nil || metadata.Type != "Cron" || metadata.StartNodeID != "start" {
		t.Fatalf("incorrect imported schedule: %+v, metadata: %+v", rule, metadata)
	}
	if _, exists := flow_initial_data(rule)["username"]; exists {
		t.Fatal("creator username should be a node configuration, not a trigger parameter")
	}
	definition, err := decode_user_flow_definition(flow.Definition)
	if err != nil {
		t.Fatal(err)
	}
	schema := definition.Nodes["validate"].ConfigSchema
	if len(definition.ContextSchema) != 0 || len(schema) != 2 || schema[0].Key != "username" || schema[0].Label != "username" || schema[0].Type != "string" || !schema[0].Required {
		t.Fatalf("import lost required creator configuration: %+v", schema)
	}
	if schema[1].Key != "remark" || schema[1].Label != "备注" || schema[1].Type != "string" || schema[1].Required {
		t.Fatalf("import lost optional remark configuration: %+v", schema)
	}
	// Check both daily fire times and rollover using the real scheduler parser.
	location := time.FixedZone("UTC+8", 8*60*60)
	from := time.Date(2026, 11, 8, 11, 59, 0, 0, location)
	wanted := []time.Time{
		time.Date(2026, 11, 8, 12, 0, 0, 0, location),
		time.Date(2026, 11, 8, 18, 0, 0, 0, location),
		time.Date(2026, 11, 9, 12, 0, 0, 0, location),
	}
	for _, expected := range wanted {
		next, err := service.next_run_at(rule.CronExpr, from)
		if err != nil || next == nil || *next != expected.UnixMilli() {
			t.Fatalf("next fire after %s: got %v, err=%v, want %s", from, next, err, expected)
		}
		from = expected
	}
	if _, err := service.UpdateUserFlow(flow.ID, UpdateUserFlowInput{}); err != nil {
		t.Fatal(err)
	}
	saved := trigger_test_rules(t, service, flow.ID)
	if len(saved) != 1 || saved[0].ID != rule.ID || *saved[0].NextRunAt != *rule.NextRunAt {
		t.Fatalf("editor save changed imported trigger: %+v", saved)
	}
}

func TestImportUserFlowTriggersRollbackAndFreshIDs(t *testing.T) {
	service := new_trigger_test_service(t)
	definition := map[string]interface{}{
		"id": "source", "name": "import test", "start_node": "entry",
		"nodes": map[string]interface{}{
			"entry": map[string]interface{}{"id": "entry", "type": "StartNode", "config": map[string]interface{}{}},
		},
		"triggers": []UserFlowTriggerInput{
			{ID: "old-id", Type: "Cron", Name: "daily", CronExpr: "0 12,18 * * *", Enabled: true},
			{Type: "Cron", Name: "invalid", CronExpr: "invalid"},
		},
	}
	raw, err := json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ImportUserFlow(string(raw)); err == nil {
		t.Fatal("accepted invalid imported cron")
	}
	for _, table := range []interface{}{&model.UserFlow{}, &model.FlowSchedule{}} {
		var count int64
		if err := service.db.Model(table).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("partial import remained: count=%d err=%v", count, err)
		}
	}
	if len(service.flow_engine.FlowDefinitions) != 0 {
		t.Fatal("failed import registered a flow")
	}
	definition["triggers"] = definition["triggers"].([]UserFlowTriggerInput)[:1]
	raw, err = json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	previous_id := "old-id"
	for index := 0; index < 2; index++ {
		flow, err := service.ImportUserFlow(string(raw))
		if err != nil {
			t.Fatal(err)
		}
		rules := trigger_test_rules(t, service, flow.ID)
		if len(rules) != 1 || rules[0].ID == previous_id || automation_schedule_metadata(rules[0]).StartNodeID != "entry" {
			t.Fatalf("import did not copy rule onto new entry: %+v", rules)
		}
		previous_id = rules[0].ID
	}
}
