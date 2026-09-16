package mcpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrUnknownTool reports a tool name that is not routable in this runtime.
var ErrUnknownTool = errors.New("未知工具")

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

// decode_json_object turns a raw JSON object declaration into the map form the
// MCP and workflow transports publish. Empty input decodes to nil so absent
// annotations stay absent.
//
// It is json_object with a signature that admits only json.RawMessage: the
// permissive any-typed variant would base64-encode a plain []byte and silently
// yield nil.
func decode_json_object(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil
	}
	return object
}
