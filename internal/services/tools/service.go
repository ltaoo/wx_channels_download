// Package tools provides the process-local tool catalog and execution service
// shared by MCP, workflow nodes, and command-line callers.
package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

var ErrUnknownTool = errors.New("未知工具")

//go:embed catalog.json
var builtin_catalog_json []byte

// Definition is the transport-neutral declaration of one service tool.
type Definition struct {
	Name        string         `json:"name"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
	Annotations map[string]any `json:"annotations,omitempty"`
	FormSchema  []FormField    `json:"form_schema"`
}

// FormField describes how one tool argument is rendered by schema-driven UIs.
type FormField struct {
	Name        string       `json:"name"`
	Label       string       `json:"label"`
	Description string       `json:"description,omitempty"`
	Type        string       `json:"type"`
	Format      string       `json:"format,omitempty"`
	Control     string       `json:"control"`
	Required    bool         `json:"required"`
	Default     any          `json:"default,omitempty"`
	HasDefault  bool         `json:"has_default,omitempty"`
	Minimum     any          `json:"minimum,omitempty"`
	Maximum     any          `json:"maximum,omitempty"`
	Options     []FormOption `json:"options,omitempty"`
}

// FormOption is one selectable enum value in a form field.
type FormOption struct {
	Label string `json:"label"`
	Value any    `json:"value"`
}

// ExecuteFunc handles one registered tool using its JSON-encoded arguments.
type ExecuteFunc func(context.Context, string, json.RawMessage) (map[string]any, error)

// SupportsFunc decides whether a tool is available in the current runtime.
type SupportsFunc func(string) bool

// Service owns the canonical runtime tool list and its execution entry point.
type Service struct {
	definitions []Definition
	by_name     map[string]Definition
	execute     ExecuteFunc
}

// New constructs a tool service from declarative tool metadata.
func New(raw_definitions []any, supports SupportsFunc, execute ExecuteFunc) (*Service, error) {
	definitions, err := parse_definitions(raw_definitions, supports)
	if err != nil {
		return nil, err
	}
	if execute == nil {
		return nil, errors.New("工具执行器不能为空")
	}
	by_name := make(map[string]Definition, len(definitions))
	for _, definition := range definitions {
		if _, exists := by_name[definition.Name]; exists {
			return nil, fmt.Errorf("工具名称重复: %s", definition.Name)
		}
		by_name[definition.Name] = definition
	}
	return &Service{definitions: definitions, by_name: by_name, execute: execute}, nil
}

// NewBuiltin constructs the application tool service from the catalog declared
// in this package. Protocol adapters must use this constructor instead of
// maintaining their own tool list.
func NewBuiltin(supports SupportsFunc, execute ExecuteFunc) (*Service, error) {
	raw_definitions, err := builtin_declarations()
	if err != nil {
		return nil, err
	}
	return New(raw_definitions, supports, execute)
}

// Catalog normalizes declarative metadata for consumers that only need the
// full static catalog, such as the workflow editor.
func Catalog(raw_definitions []any) []Definition {
	definitions, _ := parse_definitions(raw_definitions, nil)
	return definitions
}

// BuiltinCatalog returns the complete application tool catalog, independent of
// which backends happen to be enabled in one process.
func BuiltinCatalog() []Definition {
	raw_definitions, err := builtin_declarations()
	if err != nil {
		return []Definition{}
	}
	return Catalog(raw_definitions)
}

// Definitions returns the tools enabled for this service instance.
func (s *Service) Definitions() []Definition {
	if s == nil {
		return []Definition{}
	}
	result := make([]Definition, len(s.definitions))
	copy(result, s.definitions)
	return result
}

// Names returns enabled tool names in declaration order.
func (s *Service) Names() []string {
	definitions := s.Definitions()
	names := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		names = append(names, definition.Name)
	}
	return names
}

// MCPDefinitions renders the service catalog in MCP tools/list format.
func (s *Service) MCPDefinitions() []any {
	definitions := s.Definitions()
	result := make([]any, 0, len(definitions))
	for _, definition := range definitions {
		tool := map[string]any{
			"name":        definition.Name,
			"title":       definition.Title,
			"description": definition.Description,
			"inputSchema": definition.InputSchema,
		}
		if len(definition.Annotations) > 0 {
			tool["annotations"] = definition.Annotations
		}
		result = append(result, tool)
	}
	return result
}

// Call executes a tool and preserves its MCP-compatible result envelope.
func (s *Service) Call(ctx context.Context, name string, arguments json.RawMessage) (map[string]any, error) {
	if s == nil {
		return nil, errors.New("工具服务未初始化")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("工具名称不能为空")
	}
	if _, exists := s.by_name[name]; !exists {
		return nil, fmt.Errorf("%w: %s", ErrUnknownTool, name)
	}
	if len(arguments) == 0 {
		arguments = json.RawMessage("{}")
	}
	return s.execute(ctx, name, arguments)
}

// Execute invokes a tool for process-local callers and unwraps structured MCP
// results so workflow and CLI consumers receive the domain result directly.
func (s *Service) Execute(ctx context.Context, name string, arguments map[string]any) (any, error) {
	if arguments == nil {
		arguments = map[string]any{}
	}
	raw_arguments, err := json.Marshal(arguments)
	if err != nil {
		return nil, fmt.Errorf("编码工具参数失败: %w", err)
	}
	result, err := s.Call(ctx, name, raw_arguments)
	if err != nil {
		return nil, err
	}
	if structured, ok := result["structuredContent"]; ok {
		return structured, nil
	}
	return result, nil
}

func parse_definitions(raw_definitions []any, supports SupportsFunc) ([]Definition, error) {
	definitions := make([]Definition, 0, len(raw_definitions))
	seen := make(map[string]bool, len(raw_definitions))
	for _, raw_definition := range raw_definitions {
		tool := json_object(raw_definition)
		name, _ := tool["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if seen[name] {
			return nil, fmt.Errorf("工具名称重复: %s", name)
		}
		seen[name] = true
		if supports != nil && !supports(name) {
			continue
		}
		title, _ := tool["title"].(string)
		description, _ := tool["description"].(string)
		input_schema := json_object(tool["inputSchema"])
		if input_schema == nil {
			input_schema = map[string]any{
				"type":                 "object",
				"additionalProperties": false,
			}
		}
		definitions = append(definitions, Definition{
			Name:        name,
			Title:       title,
			Description: description,
			InputSchema: input_schema,
			Annotations: json_object(tool["annotations"]),
			FormSchema:  form_schema(input_schema),
		})
	}
	return definitions, nil
}

func form_schema(input_schema map[string]any) []FormField {
	properties := json_object(input_schema["properties"])
	if len(properties) == 0 {
		return []FormField{}
	}
	required_names := map[string]bool{}
	switch required := input_schema["required"].(type) {
	case []string:
		for _, name := range required {
			required_names[name] = true
		}
	case []any:
		for _, value := range required {
			if name, ok := value.(string); ok {
				required_names[name] = true
			}
		}
	}
	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}
	sort.SliceStable(names, func(left_index int, right_index int) bool {
		left_required := required_names[names[left_index]]
		right_required := required_names[names[right_index]]
		if left_required != right_required {
			return left_required
		}
		return names[left_index] < names[right_index]
	})

	fields := make([]FormField, 0, len(names))
	for _, name := range names {
		property := json_object(properties[name])
		field_type, _ := property["type"].(string)
		format, _ := property["format"].(string)
		description, _ := property["description"].(string)
		label, _ := property["title"].(string)
		if strings.TrimSpace(label) == "" {
			label = name
		}
		control := "input"
		options := form_options(property["enum"])
		if len(options) > 0 {
			control = "select"
		} else {
			switch field_type {
			case "boolean":
				control = "checkbox"
			case "array", "object":
				control = "textarea"
			}
		}
		default_value, has_default := property["default"]
		fields = append(fields, FormField{
			Name:        name,
			Label:       label,
			Description: description,
			Type:        field_type,
			Format:      format,
			Control:     control,
			Required:    required_names[name],
			Default:     default_value,
			HasDefault:  has_default,
			Minimum:     property["minimum"],
			Maximum:     property["maximum"],
			Options:     options,
		})
	}
	return fields
}

func form_options(raw_options any) []FormOption {
	values := make([]any, 0)
	switch options := raw_options.(type) {
	case []string:
		for _, value := range options {
			values = append(values, value)
		}
	case []any:
		values = append(values, options...)
	}
	result := make([]FormOption, 0, len(values))
	for _, value := range values {
		result = append(result, FormOption{Label: fmt.Sprint(value), Value: value})
	}
	return result
}

func json_object(value any) map[string]any {
	if value == nil {
		return nil
	}
	if object, ok := value.(map[string]any); ok {
		return object
	}
	raw_value, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var object map[string]any
	if err := json.Unmarshal(raw_value, &object); err != nil {
		return nil
	}
	return object
}

func builtin_declarations() ([]any, error) {
	var definitions []any
	if err := json.Unmarshal(builtin_catalog_json, &definitions); err != nil {
		return nil, fmt.Errorf("解析内置工具 catalog 失败: %w", err)
	}
	return definitions, nil
}
