package engine

import "time"

// TargetNode 定义了一个指向目标节点的边
type TargetNode struct {
	TargetID string `json:"target_id"`
}

// TriggerType 表示触发方式。
type TriggerType string

const (
	// TriggerTypeAPI 表示由内部 API 调用触发。
	TriggerTypeAPI TriggerType = "API"
	// TriggerTypeWebhook 表示由 webhook 入站调用触发。
	TriggerTypeWebhook TriggerType = "Webhook"
	// TriggerTypeCron 表示由定时触发器触发。
	TriggerTypeCron TriggerType = "Cron"
)

type RunStatus string

const (
	RunStatusQueued    RunStatus = "QUEUED"
	RunStatusRunning   RunStatus = "RUNNING"
	RunStatusWaiting   RunStatus = "WAITING"
	RunStatusCompleted RunStatus = "COMPLETED"
	RunStatusFailed    RunStatus = "FAILED"
	RunStatusCancelled RunStatus = "CANCELLED"
)

// RetryPolicy 定义单节点重试参数。
type RetryPolicy struct {
	MaxAttempts       int     `json:"max_attempts"`
	DelayMs           int     `json:"delay_ms"`
	BackoffMultiplier float64 `json:"backoff_multiplier"`
}

// TriggerInfo 记录单次运行的触发元数据。
type TriggerInfo struct {
	Type      TriggerType `json:"type"`
	Key       string      `json:"key,omitempty"`
	Source    string      `json:"source,omitempty"`
	StartedAt string      `json:"started_at"`
}

// RunRecord 持久化运行记录（含历史与审计）。
type RunRecord struct {
	RunID         string         `json:"run_id"`
	FlowID        string         `json:"flow_id"`
	Status        RunStatus      `json:"status"`
	CurrentNode   string         `json:"current_node"`
	StartedAt     time.Time      `json:"started_at"`
	CompletedAt   *time.Time     `json:"completed_at,omitempty"`
	Error         string         `json:"error,omitempty"`
	Trigger       TriggerInfo    `json:"trigger"`
	NodeAttempts  map[string]int `json:"node_attempts"`
	NodeOutputs   map[string]any `json:"node_outputs,omitempty"`
	LastUpdatedAt time.Time      `json:"last_updated_at"`
}

// NodeExecutionLog records one execution attempt at the common node boundary.
// Input is the context visible to the node before Execute, Behavior describes
// the node configuration/routing, and Output contains only context values that
// were added or changed by that attempt.
type NodeExecutionLog struct {
	Timestamp         time.Time              `json:"timestamp"`
	FlowID            string                 `json:"flow_id"`
	RunID             string                 `json:"run_id"`
	NodeID            string                 `json:"node_id"`
	NodeName          string                 `json:"node_name,omitempty"`
	NodeType          string                 `json:"node_type"`
	Attempt           int                    `json:"attempt"`
	Outcome           string                 `json:"outcome"`
	DurationMs        int64                  `json:"duration_ms"`
	Input             map[string]interface{} `json:"input"`
	Behavior          map[string]interface{} `json:"behavior"`
	Output            map[string]interface{} `json:"output"`
	RemovedOutputKeys []string               `json:"removed_output_keys,omitempty"`
	Success           bool                   `json:"success"`
	NextNodeIDs       []string               `json:"next_node_ids,omitempty"`
	Error             string                 `json:"error,omitempty"`
}

// NodeExecutionLogHandler receives node execution audit entries. The engine
// deliberately does not depend on a logging implementation.
type NodeExecutionLogHandler func(NodeExecutionLog)

// NodeExecutionStatus is a lightweight live update emitted before and after a
// node attempt. Detailed input, behavior, and output stay in NodeExecutionLog.
type NodeExecutionStatus struct {
	Timestamp time.Time `json:"timestamp"`
	FlowID    string    `json:"flow_id"`
	RunID     string    `json:"run_id"`
	NodeID    string    `json:"node_id"`
	NodeName  string    `json:"node_name,omitempty"`
	NodeType  string    `json:"node_type"`
	Attempt   int       `json:"attempt"`
	Status    NodeState `json:"status"`
	Error     string    `json:"error,omitempty"`
}

type NodeExecutionStatusHandler func(NodeExecutionStatus)

type FieldSchema struct {
	Key      string        `json:"key"`
	Type     string        `json:"type"`
	Required bool          `json:"required"`
	Fields   []FieldSchema `json:"fields,omitempty"`
}
