package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/fatih/color"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"wx_channel/frontend"
	result "wx_channel/internal/util"
	"wx_channel/pkg/scraper/wxchannels"
)

func (c *APIClient) handle_index(ctx *gin.Context) {
	c.handle_download_page(ctx)
}

func (c *APIClient) handle_download_page(ctx *gin.Context) {
	c.render_frontend_file(ctx, "inject/index.html")
}

func (c *APIClient) handle_content_page(ctx *gin.Context) {
	c.render_frontend_file(ctx, "inject/content.html")
}

func (c *APIClient) handle_browse_history_page(ctx *gin.Context) {
	c.render_frontend_file(ctx, "inject/browsehistory.html")
}

func (c *APIClient) handle_channels_page(ctx *gin.Context) {
	log.Println("[ROUTE] handle_channels_page called, rendering channels.html")
	c.render_frontend_file(ctx, "inject/channels.html")
}

// TODO: requires velo/fileserver
// func (c *APIClient) handle_migration_page(ctx *gin.Context) {
// 	c.render_frontend_file(ctx, "migration.html")
// }

func (c *APIClient) render_frontend_file(ctx *gin.Context, name string) {
	data, err := frontend.Assets().ReadRoot(name)
	if err != nil {
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}
	cfg_byte, _ := json.Marshal(c.cfg)
	html := string(data)
	html = strings.ReplaceAll(html, "__WX_DOWNLOAD_CONFIG_JSON__", string(cfg_byte))
	html = strings.ReplaceAll(html, "__WX_DOWNLOAD_VERSION__", "local")

	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, html)
}

func (c *APIClient) build_http_handler() http.Handler {
	frontend_handler := frontend.NewServer(c.cfg.Mode)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if should_serve_by_api(r.URL.Path) {
			c.engine.ServeHTTP(w, r)
			return
		}
		frontend_handler.ServeHTTP(w, r)
	})
}

// handle_frontend_tip handles frontend log/tip messages posted from injected pages.
func (c *APIClient) handle_frontend_tip(ctx *gin.Context) {
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		result.Err(ctx, 400, "read body failed")
		return
	}
	var data wxchannels.FrontendTip
	if err := json.Unmarshal(body, &data); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	prefix_text := "[FRONTEND]"
	prefix := data.Prefix
	if prefix == nil {
		prefix = &prefix_text
	}
	if data.End == 1 {
		fmt.Println()
	} else if data.Replace == 1 {
		fmt.Printf("\r\033[K%v%s", *prefix, data.Msg)
	} else if data.IgnorePrefix == 1 {
		fmt.Printf("%s\n", data.Msg)
	} else {
		fmt.Printf("%v%s\n", *prefix, data.Msg)
	}
	result.Ok(ctx, nil)
}

// handle_frontend_error handles frontend error messages posted from injected pages.
func (c *APIClient) handle_frontend_error(ctx *gin.Context) {
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		result.Err(ctx, 400, "read body failed")
		return
	}
	var data wxchannels.FrontendErrorTip
	if err := json.Unmarshal(body, &data); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	color.Red(fmt.Sprintf("[FRONTEND ERROR]%s\n", data.Msg))
	result.Ok(ctx, nil)
}

// handle_frontend_report handles unified frontend reports, level is "info" or "error".
func (c *APIClient) handle_frontend_report(ctx *gin.Context) {
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		result.Err(ctx, 400, "read body failed")
		return
	}

	var data wxchannels.FrontendReport
	if err := json.Unmarshal(body, &data); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	var extra_fields map[string]interface{}
	if err := json.Unmarshal(body, &extra_fields); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}

	// Write to log file -- parse all fields to support arbitrary key=value passed by the frontend fluent logger
	delete(extra_fields, "level")
	delete(extra_fields, "msg")
	delete(extra_fields, "message")
	delete(extra_fields, "end")
	delete(extra_fields, "replace")
	delete(extra_fields, "ignore_prefix")
	delete(extra_fields, "prefix")

	report_message := data.Message
	if report_message == "" {
		report_message = data.Msg
	}
	if report_message == "" {
		report_message = "frontend report"
	}
	evt := c.logger.WithLevel(zerolog_level(data.Level)).
		Str("source", "frontend")
	for k, v := range extra_fields {
		evt = evt.Interface(k, normalize_frontend_report_value(v))
	}
	evt.Msg(report_message)

	// Terminal display
	if data.Level == "error" {
		color.Red(fmt.Sprintf("[FRONTEND ERROR]%s\n", report_message))
	} else {
		prefix_text := "[FRONTEND]"
		prefix := data.Prefix
		if prefix == nil {
			prefix = &prefix_text
		}
		if data.End == 1 {
			fmt.Println()
		} else if data.Replace == 1 {
			fmt.Printf("\r\033[K%v%s", *prefix, report_message)
		} else if data.IgnorePrefix == 1 {
			fmt.Printf("%s\n", report_message)
		} else {
			fmt.Printf("%v%s\n", *prefix, report_message)
		}
	}
	result.Ok(ctx, nil)
}

func normalize_frontend_report_value(v interface{}) interface{} {
	switch value := v.(type) {
	case string:
		var parsed interface{}
		if err := json.Unmarshal([]byte(value), &parsed); err == nil {
			return normalize_frontend_report_value(parsed)
		}
		return value
	case map[string]interface{}:
		normalized := make(map[string]interface{}, len(value))
		for k, child := range value {
			normalized[k] = normalize_frontend_report_value(child)
		}
		return normalized
	case []interface{}:
		normalized := make([]interface{}, len(value))
		for i, child := range value {
			normalized[i] = normalize_frontend_report_value(child)
		}
		return normalized
	default:
		return value
	}
}

func zerolog_level(level string) zerolog.Level {
	switch level {
	case "error":
		return zerolog.ErrorLevel
	case "warn":
		return zerolog.WarnLevel
	case "debug":
		return zerolog.DebugLevel
	default:
		return zerolog.InfoLevel
	}
}

func should_serve_by_api(path string) bool {
	if path == "/" ||
		path == "/favicon.ico" ||
		path == "/filehelper" ||
		path == "/play" ||
		path == "/file" ||
		path == "/preview" ||
		path == "/content" ||
		path == "/browsehistory" ||
		path == "/channels" ||
		path == "/migration" ||
		path == "/admin" ||
		path == "/influencers" ||
		path == "/report" {
		return true
	}

	api_prefixes := []string{
		"/api/",
		"/ws/",
		"/rss/",
		"/mp/",
		"/browse_history/",
		"/influencers/",
		"/account/",
		"/video/",
		"/channels/",
		"/xiaohongshu/",
		"/bilibili/",
		"/douban/",
		"/instagram/",
		"/weibo/",
		"/__assets/",
	}
	for _, prefix := range api_prefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
