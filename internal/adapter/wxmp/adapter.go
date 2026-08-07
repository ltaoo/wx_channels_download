package wxmpadapter

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
	Config         *config.Config
}

// OfficialAccountAdapter owns all wxmp platform and runtime capabilities.
type OfficialAccountAdapter struct {
	runtimeMu         sync.Mutex
	runtimeRegistered bool
	routes            *Routes
	interceptorConfig *InterceptorPluginConfig
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

func (a *OfficialAccountAdapter) PlatformID() string { return PlatformID }

func (a *OfficialAccountAdapter) Fetch(rawURL string) (any, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, errors.New("wxmp url is empty")
	}
	return (&wxmp.OfficialAccountDownload{}).FetchArticle(rawURL)
}

// Register creates and initializes a standalone official-account adapter.
func Register(d Deps) (*OfficialAccountAdapter, error) {
	officialAccountAdapter := NewOfficialAccountAdapter()
	if err := officialAccountAdapter.register(d); err != nil {
		return nil, err
	}
	return officialAccountAdapter, nil
}

// register wires up static assets, interceptor plugins, routes, and lifecycle
// state on this adapter instance.
func (a *OfficialAccountAdapter) register(d Deps) error {
	a.runtimeMu.Lock()
	if a.runtimeRegistered {
		a.runtimeMu.Unlock()
		return errors.New("wxmp adapter runtime is already registered")
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
		if err := wxmp.RegisterStaticAssets(d.StaticAssets); err != nil {
			return fmt.Errorf("wxmp static assets: %w", err)
		}
	}

	var interceptorConfig *InterceptorPluginConfig
	if d.Interceptor != nil {
		interceptorConfig = NewConfig(d.Config)
		for _, p := range interceptorConfig.GetPlugins(adapter.AdapterContext{DB: d.DB, Logger: d.Logger}) {
			d.Interceptor.AddPostPlugin(p)
		}
	}

	r := NewRoutes(d.Config, d.Logger, d.DB)
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

// RegisterRuntime exposes the adapter through the shared registry contract.
func (a *OfficialAccountAdapter) RegisterRuntime(d adapter.RuntimeDeps) (adapter.RuntimeHandle, error) {
	if err := a.register(Deps{
		StaticAssets:   d.StaticAssets,
		RouteRegistrar: d.Routes,
		Interceptor:    d.Interceptor,
		DB:             d.DB,
		Logger:         d.Logger,
		Bus:            d.Bus,
		Config:         d.Config,
	}); err != nil {
		return nil, err
	}
	return a, nil
}

// Stop shuts down the adapter's routes.
func (a *OfficialAccountAdapter) Stop() {
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
