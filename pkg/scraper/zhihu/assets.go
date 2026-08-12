package zhihu

import (
	"io/fs"
)

var inject_assets = embeddedInjectFS()

// InjectAssets returns scripts owned by the Zhihu scraper.
func InjectAssets() fs.FS {
	return inject_assets
}
