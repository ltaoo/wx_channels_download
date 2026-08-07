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
	RunStatusQueued     RunStatus = "QUEUED"
	RunStatusRunning    RunStatus = "RUNNING"
	RunStatusWaiting    RunStatus = "WAITING"
	RunStatusCompleted  RunStatus = "COMPLETED"
	RunStatusFailed     RunStatus = "FAILED"
	RunStatusCancelled  RunStatus = "CANCELLED"
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
	RunID          string            `json:"run_id"`
	FlowID         string            `json:"flow_id"`
	Status         RunStatus         `json:"status"`
	CurrentNode    string            `json:"current_node"`
	StartedAt      time.Time         `json:"started_at"`
	CompletedAt    *time.Time        `json:"completed_at,omitempty"`
	Error          string            `json:"error,omitempty"`
	Trigger        TriggerInfo       `json:"trigger"`
	NodeAttempts   map[string]int    `json:"node_attempts"`
	NodeOutputs    map[string]any     `json:"node_outputs,omitempty"`
	LastUpdatedAt  time.Time         `json:"last_updated_at"`
}

type FieldSchema struct {
	Key      string `json:"key"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
	Fields   []FieldSchema `json:"fields,omitempty"`
}
