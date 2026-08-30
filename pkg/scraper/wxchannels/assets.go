package wxchannels

import (
	"io/fs"
	"net/url"
	"strings"

	"wx_channel/frontend"
)

const asset_path_prefix = "/wxchannels"

// InjectAssetsPath is the proxy path that serves wxchannels-owned scripts.
const InjectAssetsPath = "/__assets" + asset_path_prefix + "/inject"

var inject_assets = embeddedInjectFS()

// InjectAssets returns the scripts owned by the video-channel scraper.
// Mount paths and HTTP registration remain the responsibility of the caller.
func InjectAssets() fs.FS {
	return inject_assets
}

func asset_url(base_url, name string, query ...url.Values) string {
	return frontend.NewURLBuild(base_url, nil)(asset_path_prefix+"/"+strings.TrimLeft(name, "/"), query...)
}
