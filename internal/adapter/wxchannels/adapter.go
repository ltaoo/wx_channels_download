package wxchannelsadapter

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/rs/zerolog"

	"wx_channel/internal/adapter"
	"wx_channel/internal/config"
	"wx_channel/internal/events"
	"wx_channel/pkg/hermes"
	"wx_channel/pkg/scraper/wxchannels"
)

// ChannelsAdapter owns all wxchannels platform and runtime capabilities.
type ChannelsAdapter struct {
	runtime_mu         sync.Mutex
	runtime_registered bool
	config             *config.Config
	routes             *WebsocketRoutes
	interceptor_config *InterceptorPluginConfig
	hooks              *hermes.HookManager
	logger             *zerolog.Logger
	status_bus         *events.Bus
}

var (
	_ adapter.PlatformAdapter          = (*ChannelsAdapter)(nil)
	_ adapter.RuntimeAdapter           = (*ChannelsAdapter)(nil)
	_ adapter.RuntimeHandle            = (*ChannelsAdapter)(nil)
	_ adapter.FetchDownloadTaskBuilder = (*ChannelsAdapter)(nil)
	_ adapter.Postprocessor            = (*ChannelsAdapter)(nil)
	_ adapter.PlatformStatusDescriber  = (*ChannelsAdapter)(nil)
	_ adapter.PlatformStatusRefresher  = (*ChannelsAdapter)(nil)
)

const (
	wxchannels_status_key_page = PlatformID + ":page"
	wxchannels_status_key_sph  = PlatformID + ":sph"
)

func init() {
	adapter.Register(NewChannelsAdapter())
}

func NewChannelsAdapter() *ChannelsAdapter {
	return &ChannelsAdapter{}
}

func (a *ChannelsAdapter) PlatformID() string { return PlatformID }

func (a *ChannelsAdapter) PlatformStatuses() []adapter.PlatformStatusDescriptor {
	return []adapter.PlatformStatusDescriptor{
		{Platform: PlatformID, Key: wxchannels_status_key_page, Name: "视频号页面"},
		{Platform: PlatformID, Key: wxchannels_status_key_sph, Name: "视频号分享链接"},
	}
}

func (a *ChannelsAdapter) Fetch(raw_url string) (any, error) {
	if a == nil {
		return nil, errors.New("wxchannels adapter is not initialized")
	}
	raw_url = strings.TrimSpace(raw_url)
	if raw_url == "" {
		return nil, errors.New("wxchannels url is empty")
	}

	a.runtime_mu.Lock()
	routes := a.routes
	a.runtime_mu.Unlock()
	if routes == nil || routes.client == nil {
		return nil, errors.New("wxchannels runtime is not initialized")
	}

	return routes.client.Fetch(wxchannels.FetchParams{URL: raw_url})
}

// Register creates and initializes a standalone channels adapter.
func Register(d *adapter.AdapterOptions) (*ChannelsAdapter, error) {
	channels_adapter := NewChannelsAdapter()
	if err := channels_adapter.register(d); err != nil {
		return nil, err
	}
	return channels_adapter, nil
}

// register wires up static assets, interceptor plugins, routes, and lifecycle
// state on this adapter instance.
func (a *ChannelsAdapter) register(d *adapter.AdapterOptions) error {
	if d == nil {
		return errors.New("wxchannels runtime dependencies are nil")
	}
	a.runtime_mu.Lock()
	if a.runtime_registered {
		a.runtime_mu.Unlock()
		return errors.New("wxchannels adapter runtime is already registered")
	}
	a.runtime_registered = true
	a.runtime_mu.Unlock()

	registered := false
	defer func() {
		if registered {
			return
		}
		a.runtime_mu.Lock()
		a.runtime_registered = false
		a.runtime_mu.Unlock()
	}()

	if d.StaticAssets != nil {
		if err := register_static_assets(d.StaticAssets); err != nil {
			return fmt.Errorf("wxchannels static assets: %w", err)
		}
	}

	var interceptor_config *InterceptorPluginConfig
	if d.Interceptor != nil {
		if d.Config == nil {
			return errors.New("wxchannels config is required for interceptor registration")
		}
		if d.Logger != nil {
			d.Logger.Info().
				Str("file", "internal/adapter/wxchannels/adapter.go").
				Bool("global_script_configured", d.Config.GlobalScriptPath != "").
				Str("global_script_path", d.Config.GlobalScriptPath).
				Msg("wxchannels adapter register: creating interceptor config")
		}
		interceptor_config = NewConfig(d.Config, d.Logger)
		for _, p := range interceptor_config.GetPlugins() {
			d.Interceptor.AddPostPlugin(p)
		}
	}

	refresh_interval := 0
	if d.Config != nil {
		refresh_interval = d.Config.GetInt("channels.refreshInterval")
	}
	r := NewWebsocketRoutes(refresh_interval, d.Cookies)
	bind_platform_status_events(r, d.Bus)
	if d.Routes != nil {
		r.RegisterRoutes(d.Routes)
	}

	a.runtime_mu.Lock()
	a.routes = r
	a.interceptor_config = interceptor_config
	a.hooks = d.Hooks
	a.logger = d.Logger
	a.config = d.Config
	a.status_bus = d.Bus
	a.runtime_mu.Unlock()
	registered = true
	return nil
}

func bind_platform_status_events(routes *WebsocketRoutes, bus *events.Bus) {
	if routes == nil || routes.client == nil || bus == nil {
		return
	}
	routes.client.OnConnected = func(_ *wxchannels.WebsocketClient) {
		publish_wxchannels_platform_statuses(routes, bus)
	}
	routes.client.OnDisconnected = func(_ *wxchannels.WebsocketClient) {
		publish_wxchannels_platform_statuses(routes, bus)
	}
	publish_wxchannels_platform_statuses(routes, bus)
}

func publish_wxchannels_platform_statuses(routes *WebsocketRoutes, bus *events.Bus) {
	publish_wxchannels_page_status(routes, bus)
	publish_wxchannels_sph_status(routes, bus)
}

func publish_wxchannels_page_status(routes *WebsocketRoutes, bus *events.Bus) {
	if routes == nil || routes.client == nil || bus == nil {
		return
	}
	available := routes.client.Available()
	reason := ""
	if !available {
		reason = "等待视频号页面连接"
	}
	bus.Publish(events.PlatformStatusChanged{
		Platform:  PlatformID,
		Key:       wxchannels_status_key_page,
		Name:      "视频号页面",
		Status:    platform_status_name(available),
		Available: available,
		Reason:    reason,
	})
}

func publish_wxchannels_sph_status(routes *WebsocketRoutes, bus *events.Bus) {
	if routes == nil || routes.client == nil || bus == nil {
		return
	}
	err := routes.client.CheckSphCookie()
	available := err == nil
	bus.Publish(events.PlatformStatusChanged{
		Platform:  PlatformID,
		Key:       wxchannels_status_key_sph,
		Name:      "视频号分享链接",
		Status:    platform_status_name(available),
		Available: available,
		Reason:    wxchannels_sph_status_reason(err),
	})
}

func wxchannels_sph_status_reason(err error) string {
	if err == nil {
		return ""
	}
	text := strings.TrimSpace(err.Error())
	if strings.Contains(text, "no yuanbao.tencent.com cookie") {
		return "缺少 yuanbao.tencent.com Cookie"
	}
	if text == "" {
		return "无法读取 yuanbao.tencent.com Cookie"
	}
	return "读取 yuanbao.tencent.com Cookie 失败：" + text
}

func platform_status_name(available bool) string {
	if available {
		return "available"
	}
	return "unavailable"
}

// RegisterRuntime exposes the complete adapter through the shared registry
// contract. The concrete package remains responsible for interpreting config.
func (a *ChannelsAdapter) RegisterRuntime(d *adapter.AdapterOptions) (adapter.RuntimeHandle, error) {
	if err := a.register(d); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *ChannelsAdapter) RefreshPlatformStatus() {
	if a == nil {
		return
	}
	a.runtime_mu.Lock()
	routes := a.routes
	bus := a.status_bus
	a.runtime_mu.Unlock()
	publish_wxchannels_platform_statuses(routes, bus)
}

// Stop shuts down the adapter's routes.
func (a *ChannelsAdapter) Stop() {
	if a == nil {
		return
	}
	a.runtime_mu.Lock()
	routes := a.routes
	a.runtime_registered = false
	a.routes = nil
	a.interceptor_config = nil
	a.hooks = nil
	a.logger = nil
	a.config = nil
	a.status_bus = nil
	a.runtime_mu.Unlock()
	if routes != nil {
		routes.Stop()
	}
}

func (a *ChannelsAdapter) config_bool(key string) bool {
	if a == nil {
		return false
	}
	a.runtime_mu.Lock()
	runtime_config := a.config
	a.runtime_mu.Unlock()
	if runtime_config == nil {
		return false
	}
	return runtime_config.GetBool(key)
}
