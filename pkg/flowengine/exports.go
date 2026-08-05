package flowengine

import "wx_channel/pkg/flowengine/engine"

// Re-export engine types so external callers can use flowengine.X instead of engine.X.

// Node types
type (
	NodeDefinition = engine.NodeDefinition
	Node           = engine.Node
	NodeState      = engine.NodeState
	NodeError      = engine.NodeError
)

// Node state constants
const (
	StatePending              = engine.StatePending
	StateRunning              = engine.StateRunning
	StateCompleted            = engine.StateCompleted
	StateFailed               = engine.StateFailed
	StateRetrying             = engine.StateRetrying
	StateWaitingForUser       = engine.StateWaitingForUser
	StateWaitingForSubprocess = engine.StateWaitingForSubprocess
	StateWaitingForMerge      = engine.StateWaitingForMerge
)

// Flow types
type (
	FlowDefinition   = engine.FlowDefinition
	FlowEngine       = engine.FlowEngine
	ProcessContext   = engine.ProcessContext
	TargetNode       = engine.TargetNode
	FieldSchema      = engine.FieldSchema
	StartFlowOptions = engine.StartFlowOptions
)

// Run status types
type (
	RunStatus  = engine.RunStatus
	RunRecord  = engine.RunRecord
	RetryPolicy = engine.RetryPolicy
	TriggerInfo = engine.TriggerInfo
	TriggerType = engine.TriggerType
)

// Run status constants
const (
	RunStatusQueued    = engine.RunStatusQueued
	RunStatusRunning   = engine.RunStatusRunning
	RunStatusWaiting   = engine.RunStatusWaiting
	RunStatusCompleted = engine.RunStatusCompleted
	RunStatusFailed    = engine.RunStatusFailed
	RunStatusCancelled = engine.RunStatusCancelled
)

// Trigger type constants
const (
	TriggerTypeAPI     = engine.TriggerTypeAPI
	TriggerTypeWebhook = engine.TriggerTypeWebhook
	TriggerTypeCron    = engine.TriggerTypeCron
)
