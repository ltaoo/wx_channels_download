package wxchannelsadapter

import (
	"errors"
	"fmt"
	"sync"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/adapter"
	"wx_channel/internal/config"
	"wx_channel/internal/events"
	"wx_channel/internal/webassets"
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
	RefreshInterval int
}

// ChannelsAdapter owns all wxchannels platform and runtime capabilities.
type ChannelsAdapter struct {
	runtimeMu         sync.Mutex
	runtimeRegistered bool
	routes            *WebsocketRoutes
	interceptorConfig *InterceptorPluginConfig
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
	return &ChannelsAdapter{}
}

func (a *ChannelsAdapter) PlatformID() string { return PlatformID }

// Register creates and initializes a standalone channels adapter.
func Register(d Deps) (*ChannelsAdapter, error) {
	channelsAdapter := NewChannelsAdapter()
	if err := channelsAdapter.register(d); err != nil {
		return nil, err
	}
	return channelsAdapter, nil
}

// register wires up static assets, interceptor plugins, routes, and lifecycle
// state on this adapter instance.
func (a *ChannelsAdapter) register(d Deps) error {
	a.runtimeMu.Lock()
	if a.runtimeRegistered {
		a.runtimeMu.Unlock()
		return errors.New("wxchannels adapter runtime is already registered")
	}
	a.runtimeRegistered = true
	a.runtimeMu.Unlock()

	registered := false
	defer func() {
		if registered {
			return
		}
		a.runtimeMu.Lock()
		a.runtimeRegistered = false
		a.runtimeMu.Unlock()
	}()

	if d.StaticAssets != nil {
		if err := wxchannels.RegisterStaticAssets(d.StaticAssets); err != nil {
			return fmt.Errorf("wxchannels static assets: %w", err)
		}
	}

	var interceptorConfig *InterceptorPluginConfig
	if d.Interceptor != nil {
		if d.Config == nil {
			return errors.New("wxchannels config is required for interceptor registration")
		}
		interceptorConfig = NewConfig(d.Config)
		for _, p := range interceptorConfig.GetPlugins(adapter.AdapterContext{
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

	a.runtimeMu.Lock()
	a.routes = r
	a.interceptorConfig = interceptorConfig
	a.runtimeMu.Unlock()
	registered = true
	return nil
}

// RegisterRuntime exposes the complete adapter through the shared registry
// contract. The concrete package remains responsible for interpreting config.
func (a *ChannelsAdapter) RegisterRuntime(d adapter.RuntimeDeps) (adapter.RuntimeHandle, error) {
	refreshInterval := 0
	if d.Config != nil {
		refreshInterval = d.Config.GetInt("channels.refreshInterval")
	}
	if err := a.register(Deps{
		StaticAssets:    d.StaticAssets,
		RouteRegistrar:  d.Routes,
		Interceptor:     d.Interceptor,
		DB:              d.DB,
		Logger:          d.Logger,
		Bus:             d.Bus,
		Config:          d.Config,
		RefreshInterval: refreshInterval,
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
	a.runtimeMu.Lock()
	routes := a.routes
	a.runtimeRegistered = false
	a.routes = nil
	a.interceptorConfig = nil
	a.runtimeMu.Unlock()
	if routes != nil {
		routes.Stop()
	}
}
