package engine

import (
	"sync"
)

type ProcessContext struct {
	InstanceID string
	FlowID     string
	// Data 存储流程变量（用户输入、节点输出、中间结果）的扁平并集。
	Data map[string]interface{}
	// Inputs 记录触发输入 + 手动输入，供 {{input.*}} 模板解析。
	Inputs map[string]interface{}
	// Outputs 记录节点写出的输出键，供 {{output.*}} 模板解析。
	Outputs map[string]interface{}
	// Globals 记录节点显式写入的全局变量，供 {{global.*}} 模板解析。
	Globals map[string]interface{}
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

// SetInput writes an input value into both the flat union and the input
// projection. It is intentionally unlocked: driveFlow executes nodes on a
// single goroutine, and Mu is only used for state/persistence snapshots.
func (ctx *ProcessContext) SetInput(key string, value interface{}) {
	ctx.Data[key] = value
	if ctx.Inputs == nil {
		ctx.Inputs = map[string]interface{}{}
	}
	ctx.Inputs[key] = value
}

// SetOutput writes a node output into both the flat union and the output
// projection.
func (ctx *ProcessContext) SetOutput(key string, value interface{}) {
	ctx.Data[key] = value
	if ctx.Outputs == nil {
		ctx.Outputs = map[string]interface{}{}
	}
	ctx.Outputs[key] = value
}

// SetGlobal writes a global variable into both the flat union and the global
// projection.
func (ctx *ProcessContext) SetGlobal(key string, value interface{}) {
	ctx.Data[key] = value
	if ctx.Globals == nil {
		ctx.Globals = map[string]interface{}{}
	}
	ctx.Globals[key] = value
}
