package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	default_run_history_limit        = 256
	run_history_compaction_threshold = 256
)

type StartFlowOptions struct {
	Trigger TriggerInfo
	Async   bool
}

type FlowDefinition struct {
	ID            string                    `json:"id"`
	Name          string                    `json:"name"`
	StartNodeID   string                    `json:"start_node"`
	ContextSchema []FieldSchema             `json:"context_schema"`
	Nodes         map[string]NodeDefinition `json:"nodes"`
}

type flowInstanceSnapshot struct {
	Data       map[string]interface{}
	NodeStates map[string]NodeState
	UpdatedAt  time.Time
}

type FlowEngine struct {
	FlowDefinitions map[string]FlowDefinition
	RunningContexts map[string]*ProcessContext
	NodeRegistry    map[string]func(config map[string]interface{}) Node

	// RunHistory 记录执行历史，默认内存实现；可替换为数据库持久化。
	RunHistory map[string]*RunRecord
	// ContextSnapshots 持久化运行上下文，默认内存实现。
	ContextSnapshots map[string]flowInstanceSnapshot
	// RunHistoryLimit limits retained terminal runs and their context snapshots.
	// Zero uses the default limit; negative values retain all terminal runs.
	RunHistoryLimit    int
	completed_run_ids  []string
	completed_run_head int

	// Semaphore for limiting并发执行量。0 表示不限制。
	ConcurrencyLimit int
	runSemaphore     chan struct{}

	cronMu   sync.RWMutex
	cronJobs map[string]*struct {
		Stop chan struct{}
	}

	sync.RWMutex
}

func (e *FlowEngine) ensureMaps() {
	e.Lock()
	defer e.Unlock()
	if e.FlowDefinitions == nil {
		e.FlowDefinitions = map[string]FlowDefinition{}
	}
	if e.RunningContexts == nil {
		e.RunningContexts = map[string]*ProcessContext{}
	}
	if e.NodeRegistry == nil {
		e.NodeRegistry = map[string]func(map[string]interface{}) Node{}
	}
	if e.RunHistory == nil {
		e.RunHistory = map[string]*RunRecord{}
	}
	if e.ContextSnapshots == nil {
		e.ContextSnapshots = map[string]flowInstanceSnapshot{}
	}
	if e.cronJobs == nil {
		e.cronJobs = map[string]*struct {
			Stop chan struct{}
		}{}
	}
	if e.ConcurrencyLimit > 0 {
		if e.runSemaphore == nil || cap(e.runSemaphore) != e.ConcurrencyLimit {
			e.runSemaphore = make(chan struct{}, e.ConcurrencyLimit)
		}
	} else {
		e.runSemaphore = nil
	}
}

func (e *FlowEngine) SetConcurrencyLimit(limit int) {
	e.Lock()
	defer e.Unlock()
	if limit < 0 {
		limit = 0
	}
	e.ConcurrencyLimit = limit
	if limit == 0 {
		e.runSemaphore = nil
		return
	}
	if e.runSemaphore == nil || cap(e.runSemaphore) != limit {
		e.runSemaphore = make(chan struct{}, limit)
	}
}

func (e *FlowEngine) RegisterNode(type_name string, constructor func(map[string]interface{}) Node) {
	e.Lock()
	defer e.Unlock()
	if e.NodeRegistry == nil {
		e.NodeRegistry = map[string]func(map[string]interface{}) Node{}
	}
	e.NodeRegistry[type_name] = constructor
}

func (e *FlowEngine) SetFlowDefinitions(nodes map[string]FlowDefinition) {
	e.Lock()
	defer e.Unlock()
	e.FlowDefinitions = nodes
}

func (e *FlowEngine) StartFlow(flow_id string, initial_data map[string]interface{}) (string, error) {
	return e.StartFlowWithOptions(flow_id, initial_data, StartFlowOptions{})
}

func (e *FlowEngine) ListRunRecords(flow_id string, limit int) []RunRecord {
	e.RLock()
	defer e.RUnlock()

	if limit <= 0 {
		limit = len(e.RunHistory)
	}

	records := make([]RunRecord, 0, len(e.RunHistory))
	for _, record := range e.RunHistory {
		if record == nil {
			continue
		}
		if flow_id != "" && record.FlowID != flow_id {
			continue
		}

		copied := *record
		copied.NodeAttempts = snapshotNodeAttempts(record.NodeAttempts)
		copied.NodeOutputs = snapshot_map(record.NodeOutputs)
		records = append(records, copied)
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].LastUpdatedAt.After(records[j].LastUpdatedAt)
	})
	if len(records) > limit {
		records = records[:limit]
	}
	return records
}

func (e *FlowEngine) StartFlowWithOptions(flow_id string, initial_data map[string]interface{}, options StartFlowOptions) (string, error) {
	e.ensureMaps()
	if initial_data == nil {
		initial_data = map[string]interface{}{}
	}

	ref_flow, ok := e.getFlowDefinition(flow_id)
	if !ok {
		return "", errors.New("flow not found")
	}

	trigger := options.Trigger
	if trigger.Type == "" {
		trigger.Type = TriggerTypeAPI
	}
	if trigger.StartedAt == "" {
		trigger.StartedAt = time.Now().Format(time.RFC3339Nano)
	}

	instanceID := fmt.Sprintf("instance-%d", time.Now().UnixNano())
	ctx := &ProcessContext{
		InstanceID:   instanceID,
		FlowID:       flow_id,
		Data:         map[string]interface{}{},
		NodeStates:   map[string]NodeState{},
		NodeAttempts: map[string]int{},
		EngineRef:    e,
		TriggerType:  string(trigger.Type),
		TriggerKey:   trigger.Key,
	}
	for key, value := range initial_data {
		ctx.Data[key] = value
	}

	e.Lock()
	e.RunningContexts[instanceID] = ctx
	e.RunHistory[instanceID] = &RunRecord{
		RunID:         instanceID,
		FlowID:        flow_id,
		Status:        RunStatusQueued,
		StartedAt:     time.Now(),
		Trigger:       trigger,
		NodeAttempts:  map[string]int{},
		LastUpdatedAt: time.Now(),
	}
	e.Unlock()
	e.persistContext(ctx)
	e.update_run_record(ctx, RunStatusRunning, nil)

	runWithSemaphore := func() bool {
		e.Lock()
		sem := e.runSemaphore
		e.Unlock()
		if sem == nil {
			return false
		}
		sem <- struct{}{}
		return true
	}
	releaseSemaphore := func() {
		e.Lock()
		sem := e.runSemaphore
		e.Unlock()
		if sem != nil {
			select {
			case <-sem:
			default:
			}
		}
	}

	exec := func() error {
		err := e.driveFlow(ctx, []string{ref_flow.StartNodeID})
		e.persistContext(ctx)
		if err != nil {
			e.update_run_record(ctx, RunStatusFailed, err)
			e.Lock()
			delete(e.RunningContexts, instanceID)
			e.Unlock()
			return err
		}
		if nodeIsWaiting(ctx) {
			e.update_run_record(ctx, RunStatusWaiting, nil)
			return nil
		}
		e.Lock()
		delete(e.RunningContexts, instanceID)
		e.Unlock()
		e.update_run_record(ctx, RunStatusCompleted, nil)
		return nil
	}

	if options.Async {
		acquired := runWithSemaphore()
		go func() {
			defer func() {
				if acquired {
					releaseSemaphore()
				}
			}()
			_ = exec()
		}()
		return instanceID, nil
	}

	acquired := runWithSemaphore()
	err := exec()
	if acquired {
		releaseSemaphore()
	}
	if err != nil {
		return instanceID, err
	}
	if nodeIsWaiting(ctx) {
		return instanceID, nil
	}
	return instanceID, nil
}

func (e *FlowEngine) StartFlowByWebhook(flow_id string, route string, initial_data map[string]interface{}) (string, error) {
	return e.StartFlowWithOptions(flow_id, initial_data, StartFlowOptions{
		Trigger: TriggerInfo{Type: TriggerTypeWebhook, Key: route},
	})
}

func (e *FlowEngine) StartFlowByAPI(flow_id string, caller string, initial_data map[string]interface{}) (string, error) {
	return e.StartFlowWithOptions(flow_id, initial_data, StartFlowOptions{
		Trigger: TriggerInfo{Type: TriggerTypeAPI, Key: caller},
	})
}

func (e *FlowEngine) ScheduleCron(flow_id string, expression string, initial_data map[string]interface{}, trigger TriggerInfo) (string, error) {
	e.ensureMaps()
	interval, err := parseCronLikeDuration(expression)
	if err != nil {
		return "", err
	}
	if trigger.Type == "" {
		trigger.Type = TriggerTypeCron
	}
	if trigger.Key == "" {
		trigger.Key = expression
	}
	if initial_data == nil {
		initial_data = map[string]interface{}{}
	}

	jobID := fmt.Sprintf("cron-%d", time.Now().UnixNano())
	stop := make(chan struct{})
	e.cronMu.Lock()
	e.cronJobs[jobID] = &struct {
		Stop chan struct{}
	}{Stop: stop}
	e.cronMu.Unlock()

	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				_, _ = e.StartFlowWithOptions(flow_id, clone_string_map(initial_data), StartFlowOptions{
					Async: true,
					Trigger: TriggerInfo{
						Type:   trigger.Type,
						Key:    trigger.Key,
						Source: trigger.Source,
					},
				})
			case <-stop:
				ticker.Stop()
				return
			}
		}
	}()
	return jobID, nil
}

func (e *FlowEngine) StopCron(job_id string) bool {
	e.cronMu.Lock()
	defer e.cronMu.Unlock()
	job, ok := e.cronJobs[job_id]
	if !ok {
		return false
	}
	delete(e.cronJobs, job_id)
	close(job.Stop)
	return true
}

func (e *FlowEngine) GetRunSnapshot(instance_id string) (*RunRecord, bool) {
	e.RLock()
	defer e.RUnlock()
	record, ok := e.RunHistory[instance_id]
	if !ok || record == nil {
		return nil, false
	}
	copied := *record
	if record.NodeAttempts != nil {
		copied.NodeAttempts = map[string]int{}
		for k, v := range record.NodeAttempts {
			copied.NodeAttempts[k] = v
		}
	}
	if record.NodeOutputs != nil {
		copied.NodeOutputs = snapshot_map(record.NodeOutputs)
	}
	return &copied, true
}

func (e *FlowEngine) GetRunContext(instance_id string) (map[string]interface{}, map[string]NodeState) {
	e.RLock()
	defer e.RUnlock()
	if snapshot, ok := e.ContextSnapshots[instance_id]; ok {
		return snapshot_map(snapshot.Data), snapshotNodeStates(snapshot.NodeStates)
	}
	if ctx, ok := e.RunningContexts[instance_id]; ok && ctx != nil {
		ctx.Mu.Lock()
		defer ctx.Mu.Unlock()
		return snapshot_map(ctx.Data), snapshotNodeStates(ctx.NodeStates)
	}
	return nil, nil
}

func (e *FlowEngine) getFlowDefinition(flow_id string) (FlowDefinition, bool) {
	e.RLock()
	defer e.RUnlock()
	ref_flow, ok := e.FlowDefinitions[flow_id]
	return ref_flow, ok
}

func (e *FlowEngine) driveFlow(ctx *ProcessContext, node_ids []string) error {
	for _, node_id := range node_ids {
		flowDef, ok := e.getFlowDefinition(ctx.FlowID)
		if !ok {
			return fmt.Errorf("flow not found: %s", ctx.FlowID)
		}
		nodeDef, ok := flowDef.Nodes[node_id]
		if !ok {
			return fmt.Errorf("node not found: %s", node_id)
		}
		nodeImpl := e.createNodeImpl(nodeDef)
		if nodeImpl == nil {
			return fmt.Errorf("node constructor not found: %s", nodeDef.Type)
		}

		for {
			ctx.Mu.Lock()
			ctx.CurrentNode = node_id
			ctx.NodeStates[node_id] = StateRunning
			ctx.NodeAttempts[node_id]++
			attempt := ctx.NodeAttempts[node_id]
			ctx.Mu.Unlock()

			e.persistContext(ctx)
			e.update_run_record(ctx, RunStatusRunning, nil)

			success, nextNodeIDs, err := nodeImpl.Execute(ctx)
			if err == nil && success && ctx.NodeStates[node_id] == StateRunning {
				ctx.Mu.Lock()
				ctx.NodeStates[node_id] = StateCompleted
				ctx.Mu.Unlock()

				e.persistContext(ctx)
				e.update_run_record(ctx, RunStatusRunning, nil)
				if len(nextNodeIDs) > 0 {
					if err := e.driveFlow(ctx, nextNodeIDs); err != nil {
						return err
					}
				}
				break
			}

			if ctx.NodeStates[node_id] != StateRunning && ctx.NodeStates[node_id] != StateRetrying {
				ctx.Mu.Lock()
				ctx.NodeStates[node_id] = StateWaitingForUser
				ctx.Mu.Unlock()

				e.persistContext(ctx)
				e.update_run_record(ctx, RunStatusWaiting, nil)
				return nil
			}

			if nodeDef.RetryPolicy != nil && attempt < nodeDef.RetryPolicy.MaxAttempts {
				delay := retryDelay(nodeDef.RetryPolicy, attempt)
				e.persistContext(ctx)
				e.update_run_record(ctx, RunStatusRunning, nil)
				if delay > 0 {
					time.Sleep(delay)
				}
				ctx.Mu.Lock()
				ctx.NodeStates[node_id] = StateRetrying
				ctx.Mu.Unlock()
				continue
			}

			baseErr := err
			if baseErr == nil {
				baseErr = errors.New("node execution failed")
			}
			ctx.Mu.Lock()
			ctx.NodeStates[node_id] = StateFailed
			ctx.Mu.Unlock()

			e.persistContext(ctx)
			e.update_run_record(ctx, RunStatusFailed, baseErr)
			if nodeDef.ErrorNextNodeID != "" {
				if err := e.driveFlow(ctx, []string{nodeDef.ErrorNextNodeID}); err != nil {
					return err
				}
				break
			}

			return &NodeError{NodeID: node_id, NodeType: nodeImpl.Type(), Err: baseErr}
		}
	}
	return nil
}

func (e *FlowEngine) createNodeImpl(def NodeDefinition) Node {
	e.RLock()
	constructor, ok := e.NodeRegistry[def.Type]
	e.RUnlock()
	if !ok || constructor == nil {
		return nil
	}
	config := map[string]interface{}{}
	for key, value := range def.Config {
		config[key] = value
	}
	if def.Config == nil {
		config = map[string]interface{}{}
	}
	config["id"] = def.ID
	if len(def.NextNodeIDs) > 0 {
		config["next_node_ids"] = def.NextNodeIDs
	}
	if len(def.NextNodes) > 0 {
		config["next_nodes"] = def.NextNodes
	}
	return constructor(config)
}

func (e *FlowEngine) validateData(schema []FieldSchema, data map[string]interface{}) error {
	for _, f := range schema {
		_, exists := data[f.Key]
		if f.Required && !exists {
			return errors.New("missing required field: " + f.Key)
		}
		if !exists {
			continue
		}
		value := data[f.Key]
		if err := e.validateValue(f, value); err != nil {
			return err
		}
	}
	return nil
}

func (e *FlowEngine) validateValue(schema FieldSchema, value interface{}) error {
	switch schema.Type {
	case "string":
		if _, ok := value.(string); !ok {
			return errors.New("invalid type for field: " + schema.Key)
		}
	case "object":
		if value == nil {
			return errors.New("invalid type for field: " + schema.Key)
		}
		if len(schema.Fields) == 0 {
			return nil
		}
		obj, ok := normalizeObjectForSchema(value)
		if !ok {
			return errors.New("invalid type for field: " + schema.Key)
		}
		if err := e.validateData(schema.Fields, obj); err != nil {
			return err
		}
	case "int":
		switch value.(type) {
		case int, int32, int64:
		default:
			return errors.New("invalid type for field: " + schema.Key)
		}
	case "bool":
		if _, ok := value.(bool); !ok {
			return errors.New("invalid type for field: " + schema.Key)
		}
	}
	return nil
}

func normalizeObjectForSchema(value interface{}) (map[string]interface{}, bool) {
	switch v := value.(type) {
	case map[string]interface{}:
		return v, true
	case map[string]string:
		out := make(map[string]interface{}, len(v))
		for key, val := range v {
			out[key] = val
		}
		return out, true
	}

	rv := reflect.ValueOf(value)
	for rv.IsValid() && rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil, false
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() || rv.Kind() != reflect.Struct {
		return nil, false
	}

	rt := rv.Type()
	out := map[string]interface{}{}
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if !rv.Field(i).CanInterface() {
			continue
		}

		key := field.Name
		out[key] = rv.Field(i).Interface()

		jsonTag := strings.TrimSpace(strings.Split(field.Tag.Get("json"), ",")[0])
		if jsonTag != "" && jsonTag != "-" {
			out[jsonTag] = rv.Field(i).Interface()
		}
	}
	return out, true
}

func (e *FlowEngine) filterData(schema []FieldSchema, data map[string]interface{}) map[string]interface{} {
	if len(schema) == 0 {
		return data
	}
	out := map[string]interface{}{}
	for _, f := range schema {
		if value, ok := data[f.Key]; ok {
			out[f.Key] = value
		}
	}
	return out
}

func (e *FlowEngine) RunNodeStandalone(def NodeDefinition, input map[string]interface{}) (map[string]interface{}, error) {
	e.ensureMaps()
	if input == nil {
		input = map[string]interface{}{}
	}
	if err := e.validateData(def.InputSchema, input); err != nil {
		return nil, err
	}
	flowID := "__test__"
	e.Lock()
	if e.FlowDefinitions == nil {
		e.FlowDefinitions = map[string]FlowDefinition{}
	}
	e.FlowDefinitions[flowID] = FlowDefinition{
		ID:          flowID,
		Name:        "test",
		StartNodeID: def.ID,
		Nodes:       map[string]NodeDefinition{def.ID: def},
	}
	e.Unlock()

	ctx := &ProcessContext{
		InstanceID:   fmt.Sprintf("instance-%d", time.Now().UnixNano()),
		FlowID:       flowID,
		Data:         map[string]interface{}{},
		NodeStates:   map[string]NodeState{},
		NodeAttempts: map[string]int{},
		EngineRef:    e,
	}
	for k, v := range input {
		ctx.Data[k] = v
	}

	node := e.createNodeImpl(def)
	if node == nil {
		return nil, errors.New("node constructor not found: " + def.Type)
	}
	_, _, err := node.Execute(ctx)
	if err != nil {
		return nil, err
	}
	return e.filterData(def.OutputSchema, ctx.Data), nil
}

func (e *FlowEngine) getNextNodeIDsFromDefinition(ctx *ProcessContext, nodeID string) []string {
	flowDef, ok := e.getFlowDefinition(ctx.FlowID)
	if !ok {
		return nil
	}
	nodeDef, ok := flowDef.Nodes[nodeID]
	if !ok {
		return nil
	}
	nextIDs := make([]string, 0, len(nodeDef.NextNodes))
	for _, target := range nodeDef.NextNodes {
		nextIDs = append(nextIDs, target.TargetID)
	}
	return nextIDs
}

func (e *FlowEngine) GetNextNodeIDsFromDefinition(ctx *ProcessContext, nodeID string) []string {
	return e.getNextNodeIDsFromDefinition(ctx, nodeID)
}

func (e *FlowEngine) CompleteManualTask(ins_id, node_id string, input_data map[string]interface{}) error {
	e.RLock()
	ctx := e.RunningContexts[ins_id]
	e.RUnlock()
	if ctx == nil {
		return errors.New("workflow instance not found")
	}

	if input_data != nil {
		ctx.Mu.Lock()
		for key, value := range input_data {
			ctx.Data[key] = value
		}
		ctx.Mu.Unlock()
	}

	ctx.Mu.Lock()
	ctx.NodeStates[node_id] = StateCompleted
	ctx.Mu.Unlock()

	e.persistContext(ctx)
	e.update_run_record(ctx, RunStatusRunning, nil)

	flow, ok := e.getFlowDefinition(ctx.FlowID)
	if !ok {
		return errors.New("flow not found")
	}
	if err := e.driveFlow(ctx, flow.Nodes[node_id].NextNodeIDs); err != nil {
		e.update_run_record(ctx, RunStatusFailed, err)
		return err
	}
	if nodeIsWaiting(ctx) {
		e.update_run_record(ctx, RunStatusWaiting, nil)
		return nil
	}
	e.Lock()
	delete(e.RunningContexts, ins_id)
	e.Unlock()
	e.update_run_record(ctx, RunStatusCompleted, nil)
	return nil
}

func (e *FlowEngine) update_run_record(ctx *ProcessContext, status RunStatus, err error) {
	if ctx == nil {
		return
	}
	now := time.Now()
	e.RLock()
	persisted_snapshot, has_persisted_snapshot := e.ContextSnapshots[ctx.InstanceID]
	e.RUnlock()
	ctx.Mu.Lock()
	data_snapshot := persisted_snapshot.Data
	if !has_persisted_snapshot {
		data_snapshot = snapshot_map(ctx.Data)
	}
	state_attempts := snapshotNodeAttempts(ctx.NodeAttempts)
	current_node := ctx.CurrentNode
	trigger_type := ctx.TriggerType
	trigger_key := ctx.TriggerKey
	ctx.Mu.Unlock()

	record := &RunRecord{
		RunID:         ctx.InstanceID,
		FlowID:        ctx.FlowID,
		Status:        status,
		CurrentNode:   current_node,
		NodeOutputs:   data_snapshot,
		LastUpdatedAt: now,
		NodeAttempts:  state_attempts,
	}

	e.Lock()
	defer e.Unlock()
	existing := e.RunHistory[ctx.InstanceID]
	was_terminal := existing != nil && is_terminal_run_status(existing.Status)
	if existing == nil {
		record.StartedAt = now
		record.Trigger = TriggerInfo{
			Type:      TriggerType(trigger_type),
			Key:       trigger_key,
			StartedAt: now.Format(time.RFC3339Nano),
		}
	} else {
		record.StartedAt = existing.StartedAt
		record.Trigger = existing.Trigger
	}
	if is_terminal_run_status(status) {
		record.CompletedAt = ptrToTime(now)
	}
	if err != nil {
		record.Error = err.Error()
	}
	e.RunHistory[ctx.InstanceID] = record
	if is_terminal_run_status(status) && !was_terminal {
		e.retain_completed_run_locked(ctx.InstanceID)
	}
}

func (r *RunRecord) CompleteAtOrNil(status RunStatus) *time.Time {
	if !is_terminal_run_status(status) {
		return nil
	}
	now := time.Now()
	return &now
}

func is_terminal_run_status(status RunStatus) bool {
	return status == RunStatusCompleted || status == RunStatusFailed || status == RunStatusCancelled
}

func (e *FlowEngine) retain_completed_run_locked(instance_id string) {
	history_limit := e.RunHistoryLimit
	if history_limit == 0 {
		history_limit = default_run_history_limit
	}
	if history_limit < 0 {
		return
	}

	e.completed_run_ids = append(e.completed_run_ids, instance_id)
	for len(e.completed_run_ids)-e.completed_run_head > history_limit {
		expired_instance_id := e.completed_run_ids[e.completed_run_head]
		e.completed_run_ids[e.completed_run_head] = ""
		e.completed_run_head++
		delete(e.RunHistory, expired_instance_id)
		delete(e.ContextSnapshots, expired_instance_id)
	}
	if e.completed_run_head >= run_history_compaction_threshold &&
		e.completed_run_head*2 >= len(e.completed_run_ids) {
		remaining_count := copy(e.completed_run_ids, e.completed_run_ids[e.completed_run_head:])
		for history_index := remaining_count; history_index < len(e.completed_run_ids); history_index++ {
			e.completed_run_ids[history_index] = ""
		}
		e.completed_run_ids = e.completed_run_ids[:remaining_count]
		e.completed_run_head = 0
	}
}

func ptrToTime(now time.Time) *time.Time {
	timeCopy := now
	return &timeCopy
}

func (e *FlowEngine) persistContext(ctx *ProcessContext) {
	if ctx == nil {
		return
	}
	ctx.Mu.Lock()
	data_snapshot := snapshot_map(ctx.Data)
	state_snapshot := snapshotNodeStates(ctx.NodeStates)
	ctx.Mu.Unlock()

	e.Lock()
	defer e.Unlock()
	if e.ContextSnapshots == nil {
		e.ContextSnapshots = map[string]flowInstanceSnapshot{}
	}
	e.ContextSnapshots[ctx.InstanceID] = flowInstanceSnapshot{
		Data:       data_snapshot,
		NodeStates: state_snapshot,
		UpdatedAt:  time.Now(),
	}
}

func clone_string_map(values map[string]interface{}) map[string]interface{} {
	if len(values) == 0 {
		return map[string]interface{}{}
	}
	copied := make(map[string]interface{}, len(values))
	for key, value := range values {
		copied[key] = clone_data(value)
	}
	return copied
}

func snapshot_map(values map[string]interface{}) map[string]interface{} {
	if len(values) == 0 {
		return map[string]interface{}{}
	}
	copied := make(map[string]interface{}, len(values))
	for key, value := range values {
		copied[key] = clone_data(value)
	}
	return copied
}

func snapshotNodeStates(values map[string]NodeState) map[string]NodeState {
	if len(values) == 0 {
		return map[string]NodeState{}
	}
	copied := make(map[string]NodeState, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return copied
}

func snapshotNodeAttempts(values map[string]int) map[string]int {
	if len(values) == 0 {
		return map[string]int{}
	}
	copied := make(map[string]int, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return copied
}

type clone_data_visit struct {
	kind    reflect.Kind
	pointer uintptr
}

func clone_data(value interface{}) interface{} {
	cloned_value, fast_path_ok := clone_json_data(value, nil)
	if fast_path_ok {
		return cloned_value
	}

	raw, marshal_error := json.Marshal(value)
	if marshal_error != nil {
		return value
	}
	var out interface{}
	if unmarshal_error := json.Unmarshal(raw, &out); unmarshal_error != nil {
		return value
	}
	return out
}

func clone_json_data(value interface{}, visiting map[clone_data_visit]struct{}) (interface{}, bool) {
	switch typed_value := value.(type) {
	case nil:
		return nil, true
	case string:
		return typed_value, true
	case bool:
		return typed_value, true
	case int:
		return float64(typed_value), true
	case int8:
		return float64(typed_value), true
	case int16:
		return float64(typed_value), true
	case int32:
		return float64(typed_value), true
	case int64:
		return float64(typed_value), true
	case uint:
		return float64(typed_value), true
	case uint8:
		return float64(typed_value), true
	case uint16:
		return float64(typed_value), true
	case uint32:
		return float64(typed_value), true
	case uint64:
		return float64(typed_value), true
	case uintptr:
		return float64(typed_value), true
	case float64:
		if typed_value != typed_value || typed_value > 1.7976931348623157e+308 || typed_value < -1.7976931348623157e+308 {
			return nil, false
		}
		return typed_value, true
	case map[string]interface{}:
		if typed_value == nil {
			return nil, true
		}
		if visiting == nil {
			visiting = make(map[clone_data_visit]struct{})
		}
		visit := clone_data_visit{kind: reflect.Map, pointer: reflect.ValueOf(typed_value).Pointer()}
		if _, exists := visiting[visit]; exists {
			return nil, false
		}
		visiting[visit] = struct{}{}
		cloned_map := make(map[string]interface{}, len(typed_value))
		for key, nested_value := range typed_value {
			cloned_nested_value, ok := clone_json_data(nested_value, visiting)
			if !ok {
				delete(visiting, visit)
				return nil, false
			}
			cloned_map[key] = cloned_nested_value
		}
		delete(visiting, visit)
		return cloned_map, true
	case []interface{}:
		if typed_value == nil {
			return nil, true
		}
		if visiting == nil {
			visiting = make(map[clone_data_visit]struct{})
		}
		visit := clone_data_visit{kind: reflect.Slice, pointer: reflect.ValueOf(typed_value).Pointer()}
		if _, exists := visiting[visit]; exists {
			return nil, false
		}
		visiting[visit] = struct{}{}
		cloned_slice := make([]interface{}, len(typed_value))
		for value_index, nested_value := range typed_value {
			cloned_nested_value, ok := clone_json_data(nested_value, visiting)
			if !ok {
				delete(visiting, visit)
				return nil, false
			}
			cloned_slice[value_index] = cloned_nested_value
		}
		delete(visiting, visit)
		return cloned_slice, true
	case map[string]string:
		if typed_value == nil {
			return nil, true
		}
		cloned_map := make(map[string]interface{}, len(typed_value))
		for key, nested_value := range typed_value {
			cloned_map[key] = nested_value
		}
		return cloned_map, true
	case []string:
		if typed_value == nil {
			return nil, true
		}
		cloned_slice := make([]interface{}, len(typed_value))
		for value_index, nested_value := range typed_value {
			cloned_slice[value_index] = nested_value
		}
		return cloned_slice, true
	default:
		return nil, false
	}
}

func retryDelay(policy *RetryPolicy, attempt int) time.Duration {
	if policy == nil || policy.DelayMs <= 0 {
		return 0
	}
	delayMs := float64(policy.DelayMs)
	mult := policy.BackoffMultiplier
	if mult > 1 && attempt > 1 {
		for i := 1; i < attempt; i++ {
			delayMs *= mult
		}
	}
	return time.Duration(delayMs) * time.Millisecond
}

func parseCronLikeDuration(expression string) (time.Duration, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return 0, errors.New("empty cron expression")
	}
	if strings.HasPrefix(expression, "@every") {
		durationText := strings.TrimSpace(strings.TrimPrefix(expression, "@every"))
		return time.ParseDuration(durationText)
	}
	switch expression {
	case "@hourly":
		return time.Hour, nil
	case "@daily":
		return 24 * time.Hour, nil
	default:
		return time.ParseDuration(expression)
	}
}

func nodeIsWaiting(ctx *ProcessContext) bool {
	ctx.Mu.Lock()
	defer ctx.Mu.Unlock()
	for _, state := range ctx.NodeStates {
		if state == StateWaitingForUser || state == StateWaitingForMerge || state == StateWaitingForSubprocess {
			return true
		}
	}
	return false
}
