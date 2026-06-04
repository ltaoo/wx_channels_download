package pipeline

import "context"

type GatewayMode string

const (
	GatewayExclusive GatewayMode = "exclusive"
	GatewayParallel  GatewayMode = "parallel"
	GatewayMerge     GatewayMode = "merge"
)

type GatewayRule struct {
	Condition func(ctx context.Context, pc *Context) bool
	NextNodes []string
}

type GatewayNode struct {
	id string

	Mode GatewayMode

	Rules       []GatewayRule
	DefaultNext []string

	Dependencies []string
}

func NewGatewayNode(id string, mode GatewayMode) *GatewayNode {
	return &GatewayNode{id: id, Mode: mode}
}

func (n *GatewayNode) ID() string   { return n.id }
func (n *GatewayNode) Type() string { return "gateway" }

func (n *GatewayNode) Execute(ctx context.Context, pc *Context) ([]string, error) {
	switch n.Mode {
	case GatewayExclusive:
		for _, rule := range n.Rules {
			if rule.Condition != nil && rule.Condition(ctx, pc) {
				return rule.NextNodes, nil
			}
		}
		return n.DefaultNext, nil
	case GatewayParallel:
		return nil, nil
	case GatewayMerge:
		for _, depID := range n.Dependencies {
			if pc.GetNodeState(depID) != StateCompleted {
				pc.SetNodeState(n.id, StateWaiting)
				return nil, nil
			}
		}
		return nil, nil
	default:
		return nil, nil
	}
}
