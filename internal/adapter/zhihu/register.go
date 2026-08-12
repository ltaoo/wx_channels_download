package zhihuadapter

import (
	"fmt"

	"wx_channel/internal/adapter"
)

// Handle owns the adapter's runtime components.
type Handle struct {
	routes          *Routes
	runtime_handler *handler
}

// Register wires up static assets, interceptor plugins, and HTTP routes.
func Register(d *adapter.AdapterOptions) (*Handle, error) {
	if d == nil {
		return nil, fmt.Errorf("zhihu runtime dependencies are nil")
	}
	// 1. Static assets
	if d.StaticAssets != nil {
		if err := register_static_assets(d.StaticAssets); err != nil {
			return nil, fmt.Errorf("zhihu static assets: %w", err)
		}
	}

	// 2. Interceptor plugins
	icfg := NewConfig(d.Config)
	if d.Interceptor != nil {
		for _, p := range icfg.GetPlugins() {
			d.Interceptor.AddPostPlugin(p)
		}
	}

	// 3. Routes
	r := NewRoutes(d.Cookies, d.Logger)
	if d.Routes != nil {
		r.RegisterRoutes(d.Routes)
		if d.Logger != nil {
			d.Logger.Info().Msg("zhihu routes registered")
		}
	}

	return &Handle{routes: r}, nil
}

// RegisterRuntime exposes the adapter through the shared registry contract.
func (h *handler) RegisterRuntime(d *adapter.AdapterOptions) (adapter.RuntimeHandle, error) {
	if d == nil {
		return nil, fmt.Errorf("zhihu runtime dependencies are nil")
	}
	h.set_runtime(d.Cookies, d.Logger)
	if d.Logger != nil {
		d.Logger.Info().Msg("zhihu adapter registering runtime")
	}
	handle, err := Register(d)
	if err != nil {
		h.set_runtime(nil, nil)
		return nil, err
	}
	handle.runtime_handler = h
	return handle, nil
}

// Stop shuts down the adapter's routes.
func (h *Handle) Stop() {
	if h == nil {
		return
	}
	if h.routes != nil {
		h.routes.Stop()
	}
	if h.runtime_handler != nil {
		h.runtime_handler.set_runtime(nil, nil)
	}
	h.routes = nil
	h.runtime_handler = nil
}
