package services

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"wx_channel/internal/database/model"
	"wx_channel/internal/events"
	"wx_channel/pkg/flowengine"
)

func import_runtime_flow(t *testing.T, service *AutomationService, nodes map[string]flowengine.NodeDefinition, schema ...flowengine.FieldSchema) *model.UserFlow {
	t.Helper()
	definition := flowengine.FlowDefinition{ID: "template", Name: t.Name(), StartNodeID: "start", ContextSchema: schema, Nodes: nodes}
	raw, err := json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	flow, err := service.ImportUserFlow(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	return flow
}

func runtime_test_node(id, node_type string, config map[string]interface{}, next ...string) flowengine.NodeDefinition {
	return flowengine.NodeDefinition{ID: id, Type: node_type, Config: config, NextNodeIDs: next}
}

func wait_runtime_node(t *testing.T, service *AutomationService, id, node string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		run, err := service.GetRun(id)
		if err != nil {
			t.Fatal(err)
		}
		if run.CurrentNode == node {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("run %s never entered node %s", id, node)
}

func TestAutomationCancellationAndDeadlineStopJS(t *testing.T) {
	service := new_trigger_test_service(t)
	t.Cleanup(service.Stop)
	flow := import_runtime_flow(t, service, map[string]flowengine.NodeDefinition{
		"start":   runtime_test_node("start", "StartNode", nil, "forever"),
		"forever": runtime_test_node("forever", "JSCodeNode", map[string]interface{}{"code": "while (true) {}", "output_key": "never"}, "after"),
		"after":   runtime_test_node("after", "FailNode", map[string]interface{}{"message": "must not run"}),
	})
	run, err := service.TriggerFlowDirect(flow.ID, nil)
	if err != nil || run.ID == "" {
		t.Fatalf("run not durably allocated: %+v %v", run, err)
	}
	wait_runtime_node(t, service, run.ID, "forever")
	if err := service.CancelRun(run.ID); err != nil {
		t.Fatal(err)
	}
	service.wg.Wait()
	saved, _ := service.GetRun(run.ID)
	if saved.Status != "CANCELLED" || saved.CurrentNode != "forever" {
		t.Fatalf("cancel overwritten: %+v", saved)
	}
	rule, err := service.CreateSchedule(CreateScheduleInput{Name: "deadline", FlowID: flow.ID, CronExpr: "@daily", TimeoutSec: 1})
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	timed, err := service.TriggerSchedule(rule.ID)
	if err != nil || timed.Status != "FAILED" || !strings.Contains(timed.Error, "deadline") || time.Since(started) > 2*time.Second {
		t.Fatalf("deadline ineffective: %+v, %v", timed, err)
	}
}

func TestAutomationManualResumeSurvivesRestartAndGraphEdit(t *testing.T) {
	first := new_trigger_test_service(t)
	flow := import_runtime_flow(t, first, map[string]flowengine.NodeDefinition{
		"start":   runtime_test_node("start", "StartNode", nil, "seed"),
		"seed":    runtime_test_node("seed", "SetVariableNode", map[string]interface{}{"variables": map[string]interface{}{"state": "original"}}, "confirm"),
		"confirm": {ID: "confirm", Type: "ManualNode", InputSchema: []flowengine.FieldSchema{{Key: "approved", Type: "boolean", Required: true}}, NextNodeIDs: []string{"result"}},
		"result":  runtime_test_node("result", "JSCodeNode", map[string]interface{}{"code": "global.state + ':' + output.confirm.response.approved", "output_key": "result_value"}),
	})
	run, err := first.TriggerFlowDirect(flow.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	first.wg.Wait()
	waiting, _ := first.GetRun(run.ID)
	if waiting.Status != "WAITING" || waiting.Checkpoint == "" {
		t.Fatalf("not recoverable: %+v", waiting)
	}
	if _, err := first.TriggerFlowDirect(flow.ID, nil); err == nil {
		t.Fatal("waiting run allowed duplicate execution")
	}
	if _, err := first.ResumeRun(run.ID, "confirm", map[string]interface{}{"approved": "bad type"}); err == nil {
		t.Fatal("invalid confirmation accepted")
	}
	definition, _ := decode_user_flow_definition(flow.Definition)
	result := definition.Nodes["result"]
	result.Config["code"] = "'changed graph'"
	definition.Nodes["result"] = result
	raw, _ := json.Marshal(definition)
	if err := first.db.Model(&model.UserFlow{}).Where("id = ?", flow.ID).Update("definition", string(raw)).Error; err != nil {
		t.Fatal(err)
	}
	first.Stop()
	second := NewAutomationService(first.db, nil, flowengine.NewWorkflowEngine(), nil)
	t.Cleanup(second.Stop)
	second.recover_interrupted_runs()
	resumed, err := second.ResumeRun(run.ID, "confirm", map[string]interface{}{"approved": true})
	if err != nil || resumed.ID != run.ID {
		t.Fatalf("resume: %+v %v", resumed, err)
	}
	second.wg.Wait()
	completed, _ := second.GetRun(run.ID)
	if completed.Status != "COMPLETED" || !strings.Contains(completed.NodeOutputs, "original:true") || strings.Contains(completed.NodeOutputs, "changed graph") {
		t.Fatalf("resume changed definition/scopes: %+v", completed)
	}
	if _, err := second.ResumeRun(run.ID, "confirm", nil); err == nil {
		t.Fatal("confirmed twice")
	}
	var logs []flowengine.NodeExecutionLog
	if json.Unmarshal([]byte(completed.NodeLogs), &logs) != nil || len(logs) < 4 {
		t.Fatalf("node logs missing: %s", completed.NodeLogs)
	}
}

func TestAutomationRetryUsesRecordedDefinitionAndInput(t *testing.T) {
	service := new_trigger_test_service(t)
	t.Cleanup(service.Stop)
	flow := import_runtime_flow(t, service, map[string]flowengine.NodeDefinition{
		"start": runtime_test_node("start", "StartNode", nil, "fail"),
		"fail":  runtime_test_node("fail", "FailNode", map[string]interface{}{"message": "old {{input.value}}"}),
	}, flowengine.FieldSchema{Key: "value", Type: "string", Required: true})
	run, err := service.TriggerFlowDirect(flow.ID, map[string]interface{}{"value": "input"})
	if err != nil {
		t.Fatal(err)
	}
	service.wg.Wait()
	definition, _ := decode_user_flow_definition(flow.Definition)
	node := definition.Nodes["fail"]
	node.Config["message"] = "new {{input.value}}"
	definition.Nodes["fail"] = node
	service.register_user_flow(*definition)
	original, err := service.RetryRun(run.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	service.wg.Wait()
	saved, _ := service.GetRun(original.ID)
	if saved.Error != "node fail (FailNode): old input" || saved.RetryOf != run.ID {
		t.Fatalf("wrong original retry: %+v", saved)
	}
	current, err := service.RetryRun(run.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	service.wg.Wait()
	saved, _ = service.GetRun(current.ID)
	if !strings.Contains(saved.Error, "new input") {
		t.Fatalf("current definition unused: %+v", saved)
	}
}

func TestAutomationEventMatchingAndAuthenticatedWebhook(t *testing.T) {
	service := new_trigger_test_service(t)
	t.Cleanup(service.Stop)
	flow := import_runtime_flow(t, service, map[string]flowengine.NodeDefinition{
		"start": runtime_test_node("start", "StartNode", nil, "read"),
		"read":  runtime_test_node("read", "JSCodeNode", map[string]interface{}{"code": "input.event.data.value", "output_key": "event_value"}),
	}, flowengine.FieldSchema{Key: "event", Type: "object"})
	_, err := service.UpdateUserFlow(flow.ID, UpdateUserFlowInput{Triggers: []UserFlowTriggerInput{
		{Type: "Event", Name: "event", EventKey: "custom.created"}, {Type: "Webhook", Name: "hook"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	unmatched, err := service.DispatchEvent("other.event", nil)
	if err != nil || len(unmatched) != 0 {
		t.Fatal("unmatched event executed")
	}
	runs, err := service.DispatchEvent("custom.created", map[string]interface{}{"value": "received"})
	if err != nil || len(runs) != 1 {
		t.Fatalf("event failed: %+v %v", runs, err)
	}
	service.wg.Wait()
	saved, _ := service.GetRun(runs[0].ID)
	if saved.Status != "COMPLETED" || !strings.Contains(saved.NodeOutputs, "received") {
		t.Fatalf("event payload missing: %+v", saved)
	}
	var hook model.FlowSchedule
	for _, rule := range trigger_test_rules(t, service, flow.ID) {
		if automation_schedule_metadata(rule).Type == "Webhook" {
			hook = rule
		}
	}
	token := automation_schedule_metadata(hook).WebhookToken
	if len(token) != 64 {
		t.Fatal("webhook token was not generated")
	}
	if _, err := service.TriggerWebhook(hook.ID, "wrong", nil); err == nil {
		t.Fatal("unauthenticated webhook accepted")
	}
	hooked, err := service.TriggerWebhook(hook.ID, token, map[string]interface{}{"value": "hooked"})
	if err != nil {
		t.Fatal(err)
	}
	service.wg.Wait()
	saved, _ = service.GetRun(hooked.ID)
	if saved.Status != "COMPLETED" || saved.TriggerType != "Webhook" || !strings.Contains(saved.NodeOutputs, "hooked") {
		t.Fatalf("webhook failed: %+v", saved)
	}
	// Internal progress events cannot recursively start workflows.
	if _, err := service.DispatchEvent(events.TypeAutomationNodeLog, nil); err == nil {
		t.Fatal("automation recursion accepted")
	}
}

func TestAutomationExecutionPolicySurvivesLegacySave(t *testing.T) {
	service := new_trigger_test_service(t)
	fail := runtime_test_node("fail", "FailNode", map[string]interface{}{"message": "failure"})
	fail.RetryPolicy = &flowengine.RetryPolicy{MaxAttempts: 3, DelayMs: 1, BackoffMultiplier: 2}
	fail.ErrorNextNodeID = "recover"
	flow := import_runtime_flow(t, service, map[string]flowengine.NodeDefinition{
		"start": runtime_test_node("start", "StartNode", nil, "fail"), "fail": fail,
		"recover": runtime_test_node("recover", "JSCodeNode", map[string]interface{}{"code": "output.fail.error.message", "output_key": "recovered"}),
	})
	definition, _ := decode_user_flow_definition(flow.Definition)
	inputs := []UserFlowNodeInput{}
	for _, node := range definition.Nodes {
		inputs = append(inputs, UserFlowNodeInput{ID: node.ID, Type: node.Type, Config: node.Config, NextIDs: node.NextNodeIDs})
	}
	saved, err := service.UpdateUserFlow(flow.ID, UpdateUserFlowInput{Nodes: inputs})
	if err != nil {
		t.Fatal(err)
	}
	definition, _ = decode_user_flow_definition(saved.Definition)
	if definition.Nodes["fail"].RetryPolicy.MaxAttempts != 3 || definition.Nodes["fail"].ErrorNextNodeID != "recover" {
		t.Fatal("legacy save dropped execution policy")
	}
	run, err := service.TriggerFlowDirect(flow.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	service.wg.Wait()
	result, _ := service.GetRun(run.ID)
	if result.Status != "COMPLETED" || !strings.Contains(result.NodeAttempts, `"fail":3`) {
		t.Fatalf("retry policy not executed: %+v", result)
	}
}
