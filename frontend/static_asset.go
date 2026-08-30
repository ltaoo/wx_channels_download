package frontend

import (
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/rs/zerolog"
)

type StaticAssetMockOptions struct {
	PlatformPrefix string
	PlatformFS     fs.FS
	UserScriptPath string
	Logger         *zerolog.Logger
}

func MockStaticAsset(pathname string, request_header http.Header, mock func(int, map[string]string, string), options StaticAssetMockOptions) bool {
	logger := options.Logger
	user_script_asset_path := ""
	if options.UserScriptPath != "" {
		user_script_asset_path = UserGlobalScriptAssetPath(options.UserScriptPath)
		if logger != nil && strings.HasPrefix(pathname, "/__assets/user/") && pathname != user_script_asset_path {
			logger.Warn().
				Str("file", "frontend/static_asset.go").
				Str("pathname", pathname).
				Str("expected_asset_path", user_script_asset_path).
				Str("path", options.UserScriptPath).
				Msg("user script asset request does not match configured global script asset path")
		}
	} else if logger != nil && strings.HasPrefix(pathname, "/__assets/user/") {
		logger.Warn().
			Str("file", "frontend/static_asset.go").
			Str("pathname", pathname).
			Msg("user script asset request received but global script path is empty")
	}

	var (
		data          []byte
		err           error
		rel           string
		cache_control string
		matched       bool
		raw_size      int
	)
	switch {
	case strings.HasPrefix(pathname, "/__assets/public/"):
		matched = true
		rel = strings.TrimPrefix(pathname, "/__assets/public/")
		data, err = Assets().ReadPublic(rel)
		cache_control = PublicAssetCacheControl
	case strings.HasPrefix(pathname, "/__assets/inject/"):
		matched = true
		rel = strings.TrimPrefix(pathname, "/__assets/inject/")
		data, err = Assets().ReadInject(rel)
		cache_control = SrcAssetCacheControl
	case strings.HasPrefix(pathname, "/__assets/src/"):
		matched = true
		rel = strings.TrimPrefix(pathname, "/__assets/src/")
		data, err = Assets().ReadSrc(rel)
		cache_control = SrcAssetCacheControl
	case user_script_asset_path != "" && pathname == user_script_asset_path:
		matched = true
		rel = path.Base(user_script_asset_path)
		data, err = os.ReadFile(options.UserScriptPath)
		cache_control = SrcAssetCacheControl
	case options.PlatformPrefix != "" && options.PlatformFS != nil && strings.HasPrefix(pathname, options.PlatformPrefix):
		matched = true
		rel = strings.TrimPrefix(pathname, options.PlatformPrefix)
		var ok bool
		rel, ok = clean_mock_asset_rel(rel)
		if !ok {
			err = fs.ErrInvalid
			break
		}
		data, err = fs.ReadFile(options.PlatformFS, rel)
		cache_control = SrcAssetCacheControl
	default:
		return false
	}
	if err != nil {
		if logger != nil {
			logger.Warn().
				Err(err).
				Str("file", "frontend/static_asset.go").
				Str("pathname", pathname).
				Str("asset", rel).
				Msg("failed to read interceptor static asset")
		}
		if matched {
			mock(http.StatusNotFound, map[string]string{
				"Content-Type":                StaticAssetContentType(rel),
				"Cache-Control":               "no-store",
				"Access-Control-Allow-Origin": "*",
			}, "")
			return true
		}
		return false
	}
	raw_size = len(data)
	data = StaticAssetResponseData(rel, data)
	headers := map[string]string{
		"Content-Type":                StaticAssetContentType(rel),
		"Cache-Control":               cache_control,
		"Access-Control-Allow-Origin": "*",
	}
	if cache_control == SrcAssetCacheControl {
		etag := StaticAssetETag(data)
		headers["ETag"] = etag
		if request_header != nil && strings.Contains(request_header.Get("If-None-Match"), etag) {
			if logger != nil && pathname == user_script_asset_path {
				logger.Info().
					Str("file", "frontend/static_asset.go").
					Str("asset_path", user_script_asset_path).
					Str("path", options.UserScriptPath).
					Str("etag", etag).
					Msg("global script asset matched etag; returning not modified")
			}
			mock(http.StatusNotModified, headers, "")
			return true
		}
	}
	if logger != nil && pathname == user_script_asset_path {
		logger.Info().
			Str("file", "frontend/static_asset.go").
			Str("asset_path", user_script_asset_path).
			Str("path", options.UserScriptPath).
			Int("bytes", raw_size).
			Msg("serving global script asset through interceptor")
	}
	mock(http.StatusOK, headers, string(data))
	return true
}

func clean_mock_asset_rel(rel string) (string, bool) {
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
