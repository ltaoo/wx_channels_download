package wxchannels

import (
	"io/fs"
	"strings"
)

// Assets contains the scripts owned by the video-channel scraper.
var Assets = NewAssets()

type InjectedAssets struct {
	RootFS   fs.FS
	InjectFS fs.FS
}

func NewAssets() *InjectedAssets {
	return &InjectedAssets{RootFS: embeddedRootFS(), InjectFS: embeddedInjectFS()}
}

func (a *InjectedAssets) ReadInject(name string) ([]byte, error) {
	return fs.ReadFile(a.InjectFS, name)
}

// ChannelInjectAssetURL builds a URL for an asset owned by this package.
// The endpoint is shared with the frontend asset server, but ownership stays
// in the wxchannels package.
func ChannelInjectAssetURL(baseURL, name string) string {
	return strings.TrimRight(baseURL, "/") + "/inject/" + strings.TrimLeft(name, "/")
}
