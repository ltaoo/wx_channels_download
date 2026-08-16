package wxmp

import (
	"io/fs"
)

var inject_assets = embeddedInjectFS()

// InjectAssets returns scripts owned by the official-account scraper.
func InjectAssets() fs.FS {
	return inject_assets
}
