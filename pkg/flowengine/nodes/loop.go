package nodes

import (
	"errors"
	"fmt"
	"reflect"

	"wx_channel/pkg/flowengine/engine"
)

type LoopNode struct {
	Id     string
	Config map[string]interface{}
}

func NewLoopNode(config map[string]interface{}) engine.Node {
	id, _ := config["id"].(string)
	return &LoopNode{Id: id, Config: config}
}

func (n *LoopNode) ID() string   { return n.Id }
func (n *LoopNode) Type() string { return "LoopNode" }

func (n *LoopNode) Execute(ctx *engine.ProcessContext) (bool, []string, error) {
	iterableKey, _ := n.Config["iterable_key"].(string)
	if iterableKey == "" {
		return false, nil, errors.New("missing iterable_key")
	}

	rawIterable, ok := ctx.Data[iterableKey]
	if !ok {
		return false, nil, fmt.Errorf("missing iterable data for key %q", iterableKey)
	}
	items, err := toInterfaceSlice(rawIterable)
	if err != nil {
		return false, nil, err
	}

	workflow, ok := n.Config["workflow"].(engine.FlowDefinition)
	if !ok {
		return false, nil, errors.New("missing workflow")
	}

	itemKey, _ := n.Config["item_key"].(string)
	rawItemFields, _ := n.Config["item_fields"].(map[string]interface{})
	itemFields := map[string]string{}
	for target, source := range rawItemFields {
		if src, ok := source.(string); ok {
			itemFields[target] = src
		}
	}
	cleanupKeys := toStringSlice(n.Config["cleanup_keys"])
	continueOnError, _ := n.Config["continue_on_error"].(bool)

	workflowNode := NewWorkflowNode(map[string]interface{}{"workflow": workflow})

	for index, item := range items {
		snapshot := map[string]struct {
			exists bool
			value  interface{}
		}{}

		restore := func() {
			for key, state := range snapshot {
				if state.exists {
					ctx.Data[key] = state.value
				} else {
					delete(ctx.Data, key)
				}
			}
			for _, key := range cleanupKeys {
				if _, ok := snapshot[key]; ok {
					continue
				}
				delete(ctx.Data, key)
			}
		}

		saveAndSet := func(key string, value interface{}) {
			if _, exists := snapshot[key]; !exists {
				stateValue, hasValue := ctx.Data[key]
				snapshot[key] = struct {
					exists bool
					value  interface{}
				}{
					exists: hasValue,
					value:  stateValue,
				}
			}
			ctx.Data[key] = value
		}

		if itemKey != "" {
			saveAndSet(itemKey, item)
		}

		itemContext, err := extractLoopItemContext(item, itemFields)
		if err != nil {
			restore()
			return false, nil, fmt.Errorf("loop iteration %d: %w", index, err)
		}
		for targetKey, value := range itemContext {
			saveAndSet(targetKey, value)
		}

		_, _, err = workflowNode.Execute(ctx)
		restore()
		if err != nil {
			if continueOnError {
				continue
			}
			return false, nil, fmt.Errorf("loop iteration %d failed: %w", index, err)
		}
	}

	next := ctx.EngineRef.GetNextNodeIDsFromDefinition(ctx, n.Id)
	return true, next, nil
}

func extractLoopItemContext(item interface{}, itemFields map[string]string) (map[string]interface{}, error) {
	if len(itemFields) > 0 {
		fields := map[string]interface{}{}
		for targetKey, sourceKey := range itemFields {
			value, ok := getItemField(item, sourceKey)
			if !ok {
				continue
			}
			fields[targetKey] = value
		}
		return fields, nil
	}

	switch typed := item.(type) {
	case map[string]interface{}:
		contextFields := make(map[string]interface{}, len(typed))
		for key, value := range typed {
			contextFields[key] = value
		}
		return contextFields, nil
	case map[string]string:
		contextFields := make(map[string]interface{}, len(typed))
		for key, value := range typed {
			contextFields[key] = value
		}
		return contextFields, nil
	case map[interface{}]interface{}:
		contextFields := map[string]interface{}{}
		for rawKey, value := range typed {
			key, ok := rawKey.(string)
			if !ok {
				continue
			}
			contextFields[key] = value
		}
		return contextFields, nil
	default:
		if len(itemFields) > 0 {
			return nil, fmt.Errorf("unsupported item type %T", item)
		}
		return nil, fmt.Errorf("unsupported loop item type %T, please define item_fields", item)
	}
}

func getItemField(item interface{}, key string) (interface{}, bool) {
	switch typed := item.(type) {
	case map[string]interface{}:
		value, ok := typed[key]
		return value, ok
	case map[string]string:
		value, ok := typed[key]
		if !ok {
			return nil, false
		}
		return value, true
	case map[interface{}]interface{}:
		value, ok := typed[key]
		if ok {
			return value, true
		}
		for rawKey, rawValue := range typed {
			if rawKeyStr, ok := rawKey.(string); ok && rawKeyStr == key {
				return rawValue, true
			}
		}
		return nil, false
	}

	rv := reflect.ValueOf(item)
	if !rv.IsValid() {
		return nil, false
	}
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil, false
	}
	field := rv.FieldByName(key)
	if !field.IsValid() {
		return nil, false
	}
	return field.Interface(), true
}

func toInterfaceSlice(values interface{}) ([]interface{}, error) {
	if values == nil {
		return nil, errors.New("iterable is nil")
	}
	switch typed := values.(type) {
	case []interface{}:
		return typed, nil
	}

	rv := reflect.ValueOf(values)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil, fmt.Errorf("iterable must be a slice, got %T", values)
	}

	length := rv.Len()
	out := make([]interface{}, 0, length)
	for i := 0; i < length; i++ {
		out = append(out, rv.Index(i).Interface())
	}
	return out, nil
}

func toStringSlice(value interface{}) []string {
	raw, ok := value.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if str, ok := item.(string); ok {
			out = append(out, str)
		}
	}
	return out
}

