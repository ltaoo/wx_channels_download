package engine

import "fmt"

// NodeDefinition 对应 JSON 配置中 nodes 数组的每一项
type NodeDefinition struct {
	ID     string                 `json:"id"`
	Type   string                 `json:"type"`
	Name   string                 `json:"name"`
	Config map[string]interface{} `json:"config"`
	// NextNodes 定义了流程驱动到下一个节点的方式
	// 这是一个 NodeDefinition 特有的字段，用于非网关节点的执行逻辑。
	NextNodes         []TargetNode  `json:"next_nodes"`
	NextNodeIDs       []string      `json:"next_node_ids"`
	ErrorNextNodeID   string        `json:"error_next_node"`
	RetryPolicy       *RetryPolicy  `json:"retry"`
	InputSchema       []FieldSchema `json:"input_schema"`
	OutputSchema      []FieldSchema `json:"output_schema"`
}

// --- 流程状态常量 ---
type NodeState string

const (
	StatePending              NodeState = "PENDING"
	StateRunning              NodeState = "RUNNING"
	StateCompleted            NodeState = "COMPLETED"
	StateFailed               NodeState = "FAILED"
	StateRetrying             NodeState = "RETRYING"
	StateWaitingForUser       NodeState = "WAITING_FOR_USER"       // ManualNode 等待人工操作
	StateWaitingForSubprocess NodeState = "WAITING_FOR_SUBPROCESS" // SubprocessNode 等待子流程完成
	StateWaitingForMerge      NodeState = "WAITING_FOR_MERGE"      // Parallel/Inclusive 网关汇聚等待
)

// --- 1. 节点接口定义 ---
type Node interface {
	ID() string
	Type() string
	// Execute 是节点的核心执行方法
	// 返回：执行结果（如是否成功），和下一步要激活的节点ID列表
	Execute(ctx *ProcessContext) (bool, []string, error)
}

// FlowDefinition 对应整个 JSON 流程配置文件
type NodeError struct {
	NodeID   string
	NodeType string
	Err      error
}

func (e *NodeError) Error() string {
	return fmt.Sprintf("node %s (%s): %v", e.NodeID, e.NodeType, e.Err)
}
func (e *NodeError) Unwrap() error { return e.Err }
