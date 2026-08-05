package wxmp

import (
	"fmt"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/adapter"
	"wx_channel/internal/config"
	"wx_channel/internal/events"
	"wx_channel/internal/webassets"
	scraper "wx_channel/pkg/scraper/wxmp"
)

// Deps holds the dependencies needed to register the wxmp adapter.
type Deps struct {
	StaticAssets     *webassets.Registry
	RouteRegistrar   RouteRegistrar
	Interceptor      adapter.InterceptorRegistrar
	DB               *gorm.DB
	Logger           *zerolog.Logger
	Bus              *events.Bus
	Config           *config.Config
}

// Handle provides access to the adapter's runtime components.
type Handle struct {
	routes *Routes
}

// Register wires up all wxmp adapter components: static assets,
// interceptor plugins, and HTTP routes.
func Register(d Deps) (*Handle, error) {
	// 1. Static assets
	if d.StaticAssets != nil {
		if err := scraper.RegisterStaticAssets(d.StaticAssets); err != nil {
			return nil, fmt.Errorf("wxmp static assets: %w", err)
		}
	}

	// 2. Interceptor plugins
	icfg := NewConfig(d.Config)
	if d.Interceptor != nil {
		for _, p := range icfg.GetPlugins(adapter.AdapterContext{DB: d.DB, Logger: d.Logger}) {
			d.Interceptor.AddPostPlugin(p)
		}
	}

	// 3. Routes
	r := NewRoutes(d.Config, d.Logger, d.DB)
	if d.RouteRegistrar != nil {
		r.RegisterRoutes(d.RouteRegistrar)
	}

	return &Handle{routes: r}, nil
}

// RegisterRuntime exposes the adapter through the shared registry contract.
func (h *handler) RegisterRuntime(d adapter.RuntimeDeps) (adapter.RuntimeHandle, error) {
	return Register(Deps{
		StaticAssets:     d.StaticAssets,
		RouteRegistrar:   d.Routes,
		Interceptor:      d.Interceptor,
		DB:               d.DB,
		Logger:           d.Logger,
		Bus:              d.Bus,
		Config:           d.Config,
	})
}

// Stop shuts down the adapter's routes.
func (h *Handle) Stop() {
	if h != nil && h.routes != nil {
		h.routes.Stop()
	}
}
