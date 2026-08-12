package wxchannels

import (
	"io/fs"
)

var inject_assets = embeddedInjectFS()

// InjectAssets returns the scripts owned by the video-channel scraper.
// Mount paths and HTTP registration remain the responsibility of the caller.
func InjectAssets() fs.FS {
	return inject_assets
}
