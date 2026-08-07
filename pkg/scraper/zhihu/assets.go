package zhihu

import (
	"fmt"
	"io/fs"
	"strings"

	"wx_channel/internal/webassets"
)

// Assets contains the scripts owned by the zhihu scraper.
var Assets = NewAssets()

type InjectedAssets struct {
	InjectFS fs.FS
}

// StaticAssetsPath is the HTTP mount owned by the zhihu scraper.
// Keeping platform assets in a namespace prevents filename collisions with
// shared frontend assets and other scraper packages.
const StaticAssetsPath = "/__assets/platform/zhihu"

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

// InjectAssetURL builds a URL for an asset owned by this package.
func InjectAssetURL(baseURL, name string) string {
	return strings.TrimRight(baseURL, "/") + "/platform/zhihu/" + strings.TrimLeft(name, "/")
}
