package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"wx_channel/pkg/configapi"
)

// ScopedConfig gives a module read-only access to one configuration namespace.
// It intentionally exposes only typed reads and cannot reach sibling modules.
type ScopedConfig struct {
	snapshot configapi.Snapshot
}

func NewScopedConfig(snapshot configapi.Snapshot) *ScopedConfig {
	return &ScopedConfig{snapshot: snapshot}
}

func (s *ScopedConfig) GetString(key string) string {
	value, _ := s.get(key)
	if value == nil {
		return ""
	}
	if typed, ok := value.(string); ok {
		return typed
	}
	return fmt.Sprint(value)
}

func (s *ScopedConfig) GetBool(key string) bool {
	value, _ := s.get(key)
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, _ := strconv.ParseBool(strings.TrimSpace(typed))
		return parsed
	default:
		number, ok := scoped_number(value)
		return ok && number != 0
	}
}

func (s *ScopedConfig) GetInt(key string) int {
	return int(s.GetInt64(key))
}

func (s *ScopedConfig) GetInt64(key string) int64 {
	value, _ := s.get(key)
	if typed, ok := value.(string); ok {
		parsed, _ := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return parsed
	}
	number, _ := scoped_number(value)
	return int64(number)
}

func (s *ScopedConfig) GetFloat64(key string) float64 {
	value, _ := s.get(key)
	if typed, ok := value.(string); ok {
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed
	}
	number, _ := scoped_number(value)
	return number
}

func (s *ScopedConfig) GetDuration(key string) time.Duration {
	value, _ := s.get(key)
	if typed, ok := value.(string); ok {
		parsed, _ := time.ParseDuration(strings.TrimSpace(typed))
		return parsed
	}
	return time.Duration(s.GetInt64(key))
}

func (s *ScopedConfig) GetStringSlice(key string) []string {
	value, _ := s.get(key)
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		result := make([]string, len(typed))
		for index, item := range typed {
			result[index] = fmt.Sprint(item)
		}
		return result
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		parts := strings.Split(typed, ",")
		for index := range parts {
			parts[index] = strings.TrimSpace(parts[index])
		}
		return parts
	default:
		return nil
	}
}

func (s *ScopedConfig) GetStringMap(key string) map[string]interface{} {
	value, _ := s.get(key)
	values, ok := value.(map[string]any)
	if !ok {
		return map[string]interface{}{}
	}
	result := make(map[string]interface{}, len(values))
	for item_key, item_value := range values {
		result[item_key] = item_value
	}
	return result
}

func (s *ScopedConfig) IsSet(key string) bool {
	_, exists := s.get(key)
	return exists
}

func (s *ScopedConfig) get(key string) (any, bool) {
	if s == nil {
		return nil, false
	}
	var current any = s.snapshot.Values()
	for _, part := range strings.Split(strings.Trim(strings.TrimSpace(key), "."), ".") {
		values, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = scoped_map_value(values, part)
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func scoped_map_value(values map[string]any, key string) (any, bool) {
	if value, exists := values[key]; exists {
		return value, true
	}
	for candidate, value := range values {
		if strings.EqualFold(candidate, key) {
			return value, true
		}
	}
	return nil, false
}

func scoped_number(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case float32:
		return float64(typed), true
	case float64:
		return typed, true
	default:
		return 0, false
	}
}
