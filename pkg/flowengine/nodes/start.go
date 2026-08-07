package nodes

import "wx_channel/pkg/flowengine/engine"

type StartNode struct {
	Id     string
	Config map[string]interface{}
}

func NewStartNode(config map[string]interface{}) engine.Node {
	id, _ := config["id"].(string)
	return &StartNode{Id: id, Config: config}
}

func (n *StartNode) ID() string   { return n.Id }
func (n *StartNode) Type() string { return "StartNode" }

func (n *StartNode) Execute(ctx *engine.ProcessContext) (bool, []string, error) {
	next := ctx.EngineRef.GetNextNodeIDsFromDefinition(ctx, n.Id)
	return true, next, nil
}
