// Package adapter defines the platform adapter contract and global registry.
// Every platform adapter implements PlatformAdapter (at minimum PlatformHandler).
// Additional capabilities — RuntimeAdapter, Postprocessor —
// are optional and discovered via type assertion at runtime.
package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/config"
	"wx_channel/internal/events"
	"wx_channel/internal/webassets"
	"wx_channel/pkg/hermes"
)

// ---------- Interfaces ----------

// PlatformHandler is the platform handler interface.
// Each platform module implements this interface, responsible for parsing its scraper type and generating download tasks.
type PlatformHandler interface {
	// PlatformID returns the unique platform identifier, e.g. "wxchannels", "wx_mp"
	PlatformID() string

	// Fetch retrieves raw scraper data for a platform-specific URL.
	Fetch(rawURL string) (any, error)

	// BuildDownloadTask generates a download task result from the platform's raw content JSON and download config.
	// contentJSON: raw JSON data of the platform scraper object
	// configJSON: platform-specific download config JSON (directory, filename, quality, overwrite strategy, etc.)
	// Content and Account are embedded in the returned DownloadTaskResult.
	BuildDownloadTask(contentJSON json.RawMessage, configJSON json.RawMessage) (*DownloadTaskResult, error)

	// BuildBrowseHistory converts intercepted platform content into a browse
	// history record and its related account.
	BuildBrowseHistory(contentJSON json.RawMessage) (*BrowseHistoryResult, error)
}

// PlatformAdapter is the complete interface for a platform adapter.
// Every adapter must implement PlatformHandler at minimum.
// Additional capabilities — RuntimeAdapter, Postprocessor —
// are optional and discovered via type assertion at runtime.
type PlatformAdapter interface {
	PlatformHandler
}

// RouteRegistrar is the HTTP capability exposed by the host to adapters.
type RouteRegistrar interface {
	RegisterGET(path string, handler gin.HandlerFunc)
	RegisterPOST(path string, handler gin.HandlerFunc)
}

// InterceptorRegistrar is the proxy capability exposed by the host to adapters.
type InterceptorRegistrar interface {
	AddPostPlugin(plugin interface{})
}

// RuntimeDeps contains host capabilities shared with a runtime adapter.
// Keeping this contract in the registry allows a future dynamic loader to
// construct adapters without application code importing concrete packages.
type RuntimeDeps struct {
	StaticAssets *webassets.Registry
	Routes       RouteRegistrar
	Interceptor  InterceptorRegistrar
	DB           *gorm.DB
	Logger       *zerolog.Logger
	Bus          *events.Bus
	Config       *config.Config
	Hooks        *hermes.HookManager
}

// RuntimeHandle owns resources started by an adapter.
type RuntimeHandle interface {
	Stop()
}

// RuntimeAdapter is implemented by adapters that install routes, assets,
// interceptor hooks, or other long-lived runtime components.
type RuntimeAdapter interface {
	RegisterRuntime(RuntimeDeps) (RuntimeHandle, error)
}

// PostprocessDeps contains host services required during download postprocessing.
type PostprocessDeps struct {
	DB       *gorm.DB
	Logger   zerolog.Logger
	BasePath string
}

// Postprocessor is an optional adapter capability.
type Postprocessor interface {
	Postprocess(context.Context, *hermes.TaskJob, PostprocessDeps) error
}

// ---------- AdapterContext ----------

// AdapterContext bundles the shared dependencies that platform adapters need:
// database access, structured logging, an event bus, and the absolute download root.
type AdapterContext struct {
	DB       *gorm.DB
	Logger   *zerolog.Logger
	Bus      *events.Bus
	BasePath string // absolute download root directory
}

// ---------- Global registry ----------

var (
	handlersMu sync.RWMutex
	handlers   = map[string]PlatformHandler{}
)

// Register registers a platform adapter. Should be called in init().
func Register(h PlatformAdapter) {
	handlersMu.Lock()
	defer handlersMu.Unlock()
	id := h.PlatformID()
	if _, dup := handlers[id]; dup {
		panic(fmt.Sprintf("adapter: duplicate platform adapter %q", id))
	}
	handlers[id] = h
}

// Get retrieves a handler by platform ID, returns nil if not found.
func Get(platformID string) PlatformHandler {
	handlersMu.RLock()
	defer handlersMu.RUnlock()
	return handlers[platformID]
}

// IDs returns all registered platform IDs in stable order.
func IDs() []string {
	handlersMu.RLock()
	defer handlersMu.RUnlock()
	ids := make([]string, 0, len(handlers))
	for id := range handlers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
