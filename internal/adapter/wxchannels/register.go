package wxchannels

import (
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/adapterctx"
	"wx_channel/internal/config"
	"wx_channel/internal/download/registry"
	"wx_channel/internal/events"
	"wx_channel/internal/webassets"
	scraper "wx_channel/pkg/scraper/wxchannels"
)

// Deps holds the dependencies needed to register the wxchannels adapter.
type Deps struct {
	StaticAssets       *webassets.Registry
	RouteRegistrar     RouteRegistrar
	Interceptor        registry.InterceptorRegistrar
	DB                 *gorm.DB
	Logger             *zerolog.Logger
	Bus                *events.Bus
	Config             *config.Config
	RefreshInterval    int
	SphCookie          string
	RemoteServerMode   bool
	CreateDownloadTask registry.DownloadTaskCreator
}

// Handle provides access to the adapter's runtime components.
type Handle struct {
	routes         *WebsocketRoutes
	interceptorCfg *InterceptorPluginConfig
}

// Register wires up all wxchannels adapter components: static assets,
// interceptor plugins, HTTP routes, and lifecycle hooks.
func Register(d Deps) (*Handle, error) {
	// 1. Static assets
	if d.StaticAssets != nil {
		if err := scraper.RegisterStaticAssets(d.StaticAssets); err != nil {
			return nil, fmt.Errorf("wxchannels static assets: %w", err)
		}
	}

	// 2. Interceptor plugins
	icfg := NewConfig(d.Config)
	if d.Interceptor != nil {
		for _, p := range icfg.GetPlugins(adapterctx.AdapterContext{DB: d.DB, Logger: *d.Logger, Bus: d.Bus, BasePath: d.Config.GetDownloadDir()}) {
			d.Interceptor.AddPostPlugin(p)
		}
	}

	// 3. Routes
	var createTask ChannelsTaskCreator
	if d.CreateDownloadTask != nil {
		createTask = func(content json.RawMessage, savePath, filename, spec string, downloadCover, overwrite, duplicate, convertMP3 bool) (any, error) {
			cfg := map[string]any{
				"save_path":      savePath,
				"filename":       filename,
				"download_cover": downloadCover,
				"overwrite":      overwrite,
				"duplicate":      duplicate,
				"convert_mp3":    convertMP3,
			}
			if spec != "" {
				cfg["spec"] = spec
			}
			if convertMP3 {
				cfg["suffix"] = ".mp3"
			}
			configJSON, err := json.Marshal(cfg)
			if err != nil {
				return nil, fmt.Errorf("wxchannels download config: %w", err)
			}
			return d.CreateDownloadTask(PlatformID, content, configJSON)
		}
	}
	r := NewWebsocketRoutes(d.RefreshInterval, d.DB, d.SphCookie, d.RemoteServerMode, createTask)
	if d.RouteRegistrar != nil {
		r.RegisterRoutes(d.RouteRegistrar)
	}

	return &Handle{routes: r, interceptorCfg: icfg}, nil
}

// RegisterRuntime exposes the complete adapter through the shared registry
// contract. The concrete package remains responsible for interpreting config.
func (h *handler) RegisterRuntime(d registry.RuntimeDeps) (registry.RuntimeHandle, error) {
	refreshInterval := 0
	sphCookie := ""
	if d.Config != nil {
		refreshInterval = d.Config.GetInt("channels.refreshInterval")
		sphCookie = d.Config.GetString("cloudflare.sphCookie")
	}
	return Register(Deps{
		StaticAssets:       d.StaticAssets,
		RouteRegistrar:     d.Routes,
		Interceptor:        d.Interceptor,
		DB:                 d.DB,
		Logger:             d.Logger,
		Bus:                d.Bus,
		Config:             d.Config,
		RefreshInterval:    refreshInterval,
		SphCookie:          sphCookie,
		RemoteServerMode:   d.RemoteServerMode,
		CreateDownloadTask: d.CreateDownloadTask,
	})
}

// Stop shuts down the adapter's routes.
func (h *Handle) Stop() {
	if h != nil && h.routes != nil {
		h.routes.Stop()
	}
}

// HasGlobalScript reports whether a global inject script is configured.
func (h *Handle) HasGlobalScript() bool {
	return h != nil && h.interceptorCfg != nil && h.interceptorCfg.HasGlobalScript()
}

// GlobalScriptFilepath returns the global inject script filepath.
func (h *Handle) GlobalScriptFilepath() string {
	if h != nil && h.interceptorCfg != nil {
		return h.interceptorCfg.GlobalScriptFilepath()
	}
	return ""
}
