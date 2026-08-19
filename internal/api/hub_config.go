package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"wx_channel/internal/config"
	"wx_channel/internal/hub"
	"wx_channel/internal/services"
)

var hub_instance_name_pattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)

type hub_instance_capabilities struct {
	WXChannels bool `json:"wxchannels"`
	Download   bool `json:"download"`
}

type hub_instance_settings struct {
	Name               string                    `json:"name"`
	Enabled            bool                      `json:"enabled"`
	URL                string                    `json:"url"`
	ID                 string                    `json:"id"`
	ClientID           string                    `json:"clientId"`
	Token              string                    `json:"token"`
	HTTPTimeoutSeconds int                       `json:"httpTimeoutSeconds"`
	Capabilities       hub_instance_capabilities `json:"capabilities"`
}

func (c *APIClient) configure_hub_service() {
	if c == nil {
		return
	}
	var application_config *config.Config
	if c.cfg != nil {
		application_config = c.cfg.Original
	}
	named_configs, default_name, err := load_hub_configs(application_config)
	if err != nil {
		c.hub_config_error = err
		return
	}
	c.hub_service = services.NewHubService(services.HubServiceOptions{
		Configs:             named_configs,
		DefaultName:         default_name,
		DownloadTaskService: c.download_task_service,
		Logger:              c.logger,
	})
}

func load_hub_configs(application_config *config.Config) ([]services.NamedHubConfig, string, error) {
	if application_config == nil {
		return nil, "", nil
	}
	settings, err := decode_hub_instance_settings(application_config.GetRaw("hub.instances"))
	if err != nil {
		return nil, "", err
	}
	return resolve_hub_configs(settings, application_config.GetString("hub.defaultInstance"))
}

func resolve_hub_configs(settings []hub_instance_settings, requested_default_name string) ([]services.NamedHubConfig, string, error) {
	if len(settings) == 0 {
		return nil, "", nil
	}

	named_configs := make([]services.NamedHubConfig, 0, len(settings))
	seen_names := make(map[string]struct{}, len(settings))
	seen_registrations := make(map[string]string, len(settings))
	first_enabled_name := ""
	for index, instance := range settings {
		instance_name := strings.TrimSpace(instance.Name)
		if !hub_instance_name_pattern.MatchString(instance_name) {
			return nil, "", fmt.Errorf("hub.instances[%d].name 不合法", index)
		}
		if _, exists := seen_names[instance_name]; exists {
			return nil, "", fmt.Errorf("hub.instances 包含重复名称 %q", instance_name)
		}
		seen_names[instance_name] = struct{}{}

		capabilities := make([]string, 0, 2)
		if instance.Capabilities.WXChannels {
			capabilities = append(capabilities, hub.CapabilityWXChannelsFetch)
		}
		if instance.Capabilities.Download {
			capabilities = append(capabilities, hub.CapabilityDownloadCreate)
		}
		http_timeout_seconds := instance.HTTPTimeoutSeconds
		if http_timeout_seconds <= 0 {
			http_timeout_seconds = 30
		}
		hub_config := hub.Config{
			Enabled:      instance.Enabled,
			URL:          strings.TrimSpace(instance.URL),
			HubID:        strings.TrimSpace(instance.ID),
			ClientID:     strings.TrimSpace(instance.ClientID),
			Token:        strings.TrimSpace(instance.Token),
			Capabilities: capabilities,
			HTTPTimeout:  time.Duration(http_timeout_seconds) * time.Second,
		}
		if instance.Enabled {
			if hub_config.URL == "" {
				return nil, "", fmt.Errorf("hub.instances[%d] %q 缺少 url", index, instance_name)
			}
			if hub_config.HubID == "" {
				return nil, "", fmt.Errorf("hub.instances[%d] %q 缺少 id", index, instance_name)
			}
			if hub_config.ClientID == "" {
				return nil, "", fmt.Errorf("hub.instances[%d] %q 缺少 clientId", index, instance_name)
			}
			if hub_config.Token == "" {
				return nil, "", fmt.Errorf("hub.instances[%d] %q 缺少 token", index, instance_name)
			}
			if first_enabled_name == "" {
				first_enabled_name = instance_name
			}
			registration_key := strings.TrimRight(hub_config.URL, "/") + "\x00" + hub_config.HubID + "\x00" + hub_config.ClientID
			if existing_name, exists := seen_registrations[registration_key]; exists {
				return nil, "", fmt.Errorf(
					"Hub %q 与 %q 注册了相同的 url、id 和 clientId",
					instance_name,
					existing_name,
				)
			}
			seen_registrations[registration_key] = instance_name
		}
		named_configs = append(named_configs, services.NamedHubConfig{Name: instance_name, Config: hub_config})
	}

	default_name := strings.TrimSpace(requested_default_name)
	if default_name == "" {
		default_name = first_enabled_name
		if default_name == "" {
			default_name = named_configs[0].Name
		}
	}
	if _, exists := seen_names[default_name]; !exists {
		return nil, "", fmt.Errorf("hub.defaultInstance %q 不存在于 hub.instances", default_name)
	}
	return named_configs, default_name, nil
}

func decode_hub_instance_settings(raw_instances any) ([]hub_instance_settings, error) {
	if raw_instances == nil {
		return nil, nil
	}
	data, err := json.Marshal(raw_instances)
	if err != nil {
		return nil, fmt.Errorf("编码 hub.instances 失败: %w", err)
	}
	if string(data) == "null" || string(data) == "[]" {
		return nil, nil
	}
	var settings []hub_instance_settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("解析 hub.instances 失败: %w", err)
	}
	if settings == nil {
		return nil, errors.New("hub.instances 必须是数组")
	}
	return settings, nil
}
