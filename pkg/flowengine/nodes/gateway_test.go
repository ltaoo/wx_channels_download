package nodes

import (
	"testing"

	"wx_channel/pkg/flowengine/engine"
)

func gateway_context(data map[string]interface{}, input_keys ...string) *engine.ProcessContext {
	return &engine.ProcessContext{
		Data:        data,
		InputKeys:   input_keys,
		NodeOutputs: map[string]map[string]interface{}{},
		Globals:     map[string]interface{}{},
	}
}

// gateway_exclusive executes an Exclusive gateway with a yes-rule followed by a
// `true` fallback, so a false result is distinguishable from an error and the
// table can assert which branch was taken.
func gateway_exclusive(t *testing.T, ctx *engine.ProcessContext, config map[string]interface{}, condition string) (string, error) {
	t.Helper()
	config["id"] = "gate"
	config["gateway_type"] = "Exclusive"
	config["rules"] = []map[string]interface{}{
		{"condition": condition, "target_id": "yes"},
		{"condition": "true", "target_id": "no"},
	}
	node := NewGatewayNode(config)
	ok, next, err := node.Execute(ctx)
	if err != nil {
		return "", err
	}
	if !ok || len(next) != 1 {
		t.Fatalf("Execute returned ok=%v next=%v", ok, next)
	}
	return next[0], nil
}

// TestGatewayDefaultLanguageIsExpr is the backward-compatibility pin: a gateway
// with no condition_language evaluates with expr, including expr-only syntax
// (len()) and expr's strict bool rule (a non-bool result never passes).
func TestGatewayDefaultLanguageIsExpr(t *testing.T) {
	ctx := gateway_context(map[string]interface{}{"videos": []interface{}{"a", "b"}}, "videos")

	got, err := gateway_exclusive(t, ctx, map[string]interface{}{}, "len(input.videos) > 1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "yes" {
		t.Fatalf("branch = %q, want yes", got)
	}

	// expr returns only bools; a non-bool result is silently false, so the
	// fallback rule wins.
	got, err = gateway_exclusive(t, ctx, map[string]interface{}{}, "input.videos")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "no" {
		t.Fatalf("expr non-bool result took branch %q, want no", got)
	}
}

// TestGatewayJSConditionTruthiness pins JavaScript truthiness for a js
// condition: undefined / null / 0 / "" are false, while a non-empty string, a
// non-empty array (a Go slice is a real JS array) and an object are true.
// This differs from expr, which would treat `input.username` as false.
func TestGatewayJSConditionTruthiness(t *testing.T) {
	cases := []struct {
		name       string
		data       map[string]interface{}
		input_keys []string
		condition  string
		want       string
	}{
		{name: "number comparison", data: map[string]interface{}{"count": 3}, input_keys: []string{"count"}, condition: "input.count != 0", want: "yes"},
		{name: "non empty string", data: map[string]interface{}{"username": "v2_abc"}, input_keys: []string{"username"}, condition: "input.username", want: "yes"},
		{name: "empty string value", data: map[string]interface{}{"username": ""}, input_keys: []string{"username"}, condition: "input.username", want: "no"},
		{name: "false boolean", data: map[string]interface{}{"flag": false}, input_keys: []string{"flag"}, condition: "input.flag", want: "no"},
		{name: "missing key is undefined", data: map[string]interface{}{}, input_keys: []string{"username"}, condition: "input.username", want: "no"},
		{name: "empty string literal", condition: `""`, want: "no"},
		{name: "zero literal", condition: "0", want: "no"},
		{name: "string zero literal is truthy", condition: `"0"`, want: "yes"},
		{name: "non empty array length", data: map[string]interface{}{"items": []interface{}{"a"}}, input_keys: []string{"items"}, condition: "input.items.length > 0", want: "yes"},
		{name: "empty array length", data: map[string]interface{}{"items": []interface{}{}}, input_keys: []string{"items"}, condition: "input.items.length > 0", want: "no"},
		{name: "object literal is truthy", condition: "({})", want: "yes"},
	}

	for _, test_case := range cases {
		t.Run(test_case.name, func(t *testing.T) {
			ctx := gateway_context(test_case.data, test_case.input_keys...)
			got, err := gateway_exclusive(t, ctx, map[string]interface{}{"condition_language": "js"}, test_case.condition)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != test_case.want {
				t.Fatalf("branch = %q, want %q", got, test_case.want)
			}
		})
	}
}

// TestGatewayJSConditionFailureFailsNode pins that a thrown exception or a
// syntax error fails the node instead of silently routing.
func TestGatewayJSConditionFailureFailsNode(t *testing.T) {
	cases := []struct {
		name      string
		condition string
	}{
		{name: "runtime throw", condition: `(() => { throw new Error("boom") })()`},
		{name: "property of undefined", condition: "input.nope.deep"},
		{name: "syntax error", condition: "input.count =="},
	}

	for _, test_case := range cases {
		t.Run(test_case.name, func(t *testing.T) {
			ctx := gateway_context(map[string]interface{}{"count": 1}, "count")
			if _, err := gateway_exclusive(t, ctx, map[string]interface{}{"condition_language": "js"}, test_case.condition); err == nil {
				t.Fatalf("expected an error for condition %q", test_case.condition)
			}
		})
	}
}

// TestGatewayUnsupportedLanguageFailsNode pins that an unvalidated language
// does not silently fall back to expr at run time.
func TestGatewayUnsupportedLanguageFailsNode(t *testing.T) {
	ctx := gateway_context(map[string]interface{}{"count": 1}, "count")
	if _, err := gateway_exclusive(t, ctx, map[string]interface{}{"condition_language": "python"}, "input.count > 0"); err == nil {
		t.Fatal("expected an error for an unsupported condition_language")
	}
}

// TestGatewayParallelUnaffected pins that the language setting is ignored by a
// Parallel gateway, which routes on its next_node_ids unconditionally.
func TestGatewayParallelUnaffected(t *testing.T) {
	config := map[string]interface{}{
		"id":                 "gate",
		"gateway_type":       "Parallel",
		"condition_language": "js",
		"next_node_ids":      []string{"a", "b"},
		"rules":              []map[string]interface{}{{"condition": "input.", "target_id": "a"}},
	}
	ctx := gateway_context(map[string]interface{}{})
	ok, next, err := NewGatewayNode(config).Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok || len(next) != 2 || next[0] != "a" || next[1] != "b" {
		t.Fatalf("Execute returned ok=%v next=%v", ok, next)
	}
}

func TestConditionLanguageFromConfig(t *testing.T) {
	cases := []struct {
		name   string
		config map[string]interface{}
		want   string
	}{
		{name: "nil config", config: nil, want: "expr"},
		{name: "absent key", config: map[string]interface{}{}, want: "expr"},
		{name: "empty value", config: map[string]interface{}{"condition_language": ""}, want: "expr"},
		{name: "blank value", config: map[string]interface{}{"condition_language": "   "}, want: "expr"},
		{name: "expr", config: map[string]interface{}{"condition_language": "expr"}, want: "expr"},
		{name: "upper case expr", config: map[string]interface{}{"condition_language": "EXPR"}, want: "expr"},
		{name: "js", config: map[string]interface{}{"condition_language": "js"}, want: "js"},
		{name: "upper case js", config: map[string]interface{}{"condition_language": "JS"}, want: "js"},
		{name: "padded js", config: map[string]interface{}{"condition_language": " js "}, want: "js"},
		{name: "unsupported", config: map[string]interface{}{"condition_language": "python"}, want: "python"},
	}

	for _, test_case := range cases {
		t.Run(test_case.name, func(t *testing.T) {
			if got := ConditionLanguage(test_case.config); got != test_case.want {
				t.Fatalf("ConditionLanguage = %q, want %q", got, test_case.want)
			}
		})
	}
}

// TestEvaluateConditionExprMatchesLegacyBehavior pins the expr evaluator the
// built-in flows rely on: a non-bool result is silently false (unlike JS
// truthiness, where the same condition would pass) and a compile error is
// returned.
func TestEvaluateConditionExprMatchesLegacyBehavior(t *testing.T) {
	ctx := gateway_context(map[string]interface{}{"username": "v2_abc"}, "username")

	ok, err := EvaluateCondition(ctx, `len(input.username) > 0`, ConditionLanguageExpr)
	if err != nil || !ok {
		t.Fatalf("bool condition: ok=%v err=%v", ok, err)
	}
	ok, err = EvaluateCondition(ctx, "input.username", ConditionLanguageExpr)
	if err != nil {
		t.Fatalf("non-bool condition errored: %v", err)
	}
	if ok {
		t.Fatal("expr must treat a non-bool result as false")
	}
	if _, err = EvaluateCondition(ctx, `username == "x"`, ConditionLanguageExpr); err == nil {
		t.Fatal("expr must reject a bare identifier")
	}
}
