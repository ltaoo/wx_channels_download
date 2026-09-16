package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// tool declares one business tool completely: the metadata published over
// tools/list, the predicate that decides whether it is available in the current
// runtime, and the handler that executes it. Every tool is declared exactly
// once, next to the file that implements its handler, so a renamed or deleted
// handler is a compile error rather than a metadata drift.
type tool struct {
	name         string
	title        string
	description  string
	input_schema json.RawMessage
	annotations  json.RawMessage
	supports     func(*ToolSet) bool
	handle       tool_handler
}

// tool_handler executes one tool with its JSON-encoded arguments.
type tool_handler func(*ToolSet, context.Context, json.RawMessage) (map[string]any, error)

// without_arguments adapts a context-only handler to tool_handler.
func without_arguments(handler func(*ToolSet, context.Context) (map[string]any, error)) tool_handler {
	return func(toolset *ToolSet, ctx context.Context, _ json.RawMessage) (map[string]any, error) {
		return handler(toolset, ctx)
	}
}

// Availability predicates. Each body is transcribed verbatim from the
// supports_tool switch this registry replaces, so backend wiring changes are
// never silently reinterpreted.

func supports_api_client(s *ToolSet) bool {
	return s.api_client != nil
}

func supports_scraper(s *ToolSet) bool {
	return s.scraper_jobs != nil || s.api_client != nil
}

func supports_download_content(s *ToolSet) bool {
	return (s.scraper_jobs != nil || s.api_client != nil) && (s.download_task_creator != nil || s.api_client != nil)
}

// supports_wxchannels keeps the api_client arm as load-bearing: ToolSet.Register
// materializes the supported set once, at construction, which on the HTTP host
// happens before the adapter is registered. Dropping the OR would permanently
// hide these tools from the api_client-only CLI runtime.
func supports_wxchannels(s *ToolSet) bool {
	return s.wxchannels != nil || s.api_client != nil
}

func supports_wxchannels_download(s *ToolSet) bool {
	return (s.wxchannels != nil || s.api_client != nil) && (s.download_task_creator != nil || s.api_client != nil)
}

func supports_data(s *ToolSet) bool {
	return s.data_reader != nil || s.api_client != nil
}

func supports_task_delete(s *ToolSet) bool {
	return s.download_task_deleter != nil
}

func supports_task_create(s *ToolSet) bool {
	return s.download_task_creator != nil || s.api_client != nil
}

func supports_wxmp(s *ToolSet) bool {
	return s.wxmp != nil
}

func supports_sph(s *ToolSet) bool {
	return s.sph_deployer != nil
}

func supports_zhihu(s *ToolSet) bool {
	return s.zhihu_collections != nil && s.zhihu_credentials != nil
}

func supports_automation(s *ToolSet) bool {
	return s.automation != nil
}

// concat_tools flattens the per-domain declaration slices. Concatenation is the
// registration step: a tools_* slice that tool_declarations does not list is
// silently ignored, and neither validate_tool_registry() nor the completeness
// test can see it.
func concat_tools(groups ...[]tool) []tool {
	total := 0
	for _, group := range groups {
		total += len(group)
	}
	declarations := make([]tool, 0, total)
	for _, group := range groups {
		declarations = append(declarations, group...)
	}
	return declarations
}

// tool_declarations is the registration point and the published tool order.
// Adding a tool means adding one row to the tools_* slice of the file that
// declares its handler, and listing that slice here if it is not already
// listed.
var tool_declarations = concat_tools(
	tools_app,
	tools_scraper,
	tools_download_task,
	tools_wxchannels,
	tools_wxmp,
	tools_sph,
	tools_zhihu,
	tools_automation,
	tools_data,
)

func build_tool_registry(declarations []tool) map[string]tool {
	registry := make(map[string]tool, len(declarations))
	for _, declaration := range declarations {
		registry[declaration.name] = declaration
	}
	return registry
}

// tool_registry resolves at package init; callers verify with
// validate_tool_registry.
var tool_registry = build_tool_registry(tool_declarations)

// definition materializes the published metadata for one tool. The raw JSON is
// decoded here so encoding/json's key ordering — not the literal's byte order —
// decides what tools/list and `tool list` emit.
func (t tool) definition() Definition {
	input_schema := decode_json_object(t.input_schema)
	return Definition{
		Name:        t.name,
		Title:       t.title,
		Description: t.description,
		InputSchema: input_schema,
		Annotations: decode_json_object(t.annotations),
		FormSchema:  form_schema(input_schema),
	}
}

// tool_definitions holds the materialized metadata for every declared tool,
// index-aligned with tool_declarations. It is computed once at package init and
// treated as read-only by all callers.
var tool_definitions = build_tool_definitions(tool_declarations)

func build_tool_definitions(declarations []tool) []Definition {
	definitions := make([]Definition, 0, len(declarations))
	for _, declaration := range declarations {
		definitions = append(definitions, declaration.definition())
	}
	return definitions
}

// validate_tool_registry reports duplicate, empty, or incomplete declarations.
func validate_tool_registry() error {
	if len(tool_registry) != len(tool_declarations) {
		return fmt.Errorf("工具声明名称重复: %d 行映射到 %d 个名称", len(tool_declarations), len(tool_registry))
	}
	if len(tool_definitions) != len(tool_declarations) {
		return fmt.Errorf("工具定义数量 %d 与声明数量 %d 不一致", len(tool_definitions), len(tool_declarations))
	}
	for index, declaration := range tool_declarations {
		if declaration.name == "" {
			return fmt.Errorf("工具声明缺少名称")
		}
		if len(declaration.input_schema) == 0 {
			return fmt.Errorf("工具 %s 缺少参数 schema", declaration.name)
		}
		if declaration.supports == nil {
			return fmt.Errorf("工具 %s 缺少可用性判断", declaration.name)
		}
		if declaration.handle == nil {
			return fmt.Errorf("工具 %s 缺少执行器", declaration.name)
		}
		// tool_definitions is paired with tool_declarations by index, and a
		// schema that failed to decode would publish as null. Both are silent
		// until a transport encodes them, so catch them here.
		definition := tool_definitions[index]
		if definition.Name != declaration.name {
			return fmt.Errorf("工具定义第 %d 行是 %s，与声明 %s 不对应", index, definition.Name, declaration.name)
		}
		if definition.InputSchema == nil {
			return fmt.Errorf("工具 %s 的参数 schema 无法解析", declaration.name)
		}
	}
	return nil
}

func (s *ToolSet) supports_tool(name string) bool {
	if s == nil {
		return false
	}
	declaration, exists := tool_registry[name]
	return exists && declaration.supports(s)
}

func (s *ToolSet) execute_tool(ctx context.Context, name string, raw_arguments json.RawMessage) (map[string]any, error) {
	declaration, exists := tool_registry[name]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrUnknownTool, name)
	}
	return declaration.handle(s, ctx, raw_arguments)
}

// call is the dispatch entry point for every transport. Availability is gated
// by supports_tool, not by registry membership: a tool that is declared but not
// supported in this runtime must be indistinguishable from an unknown one.
func (s *ToolSet) call(ctx context.Context, name string, raw_arguments json.RawMessage) (map[string]any, error) {
	if s == nil {
		return nil, errors.New("工具服务未初始化")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("工具名称不能为空")
	}
	if !s.supports_tool(name) {
		return nil, fmt.Errorf("%w: %s", ErrUnknownTool, name)
	}
	if len(raw_arguments) == 0 {
		raw_arguments = json.RawMessage("{}")
	}
	return s.execute_tool(ctx, name, raw_arguments)
}
