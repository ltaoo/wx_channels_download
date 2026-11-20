package wxchannels

import (
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/parser"
	"github.com/ltaoo/echo"
	"github.com/rs/zerolog"
)

var liteapp_sdk_reg = regexp.MustCompile(`^/t/wx_fed/cdn_libs/res/libpag/[^/]+/libpag(?:\.min)?\.js$`)
var liteapp_module_id_reg = regexp.MustCompile(`^[0-9]{1,10}$`)

const liteapp_asset_path = "/__assets/wxchannels/liteapp/"

//go:embed liteapp_runtime.js
var liteapp_runtime_js string

func inject_liteapp_runtime(ctx *echo.Context, logger *zerolog.Logger) bool {
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
	ctx.SetResponseBody(sdk_source + "\n;" + liteapp_runtime_js)
	return true
}

func rewrite_liteapp_module(module_id, source string) (string, string, error) {
	if !liteapp_module_id_reg.MatchString(module_id) || !strings.Contains(source, "async finderGetCommentDetail(") {
		return "", "", fmt.Errorf("unsupported LiteApp API module")
	}
	rewritten := rewrite_channels_api_js(source, false)
	if rewritten == source {
		return "", "", fmt.Errorf("no API rewrite matched")
	}
	program, err := parser.ParseFile(nil, "liteapp-module.js", "("+rewritten+")", 0)
	if err != nil {
		return "", "", fmt.Errorf("invalid rewritten module: %w", err)
	}
	if len(program.Body) != 1 {
		return "", "", fmt.Errorf("expected one module factory")
	}
	statement, ok := program.Body[0].(*ast.ExpressionStatement)
	if !ok {
		return "", "", fmt.Errorf("expected a module expression")
	}
	factory, ok := statement.Expression.(*ast.FunctionLiteral)
	if !ok || factory.Async || factory.Generator {
		return "", "", fmt.Errorf("expected a synchronous module factory")
	}
	// Include the wrapper in the digest so loader changes also invalidate its URL.
	body := fmt.Sprintf("window.__wxchannels_liteapp_rewrite__.register(%q,(%s));\n", module_id, rewritten)
	revision := fmt.Sprintf("%x", sha256.Sum256([]byte(body)))
	return body, revision, nil
}

func new_liteapp_rewriter(page_plugin *echo.Plugin, logger *zerolog.Logger) func(*echo.Context) bool {
	var mutex sync.Mutex
	modules := map[string]string{}
	last_status := `{}`
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
		if pathname == liteapp_asset_path+"compile" && ctx.Req.Method == http.MethodPost {
			var input struct {
				ModuleID string `json:"module_id"`
				Source   string `json:"source"`
			}
			body, err := io.ReadAll(io.LimitReader(ctx.Req.Body, (512<<10)+1))
			if err != nil || len(body) > 512<<10 || json.Unmarshal(body, &input) != nil {
				ctx.Mock(http.StatusBadRequest, headers, `{"error":"invalid module request"}`)
				return true
			}
			script, revision, err := rewrite_liteapp_module(input.ModuleID, input.Source)
			if err != nil {
				response, _ := json.Marshal(map[string]string{"error": err.Error()})
				ctx.Mock(http.StatusUnprocessableEntity, headers, string(response))
				return true
			}
			module_path := liteapp_asset_path + "modules/" + revision + ".js"
			mutex.Lock()
			if _, exists := modules[module_path]; !exists && len(modules) >= 32 {
				mutex.Unlock()
				ctx.Mock(http.StatusTooManyRequests, headers, `{"error":"module revision limit reached; restart proxy"}`)
				return true
			}
			modules[module_path] = script
			mutex.Unlock()
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
			response, _ := json.Marshal(map[string]string{"url": "https://res.wx.qq.com" + module_path + "?v=" + revision, "revision": revision, "bootstrap": bootstrap})
			logger.Info().Str("module_id", input.ModuleID).Str("revision", revision).Msg("rewrote LiteApp business module")
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
		if ctx.Req.Method == http.MethodGet || ctx.Req.Method == http.MethodHead {
			mutex.Lock()
			script, exists := modules[pathname]
			mutex.Unlock()
			if exists {
				headers["Content-Type"] = "application/javascript; charset=utf-8"
				ctx.Mock(http.StatusOK, headers, script)
				return true
			}
		}
		ctx.Mock(http.StatusNotFound, headers, `{"error":"unknown LiteApp asset"}`)
		return true
	}
}
