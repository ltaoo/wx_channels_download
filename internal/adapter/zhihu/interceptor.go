package zhihuadapter

import (
	"fmt"

	"wx_channel/frontend"
	"wx_channel/internal/adapter"
	"wx_channel/pkg/configapi"
	"wx_channel/pkg/scraper/zhihu"
)

var InterceptorConfigDeclaration = configapi.Declare("zhihu", "api")

// InterceptorPluginConfig owns the zhihu scraper configuration used
// by the local interceptor.
type InterceptorPluginConfig struct {
	settings       *ZhihuPluginConfig
	version        string
	asset_base_url string
}

// NewConfig creates an InterceptorPluginConfig from the application config.
func NewConfig(provider configapi.Provider, runtime configapi.Runtime) (*InterceptorPluginConfig, error) {
	var settings ZhihuPluginConfig
	if err := InterceptorConfigDeclaration.Decode(provider, "zhihu", &settings); err != nil {
		return nil, fmt.Errorf("zhihu config: %w", err)
	}
	var api_config struct {
		Protocol string `json:"protocol"`
		Hostname string `json:"hostname"`
		Port     int    `json:"port"`
	}
	if err := InterceptorConfigDeclaration.Decode(provider, "api", &api_config); err != nil {
		return nil, fmt.Errorf("api config: %w", err)
	}
	return &InterceptorPluginConfig{
		settings:       &settings,
		version:        runtime.Version,
		asset_base_url: frontend.AssetsBaseURLFromConfig(api_config.Protocol, api_config.Hostname, api_config.Port),
	}, nil
}

// GetPlugins returns zhihu injection and callback plugins wired to
// adapter-owned persistence and browse events.
func (c *InterceptorPluginConfig) GetPlugins(ctx adapter.AdapterContext) []interface{} {
	if c == nil || c.settings == nil || !c.settings.Enabled {
		return nil
	}

	// Create zhihu interceptor plugin for injecting zhihu.main.js
	plugin := zhihu.CreateZhihuInterceptorPlugin(c.settings.Cookie, c.asset_base_url, c.version)
	if plugin == nil {
		return nil
	}
	return []interface{}{plugin}
}
