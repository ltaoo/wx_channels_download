package wxchannels

import (
	"fmt"
	"io/fs"
	"strings"

	"wx_channel/frontend"
	"wx_channel/internal/interceptor/proxy"
	"wx_channel/internal/webassets"
)

// Assets contains the scripts owned by the video-channel scraper.
var Assets = NewAssets()

type InjectedAssets struct {
	RootFS   fs.FS
	InjectFS fs.FS
}

// StaticAssetsPath is the HTTP mount owned by the video-channel scraper.
// Keeping platform assets in a namespace prevents filename collisions with
// shared frontend assets and other scraper packages.
const StaticAssetsPath = "/__assets/platform/wxchannels"

func NewAssets() *InjectedAssets {
	return &InjectedAssets{RootFS: embeddedRootFS(), InjectFS: embeddedInjectFS()}
}

func (a *InjectedAssets) ReadInject(name string) ([]byte, error) {
	return fs.ReadFile(a.InjectFS, name)
}

// RegisterStaticAssets registers the assets owned by this package with the
// application asset registry. The scraper package depends only on the neutral
// registry, never on the API server implementation.
func RegisterStaticAssets(registry *webassets.Registry) error {
	if err := registry.Register(StaticAssetsPath, Assets.InjectFS); err != nil {
		return err
	}
	// Keep explicit aliases for pages injected before the platform namespace was
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

// MockStaticAsset serves a platform-owned asset when the browser requests it
// from channels.weixin.qq.com through the local interceptor. Those same-origin
// requests never reach the local API server, so they must be handled here as
// well as registered with the API asset registry.
func MockStaticAsset(ctx proxy.Context, pathname string) bool {
	const prefix = StaticAssetsPath + "/"
	if !strings.HasPrefix(pathname, prefix) {
		return false
	}
	rel := strings.TrimPrefix(pathname, prefix)
	if !fs.ValidPath(rel) {
		return false
	}
	data, err := Assets.ReadInject(rel)
	if err != nil {
		return false
	}
	etag := frontend.ChannelStaticAssetETag(data)
	headers := map[string]string{
		"Content-Type":  frontend.ChannelStaticAssetContentType(rel),
		"Cache-Control": frontend.ChannelSrcAssetCacheControl,
		"ETag":          etag,
	}
	if req := ctx.Req(); req != nil && req.Header != nil && strings.Contains(req.Header.Get("If-None-Match"), etag) {
		ctx.Mock(304, headers, "")
		return true
	}
	ctx.Mock(200, headers, string(data))
	return true
}

// ChannelInjectAssetURL builds a URL for an asset owned by this package.
// The endpoint is shared with the frontend asset server, but ownership stays
// in the wxchannels package.
func ChannelInjectAssetURL(baseURL, name string) string {
	return strings.TrimRight(baseURL, "/") + "/platform/wxchannels/" + strings.TrimLeft(name, "/")
}
