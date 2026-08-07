package nodes

import (
	"wx_channel/pkg/flowengine/engine"

	"github.com/expr-lang/expr"
)

type ExprNode struct {
	Id     string
	Config map[string]interface{}
}

func NewExprNode(config map[string]interface{}) engine.Node {
	id, _ := config["id"].(string)
	return &ExprNode{Id: id, Config: config}
}

func (n *ExprNode) ID() string   { return n.Id }
func (n *ExprNode) Type() string { return "ValueCalcNode" }

func (n *ExprNode) Execute(ctx *engine.ProcessContext) (bool, []string, error) {
	exprStr, _ := n.Config["expression"].(string)
	program, err := expr.Compile(exprStr)
	if err != nil {
		return false, nil, err
	}
	out, err := expr.Run(program, ctx.Data)
	if err != nil {
		return false, nil, err
	}
	outKey := "calc_out"
	if v, ok := n.Config["output_key"].(string); ok && v != "" {
		outKey = v
	}
	ctx.Data[outKey] = out
	next := ctx.EngineRef.GetNextNodeIDsFromDefinition(ctx, n.Id)
	return true, next, nil
}
