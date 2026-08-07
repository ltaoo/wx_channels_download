package nodes

import (
	"fmt"

	"wx_channel/pkg/flowengine/engine"
)

type FuncNode struct {
	Id     string
	Config map[string]interface{}
}

func NewFuncNode(config map[string]interface{}) engine.Node {
	id, _ := config["id"].(string)
	return &FuncNode{Id: id, Config: config}
}

func (n *FuncNode) ID() string   { return n.Id }
func (n *FuncNode) Type() string { return "FuncNode" }

func (n *FuncNode) Execute(ctx *engine.ProcessContext) (bool, []string, error) {
	fn, ok := n.Config["func"].(func(map[string]interface{}) (interface{}, error))
	if !ok {
		return false, nil, fmt.Errorf("func not provided")
	}
	out, err := fn(ctx.Data)
	if err != nil {
		return false, nil, err
	}
	if outKey, ok := n.Config["output_key"].(string); ok && outKey != "" {
		ctx.Data[outKey] = out
	}
	next := ctx.EngineRef.GetNextNodeIDsFromDefinition(ctx, n.Id)
	return true, next, nil
}
