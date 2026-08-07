package engine

import (
	"sync"
)

type ProcessContext struct {
	InstanceID string
	FlowID     string
	// Data 存储流程变量（用户输入、节点输出、中间结果）
	Data map[string]interface{}
	// NodeStates 存储流程中每个节点实例的当前状态
	NodeStates map[string]NodeState
	// Mu 保证并发安全
	Mu sync.Mutex
	// EngineRef 对 Engine 的引用，用于 SubprocessNode 回调
	EngineRef *FlowEngine

	// NodeAttempts 记录每个节点在本次运行中的尝试次数。
	NodeAttempts map[string]int

	// CurrentNode 表示当前正在执行的节点 ID。
	CurrentNode string

	// TriggerType 标记触发方式（如：API/Webhook/Cron）。
	TriggerType string

	// TriggerKey 记录触发来源标识（如 webhook 路径、Cron 名称）。
	TriggerKey string
}
