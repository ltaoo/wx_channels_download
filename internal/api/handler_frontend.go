package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"wx_channel/frontend"
	result "wx_channel/internal/apiresult"
	"wx_channel/internal/config"
	"wx_channel/internal/logtime"
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
	Level        string     `json:"level"`
	Message      string     `json:"message"`
	Msg          string     `json:"msg"`
	Time         *time.Time `json:"time,omitempty"`
	End          int        `json:"end,omitempty"`
	Replace      int        `json:"replace,omitempty"`
	IgnorePrefix int        `json:"ignore_prefix,omitempty"`
	Prefix       *string    `json:"prefix,omitempty"`
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
	frontend_variables := map[string]any{
		"apiHost":                    config.APIClientHost(c.cfg.Hostname, c.cfg.Port),
		"apiOrigin":                  config.APIClientOrigin(c.cfg.Protocol, c.cfg.Hostname, c.cfg.Port),
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
	cfg_byte, _ := json.Marshal(frontend_variables)
	html := string(data)
	html = strings.ReplaceAll(html, "__WX_DOWNLOAD_CONFIG_JSON__", string(cfg_byte))
	html = strings.ReplaceAll(html, "__WX_DOWNLOAD_VERSION__", c.cfg.Version)
	return []byte(html)
}

func (c *APIClient) build_http_handler() http.Handler {
	frontendHandler := renderFrontendHTMLResponses(
		renderFrontendJSImportResponses(frontend.NewServer(c.cfg.Mode), c.cfg.Version),
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

// renderFrontendJSImportResponses adds the application version to local module
// imports. Like Vite's import analysis, this makes every module in the graph use
// a versioned URL instead of only versioning the entry module in index.html.
func renderFrontendJSImportResponses(next http.Handler, version string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer := &frontendJSImportResponseWriter{
			ResponseWriter: w,
			enabled:        r.Method == http.MethodGet && version != "",
		}
		next.ServeHTTP(writer, r)
		writer.flush(version)
	})
}

type frontendJSImportResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	wroteHeader  bool
	bufferScript bool
	enabled      bool
	body         bytes.Buffer
}

func (w *frontendJSImportResponseWriter) WriteHeader(statusCode int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.statusCode = statusCode
	w.bufferScript = w.enabled &&
		statusCode == http.StatusOK &&
		w.Header().Get("Content-Encoding") == "" &&
		isJavaScriptContentType(w.Header().Get("Content-Type"))
	if !w.bufferScript {
		w.ResponseWriter.WriteHeader(statusCode)
	}
}

func (w *frontendJSImportResponseWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.bufferScript {
		return w.body.Write(data)
	}
	return w.ResponseWriter.Write(data)
}

func (w *frontendJSImportResponseWriter) flush(version string) {
	if !w.bufferScript {
		return
	}
	body := versionJSImportSpecifiers(w.body.Bytes(), version)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.ResponseWriter.WriteHeader(w.statusCode)
	_, _ = w.ResponseWriter.Write(body)
}

func isJavaScriptContentType(contentType string) bool {
	if separator := strings.IndexByte(contentType, ';'); separator >= 0 {
		contentType = contentType[:separator]
	}
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "application/javascript", "text/javascript", "application/ecmascript", "text/ecmascript", "application/x-javascript":
		return true
	default:
		return false
	}
}

type jsTokenKind uint8

const (
	jsTokenEOF jsTokenKind = iota
	jsTokenIdentifier
	jsTokenString
	jsTokenPunctuation
	jsTokenOther
)

type jsToken struct {
	kind       jsTokenKind
	text       string
	start      int
	end        int
	valueStart int
	valueEnd   int
}

type jsScanner struct {
	source         []byte
	position       int
	canStartRegexp bool
}

type jsSpecifierRange struct {
	start int
	end   int
}

func versionJSImportSpecifiers(source []byte, version string) []byte {
	if version == "" {
		return source
	}

	ranges := findJSImportSpecifierRanges(source)
	if len(ranges) == 0 {
		return source
	}

	var output bytes.Buffer
	output.Grow(len(source) + len(ranges)*(len(version)+3))
	last := 0
	changed := false
	for _, item := range ranges {
		specifier := string(source[item.start:item.end])
		versioned, ok := versionJSImportSpecifier(specifier, version)
		if !ok {
			continue
		}
		output.Write(source[last:item.start])
		output.WriteString(versioned)
		last = item.end
		changed = true
	}
	if !changed {
		return source
	}
	output.Write(source[last:])
	return output.Bytes()
}

func findJSImportSpecifierRanges(source []byte) []jsSpecifierRange {
	scanner := jsScanner{source: source, canStartRegexp: true}
	var ranges []jsSpecifierRange
	for {
		token := scanner.next()
		if token.kind == jsTokenEOF {
			return ranges
		}
		if token.kind != jsTokenIdentifier {
			continue
		}

		var specifier jsToken
		var ok bool
		lookahead := scanner
		switch token.text {
		case "import":
			specifier, ok = findImportSpecifier(&lookahead)
		case "export":
			specifier, ok = findExportSpecifier(&lookahead)
		}
		if ok {
			if len(ranges) > 0 && ranges[len(ranges)-1].start == specifier.valueStart && ranges[len(ranges)-1].end == specifier.valueEnd {
				continue
			}
			ranges = append(ranges, jsSpecifierRange{
				start: specifier.valueStart,
				end:   specifier.valueEnd,
			})
		}
	}
}

func findImportSpecifier(scanner *jsScanner) (jsToken, bool) {
	token := scanner.next()
	if token.kind == jsTokenString { // import "./side-effect.js"
		return token, true
	}
	if token.kind == jsTokenPunctuation {
		switch token.text {
		case ".": // import.meta
			return jsToken{}, false
		case "(": // import("./lazy.js")
			token = scanner.next()
			return token, token.kind == jsTokenString
		}
	}

	braceDepth := 0
	for token.kind != jsTokenEOF {
		if token.kind == jsTokenPunctuation {
			switch token.text {
			case "{":
				braceDepth++
			case "}":
				if braceDepth > 0 {
					braceDepth--
				}
			case ";":
				if braceDepth == 0 {
					return jsToken{}, false
				}
			}
		}
		if braceDepth == 0 && token.kind == jsTokenIdentifier && token.text == "from" {
			token = scanner.next()
			return token, token.kind == jsTokenString
		}
		if braceDepth == 0 && token.kind == jsTokenIdentifier && (token.text == "import" || token.text == "export") {
			return jsToken{}, false
		}
		token = scanner.next()
	}
	return jsToken{}, false
}

func findExportSpecifier(scanner *jsScanner) (jsToken, bool) {
	braceDepth := 0
	for token := scanner.next(); token.kind != jsTokenEOF; token = scanner.next() {
		if token.kind == jsTokenPunctuation {
			switch token.text {
			case "{":
				braceDepth++
			case "}":
				if braceDepth > 0 {
					braceDepth--
				}
			case ";":
				if braceDepth == 0 {
					return jsToken{}, false
				}
			}
		}
		if braceDepth == 0 && token.kind == jsTokenIdentifier && token.text == "from" {
			token = scanner.next()
			return token, token.kind == jsTokenString
		}
		if braceDepth == 0 && token.kind == jsTokenIdentifier && (token.text == "import" || token.text == "export") {
			return jsToken{}, false
		}
	}
	return jsToken{}, false
}

func (scanner *jsScanner) next() jsToken {
	for scanner.position < len(scanner.source) {
		start := scanner.position
		current := scanner.source[start]

		if isJSSpace(current) {
			scanner.position++
			continue
		}
		if current == '/' && start+1 < len(scanner.source) {
			switch scanner.source[start+1] {
			case '/':
				scanner.position = skipJSLineComment(scanner.source, start+2)
				continue
			case '*':
				scanner.position = skipJSBlockComment(scanner.source, start+2)
				continue
			}
			if scanner.canStartRegexp {
				scanner.position = skipJSRegexp(scanner.source, start+1)
				scanner.canStartRegexp = false
				return jsToken{kind: jsTokenOther, start: start, end: scanner.position}
			}
		}
		if current == '\'' || current == '"' {
			end := skipJSQuotedString(scanner.source, start, current)
			scanner.position = end
			scanner.canStartRegexp = false
			valueEnd := end
			if valueEnd > start+1 && scanner.source[valueEnd-1] == current {
				valueEnd--
			}
			return jsToken{
				kind:       jsTokenString,
				start:      start,
				end:        end,
				valueStart: start + 1,
				valueEnd:   valueEnd,
			}
		}
		if current == '`' {
			scanner.position = skipJSTemplate(scanner.source, start+1)
			scanner.canStartRegexp = false
			return jsToken{kind: jsTokenOther, start: start, end: scanner.position}
		}
		if isJSIdentifierStart(current) {
			scanner.position++
			for scanner.position < len(scanner.source) && isJSIdentifierContinue(scanner.source[scanner.position]) {
				scanner.position++
			}
			text := string(scanner.source[start:scanner.position])
			scanner.canStartRegexp = jsKeywordAllowsRegexp(text)
			return jsToken{kind: jsTokenIdentifier, text: text, start: start, end: scanner.position}
		}
		if current >= '0' && current <= '9' {
			scanner.position++
			for scanner.position < len(scanner.source) && isJSNumberContinue(scanner.source[scanner.position]) {
				scanner.position++
			}
			scanner.canStartRegexp = false
			return jsToken{kind: jsTokenOther, start: start, end: scanner.position}
		}

		scanner.position++
		text := string(scanner.source[start:scanner.position])
		scanner.canStartRegexp = current != ')' && current != ']' && current != '}' && current != '.'
		return jsToken{kind: jsTokenPunctuation, text: text, start: start, end: scanner.position}
	}
	return jsToken{kind: jsTokenEOF, start: len(scanner.source), end: len(scanner.source)}
}

func versionJSImportSpecifier(specifier string, version string) (string, bool) {
	if strings.ContainsRune(specifier, '\\') || !isLocalJSImportSpecifier(specifier) {
		return specifier, false
	}
	parsed, err := url.Parse(specifier)
	if err != nil {
		return specifier, false
	}
	extension := strings.ToLower(parsed.Path)
	if !strings.HasSuffix(extension, ".js") && !strings.HasSuffix(extension, ".mjs") {
		return specifier, false
	}
	query := parsed.Query()
	query.Set("v", version)
	parsed.RawQuery = query.Encode()
	return parsed.String(), true
}

func isLocalJSImportSpecifier(specifier string) bool {
	return strings.HasPrefix(specifier, "./") ||
		strings.HasPrefix(specifier, "../") ||
		strings.HasPrefix(specifier, "/") && !strings.HasPrefix(specifier, "//") ||
		strings.HasPrefix(specifier, "@/")
}

func skipJSQuotedString(source []byte, start int, quote byte) int {
	for position := start + 1; position < len(source); position++ {
		if source[position] == '\\' {
			position++
			continue
		}
		if source[position] == quote {
			return position + 1
		}
	}
	return len(source)
}

func skipJSTemplate(source []byte, position int) int {
	for position < len(source) {
		if source[position] == '\\' {
			position += 2
			continue
		}
		if source[position] == '`' {
			return position + 1
		}
		position++
	}
	return len(source)
}

func skipJSLineComment(source []byte, position int) int {
	for position < len(source) && source[position] != '\n' && source[position] != '\r' {
		position++
	}
	return position
}

func skipJSBlockComment(source []byte, position int) int {
	for position+1 < len(source) {
		if source[position] == '*' && source[position+1] == '/' {
			return position + 2
		}
		position++
	}
	return len(source)
}

func skipJSRegexp(source []byte, position int) int {
	inCharacterClass := false
	for position < len(source) {
		switch source[position] {
		case '\\':
			position += 2
			continue
		case '[':
			inCharacterClass = true
		case ']':
			inCharacterClass = false
		case '/':
			if !inCharacterClass {
				position++
				for position < len(source) && isJSIdentifierContinue(source[position]) {
					position++
				}
				return position
			}
		case '\n', '\r':
			return position
		}
		position++
	}
	return len(source)
}

func isJSSpace(value byte) bool {
	switch value {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	default:
		return false
	}
}

func isJSIdentifierStart(value byte) bool {
	return value == '_' || value == '$' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func isJSIdentifierContinue(value byte) bool {
	return isJSIdentifierStart(value) || value >= '0' && value <= '9'
}

func isJSNumberContinue(value byte) bool {
	return isJSIdentifierContinue(value) || value == '.'
}

func jsKeywordAllowsRegexp(keyword string) bool {
	switch keyword {
	case "await", "case", "delete", "do", "else", "in", "instanceof", "new", "return", "throw", "typeof", "void", "yield":
		return true
	default:
		return false
	}
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

	report_bodies := []json.RawMessage{body}
	if trimmed_body := bytes.TrimSpace(body); len(trimmed_body) > 0 && trimmed_body[0] == '[' {
		if err := json.Unmarshal(trimmed_body, &report_bodies); err != nil {
			result.Err(ctx, 400, err.Error())
			return
		}
	}

	type frontend_report_payload struct {
		data         FrontendReport
		extra_fields map[string]interface{}
	}
	reports := make([]frontend_report_payload, 0, len(report_bodies))
	for _, report_body := range report_bodies {
		var report frontend_report_payload
		if err := json.Unmarshal(report_body, &report.data); err != nil {
			result.Err(ctx, 400, err.Error())
			return
		}
		if err := json.Unmarshal(report_body, &report.extra_fields); err != nil {
			result.Err(ctx, 400, err.Error())
			return
		}
		reports = append(reports, report)
	}

	for _, report := range reports {
		data := report.data
		extra_fields := report.extra_fields
		// Write to log file -- parse all fields to support arbitrary key=value passed by the frontend fluent logger
		delete(extra_fields, "level")
		delete(extra_fields, "msg")
		delete(extra_fields, "message")
		delete(extra_fields, "end")
		delete(extra_fields, "replace")
		delete(extra_fields, "ignore_prefix")
		delete(extra_fields, "prefix")
		delete(extra_fields, "component")
		delete(extra_fields, "time")

		report_message := data.Message
		if report_message == "" {
			report_message = data.Msg
		}
		if report_message == "" {
			report_message = "frontend report"
		}
		evt := c.logger.WithLevel(zerologLevel(data.Level)).
			Str("component", "frontend")
		if data.Time != nil {
			evt.Ctx(logtime.WithTimestamp(ctx.Request.Context(), data.Time.In(time.Local)))
		}
		for key, value := range extra_fields {
			evt = evt.Interface(key, normalizeFrontendReportValue(value))
		}
		evt.Msg(report_message)

		// Terminal display
		if data.Level == "error" {
			color.Red(fmt.Sprintf("[FRONTEND ERROR]%s\n", report_message))
			continue
		}
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
