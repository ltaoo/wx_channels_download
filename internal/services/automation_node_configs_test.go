package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"wx_channel/pkg/flowengine"
	"wx_channel/pkg/flowengine/engine"
)

func TestAutomationNodeConfigsPersistAndReachEachHTTPRequest(t *testing.T) {
	service := new_trigger_test_service(t)
	t.Cleanup(service.Stop)
	requests := make(chan string, 8)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests <- request.Header.Get("Authorization")
		_, _ = response.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	nodes := map[string]flowengine.NodeDefinition{
		"start": runtime_test_node("start", "StartNode", nil, "first"),
		"fail":  runtime_test_node("fail", "FailNode", map[string]interface{}{"message": "intentional failure for retry"}),
	}
	for _, node_id := range []string{"first", "second"} {
		node := runtime_test_node(node_id, "APICallNode", map[string]interface{}{
			"url": server.URL, "headers": map[string]interface{}{"Authorization": "Bearer {{config.apikey}}"}, "output_key": node_id + "_result",
		})
		node.Name = node_id
		node.ConfigSchema = []engine.NodeConfigField{{Key: "apikey", Type: "string", Required: true, Secret: true}}
		if node_id == "first" {
			node.NextNodeIDs = []string{"second"}
		} else {
			node.NextNodeIDs = []string{"fail"}
		}
		nodes[node_id] = node
	}
	flow := import_runtime_flow(t, service, nodes)
	if _, err := service.TriggerFlowDirect(flow.ID, nil); err == nil || !strings.Contains(err.Error(), "apikey") {
		t.Fatalf("unconfigured automation ran: %v", err)
	}
	values := map[string]map[string]interface{}{"first": {"apikey": "first-key"}, "second": {"apikey": "second-key"}}
	if _, err := service.UpdateUserFlow(flow.ID, UpdateUserFlowInput{NodeConfigs: values}); err != nil {
		t.Fatal(err)
	}
	// Reload registrations from the database, as on application startup.
	service.load_user_flows()
	persisted, err := service.GetUserFlow(flow.ID)
	if err != nil || decode_node_configs(persisted.NodeConfigs)["second"]["apikey"] != "second-key" || strings.Contains(persisted.Definition, "first-key") {
		t.Fatalf("values were not stored separately on the automation: %+v %v", persisted, err)
	}
	run, err := service.TriggerFlowDirect(flow.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	service.wg.Wait()
	saved, _ := service.GetRun(run.ID)
	if saved.Status != "FAILED" || !strings.Contains(saved.Error, "intentional failure") {
		t.Fatalf("HTTP run failed: %s", saved.Error)
	}
	if len(requests) != 2 {
		t.Fatalf("expected two HTTP requests, got %d", len(requests))
	}
	if first, second := <-requests, <-requests; first != "Bearer first-key" || second != "Bearer second-key" {
		t.Fatalf("node configurations collided: %s %s", first, second)
	}
	values["first"]["apikey"] = "new-key"
	if _, err := service.UpdateUserFlow(flow.ID, UpdateUserFlowInput{NodeConfigs: values}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RetryRun(run.ID, false); err != nil {
		t.Fatal(err)
	}
	service.wg.Wait()
	if len(requests) != 2 {
		t.Fatalf("expected two HTTP requests, got %d", len(requests))
	}
	if first, second := <-requests, <-requests; first != "Bearer first-key" || second != "Bearer second-key" {
		t.Fatalf("recorded retry used changed configuration: %s %s", first, second)
	}
	if _, err := service.RetryRun(run.ID, true); err != nil {
		t.Fatal(err)
	}
	service.wg.Wait()
	if len(requests) != 2 {
		t.Fatalf("expected two HTTP requests, got %d", len(requests))
	}
	if first, second := <-requests, <-requests; first != "Bearer new-key" || second != "Bearer second-key" {
		t.Fatalf("current retry did not use current configuration: %s %s", first, second)
	}
	rule, err := service.CreateSchedule(CreateScheduleInput{Name: "configured cron", FlowID: flow.ID, CronExpr: "@daily"})
	if err != nil {
		t.Fatal(err)
	}
	scheduled, err := service.TriggerSchedule(rule.ID)
	if err != nil || scheduled.Status != "FAILED" || !strings.Contains(scheduled.Error, "intentional failure") {
		t.Fatalf("scheduled run failed: %+v %v", scheduled, err)
	}
	if len(requests) != 2 {
		t.Fatalf("expected two HTTP requests, got %d", len(requests))
	}
	if first, second := <-requests, <-requests; first != "Bearer new-key" || second != "Bearer second-key" {
		t.Fatalf("schedule did not share automation settings: %s %s", first, second)
	}
}

func TestAutomationNodeConfigValidationAndSchemaEditing(t *testing.T) {
	service := new_trigger_test_service(t)
	t.Cleanup(service.Stop)
	node := runtime_test_node("fail", "FailNode", map[string]interface{}{"message": "{{config.token}}"})
	node.Name = "request"
	node.ConfigSchema = []engine.NodeConfigField{
		{Key: "token", Type: "string", Required: true}, {Key: "count", Type: "integer"}, {Key: "enabled", Type: "boolean", Required: true},
	}
	flow := import_runtime_flow(t, service, map[string]flowengine.NodeDefinition{"start": runtime_test_node("start", "StartNode", nil, "fail"), "fail": node})
	for _, values := range []map[string]interface{}{
		{"token": " "}, {"token": "key", "enabled": "false"}, {"token": "key", "enabled": false, "count": 1.5},
	} {
		if _, err := service.UpdateUserFlow(flow.ID, UpdateUserFlowInput{NodeConfigs: map[string]map[string]interface{}{"fail": values}}); err == nil {
			t.Fatalf("accepted invalid values: %+v", values)
		}
	}
	valid := map[string]map[string]interface{}{"fail": {"token": "key", "enabled": false, "count": 0, "removed": "stale"}, "deleted-node": {"token": "stale"}}
	flow, err := service.UpdateUserFlow(flow.ID, UpdateUserFlowInput{NodeConfigs: valid})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(flow.NodeConfigs, "stale") || decode_node_configs(flow.NodeConfigs)["fail"]["enabled"] != false {
		t.Fatalf("normalization lost false/zero or retained undeclared settings: %s", flow.NodeConfigs)
	}
	// Omission from older editor clients preserves declarations and values.
	flow, err = service.UpdateUserFlow(flow.ID, UpdateUserFlowInput{Nodes: []UserFlowNodeInput{
		{ID: "start", Type: "StartNode", NextIDs: []string{"fail"}}, {ID: "fail", Type: "FailNode", Config: node.Config},
	}})
	if err != nil {
		t.Fatal(err)
	}
	definition, _ := decode_user_flow_definition(flow.Definition)
	if len(definition.Nodes["fail"].ConfigSchema) != 3 || !strings.Contains(flow.NodeConfigs, "key") {
		t.Fatalf("legacy save lost settings: %+v", flow)
	}
	flow, err = service.UpdateUserFlow(flow.ID, UpdateUserFlowInput{Nodes: []UserFlowNodeInput{
		{ID: "start", Type: "StartNode", NextIDs: []string{"fail"}}, {ID: "fail", Type: "FailNode", Config: node.Config, ConfigSchema: []engine.NodeConfigField{{Key: "token", Type: "number", Required: true}}},
	}})
	if err != nil || flow.NodeConfigs != "{}" {
		t.Fatalf("changing a type did not invalidate its old value: %+v %v", flow, err)
	}
	// Removing a declaration also removes its stored value.
	_, err = service.UpdateUserFlow(flow.ID, UpdateUserFlowInput{Nodes: []UserFlowNodeInput{
		{ID: "start", Type: "StartNode", NextIDs: []string{"fail"}}, {ID: "fail", Type: "FailNode", Config: map[string]interface{}{"message": "done"}, ConfigSchema: []engine.NodeConfigField{}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	flow, _ = service.GetUserFlow(flow.ID)
	if flow.NodeConfigs != "{}" {
		t.Fatalf("removed declaration kept values: %s", flow.NodeConfigs)
	}
}

func TestAutomationConfigScopeWorksInJSAndImportedRuntimeValuesAreDiscarded(t *testing.T) {
	service := new_trigger_test_service(t)
	t.Cleanup(service.Stop)
	node := runtime_test_node("js", "JSCodeNode", map[string]interface{}{"code": "config.count + 1", "output_key": "result"})
	node.ConfigSchema = []engine.NodeConfigField{{Key: "count", Type: "integer", Required: true}}
	node.ConfigValues = map[string]interface{}{"count": 99}
	flow := import_runtime_flow(t, service, map[string]flowengine.NodeDefinition{"start": runtime_test_node("start", "StartNode", nil, "js"), "js": node})
	if strings.Contains(flow.Definition, "config_values") {
		t.Fatal("import retained runtime values")
	}
	if _, err := service.UpdateUserFlow(flow.ID, UpdateUserFlowInput{NodeConfigs: map[string]map[string]interface{}{"js": {"count": 0}}}); err != nil {
		t.Fatal(err)
	}
	run, err := service.TriggerFlowDirect(flow.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	service.wg.Wait()
	saved, _ := service.GetRun(run.ID)
	var outputs map[string]interface{}
	_ = json.Unmarshal([]byte(saved.NodeOutputs), &outputs)
	if saved.Status != "COMPLETED" || outputs["result"] != float64(1) {
		t.Fatalf("JS config scope failed: %+v", saved)
	}
}

func TestAutomationNodeConfigChoicesAndMultilineTemplates(t *testing.T) {
	service := new_trigger_test_service(t)
	t.Cleanup(service.Stop)
	node := runtime_test_node("push", "FailNode", map[string]interface{}{"message": "{{config.message_template}}"})
	node.ConfigSchema = []engine.NodeConfigField{
		{Key: "push_method", Type: "string", Required: true, Options: []engine.NodeConfigOption{{Label: "企业微信", Value: "wecom"}, {Label: "Webhook", Value: "webhook"}}},
		{Key: "message_template", Type: "string", Multiline: true},
	}
	flow := import_runtime_flow(t, service, map[string]flowengine.NodeDefinition{"start": runtime_test_node("start", "StartNode", nil, "push"), "push": node})
	definition, _ := decode_user_flow_definition(flow.Definition)
	if len(definition.Nodes["push"].ConfigSchema[0].Options) != 2 || !definition.Nodes["push"].ConfigSchema[1].Multiline {
		t.Fatal("import lost form options or multiline control")
	}
	if _, err := service.UpdateUserFlow(flow.ID, UpdateUserFlowInput{NodeConfigs: map[string]map[string]interface{}{"push": {"push_method": "unsupported"}}}); err == nil || !strings.Contains(err.Error(), "选项") {
		t.Fatalf("invalid choice accepted: %v", err)
	}
	template := "账号：{username}\n标题：{title}"
	saved, err := service.UpdateUserFlow(flow.ID, UpdateUserFlowInput{NodeConfigs: map[string]map[string]interface{}{"push": {"push_method": "webhook", "message_template": template}}})
	if err != nil || decode_node_configs(saved.NodeConfigs)["push"]["message_template"] != template {
		t.Fatalf("multiline template did not round trip: %+v %v", saved, err)
	}
	for _, field := range []engine.NodeConfigField{
		{Key: "number", Type: "number", Multiline: true},
		{Key: "secret", Type: "string", Multiline: true, Secret: true},
		{Key: "method", Type: "string", Options: []engine.NodeConfigOption{{Value: "wecom"}, {Value: "wecom"}}},
	} {
		if err := validate_node_config_schema([]engine.NodeConfigField{field}); err == nil {
			t.Fatalf("invalid form declaration accepted: %+v", field)
		}
	}
}

func TestAutomationNodeConfigsResumeAndReferencedSubflowKeepSnapshots(t *testing.T) {
	service := new_trigger_test_service(t)
	t.Cleanup(service.Stop)
	result := runtime_test_node("result", "FailNode", map[string]interface{}{"message": "snapshot={{config.token}}"})
	result.ConfigSchema = []engine.NodeConfigField{{Key: "token", Type: "string", Required: true}}
	child := import_runtime_flow(t, service, map[string]flowengine.NodeDefinition{
		"start": runtime_test_node("start", "StartNode", nil, "result"), "result": result,
	})
	set_token := func(flow_id, token string) {
		t.Helper()
		if _, err := service.UpdateUserFlow(flow_id, UpdateUserFlowInput{NodeConfigs: map[string]map[string]interface{}{"result": {"token": token}}}); err != nil {
			t.Fatal(err)
		}
	}
	set_token(child.ID, "child-original")
	parent := import_runtime_flow(t, service, map[string]flowengine.NodeDefinition{
		"start": runtime_test_node("start", "StartNode", nil, "child"),
		"child": runtime_test_node("child", "WorkflowNode", map[string]interface{}{"flow_id": child.ID}),
	})
	run, err := service.TriggerFlowDirect(parent.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	service.wg.Wait()
	set_token(child.ID, "child-new")
	retried, err := service.RetryRun(run.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	service.wg.Wait()
	saved, _ := service.GetRun(retried.ID)
	if !strings.Contains(saved.Error, "snapshot=child-original") {
		t.Fatalf("child retry lost its frozen configuration: %s", saved.Error)
	}
	manual := import_runtime_flow(t, service, map[string]flowengine.NodeDefinition{
		"start":   runtime_test_node("start", "StartNode", nil, "confirm"),
		"confirm": runtime_test_node("confirm", "ManualNode", nil, "result"), "result": result,
	})
	set_token(manual.ID, "manual-original")
	waiting, err := service.TriggerFlowDirect(manual.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	service.wg.Wait()
	set_token(manual.ID, "manual-new")
	if _, err := service.ResumeRun(waiting.ID, "confirm", nil); err != nil {
		t.Fatal(err)
	}
	service.wg.Wait()
	saved, _ = service.GetRun(waiting.ID)
	if !strings.Contains(saved.Error, "snapshot=manual-original") {
		t.Fatalf("manual resume lost its frozen configuration: %s", saved.Error)
	}
}
