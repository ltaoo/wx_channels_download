package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"wx_channel/frontend"
	result "wx_channel/internal/apiresult"
)

type FrontendTip struct {
	End          int     `json:"end"`
	Replace      int     `json:"replace"`
	IgnorePrefix int     `json:"ignore_prefix"`
	Prefix       *string `json:"prefix"`
	Msg          string  `json:"msg"`
}

type FrontendErrorTip struct {
	Alert int    `json:"alert"`
	Msg   string `json:"msg"`
}

// FrontendReport is a unified frontend report, level is "info" or "error"
type FrontendReport struct {
	Level        string  `json:"level"`
	Message      string  `json:"message"`
	Msg          string  `json:"msg"`
	End          int     `json:"end,omitempty"`
	Replace      int     `json:"replace,omitempty"`
	IgnorePrefix int     `json:"ignore_prefix,omitempty"`
	Prefix       *string `json:"prefix,omitempty"`
}

func (c *APIClient) handle_index(ctx *gin.Context) {
	c.renderFrontendFile(ctx, "index.html")
}

func (c *APIClient) renderFrontendFile(ctx *gin.Context, name string) {
	data, err := frontend.Assets().ReadRoot(name)
	if err != nil {
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}
	html := c.renderFrontendHTML(data)

	setFrontendHTMLCacheHeaders(ctx.Writer.Header())
	ctx.Data(http.StatusOK, "text/html; charset=utf-8", html)
}

func (c *APIClient) renderFrontendHTML(data []byte) []byte {
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
	html = strings.ReplaceAll(html, "__WX_DOWNLOAD_VERSION__", c.cfg.Version)
	return []byte(html)
}

func (c *APIClient) build_http_handler() http.Handler {
	frontendHandler := renderFrontendHTMLResponses(
		frontend.NewServer(c.cfg.Mode),
		c.renderFrontendHTML,
	)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldServeByAPI(r.URL.Path) {
			c.engine.ServeHTTP(w, r)
			return
		}
		frontendHandler.ServeHTTP(w, r)
	})
}

// renderFrontendHTMLResponses applies the same runtime-variable rendering used
// by the root route to index.html responses produced by the SPA fallback.
func renderFrontendHTMLResponses(next http.Handler, render func([]byte) []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer := &frontendHTMLResponseWriter{ResponseWriter: w}
		next.ServeHTTP(writer, r)
		writer.flush(render)
	})
}

type frontendHTMLResponseWriter struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
	bufferHTML  bool
	body        bytes.Buffer
}

func (w *frontendHTMLResponseWriter) WriteHeader(statusCode int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.statusCode = statusCode
	w.bufferHTML = statusCode >= http.StatusOK &&
		statusCode != http.StatusNoContent &&
		statusCode != http.StatusNotModified &&
		strings.HasPrefix(strings.ToLower(w.Header().Get("Content-Type")), "text/html")
	if !w.bufferHTML {
		w.ResponseWriter.WriteHeader(statusCode)
	}
}

func (w *frontendHTMLResponseWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.bufferHTML {
		return w.body.Write(data)
	}
	return w.ResponseWriter.Write(data)
}

func (w *frontendHTMLResponseWriter) flush(render func([]byte) []byte) {
	if !w.bufferHTML {
		return
	}
	body := render(w.body.Bytes())
	setFrontendHTMLCacheHeaders(w.Header())
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.ResponseWriter.WriteHeader(w.statusCode)
	_, _ = w.ResponseWriter.Write(body)
}

func setFrontendHTMLCacheHeaders(header http.Header) {
	header.Set("Cache-Control", "no-store")
	header.Set("Pragma", "no-cache")
	header.Set("Expires", "0")
}

// handleFrontendTip handles frontend log/tip messages posted from injected pages.
func (c *APIClient) handleFrontendTip(ctx *gin.Context) {
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		result.Err(ctx, 400, "read body failed")
		return
	}
	var data FrontendTip
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
	var data FrontendErrorTip
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

	var data FrontendReport
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
	delete(extraFields, "component")

	reportMessage := data.Message
	if reportMessage == "" {
		reportMessage = data.Msg
	}
	if reportMessage == "" {
		reportMessage = "frontend report"
	}
	evt := c.logger.WithLevel(zerologLevel(data.Level)).
		Str("component", "frontend")
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
	if path == "/favicon.ico" || path == "/" || path == "/play" || path == "/report" || path == "/mcp" {
		return true
	}

	apiPrefixes := []string{
		"/api/",
		"/ws/",
		"/rss/",
		"/mp/",
		"/__assets/",
	}
	for _, prefix := range apiPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
