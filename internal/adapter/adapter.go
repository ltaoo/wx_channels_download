// Package adapter defines the platform adapter contract and global registry.
// Every platform adapter implements PlatformAdapter (at minimum AdapterHandler).
// Additional capabilities — RuntimeAdapter, Postprocessor —
// are optional and discovered via type assertion at runtime.
package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/config"
	"wx_channel/internal/database/model"
	"wx_channel/internal/events"
	"wx_channel/internal/webassets"
	"wx_channel/pkg/cache"
	"wx_channel/pkg/cookies"
	"wx_channel/pkg/hermes"
)

// ---------- Interfaces ----------

// AdapterHandler is the platform handler interface.
// Each platform module implements this interface, responsible for parsing its scraper type and generating download tasks.
type AdapterHandler interface {
	// PlatformID returns the unique platform identifier, e.g. "wxchannels", "wx_mp"
	PlatformID() string

	// Fetch retrieves raw scraper data for a platform-specific URL.
	Fetch(rawURL string) (any, error)

	// ToContent converts raw scraper data into the shared content model.
	ToContent(data any) (*model.Content, error)

	// ToAccount converts raw scraper data into the shared account model.
	ToAccount(data any) (*model.Account, error)

	// ToContentDetails converts raw scraper data into ordered, type-specific
	// content detail records. Fetch jobs publish these records incrementally.
	ToContentDetails(data any) ([]ContentDetail, error)

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
// Every adapter must implement AdapterHandler at minimum.
// Additional capabilities — RuntimeAdapter, Postprocessor —
// are optional and discovered via type assertion at runtime.
type PlatformAdapter interface {
	AdapterHandler
}

// ProgressFetchAdapter is an optional fetch capability for adapters that emit
// progress events associated with a caller-provided request ID.
type ProgressFetchAdapter interface {
	FetchWithProgress(raw_url string, request_id string) (any, error)
}

// FetchOptions controls a context-aware scraper fetch.
type FetchOptions struct {
	RequestID       string
	ForceRefresh    bool
	ArtifactHandler FetchArtifactHandler
}

const (
	FetchArtifactStageContent       = "content"
	FetchArtifactStageAccount       = "account"
	FetchArtifactStageContentDetail = "content_detail"
)

// ContentDetail wraps a type-specific model with a stable key for incremental
// fetch-job updates.
type ContentDetail struct {
	Type string `json:"type"`
	Key  string `json:"key"`
	Data any    `json:"data"`
}

// FetchArtifact is a platform-neutral piece of a fetch result. Context-aware
// adapters may emit artifacts before their complete raw result is available.
type FetchArtifact struct {
	Stage         string
	Content       *model.Content
	Account       *model.Account
	ContentDetail *ContentDetail
	Current       int
	Total         int
}

// FetchArtifactHandler receives ordered fetch artifacts.
type FetchArtifactHandler func(FetchArtifact)

// ContextProgressFetchAdapter supports cancellation and cache refresh options.
type ContextProgressFetchAdapter interface {
	FetchWithProgressContext(fetch_context context.Context, raw_url string, options FetchOptions) (any, error)
}

// FetchCacheAdapter owns persistent fetch caches associated with source URLs.
type FetchCacheAdapter interface {
	ClearFetchCache(raw_url string) (bool, error)
}

// PlatformStatusRefresher is an optional runtime capability for adapters whose
// availability depends on mutable local state such as cookies.
type PlatformStatusRefresher interface {
	RefreshPlatformStatus()
}

// PlatformStatusDescriptor describes one user-visible availability entry. Most
// adapters expose one descriptor keyed by PlatformID; adapters with URL-specific
// capabilities may expose several descriptors under the same platform.
type PlatformStatusDescriptor struct {
	Platform string
	Key      string
	Name     string
}

// PlatformStatusDescriber is implemented by adapters that expose one or more
// status entries instead of a single platform-wide status.
type PlatformStatusDescriber interface {
	PlatformStatuses() []PlatformStatusDescriptor
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

// AdapterOptions contains host capabilities shared with one runtime adapter. The
// application creates a platform-scoped instance and passes its pointer to the
// adapter. Adapters must treat it as immutable; concrete host structures are
// exposed by pointer, while framework-facing capabilities use narrow
// interfaces.
type AdapterOptions struct {
	StaticAssets *webassets.Registry
	Routes       RouteRegistrar
	Interceptor  InterceptorRegistrar
	DB           *gorm.DB
	Logger       *zerolog.Logger
	Bus          *events.Bus
	Config       *config.Config
	Cache        *cache.CacheProvider // already scoped to the adapter's platform namespace
	Cookies      *cookies.Reader
	Hooks        *hermes.HookManager
}

// RuntimeHandle owns resources started by an adapter.
type RuntimeHandle interface {
	Stop()
}

// RuntimeAdapter is implemented by adapters that install routes, assets,
// interceptor hooks, or other long-lived runtime components.
type RuntimeAdapter interface {
	RegisterRuntime(*AdapterOptions) (RuntimeHandle, error)
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

// ---------- Global registry ----------

var (
	handlersMu sync.RWMutex
	handlers   = map[string]AdapterHandler{}
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
func Get(platformID string) AdapterHandler {
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

// StatusDescriptors returns all registered status entries in stable order.
func StatusDescriptors() []PlatformStatusDescriptor {
	handlersMu.RLock()
	defer handlersMu.RUnlock()
	descriptors := make([]PlatformStatusDescriptor, 0, len(handlers))
	for platform_id, handler := range handlers {
		if describer, ok := handler.(PlatformStatusDescriber); ok {
			for _, descriptor := range describer.PlatformStatuses() {
				descriptor = normalize_status_descriptor(platform_id, descriptor)
				if descriptor.Key != "" {
					descriptors = append(descriptors, descriptor)
				}
			}
			continue
		}
		descriptors = append(descriptors, PlatformStatusDescriptor{
			Platform: platform_id,
			Key:      platform_id,
		})
	}
	sort.Slice(descriptors, func(i, j int) bool {
		if descriptors[i].Platform == descriptors[j].Platform {
			return descriptors[i].Key < descriptors[j].Key
		}
		return descriptors[i].Platform < descriptors[j].Platform
	})
	return descriptors
}

func normalize_status_descriptor(platform_id string, descriptor PlatformStatusDescriptor) PlatformStatusDescriptor {
	descriptor.Platform = strings.TrimSpace(descriptor.Platform)
	if descriptor.Platform == "" {
		descriptor.Platform = platform_id
	}
	descriptor.Key = strings.TrimSpace(descriptor.Key)
	if descriptor.Key == "" {
		descriptor.Key = descriptor.Platform
	}
	descriptor.Name = strings.TrimSpace(descriptor.Name)
	return descriptor
}
