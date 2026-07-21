package wxmp

import (
	"fmt"
	"io/fs"
	"strings"

	"wx_channel/frontend"
	"wx_channel/internal/interceptor/proxy"
	"wx_channel/internal/webassets"
)

// Assets contains the scripts owned by the wxmp scraper.
var Assets = NewAssets()

type InjectedAssets struct {
	RootFS   fs.FS
	InjectFS fs.FS
}

// StaticAssetsPath is the HTTP mount owned by the official-account scraper.
const StaticAssetsPath = "/__assets/platform/wxmp"

func NewAssets() *InjectedAssets {
	return &InjectedAssets{RootFS: embeddedRootFS(), InjectFS: embeddedInjectFS()}
}

func (a *InjectedAssets) ReadInject(name string) ([]byte, error) {
	return fs.ReadFile(a.InjectFS, name)
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

// MockStaticAsset serves a platform-owned asset for same-origin requests
// intercepted on mp.weixin.qq.com.
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
func ChannelInjectAssetURL(baseURL, name string) string {
	return strings.TrimRight(baseURL, "/") + "/platform/wxmp/" + strings.TrimLeft(name, "/")
}
