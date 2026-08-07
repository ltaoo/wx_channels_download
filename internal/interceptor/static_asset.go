package interceptor

import (
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/rs/zerolog"

	"wx_channel/frontend"
	"wx_channel/internal/interceptor/proxy"
)

type FrontendStaticAssetMockOptions struct {
	PlatformPrefix string
	PlatformFS     fs.FS
	UserScriptPath string
	Logger         *zerolog.Logger
}

func MockFrontendStaticAsset(ctx proxy.Context, pathname string, options FrontendStaticAssetMockOptions) bool {
	logger := options.Logger
	user_script_asset_path := ""
	if options.UserScriptPath != "" {
		user_script_asset_path = frontend.UserGlobalScriptAssetPath(options.UserScriptPath)
		if logger != nil && strings.HasPrefix(pathname, "/__assets/user/") && pathname != user_script_asset_path {
			logger.Warn().
				Str("file", "internal/interceptor/static_asset.go").
				Str("pathname", pathname).
				Str("expected_asset_path", user_script_asset_path).
				Str("path", options.UserScriptPath).
				Msg("user script asset request does not match configured global script asset path")
		}
	} else if logger != nil && strings.HasPrefix(pathname, "/__assets/user/") {
		logger.Warn().
			Str("file", "internal/interceptor/static_asset.go").
			Str("pathname", pathname).
			Msg("user script asset request received but global script path is empty")
	}

	var (
		data          []byte
		err           error
		rel           string
		cache_control string
		raw_size      int
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
	case user_script_asset_path != "" && pathname == user_script_asset_path:
		rel = path.Base(user_script_asset_path)
		data, err = os.ReadFile(options.UserScriptPath)
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
		if logger != nil && pathname == user_script_asset_path {
			logger.Warn().
				Err(err).
				Str("file", "internal/interceptor/static_asset.go").
				Str("asset_path", user_script_asset_path).
				Str("path", options.UserScriptPath).
				Msg("failed to read global script asset for interceptor response")
		}
		return false
	}
	raw_size = len(data)
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
			if logger != nil && pathname == user_script_asset_path {
				logger.Info().
					Str("file", "internal/interceptor/static_asset.go").
					Str("asset_path", user_script_asset_path).
					Str("path", options.UserScriptPath).
					Str("etag", etag).
					Msg("global script asset matched etag; returning not modified")
			}
			ctx.Mock(http.StatusNotModified, headers, "")
			return true
		}
	}
	if logger != nil && pathname == user_script_asset_path {
		logger.Info().
			Str("file", "internal/interceptor/static_asset.go").
			Str("asset_path", user_script_asset_path).
			Str("path", options.UserScriptPath).
			Int("bytes", raw_size).
			Msg("serving global script asset through interceptor")
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
