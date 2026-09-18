package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wx_channel/pkg/flowengine/engine"
)

// TestManualLiveDownloadFlowIsMigrated keeps the hand-migrated sample flow at
// the repository root valid under the scope contract, so the documented
// migration cannot silently rot.
func TestManualLiveDownloadFlowIsMigrated(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "manual_live_download_flow.json"))
	if err != nil {
		t.Skipf("sample flow not available: %v", err)
	}
	definition, err := decode_user_flow_definition(string(raw))
	if err != nil {
		t.Fatalf("decode sample flow: %v", err)
	}
	if err := normalize_user_flow_definition(definition); err != nil {
		t.Fatalf("sample flow does not satisfy the scope contract: %v", err)
	}
}

func scoped_user_flow(name string) *engine.FlowDefinition {
	return &engine.FlowDefinition{
		ID:            "flow-u-test",
		Name:          name,
		StartNodeID:   "start",
		ContextSchema: []engine.FieldSchema{{Key: "username", Type: "string"}},
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
					"output_key": "cleanup",
				},
				NextNodes:   []engine.TargetNode{{TargetID: "end"}},
				NextNodeIDs: []string{"end"},
			},
			"end": {ID: "end", Type: "EndNode"},
		},
	}
}

// TestNormalizeUserFlowDefinitionAcceptsScopedFlow pins the accept path so the
// new scope rules do not reject a flow that only reads through
// input./output./global.
func TestNormalizeUserFlowDefinitionAcceptsScopedFlow(t *testing.T) {
	definition := scoped_user_flow("scoped")
	definition.Nodes["fetch"] = engine.NodeDefinition{
		ID:   "fetch",
		Type: "ServiceNode",
		Config: map[string]interface{}{
			"tool_name": "get_wxchannels_account_videos",
			"arguments": map[string]interface{}{"username": "{{output.cleanup.cleanup}}"},
		},
		NextNodes:   []engine.TargetNode{{TargetID: "end"}},
		NextNodeIDs: []string{"end"},
	}
	cleanup := definition.Nodes["cleanup"]
	cleanup.NextNodes = []engine.TargetNode{{TargetID: "fetch"}}
	cleanup.NextNodeIDs = []string{"fetch"}
	definition.Nodes["cleanup"] = cleanup

	if err := normalize_user_flow_definition(definition); err != nil {
		t.Fatalf("expected a valid flow, got: %v", err)
	}
}

// TestNormalizeUserFlowDefinitionRejectsUnscopedReads is the regression guard
// for the save/import path: a definition whose reads are not explicitly scoped
// must be rejected before it reaches the engine.
func TestNormalizeUserFlowDefinitionRejectsUnscopedReads(t *testing.T) {
	cases := []struct {
		name      string
		mutate    func(*engine.FlowDefinition)
		wantError string
	}{
		{
			name: "bare template token",
			mutate: func(definition *engine.FlowDefinition) {
				definition.Nodes["fetch"] = engine.NodeDefinition{
					ID:   "fetch",
					Type: "ServiceNode",
					Config: map[string]interface{}{
						"tool_name": "get_wxchannels_account_videos",
						"arguments": map[string]interface{}{"username": "{{username}}"},
					},
				}
			},
			wantError: "缺少显式作用域",
		},
		{
			name: "ctx namespace",
			mutate: func(definition *engine.FlowDefinition) {
				definition.Nodes["fetch"] = engine.NodeDefinition{
					ID:   "fetch",
					Type: "ServiceNode",
					Config: map[string]interface{}{
						"tool_name": "get_wxchannels_account_videos",
						"arguments": map[string]interface{}{"username": "{{ctx.username}}"},
					},
				}
			},
			wantError: `作用域 "ctx" 不存在`,
		},
		{
			// JSCodeNode bodies are opaque, so the read has to sit in an
			// expression for the validator to see it.
			name: "unknown producer",
			mutate: func(definition *engine.FlowDefinition) {
				definition.Nodes["calc"] = engine.NodeDefinition{
					ID:     "calc",
					Type:   "ExprNode",
					Config: map[string]interface{}{"expression": "output.unknown.k", "output_key": "calc"},
				}
			},
			wantError: `不存在生产者节点 "unknown"`,
		},
		{
			name: "js node without output_key",
			mutate: func(definition *engine.FlowDefinition) {
				cleanup := definition.Nodes["cleanup"]
				cleanup.Config = map[string]interface{}{"code": `({x: 1})`}
				definition.Nodes["cleanup"] = cleanup
			},
			wantError: "必须声明 output_key",
		},
		{
			name: "output_key shadows a context schema key",
			mutate: func(definition *engine.FlowDefinition) {
				cleanup := definition.Nodes["cleanup"]
				cleanup.Config["output_key"] = "username"
				definition.Nodes["cleanup"] = cleanup
			},
			wantError: "与流程入参名冲突",
		},
		{
			name: "unsupported node type",
			mutate: func(definition *engine.FlowDefinition) {
				definition.Nodes["sneaky"] = engine.NodeDefinition{ID: "sneaky", Type: "FuncNode"}
			},
			wantError: "节点类型不支持",
		},
	}

	for _, test_case := range cases {
		t.Run(test_case.name, func(t *testing.T) {
			definition := scoped_user_flow("rejected")
			test_case.mutate(definition)
			err := normalize_user_flow_definition(definition)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", test_case.wantError)
			}
			if !strings.Contains(err.Error(), test_case.wantError) {
				t.Fatalf("error %q does not contain %q", err.Error(), test_case.wantError)
			}
		})
	}
}

// TestApplyUserFlowUpdateEnforcesScopeContract guards the hole where the editor
// save path validated node configs but never the assembled graph. UpdateUserFlow
// itself needs a database, so the pure node-assembly helper it delegates to is
// exercised directly.
func TestApplyUserFlowUpdateEnforcesScopeContract(t *testing.T) {
	definition := scoped_user_flow("delegated")
	definition.Nodes = nil

	scoped_save := UpdateUserFlowInput{
		StartNodeID: "start",
		Nodes: []UserFlowNodeInput{
			{ID: "start", Type: "StartNode", NextIDs: []string{"fetch"}},
			{
				ID:   "fetch",
				Type: "ServiceNode",
				Config: map[string]interface{}{
					"tool_name": "get_wxchannels_account_videos",
					"arguments": map[string]interface{}{"username": "{{input.username}}"},
				},
				NextIDs: []string{"end"},
			},
			{ID: "end", Type: "EndNode"},
		},
	}
	if err := apply_user_flow_update(definition, scoped_save); err != nil {
		t.Fatalf("expected the scoped save to be accepted, got: %v", err)
	}

	unscoped_save := UpdateUserFlowInput{
		StartNodeID: "start",
		Nodes: []UserFlowNodeInput{
			{ID: "start", Type: "StartNode", NextIDs: []string{"fetch"}},
			{
				ID:   "fetch",
				Type: "ServiceNode",
				Config: map[string]interface{}{
					"tool_name": "get_wxchannels_account_videos",
					"arguments": map[string]interface{}{"username": "{{username}}"},
				},
				NextIDs: []string{"end"},
			},
			{ID: "end", Type: "EndNode"},
		},
	}
	definition = scoped_user_flow("delegated")
	definition.Nodes = nil
	err := apply_user_flow_update(definition, unscoped_save)
	if err == nil {
		t.Fatal("expected an unscoped read to be rejected on save")
	}
	if !strings.Contains(err.Error(), "缺少显式作用域") {
		t.Fatalf("error %q does not mention the missing scope", err.Error())
	}
}

// TestUserFlowNodeCatalogPublishesScopeContract keeps the editor hints aligned
// with the runtime: no input_map / data, keys is an object, and the global
// scope has a declaring node type.
func TestUserFlowNodeCatalogPublishesScopeContract(t *testing.T) {
	catalog := SortedUserFlowNodeCatalog()
	by_type := map[string]UserFlowNodeCatalogItem{}
	for _, item := range catalog {
		by_type[item.Type] = item
	}

	for _, node_type := range []string{"SetVariableNode", "JSCodeNode", "APICallNode", "ServiceNode"} {
		if _, ok := by_type[node_type]; !ok {
			t.Fatalf("catalog is missing %s", node_type)
		}
	}

	service := by_type["ServiceNode"]
	has_input_map := false
	has_keys := false
	for _, key := range service.ConfigKeys {
		if key.Key == "input_map" {
			has_input_map = true
		}
		if key.Key == "keys" {
			has_keys = true
		}
	}
	if has_input_map {
		t.Fatal("ServiceNode still advertises the removed input_map")
	}
	if has_keys {
		t.Fatal("keys belongs to APICallNode, not ServiceNode")
	}

	api := by_type["APICallNode"]
	api_keys_type := ""
	for _, key := range api.ConfigKeys {
		if key.Key == "keys" {
			api_keys_type = key.Type
		}
	}
	if api_keys_type != "object" {
		t.Fatalf("APICallNode.keys type = %q, want object", api_keys_type)
	}

	js := by_type["JSCodeNode"]
	output_key_required := false
	for _, key := range js.ConfigKeys {
		if key.Key == "output_key" {
			output_key_required = key.Required
		}
	}
	if !output_key_required {
		t.Fatal("JSCodeNode.output_key must be advertised as required")
	}

	// GatewayNode conditions are expr by default but can opt into js, so the
	// key must be discoverable in the editor's raw JSON view.
	gateway := by_type["GatewayNode"]
	condition_language_type := ""
	for _, key := range gateway.ConfigKeys {
		if key.Key == "condition_language" {
			condition_language_type = key.Type
		}
	}
	if condition_language_type != "string" {
		t.Fatalf("GatewayNode.condition_language type = %q, want string", condition_language_type)
	}
}
