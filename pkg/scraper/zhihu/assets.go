package zhihu

import (
	"fmt"
	"io/fs"
	"net/url"
	"strings"

	"wx_channel/frontend"
	"wx_channel/internal/webassets"
)

// Assets contains the scripts owned by the zhihu scraper.
var Assets = NewAssets()

type InjectedAssets struct {
	InjectFS fs.FS
}

const assetPathPrefix = "/zhihu"

// StaticAssetsPath is the HTTP mount owned by the zhihu scraper.
// Keeping scraper assets in a namespace prevents filename collisions with
// shared frontend assets and other scraper packages.
const StaticAssetsPath = "/__assets" + assetPathPrefix + "/inject"

func NewAssets() *InjectedAssets {
	return &InjectedAssets{InjectFS: embeddedInjectFS()}
}

// RegisterStaticAssets registers the assets owned by this package with the
// application asset registry.
func RegisterStaticAssets(registry *webassets.Registry) error {
	if err := registry.Register(StaticAssetsPath, Assets.InjectFS); err != nil {
		return err
	}
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
func AssetURL(baseURL, name string, query ...url.Values) string {
	return frontend.NewURLBuild(baseURL, nil)(assetPathPrefix+"/"+strings.TrimLeft(name, "/"), query...)
}
