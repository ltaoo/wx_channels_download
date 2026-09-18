package nodes

import (
	"fmt"
	"strings"

	"wx_channel/pkg/flowengine/engine"
)

// JSCodeNode executes a JavaScript snippet via goja.
//
// The three variable scopes are exposed to the script as `input`, `output` and
// `global` objects, matching the template/expression namespaces. The script's
// completion value is written to ctx.Data[output_key]; output_key is required,
// because an implicit multi-key merge would let a script silently overwrite
// unrelated context keys (e.g. an account `username` clobbered by a live-stream
// object).
type JSCodeNode struct {
	Id     string
	Config map[string]interface{}
}

func NewJSCodeNode(config map[string]interface{}) engine.Node {
	id, _ := config["id"].(string)
	return &JSCodeNode{Id: id, Config: config}
}

func (n *JSCodeNode) ID() string   { return n.Id }
func (n *JSCodeNode) Type() string { return "JSCodeNode" }

func (n *JSCodeNode) Execute(ctx *engine.ProcessContext) (bool, []string, error) {
	code, _ := n.Config["code"].(string)
	if code == "" {
		return false, nil, fmt.Errorf("JSCodeNode: code not provided")
	}
	outKey, _ := n.Config["output_key"].(string)
	outKey = strings.TrimSpace(outKey)
	if outKey == "" {
		return false, nil, fmt.Errorf("JSCodeNode: output_key is required")
	}

	vm, err := new_scope_vm(ctx.ScopeEnv())
	if err != nil {
		return false, nil, err
	}

	result, err := vm.RunString(code)
	if err != nil {
		return false, nil, err
	}

	ctx.Data[outKey] = result.Export()

	next := ctx.EngineRef.GetNextNodeIDsFromDefinition(ctx, n.Id)
	return true, next, nil
}
