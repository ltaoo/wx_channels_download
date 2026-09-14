package nodes

import (
	"fmt"

	"wx_channel/pkg/flowengine/engine"

	"github.com/dop251/goja"
)

// JSCodeNode executes a JavaScript snippet via goja.
//
// The full process context is exposed to the script as a `data` object
// (a snapshot of ctx.Data). The script's completion value is written back:
//   - if `output_key` is set, the value is written to ctx.Data[output_key];
//   - otherwise, if the value is a plain object, its entries are merged into
//     ctx.Data, letting one node emit multiple fields at once.
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

	vm := goja.New()
	if err := vm.Set("data", ctx.Data); err != nil {
		return false, nil, err
	}

	result, err := vm.RunString(code)
	if err != nil {
		return false, nil, err
	}

	if outKey, _ := n.Config["output_key"].(string); outKey != "" {
		ctx.Data[outKey] = result.Export()
	} else if obj, ok := result.(*goja.Object); ok {
		if m, ok := obj.Export().(map[string]interface{}); ok {
			for k, v := range m {
				ctx.Data[k] = v
			}
		}
	}

	next := ctx.EngineRef.GetNextNodeIDsFromDefinition(ctx, n.Id)
	return true, next, nil
}
