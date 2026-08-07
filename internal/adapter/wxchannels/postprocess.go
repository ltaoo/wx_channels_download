package wxchannelsadapter

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"wx_channel/pkg/flowengine"

	"wx_channel/internal/adapter"
	"wx_channel/pkg/hermes"
	"wx_channel/pkg/scraper/wxchannels"

	"github.com/expr-lang/expr"
)

const (
	wxchannelsPostprocessRunFlowID    = "wxchannels_postprocess"
	wxchannelsPostprocessMainFlowID   = "wxchannels_postprocess_main"
	wxchannelsPostprocessOutputFlowID = "wxchannels_postprocess_output"

	wxchannelsPostprocessRunKey = "postprocess_run"

	ctxKey                     = "context"
	ctxInputFile               = "input_file"
	ctxDecodeKey               = "decode_key"
	ctxDecryptedFile           = "decrypted_file"
	ctxMP3File                 = "mp3_file"
	ctxMP4File                 = "mp4_file"
	ctxTaskType                = "task_config_type"
	ctxTaskSuffix              = "task_config_suffix"
	ctxResourceType            = "resource_type"
	ctxResourceHasDecodeSecret = "resource_has_decode_key"
	ctxResource                = "resource"
	ctxResources               = "resources"
	ctxArchiveRequested        = "archive_requested"

	wxchannelsPostprocessContextKeyCtx                  = ctxKey
	wxchannelsPostprocessContextInputFile               = ctxInputFile
	wxchannelsPostprocessContextDecodeKey               = ctxDecodeKey
	wxchannelsPostprocessContextDecryptedFile           = ctxDecryptedFile
	wxchannelsPostprocessContextMp3File                 = ctxMP3File
	wxchannelsPostprocessContextMp4File                 = ctxMP4File
	wxchannelsPostprocessContextTaskType                = ctxTaskType
	wxchannelsPostprocessContextTaskSuffix              = ctxTaskSuffix
	wxchannelsPostprocessContextResourceType            = ctxResourceType
	wxchannelsPostprocessContextResourceHasDecodeSecret = ctxResourceHasDecodeSecret
	wxchannelsPostprocessContextResource                = ctxResource
	wxchannelsPostprocessContextTask                    = "task"
	wxchannelsPostprocessContextResources               = ctxResources
	wxchannelsPostprocessContextArchiveRequested        = ctxArchiveRequested
	wxchannelsPostprocessContextBasePath                = "base_path"

	wxchannelsPostprocessNodeStatusStarted              = "started"
	wxchannelsPostprocessNodeStatusSucceeded            = "succeeded"
	wxchannelsPostprocessNodeStatusFailed               = "failed"
	wxchannelsPostprocessFlowNodeBootstrapContext       = "bootstrap_context"
	wxchannelsPostprocessFlowNodePrepareResourceContext = "prepare_resource_context"
	wxchannelsPostprocessFlowNodeDecrypt                = "decrypt"
	wxchannelsPostprocessFlowNodeConvertMP3             = "convert_mp3"
	wxchannelsPostprocessFlowNodeFinalizeMP3            = "finalize_mp3"
	wxchannelsPostprocessFlowNodeStreamConvert          = "stream_convert"
	wxchannelsPostprocessFlowNodeFinalizeStream         = "finalize_stream"
	wxchannelsPostprocessFlowNodeZipResources           = "zip_resources"
	wxchannelsPostprocessFlowNodeTaskJobUpdate          = "task_job_update"
	wxchannelsPostprocessFlowNodeDone                   = "done"
)

type postprocessRun struct {
	task        *hermes.TaskJob
	resource    *hermes.ResourceJob
	basePath    string
	originalExt string
	log         func(string, ...interface{})
}

const wxchannelsPostprocessLogValueMaxLen = 260

func runWXChannelsNode(values map[string]interface{}, nodeID string, fn func(*postprocessRun) (interface{}, error)) (interface{}, error) {
	run, err := postprocessRunFromContext(values)
	if err != nil {
		return nil, err
	}
	input := postprocessContextSnapshot(values)
	logPostprocessNode(run, nodeID, wxchannelsPostprocessNodeStatusStarted, input, nil, nil, nil)
	result, err := fn(run)
	output := postprocessContextSnapshot(values)
	diff := postprocessContextDiff(input, output)
	if err != nil {
		logPostprocessNode(run, nodeID, wxchannelsPostprocessNodeStatusFailed, input, output, diff, err)
		return result, err
	}
	logPostprocessNode(run, nodeID, wxchannelsPostprocessNodeStatusSucceeded, input, output, diff, nil)
	return result, nil
}

func logPostprocessNode(run *postprocessRun, nodeID, status string, input map[string]interface{}, output map[string]interface{}, delta map[string][2]interface{}, err error) {
	if run == nil || run.log == nil {
		return
	}

	taskID := 0
	resourceID := 0
	resourceType := ""
	resourcePath := ""
	if run.task != nil {
		taskID = run.task.ID
	}
	if run.resource != nil {
		resourceID = run.resource.ID
		resourceType = run.resource.Type
		resourcePath = run.resource.FilePath
	}
	if err != nil {
		run.log(
			"Postprocess.wxchannels: node=%s status=%s task_id=%d resource_id=%d resource_type=%q resource_path=%q error=%v input=%s output=%s changed=%s",
			nodeID, status, taskID, resourceID, resourceType, resourcePath, err,
			formatContextSnapshot(input), formatContextSnapshot(output), formatContextDiff(delta),
		)
		return
	}
	run.log(
		"Postprocess.wxchannels: node=%s status=%s task_id=%d resource_id=%d resource_type=%q resource_path=%q input=%s output=%s changed=%s",
		nodeID, status, taskID, resourceID, resourceType, resourcePath,
		formatContextSnapshot(input), formatContextSnapshot(output), formatContextDiff(delta),
	)
}

func logPostprocessFlow(flowID string, status string, run *postprocessRun, err error) {
	if run == nil || run.log == nil {
		return
	}

	taskID := 0
	if run.task != nil {
		taskID = run.task.ID
	}
	if err != nil {
		run.log("Postprocess.wxchannels: flow=%s status=%s task_id=%d error=%v", flowID, status, taskID, err)
		return
	}
	run.log("Postprocess.wxchannels: flow=%s status=%s task_id=%d", flowID, status, taskID)
}

func postprocessContextSnapshot(values map[string]interface{}) map[string]interface{} {
	snapshot := map[string]interface{}{}
	if len(values) == 0 {
		return snapshot
	}

	for key, value := range values {
		if key == wxchannelsPostprocessRunKey || key == wxchannelsPostprocessContextKeyCtx {
			continue
		}
		snapshot[key] = normalizePostprocessContextValue(key, value)
	}
	return snapshot
}

func normalizePostprocessContextValue(key string, value interface{}) interface{} {
	if value == nil {
		return nil
	}

	switch typed := value.(type) {
	case *hermes.TaskJob:
		if typed == nil {
			return nil
		}
		return map[string]interface{}{
			"id":            typed.ID,
			"name":          typed.Name,
			"platform":      typed.Platform,
			"resource_cnt":  len(typed.Resources),
			"config_type":   taskConfigType(typed.Config),
			"config_suffix": taskConfigSuffix(typed.Config),
		}
	case hermes.TaskJob:
		return normalizePostprocessContextValue(key, &typed)
	case *hermes.ResourceJob:
		if typed == nil {
			return nil
		}
		return map[string]interface{}{
			"id":                typed.ID,
			"name":              typed.Name,
			"type":              typed.Type,
			"kind":              typed.Kind,
			"file":              typed.FilePath,
			"size":              typed.Size,
			"decode_key_exists": typed.Extra["decode_key"] != "",
		}
	case hermes.ResourceJob:
		return normalizePostprocessContextValue(key, &typed)
	case []hermes.ResourceJob:
		items := make([]interface{}, 0, len(typed))
		for _, resource := range typed {
			items = append(items, normalizePostprocessContextValue("resource", resource))
		}
		return normalizeGenericSlice(items)
	case []interface{}:
		if key == wxchannelsPostprocessContextResources {
			items := make([]interface{}, 0, len(typed))
			for _, value := range typed {
				items = append(items, normalizePostprocessContextValue("resource", value))
			}
			return normalizeGenericSlice(items)
		}
		return normalizeGenericSlice(typed)
	case map[string]interface{}:
		if key == wxchannelsPostprocessContextResource {
			return normalizePostprocessResourceMap(typed)
		}
		return normalizeGenericMap(typed)
	case map[string]string:
		return normalizeGenericMapString(typed)
	default:
		return truncateLogValue(fmt.Sprintf("%v", value))
	}
}

func normalizePostprocessResourceMap(raw map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"id":                castInt(raw["ID"]),
		"name":              castString(raw["Name"]),
		"type":              castString(raw["Type"]),
		"kind":              castString(raw["Kind"]),
		"file":              castString(raw["FilePath"]),
		"size":              castInt64(raw["Size"]),
		"decode_key_exists": postprocessResourceMapHasDecodeKey(raw),
	}
}

func postprocessResourceMapHasDecodeKey(raw map[string]interface{}) bool {
	if raw == nil {
		return false
	}
	if rawDecodeKey, ok := raw["decode_key"]; ok {
		return castString(rawDecodeKey) != ""
	}
	if rawExtra, ok := raw["Extra"].(map[string]interface{}); ok {
		return castString(rawExtra["decode_key"]) != ""
	}
	extra, ok := raw["Extra"].(map[string]string)
	if !ok {
		return false
	}
	return extra["decode_key"] != ""
}

func normalizeGenericSlice(values []interface{}) []interface{} {
	maxItems := 8
	if len(values) <= maxItems {
		return values
	}
	items := append([]interface{}{}, values[:maxItems]...)
	items = append(items, map[string]interface{}{
		"_truncated": len(values) - maxItems,
	})
	return items
}

func normalizeGenericMap(values map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(values))
	for key, value := range values {
		out[key] = truncateLogValue(fmt.Sprintf("%v", value))
	}
	return out
}

func normalizeGenericMapString(values map[string]string) map[string]interface{} {
	out := make(map[string]interface{}, len(values))
	for key, value := range values {
		out[key] = truncateLogValue(value)
	}
	return out
}

func postprocessContextDiff(before map[string]interface{}, after map[string]interface{}) map[string][2]interface{} {
	delta := map[string][2]interface{}{}
	if len(before) == 0 && len(after) == 0 {
		return delta
	}

	for key, beforeValue := range before {
		if afterValue, ok := after[key]; ok {
			if !reflect.DeepEqual(beforeValue, afterValue) {
				delta[key] = [2]interface{}{beforeValue, afterValue}
			}
		} else {
			delta[key] = [2]interface{}{beforeValue, nil}
		}
	}
	for key, afterValue := range after {
		if _, ok := before[key]; !ok {
			delta[key] = [2]interface{}{nil, afterValue}
		}
	}
	return delta
}

func formatContextSnapshot(snapshot map[string]interface{}) string {
	if len(snapshot) == 0 {
		return "{}"
	}

	keys := make([]string, 0, len(snapshot))
	for key := range snapshot {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, truncateLogValue(fmt.Sprintf("%v", snapshot[key]))))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func formatContextDiff(delta map[string][2]interface{}) string {
	if len(delta) == 0 {
		return "{}"
	}
	keys := make([]string, 0, len(delta))
	for key := range delta {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		values := delta[key]
		from := truncateLogValue(fmt.Sprintf("%v", values[0]))
		to := truncateLogValue(fmt.Sprintf("%v", values[1]))
		parts = append(parts, fmt.Sprintf("%s[from=%s,to=%s]", key, from, to))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func truncateLogValue(value string) string {
	if len(value) <= wxchannelsPostprocessLogValueMaxLen {
		return value
	}
	return value[:wxchannelsPostprocessLogValueMaxLen] + "..."
}

type wxchannelsLoggedGatewayNode struct {
	IDVal  string
	Config map[string]interface{}
}

func newWXChannelsLoggedGatewayNode(config map[string]interface{}) flowengine.Node {
	id, _ := config["id"].(string)
	return &wxchannelsLoggedGatewayNode{
		IDVal:  id,
		Config: config,
	}
}

func (n *wxchannelsLoggedGatewayNode) ID() string { return n.IDVal }

func (n *wxchannelsLoggedGatewayNode) Type() string { return "GatewayNode" }

func (n *wxchannelsLoggedGatewayNode) Execute(ctx *flowengine.ProcessContext) (bool, []string, error) {
	if ctx == nil {
		return false, nil, errors.New("gateway context is nil")
	}
	input := formatContextSnapshot(postprocessContextSnapshot(ctx.Data))
	n.logf(ctx, "gateway=%s type=%s started decision_input=%s", n.IDVal, n.configValue("gateway_type"), input)

	gatewayType, _ := n.Config["gateway_type"].(string)
	isJoining := false
	if joining, ok := n.Config["is_joining"].(bool); ok {
		isJoining = joining
	}

	if isJoining {
		return n.handleMerge(ctx, input)
	}

	switch gatewayType {
	case "Parallel":
		next := n.nextNodeIDs()
		n.logf(ctx, "gateway=%s type=Parallel next=%v", n.IDVal, next)
		return true, next, nil
	case "Exclusive":
		rules := n.rules()
		for _, rule := range rules {
			condition, ok := rule["condition"].(string)
			if !ok {
				continue
			}
			ok, err := n.evaluateCondition(ctx, condition)
			if err != nil {
				n.logf(ctx, "gateway=%s error condition=%q target=%v err=%v", n.IDVal, condition, rule["target_id"], err)
				return false, nil, err
			}
			n.logf(ctx, "gateway=%s condition=%q result=%v target=%v", n.IDVal, condition, ok, rule["target_id"])
			if ok {
				target := castString(rule["target_id"])
				ctx.Mu.Lock()
				ctx.Data[n.ID()+"_target"] = target
				ctx.Mu.Unlock()
				n.logf(ctx, "gateway=%s selected=%q", n.IDVal, target)
				return true, []string{target}, nil
			}
		}
		return false, nil, errors.New("Exclusive Gateway failed to find a valid path")
	default:
		return false, nil, errors.New("GatewayNode: unknown gateway_type")
	}
}

func (n *wxchannelsLoggedGatewayNode) handleMerge(ctx *flowengine.ProcessContext, input string) (bool, []string, error) {
	waitListRaw, ok := n.Config["wait_for_incoming"]
	if !ok {
		return false, nil, errors.New("GatewayNode merge requires wait_for_incoming")
	}
	waitList, err := castStringSlice(waitListRaw)
	if err != nil {
		return false, nil, err
	}

	allCompleted := true
	for _, depID := range waitList {
		ctx.Mu.Lock()
		depState := ctx.NodeStates[depID]
		ctx.Mu.Unlock()
		if depState != flowengine.StateCompleted {
			allCompleted = false
			break
		}
	}
	if !allCompleted {
		ctx.Mu.Lock()
		ctx.NodeStates[n.ID()] = flowengine.StateWaitingForMerge
		ctx.Mu.Unlock()
		n.logf(ctx, "gateway=%s waiting merge input=%s", n.IDVal, input)
		return true, nil, nil
	}

	next := n.nextNodeIDs()
	n.logf(ctx, "gateway=%s merge_done next=%v", n.IDVal, next)
	return true, next, nil
}

func (n *wxchannelsLoggedGatewayNode) nextNodeIDs() []string {
	if nextNodeIDs, ok := n.Config["next_node_ids"].([]string); ok {
		return append([]string{}, nextNodeIDs...)
	}

	if rawTargets, ok := n.Config["next_nodes"].([]flowengine.TargetNode); ok {
		targets := make([]string, 0, len(rawTargets))
		for _, target := range rawTargets {
			targets = append(targets, target.TargetID)
		}
		return targets
	}

	if rawTargets, ok := n.Config["next_nodes"].([]interface{}); ok {
		targets := make([]string, 0, len(rawTargets))
		for _, raw := range rawTargets {
			if mapped, ok := raw.(map[string]interface{}); ok {
				if targetID, ok := mapped["target_id"].(string); ok {
					targets = append(targets, targetID)
				}
			}
			if target, ok := raw.(flowengine.TargetNode); ok {
				targets = append(targets, target.TargetID)
			}
		}
		return targets
	}

	return []string{}
}

func (n *wxchannelsLoggedGatewayNode) rules() []map[string]interface{} {
	if raw, ok := n.Config["rules"].([]map[string]interface{}); ok {
		return raw
	}
	if raw, ok := n.Config["rules"].([]interface{}); ok {
		rules := make([]map[string]interface{}, 0, len(raw))
		for _, item := range raw {
			if mapped, ok := item.(map[string]interface{}); ok {
				rules = append(rules, mapped)
			}
		}
		return rules
	}
	return nil
}

func (n *wxchannelsLoggedGatewayNode) configValue(key string) string {
	if value, ok := n.Config[key]; ok {
		return castString(value)
	}
	return ""
}

func (n *wxchannelsLoggedGatewayNode) evaluateCondition(ctx *flowengine.ProcessContext, condition string) (bool, error) {
	program, err := expr.Compile(condition)
	if err != nil {
		return false, err
	}
	out, err := expr.Run(program, ctx.Data)
	if err != nil {
		return false, err
	}
	result, ok := out.(bool)
	if !ok {
		return false, nil
	}
	return result, nil
}

func (n *wxchannelsLoggedGatewayNode) logf(ctx *flowengine.ProcessContext, format string, args ...interface{}) {
	if ctx == nil || n == nil {
		return
	}
	run, ok := ctx.Data[wxchannelsPostprocessRunKey].(*postprocessRun)
	if !ok || run == nil || run.log == nil {
		return
	}
	run.log("Postprocess.wxchannels: "+format, args...)
}

func castStringSlice(value interface{}) ([]string, error) {
	switch typed := value.(type) {
	case []string:
		return typed, nil
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("invalid wait_for_incoming item %T", item)
			}
			out = append(out, s)
		}
		return out, nil
	}
	return nil, fmt.Errorf("unsupported wait_for_incoming type %T", value)
}

func logPostprocessFlowNodeExecution(run *postprocessRun, flowEngine *flowengine.FlowEngine, flowID, instanceID string, err error) {
	if run == nil || run.log == nil || flowEngine == nil || instanceID == "" {
		return
	}

	record, hasRecord := flowEngine.GetRunSnapshot(instanceID)
	if !hasRecord {
		if err != nil {
			run.log("Postprocess.wxchannels: flow=%s node_execution_summary unavailable: %v", flowID, err)
		} else {
			run.log("Postprocess.wxchannels: flow=%s node_execution_summary unavailable", flowID)
		}
		return
	}

	_, nodeStates := flowEngine.GetRunContext(instanceID)

	nodeIDs := make([]string, 0, len(record.NodeAttempts))
	for nodeID := range record.NodeAttempts {
		nodeIDs = append(nodeIDs, nodeID)
	}
	for nodeID := range nodeStates {
		already := false
		for _, existingID := range nodeIDs {
			if existingID == nodeID {
				already = true
				break
			}
		}
		if !already {
			nodeIDs = append(nodeIDs, nodeID)
		}
	}
	if len(nodeIDs) == 0 {
		run.log("Postprocess.wxchannels: flow=%s node_execution_summary: no nodes were executed", flowID)
		return
	}

	sort.Strings(nodeIDs)
	for _, nodeID := range nodeIDs {
		state := "unknown"
		attempts := record.NodeAttempts[nodeID]

		if attempts == 0 {
			state = "skipped"
		}

		if len(nodeStates) > 0 {
			if nodeState, ok := nodeStates[nodeID]; ok {
				switch nodeState {
				case flowengine.StateCompleted:
					state = "succeeded"
				case flowengine.StateFailed:
					state = "failed"
				case flowengine.StateRunning, flowengine.StateRetrying:
					state = "running"
				case flowengine.StatePending:
					state = "pending"
				case flowengine.StateWaitingForUser, flowengine.StateWaitingForMerge, flowengine.StateWaitingForSubprocess:
					state = "waiting"
				default:
					state = string(nodeState)
				}
			}
		}

		// 回退推断：没有节点状态时，以流执行结果判断最后一个活跃节点是否失败。
		if state == "unknown" && attempts > 0 && record.Status == flowengine.RunStatusFailed && record.CurrentNode == nodeID {
			state = "failed"
		}
		if state == "unknown" && attempts > 0 && record.Status == flowengine.RunStatusCompleted {
			state = "succeeded"
		}

		taskSuffix := ""
		taskSuffixInfo := ""
		if attempts > 0 {
			taskSuffix = fmt.Sprintf(" attempts=%d", attempts)
		}
		if state == "failed" && record.CurrentNode == nodeID && record.Error != "" {
			taskSuffixInfo = " error=" + record.Error
		}
		run.log("Postprocess.wxchannels: flow=%s node=%s status=%s%s%s", flowID, nodeID, state, taskSuffix, taskSuffixInfo)
	}

	if record.Error != "" && record.CurrentNode != "" {
		run.log("Postprocess.wxchannels: flow=%s failed_at_node=%s error=%v", flowID, record.CurrentNode, record.Error)
	}
	if record.Status != "" {
		run.log("Postprocess.wxchannels: flow=%s final_status=%s", flowID, record.Status)
	}
}

var wxchannelsPostprocessFlow = flowengine.FlowDefinition{
	ID:   wxchannelsPostprocessRunFlowID,
	Name: "wxchannels_postprocess",
	ContextSchema: []flowengine.FieldSchema{
		{Key: wxchannelsPostprocessContextKeyCtx, Type: "object", Required: false},
		{Key: wxchannelsPostprocessContextResource, Type: "object", Required: false, Fields: []flowengine.FieldSchema{
			{Key: "ID", Type: "int", Required: true},
			{Key: "Name", Type: "string", Required: false},
			{Key: "Kind", Type: "string", Required: false},
			{Key: "Type", Type: "string", Required: true},
			{Key: "UniqueID", Type: "string", Required: false},
			{Key: "FilePath", Type: "string", Required: false},
			{Key: "Size", Type: "int", Required: false},
			{Key: "Downloaded", Type: "int", Required: false},
			{Key: "Speed", Type: "int", Required: false},
			{Key: "Extra", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
				{Key: "decode_key", Type: "string", Required: false},
			}},
		}},
		{Key: ctxInputFile, Type: "string", Required: true},
		{Key: ctxDecodeKey, Type: "any", Required: false},
		{Key: ctxResourceType, Type: "string", Required: true},
		{Key: ctxResourceHasDecodeSecret, Type: "bool", Required: false},
		{Key: ctxTaskType, Type: "int", Required: false},
		{Key: ctxTaskSuffix, Type: "string", Required: false},
		{Key: wxchannelsPostprocessRunKey, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
			{Key: "task", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
				{Key: "ID", Type: "int", Required: true},
				{Key: "Name", Type: "string", Required: false},
				{Key: "UniqueID", Type: "string", Required: false},
				{Key: "SavePath", Type: "string", Required: false},
				{Key: "FilenameTemplate", Type: "string", Required: false},
				{Key: "Platform", Type: "string", Required: false},
				{Key: "Config", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
					{Key: "type", Type: "any", Required: false},
					{Key: "suffix", Type: "string", Required: false},
				}},
			}},
			{Key: "resource", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
				{Key: "ID", Type: "int", Required: true},
				{Key: "Name", Type: "string", Required: false},
				{Key: "Kind", Type: "string", Required: false},
				{Key: "Type", Type: "string", Required: true},
				{Key: "UniqueID", Type: "string", Required: false},
				{Key: "FilePath", Type: "string", Required: false},
				{Key: "Size", Type: "int", Required: false},
				{Key: "Downloaded", Type: "int", Required: false},
				{Key: "Speed", Type: "int", Required: false},
				{Key: "Extra", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
					{Key: "decode_key", Type: "string", Required: false},
				}},
			}},
			{Key: "basePath", Type: "string", Required: false},
			{Key: "originalExt", Type: "string", Required: false},
			{Key: "log", Type: "any", Required: false},
		}},
	},
	StartNodeID: "start",
	Nodes: map[string]flowengine.NodeDefinition{
		"start": {
			ID:   "start",
			Type: "FuncNode",
			Name: "开始：准备每个资源的上下文",
			InputSchema: []flowengine.FieldSchema{
				{Key: wxchannelsPostprocessRunKey, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
					{Key: "task", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
						{Key: "ID", Type: "int", Required: true},
						{Key: "Name", Type: "string", Required: false},
						{Key: "UniqueID", Type: "string", Required: false},
						{Key: "SavePath", Type: "string", Required: false},
						{Key: "FilenameTemplate", Type: "string", Required: false},
						{Key: "Platform", Type: "string", Required: false},
						{Key: "Config", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
							{Key: "type", Type: "any", Required: false},
							{Key: "suffix", Type: "string", Required: false},
						}},
					}},
					{Key: "resource", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
						{Key: "ID", Type: "int", Required: true},
						{Key: "Name", Type: "string", Required: false},
						{Key: "Kind", Type: "string", Required: false},
						{Key: "Type", Type: "string", Required: true},
						{Key: "UniqueID", Type: "string", Required: false},
						{Key: "FilePath", Type: "string", Required: false},
						{Key: "Size", Type: "int", Required: false},
						{Key: "Downloaded", Type: "int", Required: false},
						{Key: "Speed", Type: "int", Required: false},
						{Key: "Extra", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
							{Key: "decode_key", Type: "string", Required: false},
						}},
					}},
					{Key: "basePath", Type: "string", Required: false},
					{Key: "originalExt", Type: "string", Required: false},
					{Key: "log", Type: "any", Required: false},
				},
				},
				{Key: wxchannelsPostprocessContextResource, Type: "object", Required: false, Fields: []flowengine.FieldSchema{
					{Key: "ID", Type: "int", Required: true},
					{Key: "Name", Type: "string", Required: false},
					{Key: "Kind", Type: "string", Required: false},
					{Key: "Type", Type: "string", Required: true},
					{Key: "UniqueID", Type: "string", Required: false},
					{Key: "FilePath", Type: "string", Required: false},
					{Key: "Size", Type: "int", Required: false},
					{Key: "Downloaded", Type: "int", Required: false},
					{Key: "Speed", Type: "int", Required: false},
					{Key: "Extra", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
						{Key: "decode_key", Type: "string", Required: false},
					}},
				},
				},
				{Key: ctxInputFile, Type: "string", Required: false},
				{Key: ctxResourceType, Type: "string", Required: false},
				{Key: ctxDecodeKey, Type: "any", Required: false},
				{Key: ctxTaskType, Type: "int", Required: false},
				{Key: ctxTaskSuffix, Type: "string", Required: false},
			},
			OutputSchema: []flowengine.FieldSchema{
				{Key: ctxInputFile, Type: "string", Required: true},
				{Key: ctxResourceType, Type: "string", Required: true},
				{Key: wxchannelsPostprocessContextResourceHasDecodeSecret, Type: "bool", Required: true},
				{Key: ctxDecodeKey, Type: "any", Required: false},
				{Key: ctxTaskType, Type: "int", Required: false},
				{Key: ctxTaskSuffix, Type: "string", Required: false},
			},
			Config:    map[string]interface{}{"func": prepareResourceContextNode},
			NextNodes: []flowengine.TargetNode{{TargetID: "route_resource"}},
		},
		"route_resource": {
			ID:   "route_resource",
			Type: "GatewayNode",
			Name: "资源类型分流",
			InputSchema: []flowengine.FieldSchema{
				{Key: ctxResourceType, Type: "string", Required: true},
				{Key: ctxResourceHasDecodeSecret, Type: "bool", Required: false},
			},
			OutputSchema: []flowengine.FieldSchema{
				{Key: ctxResourceType, Type: "string", Required: true},
				{Key: ctxResourceHasDecodeSecret, Type: "bool", Required: false},
			},
			Config: map[string]interface{}{
				"gateway_type": "Exclusive",
				"is_joining":   false,
				"rules": []map[string]interface{}{
					{"condition": `resource_type == "STREAM"`, "target_id": "stream_convert"},
					{"condition": "resource_has_decode_key == true", "target_id": "decrypt"},
					{"condition": "true", "target_id": "done"},
				},
			},
		},
		"decrypt": {
			ID:   "decrypt",
			Type: "FuncNode",
			Name: "微信视频号文件解密",
			InputSchema: []flowengine.FieldSchema{
				{Key: ctxInputFile, Type: "string", Required: true},
				{Key: ctxDecodeKey, Type: "any", Required: false},
			},
			OutputSchema: []flowengine.FieldSchema{
				{Key: ctxDecryptedFile, Type: "string", Required: false},
			},
			Config: map[string]interface{}{"func": decryptNode},
			NextNodes: []flowengine.TargetNode{
				{TargetID: "route_output_format"},
			},
		},
		"route_output_format": {
			ID:   "route_output_format",
			Type: "GatewayNode",
			Name: "输出格式分流",
			InputSchema: []flowengine.FieldSchema{
				{Key: ctxTaskType, Type: "int", Required: false},
				{Key: ctxTaskSuffix, Type: "string", Required: false},
			},
			OutputSchema: []flowengine.FieldSchema{
				{Key: ctxTaskType, Type: "int", Required: false},
				{Key: ctxTaskSuffix, Type: "string", Required: false},
			},
			Config: map[string]interface{}{
				"gateway_type": "Exclusive",
				"is_joining":   false,
				"rules": []map[string]interface{}{
					{"condition": `task_config_suffix == ".mp3" || task_config_suffix == "mp3"`, "target_id": "convert_mp3"},
					{"condition": "true", "target_id": "done"},
				},
			},
		},
		"stream_convert": {
			ID:   "stream_convert",
			Type: "FuncNode",
			Name: "流媒体转封装为 MP4",
			InputSchema: []flowengine.FieldSchema{
				{Key: ctxInputFile, Type: "string", Required: true},
			},
			OutputSchema: []flowengine.FieldSchema{
				{Key: ctxMP4File, Type: "string", Required: false},
			},
			Config: map[string]interface{}{"func": streamConvertNode},
			NextNodes: []flowengine.TargetNode{
				{TargetID: "finalize_stream"},
			},
		},
		"finalize_stream": {
			ID:   "finalize_stream",
			Type: "FuncNode",
			Name: "生成流媒体资源结果",
			InputSchema: []flowengine.FieldSchema{
				{Key: wxchannelsPostprocessRunKey, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
					{Key: "task", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
						{Key: "ID", Type: "int", Required: true},
						{Key: "Name", Type: "string", Required: false},
						{Key: "UniqueID", Type: "string", Required: false},
						{Key: "SavePath", Type: "string", Required: false},
						{Key: "FilenameTemplate", Type: "string", Required: false},
						{Key: "Platform", Type: "string", Required: false},
						{Key: "Config", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
							{Key: "type", Type: "any", Required: false},
							{Key: "suffix", Type: "string", Required: false},
						}},
					}},
					{Key: "resource", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
						{Key: "ID", Type: "int", Required: true},
						{Key: "Name", Type: "string", Required: false},
						{Key: "Kind", Type: "string", Required: false},
						{Key: "Type", Type: "string", Required: true},
						{Key: "UniqueID", Type: "string", Required: false},
						{Key: "FilePath", Type: "string", Required: false},
						{Key: "Size", Type: "int", Required: false},
						{Key: "Downloaded", Type: "int", Required: false},
						{Key: "Speed", Type: "int", Required: false},
						{Key: "Extra", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
							{Key: "decode_key", Type: "string", Required: false},
						}},
					}},
					{Key: "basePath", Type: "string", Required: false},
					{Key: "originalExt", Type: "string", Required: false},
					{Key: "log", Type: "any", Required: false},
				}},
				{Key: ctxMP4File, Type: "string", Required: false},
			},
			OutputSchema: []flowengine.FieldSchema{},
			Config:       map[string]interface{}{"func": finalizeStreamNode},
			NextNodes: []flowengine.TargetNode{
				{TargetID: "task_job_update"},
			},
		},
		"convert_mp3": {
			ID:   "convert_mp3",
			Type: "FuncNode",
			Name: "FFmpeg 转换 MP3",
			InputSchema: []flowengine.FieldSchema{
				{Key: ctxDecryptedFile, Type: "string", Required: true},
			},
			OutputSchema: []flowengine.FieldSchema{
				{Key: ctxMP3File, Type: "string", Required: false},
			},
			Config: map[string]interface{}{"func": convertMP3Node},
			NextNodes: []flowengine.TargetNode{
				{TargetID: "finalize_mp3"},
			},
		},
		"finalize_mp3": {
			ID:   "finalize_mp3",
			Type: "FuncNode",
			Name: "生成 MP3 资源结果",
			InputSchema: []flowengine.FieldSchema{
				{Key: wxchannelsPostprocessRunKey, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
					{Key: "task", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
						{Key: "ID", Type: "int", Required: true},
						{Key: "Name", Type: "string", Required: false},
						{Key: "UniqueID", Type: "string", Required: false},
						{Key: "SavePath", Type: "string", Required: false},
						{Key: "FilenameTemplate", Type: "string", Required: false},
						{Key: "Platform", Type: "string", Required: false},
						{Key: "Config", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
							{Key: "type", Type: "any", Required: false},
							{Key: "suffix", Type: "string", Required: false},
						}},
					}},
					{Key: "resource", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
						{Key: "ID", Type: "int", Required: true},
						{Key: "Name", Type: "string", Required: false},
						{Key: "Kind", Type: "string", Required: false},
						{Key: "Type", Type: "string", Required: true},
						{Key: "UniqueID", Type: "string", Required: false},
						{Key: "FilePath", Type: "string", Required: false},
						{Key: "Size", Type: "int", Required: false},
						{Key: "Downloaded", Type: "int", Required: false},
						{Key: "Speed", Type: "int", Required: false},
						{Key: "Extra", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
							{Key: "decode_key", Type: "string", Required: false},
						}},
					}},
					{Key: "basePath", Type: "string", Required: false},
					{Key: "originalExt", Type: "string", Required: false},
					{Key: "log", Type: "any", Required: false},
				}},
				{Key: ctxMP3File, Type: "string", Required: true},
			},
			OutputSchema: []flowengine.FieldSchema{},
			Config:       map[string]interface{}{"func": finalizeMP3Node},
			NextNodes: []flowengine.TargetNode{
				{TargetID: "task_job_update"},
			},
		},
		"zip_resources": {
			ID:   "zip_resources",
			Type: "FuncNode",
			Name: "压缩全部 Resources",
			InputSchema: []flowengine.FieldSchema{
				{Key: wxchannelsPostprocessRunKey, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
					{Key: "task", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
						{Key: "ID", Type: "int", Required: true},
						{Key: "Name", Type: "string", Required: false},
						{Key: "UniqueID", Type: "string", Required: false},
						{Key: "SavePath", Type: "string", Required: false},
						{Key: "FilenameTemplate", Type: "string", Required: false},
						{Key: "Platform", Type: "string", Required: false},
						{Key: "Config", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
							{Key: "type", Type: "any", Required: false},
							{Key: "suffix", Type: "string", Required: false},
						}},
					}},
					{Key: "resource", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
						{Key: "ID", Type: "int", Required: true},
						{Key: "Name", Type: "string", Required: false},
						{Key: "Kind", Type: "string", Required: false},
						{Key: "Type", Type: "string", Required: true},
						{Key: "UniqueID", Type: "string", Required: false},
						{Key: "FilePath", Type: "string", Required: false},
						{Key: "Size", Type: "int", Required: false},
						{Key: "Downloaded", Type: "int", Required: false},
						{Key: "Speed", Type: "int", Required: false},
						{Key: "Extra", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
							{Key: "decode_key", Type: "string", Required: false},
						}},
					}},
					{Key: "basePath", Type: "string", Required: false},
					{Key: "originalExt", Type: "string", Required: false},
					{Key: "log", Type: "any", Required: false},
				}},
			},
			OutputSchema: []flowengine.FieldSchema{
				{Key: "archive_file", Type: "string", Required: false},
			},
			Config: map[string]interface{}{"func": zipResourcesNode},
			NextNodes: []flowengine.TargetNode{
				{TargetID: "task_job_update"},
			},
		},
		"task_job_update": {
			ID:   "task_job_update",
			Type: "FuncNode",
			Name: "更新传入的 TaskJob",
			InputSchema: []flowengine.FieldSchema{
				{Key: wxchannelsPostprocessRunKey, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
					{Key: "task", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
						{Key: "ID", Type: "int", Required: true},
						{Key: "Name", Type: "string", Required: false},
						{Key: "UniqueID", Type: "string", Required: false},
						{Key: "SavePath", Type: "string", Required: false},
						{Key: "FilenameTemplate", Type: "string", Required: false},
						{Key: "Platform", Type: "string", Required: false},
						{Key: "Config", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
							{Key: "type", Type: "any", Required: false},
							{Key: "suffix", Type: "string", Required: false},
						}},
					}},
					{Key: "resource", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
						{Key: "ID", Type: "int", Required: true},
						{Key: "Name", Type: "string", Required: false},
						{Key: "Kind", Type: "string", Required: false},
						{Key: "Type", Type: "string", Required: true},
						{Key: "UniqueID", Type: "string", Required: false},
						{Key: "FilePath", Type: "string", Required: false},
						{Key: "Size", Type: "int", Required: false},
						{Key: "Downloaded", Type: "int", Required: false},
						{Key: "Speed", Type: "int", Required: false},
						{Key: "Extra", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
							{Key: "decode_key", Type: "string", Required: false},
						}},
					}},
					{Key: "basePath", Type: "string", Required: false},
					{Key: "originalExt", Type: "string", Required: false},
					{Key: "log", Type: "any", Required: false},
				}},
			},
			OutputSchema: []flowengine.FieldSchema{},
			Config:       map[string]interface{}{"func": taskJobUpdateNode},
			NextNodes:    []flowengine.TargetNode{},
		},
		"done": {
			ID:           "done",
			Type:         "FuncNode",
			Name:         "无需后处理",
			InputSchema:  []flowengine.FieldSchema{},
			OutputSchema: []flowengine.FieldSchema{},
			Config:       map[string]interface{}{"func": noopPostprocessDoneNode},
		},
	},
}

var wxchannelsPostprocessMainFlow = flowengine.FlowDefinition{
	ID:   wxchannelsPostprocessMainFlowID,
	Name: "wxchannels_postprocess_main",
	ContextSchema: []flowengine.FieldSchema{
		{Key: wxchannelsPostprocessContextKeyCtx, Type: "object", Required: false},
		{Key: wxchannelsPostprocessContextTask, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
			{Key: "ID", Type: "int", Required: true},
			{Key: "Name", Type: "string", Required: false},
			{Key: "UniqueID", Type: "string", Required: false},
			{Key: "SavePath", Type: "string", Required: false},
			{Key: "FilenameTemplate", Type: "string", Required: false},
			{Key: "Platform", Type: "string", Required: false},
			{Key: "Config", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
				{Key: "type", Type: "any", Required: false},
				{Key: "suffix", Type: "string", Required: false},
			}},
		}},
		{Key: wxchannelsPostprocessContextBasePath, Type: "string", Required: true},
		{
			Key:      wxchannelsPostprocessContextResources,
			Type:     "array",
			Required: false,
			Fields: []flowengine.FieldSchema{
				{Key: "ID", Type: "int", Required: true},
				{Key: "Name", Type: "string", Required: false},
				{Key: "Kind", Type: "string", Required: false},
				{Key: "Type", Type: "string", Required: true},
				{Key: "UniqueID", Type: "string", Required: false},
				{Key: "FilePath", Type: "string", Required: false},
				{Key: "Size", Type: "int", Required: false},
				{Key: "Downloaded", Type: "int", Required: false},
				{Key: "Speed", Type: "int", Required: false},
				{Key: "Extra", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
					{Key: "decode_key", Type: "string", Required: false},
				}},
			},
		},
		{Key: wxchannelsPostprocessContextArchiveRequested, Type: "bool", Required: true},
		{Key: ctxTaskType, Type: "int", Required: false},
		{Key: ctxTaskSuffix, Type: "string", Required: false},
		{Key: wxchannelsPostprocessRunKey, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
			{Key: "task", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
				{Key: "ID", Type: "int", Required: true},
				{Key: "Name", Type: "string", Required: false},
				{Key: "UniqueID", Type: "string", Required: false},
				{Key: "SavePath", Type: "string", Required: false},
				{Key: "FilenameTemplate", Type: "string", Required: false},
				{Key: "Platform", Type: "string", Required: false},
				{Key: "Config", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
					{Key: "type", Type: "any", Required: false},
					{Key: "suffix", Type: "string", Required: false},
				}},
			}},
			{Key: "resource", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
				{Key: "ID", Type: "int", Required: true},
				{Key: "Name", Type: "string", Required: false},
				{Key: "Kind", Type: "string", Required: false},
				{Key: "Type", Type: "string", Required: true},
				{Key: "UniqueID", Type: "string", Required: false},
				{Key: "FilePath", Type: "string", Required: false},
				{Key: "Size", Type: "int", Required: false},
				{Key: "Downloaded", Type: "int", Required: false},
				{Key: "Speed", Type: "int", Required: false},
				{Key: "Extra", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
					{Key: "decode_key", Type: "string", Required: false},
				}},
			}},
			{Key: "basePath", Type: "string", Required: false},
			{Key: "originalExt", Type: "string", Required: false},
			{Key: "log", Type: "any", Required: false},
		}},
	},
	StartNodeID: "start",
	Nodes: map[string]flowengine.NodeDefinition{
		"start": {
			ID:   "start",
			Type: "FuncNode",
			Name: "开始：初始化 postprocess 上下文",
			InputSchema: []flowengine.FieldSchema{
				{Key: wxchannelsPostprocessContextTask, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
					{Key: "ID", Type: "int", Required: true},
					{Key: "Name", Type: "string", Required: false},
					{Key: "UniqueID", Type: "string", Required: false},
					{Key: "SavePath", Type: "string", Required: false},
					{Key: "FilenameTemplate", Type: "string", Required: false},
					{Key: "Platform", Type: "string", Required: false},
					{Key: "Config", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
						{Key: "type", Type: "any", Required: false},
						{Key: "suffix", Type: "string", Required: false},
					}},
				},
				},
				{Key: wxchannelsPostprocessContextBasePath, Type: "string", Required: true},
				{Key: wxchannelsPostprocessContextArchiveRequested, Type: "bool", Required: true},
				{Key: wxchannelsPostprocessRunKey, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
					{Key: "task", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
						{Key: "ID", Type: "int", Required: true},
						{Key: "Name", Type: "string", Required: false},
						{Key: "UniqueID", Type: "string", Required: false},
						{Key: "SavePath", Type: "string", Required: false},
						{Key: "FilenameTemplate", Type: "string", Required: false},
						{Key: "Platform", Type: "string", Required: false},
						{Key: "Config", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
							{Key: "type", Type: "any", Required: false},
							{Key: "suffix", Type: "string", Required: false},
						}},
					}},
					{Key: "resource", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
						{Key: "ID", Type: "int", Required: true},
						{Key: "Name", Type: "string", Required: false},
						{Key: "Kind", Type: "string", Required: false},
						{Key: "Type", Type: "string", Required: true},
						{Key: "UniqueID", Type: "string", Required: false},
						{Key: "FilePath", Type: "string", Required: false},
						{Key: "Size", Type: "int", Required: false},
						{Key: "Downloaded", Type: "int", Required: false},
						{Key: "Speed", Type: "int", Required: false},
						{Key: "Extra", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
							{Key: "decode_key", Type: "string", Required: false},
						}},
					}},
					{Key: "basePath", Type: "string", Required: false},
					{Key: "originalExt", Type: "string", Required: false},
					{Key: "log", Type: "any", Required: false},
				},
				},
				{Key: wxchannelsPostprocessContextKeyCtx, Type: "object", Required: false},
			},
			OutputSchema: []flowengine.FieldSchema{
				{
					Key:      wxchannelsPostprocessContextResources,
					Type:     "array",
					Required: true,
					Fields: []flowengine.FieldSchema{
						{Key: "ID", Type: "int", Required: true},
						{Key: "Name", Type: "string", Required: false},
						{Key: "Kind", Type: "string", Required: false},
						{Key: "Type", Type: "string", Required: true},
						{Key: "UniqueID", Type: "string", Required: false},
						{Key: "FilePath", Type: "string", Required: false},
						{Key: "Size", Type: "int", Required: false},
						{Key: "Downloaded", Type: "int", Required: false},
						{Key: "Speed", Type: "int", Required: false},
						{Key: "Extra", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
							{Key: "decode_key", Type: "string", Required: false},
						}},
					},
				},
				{Key: wxchannelsPostprocessContextArchiveRequested, Type: "bool", Required: true},
				{Key: wxchannelsPostprocessContextTaskType, Type: "int", Required: false},
				{Key: wxchannelsPostprocessContextTaskSuffix, Type: "string", Required: false},
			},
			Config:    map[string]interface{}{"func": bootstrapPostprocessContextNode},
			NextNodes: []flowengine.TargetNode{{TargetID: "loop_resources"}},
		},
		"loop_resources": {
			ID:   "loop_resources",
			Type: "LoopNode",
			Name: "遍历并处理每个资源",
			InputSchema: []flowengine.FieldSchema{
				{
					Key:      wxchannelsPostprocessContextResources,
					Type:     "array",
					Required: true,
					Fields: []flowengine.FieldSchema{
						{Key: "ID", Type: "int", Required: true},
						{Key: "Name", Type: "string", Required: false},
						{Key: "Kind", Type: "string", Required: false},
						{Key: "Type", Type: "string", Required: true},
						{Key: "UniqueID", Type: "string", Required: false},
						{Key: "FilePath", Type: "string", Required: false},
						{Key: "Size", Type: "int", Required: false},
						{Key: "Downloaded", Type: "int", Required: false},
						{Key: "Speed", Type: "int", Required: false},
						{Key: "Extra", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
							{Key: "decode_key", Type: "string", Required: false},
						}},
					},
				},
				{Key: wxchannelsPostprocessContextArchiveRequested, Type: "bool", Required: true},
				{Key: wxchannelsPostprocessRunKey, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
					{Key: "task", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
						{Key: "ID", Type: "int", Required: true},
						{Key: "Name", Type: "string", Required: false},
						{Key: "UniqueID", Type: "string", Required: false},
						{Key: "SavePath", Type: "string", Required: false},
						{Key: "FilenameTemplate", Type: "string", Required: false},
						{Key: "Platform", Type: "string", Required: false},
						{Key: "Config", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
							{Key: "type", Type: "any", Required: false},
							{Key: "suffix", Type: "string", Required: false},
						}},
					}},
					{Key: "resource", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
						{Key: "ID", Type: "int", Required: true},
						{Key: "Name", Type: "string", Required: false},
						{Key: "Kind", Type: "string", Required: false},
						{Key: "Type", Type: "string", Required: true},
						{Key: "UniqueID", Type: "string", Required: false},
						{Key: "FilePath", Type: "string", Required: false},
						{Key: "Size", Type: "int", Required: false},
						{Key: "Downloaded", Type: "int", Required: false},
						{Key: "Speed", Type: "int", Required: false},
						{Key: "Extra", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
							{Key: "decode_key", Type: "string", Required: false},
						}},
					}},
					{Key: "basePath", Type: "string", Required: false},
					{Key: "originalExt", Type: "string", Required: false},
					{Key: "log", Type: "any", Required: false},
				},
				},
			},
			OutputSchema: []flowengine.FieldSchema{},
			Config: map[string]interface{}{
				"item_key":     wxchannelsPostprocessContextResource,
				"iterable_key": wxchannelsPostprocessContextResources,
				"cleanup_keys": []interface{}{wxchannelsPostprocessContextResource, ctxInputFile, ctxResourceType, ctxDecodeKey, ctxResourceHasDecodeSecret, ctxDecryptedFile, ctxMP4File, ctxMP3File},
				"workflow":     wxchannelsPostprocessFlow,
			},
			NextNodes: []flowengine.TargetNode{{TargetID: "archive_requested_gate"}},
		},
		"archive_requested_gate": {
			ID:   "archive_requested_gate",
			Type: "GatewayNode",
			Name: "是否需要归档",
			InputSchema: []flowengine.FieldSchema{
				{Key: wxchannelsPostprocessContextArchiveRequested, Type: "bool", Required: true},
			},
			OutputSchema: []flowengine.FieldSchema{
				{Key: wxchannelsPostprocessContextArchiveRequested, Type: "bool", Required: true},
			},
			Config: map[string]interface{}{
				"gateway_type": "Exclusive",
				"is_joining":   false,
				"rules": []map[string]interface{}{
					{"condition": "archive_requested == true", "target_id": "run_output_flow"},
					{"condition": "true", "target_id": "done"},
				},
			},
			NextNodes: []flowengine.TargetNode{},
		},
		"run_output_flow": {
			ID:   "run_output_flow",
			Type: "WorkflowNode",
			Name: "执行归档子流程",
			InputSchema: []flowengine.FieldSchema{
				{Key: wxchannelsPostprocessRunKey, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
					{Key: "task", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
						{Key: "ID", Type: "int", Required: true},
						{Key: "Name", Type: "string", Required: false},
						{Key: "UniqueID", Type: "string", Required: false},
						{Key: "SavePath", Type: "string", Required: false},
						{Key: "FilenameTemplate", Type: "string", Required: false},
						{Key: "Platform", Type: "string", Required: false},
						{Key: "Config", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
							{Key: "type", Type: "any", Required: false},
							{Key: "suffix", Type: "string", Required: false},
						}},
					}},
					{Key: "resource", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
						{Key: "ID", Type: "int", Required: true},
						{Key: "Name", Type: "string", Required: false},
						{Key: "Kind", Type: "string", Required: false},
						{Key: "Type", Type: "string", Required: true},
						{Key: "UniqueID", Type: "string", Required: false},
						{Key: "FilePath", Type: "string", Required: false},
						{Key: "Size", Type: "int", Required: false},
						{Key: "Downloaded", Type: "int", Required: false},
						{Key: "Speed", Type: "int", Required: false},
						{Key: "Extra", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
							{Key: "decode_key", Type: "string", Required: false},
						}},
					}},
					{Key: "basePath", Type: "string", Required: false},
					{Key: "originalExt", Type: "string", Required: false},
					{Key: "log", Type: "any", Required: false},
				},
				},
				{Key: ctxTaskType, Type: "int", Required: false},
				{Key: ctxTaskSuffix, Type: "string", Required: false},
			},
			OutputSchema: []flowengine.FieldSchema{},
			Config:       map[string]interface{}{"workflow": wxchannelsOutputFlow},
			NextNodes:    []flowengine.TargetNode{{TargetID: "done"}},
		},
		"done": {
			ID:           "done",
			Type:         "FuncNode",
			Name:         "后处理完成",
			InputSchema:  []flowengine.FieldSchema{},
			OutputSchema: []flowengine.FieldSchema{},
			Config:       map[string]interface{}{"func": noopPostprocessDoneNode},
		},
	},
}

var wxchannelsOutputFlow = flowengine.FlowDefinition{
	ID:   wxchannelsPostprocessOutputFlowID,
	Name: "wxchannels_postprocess_output",
	ContextSchema: []flowengine.FieldSchema{
		{Key: wxchannelsPostprocessContextKeyCtx, Type: "object", Required: false},
		{Key: ctxTaskType, Type: "int", Required: false},
		{Key: ctxTaskSuffix, Type: "string", Required: false},
		{Key: wxchannelsPostprocessRunKey, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
			{Key: "task", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
				{Key: "ID", Type: "int", Required: true},
				{Key: "Name", Type: "string", Required: false},
				{Key: "UniqueID", Type: "string", Required: false},
				{Key: "SavePath", Type: "string", Required: false},
				{Key: "FilenameTemplate", Type: "string", Required: false},
				{Key: "Platform", Type: "string", Required: false},
				{Key: "Config", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
					{Key: "type", Type: "any", Required: false},
					{Key: "suffix", Type: "string", Required: false},
				}},
			}},
			{Key: "resource", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
				{Key: "ID", Type: "int", Required: true},
				{Key: "Name", Type: "string", Required: false},
				{Key: "Kind", Type: "string", Required: false},
				{Key: "Type", Type: "string", Required: true},
				{Key: "UniqueID", Type: "string", Required: false},
				{Key: "FilePath", Type: "string", Required: false},
				{Key: "Size", Type: "int", Required: false},
				{Key: "Downloaded", Type: "int", Required: false},
				{Key: "Speed", Type: "int", Required: false},
				{Key: "Extra", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
					{Key: "decode_key", Type: "string", Required: false},
				}},
			}},
			{Key: "basePath", Type: "string", Required: false},
			{Key: "originalExt", Type: "string", Required: false},
			{Key: "log", Type: "any", Required: false},
		}},
	},
	StartNodeID: "output_start",
	Nodes: map[string]flowengine.NodeDefinition{
		"output_start": {
			ID:   "output_start",
			Type: "StartNode",
			Name: "output_start",
			InputSchema: []flowengine.FieldSchema{
				{Key: wxchannelsPostprocessRunKey, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
					{Key: "task", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
						{Key: "ID", Type: "int", Required: true},
						{Key: "Name", Type: "string", Required: false},
						{Key: "UniqueID", Type: "string", Required: false},
						{Key: "SavePath", Type: "string", Required: false},
						{Key: "FilenameTemplate", Type: "string", Required: false},
						{Key: "Platform", Type: "string", Required: false},
						{Key: "Config", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
							{Key: "type", Type: "any", Required: false},
							{Key: "suffix", Type: "string", Required: false},
						}},
					}},
					{Key: "resource", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
						{Key: "ID", Type: "int", Required: true},
						{Key: "Name", Type: "string", Required: false},
						{Key: "Kind", Type: "string", Required: false},
						{Key: "Type", Type: "string", Required: true},
						{Key: "UniqueID", Type: "string", Required: false},
						{Key: "FilePath", Type: "string", Required: false},
						{Key: "Size", Type: "int", Required: false},
						{Key: "Downloaded", Type: "int", Required: false},
						{Key: "Speed", Type: "int", Required: false},
						{Key: "Extra", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
							{Key: "decode_key", Type: "string", Required: false},
						}},
					}},
					{Key: "basePath", Type: "string", Required: false},
					{Key: "originalExt", Type: "string", Required: false},
					{Key: "log", Type: "any", Required: false},
				},
				},
				{Key: ctxTaskType, Type: "int", Required: false},
				{Key: ctxTaskSuffix, Type: "string", Required: false},
			},
			OutputSchema: []flowengine.FieldSchema{
				{Key: wxchannelsPostprocessRunKey, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
					{Key: "task", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
						{Key: "ID", Type: "int", Required: true},
						{Key: "Name", Type: "string", Required: false},
						{Key: "UniqueID", Type: "string", Required: false},
						{Key: "SavePath", Type: "string", Required: false},
						{Key: "FilenameTemplate", Type: "string", Required: false},
						{Key: "Platform", Type: "string", Required: false},
						{Key: "Config", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
							{Key: "type", Type: "any", Required: false},
							{Key: "suffix", Type: "string", Required: false},
						}},
					}},
					{Key: "resource", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
						{Key: "ID", Type: "int", Required: true},
						{Key: "Name", Type: "string", Required: false},
						{Key: "Kind", Type: "string", Required: false},
						{Key: "Type", Type: "string", Required: true},
						{Key: "UniqueID", Type: "string", Required: false},
						{Key: "FilePath", Type: "string", Required: false},
						{Key: "Size", Type: "int", Required: false},
						{Key: "Downloaded", Type: "int", Required: false},
						{Key: "Speed", Type: "int", Required: false},
						{Key: "Extra", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
							{Key: "decode_key", Type: "string", Required: false},
						}},
					}},
					{Key: "basePath", Type: "string", Required: false},
					{Key: "originalExt", Type: "string", Required: false},
					{Key: "log", Type: "any", Required: false},
				},
				},
				{Key: ctxTaskType, Type: "int", Required: false},
				{Key: ctxTaskSuffix, Type: "string", Required: false},
			},
			NextNodes: []flowengine.TargetNode{{TargetID: "route_output_format"}},
		},
		"route_output_format": {
			ID:   "route_output_format",
			Type: "GatewayNode",
			Name: "输出格式分流",
			InputSchema: []flowengine.FieldSchema{
				{Key: ctxTaskType, Type: "int", Required: false},
				{Key: ctxTaskSuffix, Type: "string", Required: false},
			},
			OutputSchema: []flowengine.FieldSchema{
				{Key: ctxTaskType, Type: "int", Required: false},
				{Key: ctxTaskSuffix, Type: "string", Required: false},
			},
			Config: map[string]interface{}{
				"gateway_type": "Exclusive",
				"is_joining":   false,
				"rules": []map[string]interface{}{
					{"condition": "task_config_type == -1", "target_id": "zip_resources"},
					{"condition": `task_config_suffix == ".mp3" || task_config_suffix == "mp3"`, "target_id": "convert_mp3"},
					{"condition": "true", "target_id": "done"},
				},
			},
		},
		"stream_convert": {
			ID:           StreamConvertNode.ID,
			Type:         StreamConvertNode.Type,
			Name:         StreamConvertNode.Name,
			InputSchema:  StreamConvertNode.InputSchema,
			OutputSchema: StreamConvertNode.OutputSchema,
			Config:       StreamConvertNode.Config,
			NextNodes: []flowengine.TargetNode{
				{TargetID: "finalize_stream"},
			},
		},
		"finalize_stream": {
			ID:           FinalizeStreamNode.ID,
			Type:         FinalizeStreamNode.Type,
			Name:         FinalizeStreamNode.Name,
			InputSchema:  FinalizeStreamNode.InputSchema,
			OutputSchema: FinalizeStreamNode.OutputSchema,
			Config:       FinalizeStreamNode.Config,
			NextNodes: []flowengine.TargetNode{
				{TargetID: "task_job_update"},
			},
		},
		"convert_mp3": {
			ID:           ConvertMP3Node.ID,
			Type:         ConvertMP3Node.Type,
			Name:         ConvertMP3Node.Name,
			InputSchema:  ConvertMP3Node.InputSchema,
			OutputSchema: ConvertMP3Node.OutputSchema,
			Config:       ConvertMP3Node.Config,
			NextNodes: []flowengine.TargetNode{
				{TargetID: "finalize_mp3"},
			},
		},
		"finalize_mp3": {
			ID:           FinalizeMP3Node.ID,
			Type:         FinalizeMP3Node.Type,
			Name:         FinalizeMP3Node.Name,
			InputSchema:  FinalizeMP3Node.InputSchema,
			OutputSchema: FinalizeMP3Node.OutputSchema,
			Config:       FinalizeMP3Node.Config,
			NextNodes: []flowengine.TargetNode{
				{TargetID: "task_job_update"},
			},
		},
		"zip_resources": {
			ID:           ZipResourcesNode.ID,
			Type:         ZipResourcesNode.Type,
			Name:         ZipResourcesNode.Name,
			InputSchema:  ZipResourcesNode.InputSchema,
			OutputSchema: ZipResourcesNode.OutputSchema,
			Config:       ZipResourcesNode.Config,
			NextNodes: []flowengine.TargetNode{
				{TargetID: "task_job_update"},
			},
		},
		"task_job_update": {
			ID:           TaskJobUpdateNode.ID,
			Type:         TaskJobUpdateNode.Type,
			Name:         TaskJobUpdateNode.Name,
			InputSchema:  TaskJobUpdateNode.InputSchema,
			OutputSchema: TaskJobUpdateNode.OutputSchema,
			Config:       TaskJobUpdateNode.Config,
			NextNodes:    []flowengine.TargetNode{},
		},
		"done": {
			ID:           "done",
			Type:         "FuncNode",
			Name:         "无需后处理",
			InputSchema:  []flowengine.FieldSchema{},
			OutputSchema: []flowengine.FieldSchema{},
			Config:       map[string]interface{}{"func": noopPostprocessDoneNode},
		},
	},
}

// GetWXChannelsPostprocessFlowVisualization returns a read-only flow graph payload
// for frontend visualization.
func GetWXChannelsPostprocessFlowVisualization(flowID string) (*WXChannelsPostprocessFlowVisualizationPayload, error) {
	available := []flowengine.FlowDefinition{wxchannelsPostprocessMainFlow, wxchannelsPostprocessFlow, wxchannelsOutputFlow}
	payload, err := flowengine.BuildFlowVisualizationPayload(available, flowID, flowengine.FlowVisualizationOptions{
		Platform: "wxchannels",
		Purpose:  "postprocess-flow-visualization",
		Editable: false,
	})
	if err != nil {
		return nil, err
	}
	return payload, nil
}

type WXChannelsPostprocessFlowVisualizationPayload = flowengine.FlowVisualizationPayload

// DecryptNode decrypts the downloaded file using the decode key.
// Context values:
//   - input_file: path to the downloaded file
//   - decode_key: decode key as string/number
//
// Output:
//   - decrypted_file: path to the decrypted file
var DecryptNode = flowengine.NodeDefinition{
	ID:   "decrypt",
	Type: "FuncNode",
	Name: "微信视频号文件解密",
	InputSchema: []flowengine.FieldSchema{
		{Key: ctxInputFile, Type: "string", Required: true},
		{Key: ctxDecodeKey, Type: "any", Required: false},
	},
	OutputSchema: []flowengine.FieldSchema{
		{Key: ctxDecryptedFile, Type: "string", Required: false},
	},
	Config: map[string]interface{}{"func": decryptNode},
}

// ConvertMP3Node converts the decrypted file to MP3 using ffmpeg.
// Context values:
//   - decrypted_file: path to the decrypted file
//
// Output:
//   - mp3_file: path to the MP3 file
var ConvertMP3Node = flowengine.NodeDefinition{
	ID:   "convert_mp3",
	Type: "FuncNode",
	Name: "FFmpeg 转换 MP3",
	InputSchema: []flowengine.FieldSchema{
		{Key: ctxDecryptedFile, Type: "string", Required: true},
	},
	OutputSchema: []flowengine.FieldSchema{
		{Key: ctxMP3File, Type: "string", Required: false},
	},
	Config: map[string]interface{}{"func": convertMP3Node},
}

var FinalizeMP3Node = flowengine.NodeDefinition{
	ID:   "finalize_mp3",
	Type: "FuncNode",
	Name: "生成 MP3 资源结果",
	InputSchema: []flowengine.FieldSchema{
		{Key: wxchannelsPostprocessRunKey, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
			{Key: "task", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
				{Key: "ID", Type: "int", Required: true},
				{Key: "Name", Type: "string", Required: false},
				{Key: "UniqueID", Type: "string", Required: false},
				{Key: "SavePath", Type: "string", Required: false},
				{Key: "FilenameTemplate", Type: "string", Required: false},
				{Key: "Platform", Type: "string", Required: false},
				{Key: "Config", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
					{Key: "type", Type: "any", Required: false},
					{Key: "suffix", Type: "string", Required: false},
				}},
			}},
			{Key: "resource", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
				{Key: "ID", Type: "int", Required: true},
				{Key: "Name", Type: "string", Required: false},
				{Key: "Kind", Type: "string", Required: false},
				{Key: "Type", Type: "string", Required: true},
				{Key: "UniqueID", Type: "string", Required: false},
				{Key: "FilePath", Type: "string", Required: false},
				{Key: "Size", Type: "int", Required: false},
				{Key: "Downloaded", Type: "int", Required: false},
				{Key: "Speed", Type: "int", Required: false},
				{Key: "Extra", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
					{Key: "decode_key", Type: "string", Required: false},
				}},
			}},
			{Key: "basePath", Type: "string", Required: false},
			{Key: "originalExt", Type: "string", Required: false},
			{Key: "log", Type: "any", Required: false},
		},
		},
		{Key: ctxMP3File, Type: "string", Required: true},
	},
	OutputSchema: []flowengine.FieldSchema{},
	Config:       map[string]interface{}{"func": finalizeMP3Node},
}

// StreamConvertNode converts a stream file to a playable format.
// MKV and MP4 files pass through unchanged. FLV/TS files are remuxed to MP4.
//
// Context values:
//   - input_file: original downloaded file path
//
// Output:
//   - mp4_file: final playable file path (may be original MKV/MP4 or converted MP4)
var StreamConvertNode = flowengine.NodeDefinition{
	ID:   "stream_convert",
	Type: "FuncNode",
	Name: "流媒体转封装为 MP4",
	InputSchema: []flowengine.FieldSchema{
		{Key: ctxInputFile, Type: "string", Required: true},
	},
	OutputSchema: []flowengine.FieldSchema{
		{Key: ctxMP4File, Type: "string", Required: false},
	},
	Config: map[string]interface{}{"func": streamConvertNode},
}

// FinalizeStreamNode applies the stream conversion result to the resource model.
var FinalizeStreamNode = flowengine.NodeDefinition{
	ID:   "finalize_stream",
	Type: "FuncNode",
	Name: "生成流媒体资源结果",
	InputSchema: []flowengine.FieldSchema{
		{Key: wxchannelsPostprocessRunKey, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
			{Key: "task", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
				{Key: "ID", Type: "int", Required: true},
				{Key: "Name", Type: "string", Required: false},
				{Key: "UniqueID", Type: "string", Required: false},
				{Key: "SavePath", Type: "string", Required: false},
				{Key: "FilenameTemplate", Type: "string", Required: false},
				{Key: "Platform", Type: "string", Required: false},
				{Key: "Config", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
					{Key: "type", Type: "any", Required: false},
					{Key: "suffix", Type: "string", Required: false},
				}},
			}},
			{Key: "resource", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
				{Key: "ID", Type: "int", Required: true},
				{Key: "Name", Type: "string", Required: false},
				{Key: "Kind", Type: "string", Required: false},
				{Key: "Type", Type: "string", Required: true},
				{Key: "UniqueID", Type: "string", Required: false},
				{Key: "FilePath", Type: "string", Required: false},
				{Key: "Size", Type: "int", Required: false},
				{Key: "Downloaded", Type: "int", Required: false},
				{Key: "Speed", Type: "int", Required: false},
				{Key: "Extra", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
					{Key: "decode_key", Type: "string", Required: false},
				}},
			}},
			{Key: "basePath", Type: "string", Required: false},
			{Key: "originalExt", Type: "string", Required: false},
			{Key: "log", Type: "any", Required: false},
		},
		},
		{Key: ctxMP4File, Type: "string", Required: false},
	},
	OutputSchema: []flowengine.FieldSchema{},
	Config:       map[string]interface{}{"func": finalizeStreamNode},
}

var ZipResourcesNode = flowengine.NodeDefinition{
	ID:   "zip_resources",
	Type: "FuncNode",
	Name: "压缩全部 Resources",
	InputSchema: []flowengine.FieldSchema{
		{Key: wxchannelsPostprocessRunKey, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
			{Key: "task", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
				{Key: "ID", Type: "int", Required: true},
				{Key: "Name", Type: "string", Required: false},
				{Key: "UniqueID", Type: "string", Required: false},
				{Key: "SavePath", Type: "string", Required: false},
				{Key: "FilenameTemplate", Type: "string", Required: false},
				{Key: "Platform", Type: "string", Required: false},
				{Key: "Config", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
					{Key: "type", Type: "any", Required: false},
					{Key: "suffix", Type: "string", Required: false},
				}},
			}},
			{Key: "resource", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
				{Key: "ID", Type: "int", Required: true},
				{Key: "Name", Type: "string", Required: false},
				{Key: "Kind", Type: "string", Required: false},
				{Key: "Type", Type: "string", Required: true},
				{Key: "UniqueID", Type: "string", Required: false},
				{Key: "FilePath", Type: "string", Required: false},
				{Key: "Size", Type: "int", Required: false},
				{Key: "Downloaded", Type: "int", Required: false},
				{Key: "Speed", Type: "int", Required: false},
				{Key: "Extra", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
					{Key: "decode_key", Type: "string", Required: false},
				}},
			}},
			{Key: "basePath", Type: "string", Required: false},
			{Key: "originalExt", Type: "string", Required: false},
			{Key: "log", Type: "any", Required: false},
		},
		},
	},
	OutputSchema: []flowengine.FieldSchema{
		{Key: "archive_file", Type: "string", Required: false},
	},
	Config: map[string]interface{}{"func": zipResourcesNode},
}

// TaskJobUpdateNode commits post-processing results to the exact TaskJob pointer
// passed into Postprocess.
var TaskJobUpdateNode = flowengine.NodeDefinition{
	ID:   "task_job_update",
	Type: "FuncNode",
	Name: "更新传入的 TaskJob",
	InputSchema: []flowengine.FieldSchema{
		{Key: wxchannelsPostprocessRunKey, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
			{Key: "task", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
				{Key: "ID", Type: "int", Required: true},
				{Key: "Name", Type: "string", Required: false},
				{Key: "UniqueID", Type: "string", Required: false},
				{Key: "SavePath", Type: "string", Required: false},
				{Key: "FilenameTemplate", Type: "string", Required: false},
				{Key: "Platform", Type: "string", Required: false},
				{Key: "Config", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
					{Key: "type", Type: "any", Required: false},
					{Key: "suffix", Type: "string", Required: false},
				}},
			}},
			{Key: "resource", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
				{Key: "ID", Type: "int", Required: true},
				{Key: "Name", Type: "string", Required: false},
				{Key: "Kind", Type: "string", Required: false},
				{Key: "Type", Type: "string", Required: true},
				{Key: "UniqueID", Type: "string", Required: false},
				{Key: "FilePath", Type: "string", Required: false},
				{Key: "Size", Type: "int", Required: false},
				{Key: "Downloaded", Type: "int", Required: false},
				{Key: "Speed", Type: "int", Required: false},
				{Key: "Extra", Type: "object", Required: false, Fields: []flowengine.FieldSchema{
					{Key: "decode_key", Type: "string", Required: false},
				}},
			}},
			{Key: "basePath", Type: "string", Required: false},
			{Key: "originalExt", Type: "string", Required: false},
			{Key: "log", Type: "any", Required: false},
		},
		},
	},
	OutputSchema: []flowengine.FieldSchema{},
	Config:       map[string]interface{}{"func": taskJobUpdateNode},
}

// Postprocess performs wxchannels-specific decrypt and media conversion.
func (a *ChannelsAdapter) Postprocess(ctx context.Context, info *hermes.TaskJob, deps adapter.PostprocessDeps) error {
	log := func(msg string, args ...interface{}) {
		deps.Logger.Info().Msg(fmt.Sprintf(msg, args...))
	}
	log("Postprocessor.wxchannels: task_id=%d processing %d resources", info.ID, len(info.Resources))
	archiveRequested := isZIPOutput(info.Config)
	log("Postprocessor.wxchannels: task_id=%d config.suffix=%q archiveRequested=%v", info.ID, taskConfigSuffix(info.Config), archiveRequested)

	if err := runWXChannelsPostprocessFlow(wxchannelsPostprocessMainFlow, map[string]interface{}{
		ctxKey:                                       ctx,
		wxchannelsPostprocessContextTask:             info,
		wxchannelsPostprocessContextBasePath:         deps.BasePath,
		wxchannelsPostprocessContextArchiveRequested: archiveRequested,
		wxchannelsPostprocessRunKey: &postprocessRun{
			task:     info,
			basePath: deps.BasePath,
			log:      log,
		},
	}); err != nil {
		return fmt.Errorf("wxchannels postprocess: %w", err)
	}

	return nil
}

func isZIPOutput(config map[string]any) bool {
	return taskConfigType(config) == -1 || strings.EqualFold(taskConfigSuffix(config), ".zip")
}

func postprocessResourceName(basePath, filePath string) string {
	relPath, _ := filepath.Rel(basePath, filePath)
	if relPath != "" && !filepath.IsAbs(relPath) {
		return relPath
	}
	return filepath.Base(filePath)
}

func taskConfigType(config map[string]any) int {
	if config == nil {
		return 0
	}
	v, _ := config["type"].(int)
	if v != 0 {
		return v
	}
	switch t := config["type"].(type) {
	case float64:
		return int(t)
	case float32:
		return int(t)
	case int32:
		return int(t)
	case int64:
		return int(t)
	case int16:
		return int(t)
	case uint64:
		return int(t)
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(t))
		if err == nil {
			return i
		}
	}
	return 0
}

func taskConfigSuffix(config map[string]any) string {
	if config == nil {
		return ""
	}
	value, _ := config["suffix"].(string)
	return strings.TrimSpace(value)
}

func runWXChannelsPostprocessFlow(flow flowengine.FlowDefinition, ctx map[string]interface{}) error {
	if ctx == nil {
		ctx = map[string]interface{}{}
	}
	if _, ok := ctx[ctxKey]; !ok {
		ctx[ctxKey] = context.Background()
	}
	run, _ := postprocessRunFromContext(ctx)
	logPostprocessFlow(flow.ID, "started", run, nil)
	if run != nil && run.log != nil {
		run.log("Postprocess.wxchannels: flow=%s input=%s", flow.ID, formatContextSnapshot(postprocessContextSnapshot(ctx)))
	}
	flowEngine := flowengine.NewWorkflowEngine()
	flowEngine.RegisterNode("GatewayNode", newWXChannelsLoggedGatewayNode)
	flowEngine.SetFlowDefinitions(map[string]flowengine.FlowDefinition{
		flow.ID: flow,
	})
	instanceID, err := flowEngine.StartFlow(flow.ID, ctx)
	if err != nil {
		logPostprocessFlow(flow.ID, "failed", run, err)
		logPostprocessFlowNodeExecution(run, flowEngine, flow.ID, instanceID, err)
		return err
	}
	logPostprocessFlow(flow.ID, "succeeded", run, nil)
	logPostprocessFlowNodeExecution(run, flowEngine, flow.ID, instanceID, nil)
	return err
}

func bootstrapPostprocessContextNode(values map[string]interface{}) (interface{}, error) {
	return runWXChannelsNode(values, wxchannelsPostprocessFlowNodeBootstrapContext, func(run *postprocessRun) (interface{}, error) {
		task, ok := values[wxchannelsPostprocessContextTask].(*hermes.TaskJob)
		if !ok || task == nil {
			return nil, fmt.Errorf("缺少 task")
		}

		run.task = task
		if run.basePath == "" {
			run.basePath, _ = values[wxchannelsPostprocessContextBasePath].(string)
		}
		values[wxchannelsPostprocessRunKey] = run

		resources := make([]interface{}, 0, len(task.Resources))
		for i := range task.Resources {
			resources = append(resources, resourceToContextMap(&task.Resources[i]))
		}
		values[wxchannelsPostprocessContextResources] = resources

		values[wxchannelsPostprocessContextTaskType] = taskConfigType(task.Config)
		values[wxchannelsPostprocessContextTaskSuffix] = taskConfigSuffix(task.Config)
		return nil, nil
	})
}

func postprocessRunFromContext(values map[string]interface{}) (*postprocessRun, error) {
	run, _ := values[wxchannelsPostprocessRunKey].(*postprocessRun)
	if run == nil {
		return nil, fmt.Errorf("缺少 postprocess_run")
	}
	if run.task == nil {
		task, ok := values[wxchannelsPostprocessContextTask].(*hermes.TaskJob)
		if !ok || task == nil {
			return nil, fmt.Errorf("缺少 task")
		}
		run.task = task
	}
	if run.resource == nil {
		run.resource = resolvePostprocessResource(values[wxchannelsPostprocessContextResource])
	}
	if run.resource != nil {
		run.originalExt = strings.ToLower(filepath.Ext(run.resource.FilePath))
	}
	return run, nil
}

func prepareResourceContextNode(values map[string]interface{}) (interface{}, error) {
	return runWXChannelsNode(values, wxchannelsPostprocessFlowNodePrepareResourceContext, func(run *postprocessRun) (interface{}, error) {
		resource, hasResource := values[wxchannelsPostprocessContextResource].(*hermes.ResourceJob)
		if !hasResource {
			resource = resolvePostprocessResource(values[wxchannelsPostprocessContextResource])
		}
		if run.resource != resource {
			run.resource = resource
		}
		if run.resource == nil {
			return nil, fmt.Errorf("缺少 resource")
		}

		inputFile := ""
		if input, ok := values[ctxInputFile].(string); ok {
			inputFile = input
		}
		if inputFile == "" {
			inputFile = run.resource.FilePath
		}
		inputFile = strings.TrimSpace(inputFile)
		if inputFile == "" {
			return nil, fmt.Errorf("缺少 input_file")
		}
		values[ctxInputFile] = inputFile

		resourceType := ""
		if rt, ok := values[ctxResourceType].(string); ok {
			resourceType = rt
		}
		if resourceType == "" {
			resourceType = run.resource.Type
		}
		if resourceType == "" {
			return nil, fmt.Errorf("缺少 resource_type")
		}
		values[ctxResourceType] = resourceType

		decodeKey, ok := values[ctxDecodeKey]
		if !ok || decodeKey == nil {
			decodeKey = run.resource.Extra["decode_key"]
		}
		values[ctxDecodeKey] = decodeKey
		values[wxchannelsPostprocessContextResourceHasDecodeSecret] = decodeKeyToString(decodeKey) != ""
		if run.log != nil {
			run.log("Postprocess.wxchannels: resource_context_prepared: resource_id=%d resource_type=%q input_file=%q decode_key=%q resource_has_decode_key=%v",
				run.resource.ID, resourceType, inputFile, decodeKeyToString(decodeKey), values[wxchannelsPostprocessContextResourceHasDecodeSecret])
		}

		values[wxchannelsPostprocessContextTaskType] = taskConfigTypeFromContext(values)
		values[wxchannelsPostprocessContextTaskSuffix] = taskConfigSuffixFromContext(values)
		return nil, nil
	})
}

func noopPostprocessDoneNode(values map[string]interface{}) (interface{}, error) {
	return runWXChannelsNode(values, wxchannelsPostprocessFlowNodeDone, func(*postprocessRun) (interface{}, error) {
		return nil, nil
	})
}

func taskConfigTypeFromContext(values map[string]interface{}) int {
	if values == nil {
		return 0
	}
	v, ok := values[ctxTaskType]
	if !ok {
		return 0
	}
	if taskType, ok := v.(int); ok {
		return taskType
	}
	if taskType, ok := v.(float64); ok {
		return int(taskType)
	}
	if taskType, ok := v.(string); ok {
		parsed, err := strconv.Atoi(strings.TrimSpace(taskType))
		if err == nil {
			return parsed
		}
	}
	return 0
}

func taskConfigSuffixFromContext(values map[string]interface{}) string {
	if values == nil {
		return ""
	}
	v, ok := values[ctxTaskSuffix]
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func resolvePostprocessResource(value interface{}) *hermes.ResourceJob {
	if value == nil {
		return nil
	}
	if resource, ok := value.(*hermes.ResourceJob); ok {
		return resource
	}
	if resource, ok := value.(hermes.ResourceJob); ok {
		return &resource
	}
	if raw, ok := value.(map[string]interface{}); ok {
		return resolvePostprocessResourceFromMap(raw)
	}
	if raw, ok := value.(map[string]string); ok {
		converted := make(map[string]interface{}, len(raw))
		for key, v := range raw {
			converted[key] = v
		}
		return resolvePostprocessResourceFromMap(converted)
	}
	return nil
}

func resourceToContextMap(resource *hermes.ResourceJob) map[string]interface{} {
	if resource == nil {
		return map[string]interface{}{}
	}
	extra := make(map[string]interface{}, len(resource.Extra))
	for key, value := range resource.Extra {
		extra[key] = value
	}
	return map[string]interface{}{
		"ID":         resource.ID,
		"Name":       resource.Name,
		"Kind":       resource.Kind,
		"Type":       resource.Type,
		"UniqueID":   resource.UniqueID,
		"FilePath":   resource.FilePath,
		"Size":       resource.Size,
		"Downloaded": resource.Downloaded,
		"Speed":      resource.Speed,
		"Extra":      extra,
	}
}

func resolvePostprocessResourceFromMap(raw map[string]interface{}) *hermes.ResourceJob {
	resource := &hermes.ResourceJob{}
	resource.ID = castInt(raw["ID"])
	resource.Name = castString(raw["Name"])
	resource.Kind = castString(raw["Kind"])
	resource.Type = castString(raw["Type"])
	resource.UniqueID = castString(raw["UniqueID"])
	resource.FilePath = castString(raw["FilePath"])
	resource.Size = castInt64(raw["Size"])
	resource.Downloaded = castInt64(raw["Downloaded"])
	resource.Speed = castInt64(raw["Speed"])
	resource.Extra = map[string]string{}

	if rawExtra, ok := raw["Extra"].(map[string]interface{}); ok {
		for key, value := range rawExtra {
			resource.Extra[key] = castString(value)
		}
	} else if rawExtra, ok := raw["Extra"].(map[string]string); ok {
		for key, value := range rawExtra {
			resource.Extra[key] = value
		}
	}
	return resource
}

func castInt64(value interface{}) int64 {
	switch typed := value.(type) {
	case nil:
		return 0
	case int:
		return int64(typed)
	case int32:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	case uint64:
		return int64(typed)
	case uint:
		return int64(typed)
	case uint32:
		return int64(typed)
	case string:
		if v, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64); err == nil {
			return v
		}
	}
	return 0
}

func castInt(value interface{}) int {
	return int(castInt64(value))
}

func castString(value interface{}) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	switch typed := value.(type) {
	case fmt.Stringer:
		return typed.String()
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	}
}

func nodeContext(values map[string]interface{}) context.Context {
	if v, ok := values[ctxKey].(context.Context); ok && v != nil {
		return v
	}
	return context.Background()
}

func decryptNode(values map[string]interface{}) (interface{}, error) {
	return runWXChannelsNode(values, wxchannelsPostprocessFlowNodeDecrypt, func(run *postprocessRun) (interface{}, error) {
		inputFile, _ := values[ctxInputFile].(string)
		if inputFile == "" {
			return nil, fmt.Errorf("缺少 input_file")
		}
		if run.task == nil || run.resource == nil {
			return nil, fmt.Errorf("缺少 task 或 resource")
		}

		decodeKeyStr := strings.TrimSpace(decodeKeyToString(values[ctxDecodeKey]))
		key, err := strconv.ParseUint(decodeKeyStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("解析 decode_key 失败: %w", err)
		}

		tmpFile := inputFile + ".tmp"
		if err := wxchannels.DecryptFile(inputFile, tmpFile, key, 131072); err != nil {
			_ = os.Remove(tmpFile)
			return nil, err
		}
		if err := os.Rename(tmpFile, inputFile); err != nil {
			_ = os.Remove(tmpFile)
			return nil, fmt.Errorf("原地替换解密文件失败: %w", err)
		}
		values[ctxDecryptedFile] = inputFile
		return nil, nil
	})
}

func decodeKeyToString(value interface{}) string {
	switch key := value.(type) {
	case string:
		return key
	case int:
		return strconv.Itoa(key)
	case int32:
		return strconv.Itoa(int(key))
	case int64:
		return strconv.FormatInt(key, 10)
	case uint64:
		return strconv.FormatUint(key, 10)
	case float64:
		return strconv.FormatInt(int64(key), 10)
	case float32:
		return strconv.FormatInt(int64(key), 10)
	}
	return ""
}

func convertMP3Node(values map[string]interface{}) (interface{}, error) {
	return runWXChannelsNode(values, wxchannelsPostprocessFlowNodeConvertMP3, func(*postprocessRun) (interface{}, error) {
		decryptedFile, _ := values[ctxDecryptedFile].(string)
		if decryptedFile == "" {
			return nil, fmt.Errorf("缺少 decrypted_file")
		}

		baseName := filepath.Base(decryptedFile)
		ext := filepath.Ext(baseName)
		mp3File := filepath.Join(filepath.Dir(decryptedFile), strings.TrimSuffix(baseName, ext))
		tmpFile := mp3File + ".converting"

		cmd := exec.CommandContext(
			nodeContext(values),
			"ffmpeg",
			"-i", decryptedFile,
			"-vn",
			"-acodec", "libmp3lame",
			"-ab", "192k",
			"-f", "mp3",
			"-y",
			tmpFile,
		)
		output, err := cmd.CombinedOutput()
		if err != nil {
			_ = os.Remove(tmpFile)
			return nil, fmt.Errorf("ffmpeg 转换失败: %w\n%s", err, string(output))
		}
		if err := replaceConvertedFile(decryptedFile, tmpFile, mp3File); err != nil {
			return nil, fmt.Errorf("替换 MP3 转换文件失败: %w", err)
		}
		values[ctxMP3File] = mp3File
		return nil, nil
	})
}

func finalizeMP3Node(values map[string]interface{}) (interface{}, error) {
	return runWXChannelsNode(values, wxchannelsPostprocessFlowNodeFinalizeMP3, func(run *postprocessRun) (interface{}, error) {
		mp3File, _ := values[ctxMP3File].(string)
		if mp3File == "" {
			return nil, fmt.Errorf("缺少 mp3_file")
		}
		if run.resource == nil {
			return nil, fmt.Errorf("缺少 resource")
		}

		if run.resource.FilePath != mp3File {
			_ = os.Remove(run.resource.FilePath)
		}
		run.resource.Kind = "audio/mpeg"
		if title := run.resource.Extra["title"]; title != "" {
			run.resource.Name = sanitizeBGMName(title)
		} else {
			run.resource.Name = postprocessResourceName(run.basePath, mp3File)
		}
		run.resource.FilePath = mp3File
		return nil, nil
	})
}

func streamConvertNode(values map[string]interface{}) (interface{}, error) {
	return runWXChannelsNode(values, wxchannelsPostprocessFlowNodeStreamConvert, func(run *postprocessRun) (interface{}, error) {
		inputFile, _ := values[ctxInputFile].(string)
		if inputFile == "" {
			return nil, fmt.Errorf("缺少 input_file")
		}
		if run.task == nil || run.resource == nil {
			return nil, fmt.Errorf("缺少 task 或 resource")
		}

		ctx := nodeContext(values)
		videoCodec, err := probeFirstMediaCodec(ctx, inputFile, "v:0")
		if err != nil {
			return nil, fmt.Errorf("检测直播视频编码失败: %w", err)
		}
		if videoCodec == "" {
			return nil, fmt.Errorf("直播文件不包含视频轨道")
		}
		audioCodec, err := probeFirstMediaCodec(ctx, inputFile, "a:0")
		if err != nil {
			return nil, fmt.Errorf("检测直播音频编码失败: %w", err)
		}

		ext := strings.ToLower(filepath.Ext(inputFile))
		baseName := filepath.Base(inputFile)
		mp4File := inputFile
		if ext != ".mp4" {
			mp4File = filepath.Join(filepath.Dir(inputFile), strings.TrimSuffix(baseName, ext)+".mp4")
		}
		tmpFile := mp4File + ".converting"

		cmd := exec.CommandContext(ctx, "ffmpeg", buildStreamMP4Args(inputFile, tmpFile, videoCodec, audioCodec)...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			_ = os.Remove(tmpFile)
			return nil, fmt.Errorf("ffmpeg stream remux 失败: %w\n%s", err, string(output))
		}
		if err := replaceConvertedFile(inputFile, tmpFile, mp4File); err != nil {
			return nil, fmt.Errorf("替换 MP4 转换文件失败: %w", err)
		}

		values[ctxMP4File] = mp4File
		return nil, nil
	})
}

func probeFirstMediaCodec(ctx context.Context, inputFile, selector string) (string, error) {
	cmd := exec.CommandContext(
		ctx,
		"ffprobe",
		"-v", "error",
		"-select_streams", selector,
		"-show_entries", "stream=codec_name",
		"-of", "default=noprint_wrappers=1:nokey=1",
		inputFile,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ffprobe 失败: %w\n%s", err, string(output))
	}
	codec, _, _ := strings.Cut(strings.TrimSpace(string(output)), "\n")
	return strings.ToLower(strings.TrimSpace(codec)), nil
}

func buildStreamMP4Args(inputFile, outputFile, videoCodec, audioCodec string) []string {
	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-fflags", "+genpts",
		"-i", inputFile,
		"-map", "0:v:0",
		"-map", "0:a:0?",
		"-map_metadata", "0",
	}

	switch strings.ToLower(strings.TrimSpace(videoCodec)) {
	case "hevc", "h265":
		// Apple system players require the hvc1 sample entry for reliable HEVC
		// playback. This only rewrites container metadata and preserves quality.
		args = append(args, "-c:v", "copy", "-tag:v", "hvc1")
	case "h264", "avc":
		args = append(args, "-c:v", "copy")
	default:
		// Fall back to the broadly supported H.264/yuv420p combination when the
		// source codec cannot be played directly by common system players.
		args = append(args,
			"-c:v", "libx264",
			"-preset", "medium",
			"-crf", "20",
			"-pix_fmt", "yuv420p",
			"-fps_mode", "vfr",
		)
	}

	switch strings.ToLower(strings.TrimSpace(audioCodec)) {
	case "":
	case "aac":
		args = append(args, "-c:a", "copy", "-bsf:a", "aac_adtstoasc")
	default:
		args = append(args, "-c:a", "aac", "-b:a", "192k")
	}

	return append(args,
		"-avoid_negative_ts", "make_zero",
		"-video_track_timescale", "90000",
		"-movflags", "+faststart",
		"-f", "mp4",
		"-y",
		outputFile,
	)
}

func finalizeStreamNode(values map[string]interface{}) (interface{}, error) {
	return runWXChannelsNode(values, wxchannelsPostprocessFlowNodeFinalizeStream, func(run *postprocessRun) (interface{}, error) {
		if run.resource == nil {
			return nil, fmt.Errorf("缺少 resource")
		}
		mp4File, _ := values[ctxMP4File].(string)
		if mp4File == "" {
			return nil, fmt.Errorf("缺少 mp4_file")
		}
		run.resource.FilePath = mp4File
		if strings.TrimSpace(run.resource.Name) == "" {
			run.resource.Name = postprocessResourceName(run.basePath, mp4File)
		}
		run.resource.Kind = "video/mp4"
		return nil, nil
	})
}

func taskJobUpdateNode(values map[string]interface{}) (interface{}, error) {
	return runWXChannelsNode(values, wxchannelsPostprocessFlowNodeTaskJobUpdate, func(run *postprocessRun) (interface{}, error) {
		if run.task == nil {
			return nil, fmt.Errorf("缺少 task")
		}
		if run.resource == nil {
			for i := range run.task.Resources {
				if err := updatePostprocessedResource(&run.task.Resources[i]); err != nil {
					return nil, fmt.Errorf("更新 TaskJob resource[%d]: %w", i, err)
				}
			}
			return nil, nil
		}

		for i := range run.task.Resources {
			target := &run.task.Resources[i]
			if target == run.resource || (run.resource.ID > 0 && target.ID == run.resource.ID) ||
				(run.resource.UniqueID != "" && target.UniqueID == run.resource.UniqueID) {
				if target != run.resource {
					*target = *run.resource
					run.resource = target
				}
				if err := updatePostprocessedResource(target); err != nil {
					return nil, fmt.Errorf("更新 TaskJob resource[%d]: %w", i, err)
				}
				return nil, nil
			}
		}
		return nil, fmt.Errorf("TaskJob 中找不到 postprocess resource id=%d unique_id=%q", run.resource.ID, run.resource.UniqueID)
	})
}

func updatePostprocessedResource(resource *hermes.ResourceJob) error {
	if hermes.CanonicalExtensionForMIMEType(resource.Kind) == "" {
		return fmt.Errorf("resource kind %q 不是可映射的 MIME type", resource.Kind)
	}
	if resource.FilePath == "" {
		return nil
	}
	stat, err := os.Stat(resource.FilePath)
	if err != nil {
		return fmt.Errorf("读取最终资源 %q 信息失败: %w", resource.FilePath, err)
	}
	resource.Size = stat.Size()
	return nil
}

func zipResourcesNode(values map[string]interface{}) (interface{}, error) {
	return runWXChannelsNode(values, wxchannelsPostprocessFlowNodeZipResources, func(run *postprocessRun) (interface{}, error) {
		if run.task == nil {
			return nil, fmt.Errorf("缺少 task")
		}

		archiveUniqueID := strings.TrimSpace(run.task.UniqueID) + "_zip"
		if archiveUniqueID == "_zip" {
			archiveUniqueID = fmt.Sprintf("task_%d_zip", run.task.ID)
		}
		archivePath := filepath.Join(run.basePath, run.task.SavePath, archiveUniqueID)
		for _, resource := range run.task.Resources {
			if filepath.Clean(resource.FilePath) == filepath.Clean(archivePath) {
				return nil, fmt.Errorf("ZIP 输出路径与资源输入路径冲突: %s", archivePath)
			}
		}
		if err := os.MkdirAll(filepath.Dir(archivePath), 0755); err != nil {
			return nil, fmt.Errorf("创建 ZIP 输出目录失败: %w", err)
		}
		if err := writeResourcesZIP(archivePath, run.task.Resources); err != nil {
			return nil, err
		}

		for _, resource := range run.task.Resources {
			if resource.FilePath == "" {
				continue
			}
			if err := os.Remove(resource.FilePath); err != nil && !os.IsNotExist(err) {
				return nil, fmt.Errorf("ZIP 已生成，但删除源文件 %q 失败: %w", resource.FilePath, err)
			}
		}

		archiveName := strings.TrimSpace(run.task.Name)
		if archiveName == "" {
			archiveName = "archive"
		}
		archiveResource := hermes.ResourceJob{
			Name:     archiveName,
			Kind:     "application/zip",
			Type:     "FILE",
			UniqueID: archiveUniqueID,
			FilePath: archivePath,
			Extra:    map[string]string{"title": archiveName},
		}
		if len(run.task.Resources) > 0 {
			archiveResource.ID = run.task.Resources[0].ID
		}
		if stat, err := os.Stat(archivePath); err == nil {
			archiveResource.Size = stat.Size()
		}
		run.task.Resources = []hermes.ResourceJob{archiveResource}
		values["archive_file"] = archivePath
		return nil, nil
	})
}

func writeResourcesZIP(archivePath string, resources []hermes.ResourceJob) error {
	tmpFile := archivePath + ".archiving"
	_ = os.Remove(tmpFile)
	file, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("创建 ZIP 临时文件失败: %w", err)
	}
	writer := zip.NewWriter(file)
	entryNames := make(map[string]int, len(resources))

	fail := func(cause error) error {
		_ = writer.Close()
		_ = file.Close()
		_ = os.Remove(tmpFile)
		return cause
	}
	for i, resource := range resources {
		if resource.FilePath == "" {
			return fail(fmt.Errorf("resource[%d] 缺少 file_path", i))
		}
		input, err := os.Open(resource.FilePath)
		if err != nil {
			return fail(fmt.Errorf("打开 ZIP 源文件 %q 失败: %w", resource.FilePath, err))
		}
		info, err := input.Stat()
		if err != nil {
			_ = input.Close()
			return fail(fmt.Errorf("读取 ZIP 源文件信息 %q 失败: %w", resource.FilePath, err))
		}
		if !info.Mode().IsRegular() {
			_ = input.Close()
			return fail(fmt.Errorf("ZIP 源文件不是普通文件: %s", resource.FilePath))
		}

		entryName := uniqueZIPEntryName(resource, i, entryNames)
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			_ = input.Close()
			return fail(fmt.Errorf("创建 ZIP entry %q 失败: %w", entryName, err))
		}
		header.Name = entryName
		header.Method = zip.Deflate
		entry, err := writer.CreateHeader(header)
		if err != nil {
			_ = input.Close()
			return fail(fmt.Errorf("写入 ZIP entry %q 失败: %w", entryName, err))
		}
		_, copyErr := io.Copy(entry, input)
		closeErr := input.Close()
		if copyErr != nil {
			return fail(fmt.Errorf("压缩文件 %q 失败: %w", resource.FilePath, copyErr))
		}
		if closeErr != nil {
			return fail(fmt.Errorf("关闭 ZIP 源文件 %q 失败: %w", resource.FilePath, closeErr))
		}
	}
	if err := writer.Close(); err != nil {
		_ = file.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("完成 ZIP 数据写入失败: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("关闭 ZIP 临时文件失败: %w", err)
	}
	if err := os.Remove(archivePath); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("删除已有 ZIP 输出失败: %w", err)
	}
	if err := os.Rename(tmpFile, archivePath); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("提交 ZIP 输出失败: %w", err)
	}
	return nil
}

func uniqueZIPEntryName(resource hermes.ResourceJob, index int, seen map[string]int) string {
	name := filepath.Base(strings.TrimSpace(resource.Name))
	if name == "." || name == "" {
		name = filepath.Base(resource.FilePath)
	}
	if name == "." || name == "" {
		name = fmt.Sprintf("resource_%d", index+1)
	}
	if filepath.Ext(name) == "" && resource.Kind != "" {
		if ext := hermes.CanonicalExtensionForMIMEType(resource.Kind); ext != "" {
			name += ext
		}
	}
	seen[name]++
	if seen[name] == 1 {
		return name
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	return fmt.Sprintf("%s_%d%s", base, seen[name], ext)
}

// replaceConvertedFile replaces inputFile with the completed conversion. The
// temporary output is kept when the final rename fails so it can be recovered.
func replaceConvertedFile(inputFile, tmpFile, finalFile string) error {
	if finalFile != inputFile {
		if err := os.Remove(finalFile); err != nil && !os.IsNotExist(err) {
			_ = os.Remove(tmpFile)
			return fmt.Errorf("删除已有输出文件失败: %w", err)
		}
	}
	if err := os.Remove(inputFile); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("删除原文件失败: %w", err)
	}
	if err := os.Rename(tmpFile, finalFile); err != nil {
		return fmt.Errorf("重命名临时文件 %q 为 %q 失败: %w", tmpFile, finalFile, err)
	}
	return nil
}
