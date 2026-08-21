package zhihuadapter

import (
	"fmt"

	"wx_channel/internal/adapter"
	"wx_channel/internal/config"
)

var _ adapter.RuntimeEnablement = (*handler)(nil)

// RuntimeEnabled reports whether the Zhihu runtime is enabled in config.
func (handler_instance *handler) RuntimeEnabled(application_config *config.Config) bool {
	return handler_instance != nil && application_config != nil && application_config.GetBool("zhihu.enabled")
}

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
	r.set_persistent_cache(d.Cache)
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
	h.set_runtime(d.Cookies, d.Logger, d.Bus)
	h.set_persistent_cache(d.Cache)
	if d.Logger != nil {
		d.Logger.Info().Msg("zhihu adapter registering runtime")
	}
	handle, err := Register(d)
	if err != nil {
		h.set_runtime(nil, nil, nil)
		h.set_persistent_cache(nil)
		return nil, err
	}
	handle.runtime_handler = h
	h.RefreshPlatformStatus()
	return handle, nil
}

// Stop shuts down the adapter's routes.
func (h *Handle) Stop() {
	if h == nil {
		return
	}
	if h.runtime_handler != nil {
		h.runtime_handler.stop_status_check()
		h.runtime_handler.set_runtime(nil, nil, nil)
		h.runtime_handler.set_persistent_cache(nil)
	}
	h.routes = nil
	h.runtime_handler = nil
}
