package wxmpadapter

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/rs/zerolog"

	"wx_channel/internal/adapter"
	"wx_channel/internal/events"
	"wx_channel/pkg/cache"
	"wx_channel/pkg/scraper/wxmp"
)

// OfficialAccountAdapter owns all wxmp platform and runtime capabilities.
type OfficialAccountAdapter struct {
	runtime_mu          sync.Mutex
	runtime_registered  bool
	client              *wxmp.Client
	file_cache          *cache.CacheProvider
	routes              *Routes
	interceptor_config  *InterceptorPluginConfig
	status_mu           sync.Mutex
	status_bus          *events.Bus
	cancel_status_check func()
}

var (
	_ adapter.PlatformAdapter          = (*OfficialAccountAdapter)(nil)
	_ adapter.RuntimeAdapter           = (*OfficialAccountAdapter)(nil)
	_ adapter.RuntimeHandle            = (*OfficialAccountAdapter)(nil)
	_ adapter.FetchCacheAdapter        = (*OfficialAccountAdapter)(nil)
	_ adapter.FetchDownloadTaskBuilder = (*OfficialAccountAdapter)(nil)
	_ adapter.Postprocessor            = (*OfficialAccountAdapter)(nil)
)

func init() {
	adapter.Register(NewOfficialAccountAdapter())
}

func NewOfficialAccountAdapter() *OfficialAccountAdapter {
	logger := zerolog.Nop()
	return &OfficialAccountAdapter{client: wxmp.NewClient(nil, &logger)}
}

func (a *OfficialAccountAdapter) PlatformID() string { return PlatformID }

func (a *OfficialAccountAdapter) Fetch(raw_url string) (any, error) {
	raw_url = strings.TrimSpace(raw_url)
	if raw_url == "" {
		return nil, errors.New("wxmp url is empty")
	}
	a.runtime_mu.Lock()
	client := a.client
	a.runtime_mu.Unlock()
	if client == nil {
		return nil, errors.New("wxmp scraper client is nil")
	}
	return client.FetchArticle(raw_url)
}

// Register creates and initializes a standalone official-account adapter.
func Register(d *adapter.AdapterOptions) (*OfficialAccountAdapter, error) {
	official_account_adapter := NewOfficialAccountAdapter()
	if err := official_account_adapter.register(d); err != nil {
		return nil, err
	}
	return official_account_adapter, nil
}

// register wires up static assets, interceptor plugins, routes, and lifecycle
// state on this adapter instance.
func (a *OfficialAccountAdapter) register(d *adapter.AdapterOptions) error {
	if d == nil {
		return errors.New("wxmp runtime dependencies are nil")
	}
	a.runtime_mu.Lock()
	if a.runtime_registered {
		a.runtime_mu.Unlock()
		return errors.New("wxmp adapter runtime is already registered")
	}
	a.runtime_registered = true
	if d.Logger != nil {
		a.client = wxmp.NewClient(nil, d.Logger)
	}
	if a.client != nil {
		a.client.SetPersistentCache(d.Cache)
	}
	a.file_cache = d.Cache
	a.runtime_mu.Unlock()
	a.status_mu.Lock()
	a.status_bus = d.Bus
	a.status_mu.Unlock()

	registered := false
	defer func() {
		if registered {
			return
		}
		a.runtime_mu.Lock()
		a.runtime_registered = false
		a.file_cache = nil
		if a.client != nil {
			a.client.SetPersistentCache(nil)
		}
		a.runtime_mu.Unlock()
		a.stop_status_check()
		a.status_mu.Lock()
		a.status_bus = nil
		a.status_mu.Unlock()
	}()

	if d.StaticAssets != nil {
		if err := register_static_assets(d.StaticAssets); err != nil {
			return fmt.Errorf("wxmp static assets: %w", err)
		}
	}

	var interceptor_config *InterceptorPluginConfig
	if d.Interceptor != nil {
		interceptor_config = NewConfig(d.Config)
		for _, p := range interceptor_config.GetPlugins() {
			d.Interceptor.AddPostPlugin(p)
		}
	}

	r := NewRoutes(d.Config, d.Logger)
	r.set_persistent_cache(d.Cache)
	if d.Routes != nil {
		r.RegisterRoutes(d.Routes)
	}

	a.runtime_mu.Lock()
	a.routes = r
	a.interceptor_config = interceptor_config
	a.runtime_mu.Unlock()
	registered = true
	return nil
}

// RegisterRuntime exposes the adapter through the shared registry contract.
func (a *OfficialAccountAdapter) RegisterRuntime(d *adapter.AdapterOptions) (adapter.RuntimeHandle, error) {
	if err := a.register(d); err != nil {
		return nil, err
	}
	a.RefreshPlatformStatus()
	return a, nil
}

// Stop shuts down the adapter's routes.
func (a *OfficialAccountAdapter) Stop() {
	if a == nil {
		return
	}
	a.runtime_mu.Lock()
	routes := a.routes
	client := a.client
	a.runtime_registered = false
	a.routes = nil
	a.file_cache = nil
	a.interceptor_config = nil
	a.runtime_mu.Unlock()
	if client != nil {
		client.SetPersistentCache(nil)
	}
	a.stop_status_check()
	a.status_mu.Lock()
	a.status_bus = nil
	a.status_mu.Unlock()
	if routes != nil {
		routes.Stop()
	}
}

// FetchCacheEntries returns the HTML cache produced for one article fetch.
func (a *OfficialAccountAdapter) FetchCacheEntries(raw_url string, _ any) ([]adapter.FetchCacheEntry, error) {
	cache_file, err := wxmp.LookupHTMLCache(a.runtime_file_cache(), strings.TrimSpace(raw_url))
	if err != nil || cache_file == nil {
		return nil, err
	}
	return []adapter.FetchCacheEntry{{
		Key:  "article-html",
		Name: "文章 HTML",
		URL:  strings.TrimSpace(raw_url),
		Path: cache_file.Path,
		Size: cache_file.Size,
	}}, nil
}

// ClearFetchCache removes the HTML cache produced for one article URL.
func (a *OfficialAccountAdapter) ClearFetchCache(raw_url string) (bool, error) {
	return wxmp.ClearHTMLCache(a.runtime_file_cache(), strings.TrimSpace(raw_url))
}

func (a *OfficialAccountAdapter) runtime_file_cache() *cache.CacheProvider {
	if a == nil {
		return nil
	}
	a.runtime_mu.Lock()
	file_cache := a.file_cache
	a.runtime_mu.Unlock()
	return file_cache
}
