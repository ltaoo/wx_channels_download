package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	"wx_channel/internal/bridge"
	"wx_channel/internal/config"
)

var bridge_device_id_pattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)

type bridge_device_settings struct {
	Enabled            bool
	URL                string
	DeviceID           string
	DeviceName         string
	Token              string
	HTTPTimeoutSeconds int
	Methods            []string
}

type legacy_bridge_capability_settings struct {
	WXChannels bool `json:"wxchannels"`
	Download   bool `json:"download"`
}

type legacy_bridge_instance_settings struct {
	Name               string                             `json:"name"`
	Enabled            bool                               `json:"enabled"`
	URL                string                             `json:"url"`
	ID                 string                             `json:"id"`
	ClientID           string                             `json:"clientId"`
	Token              string                             `json:"token"`
	HTTPTimeoutSeconds int                                `json:"httpTimeoutSeconds"`
	Capabilities       *legacy_bridge_capability_settings `json:"capabilities"`
}

func load_bridge_config(application_config *config.Config) (*bridge.Config, error) {
	if application_config == nil {
		return nil, nil
	}
	configured_methods, err := parse_bridge_methods(application_config.GetString("bridge.methods"))
	if err != nil {
		return nil, err
	}
	settings := bridge_device_settings{
		Enabled:            application_config.GetBool("bridge.enabled"),
		URL:                application_config.GetString("bridge.url"),
		DeviceID:           application_config.GetString("bridge.deviceId"),
		DeviceName:         application_config.GetString("bridge.deviceName"),
		Token:              application_config.GetString("bridge.token"),
		HTTPTimeoutSeconds: application_config.GetInt("bridge.httpTimeoutSeconds"),
		Methods:            configured_methods,
	}
	if bridge_device_configured(settings) {
		return resolve_bridge_config(settings, "")
	}

	legacy_instances, err := decode_legacy_bridge_instances(application_config.GetRaw("bridge.instances"))
	if err != nil {
		return nil, err
	}
	if len(legacy_instances) == 0 {
		return nil, nil
	}
	if len(legacy_instances) > 1 {
		return nil, errors.New("当前版本一个设备只能连接一个 Bridge；请将 bridge.instances 迁移为单 Bridge 配置")
	}
	legacy := legacy_instances[0]
	var legacy_methods []string
	if legacy.Capabilities != nil {
		legacy_methods = []string{}
		if legacy.Capabilities.WXChannels {
			legacy_methods = append(legacy_methods, bridge.MethodWXChannelsFetch)
		}
		if legacy.Capabilities.Download {
			legacy_methods = append(legacy_methods, bridge.MethodDownloadCreate)
		}
	}
	return resolve_bridge_config(
		bridge_device_settings{
			Enabled:            legacy.Enabled,
			URL:                legacy.URL,
			DeviceID:           legacy.ClientID,
			DeviceName:         legacy.ClientID,
			Token:              legacy.Token,
			HTTPTimeoutSeconds: legacy.HTTPTimeoutSeconds,
			Methods:            legacy_methods,
		},
		strings.TrimSpace(legacy.ID),
	)
}

func bridge_device_configured(settings bridge_device_settings) bool {
	return settings.Enabled ||
		strings.TrimSpace(settings.URL) != "" ||
		strings.TrimSpace(settings.DeviceID) != "" ||
		strings.TrimSpace(settings.DeviceName) != "" ||
		strings.TrimSpace(settings.Token) != "" ||
		settings.Methods != nil
}

func resolve_bridge_config(settings bridge_device_settings, legacy_bridge_id string) (*bridge.Config, error) {
	hostname, _ := os.Hostname()
	device_name := strings.TrimSpace(settings.DeviceName)
	if device_name == "" {
		device_name = strings.TrimSpace(hostname)
	}
	if device_name == "" {
		device_name = "Unnamed device"
	}
	if len(device_name) > 128 {
		return nil, errors.New("bridge.deviceName 不能超过 128 个字符")
	}
	if strings.ContainsAny(device_name, "\r\n") {
		return nil, errors.New("bridge.deviceName 不能包含换行符")
	}
	device_id := strings.TrimSpace(settings.DeviceID)
	if device_id == "" {
		device_id = normalize_device_id(hostname)
	}

	http_timeout_seconds := settings.HTTPTimeoutSeconds
	if http_timeout_seconds <= 0 {
		http_timeout_seconds = 30
	}
	var configured_methods []string
	if settings.Methods != nil {
		configured_methods = append([]string{}, settings.Methods...)
	}
	bridge_config := &bridge.Config{
		Enabled:        settings.Enabled,
		URL:            strings.TrimSpace(settings.URL),
		DeviceID:       device_id,
		DeviceName:     device_name,
		DeviceOS:       runtime.GOOS,
		Token:          strings.TrimSpace(settings.Token),
		Methods:        configured_methods,
		HTTPTimeout:    time.Duration(http_timeout_seconds) * time.Second,
		LegacyBridgeID: legacy_bridge_id,
	}
	if !settings.Enabled {
		return bridge_config, nil
	}
	if bridge_config.URL == "" {
		return nil, errors.New("bridge.url 不能为空")
	}
	parsed_url, err := url.Parse(bridge_config.URL)
	if err != nil || parsed_url.Host == "" || (parsed_url.Scheme != "http" && parsed_url.Scheme != "https") {
		return nil, errors.New("bridge.url 必须是 HTTP 或 HTTPS URL")
	}
	if !bridge_device_id_pattern.MatchString(bridge_config.DeviceID) {
		return nil, errors.New("bridge.deviceId 不合法；仅支持字母、数字、点、下划线、冒号和连字符")
	}
	if bridge_config.Token == "" {
		return nil, errors.New("bridge.token 不能为空")
	}
	return bridge_config, nil
}

func parse_bridge_methods(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "auto") {
		return nil, nil
	}
	if strings.EqualFold(value, "none") {
		return []string{}, nil
	}
	methods := make([]string, 0)
	seen_methods := make(map[string]struct{})
	for _, item := range strings.Split(value, ",") {
		method := strings.TrimSpace(item)
		if !bridge_device_id_pattern.MatchString(method) {
			return nil, fmt.Errorf("bridge.methods 包含不合法的方法名 %q", method)
		}
		if _, exists := seen_methods[method]; exists {
			continue
		}
		seen_methods[method] = struct{}{}
		methods = append(methods, method)
	}
	return methods, nil
}

func normalize_device_id(value string) string {
	value = strings.TrimSpace(value)
	var result strings.Builder
	result.Grow(len(value))
	for _, character := range value {
		valid := character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '.' || character == '_' || character == ':' || character == '-'
		if valid {
			result.WriteRune(character)
		} else if result.Len() > 0 {
			result.WriteByte('-')
		}
		if result.Len() >= 128 {
			break
		}
	}
	device_id := strings.Trim(result.String(), "-.:_")
	if device_id == "" {
		return "device"
	}
	return device_id
}

func decode_legacy_bridge_instances(raw_instances any) ([]legacy_bridge_instance_settings, error) {
	if raw_instances == nil {
		return nil, nil
	}
	data, err := json.Marshal(raw_instances)
	if err != nil {
		return nil, fmt.Errorf("编码 bridge.instances 失败: %w", err)
	}
	if string(data) == "null" || string(data) == "[]" {
		return nil, nil
	}
	var settings []legacy_bridge_instance_settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("解析 bridge.instances 失败: %w", err)
	}
	if settings == nil {
		return nil, errors.New("bridge.instances 必须是数组")
	}
	return settings, nil
}
