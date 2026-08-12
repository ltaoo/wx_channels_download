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
	result "wx_channel/internal/apiresult"
	scraper_wxchannels "wx_channel/pkg/scraper/wxchannels"
)

func (c *APIClient) handle_index(ctx *gin.Context) {
	c.handle_download_page(ctx)
}

func (c *APIClient) handle_download_page(ctx *gin.Context) {
	c.renderFrontendFile(ctx, "inject/index.html")
}

func (c *APIClient) handle_home_page(ctx *gin.Context) {
	c.renderFrontendFile(ctx, "inject/home.html")
}

func (c *APIClient) handle_content_page(ctx *gin.Context) {
	c.renderFrontendFile(ctx, "inject/content.html")
}

func (c *APIClient) handle_content_detail_page(ctx *gin.Context) {
	c.renderFrontendFile(ctx, "inject/content_detail.html")
}

func (c *APIClient) handle_browse_history_page(ctx *gin.Context) {
	c.renderFrontendFile(ctx, "inject/browsehistory.html")
}

func (c *APIClient) handle_account_page(ctx *gin.Context) {
	c.renderFrontendFile(ctx, "inject/account.html")
}

func (c *APIClient) handle_logs_page(ctx *gin.Context) {
	c.renderFrontendFile(ctx, "inject/logs.html")
}

func (c *APIClient) handle_channels_page(ctx *gin.Context) {
	log.Println("[ROUTE] handle_channels_page called, rendering channels.html")
	c.renderFrontendFile(ctx, "inject/channels.html")
}

// TODO: requires velo/fileserver
// func (c *APIClient) handle_migration_page(ctx *gin.Context) {
// 	c.renderFrontendFile(ctx, "migration.html")
// }

func (c *APIClient) renderFrontendFile(ctx *gin.Context, name string) {
	data, err := frontend.Assets().ReadRoot(name)
	if err != nil {
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}
	max_running := c.cfg.Original.GetInt("download.maxRunning")
	if max_running == 0 {
		max_running = 3
	}
	frontendVariables := map[string]any{
		"apiHost":                    fmt.Sprintf("%s:%d", c.cfg.Hostname, c.cfg.Port),
		"apiOrigin":                  fmt.Sprintf("%s://%s:%d", c.cfg.Protocol, c.cfg.Hostname, c.cfg.Port),
		"apiProtocol":                c.cfg.Protocol,
		"remoteServerEnabled":        c.cfg.Original.GetBool("download.remoteServer.enabled"),
		"remoteServerOrigin":         fmt.Sprintf("%s://%s:%d", c.cfg.RemoteServerProtocol, c.cfg.RemoteServerHostname, c.cfg.RemoteServerPort),
		"maxRunning":                 max_running,
		"downloadFilenameTemplate":   c.cfg.Original.GetString("download.filenameTemplate"),
		"defaultHighest":             c.cfg.Original.GetBool("channels.download.defaultHighest") || c.cfg.Original.GetBool("download.defaultHighest"),
		"downloadPauseWhenDownload":  c.cfg.Original.GetBool("channels.download.pauseWhenDownload"),
		"downloadInFrontend":         c.cfg.Original.GetBool("channels.download.frontend"),
		"downloadForceCheckAllFeeds": c.cfg.Original.GetBool("channels.download.forceCheckAllFeeds"),
	}
	cfgByte, _ := json.Marshal(frontendVariables)
	html := string(data)
	html = strings.ReplaceAll(html, "__WX_DOWNLOAD_CONFIG_JSON__", string(cfgByte))
	html = strings.ReplaceAll(html, "__WX_DOWNLOAD_VERSION__", "local")

	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, html)
}

func (c *APIClient) build_http_handler() http.Handler {
	frontendHandler := frontend.NewServer(c.cfg.Mode)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldServeByAPI(r.URL.Path) {
			c.engine.ServeHTTP(w, r)
			return
		}
		frontendHandler.ServeHTTP(w, r)
	})
}

// handleFrontendTip handles frontend log/tip messages posted from injected pages.
func (c *APIClient) handleFrontendTip(ctx *gin.Context) {
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		result.Err(ctx, 400, "read body failed")
		return
	}
	var data scraper_wxchannels.FrontendTip
	if err := json.Unmarshal(body, &data); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	prefixText := "[FRONTEND]"
	prefix := data.Prefix
	if prefix == nil {
		prefix = &prefixText
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

// handleFrontendError handles frontend error messages posted from injected pages.
func (c *APIClient) handleFrontendError(ctx *gin.Context) {
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		result.Err(ctx, 400, "read body failed")
		return
	}
	var data scraper_wxchannels.FrontendErrorTip
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

	var data scraper_wxchannels.FrontendReport
	if err := json.Unmarshal(body, &data); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	var extraFields map[string]interface{}
	if err := json.Unmarshal(body, &extraFields); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}

	// Write to log file -- parse all fields to support arbitrary key=value passed by the frontend fluent logger
	delete(extraFields, "level")
	delete(extraFields, "msg")
	delete(extraFields, "message")
	delete(extraFields, "end")
	delete(extraFields, "replace")
	delete(extraFields, "ignore_prefix")
	delete(extraFields, "prefix")

	reportMessage := data.Message
	if reportMessage == "" {
		reportMessage = data.Msg
	}
	if reportMessage == "" {
		reportMessage = "frontend report"
	}
	evt := c.logger.WithLevel(zerologLevel(data.Level)).
		Str("source", "frontend")
	for k, v := range extraFields {
		evt = evt.Interface(k, normalizeFrontendReportValue(v))
	}
	evt.Msg(reportMessage)

	// Terminal display
	if data.Level == "error" {
		color.Red(fmt.Sprintf("[FRONTEND ERROR]%s\n", reportMessage))
	} else {
		prefixText := "[FRONTEND]"
		prefix := data.Prefix
		if prefix == nil {
			prefix = &prefixText
		}
		if data.End == 1 {
			fmt.Println()
		} else if data.Replace == 1 {
			fmt.Printf("\r\033[K%v%s", *prefix, reportMessage)
		} else if data.IgnorePrefix == 1 {
			fmt.Printf("%s\n", reportMessage)
		} else {
			fmt.Printf("%v%s\n", *prefix, reportMessage)
		}
	}
	result.Ok(ctx, nil)
}

func normalizeFrontendReportValue(v interface{}) interface{} {
	switch value := v.(type) {
	case string:
		var parsed interface{}
		if err := json.Unmarshal([]byte(value), &parsed); err == nil {
			return normalizeFrontendReportValue(parsed)
		}
		return value
	case map[string]interface{}:
		normalized := make(map[string]interface{}, len(value))
		for k, child := range value {
			normalized[k] = normalizeFrontendReportValue(child)
		}
		return normalized
	case []interface{}:
		normalized := make([]interface{}, len(value))
		for i, child := range value {
			normalized[i] = normalizeFrontendReportValue(child)
		}
		return normalized
	default:
		return value
	}
}

func zerologLevel(level string) zerolog.Level {
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

func shouldServeByAPI(path string) bool {
	if path == "/" ||
		path == "/favicon.ico" ||
		path == "/home" ||
		path == "/filehelper" ||
		path == "/play" ||
		path == "/preview" ||
		path == "/content" ||
		path == "/content/detail" ||
		path == "/browsehistory" ||
		path == "/account" ||
		path == "/logs" ||
		path == "/channels" ||
		path == "/migration" ||
		path == "/admin" ||
		path == "/influencers" ||
		path == "/report" {
		return true
	}

	apiPrefixes := []string{
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
	for _, prefix := range apiPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
