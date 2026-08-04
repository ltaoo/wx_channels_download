// Package registry holds platform download-task builder registrations.
package registry

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
	"wx_channel/internal/download/types"
	"wx_channel/internal/events"
	"wx_channel/internal/webassets"
	"wx_channel/pkg/hermes"
)

// PlatformHandler is the platform handler interface.
// Each platform module implements this interface, responsible for parsing its scraper type and generating download tasks.
type PlatformHandler interface {
	// PlatformID returns the unique platform identifier, e.g. "wxchannels", "wx_mp"
	PlatformID() string

	// BuildDownloadTask generates a download task result from the platform's raw content JSON and download config.
	// contentJSON: raw JSON data of the platform scraper object
	// configJSON: platform-specific download config JSON (directory, filename, quality, overwrite strategy, etc.)
	// Content and Account are embedded in the returned DownloadTaskResult.
	BuildDownloadTask(contentJSON json.RawMessage, configJSON json.RawMessage) (*types.DownloadTaskResult, error)
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

// DownloadTaskCreator lets adapter-owned routes create tasks without importing
// the services or API packages.
type DownloadTaskCreator func(platformID string, contentJSON, configJSON json.RawMessage) (any, error)

// RuntimeDeps contains host capabilities shared with a runtime adapter.
// Keeping this contract in the registry allows a future dynamic loader to
// construct adapters without application code importing concrete packages.
type RuntimeDeps struct {
	StaticAssets       *webassets.Registry
	Routes             RouteRegistrar
	Interceptor        InterceptorRegistrar
	DB                 *gorm.DB
	Logger             *zerolog.Logger
	Bus                *events.Bus
	Config             *config.Config
	RemoteServerMode   bool
	CreateDownloadTask DownloadTaskCreator
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

// GlobalScriptHandle is an optional runtime-handle capability used for status
// output. It deliberately stays separate from RuntimeHandle.
type GlobalScriptHandle interface {
	HasGlobalScript() bool
	GlobalScriptFilepath() string
}

// PostprocessDeps contains host services required during download postprocessing.
type PostprocessDeps struct {
	DB       *gorm.DB
	Logger   zerolog.Logger
	BasePath string
}

// Postprocessor is an optional adapter capability.
type Postprocessor interface {
	Postprocess(context.Context, *hermes.PostprocessInfo, PostprocessDeps) error
}

var (
	handlersMu sync.RWMutex
	handlers   = map[string]PlatformHandler{}
)

// Register registers a platform handler. Should be called in init().
func Register(h PlatformHandler) {
	handlersMu.Lock()
	defer handlersMu.Unlock()
	id := h.PlatformID()
	if _, dup := handlers[id]; dup {
		panic(fmt.Sprintf("registry: duplicate platform handler %q", id))
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
