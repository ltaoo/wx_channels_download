package zhihuadapter

import (
	"fmt"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/adapter"
	"wx_channel/internal/webassets"
	"wx_channel/pkg/configapi"
	"wx_channel/pkg/events"
	"wx_channel/pkg/scraper/zhihu"
)

// Deps holds the dependencies needed to register the zhihu adapter.
type Deps struct {
	StaticAssets   *webassets.Registry
	RouteRegistrar RouteRegistrar
	Interceptor    adapter.InterceptorRegistrar
	DB             *gorm.DB
	Logger         *zerolog.Logger
	Bus            *events.Bus
	ConfigProvider configapi.Provider
	Runtime        configapi.Runtime
}

// Handle owns the adapter's runtime components.
type Handle struct {
	routes *Routes
}

// Register wires up static assets, interceptor plugins, and HTTP routes.
func Register(d Deps) (*Handle, error) {
	// 1. Static assets
	if d.StaticAssets != nil {
		if err := zhihu.RegisterStaticAssets(d.StaticAssets); err != nil {
			return nil, fmt.Errorf("zhihu static assets: %w", err)
		}
	}

	// 2. Interceptor plugins
	config_provider := d.ConfigProvider
	if d.Interceptor != nil {
		if config_provider == nil {
			return nil, fmt.Errorf("zhihu config is required for interceptor registration")
		}
		icfg, err := NewConfig(config_provider, d.Runtime)
		if err != nil {
			return nil, fmt.Errorf("zhihu interceptor config: %w", err)
		}
		for _, p := range icfg.GetPlugins(adapter.AdapterContext{
			DB:     d.DB,
			Logger: d.Logger,
			Bus:    d.Bus,
		}) {
			d.Interceptor.AddPostPlugin(p)
		}
	}

	// 3. Routes
	workdir := d.Runtime.WorkDir
	r := NewRoutes(workdir, config_provider)
	if d.RouteRegistrar != nil {
		r.RegisterRoutes(d.RouteRegistrar)
		if d.Logger != nil {
			d.Logger.Info().Str("workdir", workdir).Msg("zhihu routes registered")
		}
	}

	return &Handle{routes: r}, nil
}

// RegisterRuntime exposes the adapter through the shared registry contract.
func (h *handler) RegisterRuntime(d adapter.RuntimeDeps) (adapter.RuntimeHandle, error) {
	if d.Logger != nil {
		d.Logger.Info().Msg("zhihu adapter registering runtime")
	}
	config_provider := d.ConfigProvider
	h.set_config_provider(config_provider)
	return Register(Deps{
		StaticAssets:   d.StaticAssets,
		RouteRegistrar: d.Routes,
		Interceptor:    d.Interceptor,
		DB:             d.DB,
		Logger:         d.Logger,
		Bus:            d.Bus,
		ConfigProvider: config_provider,
		Runtime:        d.Runtime,
	})
}

// Stop shuts down the adapter's routes.
func (h *Handle) Stop() {
	if h != nil && h.routes != nil {
		h.routes.Stop()
	}
}
