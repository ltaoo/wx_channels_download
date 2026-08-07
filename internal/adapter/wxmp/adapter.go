package wxmpadapter

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
	"wx_channel/pkg/scraper/wxmp"
)

// Deps holds the dependencies needed to register the wxmp adapter.
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

// OfficialAccountAdapter owns all wxmp platform and runtime capabilities.
type OfficialAccountAdapter struct {
	runtime_mu         sync.Mutex
	runtime_registered bool
	routes             *Routes
	interceptor_config *InterceptorPluginConfig
}

var (
	_ adapter.PlatformAdapter = (*OfficialAccountAdapter)(nil)
	_ adapter.RuntimeAdapter  = (*OfficialAccountAdapter)(nil)
	_ adapter.RuntimeHandle   = (*OfficialAccountAdapter)(nil)
	_ adapter.Postprocessor   = (*OfficialAccountAdapter)(nil)
)

func init() {
	adapter.Register(NewOfficialAccountAdapter())
}

func NewOfficialAccountAdapter() *OfficialAccountAdapter {
	return &OfficialAccountAdapter{}
}

// Register creates and initializes a standalone official-account adapter.
func Register(d Deps) (*OfficialAccountAdapter, error) {
	official_account_adapter := NewOfficialAccountAdapter()
	if err := official_account_adapter.register(d); err != nil {
		return nil, err
	}
	return official_account_adapter, nil
}

// register wires up static assets, interceptor plugins, routes, and lifecycle
// state on this adapter instance.
func (a *OfficialAccountAdapter) register(d Deps) error {
	a.runtime_mu.Lock()
	if a.runtime_registered {
		a.runtime_mu.Unlock()
		return errors.New("wxmp adapter runtime is already registered")
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
		if err := wxmp.RegisterStaticAssets(d.StaticAssets); err != nil {
			return fmt.Errorf("wxmp static assets: %w", err)
		}
	}

	// 2. Interceptor plugins
	config_provider := d.ConfigProvider
	var icfg *InterceptorPluginConfig
	if d.Interceptor != nil {
		if config_provider == nil {
			return errors.New("wxmp config is required for interceptor registration")
		}
		var err error
		icfg, err = NewConfig(config_provider, d.Runtime)
		if err != nil {
			return fmt.Errorf("wxmp interceptor config: %w", err)
		}
		for _, p := range icfg.GetPlugins(adapter.AdapterContext{DB: d.DB, Logger: d.Logger}) {
			d.Interceptor.AddPostPlugin(p)
		}
	}

	// 3. Routes
	r, err := NewRoutes(config_provider, d.Runtime, d.Logger, d.DB)
	if err != nil {
		return fmt.Errorf("wxmp routes config: %w", err)
	}
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

// RegisterRuntime exposes the adapter through the shared registry contract.
func (a *OfficialAccountAdapter) RegisterRuntime(d adapter.RuntimeDeps) (adapter.RuntimeHandle, error) {
	err := a.register(Deps{
		StaticAssets:   d.StaticAssets,
		RouteRegistrar: d.Routes,
		Interceptor:    d.Interceptor,
		DB:             d.DB,
		Logger:         d.Logger,
		Bus:            d.Bus,
		ConfigProvider: d.ConfigProvider,
		Runtime:        d.Runtime,
	})
	if err != nil {
		return nil, err
	}
	return a, nil
}

// Stop shuts down the adapter's routes.
func (a *OfficialAccountAdapter) Stop() {
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
