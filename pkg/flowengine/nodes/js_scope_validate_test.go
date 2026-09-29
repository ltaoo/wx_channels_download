package nodes

import (
	"strings"
	"testing"

	"wx_channel/pkg/flowengine/engine"
)

func scope_gateway_with_language(node_id, condition, target_id, language string) engine.NodeDefinition {
	return engine.NodeDefinition{
		ID:   node_id,
		Type: "GatewayNode",
		Config: map[string]interface{}{
			"gateway_type":       "Exclusive",
			"condition_language": language,
			"rules":              []map[string]interface{}{{"condition": condition, "target_id": target_id}},
		},
	}
}

func scope_gateway_js(node_id, condition, target_id string) engine.NodeDefinition {
	return scope_gateway_with_language(node_id, condition, target_id, ConditionLanguageJS)
}

// scope_js_flow builds `start → producer → setter → gate → end` so a JS
// condition can be checked against a declared input, an ancestor producer key
// and a SetVariableNode global.
func scope_js_flow(gate engine.NodeDefinition) engine.FlowDefinition {
	return scope_test_flow([]string{"username", "count", "items", "flag"}, map[string]engine.NodeDefinition{
		"start": scope_next("start", "producer"),
		"producer": {
			ID:           "producer",
			Type:         "JSCodeNode",
			Config:       map[string]interface{}{"code": "({payload: input.count})", "output_key": "cleanup"},
			OutputSchema: []engine.FieldSchema{{Key: "payload", Type: "number"}},
			NextNodes:    []engine.TargetNode{{TargetID: "setter"}},
			NextNodeIDs:  []string{"setter"},
		},
		"setter": {
			ID:          "setter",
			Type:        "SetVariableNode",
			Config:      map[string]interface{}{"variables": map[string]interface{}{"token": "{{input.username}}"}},
			NextNodes:   []engine.TargetNode{{TargetID: "gate"}},
			NextNodeIDs: []string{"gate"},
		},
		"gate": gate,
		"end":  scope_end("end"),
	})
}

// TestValidateFlowScopesAcceptsJSConditions pins the JS scanner's happy path:
// scoped reads, string-literal and optional-chain member access, whitelisted
// built-ins and arrow-function parameters (which are bindings, not bare names).
func TestValidateFlowScopesAcceptsJSConditions(t *testing.T) {
	conditions := []string{
		`input.username != ""`,
		`input.username !== ""`,
		`Math.max(input.count, 0) > 0`,
		`Number.isFinite(input.count)`,
		`input.username.length > 0`,
		`input["username"].length > 0`,
		`output.producer.payload == 1`,
		`global.token != null`,
		`input.items.map(v => v.id).length > 0`,
		`output?.producer?.payload != null`,
		`let threshold = Math.max(input.count, 0); input.items.length > threshold`,
		`typeof input.username !== "undefined"`,
		`[input.count, input.items.length].length > 0`,
		`input.count > 0 ? input.username != "" : input.flag`,
		// Statement forms: the scanner walks them and must not trip over an
		// absent optional field (TryStatement.Finally here).
		`try { input.count > 0 } catch (e) { false }`,
		`for (const item of input.items) { item != null }`,
		"`${input.username}:${input.count}`",
	}

	for _, condition := range conditions {
		t.Run(condition, func(t *testing.T) {
			if err := ValidateFlowScopes(scope_js_flow(scope_gateway_js("gate", condition, "end"))); err != nil {
				t.Fatalf("expected a valid JS condition, got: %v", err)
			}
		})
	}
}

// TestValidateFlowScopesRejectsJSConditions pins the JS diagnostics: a bare
// name (the typo the lint exists to catch), a scope path that does not resolve,
// a dynamic index, a syntax error and an unsupported language.
func TestValidateFlowScopesRejectsJSConditions(t *testing.T) {
	cases := []struct {
		name      string
		gate      engine.NodeDefinition
		wantErr   string
		wantExtra string
	}{
		{
			name:    "bare name",
			gate:    scope_gateway_js("gate", `username != ""`, "end"),
			wantErr: `未声明的名称 "username"`,
		},
		{
			name:      "bare name uses the JS message",
			gate:      scope_gateway_js("gate", `input && input.username`, "end"),
			wantErr:   `未声明的名称 "input"`,
			wantExtra: "JavaScript 条件",
		},
		{
			name:    "missing producer or key",
			gate:    scope_gateway_js("gate", "output.x > 0", "end"),
			wantErr: "缺少生产者节点或产出键",
		},
		{
			name:    "undeclared input key",
			gate:    scope_gateway_js("gate", "input.nope == 1", "end"),
			wantErr: `未声明入参 "nope"`,
		},
		{
			name:    "undeclared global",
			gate:    scope_gateway_js("gate", "global.nope != null", "end"),
			wantErr: `没有 SetVariableNode 声明全局变量 "nope"`,
		},
		{
			name:    "dynamic property access",
			gate:    scope_gateway_js("gate", "input[k] > 0", "end"),
			wantErr: "动态属性访问",
		},
		{
			name:    "dynamic property on a deep path",
			gate:    scope_gateway_js("gate", "output.producer[k] > 0", "end"),
			wantErr: "动态属性访问",
		},
		{
			name:    "syntax error",
			gate:    scope_gateway_js("gate", "input.count ==", "end"),
			wantErr: "无法解析",
		},
		{
			name:    "unsupported language",
			gate:    scope_gateway_with_language("gate", "input.count > 0", "end", "python"),
			wantErr: "不支持（仅支持 expr / js）",
		},
	}

	for _, test_case := range cases {
		t.Run(test_case.name, func(t *testing.T) {
			err := ValidateFlowScopes(scope_js_flow(test_case.gate))
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", test_case.wantErr)
			}
			if !strings.Contains(err.Error(), test_case.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), test_case.wantErr)
			}
			if test_case.wantExtra != "" && !strings.Contains(err.Error(), test_case.wantExtra) {
				t.Fatalf("error %q does not contain %q", err.Error(), test_case.wantExtra)
			}
		})
	}
}

// TestValidateFlowScopesKeepsExprGatewayDiagnostics pins that a gateway with no
// condition_language still reports the pre-existing expr wording, so the expr
// branch of the dispatch is provably unchanged.
func TestValidateFlowScopesKeepsExprGatewayDiagnostics(t *testing.T) {
	definition := scope_test_flow([]string{"videos"}, map[string]engine.NodeDefinition{
		"start": scope_next("start", "gate"),
		"gate":  scope_gateway("gate", "videos != nil", "end"),
		"end":   scope_end("end"),
	})
	err := ValidateFlowScopes(definition)
	if err == nil {
		t.Fatal("expected an error for a bare expr reference")
	}
	if !strings.Contains(err.Error(), `未声明作用域的名称 "videos"`) {
		t.Fatalf("expr gateway diagnostic changed: %v", err)
	}
	if strings.Contains(err.Error(), "JavaScript 条件") {
		t.Fatalf("expr gateway took the JS path: %v", err)
	}
}

// TestJSConditionScopePaths pins the scanner output directly, including that a
// static chain reports one path (not its prefixes) and that whitelisted roots
// are not reported as bare names.
func TestJSConditionScopePaths(t *testing.T) {
	cases := []struct {
		name        string
		code        string
		wantPaths   []string
		wantBare    []string
		wantDynamic []string
	}{
		{
			name:      "static chain reports one path",
			code:      "input.a.b > 0",
			wantPaths: []string{"input.a.b"},
		},
		{
			name:      "bracket literal chain",
			code:      `input["a"].b > 0`,
			wantPaths: []string{"input.a.b"},
		},
		{
			name:      "optional chain",
			code:      "output?.producer?.payload == 1",
			wantPaths: []string{"output.producer.payload"},
		},
		{
			name:      "whitelisted root is not bare",
			code:      "Math.max(input.count, 0) > 0",
			wantPaths: []string{"input.count"},
		},
		{
			name:      "method name is not a bare name",
			code:      "input.items.map(v => v.id).length > 0",
			wantPaths: []string{"input.items.map"},
		},
		{
			name:     "unknown root is bare",
			code:     "foo.bar > 0",
			wantBare: []string{"foo"},
		},
		{
			name:        "dynamic index",
			code:        "input[k] > 0",
			wantDynamic: []string{"input"},
		},
	}

	for _, test_case := range cases {
		t.Run(test_case.name, func(t *testing.T) {
			paths, bare, dynamic, err := js_condition_scope_paths(test_case.code)
			if err != nil {
				t.Fatalf("unexpected parse error: %v", err)
			}
			assert_strings(t, "paths", paths, test_case.wantPaths)
			assert_strings(t, "bare", bare, test_case.wantBare)
			assert_strings(t, "dynamic", dynamic, test_case.wantDynamic)
		})
	}
}

func assert_strings(t *testing.T, label string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("%s = %v, want %v", label, got, want)
		}
	}
}
