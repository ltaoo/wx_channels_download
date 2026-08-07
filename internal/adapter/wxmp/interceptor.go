package wxmpadapter

import (
	"wx_channel/internal/adapter"
	"wx_channel/pkg/configapi"
	"wx_channel/pkg/scraper/wxmp"
)

// InterceptorPluginConfig owns the official-account scraper configuration used
// by the local interceptor.
type InterceptorPluginConfig struct {
	settings *wxmp.OfficialAccountConfig
	version  string
}

func NewConfig(provider configapi.Provider, runtime configapi.Runtime) (*InterceptorPluginConfig, error) {
	settings, err := wxmp.NewOfficialAccountConfig(provider, runtime)
	if err != nil {
		return nil, err
	}
	return &InterceptorPluginConfig{settings: settings, version: runtime.Version}, nil
}

// GetPlugins returns the official-account injection plugin.
func (c *InterceptorPluginConfig) GetPlugins(ctx adapter.AdapterContext) []interface{} {
	if c == nil || c.settings == nil {
		return nil
	}

	return []interface{}{
		wxmp.CreateOfficialAccountInterceptorPlugin(c.settings, c.version),
	}
}
