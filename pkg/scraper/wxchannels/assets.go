package wxchannels

import (
	"fmt"
	"io/fs"
	"net/url"
	"strings"

	"wx_channel/frontend"
	"wx_channel/internal/webassets"
)

// Assets contains the scripts owned by the video-channel scraper.
var Assets = NewAssets()

type InjectedAssets struct {
	InjectFS fs.FS
}

const assetPathPrefix = "/wxchannels"

// StaticAssetsPath is the HTTP mount owned by the video-channel scraper.
// Keeping scraper assets in a namespace prevents filename collisions with
// shared frontend assets and other scraper packages.
const StaticAssetsPath = "/__assets" + assetPathPrefix + "/inject"

func NewAssets() *InjectedAssets {
	return &InjectedAssets{InjectFS: embeddedInjectFS()}
}

// RegisterStaticAssets registers the assets owned by this package with the
// application asset registry. The scraper package depends only on the neutral
// registry, never on the API server implementation.
func RegisterStaticAssets(registry *webassets.Registry) error {
	if err := registry.Register(StaticAssetsPath, Assets.InjectFS); err != nil {
		return err
	}
	// Keep explicit aliases for pages injected before the scraper namespace was
	// introduced. Unlike the old API fallback chain, aliases remain owned by
	// this package and duplicate filenames fail during application setup.
	entries, err := fs.ReadDir(Assets.InjectFS, ".")
	if err != nil {
		return fmt.Errorf("read legacy asset aliases: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if err := registry.RegisterFile("/__assets/inject/"+entry.Name(), Assets.InjectFS, entry.Name()); err != nil {
			return fmt.Errorf("register legacy asset %q: %w", entry.Name(), err)
		}
	}
	return nil
}

// AssetURL builds a URL for an asset owned by this package.
// The endpoint is shared with the frontend asset server, but ownership stays
// in the wxchannels package.
func AssetURL(baseURL, name string, query ...url.Values) string {
	return frontend.NewURLBuild(baseURL, nil)(assetPathPrefix+"/"+strings.TrimLeft(name, "/"), query...)
}
