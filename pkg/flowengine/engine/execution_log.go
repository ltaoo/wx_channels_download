package engine

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"unicode/utf8"
)

const log_section_bytes = 4096

// Compact only audit copies; execution data and recovery checkpoints stay complete.
func compact_node_execution_log(entry NodeExecutionLog) NodeExecutionLog {
	entry.Input = compact_log_map(entry.Input)
	entry.Behavior = compact_log_map(entry.Behavior)
	entry.Output = compact_log_map(entry.Output)
	return entry
}

func compact_log_map(values map[string]interface{}) map[string]interface{} {
	if values == nil {
		return nil
	}
	return compact_log_value("", values, 0, log_section_bytes).(map[string]interface{})
}

func compact_log_value(key string, value interface{}, depth, budget int) interface{} {
	if is_sensitive_log_key(key) {
		return "[REDACTED]"
	}
	switch typed_value := value.(type) {
	case string:
		limit := 256
		if available := (budget - 64) / 6; available < limit {
			limit = available
		}
		if len(typed_value) <= limit {
			return typed_value
		}
		for limit > 0 && !utf8.ValidString(typed_value[:limit]) {
			limit--
		}
		return fmt.Sprintf("%s… [%d bytes]", typed_value[:limit], len(typed_value))
	case map[string]interface{}:
		if depth >= 3 && len(typed_value) > 0 {
			return map[string]interface{}{"__summary__": "object", "fields": len(typed_value)}
		}
		keys := make([]string, 0, len(typed_value))
		for field := range typed_value {
			keys = append(keys, field)
		}
		sort.Slice(keys, func(left, right int) bool {
			left_priority := log_value_priority(typed_value[keys[left]])
			right_priority := log_value_priority(typed_value[keys[right]])
			if left_priority != right_priority {
				return left_priority < right_priority
			}
			return keys[left] < keys[right]
		})
		result := map[string]interface{}{}
		remaining := budget - 64
		for _, field := range keys {
			if len(result) == 16 || remaining < 128 {
				break
			}
			raw_key, _ := json.Marshal(field)
			available := remaining - len(raw_key) - 2
			if available < 128 {
				continue
			}
			summary := compact_log_value(field, typed_value[field], depth+1, available)
			raw_value, _ := json.Marshal(summary)
			result[field] = summary
			remaining -= len(raw_key) + len(raw_value) + 2
		}
		if omitted := len(keys) - len(result); omitted > 0 {
			result["__omitted_fields__"] = omitted
		}
		return result
	case []interface{}:
		if typed_value == nil {
			return nil
		}
		if depth >= 3 && len(typed_value) > 0 {
			return map[string]interface{}{"__summary__": "array", "count": len(typed_value)}
		}
		items := make([]interface{}, 0, 3)
		remaining := budget - 64
		for _, item := range typed_value {
			if len(items) == 3 || remaining < 128 {
				break
			}
			summary := compact_log_value("", item, depth+1, remaining)
			raw, _ := json.Marshal(summary)
			items = append(items, summary)
			remaining -= len(raw) + 1
		}
		if len(items) < len(typed_value) {
			return map[string]interface{}{"__summary__": "array", "count": len(typed_value), "sample": items}
		}
		return items
	case nil, bool,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64, uintptr,
		float32, float64, json.Number:
		raw, err := json.Marshal(typed_value)
		if err != nil || len(raw) > budget {
			return "[OMITTED]"
		}
		return typed_value
	default:
		raw, err := json.Marshal(typed_value)
		if err != nil {
			return fmt.Sprintf("[UNSERIALIZABLE:%T]", typed_value)
		}
		var normalized interface{}
		if json.Unmarshal(raw, &normalized) != nil {
			return fmt.Sprintf("[UNSERIALIZABLE:%T]", typed_value)
		}
		return compact_log_value(key, normalized, depth, budget)
	}
}

func log_value_priority(value interface{}) int {
	if text, ok := value.(string); ok && len(text) > 256 {
		return 1
	}
	switch reflect.ValueOf(value).Kind() {
	case reflect.Map, reflect.Slice, reflect.Array, reflect.Struct, reflect.Ptr:
		return 1
	default:
		return 0
	}
}
