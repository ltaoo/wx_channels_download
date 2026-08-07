package nodes

import "wx_channel/pkg/flowengine/engine"

type ManualNode struct {
	Id     string
	Config map[string]interface{}
}

func NewManualNode(config map[string]interface{}) engine.Node {
	id, _ := config["id"].(string)
	return &ManualNode{Id: id, Config: config}
}

func (n *ManualNode) ID() string   { return n.Id }
func (n *ManualNode) Type() string { return "ManualNode" }

func (n *ManualNode) Execute(ctx *engine.ProcessContext) (bool, []string, error) {
	// 1. 设置节点状态，流程暂停
	ctx.Mu.Lock()
	ctx.NodeStates[n.Id] = engine.StateWaitingForUser
	ctx.Mu.Unlock()

	// 2. 返回空列表，停止 driveFlow 递归
	return true, nil, nil
}
