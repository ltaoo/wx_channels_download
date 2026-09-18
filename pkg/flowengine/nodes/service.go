package nodes

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"wx_channel/pkg/flowengine/engine"
)

const default_service_output_key = "service_result"

// service_template_re matches a {{a.b.c}} token and captures the whole dotted
// path. The path is resolved scope-then-path: the first segment names one of
// the three scopes (input/output/global) and the remaining segments drill
// through maps and numeric slice indexes.
var service_template_re = regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_]*(?:\s*\.\s*[A-Za-z_][A-Za-z0-9_]*)*)\s*\}\}`)

// ServiceToolExecutor runs one named service tool with JSON-compatible
// arguments and returns its structured output.
type ServiceToolExecutor func(context.Context, string, map[string]any) (any, error)

// ServiceNode invokes a tool exposed by the process-local MCP server.
type ServiceNode struct {
	node_id  string
	Config   map[string]interface{}
	executor ServiceToolExecutor
}

// NewServiceNodeFactory binds a service executor to a flow-engine node
// constructor. Keeping the executor on the engine avoids process-wide state.
func NewServiceNodeFactory(executor ServiceToolExecutor) func(map[string]interface{}) engine.Node {
	return func(config map[string]interface{}) engine.Node {
		id, _ := config["id"].(string)
		return &ServiceNode{node_id: id, Config: config, executor: executor}
	}
}

func (n *ServiceNode) ID() string   { return n.node_id }
func (n *ServiceNode) Type() string { return "ServiceNode" }

func (n *ServiceNode) Execute(process_context *engine.ProcessContext) (bool, []string, error) {
	if n.executor == nil {
		return false, nil, fmt.Errorf("service executor is not configured")
	}
	tool_name, _ := n.Config["tool_name"].(string)
	tool_name = strings.TrimSpace(tool_name)
	if tool_name == "" {
		return false, nil, fmt.Errorf("missing tool_name")
	}

	arguments, err := service_arguments(n.Config, process_context)
	if err != nil {
		return false, nil, err
	}
	// Audit the arguments that were actually resolved from the templates, not
	// just the raw config, so the execution log matches the request that ran.
	process_context.SetExecutionDetail("arguments", arguments)
	execution_context := context.Background()
	cancel := func() {}
	if timeout_seconds := service_timeout_seconds(n.Config); timeout_seconds > 0 {
		execution_context, cancel = context.WithTimeout(execution_context, time.Duration(timeout_seconds)*time.Second)
	}
	defer cancel()

	result, err := n.executor(execution_context, tool_name, arguments)
	if err != nil {
		return false, nil, fmt.Errorf("service tool %s failed: %w", tool_name, err)
	}
	output_key := default_service_output_key
	if configured_key, ok := n.Config["output_key"].(string); ok && strings.TrimSpace(configured_key) != "" {
		output_key = strings.TrimSpace(configured_key)
	}
	process_context.Mu.Lock()
	process_context.Data[output_key] = result
	process_context.Mu.Unlock()

	next_ids := process_context.EngineRef.GetNextNodeIDsFromDefinition(process_context, n.node_id)
	return true, next_ids, nil
}

func service_arguments(config map[string]interface{}, process_context *engine.ProcessContext) (map[string]any, error) {
	arguments := map[string]any{}
	if configured_arguments, ok := config["arguments"]; ok && configured_arguments != nil {
		argument_map, ok := configured_arguments.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("service arguments must be an object")
		}
		for key, value := range argument_map {
			arguments[key] = value
		}
	}

	process_context.Mu.Lock()
	defer process_context.Mu.Unlock()

	for key, value := range arguments {
		resolved, err := resolve_service_template(value, process_context)
		if err != nil {
			return nil, fmt.Errorf("argument %s: %w", key, err)
		}
		arguments[key] = resolved
	}
	return arguments, nil
}

// resolve_service_template renders {{scope.path}} templates in a single
// argument value against the process context. A value that is not a string, or
// a string without "{{", is returned unchanged. A string that is exactly one
// token resolves to the raw context value (preserving its type); otherwise each
// token is interpolated into the rendered string.
//
// Every read must declare its scope: input.<key> for run parameters declared by
// the flow context schema, output.<node_id>.<key> for a specific upstream
// producer, and global.<key> for variables written by SetVariableNode. A bare
// {{key}} or {{ctx.key}} is rejected, and a "{{" that matches no token at all
// is an error rather than a silent passthrough.
func resolve_service_template(value any, ctx *engine.ProcessContext) (any, error) {
	text, ok := value.(string)
	if !ok || !strings.Contains(text, "{{") {
		return value, nil
	}
	matches := service_template_re.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("invalid template expression: %s", text)
	}
	if len(matches) == 1 {
		start, end := matches[0][0], matches[0][1]
		if start == 0 && end == len(text) {
			return service_template_lookup(ctx, text[matches[0][2]:matches[0][3]])
		}
	}
	for _, match := range matches {
		if _, err := service_template_lookup(ctx, text[match[2]:match[3]]); err != nil {
			return nil, err
		}
	}
	replaced := service_template_re.ReplaceAllStringFunc(text, func(match string) string {
		path := service_template_re.FindStringSubmatch(match)[1]
		resolved, _ := service_template_lookup(ctx, path)
		return fmt.Sprint(resolved)
	})
	return replaced, nil
}

// service_template_lookup resolves one dotted scope path. The first segment
// names the scope, the second addresses the key (or producer node), and any
// further segments drill through maps by key and through slices by numeric
// index. Missing scopes, producers, keys and bad indexes are hard errors.
func service_template_lookup(ctx *engine.ProcessContext, path string) (any, error) {
	segments := strings.Split(path, ".")
	for index := range segments {
		segments[index] = strings.TrimSpace(segments[index])
	}
	not_found := fmt.Errorf("context value not found: %s", path)
	switch segments[0] {
	case "input":
		if len(segments) < 2 || !input_key_declared(ctx, segments[1]) {
			return nil, not_found
		}
		value, exists := ctx.Data[segments[1]]
		if !exists {
			return nil, not_found
		}
		return drill_template_path(value, segments[2:], not_found)
	case "output":
		if len(segments) < 3 {
			return nil, not_found
		}
		produced, exists := ctx.NodeOutputs[segments[1]]
		if !exists {
			return nil, not_found
		}
		value, exists := produced[segments[2]]
		if !exists {
			return nil, not_found
		}
		return drill_template_path(value, segments[3:], not_found)
	case "global":
		if len(segments) < 2 {
			return nil, not_found
		}
		value, exists := ctx.Globals[segments[1]]
		if !exists {
			return nil, not_found
		}
		return drill_template_path(value, segments[2:], not_found)
	}
	if len(segments) == 1 {
		return nil, fmt.Errorf("template %s must declare an explicit scope (input/output/global)", path)
	}
	return nil, fmt.Errorf("unknown template namespace: %s", segments[0])
}

func input_key_declared(ctx *engine.ProcessContext, key string) bool {
	for _, declared := range ctx.InputKeys {
		if declared == key {
			return true
		}
	}
	return false
}

func drill_template_path(value any, segments []string, not_found error) (any, error) {
	current := value
	for _, segment := range segments {
		next, ok := drill_template_segment(current, segment)
		if !ok {
			return nil, not_found
		}
		current = next
	}
	return current, nil
}

func drill_template_segment(value any, segment string) (any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		nested, ok := typed[segment]
		return nested, ok
	case map[string]string:
		nested, ok := typed[segment]
		return nested, ok
	case []any:
		index, err := strconv.Atoi(segment)
		if err != nil || index < 0 || index >= len(typed) {
			return nil, false
		}
		return typed[index], true
	default:
		return nil, false
	}
}

func service_timeout_seconds(config map[string]interface{}) int {
	switch value := config["timeout_seconds"].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}
