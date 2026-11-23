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
	c.render_frontend_file(ctx, "index.html")
}

func (c *APIClient) render_frontend_file(ctx *gin.Context, name string) {
	data, err := frontend.Assets().ReadRoot(name)
	if err != nil {
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}
	html := c.render_frontend_html(data)

	set_frontend_html_cache_headers(ctx.Writer.Header())
	ctx.Data(http.StatusOK, "text/html; charset=utf-8", html)
}

func (c *APIClient) render_frontend_html(data []byte) []byte {
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
	frontend_handler := render_frontend_html_responses(
		render_frontend_js_import_responses(frontend.NewServer(c.cfg.Mode), c.cfg.Version),
		c.render_frontend_html,
	)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if should_serve_by_api(r.URL.Path) {
			c.engine.ServeHTTP(w, r)
			return
		}
		frontend_handler.ServeHTTP(w, r)
	})
}

// render_frontend_js_import_responses adds the application version to local module
// imports. Like Vite's import analysis, this makes every module in the graph use
// a versioned URL instead of only versioning the entry module in index.html.
func render_frontend_js_import_responses(next http.Handler, version string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer := &frontend_js_import_response_writer{
			ResponseWriter: w,
			enabled:        r.Method == http.MethodGet && version != "",
		}
		next.ServeHTTP(writer, r)
		writer.flush(version)
	})
}

type frontend_js_import_response_writer struct {
	http.ResponseWriter
	status_code   int
	wrote_header  bool
	buffer_script bool
	enabled       bool
	body          bytes.Buffer
}

func (w *frontend_js_import_response_writer) WriteHeader(status_code int) {
	if w.wrote_header {
		return
	}
	w.wrote_header = true
	w.status_code = status_code
	w.buffer_script = w.enabled &&
		status_code == http.StatusOK &&
		w.Header().Get("Content-Encoding") == "" &&
		is_java_script_content_type(w.Header().Get("Content-Type"))
	if !w.buffer_script {
		w.ResponseWriter.WriteHeader(status_code)
	}
}

func (w *frontend_js_import_response_writer) Write(data []byte) (int, error) {
	if !w.wrote_header {
		w.WriteHeader(http.StatusOK)
	}
	if w.buffer_script {
		return w.body.Write(data)
	}
	return w.ResponseWriter.Write(data)
}

func (w *frontend_js_import_response_writer) flush(version string) {
	if !w.buffer_script {
		return
	}
	body := version_js_import_specifiers(w.body.Bytes(), version)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.ResponseWriter.WriteHeader(w.status_code)
	_, _ = w.ResponseWriter.Write(body)
}

func is_java_script_content_type(content_type string) bool {
	if separator := strings.IndexByte(content_type, ';'); separator >= 0 {
		content_type = content_type[:separator]
	}
	switch strings.ToLower(strings.TrimSpace(content_type)) {
	case "application/javascript", "text/javascript", "application/ecmascript", "text/ecmascript", "application/x-javascript":
		return true
	default:
		return false
	}
}

type js_token_kind uint8

const (
	js_token_eof js_token_kind = iota
	js_token_identifier
	js_token_string
	js_token_punctuation
	js_token_other
)

type js_token struct {
	kind        js_token_kind
	text        string
	start       int
	end         int
	value_start int
	value_end   int
}

type js_scanner struct {
	source           []byte
	position         int
	can_start_regexp bool
}

type js_specifier_range struct {
	start int
	end   int
}

func version_js_import_specifiers(source []byte, version string) []byte {
	if version == "" {
		return source
	}

	ranges := find_js_import_specifier_ranges(source)
	if len(ranges) == 0 {
		return source
	}

	var output bytes.Buffer
	output.Grow(len(source) + len(ranges)*(len(version)+3))
	last := 0
	changed := false
	for _, item := range ranges {
		specifier := string(source[item.start:item.end])
		versioned, ok := version_js_import_specifier(specifier, version)
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

func find_js_import_specifier_ranges(source []byte) []js_specifier_range {
	scanner := js_scanner{source: source, can_start_regexp: true}
	var ranges []js_specifier_range
	for {
		token := scanner.next()
		if token.kind == js_token_eof {
			return ranges
		}
		if token.kind != js_token_identifier {
			continue
		}

		var specifier js_token
		var ok bool
		lookahead := scanner
		switch token.text {
		case "import":
			specifier, ok = find_import_specifier(&lookahead)
		case "export":
			specifier, ok = find_export_specifier(&lookahead)
		}
		if ok {
			if len(ranges) > 0 && ranges[len(ranges)-1].start == specifier.value_start && ranges[len(ranges)-1].end == specifier.value_end {
				continue
			}
			ranges = append(ranges, js_specifier_range{
				start: specifier.value_start,
				end:   specifier.value_end,
			})
		}
	}
}

func find_import_specifier(scanner *js_scanner) (js_token, bool) {
	token := scanner.next()
	if token.kind == js_token_string { // import "./side-effect.js"
		return token, true
	}
	if token.kind == js_token_punctuation {
		switch token.text {
		case ".": // import.meta
			return js_token{}, false
		case "(": // import("./lazy.js")
			token = scanner.next()
			return token, token.kind == js_token_string
		}
	}

	brace_depth := 0
	for token.kind != js_token_eof {
		if token.kind == js_token_punctuation {
			switch token.text {
			case "{":
				brace_depth++
			case "}":
				if brace_depth > 0 {
					brace_depth--
				}
			case ";":
				if brace_depth == 0 {
					return js_token{}, false
				}
			}
		}
		if brace_depth == 0 && token.kind == js_token_identifier && token.text == "from" {
			token = scanner.next()
			return token, token.kind == js_token_string
		}
		if brace_depth == 0 && token.kind == js_token_identifier && (token.text == "import" || token.text == "export") {
			return js_token{}, false
		}
		token = scanner.next()
	}
	return js_token{}, false
}

func find_export_specifier(scanner *js_scanner) (js_token, bool) {
	brace_depth := 0
	for token := scanner.next(); token.kind != js_token_eof; token = scanner.next() {
		if token.kind == js_token_punctuation {
			switch token.text {
			case "{":
				brace_depth++
			case "}":
				if brace_depth > 0 {
					brace_depth--
				}
			case ";":
				if brace_depth == 0 {
					return js_token{}, false
				}
			}
		}
		if brace_depth == 0 && token.kind == js_token_identifier && token.text == "from" {
			token = scanner.next()
			return token, token.kind == js_token_string
		}
		if brace_depth == 0 && token.kind == js_token_identifier && (token.text == "import" || token.text == "export") {
			return js_token{}, false
		}
	}
	return js_token{}, false
}

func (scanner *js_scanner) next() js_token {
	for scanner.position < len(scanner.source) {
		start := scanner.position
		current := scanner.source[start]

		if is_js_space(current) {
			scanner.position++
			continue
		}
		if current == '/' && start+1 < len(scanner.source) {
			switch scanner.source[start+1] {
			case '/':
				scanner.position = skip_js_line_comment(scanner.source, start+2)
				continue
			case '*':
				scanner.position = skip_js_block_comment(scanner.source, start+2)
				continue
			}
			if scanner.can_start_regexp {
				scanner.position = skip_js_regexp(scanner.source, start+1)
				scanner.can_start_regexp = false
				return js_token{kind: js_token_other, start: start, end: scanner.position}
			}
		}
		if current == '\'' || current == '"' {
			end := skip_js_quoted_string(scanner.source, start, current)
			scanner.position = end
			scanner.can_start_regexp = false
			value_end := end
			if value_end > start+1 && scanner.source[value_end-1] == current {
				value_end--
			}
			return js_token{
				kind:        js_token_string,
				start:       start,
				end:         end,
				value_start: start + 1,
				value_end:   value_end,
			}
		}
		if current == '`' {
			scanner.position = skip_js_template(scanner.source, start+1)
			scanner.can_start_regexp = false
			return js_token{kind: js_token_other, start: start, end: scanner.position}
		}
		if is_js_identifier_start(current) {
			scanner.position++
			for scanner.position < len(scanner.source) && is_js_identifier_continue(scanner.source[scanner.position]) {
				scanner.position++
			}
			text := string(scanner.source[start:scanner.position])
			scanner.can_start_regexp = js_keyword_allows_regexp(text)
			return js_token{kind: js_token_identifier, text: text, start: start, end: scanner.position}
		}
		if current >= '0' && current <= '9' {
			scanner.position++
			for scanner.position < len(scanner.source) && is_js_number_continue(scanner.source[scanner.position]) {
				scanner.position++
			}
			scanner.can_start_regexp = false
			return js_token{kind: js_token_other, start: start, end: scanner.position}
		}

		scanner.position++
		text := string(scanner.source[start:scanner.position])
		scanner.can_start_regexp = current != ')' && current != ']' && current != '}' && current != '.'
		return js_token{kind: js_token_punctuation, text: text, start: start, end: scanner.position}
	}
	return js_token{kind: js_token_eof, start: len(scanner.source), end: len(scanner.source)}
}

func version_js_import_specifier(specifier string, version string) (string, bool) {
	if strings.ContainsRune(specifier, '\\') || !is_local_js_import_specifier(specifier) {
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

func is_local_js_import_specifier(specifier string) bool {
	return strings.HasPrefix(specifier, "./") ||
		strings.HasPrefix(specifier, "../") ||
		strings.HasPrefix(specifier, "/") && !strings.HasPrefix(specifier, "//") ||
		strings.HasPrefix(specifier, "@/")
}

func skip_js_quoted_string(source []byte, start int, quote byte) int {
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

func skip_js_template(source []byte, position int) int {
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

func skip_js_line_comment(source []byte, position int) int {
	for position < len(source) && source[position] != '\n' && source[position] != '\r' {
		position++
	}
	return position
}

func skip_js_block_comment(source []byte, position int) int {
	for position+1 < len(source) {
		if source[position] == '*' && source[position+1] == '/' {
			return position + 2
		}
		position++
	}
	return len(source)
}

func skip_js_regexp(source []byte, position int) int {
	in_character_class := false
	for position < len(source) {
		switch source[position] {
		case '\\':
			position += 2
			continue
		case '[':
			in_character_class = true
		case ']':
			in_character_class = false
		case '/':
			if !in_character_class {
				position++
				for position < len(source) && is_js_identifier_continue(source[position]) {
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

func is_js_space(value byte) bool {
	switch value {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	default:
		return false
	}
}

func is_js_identifier_start(value byte) bool {
	return value == '_' || value == '$' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func is_js_identifier_continue(value byte) bool {
	return is_js_identifier_start(value) || value >= '0' && value <= '9'
}

func is_js_number_continue(value byte) bool {
	return is_js_identifier_continue(value) || value == '.'
}

func js_keyword_allows_regexp(keyword string) bool {
	switch keyword {
	case "await", "case", "delete", "do", "else", "in", "instanceof", "new", "return", "throw", "typeof", "void", "yield":
		return true
	default:
		return false
	}
}

// render_frontend_html_responses applies the same runtime-variable rendering used
// by the root route to index.html responses produced by the SPA fallback.
func render_frontend_html_responses(next http.Handler, render func([]byte) []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer := &frontend_html_response_writer{ResponseWriter: w}
		next.ServeHTTP(writer, r)
		writer.flush(render)
	})
}

type frontend_html_response_writer struct {
	http.ResponseWriter
	status_code  int
	wrote_header bool
	buffer_html  bool
	body         bytes.Buffer
}

func (w *frontend_html_response_writer) WriteHeader(status_code int) {
	if w.wrote_header {
		return
	}
	w.wrote_header = true
	w.status_code = status_code
	w.buffer_html = status_code >= http.StatusOK &&
		status_code != http.StatusNoContent &&
		status_code != http.StatusNotModified &&
		strings.HasPrefix(strings.ToLower(w.Header().Get("Content-Type")), "text/html")
	if !w.buffer_html {
		w.ResponseWriter.WriteHeader(status_code)
	}
}

func (w *frontend_html_response_writer) Write(data []byte) (int, error) {
	if !w.wrote_header {
		w.WriteHeader(http.StatusOK)
	}
	if w.buffer_html {
		return w.body.Write(data)
	}
	return w.ResponseWriter.Write(data)
}

func (w *frontend_html_response_writer) flush(render func([]byte) []byte) {
	if !w.buffer_html {
		return
	}
	body := render(w.body.Bytes())
	set_frontend_html_cache_headers(w.Header())
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.ResponseWriter.WriteHeader(w.status_code)
	_, _ = w.ResponseWriter.Write(body)
}

func set_frontend_html_cache_headers(header http.Header) {
	header.Set("Cache-Control", "no-store")
	header.Set("Pragma", "no-cache")
	header.Set("Expires", "0")
}

func (c *APIClient) is_logging_enabled() bool {
	if c.cfg == nil || c.cfg.Original == nil {
		return false
	}
	return c.cfg.Original.LogPath() != "" || c.cfg.Original.GetBool("log.enabled")
}

// handle_frontend_tip handles frontend log/tip messages posted from injected pages.
func (c *APIClient) handle_frontend_tip(ctx *gin.Context) {
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
	if !c.is_logging_enabled() {
		result.Ok(ctx, nil)
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
		lvl := zerolog_level(data.Level)
		evt := c.logger.WithLevel(lvl).
			Str("component", "frontend")
		if data.Time != nil {
			evt.Ctx(logtime.WithTimestamp(ctx.Request.Context(), data.Time.In(time.Local)))
		}
		for key, value := range extra_fields {
			evt = evt.Interface(key, normalize_frontend_report_value(value))
		}
		evt.Msg(report_message)

		// Terminal display: errors are always displayed
		if data.Level == "error" {
			color.Red(fmt.Sprintf("[FRONTEND ERROR]%s\n", report_message))
			continue
		}

		// Non-error logs are only displayed if logging is explicitly enabled
		if !c.is_logging_enabled() || data.Level == "debug" || data.Level == "trace" {
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
	if path == "/favicon.ico" || path == "/" || path == "/play" || path == "/report" || path == "/mcp" {
		return true
	}

	api_prefixes := []string{
		"/api/",
		"/ws/",
		"/rss/",
		"/mp/",
		"/__assets/",
	}
	for _, prefix := range api_prefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
