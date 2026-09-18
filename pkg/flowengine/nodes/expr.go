package nodes

import (
	"wx_channel/pkg/flowengine/engine"

	"github.com/expr-lang/expr"
)

const default_expr_output_key = "calc_out"

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
	// expr.Env turns on strict name resolution: only input/output/global are
	// known top-level names, so a bare identifier or a typo is a compile error
	// instead of a silent nil.
	scope_env := ctx.ScopeEnv()
	program, err := expr.Compile(exprStr, expr.Env(scope_env))
	if err != nil {
		return false, nil, err
	}
	out, err := expr.Run(program, scope_env)
	if err != nil {
		return false, nil, err
	}
	outKey := default_expr_output_key
	if v, ok := n.Config["output_key"].(string); ok && v != "" {
		outKey = v
	}
	ctx.Data[outKey] = out
	next := ctx.EngineRef.GetNextNodeIDsFromDefinition(ctx, n.Id)
	return true, next, nil
}
