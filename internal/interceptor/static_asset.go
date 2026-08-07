package interceptor

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"wx_channel/frontend"
	"wx_channel/internal/interceptor/proxy"
)

type FrontendStaticAssetMockOptions struct {
	PlatformPrefix string
	PlatformFS     fs.FS
}

func MockFrontendStaticAsset(ctx proxy.Context, pathname string, options FrontendStaticAssetMockOptions) bool {
	var (
		data          []byte
		err           error
		rel           string
		cache_control string
	)
	switch {
	case strings.HasPrefix(pathname, "/__assets/public/"):
		rel = strings.TrimPrefix(pathname, "/__assets/public/")
		data, err = frontend.Assets().ReadPublic(rel)
		cache_control = frontend.PublicAssetCacheControl
	case strings.HasPrefix(pathname, "/__assets/inject/"):
		rel = strings.TrimPrefix(pathname, "/__assets/inject/")
		data, err = frontend.Assets().ReadInject(rel)
		cache_control = frontend.SrcAssetCacheControl
	case strings.HasPrefix(pathname, "/__assets/src/"):
		rel = strings.TrimPrefix(pathname, "/__assets/src/")
		data, err = frontend.Assets().ReadSrc(rel)
		cache_control = frontend.SrcAssetCacheControl
	case options.PlatformPrefix != "" && options.PlatformFS != nil && strings.HasPrefix(pathname, options.PlatformPrefix):
		rel = strings.TrimPrefix(pathname, options.PlatformPrefix)
		var ok bool
		rel, ok = cleanMockAssetRel(rel)
		if !ok {
			return false
		}
		data, err = fs.ReadFile(options.PlatformFS, rel)
		cache_control = frontend.SrcAssetCacheControl
	default:
		return false
	}
	if err != nil {
		return false
	}
	data = frontend.StaticAssetResponseData(rel, data)
	headers := map[string]string{
		"Content-Type":                frontend.StaticAssetContentType(rel),
		"Cache-Control":               cache_control,
		"Access-Control-Allow-Origin": "*",
	}
	if cache_control == frontend.SrcAssetCacheControl {
		etag := frontend.StaticAssetETag(data)
		headers["ETag"] = etag
		if req := ctx.Req(); req != nil && req.Header != nil && strings.Contains(req.Header.Get("If-None-Match"), etag) {
			ctx.Mock(http.StatusNotModified, headers, "")
			return true
		}
	}
	ctx.Mock(http.StatusOK, headers, string(data))
	return true
}

func cleanMockAssetRel(rel string) (string, bool) {
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || strings.Contains(rel, "..") || strings.ContainsRune(rel, 0) {
		return "", false
	}
	clean := path.Clean(rel)
	if clean == "." || strings.HasPrefix(clean, "../") || clean == ".." {
		return "", false
	}
	return clean, true
}
