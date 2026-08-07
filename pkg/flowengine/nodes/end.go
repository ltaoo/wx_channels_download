package nodes

import "wx_channel/pkg/flowengine/engine"

type EndNode struct {
	Id     string
	Config map[string]interface{}
}

func NewEndNode(config map[string]interface{}) engine.Node {
	id, _ := config["id"].(string)
	return &EndNode{Id: id, Config: config}
}

func (n *EndNode) ID() string   { return n.Id }
func (n *EndNode) Type() string { return "EndNode" }

func (n *EndNode) Execute(ctx *engine.ProcessContext) (bool, []string, error) {
	if p, ok := ctx.Data["local_path"].(string); ok {
		ctx.Data["result_path"] = p
	}
	return true, nil, nil
}
