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
	wxchannels_postprocess_run_flow_id    = "wxchannels_postprocess"
	wxchannels_postprocess_main_flow_id   = "wxchannels_postprocess_main"
	wxchannels_postprocess_output_flow_id = "wxchannels_postprocess_output"

	wxchannels_postprocess_run_key = "postprocess_run"

	ctx_key                        = "context"
	ctx_input_file                 = "input_file"
	ctx_decode_key                 = "decode_key"
	ctx_decrypted_file             = "decrypted_file"
	ctx_mp3_file                   = "mp3_file"
	ctx_mp4_file                   = "mp4_file"
	ctx_task_type                  = "task_config_type"
	ctx_task_suffix                = "task_config_suffix"
	ctx_resource_type              = "resource_type"
	ctx_resource_has_decode_secret = "resource_has_decode_key"
	ctx_resource                   = "resource"
	ctx_resources                  = "resources"
	ctx_archive_requested          = "archive_requested"

	wxchannels_postprocess_context_key_ctx                    = ctx_key
	wxchannels_postprocess_context_input_file                 = ctx_input_file
	wxchannels_postprocess_context_decode_key                 = ctx_decode_key
	wxchannels_postprocess_context_decrypted_file             = ctx_decrypted_file
	wxchannels_postprocess_context_mp3_file                   = ctx_mp3_file
	wxchannels_postprocess_context_mp4_file                   = ctx_mp4_file
	wxchannels_postprocess_context_task_type                  = ctx_task_type
	wxchannels_postprocess_context_task_suffix                = ctx_task_suffix
	wxchannels_postprocess_context_resource_type              = ctx_resource_type
	wxchannels_postprocess_context_resource_has_decode_secret = ctx_resource_has_decode_secret
	wxchannels_postprocess_context_resource                   = ctx_resource
	wxchannels_postprocess_context_task                       = "task"
	wxchannels_postprocess_context_resources                  = ctx_resources
	wxchannels_postprocess_context_archive_requested          = ctx_archive_requested
	wxchannels_postprocess_context_base_path                  = "base_path"

	wxchannels_postprocess_node_status_started                = "started"
	wxchannels_postprocess_node_status_succeeded              = "succeeded"
	wxchannels_postprocess_node_status_failed                 = "failed"
	wxchannels_postprocess_flow_node_bootstrap_context        = "bootstrap_context"
	wxchannels_postprocess_flow_node_prepare_resource_context = "prepare_resource_context"
	wxchannels_postprocess_flow_node_decrypt                  = "decrypt"
	wxchannels_postprocess_flow_node_convert_mp3              = "convert_mp3"
	wxchannels_postprocess_flow_node_finalize_mp3             = "finalize_mp3"
	wxchannels_postprocess_flow_node_stream_convert           = "stream_convert"
	wxchannels_postprocess_flow_node_finalize_stream          = "finalize_stream"
	wxchannels_postprocess_flow_node_zip_resources            = "zip_resources"
	wxchannels_postprocess_flow_node_task_job_update          = "task_job_update"
	wxchannels_postprocess_flow_node_done                     = "done"
)

type postprocess_run struct {
	task         *hermes.TaskJob
	resource     *hermes.ResourceJob
	base_path    string
	original_ext string
	log          func(string, ...interface{})
}

const wxchannels_postprocess_log_value_max_len = 260

func run_wx_channels_node(values map[string]interface{}, node_id string, fn func(*postprocess_run) (interface{}, error)) (interface{}, error) {
	run, err := postprocess_run_from_context(values)
	if err != nil {
		return nil, err
	}
	input := postprocess_context_snapshot(values)
	log_postprocess_node(run, node_id, wxchannels_postprocess_node_status_started, input, nil, nil, nil)
	result, err := fn(run)
	output := postprocess_context_snapshot(values)
	diff := postprocess_context_diff(input, output)
	if err != nil {
		log_postprocess_node(run, node_id, wxchannels_postprocess_node_status_failed, input, output, diff, err)
		return result, err
	}
	log_postprocess_node(run, node_id, wxchannels_postprocess_node_status_succeeded, input, output, diff, nil)
	return result, nil
}

func log_postprocess_node(run *postprocess_run, node_id, status string, input map[string]interface{}, output map[string]interface{}, delta map[string][2]interface{}, err error) {
	if run == nil || run.log == nil {
		return
	}

	task_id := 0
	resource_id := 0
	resource_type := ""
	resource_path := ""
	if run.task != nil {
		task_id = run.task.ID
	}
	if run.resource != nil {
		resource_id = run.resource.ID
		resource_type = run.resource.Type
		resource_path = run.resource.FilePath
	}
	if err != nil {
		run.log(
			"Postprocess.wxchannels: node=%s status=%s task_id=%d resource_id=%d resource_type=%q resource_path=%q error=%v input=%s output=%s changed=%s",
			node_id, status, task_id, resource_id, resource_type, resource_path, err,
			format_context_snapshot(input), format_context_snapshot(output), format_context_diff(delta),
		)
		return
	}
	run.log(
		"Postprocess.wxchannels: node=%s status=%s task_id=%d resource_id=%d resource_type=%q resource_path=%q input=%s output=%s changed=%s",
		node_id, status, task_id, resource_id, resource_type, resource_path,
		format_context_snapshot(input), format_context_snapshot(output), format_context_diff(delta),
	)
}

func log_postprocess_flow(flow_id string, status string, run *postprocess_run, err error) {
	if run == nil || run.log == nil {
		return
	}

	task_id := 0
	if run.task != nil {
		task_id = run.task.ID
	}
	if err != nil {
		run.log("Postprocess.wxchannels: flow=%s status=%s task_id=%d error=%v", flow_id, status, task_id, err)
		return
	}
	run.log("Postprocess.wxchannels: flow=%s status=%s task_id=%d", flow_id, status, task_id)
}

func postprocess_context_snapshot(values map[string]interface{}) map[string]interface{} {
	snapshot := map[string]interface{}{}
	if len(values) == 0 {
		return snapshot
	}

	for key, value := range values {
		if key == wxchannels_postprocess_run_key || key == wxchannels_postprocess_context_key_ctx {
			continue
		}
		snapshot[key] = normalize_postprocess_context_value(key, value)
	}
	return snapshot
}

func normalize_postprocess_context_value(key string, value interface{}) interface{} {
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
			"config_type":   task_config_type(typed.Config),
			"config_suffix": task_config_suffix(typed.Config),
		}
	case hermes.TaskJob:
		return normalize_postprocess_context_value(key, &typed)
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
		return normalize_postprocess_context_value(key, &typed)
	case []hermes.ResourceJob:
		items := make([]interface{}, 0, len(typed))
		for _, resource := range typed {
			items = append(items, normalize_postprocess_context_value("resource", resource))
		}
		return normalize_generic_slice(items)
	case []interface{}:
		if key == wxchannels_postprocess_context_resources {
			items := make([]interface{}, 0, len(typed))
			for _, value := range typed {
				items = append(items, normalize_postprocess_context_value("resource", value))
			}
			return normalize_generic_slice(items)
		}
		return normalize_generic_slice(typed)
	case map[string]interface{}:
		if key == wxchannels_postprocess_context_resource {
			return normalize_postprocess_resource_map(typed)
		}
		return normalize_generic_map(typed)
	case map[string]string:
		return normalize_generic_map_string(typed)
	default:
		return truncate_log_value(fmt.Sprintf("%v", value))
	}
}

func normalize_postprocess_resource_map(raw map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"id":                cast_int(raw["ID"]),
		"name":              cast_string(raw["Name"]),
		"type":              cast_string(raw["Type"]),
		"kind":              cast_string(raw["Kind"]),
		"file":              cast_string(raw["FilePath"]),
		"size":              cast_int64(raw["Size"]),
		"decode_key_exists": postprocess_resource_map_has_decode_key(raw),
	}
}

func postprocess_resource_map_has_decode_key(raw map[string]interface{}) bool {
	if raw == nil {
		return false
	}
	if raw_decode_key, ok := raw["decode_key"]; ok {
		return cast_string(raw_decode_key) != ""
	}
	if raw_extra, ok := raw["Extra"].(map[string]interface{}); ok {
		return cast_string(raw_extra["decode_key"]) != ""
	}
	extra, ok := raw["Extra"].(map[string]string)
	if !ok {
		return false
	}
	return extra["decode_key"] != ""
}

func normalize_generic_slice(values []interface{}) []interface{} {
	max_items := 8
	if len(values) <= max_items {
		return values
	}
	items := append([]interface{}{}, values[:max_items]...)
	items = append(items, map[string]interface{}{
		"_truncated": len(values) - max_items,
	})
	return items
}

func normalize_generic_map(values map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(values))
	for key, value := range values {
		out[key] = truncate_log_value(fmt.Sprintf("%v", value))
	}
	return out
}

func normalize_generic_map_string(values map[string]string) map[string]interface{} {
	out := make(map[string]interface{}, len(values))
	for key, value := range values {
		out[key] = truncate_log_value(value)
	}
	return out
}

func postprocess_context_diff(before map[string]interface{}, after map[string]interface{}) map[string][2]interface{} {
	delta := map[string][2]interface{}{}
	if len(before) == 0 && len(after) == 0 {
		return delta
	}

	for key, before_value := range before {
		if after_value, ok := after[key]; ok {
			if !reflect.DeepEqual(before_value, after_value) {
				delta[key] = [2]interface{}{before_value, after_value}
			}
		} else {
			delta[key] = [2]interface{}{before_value, nil}
		}
	}
	for key, after_value := range after {
		if _, ok := before[key]; !ok {
			delta[key] = [2]interface{}{nil, after_value}
		}
	}
	return delta
}

func format_context_snapshot(snapshot map[string]interface{}) string {
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
		parts = append(parts, fmt.Sprintf("%s=%s", key, truncate_log_value(fmt.Sprintf("%v", snapshot[key]))))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func format_context_diff(delta map[string][2]interface{}) string {
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
		from := truncate_log_value(fmt.Sprintf("%v", values[0]))
		to := truncate_log_value(fmt.Sprintf("%v", values[1]))
		parts = append(parts, fmt.Sprintf("%s[from=%s,to=%s]", key, from, to))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func truncate_log_value(value string) string {
	if len(value) <= wxchannels_postprocess_log_value_max_len {
		return value
	}
	return value[:wxchannels_postprocess_log_value_max_len] + "..."
}

type wxchannels_logged_gateway_node struct {
	id_val string
	config map[string]interface{}
}

func new_wx_channels_logged_gateway_node(config map[string]interface{}) flowengine.Node {
	id, _ := config["id"].(string)
	return &wxchannels_logged_gateway_node{
		id_val: id,
		config: config,
	}
}

func (n *wxchannels_logged_gateway_node) ID() string { return n.id_val }

func (n *wxchannels_logged_gateway_node) Type() string { return "GatewayNode" }

func (n *wxchannels_logged_gateway_node) Execute(ctx *flowengine.ProcessContext) (bool, []string, error) {
	if ctx == nil {
		return false, nil, errors.New("gateway context is nil")
	}
	input := format_context_snapshot(postprocess_context_snapshot(ctx.Data))
	n.logf(ctx, "gateway=%s type=%s started decision_input=%s", n.id_val, n.config_value("gateway_type"), input)

	gateway_type, _ := n.config["gateway_type"].(string)
	is_joining := false
	if joining, ok := n.config["is_joining"].(bool); ok {
		is_joining = joining
	}

	if is_joining {
		return n.handle_merge(ctx, input)
	}

	switch gateway_type {
	case "Parallel":
		next := n.next_node_ids()
		n.logf(ctx, "gateway=%s type=Parallel next=%v", n.id_val, next)
		return true, next, nil
	case "Exclusive":
		rules := n.rules()
		for _, rule := range rules {
			condition, ok := rule["condition"].(string)
			if !ok {
				continue
			}
			ok, err := n.evaluate_condition(ctx, condition)
			if err != nil {
				n.logf(ctx, "gateway=%s error condition=%q target=%v err=%v", n.id_val, condition, rule["target_id"], err)
				return false, nil, err
			}
			n.logf(ctx, "gateway=%s condition=%q result=%v target=%v", n.id_val, condition, ok, rule["target_id"])
			if ok {
				target := cast_string(rule["target_id"])
				ctx.Mu.Lock()
				ctx.Data[n.ID()+"_target"] = target
				ctx.Mu.Unlock()
				n.logf(ctx, "gateway=%s selected=%q", n.id_val, target)
				return true, []string{target}, nil
			}
		}
		return false, nil, errors.New("Exclusive Gateway failed to find a valid path")
	default:
		return false, nil, errors.New("GatewayNode: unknown gateway_type")
	}
}

func (n *wxchannels_logged_gateway_node) handle_merge(ctx *flowengine.ProcessContext, input string) (bool, []string, error) {
	wait_list_raw, ok := n.config["wait_for_incoming"]
	if !ok {
		return false, nil, errors.New("GatewayNode merge requires wait_for_incoming")
	}
	wait_list, err := cast_string_slice(wait_list_raw)
	if err != nil {
		return false, nil, err
	}

	all_completed := true
	for _, dep_id := range wait_list {
		ctx.Mu.Lock()
		dep_state := ctx.NodeStates[dep_id]
		ctx.Mu.Unlock()
		if dep_state != flowengine.StateCompleted {
			all_completed = false
			break
		}
	}
	if !all_completed {
		ctx.Mu.Lock()
		ctx.NodeStates[n.ID()] = flowengine.StateWaitingForMerge
		ctx.Mu.Unlock()
		n.logf(ctx, "gateway=%s waiting merge input=%s", n.id_val, input)
		return true, nil, nil
	}

	next := n.next_node_ids()
	n.logf(ctx, "gateway=%s merge_done next=%v", n.id_val, next)
	return true, next, nil
}

func (n *wxchannels_logged_gateway_node) next_node_ids() []string {
	if next_node_ids, ok := n.config["next_node_ids"].([]string); ok {
		return append([]string{}, next_node_ids...)
	}

	if raw_targets, ok := n.config["next_nodes"].([]flowengine.TargetNode); ok {
		targets := make([]string, 0, len(raw_targets))
		for _, target := range raw_targets {
			targets = append(targets, target.TargetID)
		}
		return targets
	}

	if raw_targets, ok := n.config["next_nodes"].([]interface{}); ok {
		targets := make([]string, 0, len(raw_targets))
		for _, raw := range raw_targets {
			if mapped, ok := raw.(map[string]interface{}); ok {
				if target_id, ok := mapped["target_id"].(string); ok {
					targets = append(targets, target_id)
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

func (n *wxchannels_logged_gateway_node) rules() []map[string]interface{} {
	if raw, ok := n.config["rules"].([]map[string]interface{}); ok {
		return raw
	}
	if raw, ok := n.config["rules"].([]interface{}); ok {
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

func (n *wxchannels_logged_gateway_node) config_value(key string) string {
	if value, ok := n.config[key]; ok {
		return cast_string(value)
	}
	return ""
}

func (n *wxchannels_logged_gateway_node) evaluate_condition(ctx *flowengine.ProcessContext, condition string) (bool, error) {
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

func (n *wxchannels_logged_gateway_node) logf(ctx *flowengine.ProcessContext, format string, args ...interface{}) {
	if ctx == nil || n == nil {
		return
	}
	run, ok := ctx.Data[wxchannels_postprocess_run_key].(*postprocess_run)
	if !ok || run == nil || run.log == nil {
		return
	}
	run.log("Postprocess.wxchannels: "+format, args...)
}

func cast_string_slice(value interface{}) ([]string, error) {
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

func log_postprocess_flow_node_execution(run *postprocess_run, flow_engine *flowengine.FlowEngine, flow_id, instance_id string, err error) {
	if run == nil || run.log == nil || flow_engine == nil || instance_id == "" {
		return
	}

	record, has_record := flow_engine.GetRunSnapshot(instance_id)
	if !has_record {
		if err != nil {
			run.log("Postprocess.wxchannels: flow=%s node_execution_summary unavailable: %v", flow_id, err)
		} else {
			run.log("Postprocess.wxchannels: flow=%s node_execution_summary unavailable", flow_id)
		}
		return
	}

	_, node_states := flow_engine.GetRunContext(instance_id)

	node_ids := make([]string, 0, len(record.NodeAttempts))
	for node_id := range record.NodeAttempts {
		node_ids = append(node_ids, node_id)
	}
	for node_id := range node_states {
		already := false
		for _, existing_id := range node_ids {
			if existing_id == node_id {
				already = true
				break
			}
		}
		if !already {
			node_ids = append(node_ids, node_id)
		}
	}
	if len(node_ids) == 0 {
		run.log("Postprocess.wxchannels: flow=%s node_execution_summary: no nodes were executed", flow_id)
		return
	}

	sort.Strings(node_ids)
	for _, node_id := range node_ids {
		state := "unknown"
		attempts := record.NodeAttempts[node_id]

		if attempts == 0 {
			state = "skipped"
		}

		if len(node_states) > 0 {
			if node_state, ok := node_states[node_id]; ok {
				switch node_state {
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
					state = string(node_state)
				}
			}
		}

		// 回退推断：没有节点状态时，以流执行结果判断最后一个活跃节点是否失败。
		if state == "unknown" && attempts > 0 && record.Status == flowengine.RunStatusFailed && record.CurrentNode == node_id {
			state = "failed"
		}
		if state == "unknown" && attempts > 0 && record.Status == flowengine.RunStatusCompleted {
			state = "succeeded"
		}

		task_suffix := ""
		task_suffix_info := ""
		if attempts > 0 {
			task_suffix = fmt.Sprintf(" attempts=%d", attempts)
		}
		if state == "failed" && record.CurrentNode == node_id && record.Error != "" {
			task_suffix_info = " error=" + record.Error
		}
		run.log("Postprocess.wxchannels: flow=%s node=%s status=%s%s%s", flow_id, node_id, state, task_suffix, task_suffix_info)
	}

	if record.Error != "" && record.CurrentNode != "" {
		run.log("Postprocess.wxchannels: flow=%s failed_at_node=%s error=%v", flow_id, record.CurrentNode, record.Error)
	}
	if record.Status != "" {
		run.log("Postprocess.wxchannels: flow=%s final_status=%s", flow_id, record.Status)
	}
}

var wxchannels_postprocess_flow = flowengine.FlowDefinition{
	ID:   wxchannels_postprocess_run_flow_id,
	Name: "wxchannels_postprocess",
	ContextSchema: []flowengine.FieldSchema{
		{Key: wxchannels_postprocess_context_key_ctx, Type: "object", Required: false},
		{Key: wxchannels_postprocess_context_resource, Type: "object", Required: false, Fields: []flowengine.FieldSchema{
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
		{Key: ctx_input_file, Type: "string", Required: true},
		{Key: ctx_decode_key, Type: "any", Required: false},
		{Key: ctx_resource_type, Type: "string", Required: true},
		{Key: ctx_resource_has_decode_secret, Type: "bool", Required: false},
		{Key: ctx_task_type, Type: "int", Required: false},
		{Key: ctx_task_suffix, Type: "string", Required: false},
		{Key: wxchannels_postprocess_run_key, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
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
				{Key: wxchannels_postprocess_run_key, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
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
				{Key: wxchannels_postprocess_context_resource, Type: "object", Required: false, Fields: []flowengine.FieldSchema{
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
				{Key: ctx_input_file, Type: "string", Required: false},
				{Key: ctx_resource_type, Type: "string", Required: false},
				{Key: ctx_decode_key, Type: "any", Required: false},
				{Key: ctx_task_type, Type: "int", Required: false},
				{Key: ctx_task_suffix, Type: "string", Required: false},
			},
			OutputSchema: []flowengine.FieldSchema{
				{Key: ctx_input_file, Type: "string", Required: true},
				{Key: ctx_resource_type, Type: "string", Required: true},
				{Key: wxchannels_postprocess_context_resource_has_decode_secret, Type: "bool", Required: true},
				{Key: ctx_decode_key, Type: "any", Required: false},
				{Key: ctx_task_type, Type: "int", Required: false},
				{Key: ctx_task_suffix, Type: "string", Required: false},
			},
			Config:    map[string]interface{}{"func": prepare_resource_context_node},
			NextNodes: []flowengine.TargetNode{{TargetID: "route_resource"}},
		},
		"route_resource": {
			ID:   "route_resource",
			Type: "GatewayNode",
			Name: "资源类型分流",
			InputSchema: []flowengine.FieldSchema{
				{Key: ctx_resource_type, Type: "string", Required: true},
				{Key: ctx_resource_has_decode_secret, Type: "bool", Required: false},
			},
			OutputSchema: []flowengine.FieldSchema{
				{Key: ctx_resource_type, Type: "string", Required: true},
				{Key: ctx_resource_has_decode_secret, Type: "bool", Required: false},
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
				{Key: ctx_input_file, Type: "string", Required: true},
				{Key: ctx_decode_key, Type: "any", Required: false},
			},
			OutputSchema: []flowengine.FieldSchema{
				{Key: ctx_decrypted_file, Type: "string", Required: false},
			},
			Config: map[string]interface{}{"func": decrypt_node},
			NextNodes: []flowengine.TargetNode{
				{TargetID: "route_output_format"},
			},
		},
		"route_output_format": {
			ID:   "route_output_format",
			Type: "GatewayNode",
			Name: "输出格式分流",
			InputSchema: []flowengine.FieldSchema{
				{Key: ctx_task_type, Type: "int", Required: false},
				{Key: ctx_task_suffix, Type: "string", Required: false},
			},
			OutputSchema: []flowengine.FieldSchema{
				{Key: ctx_task_type, Type: "int", Required: false},
				{Key: ctx_task_suffix, Type: "string", Required: false},
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
				{Key: ctx_input_file, Type: "string", Required: true},
			},
			OutputSchema: []flowengine.FieldSchema{
				{Key: ctx_mp4_file, Type: "string", Required: false},
			},
			Config: map[string]interface{}{"func": stream_convert_node},
			NextNodes: []flowengine.TargetNode{
				{TargetID: "finalize_stream"},
			},
		},
		"finalize_stream": {
			ID:   "finalize_stream",
			Type: "FuncNode",
			Name: "生成流媒体资源结果",
			InputSchema: []flowengine.FieldSchema{
				{Key: wxchannels_postprocess_run_key, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
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
				{Key: ctx_mp4_file, Type: "string", Required: false},
			},
			OutputSchema: []flowengine.FieldSchema{},
			Config:       map[string]interface{}{"func": finalize_stream_node},
			NextNodes: []flowengine.TargetNode{
				{TargetID: "task_job_update"},
			},
		},
		"convert_mp3": {
			ID:   "convert_mp3",
			Type: "FuncNode",
			Name: "FFmpeg 转换 MP3",
			InputSchema: []flowengine.FieldSchema{
				{Key: ctx_decrypted_file, Type: "string", Required: true},
			},
			OutputSchema: []flowengine.FieldSchema{
				{Key: ctx_mp3_file, Type: "string", Required: false},
			},
			Config: map[string]interface{}{"func": convert_mp3_node},
			NextNodes: []flowengine.TargetNode{
				{TargetID: "finalize_mp3"},
			},
		},
		"finalize_mp3": {
			ID:   "finalize_mp3",
			Type: "FuncNode",
			Name: "生成 MP3 资源结果",
			InputSchema: []flowengine.FieldSchema{
				{Key: wxchannels_postprocess_run_key, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
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
				{Key: ctx_mp3_file, Type: "string", Required: true},
			},
			OutputSchema: []flowengine.FieldSchema{},
			Config:       map[string]interface{}{"func": finalize_mp3_node},
			NextNodes: []flowengine.TargetNode{
				{TargetID: "task_job_update"},
			},
		},
		"zip_resources": {
			ID:   "zip_resources",
			Type: "FuncNode",
			Name: "压缩全部 Resources",
			InputSchema: []flowengine.FieldSchema{
				{Key: wxchannels_postprocess_run_key, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
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
			Config: map[string]interface{}{"func": zip_resources_node},
			NextNodes: []flowengine.TargetNode{
				{TargetID: "task_job_update"},
			},
		},
		"task_job_update": {
			ID:   "task_job_update",
			Type: "FuncNode",
			Name: "更新传入的 TaskJob",
			InputSchema: []flowengine.FieldSchema{
				{Key: wxchannels_postprocess_run_key, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
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
			Config:       map[string]interface{}{"func": task_job_update_node},
			NextNodes:    []flowengine.TargetNode{},
		},
		"done": {
			ID:           "done",
			Type:         "FuncNode",
			Name:         "无需后处理",
			InputSchema:  []flowengine.FieldSchema{},
			OutputSchema: []flowengine.FieldSchema{},
			Config:       map[string]interface{}{"func": noop_postprocess_done_node},
		},
	},
}

var wxchannels_postprocess_main_flow = flowengine.FlowDefinition{
	ID:   wxchannels_postprocess_main_flow_id,
	Name: "wxchannels_postprocess_main",
	ContextSchema: []flowengine.FieldSchema{
		{Key: wxchannels_postprocess_context_key_ctx, Type: "object", Required: false},
		{Key: wxchannels_postprocess_context_task, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
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
		{Key: wxchannels_postprocess_context_base_path, Type: "string", Required: true},
		{
			Key:      wxchannels_postprocess_context_resources,
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
		{Key: wxchannels_postprocess_context_archive_requested, Type: "bool", Required: true},
		{Key: ctx_task_type, Type: "int", Required: false},
		{Key: ctx_task_suffix, Type: "string", Required: false},
		{Key: wxchannels_postprocess_run_key, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
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
				{Key: wxchannels_postprocess_context_task, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
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
				{Key: wxchannels_postprocess_context_base_path, Type: "string", Required: true},
				{Key: wxchannels_postprocess_context_archive_requested, Type: "bool", Required: true},
				{Key: wxchannels_postprocess_run_key, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
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
				{Key: wxchannels_postprocess_context_key_ctx, Type: "object", Required: false},
			},
			OutputSchema: []flowengine.FieldSchema{
				{
					Key:      wxchannels_postprocess_context_resources,
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
				{Key: wxchannels_postprocess_context_archive_requested, Type: "bool", Required: true},
				{Key: wxchannels_postprocess_context_task_type, Type: "int", Required: false},
				{Key: wxchannels_postprocess_context_task_suffix, Type: "string", Required: false},
			},
			Config:    map[string]interface{}{"func": bootstrap_postprocess_context_node},
			NextNodes: []flowengine.TargetNode{{TargetID: "loop_resources"}},
		},
		"loop_resources": {
			ID:   "loop_resources",
			Type: "LoopNode",
			Name: "遍历并处理每个资源",
			InputSchema: []flowengine.FieldSchema{
				{
					Key:      wxchannels_postprocess_context_resources,
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
				{Key: wxchannels_postprocess_context_archive_requested, Type: "bool", Required: true},
				{Key: wxchannels_postprocess_run_key, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
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
				"item_key":     wxchannels_postprocess_context_resource,
				"iterable_key": wxchannels_postprocess_context_resources,
				"cleanup_keys": []interface{}{wxchannels_postprocess_context_resource, ctx_input_file, ctx_resource_type, ctx_decode_key, ctx_resource_has_decode_secret, ctx_decrypted_file, ctx_mp4_file, ctx_mp3_file},
				"workflow":     wxchannels_postprocess_flow,
			},
			NextNodes: []flowengine.TargetNode{{TargetID: "archive_requested_gate"}},
		},
		"archive_requested_gate": {
			ID:   "archive_requested_gate",
			Type: "GatewayNode",
			Name: "是否需要归档",
			InputSchema: []flowengine.FieldSchema{
				{Key: wxchannels_postprocess_context_archive_requested, Type: "bool", Required: true},
			},
			OutputSchema: []flowengine.FieldSchema{
				{Key: wxchannels_postprocess_context_archive_requested, Type: "bool", Required: true},
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
				{Key: wxchannels_postprocess_run_key, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
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
				{Key: ctx_task_type, Type: "int", Required: false},
				{Key: ctx_task_suffix, Type: "string", Required: false},
			},
			OutputSchema: []flowengine.FieldSchema{},
			Config:       map[string]interface{}{"workflow": wxchannels_output_flow},
			NextNodes:    []flowengine.TargetNode{{TargetID: "done"}},
		},
		"done": {
			ID:           "done",
			Type:         "FuncNode",
			Name:         "后处理完成",
			InputSchema:  []flowengine.FieldSchema{},
			OutputSchema: []flowengine.FieldSchema{},
			Config:       map[string]interface{}{"func": noop_postprocess_done_node},
		},
	},
}

var wxchannels_output_flow = flowengine.FlowDefinition{
	ID:   wxchannels_postprocess_output_flow_id,
	Name: "wxchannels_postprocess_output",
	ContextSchema: []flowengine.FieldSchema{
		{Key: wxchannels_postprocess_context_key_ctx, Type: "object", Required: false},
		{Key: ctx_task_type, Type: "int", Required: false},
		{Key: ctx_task_suffix, Type: "string", Required: false},
		{Key: wxchannels_postprocess_run_key, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
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
				{Key: wxchannels_postprocess_run_key, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
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
				{Key: ctx_task_type, Type: "int", Required: false},
				{Key: ctx_task_suffix, Type: "string", Required: false},
			},
			OutputSchema: []flowengine.FieldSchema{
				{Key: wxchannels_postprocess_run_key, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
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
				{Key: ctx_task_type, Type: "int", Required: false},
				{Key: ctx_task_suffix, Type: "string", Required: false},
			},
			NextNodes: []flowengine.TargetNode{{TargetID: "route_output_format"}},
		},
		"route_output_format": {
			ID:   "route_output_format",
			Type: "GatewayNode",
			Name: "输出格式分流",
			InputSchema: []flowengine.FieldSchema{
				{Key: ctx_task_type, Type: "int", Required: false},
				{Key: ctx_task_suffix, Type: "string", Required: false},
			},
			OutputSchema: []flowengine.FieldSchema{
				{Key: ctx_task_type, Type: "int", Required: false},
				{Key: ctx_task_suffix, Type: "string", Required: false},
			},
			Config: map[string]interface{}{
				"gateway_type": "Exclusive",
				"is_joining":   false,
				"rules": []map[string]interface{}{
					{"condition": `task_config_type == -1 || task_config_suffix == ".zip" || task_config_suffix == "zip"`, "target_id": "zip_resources"},
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
			Config:       map[string]interface{}{"func": noop_postprocess_done_node},
		},
	},
}

// GetWXChannelsPostprocessFlowVisualization returns a read-only flow graph payload
// for frontend visualization.
func GetWXChannelsPostprocessFlowVisualization(flow_id string) (*WXChannelsPostprocessFlowVisualizationPayload, error) {
	available := []flowengine.FlowDefinition{wxchannels_postprocess_main_flow, wxchannels_postprocess_flow, wxchannels_output_flow}
	payload, err := flowengine.BuildFlowVisualizationPayload(available, flow_id, flowengine.FlowVisualizationOptions{
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
		{Key: ctx_input_file, Type: "string", Required: true},
		{Key: ctx_decode_key, Type: "any", Required: false},
	},
	OutputSchema: []flowengine.FieldSchema{
		{Key: ctx_decrypted_file, Type: "string", Required: false},
	},
	Config: map[string]interface{}{"func": decrypt_node},
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
		{Key: ctx_decrypted_file, Type: "string", Required: true},
	},
	OutputSchema: []flowengine.FieldSchema{
		{Key: ctx_mp3_file, Type: "string", Required: false},
	},
	Config: map[string]interface{}{"func": convert_mp3_node},
}

var FinalizeMP3Node = flowengine.NodeDefinition{
	ID:   "finalize_mp3",
	Type: "FuncNode",
	Name: "生成 MP3 资源结果",
	InputSchema: []flowengine.FieldSchema{
		{Key: wxchannels_postprocess_run_key, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
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
		{Key: ctx_mp3_file, Type: "string", Required: true},
	},
	OutputSchema: []flowengine.FieldSchema{},
	Config:       map[string]interface{}{"func": finalize_mp3_node},
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
		{Key: ctx_input_file, Type: "string", Required: true},
	},
	OutputSchema: []flowengine.FieldSchema{
		{Key: ctx_mp4_file, Type: "string", Required: false},
	},
	Config: map[string]interface{}{"func": stream_convert_node},
}

// FinalizeStreamNode applies the stream conversion result to the resource model.
var FinalizeStreamNode = flowengine.NodeDefinition{
	ID:   "finalize_stream",
	Type: "FuncNode",
	Name: "生成流媒体资源结果",
	InputSchema: []flowengine.FieldSchema{
		{Key: wxchannels_postprocess_run_key, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
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
		{Key: ctx_mp4_file, Type: "string", Required: false},
	},
	OutputSchema: []flowengine.FieldSchema{},
	Config:       map[string]interface{}{"func": finalize_stream_node},
}

var ZipResourcesNode = flowengine.NodeDefinition{
	ID:   "zip_resources",
	Type: "FuncNode",
	Name: "压缩全部 Resources",
	InputSchema: []flowengine.FieldSchema{
		{Key: wxchannels_postprocess_run_key, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
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
	Config: map[string]interface{}{"func": zip_resources_node},
}

// TaskJobUpdateNode commits post-processing results to the exact TaskJob pointer
// passed into Postprocess.
var TaskJobUpdateNode = flowengine.NodeDefinition{
	ID:   "task_job_update",
	Type: "FuncNode",
	Name: "更新传入的 TaskJob",
	InputSchema: []flowengine.FieldSchema{
		{Key: wxchannels_postprocess_run_key, Type: "object", Required: true, Fields: []flowengine.FieldSchema{
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
	Config:       map[string]interface{}{"func": task_job_update_node},
}

// Postprocess performs wxchannels-specific decrypt and media conversion.
func (a *ChannelsAdapter) Postprocess(ctx context.Context, info *hermes.TaskJob, deps adapter.PostprocessDeps) error {
	log := func(msg string, args ...interface{}) {
		deps.Logger.Info().Msg(fmt.Sprintf(msg, args...))
	}
	log("Postprocessor.wxchannels: task_id=%d processing %d resources", info.ID, len(info.Resources))
	archive_requested := is_zip_output(info.Config)
	log("Postprocessor.wxchannels: task_id=%d config.suffix=%q archiveRequested=%v", info.ID, task_config_suffix(info.Config), archive_requested)

	if err := run_wx_channels_postprocess_flow(wxchannels_postprocess_main_flow, map[string]interface{}{
		ctx_key:                                          ctx,
		wxchannels_postprocess_context_task:              info,
		wxchannels_postprocess_context_base_path:         deps.BasePath,
		wxchannels_postprocess_context_archive_requested: archive_requested,
		wxchannels_postprocess_run_key: &postprocess_run{
			task:      info,
			base_path: deps.BasePath,
			log:       log,
		},
	}); err != nil {
		return fmt.Errorf("wxchannels postprocess: %w", err)
	}

	return nil
}

func is_zip_output(config map[string]any) bool {
	return task_config_type(config) == -1 || strings.EqualFold(task_config_suffix(config), ".zip")
}

func postprocess_resource_name(base_path, file_path string) string {
	rel_path, _ := filepath.Rel(base_path, file_path)
	if rel_path != "" && !filepath.IsAbs(rel_path) {
		return rel_path
	}
	return filepath.Base(file_path)
}

func task_config_type(config map[string]any) int {
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

func task_config_suffix(config map[string]any) string {
	if config == nil {
		return ""
	}
	value, _ := config["suffix"].(string)
	return strings.TrimSpace(value)
}

func run_wx_channels_postprocess_flow(flow flowengine.FlowDefinition, ctx map[string]interface{}) error {
	if ctx == nil {
		ctx = map[string]interface{}{}
	}
	if _, ok := ctx[ctx_key]; !ok {
		ctx[ctx_key] = context.Background()
	}
	run, _ := postprocess_run_from_context(ctx)
	log_postprocess_flow(flow.ID, "started", run, nil)
	if run != nil && run.log != nil {
		run.log("Postprocess.wxchannels: flow=%s input=%s", flow.ID, format_context_snapshot(postprocess_context_snapshot(ctx)))
	}
	flow_engine := flowengine.NewWorkflowEngine()
	flow_engine.RegisterNode("GatewayNode", new_wx_channels_logged_gateway_node)
	flow_engine.SetFlowDefinitions(map[string]flowengine.FlowDefinition{
		flow.ID: flow,
	})
	instance_id, err := flow_engine.StartFlow(flow.ID, ctx)
	if err != nil {
		log_postprocess_flow(flow.ID, "failed", run, err)
		log_postprocess_flow_node_execution(run, flow_engine, flow.ID, instance_id, err)
		return err
	}
	log_postprocess_flow(flow.ID, "succeeded", run, nil)
	log_postprocess_flow_node_execution(run, flow_engine, flow.ID, instance_id, nil)
	return err
}

func bootstrap_postprocess_context_node(values map[string]interface{}) (interface{}, error) {
	return run_wx_channels_node(values, wxchannels_postprocess_flow_node_bootstrap_context, func(run *postprocess_run) (interface{}, error) {
		task, ok := values[wxchannels_postprocess_context_task].(*hermes.TaskJob)
		if !ok || task == nil {
			return nil, fmt.Errorf("缺少 task")
		}

		run.task = task
		if run.base_path == "" {
			run.base_path, _ = values[wxchannels_postprocess_context_base_path].(string)
		}
		values[wxchannels_postprocess_run_key] = run

		resources := make([]interface{}, 0, len(task.Resources))
		for i := range task.Resources {
			resources = append(resources, resource_to_context_map(&task.Resources[i]))
		}
		values[wxchannels_postprocess_context_resources] = resources

		values[wxchannels_postprocess_context_task_type] = task_config_type(task.Config)
		values[wxchannels_postprocess_context_task_suffix] = task_config_suffix(task.Config)
		return nil, nil
	})
}

func postprocess_run_from_context(values map[string]interface{}) (*postprocess_run, error) {
	run, _ := values[wxchannels_postprocess_run_key].(*postprocess_run)
	if run == nil {
		return nil, fmt.Errorf("缺少 postprocess_run")
	}
	if run.task == nil {
		task, ok := values[wxchannels_postprocess_context_task].(*hermes.TaskJob)
		if !ok || task == nil {
			return nil, fmt.Errorf("缺少 task")
		}
		run.task = task
	}
	if run.resource == nil {
		run.resource = resolve_postprocess_resource(values[wxchannels_postprocess_context_resource])
	}
	if run.resource != nil {
		run.original_ext = strings.ToLower(filepath.Ext(run.resource.FilePath))
	}
	return run, nil
}

func prepare_resource_context_node(values map[string]interface{}) (interface{}, error) {
	return run_wx_channels_node(values, wxchannels_postprocess_flow_node_prepare_resource_context, func(run *postprocess_run) (interface{}, error) {
		resource, has_resource := values[wxchannels_postprocess_context_resource].(*hermes.ResourceJob)
		if !has_resource {
			resource = resolve_postprocess_resource(values[wxchannels_postprocess_context_resource])
		}
		if run.resource != resource {
			run.resource = resource
		}
		if run.resource == nil {
			return nil, fmt.Errorf("缺少 resource")
		}

		input_file := ""
		if input, ok := values[ctx_input_file].(string); ok {
			input_file = input
		}
		if input_file == "" {
			input_file = run.resource.FilePath
		}
		input_file = strings.TrimSpace(input_file)
		if input_file == "" {
			return nil, fmt.Errorf("缺少 input_file")
		}
		values[ctx_input_file] = input_file

		resource_type := ""
		if rt, ok := values[ctx_resource_type].(string); ok {
			resource_type = rt
		}
		if resource_type == "" {
			resource_type = run.resource.Type
		}
		if resource_type == "" {
			return nil, fmt.Errorf("缺少 resource_type")
		}
		values[ctx_resource_type] = resource_type

		decode_key, ok := values[ctx_decode_key]
		if !ok || decode_key == nil {
			decode_key = run.resource.Extra["decode_key"]
		}
		values[ctx_decode_key] = decode_key
		values[wxchannels_postprocess_context_resource_has_decode_secret] = decode_key_to_string(decode_key) != ""
		if run.log != nil {
			run.log("Postprocess.wxchannels: resource_context_prepared: resource_id=%d resource_type=%q input_file=%q decode_key=%q resource_has_decode_key=%v",
				run.resource.ID, resource_type, input_file, decode_key_to_string(decode_key), values[wxchannels_postprocess_context_resource_has_decode_secret])
		}

		values[wxchannels_postprocess_context_task_type] = task_config_type_from_context(values)
		values[wxchannels_postprocess_context_task_suffix] = task_config_suffix_from_context(values)
		return nil, nil
	})
}

func noop_postprocess_done_node(values map[string]interface{}) (interface{}, error) {
	return run_wx_channels_node(values, wxchannels_postprocess_flow_node_done, func(*postprocess_run) (interface{}, error) {
		return nil, nil
	})
}

func task_config_type_from_context(values map[string]interface{}) int {
	if values == nil {
		return 0
	}
	v, ok := values[ctx_task_type]
	if !ok {
		return 0
	}
	if task_type, ok := v.(int); ok {
		return task_type
	}
	if task_type, ok := v.(float64); ok {
		return int(task_type)
	}
	if task_type, ok := v.(string); ok {
		parsed, err := strconv.Atoi(strings.TrimSpace(task_type))
		if err == nil {
			return parsed
		}
	}
	return 0
}

func task_config_suffix_from_context(values map[string]interface{}) string {
	if values == nil {
		return ""
	}
	v, ok := values[ctx_task_suffix]
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func resolve_postprocess_resource(value interface{}) *hermes.ResourceJob {
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
		return resolve_postprocess_resource_from_map(raw)
	}
	if raw, ok := value.(map[string]string); ok {
		converted := make(map[string]interface{}, len(raw))
		for key, v := range raw {
			converted[key] = v
		}
		return resolve_postprocess_resource_from_map(converted)
	}
	return nil
}

func resource_to_context_map(resource *hermes.ResourceJob) map[string]interface{} {
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

func resolve_postprocess_resource_from_map(raw map[string]interface{}) *hermes.ResourceJob {
	resource := &hermes.ResourceJob{}
	resource.ID = cast_int(raw["ID"])
	resource.Name = cast_string(raw["Name"])
	resource.Kind = cast_string(raw["Kind"])
	resource.Type = cast_string(raw["Type"])
	resource.UniqueID = cast_string(raw["UniqueID"])
	resource.FilePath = cast_string(raw["FilePath"])
	resource.Size = cast_int64(raw["Size"])
	resource.Downloaded = cast_int64(raw["Downloaded"])
	resource.Speed = cast_int64(raw["Speed"])
	resource.Extra = map[string]string{}

	if raw_extra, ok := raw["Extra"].(map[string]interface{}); ok {
		for key, value := range raw_extra {
			resource.Extra[key] = cast_string(value)
		}
	} else if raw_extra, ok := raw["Extra"].(map[string]string); ok {
		for key, value := range raw_extra {
			resource.Extra[key] = value
		}
	}
	return resource
}

func cast_int64(value interface{}) int64 {
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

func cast_int(value interface{}) int {
	return int(cast_int64(value))
}

func cast_string(value interface{}) string {
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

func node_context(values map[string]interface{}) context.Context {
	if v, ok := values[ctx_key].(context.Context); ok && v != nil {
		return v
	}
	return context.Background()
}

func decrypt_node(values map[string]interface{}) (interface{}, error) {
	return run_wx_channels_node(values, wxchannels_postprocess_flow_node_decrypt, func(run *postprocess_run) (interface{}, error) {
		input_file, _ := values[ctx_input_file].(string)
		if input_file == "" {
			return nil, fmt.Errorf("缺少 input_file")
		}
		if run.task == nil || run.resource == nil {
			return nil, fmt.Errorf("缺少 task 或 resource")
		}

		decode_key_str := strings.TrimSpace(decode_key_to_string(values[ctx_decode_key]))
		key, err := strconv.ParseUint(decode_key_str, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("解析 decode_key 失败: %w", err)
		}

		tmp_file := input_file + ".tmp"
		if err := wxchannels.DecryptFile(input_file, tmp_file, key, 131072); err != nil {
			_ = os.Remove(tmp_file)
			return nil, err
		}
		if err := os.Rename(tmp_file, input_file); err != nil {
			_ = os.Remove(tmp_file)
			return nil, fmt.Errorf("原地替换解密文件失败: %w", err)
		}
		values[ctx_decrypted_file] = input_file
		return nil, nil
	})
}

func decode_key_to_string(value interface{}) string {
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

func convert_mp3_node(values map[string]interface{}) (interface{}, error) {
	return run_wx_channels_node(values, wxchannels_postprocess_flow_node_convert_mp3, func(*postprocess_run) (interface{}, error) {
		decrypted_file, _ := values[ctx_decrypted_file].(string)
		if decrypted_file == "" {
			return nil, fmt.Errorf("缺少 decrypted_file")
		}

		base_name := filepath.Base(decrypted_file)
		ext := filepath.Ext(base_name)
		mp3_file := filepath.Join(filepath.Dir(decrypted_file), strings.TrimSuffix(base_name, ext))
		tmp_file := mp3_file + ".converting"

		cmd := exec.CommandContext(
			node_context(values),
			"ffmpeg",
			"-i", decrypted_file,
			"-vn",
			"-acodec", "libmp3lame",
			"-ab", "192k",
			"-f", "mp3",
			"-y",
			tmp_file,
		)
		output, err := cmd.CombinedOutput()
		if err != nil {
			_ = os.Remove(tmp_file)
			return nil, fmt.Errorf("ffmpeg 转换失败: %w\n%s", err, string(output))
		}
		if err := replace_converted_file(decrypted_file, tmp_file, mp3_file); err != nil {
			return nil, fmt.Errorf("替换 MP3 转换文件失败: %w", err)
		}
		values[ctx_mp3_file] = mp3_file
		return nil, nil
	})
}

func finalize_mp3_node(values map[string]interface{}) (interface{}, error) {
	return run_wx_channels_node(values, wxchannels_postprocess_flow_node_finalize_mp3, func(run *postprocess_run) (interface{}, error) {
		mp3_file, _ := values[ctx_mp3_file].(string)
		if mp3_file == "" {
			return nil, fmt.Errorf("缺少 mp3_file")
		}
		if run.resource == nil {
			return nil, fmt.Errorf("缺少 resource")
		}

		if run.resource.FilePath != mp3_file {
			_ = os.Remove(run.resource.FilePath)
		}
		run.resource.Kind = "audio/mpeg"
		if title := run.resource.Extra["title"]; title != "" {
			run.resource.Name = sanitize_bgm_name(title)
		} else {
			run.resource.Name = postprocess_resource_name(run.base_path, mp3_file)
		}
		run.resource.FilePath = mp3_file
		return nil, nil
	})
}

func stream_convert_node(values map[string]interface{}) (interface{}, error) {
	return run_wx_channels_node(values, wxchannels_postprocess_flow_node_stream_convert, func(run *postprocess_run) (interface{}, error) {
		input_file, _ := values[ctx_input_file].(string)
		if input_file == "" {
			return nil, fmt.Errorf("缺少 input_file")
		}
		if run.task == nil || run.resource == nil {
			return nil, fmt.Errorf("缺少 task 或 resource")
		}

		ctx := node_context(values)
		video_codec, err := probe_first_media_codec(ctx, input_file, "v:0")
		if err != nil {
			return nil, fmt.Errorf("检测直播视频编码失败: %w", err)
		}
		if video_codec == "" {
			return nil, fmt.Errorf("直播文件不包含视频轨道")
		}
		audio_codec, err := probe_first_media_codec(ctx, input_file, "a:0")
		if err != nil {
			return nil, fmt.Errorf("检测直播音频编码失败: %w", err)
		}

		ext := strings.ToLower(filepath.Ext(input_file))
		base_name := filepath.Base(input_file)
		mp4_file := input_file
		if ext != ".mp4" {
			mp4_file = filepath.Join(filepath.Dir(input_file), strings.TrimSuffix(base_name, ext)+".mp4")
		}
		tmp_file := mp4_file + ".converting"

		cmd := exec.CommandContext(ctx, "ffmpeg", build_stream_mp4_args(input_file, tmp_file, video_codec, audio_codec)...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			_ = os.Remove(tmp_file)
			return nil, fmt.Errorf("ffmpeg stream remux 失败: %w\n%s", err, string(output))
		}
		if err := replace_converted_file(input_file, tmp_file, mp4_file); err != nil {
			return nil, fmt.Errorf("替换 MP4 转换文件失败: %w", err)
		}

		values[ctx_mp4_file] = mp4_file
		return nil, nil
	})
}

func probe_first_media_codec(ctx context.Context, input_file, selector string) (string, error) {
	cmd := exec.CommandContext(
		ctx,
		"ffprobe",
		"-v", "error",
		"-select_streams", selector,
		"-show_entries", "stream=codec_name",
		"-of", "default=noprint_wrappers=1:nokey=1",
		input_file,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ffprobe 失败: %w\n%s", err, string(output))
	}
	codec, _, _ := strings.Cut(strings.TrimSpace(string(output)), "\n")
	return strings.ToLower(strings.TrimSpace(codec)), nil
}

func build_stream_mp4_args(input_file, output_file, video_codec, audio_codec string) []string {
	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-fflags", "+genpts",
		"-i", input_file,
		"-map", "0:v:0",
		"-map", "0:a:0?",
		"-map_metadata", "0",
	}

	switch strings.ToLower(strings.TrimSpace(video_codec)) {
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

	switch strings.ToLower(strings.TrimSpace(audio_codec)) {
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
		output_file,
	)
}

func finalize_stream_node(values map[string]interface{}) (interface{}, error) {
	return run_wx_channels_node(values, wxchannels_postprocess_flow_node_finalize_stream, func(run *postprocess_run) (interface{}, error) {
		if run.resource == nil {
			return nil, fmt.Errorf("缺少 resource")
		}
		mp4_file, _ := values[ctx_mp4_file].(string)
		if mp4_file == "" {
			return nil, fmt.Errorf("缺少 mp4_file")
		}
		run.resource.FilePath = mp4_file
		if strings.TrimSpace(run.resource.Name) == "" {
			run.resource.Name = postprocess_resource_name(run.base_path, mp4_file)
		}
		run.resource.Kind = "video/mp4"
		return nil, nil
	})
}

func task_job_update_node(values map[string]interface{}) (interface{}, error) {
	return run_wx_channels_node(values, wxchannels_postprocess_flow_node_task_job_update, func(run *postprocess_run) (interface{}, error) {
		if run.task == nil {
			return nil, fmt.Errorf("缺少 task")
		}
		if run.resource == nil {
			for i := range run.task.Resources {
				if err := update_postprocessed_resource(&run.task.Resources[i]); err != nil {
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
				if err := update_postprocessed_resource(target); err != nil {
					return nil, fmt.Errorf("更新 TaskJob resource[%d]: %w", i, err)
				}
				return nil, nil
			}
		}
		return nil, fmt.Errorf("TaskJob 中找不到 postprocess resource id=%d unique_id=%q", run.resource.ID, run.resource.UniqueID)
	})
}

func update_postprocessed_resource(resource *hermes.ResourceJob) error {
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

func zip_resources_node(values map[string]interface{}) (interface{}, error) {
	return run_wx_channels_node(values, wxchannels_postprocess_flow_node_zip_resources, func(run *postprocess_run) (interface{}, error) {
		if run.task == nil {
			return nil, fmt.Errorf("缺少 task")
		}

		archive_unique_id := strings.TrimSpace(run.task.UniqueID) + "_zip"
		if archive_unique_id == "_zip" {
			archive_unique_id = fmt.Sprintf("task_%d_zip", run.task.ID)
		}
		archive_path := filepath.Join(run.base_path, run.task.SavePath, archive_unique_id)
		for _, resource := range run.task.Resources {
			if filepath.Clean(resource.FilePath) == filepath.Clean(archive_path) {
				return nil, fmt.Errorf("ZIP 输出路径与资源输入路径冲突: %s", archive_path)
			}
		}
		if err := os.MkdirAll(filepath.Dir(archive_path), 0755); err != nil {
			return nil, fmt.Errorf("创建 ZIP 输出目录失败: %w", err)
		}
		if err := write_resources_zip(archive_path, run.task.Resources); err != nil {
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

		archive_kind := mime_application_zip
		archive_name := strings.TrimSpace(run.task.Name)
		if archive_name == "" {
			archive_name = "archive"
		}
		archive_resource_name := resource_name_without_canonical_extension(archive_name, archive_kind)
		archive_resource := hermes.ResourceJob{
			Name:     archive_resource_name,
			Kind:     archive_kind,
			Type:     "FILE",
			UniqueID: archive_unique_id,
			FilePath: archive_path,
			Extra:    map[string]string{"title": archive_name},
		}
		if len(run.task.Resources) > 0 {
			archive_resource.ID = run.task.Resources[0].ID
		}
		if stat, err := os.Stat(archive_path); err == nil {
			archive_resource.Size = stat.Size()
		}
		run.task.Resources = []hermes.ResourceJob{archive_resource}
		run.resource = &run.task.Resources[0]
		values[wxchannels_postprocess_context_resource] = run.resource
		values["archive_file"] = archive_path
		return nil, nil
	})
}

func resource_name_without_canonical_extension(name, kind string) string {
	ext := hermes.CanonicalExtensionForMIMEType(kind)
	if ext == "" {
		return name
	}
	dir, base := filepath.Split(strings.TrimSpace(name))
	base_ext := filepath.Ext(base)
	if !strings.EqualFold(base_ext, ext) {
		return name
	}
	trimmed := strings.TrimSuffix(base, base_ext)
	if strings.TrimSpace(trimmed) == "" {
		return name
	}
	return dir + trimmed
}

func write_resources_zip(archive_path string, resources []hermes.ResourceJob) error {
	tmp_file := archive_path + ".archiving"
	_ = os.Remove(tmp_file)
	file, err := os.Create(tmp_file)
	if err != nil {
		return fmt.Errorf("创建 ZIP 临时文件失败: %w", err)
	}
	writer := zip.NewWriter(file)
	entry_names := make(map[string]int, len(resources))

	fail := func(cause error) error {
		_ = writer.Close()
		_ = file.Close()
		_ = os.Remove(tmp_file)
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

		entry_name := unique_zip_entry_name(resource, i, entry_names)
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			_ = input.Close()
			return fail(fmt.Errorf("创建 ZIP entry %q 失败: %w", entry_name, err))
		}
		header.Name = entry_name
		header.Method = zip.Deflate
		entry, err := writer.CreateHeader(header)
		if err != nil {
			_ = input.Close()
			return fail(fmt.Errorf("写入 ZIP entry %q 失败: %w", entry_name, err))
		}
		_, copy_err := io.Copy(entry, input)
		close_err := input.Close()
		if copy_err != nil {
			return fail(fmt.Errorf("压缩文件 %q 失败: %w", resource.FilePath, copy_err))
		}
		if close_err != nil {
			return fail(fmt.Errorf("关闭 ZIP 源文件 %q 失败: %w", resource.FilePath, close_err))
		}
	}
	if err := writer.Close(); err != nil {
		_ = file.Close()
		_ = os.Remove(tmp_file)
		return fmt.Errorf("完成 ZIP 数据写入失败: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmp_file)
		return fmt.Errorf("关闭 ZIP 临时文件失败: %w", err)
	}
	if err := os.Remove(archive_path); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tmp_file)
		return fmt.Errorf("删除已有 ZIP 输出失败: %w", err)
	}
	if err := os.Rename(tmp_file, archive_path); err != nil {
		_ = os.Remove(tmp_file)
		return fmt.Errorf("提交 ZIP 输出失败: %w", err)
	}
	return nil
}

func unique_zip_entry_name(resource hermes.ResourceJob, index int, seen map[string]int) string {
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

// replace_converted_file replaces input_file with the completed conversion. The
// temporary output is kept when the final rename fails so it can be recovered.
func replace_converted_file(input_file, tmp_file, final_file string) error {
	if final_file != input_file {
		if err := os.Remove(final_file); err != nil && !os.IsNotExist(err) {
			_ = os.Remove(tmp_file)
			return fmt.Errorf("删除已有输出文件失败: %w", err)
		}
	}
	if err := os.Remove(input_file); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tmp_file)
		return fmt.Errorf("删除原文件失败: %w", err)
	}
	if err := os.Rename(tmp_file, final_file); err != nil {
		return fmt.Errorf("重命名临时文件 %q 为 %q 失败: %w", tmp_file, final_file, err)
	}
	return nil
}
