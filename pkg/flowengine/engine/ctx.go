package engine

import (
	"sync"
)

// ProcessContext carries the runtime state of one flow run. Variable reads are
// split into three disjoint scopes, mirroring the n8n model:
//
//   - input.<key>            — a run parameter declared by the flow's
//     ContextSchema; it is a live projection of Data
//     (see ScopeEnv).
//   - output.<node_id>.<key> — a key produced by a specific upstream node;
//     reads NodeOutputs, so two nodes emitting the same
//     key never collide.
//   - global.<key>           — a variable written explicitly by a
//     SetVariableNode; reads Globals.
//
// The same key name may live in all three scopes without interfering.
type ProcessContext struct {
	InstanceID string
	FlowID     string
	// Data is the flat union of everything written into the context at
	// runtime. It is the storage behind the scopes, not a read surface:
	// templates and expressions address input/output/global explicitly.
	Data map[string]interface{}
	// NodeOutputs indexes produced keys by the node that produced them:
	// NodeOutputs[node_id][key] is the value node_id wrote for key. It is
	// the storage behind the output.<node_id>.<key> scope, and it is what
	// keeps identically named keys from different producers apart.
	NodeOutputs map[string]map[string]interface{}
	// Globals records variables written explicitly by SetVariableNode. It is
	// the storage behind the global.<key> scope and is scoped to a single run.
	Globals map[string]interface{}
	// InputKeys lists the flow's ContextSchema keys. Only the keys are kept:
	// input.<key> resolves live from Data, so an upstream in-place write is
	// reflected immediately instead of leaving a stale frozen snapshot.
	InputKeys []string
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

	// ExecutionDetails carries node-provided extras for the current attempt's
	// audit entry (for example a ServiceNode's resolved arguments). driveFlow
	// clears it before each attempt and merges it into
	// NodeExecutionLog.Behavior afterwards.
	ExecutionDetails map[string]interface{}
}

// SetExecutionDetail records an audit extra for the current node attempt. Like
// the context writes done by nodes it is intentionally unlocked: driveFlow
// executes nodes on a single goroutine, and Mu is only used for
// state/persistence snapshots.
func (ctx *ProcessContext) SetExecutionDetail(key string, value interface{}) {
	if ctx.ExecutionDetails == nil {
		ctx.ExecutionDetails = map[string]interface{}{}
	}
	ctx.ExecutionDetails[key] = value
}

// ScopeEnv builds the read environment exposed to templates and expressions:
// exactly the three namespaces input / output / global. A bare identifier (or
// a ctx.* path) is therefore not resolvable, which is what makes typos a hard
// error instead of a silent nil.
func (ctx *ProcessContext) ScopeEnv() map[string]any {
	input := map[string]any{}
	for _, key := range ctx.InputKeys {
		if value, ok := ctx.Data[key]; ok {
			input[key] = value
		}
	}
	return map[string]any{
		"input":  input,
		"output": ctx.NodeOutputs,
		"global": ctx.Globals,
	}
}

// SnapshotData returns a deep copy of the flat Data union.
func (ctx *ProcessContext) SnapshotData() map[string]interface{} {
	if ctx == nil {
		return map[string]interface{}{}
	}
	ctx.Mu.Lock()
	defer ctx.Mu.Unlock()
	return snapshot_map(ctx.Data)
}

// ChangedContextValues returns the entries of after that are new or different
// compared to before. It is the same before/after diff driveFlow uses to index
// produced keys, exposed for alternate drivers such as sub-flows.
func ChangedContextValues(before map[string]interface{}, after map[string]interface{}) map[string]interface{} {
	return changed_context_values(before, after)
}

// RecordNodeOutputs indexes the keys a node produced under its node id so
// downstream nodes can address them as output.<node_id>.<key>. changed is the
// before/after diff of the flat Data union for one attempt; retries accumulate
// into the same producer entry.
func (ctx *ProcessContext) RecordNodeOutputs(node_id string, changed map[string]interface{}) {
	if ctx == nil || node_id == "" || len(changed) == 0 {
		return
	}
	ctx.Mu.Lock()
	defer ctx.Mu.Unlock()
	if ctx.NodeOutputs == nil {
		ctx.NodeOutputs = map[string]map[string]interface{}{}
	}
	if ctx.NodeOutputs[node_id] == nil {
		ctx.NodeOutputs[node_id] = map[string]interface{}{}
	}
	for key, value := range changed {
		ctx.NodeOutputs[node_id][key] = value
	}
}

// SetGlobal writes an explicit global variable. Globals is the only writer of
// the global.<key> scope; the flat Data union is kept in sync so persistence
// and audit logs still see the value.
func (ctx *ProcessContext) SetGlobal(key string, value interface{}) {
	ctx.Data[key] = value
	if ctx.Globals == nil {
		ctx.Globals = map[string]interface{}{}
	}
	ctx.Globals[key] = value
}
