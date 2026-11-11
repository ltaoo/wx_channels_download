package nodes

import (
	"context"
	"wx_channel/pkg/flowengine/engine"
)

type CustomNodeExecutor func(context.Context, string, map[string]any, map[string]any) (any, error)

type custom_node struct {
	*ServiceNode
	node_type string
	executor  CustomNodeExecutor
}

func (node *custom_node) Type() string { return node.node_type }

func (node *custom_node) Execute(ctx *engine.ProcessContext) (bool, []string, error) {
	settings, _ := ctx.ScopeEnv()["config"].(map[string]any)
	return node.ServiceNode.execute(ctx, func(execution_context context.Context, node_type string, arguments map[string]any) (any, error) {
		return node.executor(execution_context, node_type, arguments, settings)
	})
}

// NewCustomNodeFactory reuses service templates, cancellation and output handling.
func NewCustomNodeFactory(node_type string, executor CustomNodeExecutor) func(map[string]interface{}) engine.Node {
	return func(config map[string]interface{}) engine.Node {
		config["tool_name"] = node_type
		if key, _ := config["output_key"].(string); key == "" {
			config["output_key"] = "result"
		}
		node := NewServiceNodeFactory(nil)(config).(*ServiceNode)
		return &custom_node{ServiceNode: node, node_type: node_type, executor: executor}
	}
}
