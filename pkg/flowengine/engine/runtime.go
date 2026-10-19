package engine

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

// RunCheckpoint is a complete, JSON-safe continuation at a node boundary.
// An in-flight checkpoint is diagnostic only: replaying side effects requires
// an explicit retry. Waiting checkpoints can be resumed without replay.
type RunCheckpoint struct {
	RunID        string                            `json:"run_id"`
	Definition   FlowDefinition                    `json:"definition"`
	Data         map[string]interface{}            `json:"data"`
	NodeOutputs  map[string]map[string]interface{} `json:"node_outputs"`
	Globals      map[string]interface{}            `json:"globals"`
	NodeStates   map[string]NodeState              `json:"node_states"`
	NodeAttempts map[string]int                    `json:"node_attempts"`
	CurrentNode  string                            `json:"current_node"`
	PendingNodes []string                          `json:"pending_nodes"`
	Steps        int64                             `json:"steps"`
	MaxSteps     int                               `json:"max_steps"`
	TriggerType  string                            `json:"trigger_type"`
	TriggerKey   string                            `json:"trigger_key"`
	Waiting      bool                              `json:"waiting"`
}

func (e *FlowEngine) SetCheckpointHandler(handler func(*RunCheckpoint) error) {
	e.Lock()
	defer e.Unlock()
	e.checkpoint_handler = handler
}

func (e *FlowEngine) GetFlowDefinition(flow_id string) (FlowDefinition, bool) {
	definition, ok := e.getFlowDefinition(flow_id)
	return clone_definition(definition), ok
}

func (e *FlowEngine) RegisterFlow(definition FlowDefinition) {
	e.Lock()
	defer e.Unlock()
	if e.FlowDefinitions == nil {
		e.FlowDefinitions = map[string]FlowDefinition{}
	}
	e.FlowDefinitions[definition.ID] = clone_definition(definition)
}

func (e *FlowEngine) RemoveFlow(flow_id string) {
	e.Lock()
	defer e.Unlock()
	delete(e.FlowDefinitions, flow_id)
}

func clone_definition(definition FlowDefinition) FlowDefinition {
	copied := definition
	copied.Nodes = make(map[string]NodeDefinition, len(definition.Nodes))
	copied.ContextSchema = append([]FieldSchema(nil), definition.ContextSchema...)
	for id, node := range definition.Nodes {
		node.Config = snapshot_map(node.Config)
		node.NextNodeIDs = append([]string(nil), node.NextNodeIDs...)
		node.NextNodes = append([]TargetNode(nil), node.NextNodes...)
		if node.RetryPolicy != nil {
			policy := *node.RetryPolicy
			node.RetryPolicy = &policy
		}
		copied.Nodes[id] = node
	}
	return copied
}

func (e *FlowEngine) definition_for(ctx *ProcessContext) (FlowDefinition, bool) {
	if ctx.Definition != nil {
		return *ctx.Definition, true
	}
	return e.getFlowDefinition(ctx.FlowID)
}

func (ctx *ProcessContext) Checkpoint() *RunCheckpoint {
	definition, _ := ctx.EngineRef.definition_for(ctx)
	ctx.Mu.Lock()
	defer ctx.Mu.Unlock()
	outputs := map[string]map[string]interface{}{}
	for id, values := range ctx.NodeOutputs {
		outputs[id] = snapshot_map(values)
	}
	steps := int64(0)
	if ctx.step_count != nil {
		steps = ctx.step_count.Load()
	}
	return &RunCheckpoint{RunID: ctx.InstanceID, Definition: clone_definition(definition),
		Data: snapshot_map(ctx.Data), NodeOutputs: outputs, Globals: snapshot_map(ctx.Globals),
		NodeStates: snapshotNodeStates(ctx.NodeStates), NodeAttempts: snapshotNodeAttempts(ctx.NodeAttempts),
		CurrentNode: ctx.CurrentNode, PendingNodes: append([]string(nil), ctx.PendingNodes...),
		Steps: steps, MaxSteps: ctx.MaxSteps, TriggerType: ctx.TriggerType, TriggerKey: ctx.TriggerKey,
		Waiting: ctx.NodeStates[ctx.CurrentNode] == StateWaitingForUser}
}

func (e *FlowEngine) save_checkpoint(ctx *ProcessContext) error {
	if ctx.checkpoint_disabled {
		return nil
	}
	e.RLock()
	handler := e.checkpoint_handler
	e.RUnlock()
	if handler != nil {
		if err := handler(ctx.Checkpoint()); err != nil {
			return fmt.Errorf("save workflow checkpoint: %w", err)
		}
	}
	return nil
}

func (e *FlowEngine) CancelRun(run_id string) {
	e.RLock()
	ctx := e.RunningContexts[run_id]
	e.RUnlock()
	if ctx != nil && ctx.cancel != nil {
		ctx.cancel()
		if nodeIsWaiting(ctx) {
			e.update_run_record(ctx, RunStatusCancelled, context.Canceled)
			e.Lock()
			delete(e.RunningContexts, run_id)
			e.Unlock()
		}
	}
}

func (e *FlowEngine) CancelAllRuns() {
	e.RLock()
	contexts := make([]*ProcessContext, 0, len(e.RunningContexts))
	for _, ctx := range e.RunningContexts {
		contexts = append(contexts, ctx)
	}
	e.RUnlock()
	for _, ctx := range contexts {
		if ctx.cancel != nil {
			ctx.cancel()
		}
	}
}

// ResumeCheckpoint accepts only a parked ManualNode and preserves the pending
// siblings, scopes, attempt counters and original definition across restarts.
func (e *FlowEngine) ResumeCheckpoint(parent context.Context, checkpoint *RunCheckpoint, node_id string, input map[string]interface{}) error {
	if err := e.ValidateManualInput(checkpoint, node_id, input); err != nil {
		return err
	}
	definition := clone_definition(checkpoint.Definition)
	node, ok := definition.Nodes[node_id]
	if !ok || node.Type != "ManualNode" {
		return errors.New("waiting node is not a manual task")
	}
	e.ensureMaps()
	if parent == nil {
		parent = context.Background()
	}
	run_context, cancel := context.WithCancel(parent)
	ctx := &ProcessContext{Context: run_context, cancel: cancel, Definition: &definition, ValidateSchemas: true,
		InstanceID: checkpoint.RunID, FlowID: definition.ID, Data: snapshot_map(checkpoint.Data),
		NodeOutputs: checkpoint.NodeOutputs, Globals: snapshot_map(checkpoint.Globals),
		NodeStates: snapshotNodeStates(checkpoint.NodeStates), NodeAttempts: snapshotNodeAttempts(checkpoint.NodeAttempts),
		CurrentNode: node_id, EngineRef: e, InputKeys: ContextSchemaKeys(definition.ContextSchema),
		TriggerType: checkpoint.TriggerType, TriggerKey: checkpoint.TriggerKey,
		MaxSteps: checkpoint.MaxSteps, step_count: &atomic.Int64{}}
	ctx.step_count.Store(checkpoint.Steps)
	if ctx.NodeOutputs == nil {
		ctx.NodeOutputs = map[string]map[string]interface{}{}
	}
	if err := e.validateData(node.InputSchema, input); err != nil {
		cancel()
		return err
	}
	output_key, _ := node.Config["output_key"].(string)
	if output_key == "" {
		output_key = "response"
	}
	ctx.SetOutput(node_id, output_key, snapshot_map(input))
	ctx.NodeStates[node_id] = StateCompleted
	next := append(append([]string(nil), node.NextNodeIDs...), checkpoint.PendingNodes...)
	ctx.PendingNodes = append([]string(nil), next...)
	e.Lock()
	e.RunningContexts[ctx.InstanceID] = ctx
	e.Unlock()
	e.RLock()
	semaphore := e.runSemaphore
	e.RUnlock()
	if semaphore != nil {
		select {
		case semaphore <- struct{}{}:
			defer func() { <-semaphore }()
		case <-run_context.Done():
			cancel()
			return run_context.Err()
		}
	}
	err := e.driveFlow(ctx, next)
	e.persistContext(ctx)
	status := RunStatusCompleted
	if err != nil {
		status = RunStatusFailed
		if errors.Is(err, context.Canceled) {
			status = RunStatusCancelled
		}
	}
	if err == nil && nodeIsWaiting(ctx) {
		status = RunStatusWaiting
	}
	e.update_run_record(ctx, status, err)
	if status != RunStatusWaiting {
		cancel()
		e.Lock()
		delete(e.RunningContexts, ctx.InstanceID)
		e.Unlock()
	}
	return err
}

func execute_node_safely(node Node, ctx *ProcessContext) (success bool, next_ids []string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			success = false
			err = fmt.Errorf("node panic: %v", recovered)
		}
	}()
	return node.Execute(ctx)
}

// ExecuteSubflow isolates child variables, shares cancellation and the total
// step budget, and uses the same driver/retry/audit behavior as parent nodes.
func (e *FlowEngine) ExecuteSubflow(parent *ProcessContext, definition FlowDefinition, input map[string]interface{}) (map[string]interface{}, error) {
	if parent.Depth >= 16 {
		return nil, errors.New("subflow nesting limit exceeded (16)")
	}
	definition = clone_definition(definition)
	if err := e.validateData(definition.ContextSchema, input); err != nil {
		return nil, err
	}
	parent.Mu.Lock()
	if parent.step_count == nil {
		parent.step_count = &atomic.Int64{}
	}
	parent.Mu.Unlock()
	child := &ProcessContext{Context: parent.ExecutionContext(), Definition: &definition, ValidateSchemas: parent.ValidateSchemas,
		InstanceID: parent.InstanceID, FlowID: definition.ID, EngineRef: e,
		Data: snapshot_map(input), NodeStates: map[string]NodeState{}, NodeAttempts: map[string]int{},
		NodeOutputs: map[string]map[string]interface{}{}, Globals: map[string]interface{}{},
		InputKeys: ContextSchemaKeys(definition.ContextSchema), MaxSteps: parent.MaxSteps,
		Depth: parent.Depth + 1, step_count: parent.step_count, checkpoint_disabled: true}
	if err := e.driveFlow(child, []string{definition.StartNodeID}); err != nil {
		return nil, err
	}
	if nodeIsWaiting(child) {
		return nil, errors.New("manual waits inside a subflow/iteration are not supported; place confirmation in the parent workflow")
	}
	result := map[string]interface{}{"data": child.SnapshotData(), "output": child.NodeOutputs, "global": child.Globals}
	return result, nil
}

// Parallel branches work on isolated snapshots. A configured join is executed
// once after all branches succeed; conflicting global writes fail explicitly.
func (e *FlowEngine) run_parallel(parent *ProcessContext, node NodeDefinition, starts []string) ([]string, error) {
	if parent.NodeOutputs == nil {
		parent.NodeOutputs = map[string]map[string]interface{}{}
	}
	join_id, _ := node.Config["join_node_id"].(string)
	definition, _ := e.definition_for(parent)
	if join_id != "" {
		if _, ok := definition.Nodes[join_id]; !ok {
			return nil, fmt.Errorf("parallel join node not found: %s", join_id)
		}
	}
	if len(starts) > 32 {
		return nil, errors.New("parallel branches exceed limit (32)")
	}
	visited_by := map[string]int{}
	for branch_index, start := range starts {
		queue := []string{start}
		local_seen := map[string]bool{}
		for len(queue) > 0 {
			id := queue[0]
			queue = queue[1:]
			if id == join_id || local_seen[id] {
				continue
			}
			local_seen[id] = true
			if owner, exists := visited_by[id]; exists && owner != branch_index {
				return nil, fmt.Errorf("parallel branches share node %s before join; configure a single join_node_id", id)
			}
			visited_by[id] = branch_index
			child_node, ok := definition.Nodes[id]
			if !ok {
				return nil, fmt.Errorf("parallel node not found: %s", id)
			}
			queue = append(queue, child_node.NextNodeIDs...)
			if child_node.ErrorNextNodeID != "" {
				queue = append(queue, child_node.ErrorNextNodeID)
			}
		}
	}
	run_context, cancel := context.WithCancel(parent.ExecutionContext())
	defer cancel()
	children := make([]*ProcessContext, len(starts))
	failures := make([]error, len(starts))
	var wg sync.WaitGroup
	for index, start := range starts {
		child := &ProcessContext{Context: run_context, Definition: parent.Definition, ValidateSchemas: parent.ValidateSchemas, InstanceID: parent.InstanceID,
			FlowID: parent.FlowID, EngineRef: e, Data: parent.SnapshotData(),
			NodeStates: snapshotNodeStates(parent.NodeStates), NodeAttempts: snapshotNodeAttempts(parent.NodeAttempts),
			NodeOutputs: map[string]map[string]interface{}{}, Globals: snapshot_map(parent.Globals),
			InputKeys: parent.InputKeys, MaxSteps: parent.MaxSteps, Depth: parent.Depth,
			step_count: parent.step_count, checkpoint_disabled: true, stop_node: join_id}
		for id, values := range parent.NodeOutputs {
			child.NodeOutputs[id] = snapshot_map(values)
		}
		children[index] = child
		wg.Add(1)
		go func(index int, start string, child *ProcessContext) {
			defer wg.Done()
			failures[index] = e.driveFlow(child, []string{start})
			if failures[index] == nil && join_id != "" && !child.reached_stop {
				failures[index] = fmt.Errorf("parallel branch %s did not reach join %s", start, join_id)
			}
			if failures[index] == nil && nodeIsWaiting(child) {
				failures[index] = errors.New("manual waits inside parallel branches are not supported; confirm after the join")
			}
			if failures[index] != nil {
				cancel()
			}
		}(index, start, child)
	}
	wg.Wait()
	for _, err := range failures {
		if err != nil && !errors.Is(err, context.Canceled) {
			return nil, err
		}
	}
	for _, err := range failures {
		if err != nil {
			return nil, err
		}
	}
	global_changes := map[string]interface{}{}
	for _, child := range children {
		for key, value := range changed_context_values(parent.Globals, child.Globals) {
			if previous, exists := global_changes[key]; exists && !reflect.DeepEqual(previous, value) {
				return nil, fmt.Errorf("parallel branches write conflicting global variable: %s", key)
			}
			global_changes[key] = value
		}
	}
	for _, child := range children {
		for id, values := range child.NodeOutputs {
			if !reflect.DeepEqual(parent.NodeOutputs[id], values) {
				parent.NodeOutputs[id] = snapshot_map(values)
			}
		}
		for id, state := range child.NodeStates {
			parent.NodeStates[id] = state
		}
		for id, attempts := range child.NodeAttempts {
			if attempts > parent.NodeAttempts[id] {
				parent.NodeAttempts[id] = attempts
			}
		}
		for key, value := range changed_context_values(parent.Data, child.Data) {
			parent.Data[key] = value
		}
	}
	for key, value := range global_changes {
		parent.SetGlobal(key, value)
	}
	if join_id != "" {
		return []string{join_id}, nil
	}
	return nil, nil
}

// SleepContext lets wait/loop nodes stop promptly on workflow cancellation.
func SleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (e *FlowEngine) ValidateManualInput(checkpoint *RunCheckpoint, node_id string, input map[string]interface{}) error {
	if checkpoint == nil || !checkpoint.Waiting || checkpoint.CurrentNode != node_id || checkpoint.NodeStates[node_id] != StateWaitingForUser {
		return errors.New("run is not waiting at this node")
	}
	node, ok := checkpoint.Definition.Nodes[node_id]
	if !ok || node.Type != "ManualNode" {
		return errors.New("waiting node is not a manual task")
	}
	return e.validateData(node.InputSchema, input)
}

// ExecuteSharedSubflow retains compatibility with built-in Go post-processors.
func (e *FlowEngine) ExecuteSharedSubflow(parent *ProcessContext, definition FlowDefinition) error {
	if parent.Depth >= 16 {
		return errors.New("subflow nesting limit exceeded (16)")
	}
	if parent.NodeOutputs == nil {
		parent.NodeOutputs = map[string]map[string]interface{}{}
	}
	if parent.Globals == nil {
		parent.Globals = map[string]interface{}{}
	}
	if parent.step_count == nil {
		parent.step_count = &atomic.Int64{}
	}
	child := &ProcessContext{Context: parent.ExecutionContext(), Definition: &definition, ValidateSchemas: parent.ValidateSchemas,
		InstanceID: parent.InstanceID, FlowID: definition.ID, EngineRef: e, Data: parent.Data,
		NodeStates: map[string]NodeState{}, NodeAttempts: map[string]int{}, NodeOutputs: parent.NodeOutputs,
		Globals: parent.Globals, InputKeys: ContextSchemaKeys(definition.ContextSchema),
		MaxSteps: parent.MaxSteps, Depth: parent.Depth + 1, step_count: parent.step_count, checkpoint_disabled: true}
	if err := e.driveFlow(child, []string{definition.StartNodeID}); err != nil {
		return err
	}
	if nodeIsWaiting(child) {
		return errors.New("manual wait inside a built-in child flow is unsupported")
	}
	return nil
}
