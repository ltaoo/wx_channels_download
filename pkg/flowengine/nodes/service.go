package nodes

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"wx_channel/pkg/flowengine/engine"
)

const default_service_output_key = "service_result"

var service_template_re = regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_]*)(?:\s*\.\s*([A-Za-z_][A-Za-z0-9_]*))?\s*\}\}`)

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
	process_context.SetOutput(output_key, result)
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

	if configured_input_map, ok := config["input_map"]; ok && configured_input_map != nil {
		input_map, ok := configured_input_map.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("service input_map must be an object")
		}
		for argument_name, context_key_value := range input_map {
			context_key, ok := context_key_value.(string)
			if !ok || strings.TrimSpace(context_key) == "" {
				return nil, fmt.Errorf("service input_map.%s must be a context key", argument_name)
			}
			value, exists := process_context.Data[context_key]
			if !exists {
				return nil, fmt.Errorf("service context value not found: %s", context_key)
			}
			arguments[argument_name] = value
		}
	}

	for key, value := range arguments {
		resolved, err := resolve_service_template(value, process_context)
		if err != nil {
			return nil, fmt.Errorf("argument %s: %w", key, err)
		}
		arguments[key] = resolved
	}
	return arguments, nil
}

// resolve_service_template renders {{key}} / {{namespace.key}} templates in a
// single argument value against the process context. A value that is not a
// string, or a string without "{{", is returned unchanged. A string that is
// exactly one token resolves to the raw context value (preserving its type);
// otherwise each token is interpolated into the rendered string.
//
// Namespaces route the key to a source projection: the bare form {{key}} reads
// the flat Data union; {{input.*}}, {{output.*}} and {{global.*}} read the
// corresponding projections.
func resolve_service_template(value any, ctx *engine.ProcessContext) (any, error) {
	text, ok := value.(string)
	if !ok || !strings.Contains(text, "{{") {
		return value, nil
	}
	matches := service_template_re.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return value, nil
	}
	if len(matches) == 1 {
		start, end := matches[0][0], matches[0][1]
		if start == 0 && end == len(text) {
			namespace, key := service_template_token(text, matches[0])
			resolved, err := service_template_lookup(ctx, namespace, key)
			if err != nil {
				return nil, err
			}
			return resolved, nil
		}
	}
	for _, match := range matches {
		namespace, key := service_template_token(text, match)
		if _, err := service_template_lookup(ctx, namespace, key); err != nil {
			return nil, err
		}
	}
	replaced := service_template_re.ReplaceAllStringFunc(text, func(match string) string {
		sub := service_template_re.FindStringSubmatch(match)
		namespace, key := service_template_namespace_key(sub[1], sub[2])
		value, _ := service_template_lookup(ctx, namespace, key)
		return fmt.Sprint(value)
	})
	return replaced, nil
}

// service_template_token extracts the (namespace, key) pair from a matched
// token's submatch index pairs. Group 1 is the namespace when a dotted key is
// present, otherwise it is the bare key; group 2 (if present) is the key.
func service_template_token(text string, match []int) (namespace, key string) {
	first := text[match[2]:match[3]]
	second := ""
	if match[4] >= 0 {
		second = text[match[4]:match[5]]
	}
	return service_template_namespace_key(first, second)
}

func service_template_namespace_key(first, second string) (namespace, key string) {
	if second == "" {
		return "", first
	}
	return first, second
}

func service_template_lookup(ctx *engine.ProcessContext, namespace, key string) (any, error) {
	var source map[string]any
	switch namespace {
	case "":
		source = ctx.Data
	case "input":
		source = ctx.Inputs
	case "output":
		source = ctx.Outputs
	case "global":
		source = ctx.Globals
	default:
		return nil, fmt.Errorf("unknown template namespace: %s", namespace)
	}
	value, exists := source[key]
	if !exists {
		if namespace == "" {
			return nil, fmt.Errorf("context value not found: %s", key)
		}
		return nil, fmt.Errorf("context value not found: %s.%s", namespace, key)
	}
	return value, nil
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
