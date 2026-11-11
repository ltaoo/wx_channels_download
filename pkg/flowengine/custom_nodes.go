package flowengine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/rs/zerolog"

	"wx_channel/pkg/flowengine/engine"
	"wx_channel/pkg/flowengine/nodes"
	"wx_channel/pkg/hermes"
)

type CustomNodeConfigKey struct {
	Key         string `json:"key"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

type CustomNodeDefinition struct {
	Type          string                   `json:"type"`
	Name          string                   `json:"name"`
	Description   string                   `json:"description"`
	ConfigKeys    []CustomNodeConfigKey    `json:"config_keys"`
	DefaultConfig map[string]any           `json:"default_config"`
	ConfigSchema  []engine.NodeConfigField `json:"config_schema"`
	script        *hermes.Script
}

// CustomNodeRegistry is loaded once at startup from trusted local JS files.
type CustomNodeRegistry struct {
	definitions map[string]CustomNodeDefinition
	work_dir    string
	logger      *zerolog.Logger
}

var custom_node_type_pattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*Node$`)

// LoadCustomNodes discovers <work_dir>/nodes/*.js and registers their constructors.
// A missing directory is valid; malformed files or duplicate types fail startup.
func LoadCustomNodes(ctx context.Context, flow_engine *FlowEngine, work_dir string, logger *zerolog.Logger) (*CustomNodeRegistry, error) {
	if flow_engine == nil {
		return nil, fmt.Errorf("custom nodes require a flow engine")
	}
	work_dir, err := filepath.Abs(work_dir)
	if err != nil {
		return nil, err
	}
	registry := &CustomNodeRegistry{definitions: map[string]CustomNodeDefinition{}, work_dir: work_dir, logger: logger}
	directory := filepath.Join(work_dir, "nodes")
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return registry, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".js" {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		source, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		script, err := hermes.CompileScript(path, string(source))
		if err != nil {
			return nil, err
		}
		load_ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, err = script.Run(load_ctx, logger, func(vm *goja.Runtime) error {
			_ = vm.Set("workdir", work_dir)
			return vm.Set("registerNode", func(call goja.FunctionCall) goja.Value {
				object, ok := call.Argument(0).(*goja.Object)
				if !ok {
					panic(vm.NewTypeError("registerNode expects a node definition"))
				}
				if _, ok := goja.AssertFunction(object.Get("execute")); !ok {
					panic(vm.NewTypeError("registerNode.execute must be a function"))
				}
				// Functions are not JSON values; copy only the public metadata.
				metadata := map[string]any{}
				for _, key := range []string{"type", "name", "description", "config_keys", "default_config", "config_schema"} {
					value := object.Get(key)
					if value != nil && !goja.IsUndefined(value) {
						metadata[key] = value.Export()
					}
				}
				encoded, encode_err := json.Marshal(metadata)
				var definition CustomNodeDefinition
				if encode_err != nil {
					panic(vm.NewGoError(encode_err))
				}
				if decode_err := json.Unmarshal(encoded, &definition); decode_err != nil {
					panic(vm.NewGoError(decode_err))
				}
				if !custom_node_type_pattern.MatchString(definition.Type) || strings.TrimSpace(definition.Name) == "" {
					panic(vm.NewTypeError("node type must be an identifier ending in Node and name must not be empty"))
				}
				if schema_err := engine.ValidateNodeConfigSchema(definition.ConfigSchema); schema_err != nil {
					panic(vm.NewGoError(schema_err))
				}
				if _, exists := registry.definitions[definition.Type]; exists {
					panic(vm.NewTypeError("duplicate custom node: %s", definition.Type))
				}
				flow_engine.RLock()
				_, reserved := flow_engine.NodeRegistry[definition.Type]
				flow_engine.RUnlock()
				if reserved {
					panic(vm.NewTypeError("node type is reserved: %s", definition.Type))
				}
				if definition.Type == "ServiceNode" {
					panic(vm.NewTypeError("node type is reserved: ServiceNode"))
				}
				definition.script = script
				registry.definitions[definition.Type] = definition
				return goja.Undefined()
			})
		}, func(*goja.Runtime) (goja.Value, error) { return goja.Undefined(), nil })
		cancel()
		if err != nil {
			return nil, fmt.Errorf("load custom nodes %s: %w", path, err)
		}
	}
	for node_type := range registry.definitions {
		flow_engine.RegisterNode(node_type, nodes.NewCustomNodeFactory(node_type, registry.execute))
	}
	return registry, nil
}

func (registry *CustomNodeRegistry) Has(node_type string) bool {
	if registry == nil {
		return false
	}
	_, exists := registry.definitions[node_type]
	return exists
}

func (registry *CustomNodeRegistry) Catalog() []CustomNodeDefinition {
	if registry == nil {
		return nil
	}
	catalog := make([]CustomNodeDefinition, 0, len(registry.definitions))
	for _, definition := range registry.definitions {
		catalog = append(catalog, definition)
	}
	sort.Slice(catalog, func(left, right int) bool { return catalog[left].Type < catalog[right].Type })
	return catalog
}

func (registry *CustomNodeRegistry) execute(ctx context.Context, node_type string, arguments map[string]any, settings map[string]any) (any, error) {
	definition, exists := registry.definitions[node_type]
	if !exists {
		return nil, fmt.Errorf("custom node unavailable: %s", node_type)
	}
	validated_settings := map[string]any{}
	for _, field := range definition.ConfigSchema {
		value := settings[field.Key]
		if text, ok := value.(string); value == nil || (ok && strings.TrimSpace(text) == "") {
			value = field.Default
		}
		if value == nil {
			if field.Required {
				return nil, fmt.Errorf("custom node %s requires configuration: %s", node_type, field.Key)
			}
			continue
		}
		if !engine.ValidNodeConfigValue(field, value) {
			return nil, fmt.Errorf("custom node %s configuration %s must be %s", node_type, field.Key, field.Type)
		}
		validated_settings[field.Key] = value
	}
	encoded_settings, err := json.Marshal(validated_settings)
	if err != nil {
		return nil, fmt.Errorf("encode custom node configuration: %w", err)
	}
	if err := json.Unmarshal(encoded_settings, &validated_settings); err != nil {
		return nil, err
	}
	var execute goja.Callable
	return definition.script.Run(ctx, registry.logger, func(vm *goja.Runtime) error {
		_ = vm.Set("workdir", registry.work_dir)
		_ = vm.Set("__basePath", registry.work_dir)
		return vm.Set("registerNode", func(call goja.FunctionCall) goja.Value {
			object := call.Argument(0).ToObject(vm)
			if object.Get("type").String() == node_type {
				execute, _ = goja.AssertFunction(object.Get("execute"))
			}
			return goja.Undefined()
		})
	}, func(vm *goja.Runtime) (goja.Value, error) {
		if execute == nil {
			return nil, fmt.Errorf("custom node %s did not register execute", node_type)
		}
		return execute(goja.Undefined(), vm.ToValue(arguments), vm.ToValue(validated_settings))
	})
}

// ApplyConfigSchemas keeps registered requirements authoritative for saved flows.
func (registry *CustomNodeRegistry) ApplyConfigSchemas(definition *engine.FlowDefinition) bool {
	return registry.apply_config_schemas(definition, 0)
}

func (registry *CustomNodeRegistry) apply_config_schemas(definition *engine.FlowDefinition, depth int) bool {
	if registry == nil || depth > 16 {
		return false
	}
	changed := false
	for id, node := range definition.Nodes {
		if custom, exists := registry.definitions[node.Type]; exists && custom.ConfigSchema != nil && !reflect.DeepEqual(node.ConfigSchema, custom.ConfigSchema) {
			node.ConfigSchema = append([]engine.NodeConfigField{}, custom.ConfigSchema...)
			changed = true
		}
		if child, ok := node.Config["workflow"].(engine.FlowDefinition); ok {
			changed = registry.apply_config_schemas(&child, depth+1) || changed
			node.Config["workflow"] = child
		}
		definition.Nodes[id] = node
	}
	for id, child := range definition.Subflows {
		changed = registry.apply_config_schemas(&child, depth+1) || changed
		definition.Subflows[id] = child
	}
	return changed
}
