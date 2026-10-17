package nodes

import (
	"context"
	"strings"
	"testing"

	"wx_channel/pkg/flowengine/engine"
)

func template_context() *engine.ProcessContext {
	return &engine.ProcessContext{
		Data: map[string]interface{}{
			"username": "v2_abc@finder",
			"count":    3,
		},
		InputKeys: []string{"username", "count"},
		NodeOutputs: map[string]map[string]interface{}{
			"fetch_videos": {"videos": []interface{}{"a", "b"}},
			"extract":      {"payload": map[string]interface{}{"wrapper": map[string]interface{}{"deep": "value"}}},
		},
		Globals: map[string]interface{}{"flag": true},
	}
}

// TestResolveServiceTemplateNamespaces pins the scope contract: every read must
// name one of the three disjoint scopes. input.<key> is the context-schema
// projection, output.<node_id>.<key> is a specific producer's key and
// global.<key> is a SetVariableNode write. Bare keys and the ctx. prefix are
// rejected, and a "{{" that matches no token is an error rather than a silent
// passthrough.
func TestResolveServiceTemplateNamespaces(t *testing.T) {
	cases := []struct {
		name    string
		value   interface{}
		want    interface{}
		wantErr string
	}{
		{name: "input namespace", value: "{{input.username}}", want: "v2_abc@finder"},
		{name: "namespace with spaces", value: "{{ input.username }}", want: "v2_abc@finder"},
		{name: "whole value keeps type", value: "{{input.count}}", want: 3},
		{name: "interpolated", value: "u={{input.username}}&n={{input.count}}", want: "u=v2_abc@finder&n=3"},
		{name: "output projection", value: "{{output.fetch_videos.videos}}", want: []interface{}{"a", "b"}},
		{name: "output multi segment", value: "{{output.extract.payload.wrapper.deep}}", want: "value"},
		{name: "global projection", value: "{{global.flag}}", want: true},
		{name: "non string passthrough", value: 42, want: 42},
		{name: "plain string", value: "plain", want: "plain"},
		{name: "closing braces only", value: "a}}b", want: "a}}b"},
		{name: "bare key", value: "{{username}}", wantErr: "must declare an explicit scope"},
		{name: "ctx namespace", value: "{{ctx.username}}", wantErr: "unknown template namespace: ctx"},
		{name: "unknown namespace", value: "{{foo.bar}}", wantErr: "unknown template namespace: foo"},
		{name: "input undeclared key", value: "{{input.nope}}", wantErr: "context value not found: input.nope"},
		{name: "output unknown producer", value: "{{output.unknown.k}}", wantErr: "context value not found: output.unknown.k"},
		{name: "output undeclared key", value: "{{output.fetch_videos.nope}}", wantErr: "context value not found: output.fetch_videos.nope"},
		{name: "output missing node id", value: "{{output.fetch_videos}}", wantErr: "context value not found: output.fetch_videos"},
		{name: "global undeclared", value: "{{global.nope}}", wantErr: "context value not found: global.nope"},
		{name: "unmatched braces", value: "{{会打错", wantErr: "invalid template expression"},
		{name: "unmatched braces in longer text", value: "x={{会打错", wantErr: "invalid template expression"},
	}

	for _, test_case := range cases {
		t.Run(test_case.name, func(t *testing.T) {
			got, err := resolve_service_template(test_case.value, template_context())
			if test_case.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got value %v", test_case.wantErr, got)
				}
				if !strings.Contains(err.Error(), test_case.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), test_case.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !values_equal(got, test_case.want) {
				t.Fatalf("got %#v, want %#v", got, test_case.want)
			}
		})
	}
}

func values_equal(got interface{}, want interface{}) bool {
	switch expected := want.(type) {
	case []interface{}:
		actual, ok := got.([]interface{})
		if !ok || len(actual) != len(expected) {
			return false
		}
		for index := range expected {
			if actual[index] != expected[index] {
				return false
			}
		}
		return true
	default:
		return got == want
	}
}

// TestServiceNodeReadsUpstreamRewrittenValue reproduces the
// "参数处理 → 获取视频列表" pipeline with producer addressing: the JS node
// normalizes the username and the service argument reads
// output.<producer>.<key> instead of a flat key. The audit entry must carry the
// resolved arguments while the raw config keeps the template.
func TestServiceNodeReadsUpstreamRewrittenValue(t *testing.T) {
	const prefixed = "wxchannels:v2_abc@finder"
	const stripped = "v2_abc@finder"
	const tool_name = "get_wxchannels_account_videos"

	flow_engine := &engine.FlowEngine{}
	flow_engine.RegisterNode("StartNode", NewStartNode)
	flow_engine.RegisterNode("EndNode", NewEndNode)
	flow_engine.RegisterNode("JSCodeNode", NewJSCodeNode)

	var executed_arguments map[string]interface{}
	flow_engine.RegisterNode("ServiceNode", NewServiceNodeFactory(
		func(_ context.Context, got_tool string, arguments map[string]interface{}) (interface{}, error) {
			if got_tool != tool_name {
				t.Errorf("executed tool %q, want %q", got_tool, tool_name)
			}
			executed_arguments = arguments
			return map[string]interface{}{"ok": true}, nil
		},
	))

	flow_engine.SetFlowDefinitions(map[string]engine.FlowDefinition{
		"flow-test": {
			ID:            "flow-test",
			StartNodeID:   "start",
			ContextSchema: []engine.FieldSchema{{Key: "username", Type: "string", Required: true}},
			Nodes: map[string]engine.NodeDefinition{
				"start": {
					ID:          "start",
					Type:        "StartNode",
					NextNodes:   []engine.TargetNode{{TargetID: "cleanup"}},
					NextNodeIDs: []string{"cleanup"},
				},
				"cleanup": {
					ID:   "cleanup",
					Type: "JSCodeNode",
					Config: map[string]interface{}{
						"code":       `input.username.replace(/wxchannels:/, '')`,
						"output_key": "username",
					},
					NextNodes:   []engine.TargetNode{{TargetID: "fetch"}},
					NextNodeIDs: []string{"fetch"},
				},
				"fetch": {
					ID:   "fetch",
					Type: "ServiceNode",
					Config: map[string]interface{}{
						"tool_name": tool_name,
						"arguments": map[string]interface{}{"username": "{{output.cleanup.username}}"},
					},
					NextNodes:   []engine.TargetNode{{TargetID: "end"}},
					NextNodeIDs: []string{"end"},
				},
				"end": {ID: "end", Type: "EndNode"},
			},
		},
	})

	var logs []engine.NodeExecutionLog
	flow_engine.SetNodeExecutionLogHandler(func(entry engine.NodeExecutionLog) {
		logs = append(logs, entry)
	})

	if _, err := flow_engine.StartFlow("flow-test", map[string]interface{}{"username": prefixed}); err != nil {
		t.Fatalf("start flow: %v", err)
	}

	if got := executed_arguments["username"]; got != stripped {
		t.Fatalf("tool received username %#v, want %#v", got, stripped)
	}

	fetch_log := find_node_log(t, logs, "fetch")

	resolved_arguments, ok := fetch_log.Behavior["arguments"].(map[string]interface{})
	if !ok {
		t.Fatalf("behavior.arguments missing or not an object: %#v", fetch_log.Behavior["arguments"])
	}
	if got := resolved_arguments["username"]; got != stripped {
		t.Fatalf("logged arguments.username = %#v, want %#v", got, stripped)
	}

	behavior_config, ok := fetch_log.Behavior["config"].(map[string]interface{})
	if !ok {
		t.Fatalf("behavior.config missing or not an object: %#v", fetch_log.Behavior["config"])
	}
	configured_arguments, ok := behavior_config["arguments"].(map[string]interface{})
	if !ok {
		t.Fatalf("behavior.config.arguments missing or not an object: %#v", behavior_config["arguments"])
	}
	if got := configured_arguments["username"]; got != "{{output.cleanup.username}}" {
		t.Fatalf("logged config.arguments.username = %#v, want the raw template", got)
	}

	cleanup_log := find_node_log(t, logs, "cleanup")
	if got := cleanup_log.Output["username"]; got != stripped {
		t.Fatalf("cleanup produced username = %#v, want %#v", got, stripped)
	}
}

// TestServiceNodeKeepsSameKeyFromDifferentProducers pins the fix for the
// username clobbering accident: when two producers write the same key name, a
// producer-addressed read still returns the right value even though the flat
// union only holds the last write.
func TestServiceNodeKeepsSameKeyFromDifferentProducers(t *testing.T) {
	const tool_name = "get_wxchannels_account_videos"

	flow_engine := &engine.FlowEngine{}
	flow_engine.RegisterNode("StartNode", NewStartNode)
	flow_engine.RegisterNode("EndNode", NewEndNode)
	flow_engine.RegisterNode("JSCodeNode", NewJSCodeNode)

	var executed_arguments map[string]interface{}
	flow_engine.RegisterNode("ServiceNode", NewServiceNodeFactory(
		func(_ context.Context, _ string, arguments map[string]interface{}) (interface{}, error) {
			executed_arguments = arguments
			return map[string]interface{}{"ok": true}, nil
		},
	))

	flow_engine.SetFlowDefinitions(map[string]engine.FlowDefinition{
		"flow-test": {
			ID:            "flow-test",
			StartNodeID:   "start",
			ContextSchema: []engine.FieldSchema{{Key: "account", Type: "string", Required: true}},
			Nodes: map[string]engine.NodeDefinition{
				"start": {
					ID:          "start",
					Type:        "StartNode",
					NextNodes:   []engine.TargetNode{{TargetID: "account_step"}},
					NextNodeIDs: []string{"account_step"},
				},
				// account_step writes the normalized account name under its own
				// producer key; live_step then clobbers the flat `username`
				// key with a live-stream value, which is exactly the accident
				// the producer index exists to survive.
				"account_step": {
					ID:   "account_step",
					Type: "JSCodeNode",
					Config: map[string]interface{}{
						"code":       `({username: input.account})`,
						"output_key": "account_cleanup",
					},
					NextNodes:   []engine.TargetNode{{TargetID: "live_step"}},
					NextNodeIDs: []string{"live_step"},
				},
				"live_step": {
					ID:   "live_step",
					Type: "JSCodeNode",
					Config: map[string]interface{}{
						"code":       `"live-owner"`,
						"output_key": "username",
					},
					NextNodes:   []engine.TargetNode{{TargetID: "fetch"}},
					NextNodeIDs: []string{"fetch"},
				},
				"fetch": {
					ID:   "fetch",
					Type: "ServiceNode",
					Config: map[string]interface{}{
						"tool_name": tool_name,
						"arguments": map[string]interface{}{"username": "{{output.account_step.account_cleanup.username}}"},
					},
					NextNodes:   []engine.TargetNode{{TargetID: "end"}},
					NextNodeIDs: []string{"end"},
				},
				"end": {ID: "end", Type: "EndNode"},
			},
		},
	})

	var logs []engine.NodeExecutionLog
	flow_engine.SetNodeExecutionLogHandler(func(entry engine.NodeExecutionLog) {
		logs = append(logs, entry)
	})

	if _, err := flow_engine.StartFlow("flow-test", map[string]interface{}{"account": "v2_abc@finder"}); err != nil {
		t.Fatalf("start flow: %v", err)
	}

	if got := executed_arguments["username"]; got != "v2_abc@finder" {
		t.Fatalf("tool received username %#v, want the account value %#v", got, "v2_abc@finder")
	}
	fetch_log := find_node_log(t, logs, "fetch")
	if got := fetch_log.Input["username"]; got != "live-owner" {
		t.Fatalf("expected the flat union to be polluted with %#v, got %#v", "live-owner", got)
	}
}

func find_node_log(t *testing.T, logs []engine.NodeExecutionLog, node_id string) engine.NodeExecutionLog {
	t.Helper()
	for _, entry := range logs {
		if entry.NodeID == node_id {
			return entry
		}
	}
	t.Fatalf("no execution log for node %q", node_id)
	return engine.NodeExecutionLog{}
}
