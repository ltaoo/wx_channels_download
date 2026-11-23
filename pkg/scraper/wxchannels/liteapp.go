package wxchannels

import (
	_ "embed"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/ltaoo/echo"
	"github.com/rs/zerolog"
)

var liteapp_sdk_reg = regexp.MustCompile(`^/t/wx_fed/cdn_libs/res/libpag/[^/]+/libpag(?:\.min)?\.js$`)

const liteapp_asset_path = "/__assets/wxchannels/liteapp/"

//go:embed liteapp_runtime.js
var liteapp_runtime_js string

func inject_liteapp_runtime(ctx *echo.Context, logger *zerolog.Logger, inline bool) bool {
	if ctx.Req.URL.Hostname() != "res.wx.qq.com" || !liteapp_sdk_reg.MatchString(ctx.Req.URL.Path) {
		return false
	}
	if ctx.Res == nil || ctx.Res.StatusCode != http.StatusOK || !strings.Contains(strings.ToLower(ctx.GetResponseHeader("Content-Type")), "javascript") {
		return true
	}
	ctx.SetResponseHeader("Cache-Control", "no-store")
	ctx.DelResponseHeader("ETag")
	ctx.DelResponseHeader("Last-Modified")
	if ctx.Req.Method != http.MethodGet {
		return true
	}
	sdk_source, err := ctx.GetResponseBody()
	if err != nil {
		logger.Warn().Err(err).Msg("failed to read LiteApp SDK response")
		return true
	}
	script := liteapp_runtime_js
	if !inline {
		script = `(function () {
  const script = document.createElement("script");
  script.setAttribute("data-n", "wxchannels-liteapp-runtime");
  script.crossOrigin = "anonymous";
  script.src = "https://res.wx.qq.com` + liteapp_asset_path + `runtime.js?t=" + Date.now();
  script.onerror = () => console.warn("[wxchannels LiteApp] failed to load runtime");
  (document.head || document.documentElement).appendChild(script);
})();`
	}
	ctx.SetResponseBody(sdk_source + "\n;" + script)
	return true
}

func dev_liteapp_runtime_path() string {
	candidates := []string{filepath.Join("pkg", "scraper", "wxchannels", "liteapp_runtime.js")}
	if _, source_file, _, ok := runtime.Caller(0); ok {
		candidates = append(candidates, filepath.Join(filepath.Dir(source_file), "liteapp_runtime.js"))
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return candidates[0]
}

func new_liteapp_runtime(page_plugin *echo.Plugin, logger *zerolog.Logger, inline bool) func(*echo.Context) bool {
	var mutex sync.Mutex
	last_status := `{}`
	runtime_path := ""
	if !inline {
		runtime_path = dev_liteapp_runtime_path()
	}
	return func(ctx *echo.Context) bool {
		pathname := ctx.Req.URL.Path
		if !strings.HasPrefix(pathname, liteapp_asset_path) {
			return false
		}
		headers := map[string]string{
			"Access-Control-Allow-Origin":  "https://liteapp.weixin.qq.com",
			"Access-Control-Allow-Methods": "GET, HEAD, POST, OPTIONS",
			"Access-Control-Allow-Headers": "Content-Type",
			"Cache-Control":                "no-store",
			"Content-Type":                 "application/json; charset=utf-8",
			"X-Content-Type-Options":       "nosniff",
		}
		if origin := ctx.Req.Header.Get("Origin"); origin != "" && origin != "https://liteapp.weixin.qq.com" {
			ctx.Mock(http.StatusForbidden, headers, `{"error":"unexpected origin"}`)
			return true
		}
		if ctx.Req.Method == http.MethodOptions {
			ctx.Mock(http.StatusNoContent, headers, "")
			return true
		}
		if !inline && pathname == liteapp_asset_path+"runtime.js" && (ctx.Req.Method == http.MethodGet || ctx.Req.Method == http.MethodHead) {
			// Read on every request so development edits do not require restarting Go.
			source, err := os.ReadFile(runtime_path)
			if err != nil {
				logger.Warn().Err(err).Msg("failed to read LiteApp runtime source")
				ctx.Mock(http.StatusInternalServerError, headers, `{"error":"failed to read LiteApp runtime"}`)
				return true
			}
			headers["Content-Type"] = "application/javascript; charset=utf-8"
			body := string(source)
			if ctx.Req.Method == http.MethodHead {
				body = ""
			}
			ctx.Mock(http.StatusOK, headers, body)
			return true
		}
		if pathname == liteapp_asset_path+"bootstrap" && ctx.Req.Method == http.MethodGet {
			req, _ := http.NewRequest(http.MethodGet, "https://channels.weixin.qq.com/web/pages/feed", nil)
			page_ctx := &echo.Context{Req: req, Res: &http.Response{StatusCode: http.StatusOK,
				Header: http.Header{"Content-Type": {"text/html"}}, Body: io.NopCloser(strings.NewReader("<html><head></head><body></body></html>"))}}
			page_plugin.OnResponse(page_ctx)
			bootstrap, err := page_ctx.GetResponseBody()
			if err != nil {
				ctx.Mock(http.StatusInternalServerError, headers, `{"error":"failed to build injection assets"}`)
				return true
			}
			bootstrap = strings.ReplaceAll(bootstrap, `"assets_base_url":"/__assets"`, `"assets_base_url":"https://res.wx.qq.com/__assets"`)
			response, _ := json.Marshal(map[string]string{"bootstrap": bootstrap})
			ctx.Mock(http.StatusOK, headers, string(response))
			return true
		}
		if pathname == liteapp_asset_path+"status" {
			if ctx.Req.Method == http.MethodPost {
				var status struct {
					Stage        string `json:"stage"`
					Revision     string `json:"revision"`
					Methods      int    `json:"methods"`
					InitEvents   int    `json:"init_events"`
					DetailEvents int    `json:"detail_events"`
					AppPreserved bool   `json:"app_preserved"`
					Error        string `json:"error,omitempty"`
				}
				body, err := io.ReadAll(io.LimitReader(ctx.Req.Body, 4097))
				if err != nil || len(body) > 4096 || json.Unmarshal(body, &status) != nil {
					ctx.Mock(http.StatusBadRequest, headers, `{"error":"invalid status"}`)
					return true
				}
				body, _ = json.Marshal(status)
				mutex.Lock()
				last_status = string(body)
				mutex.Unlock()
				logger.Info().RawJSON("status", body).Msg("LiteApp rewrite status")
			}
			if ctx.Req.Method == http.MethodGet || ctx.Req.Method == http.MethodPost {
				mutex.Lock()
				body := last_status
				mutex.Unlock()
				ctx.Mock(http.StatusOK, headers, body)
				return true
			}
		}
		ctx.Mock(http.StatusNotFound, headers, `{"error":"unknown LiteApp asset"}`)
		return true
	}
}
