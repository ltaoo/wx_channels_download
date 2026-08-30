package minib

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"wx_channel/pkg/clawreq"
)

const har_page_id = "page_1"

type har_context_key int

const (
	har_recorder_context_key har_context_key = iota
	har_resource_type_context_key
)

type har_recorder struct {
	mutex               sync.Mutex
	started_at          time.Time
	entries             []har_entry
	on_content_load     float64
	on_load             float64
	omit_bodies         bool
	max_body_bytes      int64
	captured_body_bytes int64
}

type har_archive struct {
	Log har_log `json:"log"`
}

type har_log struct {
	Version string      `json:"version"`
	Creator har_creator `json:"creator"`
	Pages   []har_page  `json:"pages"`
	Entries []har_entry `json:"entries"`
}

type har_creator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type har_page struct {
	StartedDateTime string           `json:"startedDateTime"`
	ID              string           `json:"id"`
	Title           string           `json:"title"`
	PageTimings     har_page_timings `json:"pageTimings"`
}

type har_page_timings struct {
	OnContentLoad float64 `json:"onContentLoad"`
	OnLoad        float64 `json:"onLoad"`
}

type har_entry struct {
	PageRef         string                 `json:"pageref"`
	StartedDateTime string                 `json:"startedDateTime"`
	Time            float64                `json:"time"`
	Request         har_request            `json:"request"`
	Response        har_response           `json:"response"`
	Cache           map[string]interface{} `json:"cache"`
	Timings         har_timings            `json:"timings"`
	ResourceType    string                 `json:"_resourceType,omitempty"`
	FromCache       string                 `json:"_fromCache,omitempty"`
	Error           string                 `json:"_error,omitempty"`
	started_at      time.Time
}

type har_request struct {
	Method      string           `json:"method"`
	URL         string           `json:"url"`
	HTTPVersion string           `json:"httpVersion"`
	Cookies     []har_cookie     `json:"cookies"`
	Headers     []har_name_value `json:"headers"`
	QueryString []har_name_value `json:"queryString"`
	PostData    *har_post_data   `json:"postData,omitempty"`
	HeadersSize int64            `json:"headersSize"`
	BodySize    int64            `json:"bodySize"`
}

type har_response struct {
	Status      int              `json:"status"`
	StatusText  string           `json:"statusText"`
	HTTPVersion string           `json:"httpVersion"`
	Cookies     []har_cookie     `json:"cookies"`
	Headers     []har_name_value `json:"headers"`
	Content     har_content      `json:"content"`
	RedirectURL string           `json:"redirectURL"`
	HeadersSize int64            `json:"headersSize"`
	BodySize    int64            `json:"bodySize"`
}

type har_content struct {
	Size      int64  `json:"size"`
	MimeType  string `json:"mimeType"`
	Text      string `json:"text,omitempty"`
	Encoding  string `json:"encoding,omitempty"`
	Truncated bool   `json:"_truncated,omitempty"`
}

type har_cookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Path     string `json:"path,omitempty"`
	Domain   string `json:"domain,omitempty"`
	Expires  string `json:"expires,omitempty"`
	HTTPOnly bool   `json:"httpOnly,omitempty"`
	Secure   bool   `json:"secure,omitempty"`
	SameSite string `json:"sameSite,omitempty"`
}

type har_name_value struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type har_post_data struct {
	MimeType  string `json:"mimeType"`
	Text      string `json:"text"`
	Truncated bool   `json:"_truncated,omitempty"`
}

type har_timings struct {
	Blocked float64 `json:"blocked"`
	DNS     float64 `json:"dns"`
	Connect float64 `json:"connect"`
	Send    float64 `json:"send"`
	Wait    float64 `json:"wait"`
	Receive float64 `json:"receive"`
	SSL     float64 `json:"ssl"`
}

func new_har_recorder(started_at time.Time, omit_bodies bool, max_body_bytes int64) *har_recorder {
	return &har_recorder{
		started_at:      started_at,
		on_content_load: -1,
		on_load:         -1,
		omit_bodies:     omit_bodies,
		max_body_bytes:  max_body_bytes,
	}
}

func with_har_recorder(ctx context.Context, recorder *har_recorder) context.Context {
	return context.WithValue(ctx, har_recorder_context_key, recorder)
}

func with_har_resource_type(ctx context.Context, resource_type string) context.Context {
	return context.WithValue(ctx, har_resource_type_context_key, resource_type)
}

func har_recorder_from_context(ctx context.Context) *har_recorder {
	if ctx == nil {
		return nil
	}
	recorder, _ := ctx.Value(har_recorder_context_key).(*har_recorder)
	return recorder
}

func har_resource_type_from_context(ctx context.Context) string {
	if ctx == nil {
		return "other"
	}
	resource_type, _ := ctx.Value(har_resource_type_context_key).(string)
	if resource_type == "" {
		return "other"
	}
	return resource_type
}

func (recorder *har_recorder) record_network(ctx context.Context, started_at time.Time, duration time.Duration, method string, raw_url string, request_headers http.Header, request_body []byte, response *clawreq.Response, request_err error) {
	if recorder == nil {
		return
	}
	response_headers := make(http.Header)
	status_code := 0
	status_text := ""
	response_body := []byte(nil)
	if response != nil {
		response_headers = response.Header
		status_code = response.StatusCode
		status_text = http.StatusText(status_code)
		response_body = response.Body
	}
	captured_request_body, request_truncated := recorder.capture_body(request_body)
	captured_response_body, response_truncated := recorder.capture_body(response_body)
	entry := build_har_entry(started_at, duration, method, raw_url, request_headers, captured_request_body, int64(len(request_body)), request_truncated, status_code, status_text, response_headers, captured_response_body, int64(len(response_body)), response_truncated, har_resource_type_from_context(ctx), "")
	if request_err != nil {
		entry.Error = request_err.Error()
	}
	recorder.append_entry(entry)
}

func (recorder *har_recorder) record_cached(ctx context.Context, raw_url string, request_headers http.Header, response_headers http.Header, resource Resource) {
	if recorder == nil {
		return
	}
	captured_response_body, response_truncated := recorder.capture_body(resource.Body)
	entry := build_har_entry(time.Now(), 0, http.MethodGet, raw_url, request_headers, nil, 0, false, resource.StatusCode, http.StatusText(resource.StatusCode), response_headers, captured_response_body, int64(len(resource.Body)), response_truncated, har_resource_type_from_context(ctx), "memory")
	recorder.append_entry(entry)
}

func (recorder *har_recorder) capture_body(body []byte) ([]byte, bool) {
	if recorder == nil || len(body) == 0 {
		return nil, false
	}
	recorder.mutex.Lock()
	defer recorder.mutex.Unlock()
	if recorder.omit_bodies {
		return nil, true
	}
	allowed_bytes := int64(len(body))
	if recorder.max_body_bytes > 0 {
		remaining_bytes := recorder.max_body_bytes - recorder.captured_body_bytes
		if remaining_bytes <= 0 {
			return nil, true
		}
		if allowed_bytes > remaining_bytes {
			allowed_bytes = remaining_bytes
		}
	}
	recorder.captured_body_bytes += allowed_bytes
	return body[:allowed_bytes], allowed_bytes < int64(len(body))
}

func (recorder *har_recorder) append_entry(entry har_entry) {
	recorder.mutex.Lock()
	recorder.entries = append(recorder.entries, entry)
	recorder.mutex.Unlock()
}

func (recorder *har_recorder) mark_content_loaded() {
	if recorder == nil {
		return
	}
	recorder.mutex.Lock()
	recorder.on_content_load = milliseconds(time.Since(recorder.started_at))
	recorder.mutex.Unlock()
}

func (recorder *har_recorder) mark_loaded() {
	if recorder == nil {
		return
	}
	recorder.mutex.Lock()
	recorder.on_load = milliseconds(time.Since(recorder.started_at))
	recorder.mutex.Unlock()
}

func (recorder *har_recorder) marshal(title string) ([]byte, error) {
	if recorder == nil {
		return nil, errors.New("minib: HAR recorder is unavailable")
	}
	recorder.mutex.Lock()
	entries := append([]har_entry(nil), recorder.entries...)
	on_content_load := recorder.on_content_load
	on_load := recorder.on_load
	recorder.mutex.Unlock()
	sort.SliceStable(entries, func(left_index, right_index int) bool {
		return entries[left_index].started_at.Before(entries[right_index].started_at)
	})
	for index := range entries {
		entries[index].started_at = time.Time{}
	}
	archive := har_archive{Log: har_log{
		Version: "1.2",
		Creator: har_creator{Name: "minib", Version: "1.0"},
		Pages: []har_page{{
			StartedDateTime: recorder.started_at.UTC().Format(time.RFC3339Nano),
			ID:              har_page_id,
			Title:           title,
			PageTimings:     har_page_timings{OnContentLoad: on_content_load, OnLoad: on_load},
		}},
		Entries: entries,
	}}
	return json.Marshal(archive)
}

func build_har_entry(started_at time.Time, duration time.Duration, method string, raw_url string, request_headers http.Header, request_body []byte, request_body_size int64, request_truncated bool, status_code int, status_text string, response_headers http.Header, response_body []byte, response_body_size int64, response_truncated bool, resource_type string, from_cache string) har_entry {
	mime_type := response_headers.Get("Content-Type")
	response_body = valid_har_text_prefix(response_body, mime_type)
	content_text, content_encoding := har_content_body(response_body, mime_type)
	entry := har_entry{
		PageRef:         har_page_id,
		StartedDateTime: started_at.UTC().Format(time.RFC3339Nano),
		Time:            milliseconds(duration),
		Request: har_request{
			Method:      method,
			URL:         raw_url,
			HTTPVersion: "HTTP/1.1",
			Cookies:     har_request_cookies(request_headers),
			Headers:     har_headers(request_headers),
			QueryString: har_query_string(raw_url),
			HeadersSize: -1,
			BodySize:    request_body_size,
		},
		Response: har_response{
			Status:      status_code,
			StatusText:  status_text,
			HTTPVersion: "HTTP/1.1",
			Cookies:     har_response_cookies(response_headers),
			Headers:     har_headers(response_headers),
			Content:     har_content{Size: response_body_size, MimeType: mime_type, Text: content_text, Encoding: content_encoding, Truncated: response_truncated},
			RedirectURL: response_headers.Get("Location"),
			HeadersSize: -1,
			BodySize:    response_body_size,
		},
		Cache:        make(map[string]interface{}),
		Timings:      har_timings{Blocked: -1, DNS: -1, Connect: -1, Send: 0, Wait: milliseconds(duration), Receive: 0, SSL: -1},
		ResourceType: resource_type,
		FromCache:    from_cache,
		started_at:   started_at,
	}
	if request_body_size > 0 {
		request_mime_type := request_headers.Get("Content-Type")
		request_body = valid_har_text_prefix(request_body, request_mime_type)
		entry.Request.PostData = &har_post_data{MimeType: request_mime_type, Text: string(request_body), Truncated: request_truncated}
	}
	return entry
}

func har_headers(headers http.Header) []har_name_value {
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)
	values := make([]har_name_value, 0, len(names))
	for _, name := range names {
		for _, value := range headers.Values(name) {
			values = append(values, har_name_value{Name: name, Value: value})
		}
	}
	return values
}

func har_query_string(raw_url string) []har_name_value {
	parsed_url, err := url.Parse(raw_url)
	if err != nil {
		return []har_name_value{}
	}
	query := parsed_url.Query()
	names := make([]string, 0, len(query))
	for name := range query {
		names = append(names, name)
	}
	sort.Strings(names)
	values := make([]har_name_value, 0, len(names))
	for _, name := range names {
		for _, value := range query[name] {
			values = append(values, har_name_value{Name: name, Value: value})
		}
	}
	return values
}

func har_request_cookies(headers http.Header) []har_cookie {
	request := &http.Request{Header: headers}
	cookie_list := request.Cookies()
	result := make([]har_cookie, 0, len(cookie_list))
	for _, cookie := range cookie_list {
		result = append(result, har_cookie{Name: cookie.Name, Value: cookie.Value})
	}
	return result
}

func har_response_cookies(headers http.Header) []har_cookie {
	response := &http.Response{Header: headers}
	cookie_list := response.Cookies()
	result := make([]har_cookie, 0, len(cookie_list))
	for _, cookie := range cookie_list {
		expires := ""
		if !cookie.Expires.IsZero() {
			expires = cookie.Expires.UTC().Format(time.RFC3339Nano)
		}
		result = append(result, har_cookie{
			Name: cookie.Name, Value: cookie.Value, Path: cookie.Path, Domain: cookie.Domain,
			Expires: expires, HTTPOnly: cookie.HttpOnly, Secure: cookie.Secure, SameSite: har_same_site(cookie.SameSite),
		})
	}
	return result
}

func har_same_site(same_site http.SameSite) string {
	switch same_site {
	case http.SameSiteStrictMode:
		return "Strict"
	case http.SameSiteLaxMode:
		return "Lax"
	case http.SameSiteNoneMode:
		return "None"
	default:
		return ""
	}
}

func har_content_body(body []byte, mime_type string) (string, string) {
	if len(body) == 0 {
		return "", ""
	}
	lower_mime_type := strings.ToLower(mime_type)
	if utf8.Valid(body) && (strings.HasPrefix(lower_mime_type, "text/") || strings.Contains(lower_mime_type, "json") || strings.Contains(lower_mime_type, "javascript") || strings.Contains(lower_mime_type, "xml") || strings.Contains(lower_mime_type, "svg") || strings.Contains(lower_mime_type, "form")) {
		return string(body), ""
	}
	return base64.StdEncoding.EncodeToString(body), "base64"
}

func valid_har_text_prefix(body []byte, mime_type string) []byte {
	if len(body) == 0 || !har_text_mime_type(mime_type) {
		return body
	}
	for len(body) > 0 && !utf8.Valid(body) {
		body = body[:len(body)-1]
	}
	return body
}

func har_text_mime_type(mime_type string) bool {
	lower_mime_type := strings.ToLower(mime_type)
	return strings.HasPrefix(lower_mime_type, "text/") || strings.Contains(lower_mime_type, "json") || strings.Contains(lower_mime_type, "javascript") || strings.Contains(lower_mime_type, "xml") || strings.Contains(lower_mime_type, "svg") || strings.Contains(lower_mime_type, "form")
}

func milliseconds(duration time.Duration) float64 {
	return float64(duration) / float64(time.Millisecond)
}

// HAR returns the completed navigation as HAR 1.2 JSON.
func (page *Page) HAR() ([]byte, error) {
	if page == nil || len(page.har_data) == 0 {
		return nil, errors.New("minib: HAR data is unavailable")
	}
	return append([]byte(nil), page.har_data...), nil
}

// SaveHAR writes HAR JSON with owner-only permissions because it may contain credentials.
func (page *Page) SaveHAR(file_path string) error {
	file_path = strings.TrimSpace(file_path)
	if file_path == "" {
		return errors.New("minib: HAR output path is empty")
	}
	if page == nil || len(page.har_data) == 0 {
		return errors.New("minib: HAR data is unavailable")
	}
	if err := os.MkdirAll(filepath.Dir(file_path), 0755); err != nil {
		return fmt.Errorf("minib: create HAR directory: %w", err)
	}
	file, err := os.OpenFile(file_path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("minib: open HAR: %w", err)
	}
	if err := file.Chmod(0600); err != nil {
		_ = file.Close()
		return fmt.Errorf("minib: secure HAR permissions: %w", err)
	}
	if _, err := file.Write(page.har_data); err != nil {
		_ = file.Close()
		return fmt.Errorf("minib: write HAR: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("minib: close HAR: %w", err)
	}
	return nil
}

// SaveHTML writes the post-JavaScript DOM serialization with owner-only
// permissions because the document can contain session-specific data.
func (page *Page) SaveHTML(file_path string) error {
	if page == nil {
		return errors.New("minib: page is nil")
	}
	file_path = strings.TrimSpace(file_path)
	if file_path == "" {
		return errors.New("minib: HTML output path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(file_path), 0755); err != nil {
		return fmt.Errorf("minib: create HTML directory: %w", err)
	}
	file, err := os.OpenFile(file_path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("minib: open HTML: %w", err)
	}
	if err := file.Chmod(0600); err != nil {
		_ = file.Close()
		return fmt.Errorf("minib: secure HTML permissions: %w", err)
	}
	if _, err := file.WriteString(page.RenderedHTML); err != nil {
		_ = file.Close()
		return fmt.Errorf("minib: write HTML: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("minib: close HTML: %w", err)
	}
	return nil
}
