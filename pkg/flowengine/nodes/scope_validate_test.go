package nodes

import (
	"strings"
	"testing"

	"wx_channel/pkg/flowengine/engine"
)

func scope_test_flow(schema []string, nodes map[string]engine.NodeDefinition) engine.FlowDefinition {
	context_schema := make([]engine.FieldSchema, 0, len(schema))
	for _, key := range schema {
		context_schema = append(context_schema, engine.FieldSchema{Key: key, Type: "string"})
	}
	return engine.FlowDefinition{
		ID:            "scope-test",
		Name:          "scope-test",
		StartNodeID:   "start",
		ContextSchema: context_schema,
		Nodes:         nodes,
	}
}

func scope_end(node_id string) engine.NodeDefinition {
	return engine.NodeDefinition{ID: node_id, Type: "EndNode"}
}

func scope_next(node_id string, next ...string) engine.NodeDefinition {
	next_nodes := make([]engine.TargetNode, 0, len(next))
	for _, target := range next {
		next_nodes = append(next_nodes, engine.TargetNode{TargetID: target})
	}
	return engine.NodeDefinition{ID: node_id, Type: "StartNode", NextNodes: next_nodes, NextNodeIDs: next}
}

func scope_gateway(node_id, condition, target_id string) engine.NodeDefinition {
	return engine.NodeDefinition{
		ID:   node_id,
		Type: "GatewayNode",
		Config: map[string]interface{}{
			"gateway_type": "Exclusive",
			"rules":        []map[string]interface{}{{"condition": condition, "target_id": target_id}},
		},
	}
}

func scope_expr(node_id, expression, output_key string) engine.NodeDefinition {
	return engine.NodeDefinition{
		ID:     node_id,
		Type:   "ExprNode",
		Config: map[string]interface{}{"expression": expression, "output_key": output_key},
	}
}

// TestValidateFlowScopesAcceptsScopedFlow is the happy path: input, output
// (keyed by producer), global and deep paths all resolve, including a producer
// key derived from output_key and the implicit <output_key>_status of an
// APICallNode.
func TestValidateFlowScopesAcceptsScopedFlow(t *testing.T) {
	definition := scope_test_flow([]string{"username"}, map[string]engine.NodeDefinition{
		"start": scope_next("start", "producer"),
		"producer": {
			ID:   "producer",
			Type: "JSCodeNode",
			Config: map[string]interface{}{
				"code":       "({username: input.username})",
				"output_key": "cleanup",
			},
			NextNodes:   []engine.TargetNode{{TargetID: "setter"}},
			NextNodeIDs: []string{"setter"},
		},
		"setter": {
			ID:     "setter",
			Type:   "SetVariableNode",
			Config: map[string]interface{}{"variables": map[string]interface{}{"token": "{{output.producer.cleanup.username}}"}},
			NextNodes: []engine.TargetNode{
				{TargetID: "fetch"},
			},
			NextNodeIDs: []string{"fetch"},
		},
		"fetch": {
			ID:   "fetch",
			Type: "APICallNode",
			Config: map[string]interface{}{
				"url":        "http://127.0.0.1/profile",
				"method":     "GET",
				"keys":       map[string]interface{}{"username": "output.producer.cleanup.username"},
				"output_key": "profile",
			},
			NextNodes:   []engine.TargetNode{{TargetID: "gate"}},
			NextNodeIDs: []string{"gate"},
		},
		"gate": scope_gateway("gate", `input.username != nil && global.token != nil && output.fetch.profile_status == 200`, "consumer"),
		"consumer": {
			ID:   "consumer",
			Type: "ServiceNode",
			Config: map[string]interface{}{
				"tool_name": "get_wxchannels_account_videos",
				"arguments": map[string]interface{}{
					"username": "{{input.username}}",
					"token":    "{{global.token}}",
					"nested":   "{{output.producer.cleanup.username}}",
					"deep":     "u={{output.fetch.profile}}",
				},
			},
			NextNodes:   []engine.TargetNode{{TargetID: "end"}},
			NextNodeIDs: []string{"end"},
		},
		"end": scope_end("end"),
	})

	if err := ValidateFlowScopes(definition); err != nil {
		t.Fatalf("expected a valid flow, got: %v", err)
	}
}

// TestValidateFlowScopesRejections pins the six rejection classes plus the
// node-level requirements the new scope rules introduce.
func TestValidateFlowScopesRejections(t *testing.T) {
	cases := []struct {
		name       string
		definition engine.FlowDefinition
		wantErr    string
	}{
		{
			name: "bare reference in expression",
			definition: scope_test_flow([]string{"username"}, map[string]engine.NodeDefinition{
				"start": scope_next("start", "calc"),
				"calc":  scope_expr("calc", "username != nil", "ok"),
				"end":   scope_end("end"),
			}),
			wantErr: "未声明作用域的名称 \"username\"",
		},
		{
			name: "bare reference in gateway condition",
			definition: scope_test_flow([]string{"videos"}, map[string]engine.NodeDefinition{
				"start": scope_next("start", "gate"),
				"gate":  scope_gateway("gate", "videos != nil", "end"),
				"end":   scope_end("end"),
			}),
			wantErr: "未声明作用域的名称 \"videos\"",
		},
		{
			name: "ctx prefix",
			definition: scope_test_flow([]string{"videos"}, map[string]engine.NodeDefinition{
				"start": scope_next("start", "gate"),
				"gate":  scope_gateway("gate", "ctx.videos != nil", "end"),
				"end":   scope_end("end"),
			}),
			wantErr: `作用域 "ctx" 不存在`,
		},
		{
			name: "bare template token",
			definition: scope_test_flow([]string{"username"}, map[string]engine.NodeDefinition{
				"start": scope_next("start", "call"),
				"call": {
					ID:          "call",
					Type:        "ServiceNode",
					Config:      map[string]interface{}{"tool_name": "t", "arguments": map[string]interface{}{"username": "{{username}}"}},
					NextNodes:   []engine.TargetNode{{TargetID: "end"}},
					NextNodeIDs: []string{"end"},
				},
				"end": scope_end("end"),
			}),
			wantErr: "缺少显式作用域",
		},
		{
			name: "unknown producer",
			definition: scope_test_flow([]string{"username"}, map[string]engine.NodeDefinition{
				"start": scope_next("start", "calc"),
				"calc":  scope_expr("calc", "output.unknown.k", "ok"),
				"end":   scope_end("end"),
			}),
			wantErr: `不存在生产者节点 "unknown"`,
		},
		{
			name: "non ancestor producer",
			definition: scope_test_flow([]string{"username"}, map[string]engine.NodeDefinition{
				"start": scope_next("start", "branch_a", "branch_b"),
				"branch_a": {
					ID:          "branch_a",
					Type:        "JSCodeNode",
					Config:      map[string]interface{}{"code": `({x: 1})`, "output_key": "x"},
					NextNodes:   []engine.TargetNode{{TargetID: "end_a"}},
					NextNodeIDs: []string{"end_a"},
				},
				"branch_b": scope_expr("branch_b", "output.branch_a.x", "ok"),
				"end_a":    scope_end("end_a"),
				"end_b":    scope_end("end_b"),
			}),
			wantErr: "不是它的上游",
		},
		{
			name: "undeclared producer key",
			definition: scope_test_flow([]string{"username"}, map[string]engine.NodeDefinition{
				"start": scope_next("start", "calc"),
				"calc":  scope_expr("calc", "output.producer.other", "ok"),
				"producer": {
					ID:          "producer",
					Type:        "JSCodeNode",
					Config:      map[string]interface{}{"code": `({x: 1})`, "output_key": "x"},
					NextNodes:   []engine.TargetNode{{TargetID: "calc"}},
					NextNodeIDs: []string{"calc"},
				},
				"end": scope_end("end"),
			}),
			wantErr: `未声明产出键 "other"`,
		},
		{
			name: "output_key shadows a context schema key",
			definition: scope_test_flow([]string{"username"}, map[string]engine.NodeDefinition{
				"start": scope_next("start", "producer"),
				"producer": {
					ID:          "producer",
					Type:        "JSCodeNode",
					Config:      map[string]interface{}{"code": `input.username`, "output_key": "username"},
					NextNodes:   []engine.TargetNode{{TargetID: "end"}},
					NextNodeIDs: []string{"end"},
				},
				"end": scope_end("end"),
			}),
			wantErr: "与流程入参名冲突",
		},
		{
			name: "js node without output_key",
			definition: scope_test_flow([]string{"username"}, map[string]engine.NodeDefinition{
				"start": scope_next("start", "producer"),
				"producer": {
					ID:          "producer",
					Type:        "JSCodeNode",
					Config:      map[string]interface{}{"code": `({x: 1})`},
					NextNodes:   []engine.TargetNode{{TargetID: "end"}},
					NextNodeIDs: []string{"end"},
				},
				"end": scope_end("end"),
			}),
			wantErr: "必须声明 output_key",
		},
		{
			name: "unmatched template braces",
			definition: scope_test_flow([]string{"username"}, map[string]engine.NodeDefinition{
				"start": scope_next("start", "call"),
				"call": {
					ID:          "call",
					Type:        "ServiceNode",
					Config:      map[string]interface{}{"tool_name": "t", "arguments": map[string]interface{}{"username": "{{oops"}},
					NextNodes:   []engine.TargetNode{{TargetID: "end"}},
					NextNodeIDs: []string{"end"},
				},
				"end": scope_end("end"),
			}),
			wantErr: "包含无效的模板表达式",
		},
		{
			name: "undeclared global",
			definition: scope_test_flow([]string{"username"}, map[string]engine.NodeDefinition{
				"start": scope_next("start", "call"),
				"call": {
					ID:          "call",
					Type:        "ServiceNode",
					Config:      map[string]interface{}{"tool_name": "t", "arguments": map[string]interface{}{"token": "{{global.token}}"}},
					NextNodes:   []engine.TargetNode{{TargetID: "end"}},
					NextNodeIDs: []string{"end"},
				},
				"end": scope_end("end"),
			}),
			wantErr: `没有 SetVariableNode 声明全局变量 "token"`,
		},
		{
			name: "undeclared input key",
			definition: scope_test_flow([]string{"username"}, map[string]engine.NodeDefinition{
				"start": scope_next("start", "calc"),
				"calc":  scope_expr("calc", "input.nope", "ok"),
				"end":   scope_end("end"),
			}),
			wantErr: `未声明入参 "nope"`,
		},
		{
			name: "sub flow is validated recursively",
			definition: scope_test_flow([]string{"username"}, map[string]engine.NodeDefinition{
				"start": scope_next("start", "run_sub"),
				"run_sub": {
					ID:   "run_sub",
					Type: "WorkflowNode",
					Config: map[string]interface{}{"workflow": scope_test_flow(nil, map[string]engine.NodeDefinition{
						"start": scope_next("start", "calc"),
						"calc":  scope_expr("calc", "username", "ok"),
						"end":   scope_end("end"),
					})},
					NextNodes:   []engine.TargetNode{{TargetID: "end"}},
					NextNodeIDs: []string{"end"},
				},
				"end": scope_end("end"),
			}),
			wantErr: "子流程 scope-test",
		},
	}

	for _, test_case := range cases {
		t.Run(test_case.name, func(t *testing.T) {
			err := ValidateFlowScopes(test_case.definition)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", test_case.wantErr)
			}
			if !strings.Contains(err.Error(), test_case.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), test_case.wantErr)
			}
		})
	}
}
