package nodes

import (
	"fmt"

	"wx_channel/pkg/flowengine/engine"
)

// SetVariableNode writes explicit global variables into the process context.
// Config["variables"] is an object mapping variable names to values, where each
// value may be a static value or a {{namespace.key}} template resolved against
// the context. Values are written via SetGlobal so they are visible to later
// nodes as {{global.*}}.
type SetVariableNode struct {
	Id     string
	Config map[string]interface{}
}

func NewSetVariableNode(config map[string]interface{}) engine.Node {
	id, _ := config["id"].(string)
	return &SetVariableNode{Id: id, Config: config}
}

func (n *SetVariableNode) ID() string   { return n.Id }
func (n *SetVariableNode) Type() string { return "SetVariableNode" }

func (n *SetVariableNode) Execute(ctx *engine.ProcessContext) (bool, []string, error) {
	variables, _ := n.Config["variables"].(map[string]interface{})
	for name, value := range variables {
		resolved, err := resolve_service_template(value, ctx)
		if err != nil {
			return false, nil, fmt.Errorf("variable %s: %w", name, err)
		}
		ctx.SetGlobal(name, resolved)
	}
	next := ctx.EngineRef.GetNextNodeIDsFromDefinition(ctx, n.Id)
	return true, next, nil
}
