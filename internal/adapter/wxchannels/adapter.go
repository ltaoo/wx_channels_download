package wxchannelsadapter

import (
	"errors"
	"fmt"
	"sync"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/adapter"
	"wx_channel/internal/webassets"
	"wx_channel/pkg/configapi"
	"wx_channel/pkg/events"
	"wx_channel/pkg/scraper/wxchannels"
)

// Deps holds the dependencies needed to register the wxchannels adapter.
type Deps struct {
	StaticAssets   *webassets.Registry
	RouteRegistrar RouteRegistrar
	Interceptor    adapter.InterceptorRegistrar
	DB             *gorm.DB
	Logger         *zerolog.Logger
	Bus            *events.Bus
	ConfigProvider configapi.Provider
	Runtime        configapi.Runtime
	BasePath       string
}

// ChannelsAdapter owns all wxchannels platform and runtime capabilities.
type ChannelsAdapter struct {
	runtime_mu         sync.Mutex
	runtime_registered bool
	routes             *WebsocketRoutes
	interceptor_config *InterceptorPluginConfig
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

	// 1. Static assets
	if d.StaticAssets != nil {
		if err := wxchannels.RegisterStaticAssets(d.StaticAssets); err != nil {
			return fmt.Errorf("wxchannels static assets: %w", err)
		}
	}

	// 2. Interceptor plugins
	config_provider := d.ConfigProvider
	var icfg *InterceptorPluginConfig
	if d.Interceptor != nil {
		if config_provider == nil {
			return errors.New("wxchannels config is required for interceptor registration")
		}
		var err error
		icfg, err = NewConfig(config_provider, d.Runtime)
		if err != nil {
			return fmt.Errorf("wxchannels interceptor config: %w", err)
		}
		for _, p := range icfg.GetPlugins(adapter.AdapterContext{DB: d.DB, Logger: d.Logger, Bus: d.Bus, BasePath: d.BasePath}) {
			d.Interceptor.AddPostPlugin(p)
		}
	}

	// 3. Routes
	r := NewWebsocketRoutes(config_provider)
	if d.RouteRegistrar != nil {
		r.RegisterRoutes(d.RouteRegistrar)
	}

	a.runtime_mu.Lock()
	a.routes = r
	a.interceptor_config = icfg
	a.runtime_mu.Unlock()
	registered = true
	return nil
}

// RegisterRuntime exposes the complete adapter through the shared registry
// contract. The concrete package remains responsible for interpreting config.
func (a *ChannelsAdapter) RegisterRuntime(d adapter.RuntimeDeps) (adapter.RuntimeHandle, error) {
	config_provider := d.ConfigProvider
	err := a.register(Deps{
		StaticAssets:   d.StaticAssets,
		RouteRegistrar: d.Routes,
		Interceptor:    d.Interceptor,
		DB:             d.DB,
		Logger:         d.Logger,
		Bus:            d.Bus,
		ConfigProvider: config_provider,
		Runtime:        d.Runtime,
		BasePath:       d.BasePath,
	})
	if err != nil {
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
	a.runtime_mu.Unlock()
	if routes != nil {
		routes.Stop()
	}
}
