package wxchannels

import (
	"fmt"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/adapter"
	"wx_channel/internal/config"
	"wx_channel/internal/events"
	"wx_channel/internal/webassets"
	scraper "wx_channel/pkg/scraper/wxchannels"
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

// AdapterWXChannels is the wxchannels platform adapter.
// It implements adapter.RuntimeHandle as the runtime component
// returned by RegisterRuntime.
type AdapterWXChannels struct {
	routes         *WebsocketRoutes
	interceptorCfg *InterceptorPluginConfig
}

// Register wires up all wxchannels adapter components: static assets,
// interceptor plugins, HTTP routes, and lifecycle hooks.
func Register(d Deps) (*AdapterWXChannels, error) {
	// 1. Static assets
	if d.StaticAssets != nil {
		if err := scraper.RegisterStaticAssets(d.StaticAssets); err != nil {
			return nil, fmt.Errorf("wxchannels static assets: %w", err)
		}
	}

	// 2. Interceptor plugins
	icfg := NewConfig(d.Config)
	if d.Interceptor != nil {
		for _, p := range icfg.GetPlugins(adapter.AdapterContext{DB: d.DB, Logger: d.Logger, Bus: d.Bus, BasePath: d.Config.GetDownloadDir()}) {
			d.Interceptor.AddPostPlugin(p)
		}
	}

	// 3. Routes
	r := NewWebsocketRoutes(d.RefreshInterval, d.Config)
	if d.RouteRegistrar != nil {
		r.RegisterRoutes(d.RouteRegistrar)
	}

	return &AdapterWXChannels{routes: r, interceptorCfg: icfg}, nil
}

// RegisterRuntime exposes the complete adapter through the shared registry
// contract. The concrete package remains responsible for interpreting config.
func (h *handler) RegisterRuntime(d adapter.RuntimeDeps) (adapter.RuntimeHandle, error) {
	refreshInterval := 0
	if d.Config != nil {
		refreshInterval = d.Config.GetInt("channels.refreshInterval")
	}
	return Register(Deps{
		StaticAssets:    d.StaticAssets,
		RouteRegistrar:  d.Routes,
		Interceptor:     d.Interceptor,
		DB:              d.DB,
		Logger:          d.Logger,
		Bus:             d.Bus,
		Config:          d.Config,
		RefreshInterval: refreshInterval,
	})
}

// Stop shuts down the adapter's routes.
func (h *AdapterWXChannels) Stop() {
	if h != nil && h.routes != nil {
		h.routes.Stop()
	}
}
