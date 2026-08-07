package configapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

func normalize_key(key string) string {
	parts := strings.Split(strings.Trim(strings.TrimSpace(key), "."), ".")
	result := parts[:0]
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return strings.Join(result, ".")
}

func infer_value_type(value any) ValueType {
	switch value.(type) {
	case bool:
		return TypeBool
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return TypeInt
	case float32, float64:
		return TypeFloat
	case []string, []any:
		return TypeStringSlice
	case map[string]any, map[any]any:
		return TypeObject
	default:
		return TypeString
	}
}

func coerce_value(value_type ValueType, value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	switch value_type {
	case TypeString, TypeSelect, TypeFile, TypeText:
		if string_value, ok := value.(string); ok {
			return string_value, nil
		}
		return fmt.Sprint(value), nil
	case TypeBool:
		switch typed := value.(type) {
		case bool:
			return typed, nil
		case string:
			parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
			if err != nil {
				return nil, errors.New("must be a boolean")
			}
			return parsed, nil
		default:
			if number, ok := numeric_value(value); ok {
				return number != 0, nil
			}
			return nil, errors.New("must be a boolean")
		}
	case TypeInt:
		switch typed := value.(type) {
		case string:
			parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
			if err != nil {
				return nil, errors.New("must be an integer")
			}
			return int(parsed), nil
		default:
			number, ok := numeric_value(value)
			if !ok || number != float64(int64(number)) {
				return nil, errors.New("must be an integer")
			}
			return int(number), nil
		}
	case TypeFloat:
		if typed, ok := value.(string); ok {
			parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
			if err != nil {
				return nil, errors.New("must be a number")
			}
			return parsed, nil
		}
		number, ok := numeric_value(value)
		if !ok {
			return nil, errors.New("must be a number")
		}
		return number, nil
	case TypeStringSlice:
		switch typed := value.(type) {
		case []string:
			return append([]string(nil), typed...), nil
		case []any:
			result := make([]string, len(typed))
			for index, item := range typed {
				result[index] = fmt.Sprint(item)
			}
			return result, nil
		case string:
			if strings.TrimSpace(typed) == "" {
				return []string{}, nil
			}
			parts := strings.Split(typed, ",")
			for index := range parts {
				parts[index] = strings.TrimSpace(parts[index])
			}
			return parts, nil
		default:
			return nil, errors.New("must be a string array")
		}
	case TypeObject:
		value_map, ok := to_string_map(value)
		if !ok {
			return nil, errors.New("must be an object")
		}
		return clone_values(value_map)
	default:
		return nil, fmt.Errorf("unsupported type %q", value_type)
	}
}

func numeric_value(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case float32:
		return float64(typed), true
	case float64:
		return typed, true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

func set_path(values map[string]any, path string, value any) {
	parts := strings.Split(normalize_key(path), ".")
	if len(parts) == 0 || parts[0] == "" {
		return
	}
	current := values
	for _, part := range parts[:len(parts)-1] {
		next_value, ok := lookup_key(current, part)
		if ok {
			if next_map, valid := to_string_map(next_value); valid {
				current[canonical_map_key(current, part)] = next_map
				current = next_map
				continue
			}
		}
		next_map := make(map[string]any)
		current[part] = next_map
		current = next_map
	}
	current[canonical_map_key(current, parts[len(parts)-1])] = value
}

func delete_path(values map[string]any, path string) {
	parts := strings.Split(normalize_key(path), ".")
	if len(parts) == 0 || parts[0] == "" {
		return
	}
	current := values
	parents := make([]map[string]any, 0, len(parts))
	keys := make([]string, 0, len(parts))
	for _, part := range parts[:len(parts)-1] {
		key := canonical_map_key(current, part)
		next, ok := current[key]
		if !ok {
			return
		}
		next_map, ok := to_string_map(next)
		if !ok {
			return
		}
		parents = append(parents, current)
		keys = append(keys, key)
		current = next_map
	}
	delete(current, canonical_map_key(current, parts[len(parts)-1]))
	for index := len(parents) - 1; index >= 0; index-- {
		if len(current) != 0 {
			break
		}
		delete(parents[index], keys[index])
		current = parents[index]
	}
}

func canonical_map_key(values map[string]any, key string) string {
	for candidate := range values {
		if strings.EqualFold(candidate, key) {
			return candidate
		}
	}
	return key
}

func merge_values(target map[string]any, source map[string]any, source_name string, provenance map[string]string, prefix string) {
	for key, value := range source {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		value_map, is_map := to_string_map(value)
		if is_map {
			target_key := canonical_map_key(target, key)
			target_value, exists := target[target_key]
			target_map, target_is_map := to_string_map(target_value)
			if !exists || !target_is_map {
				target_map = make(map[string]any)
				target[target_key] = target_map
				clear_provenance_prefix(provenance, path)
			}
			merge_values(target_map, value_map, source_name, provenance, path)
			continue
		}
		clear_provenance_prefix(provenance, path)
		target[canonical_map_key(target, key)] = value
		provenance[normalize_key(path)] = source_name
	}
}

func clear_provenance_prefix(provenance map[string]string, prefix string) {
	prefix = normalize_key(prefix)
	for key := range provenance {
		if strings.EqualFold(key, prefix) || strings.HasPrefix(strings.ToLower(key), strings.ToLower(prefix)+".") {
			delete(provenance, key)
		}
	}
}

func flatten_values(values map[string]any) map[string]any {
	flat := make(map[string]any)
	var walk func(map[string]any, string)
	walk = func(current map[string]any, prefix string) {
		keys := make([]string, 0, len(current))
		for key := range current {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			path := key
			if prefix != "" {
				path = prefix + "." + key
			}
			if nested, ok := to_string_map(current[key]); ok {
				walk(nested, path)
				continue
			}
			flat[normalize_key(path)] = current[key]
		}
	}
	walk(values, "")
	return flat
}

func equal_values(left, right any) bool {
	return reflect.DeepEqual(normalize_json_value(left), normalize_json_value(right))
}

func normalize_json_value(value any) any {
	data, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var normalized any
	if err := json.Unmarshal(data, &normalized); err != nil {
		return value
	}
	return normalized
}
