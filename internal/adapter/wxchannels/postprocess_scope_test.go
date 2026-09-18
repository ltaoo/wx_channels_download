package wxchannelsadapter

import (
	"testing"

	"wx_channel/pkg/flowengine"
	"wx_channel/pkg/flowengine/nodes"
)

// TestBuiltinFlowsRespectScopeContract pins the built-in wxchannels flows to
// the same scope rules user flows are held to. Every read in every built-in
// definition must name input.<key>, output.<producer>.<key> or global.<key>;
// sub-flows embedded in WorkflowNode config are validated recursively. This
// keeps a built-in flow from drifting back to unscoped reads that the runtime
// resolvers would only reject at run time.
func TestBuiltinFlowsRespectScopeContract(t *testing.T) {
	definitions := []struct {
		name       string
		definition flowengine.FlowDefinition
	}{
		{name: "wxchannels_postprocess_main_flow", definition: wxchannels_postprocess_main_flow},
		{name: "wxchannels_postprocess_flow", definition: wxchannels_postprocess_flow},
		{name: "wxchannels_output_flow", definition: wxchannels_output_flow},
	}

	for _, test_case := range definitions {
		t.Run(test_case.name, func(t *testing.T) {
			if err := nodes.ValidateFlowScopes(test_case.definition); err != nil {
				t.Fatalf("内置流程未通过变量作用域校验: %v", err)
			}
		})
	}
}

// TestLoggedGatewayConditionDelegatesToSharedEvaluator pins that the built-in
// postprocess gateway no longer keeps a private copy of the expr evaluator: the
// config has no condition_language, so it resolves to expr (the behavior the
// built-in flows rely on), and opting into js changes the truthiness rule
// exactly as it does for a user gateway. logf is a no-op without a
// postprocess_run in the context, so no flow needs to run.
func TestLoggedGatewayConditionDelegatesToSharedEvaluator(t *testing.T) {
	ctx := &flowengine.ProcessContext{
		Data:        map[string]interface{}{"count": 3},
		InputKeys:   []string{"count"},
		NodeOutputs: map[string]map[string]interface{}{},
		Globals:     map[string]interface{}{},
	}
	node := &wxchannels_logged_gateway_node{config: map[string]interface{}{}}

	ok, err := node.evaluate_condition(ctx, "input.count > 1")
	if err != nil || !ok {
		t.Fatalf("expr condition: ok=%v err=%v", ok, err)
	}
	// expr returns only bools: the number itself is not a pass.
	ok, err = node.evaluate_condition(ctx, "input.count")
	if err != nil {
		t.Fatalf("expr non-bool condition errored: %v", err)
	}
	if ok {
		t.Fatal("expr must treat a non-bool result as false")
	}

	node.config["condition_language"] = "js"
	ok, err = node.evaluate_condition(ctx, "input.count")
	if err != nil || !ok {
		t.Fatalf("js truthiness condition: ok=%v err=%v", ok, err)
	}
}
