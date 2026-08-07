package nodes

import (
	"errors"
	"wx_channel/pkg/flowengine/engine"

	"github.com/expr-lang/expr"
)

type GatewayNode struct {
	Id     string
	Config map[string]interface{}
}

func NewGatewayNode(config map[string]interface{}) engine.Node {
	id, _ := config["id"].(string)
	return &GatewayNode{Id: id, Config: config}
}

func (n *GatewayNode) ID() string   { return n.Id }
func (n *GatewayNode) Type() string { return "GatewayNode" }

func (n *GatewayNode) Execute(ctx *engine.ProcessContext) (bool, []string, error) {
	gatewayType, _ := n.Config["gateway_type"].(string)

	if n.Config["is_joining"].(bool) {
		// --- 汇聚逻辑 (Joining) ---
		return n.handleMerge(ctx)
	}

	// --- 分支逻辑 (Splitting) ---
	switch gatewayType {
	case "Parallel":
		// 直接返回所有后续节点ID，并行启动
		if nextNodeIDs, ok := n.Config["next_node_ids"].([]string); ok {
			return true, nextNodeIDs, nil
		}
		if targets, ok := n.Config["next_nodes"].([]engine.TargetNode); ok {
			ids := make([]string, 0, len(targets))
			for _, t := range targets {
				ids = append(ids, t.TargetID)
			}
			return true, ids, nil
		}

	case "Exclusive":
		// 遍历 rules，找到第一个条件满足的 target_id 并返回
		for _, rule := range n.Config["rules"].([]map[string]interface{}) {
			condition := rule["condition"].(string)
			ok, err := n.evaluateCondition(ctx, condition)
			if err != nil {
				return false, nil, err
			}
			if ok { // 假设表达式求值器
				return true, []string{rule["target_id"].(string)}, nil
			}
		}
		return false, nil, errors.New("Exclusive Gateway failed to find a valid path")

		// case "Inclusive":
		// 遍历 rules，收集所有满足条件的 target_id 并返回
		// ...
	}
	return false, nil, errors.New("GatewayNode: unknown gateway_type")
}

// handleMerge 处理汇聚逻辑
func (n *GatewayNode) handleMerge(ctx *engine.ProcessContext) (bool, []string, error) {
	waitList := n.Config["wait_for_incoming"].([]string)

	// 检查所有依赖节点是否都已完成
	allCompleted := true
	for _, depID := range waitList {
		if ctx.NodeStates[depID] != engine.StateCompleted {
			allCompleted = false
			break
		}
	}

	if !allCompleted {
		// 依赖未满足，设置状态并暂停
		ctx.Mu.Lock()
		ctx.NodeStates[n.ID()] = engine.StateWaitingForMerge
		ctx.Mu.Unlock()
		return true, nil, nil // 返回 nil 暂停流程
	}

	// 所有依赖完成，继续流程
	if nextNodeIds, ok := n.Config["next_node_ids"].([]string); ok {
		return true, nextNodeIds, nil
	}
	if targets, ok := n.Config["next_nodes"].([]engine.TargetNode); ok {
		ids := make([]string, 0, len(targets))
		for _, t := range targets {
			ids = append(ids, t.TargetID)
		}
		return true, ids, nil
	}
	return true, nil, nil
}

// evaluateCondition 负责解析和执行条件表达式
func (n *GatewayNode) evaluateCondition(ctx *engine.ProcessContext, condition string) (bool, error) {
	program, err := expr.Compile(condition)
	if err != nil {
		return false, err
	}
	out, err := expr.Run(program, ctx.Data)
	if err != nil {
		return false, err
	}
	b, ok := out.(bool)
	if !ok {
		return false, nil
	}
	return b, nil
}
