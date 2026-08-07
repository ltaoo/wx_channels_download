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
	scraper_wxchannels "wx_channel/pkg/scraper/wxchannels"
)

func (c *APIClient) handleIndex(ctx *gin.Context) {
	c.handleDownloadPage(ctx)
}

func (c *APIClient) handleDownloadPage(ctx *gin.Context) {
	c.renderFrontendFile(ctx, "inject/index.html")
}

func (c *APIClient) handleHomePage(ctx *gin.Context) {
	c.renderFrontendFile(ctx, "inject/home.html")
}

func (c *APIClient) handleContentPage(ctx *gin.Context) {
	c.renderFrontendFile(ctx, "inject/content.html")
}

func (c *APIClient) handleBrowseHistoryPage(ctx *gin.Context) {
	c.renderFrontendFile(ctx, "inject/browsehistory.html")
}

func (c *APIClient) handleChannelsPage(ctx *gin.Context) {
	log.Println("[ROUTE] handleChannelsPage called, rendering channels.html")
	c.renderFrontendFile(ctx, "inject/channels.html")
}

// TODO: requires velo/fileserver
// func (c *APIClient) handleMigrationPage(ctx *gin.Context) {
// 	c.renderFrontendFile(ctx, "migration.html")
// }

func (c *APIClient) renderFrontendFile(ctx *gin.Context, name string) {
	data, err := frontend.Assets().ReadRoot(name)
	if err != nil {
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}
	cfgByte, _ := json.Marshal(c.cfg)
	html := string(data)
	html = strings.ReplaceAll(html, "__WX_DOWNLOAD_CONFIG_JSON__", string(cfgByte))
	html = strings.ReplaceAll(html, "__WX_DOWNLOAD_VERSION__", "local")

	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, html)
}

func (c *APIClient) buildHTTPHandler() http.Handler {
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

// handleFrontendReport handles unified frontend reports, level is "info" or "error".
func (c *APIClient) handleFrontendReport(ctx *gin.Context) {
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
