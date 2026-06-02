package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/viper"

	"wx_channel/internal/config"
)

func (s *AdminServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	if s.cfg == nil {
		s.writeError(w, http.StatusInternalServerError, "配置未初始化")
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.writeOK(w, map[string]interface{}{
			"path":   s.cfg.FullPath,
			"schema": config.GetSchema(),
			"values": currentConfigValues(),
		})
	case http.MethodPost:
		s.handleUpdateConfig(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *AdminServer) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	values := body
	if rawValues, ok := body["values"]; ok {
		nestedValues, ok := rawValues.(map[string]interface{})
		if !ok {
			s.writeError(w, http.StatusBadRequest, "values 必须是对象")
			return
		}
		values = nestedValues
	}
	if len(values) == 0 {
		s.writeError(w, http.StatusBadRequest, "缺少配置项")
		return
	}

	schema := config.GetSchema()
	schemaByKey := make(map[string]config.ConfigItem, len(schema))
	for _, item := range schema {
		schemaByKey[item.Key] = item
	}

	updated := make(map[string]interface{}, len(values))
	for key, value := range values {
		item, ok := schemaByKey[key]
		if !ok {
			s.writeError(w, http.StatusBadRequest, fmt.Sprintf("未知配置项: %s", key))
			return
		}
		converted, err := convertConfigValue(item, value)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, fmt.Sprintf("%s: %v", key, err))
			return
		}
		updated[key] = converted
	}

	for key, value := range updated {
		s.cfg.Update(key, value)
	}

	if err := ensureConfigFilePath(s.cfg); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.cfg.Save(); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.cfg.Existing = true

	s.writeOK(w, map[string]interface{}{
		"path":   s.cfg.FullPath,
		"values": currentConfigValues(),
	})
}

func currentConfigValues() map[string]interface{} {
	values := make(map[string]interface{}, len(config.GetSchema()))
	for _, item := range config.GetSchema() {
		values[item.Key] = configValue(item)
	}
	return values
}

func configValue(item config.ConfigItem) interface{} {
	switch item.Type {
	case config.ConfigTypeBool:
		return viper.GetBool(item.Key)
	case config.ConfigTypeInt:
		if isFloatConfig(item) {
			return viper.GetFloat64(item.Key)
		}
		return viper.GetInt(item.Key)
	case config.ConfigTypeText:
		values := viper.GetStringSlice(item.Key)
		if len(values) > 0 {
			return strings.Join(values, "\n")
		}
		return viper.GetString(item.Key)
	default:
		return viper.Get(item.Key)
	}
}

func convertConfigValue(item config.ConfigItem, value interface{}) (interface{}, error) {
	if value == nil {
		return nil, nil
	}
	switch item.Type {
	case config.ConfigTypeBool:
		return toBool(value)
	case config.ConfigTypeInt:
		if isFloatConfig(item) {
			return toFloat64(value)
		}
		return toInt(value)
	case config.ConfigTypeSelect:
		if len(item.Options) > 0 {
			s, err := toString(value)
			if err != nil {
				return nil, err
			}
			for _, option := range item.Options {
				if option == s {
					return s, nil
				}
			}
			return nil, fmt.Errorf("必须是以下选项之一: %s", strings.Join(item.Options, ", "))
		}
		return value, nil
	case config.ConfigTypeString:
		if _, ok := item.Default.([]string); ok {
			return toStringSlice(value)
		}
		return toString(value)
	case config.ConfigTypeFile:
		return toString(value)
	case config.ConfigTypeText:
		if _, ok := item.Default.([]string); ok {
			return toStringSliceByLine(value)
		}
		return toString(value)
	default:
		return value, nil
	}
}

func isFloatConfig(item config.ConfigItem) bool {
	switch item.Default.(type) {
	case float32, float64:
		return true
	default:
		return false
	}
}

func toBool(value interface{}) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return false, fmt.Errorf("必须是布尔值")
		}
		return parsed, nil
	default:
		return false, fmt.Errorf("必须是布尔值")
	}
}

func toInt(value interface{}) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		if v != float64(int(v)) {
			return 0, fmt.Errorf("必须是整数")
		}
		return int(v), nil
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0, fmt.Errorf("必须是整数")
		}
		return int(i), nil
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0, fmt.Errorf("必须是整数")
		}
		return i, nil
	default:
		return 0, fmt.Errorf("必须是整数")
	}
}

func toFloat64(value interface{}) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return 0, fmt.Errorf("必须是数字")
		}
		return f, nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 0, fmt.Errorf("必须是数字")
		}
		return f, nil
	default:
		return 0, fmt.Errorf("必须是数字")
	}
}

func toString(value interface{}) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	default:
		return "", fmt.Errorf("必须是字符串")
	}
}

func toStringSlice(value interface{}) ([]string, error) {
	switch v := value.(type) {
	case []string:
		return v, nil
	case []interface{}:
		values := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("必须是字符串数组")
			}
			values = append(values, s)
		}
		return values, nil
	case string:
		if strings.TrimSpace(v) == "" {
			return []string{}, nil
		}
		parts := strings.Split(v, ",")
		values := make([]string, 0, len(parts))
		for _, part := range parts {
			values = append(values, strings.TrimSpace(part))
		}
		return values, nil
	default:
		return nil, fmt.Errorf("必须是字符串数组")
	}
}

func toStringSliceByLine(value interface{}) ([]string, error) {
	switch v := value.(type) {
	case []string:
		return v, nil
	case []interface{}:
		values := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("必须是字符串数组")
			}
			s = strings.TrimSpace(s)
			if s != "" {
				values = append(values, s)
			}
		}
		return values, nil
	case string:
		if strings.TrimSpace(v) == "" {
			return []string{}, nil
		}
		parts := strings.Split(v, "\n")
		values := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				values = append(values, part)
			}
		}
		return values, nil
	default:
		return nil, fmt.Errorf("必须是字符串数组")
	}
}

func ensureConfigFilePath(cfg *config.Config) error {
	if cfg.FullPath == "" {
		cfg.FullPath = filepath.Join(cfg.RootDir, cfg.Filename)
	}
	if cfg.FullPath == "" {
		return fmt.Errorf("配置文件路径为空")
	}
	return os.MkdirAll(filepath.Dir(cfg.FullPath), 0755)
}
