package flowengine

import (
	"wx_channel/pkg/flowengine/engine"
	"wx_channel/pkg/flowengine/nodes"
)

// NewWorkflowEngine 初始化引擎 (省略具体的注册和加载逻辑)
func NewWorkflowEngine() *engine.FlowEngine {
	e := &engine.FlowEngine{
		FlowDefinitions: make(map[string]engine.FlowDefinition),
		RunningContexts: make(map[string]*engine.ProcessContext),
		NodeRegistry:    make(map[string]func(map[string]interface{}) engine.Node),
	}
	e.RegisterNode("StartNode", nodes.NewStartNode)
	e.RegisterNode("EndNode", nodes.NewEndNode)
	e.RegisterNode("GatewayNode", nodes.NewGatewayNode)
	e.RegisterNode("ExprNode", nodes.NewExprNode)
	e.RegisterNode("FuncNode", nodes.NewFuncNode)
	e.RegisterNode("LoopNode", nodes.NewLoopNode)
	e.RegisterNode("WorkflowNode", nodes.NewWorkflowNode)
	e.RegisterNode("APICallNode", nodes.NewAPICallNode)
	e.RegisterNode("ManualNode", nodes.NewManualNode)
	return e
}
