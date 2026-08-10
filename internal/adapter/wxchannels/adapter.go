package wxchannelsadapter

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/adapter"
	"wx_channel/internal/config"
	"wx_channel/internal/events"
	"wx_channel/internal/webassets"
	"wx_channel/pkg/hermes"
	"wx_channel/pkg/scraper/wxchannels"
)

// Deps holds the dependencies needed to register the wxchannels adapter.
type Deps struct {
	StaticAssets    *webassets.Registry
	RouteRegistrar  RouteRegistrar
	Interceptor     adapter.InterceptorRegistrar
	DB              *gorm.DB
	Logger          *zerolog.Logger
	Bus             *events.Bus
	Config          *config.Config
	Hooks           *hermes.HookManager
	RefreshInterval int
}

// ChannelsAdapter owns all wxchannels platform and runtime capabilities.
type ChannelsAdapter struct {
	runtime_mu         sync.Mutex
	runtime_registered bool
	cfg                *ChannelsPluginConfig
	routes             *WebsocketRoutes
	interceptor_config *InterceptorPluginConfig
	hooks              *hermes.HookManager
	logger             *zerolog.Logger
}

var (
	_ adapter.PlatformAdapter = (*ChannelsAdapter)(nil)
	_ adapter.RuntimeAdapter  = (*ChannelsAdapter)(nil)
	_ adapter.RuntimeHandle   = (*ChannelsAdapter)(nil)
	_ adapter.Postprocessor   = (*ChannelsAdapter)(nil)
)

func init() {
	adapter.Register(NewChannelsAdapter())
}

func NewChannelsAdapter() *ChannelsAdapter {
	return &ChannelsAdapter{cfg: channels_plugin_config}
}

func (a *ChannelsAdapter) PlatformID() string { return PlatformID }

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
func Register(d Deps) (*ChannelsAdapter, error) {
	channels_adapter := NewChannelsAdapter()
	if err := channels_adapter.register(d); err != nil {
		return nil, err
	}
	return channels_adapter, nil
}

// register wires up static assets, interceptor plugins, routes, and lifecycle
// state on this adapter instance.
func (a *ChannelsAdapter) register(d Deps) error {
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
		if err := wxchannels.RegisterStaticAssets(d.StaticAssets); err != nil {
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
		for _, p := range interceptor_config.GetPlugins(adapter.AdapterContext{
			DB:       d.DB,
			Logger:   d.Logger,
			Bus:      d.Bus,
			BasePath: d.Config.GetDownloadDir(),
		}) {
			d.Interceptor.AddPostPlugin(p)
		}
	}

	r := NewWebsocketRoutes(d.RefreshInterval, d.Config)
	if d.RouteRegistrar != nil {
		r.RegisterRoutes(d.RouteRegistrar)
	}

	a.runtime_mu.Lock()
	a.routes = r
	a.interceptor_config = interceptor_config
	a.hooks = d.Hooks
	a.logger = d.Logger
	a.runtime_mu.Unlock()
	registered = true
	return nil
}

// RegisterRuntime exposes the complete adapter through the shared registry
// contract. The concrete package remains responsible for interpreting config.
func (a *ChannelsAdapter) RegisterRuntime(d adapter.RuntimeDeps) (adapter.RuntimeHandle, error) {
	refresh_interval := 0
	if d.Config != nil {
		refresh_interval = d.Config.GetInt("channels.refreshInterval")
	}
	if err := a.register(Deps{
		StaticAssets:    d.StaticAssets,
		RouteRegistrar:  d.Routes,
		Interceptor:     d.Interceptor,
		DB:              d.DB,
		Logger:          d.Logger,
		Bus:             d.Bus,
		Config:          d.Config,
		Hooks:           d.Hooks,
		RefreshInterval: refresh_interval,
	}); err != nil {
		return nil, err
	}
	return a, nil
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
	a.runtime_mu.Unlock()
	if routes != nil {
		routes.Stop()
	}
}
