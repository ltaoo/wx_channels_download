package minib

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	runtime_debug "runtime/debug"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/andybalholm/cascadia"
	"github.com/dop251/goja"
	"github.com/evanw/esbuild/pkg/api"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"wx_channel/pkg/clawreq"
)

const (
	max_redirects                 = 10
	max_script_size               = 16 << 20
	resource_concurrency          = 8
	per_host_resource_concurrency = 6
	// A finite callback budget prevents telemetry intervals from keeping a
	// navigation alive forever after the document has otherwise become idle.
	max_timer_callbacks   = 256
	max_host_callbacks    = 256
	max_dynamic_scripts   = 64
	max_dynamic_resources = 256
	max_event_loop_rounds = 8
	max_call_stack_depth  = 1024
	max_timer_time_ms     = 1000
)

// NavigateOptions controls one navigation without changing browser-wide state.
type NavigateOptions struct {
	// DisableCache bypasses cache reads and writes and sends no-cache headers.
	DisableCache bool
	// DisableSubresources skips every external resource discovered in the HTML.
	// Inline scripts may still execute unless DisableJavaScript is also true.
	DisableSubresources bool
	// DisableCSS skips stylesheet/font discovery, download, parsing, and cascade
	// computation. Inline element.style remains available and getComputedStyle
	// returns defaults plus inline declarations for scraper compatibility.
	DisableCSS bool
	// DisableImages skips image and icon downloads while keeping their DOM nodes.
	DisableImages bool
	// DisableMedia skips audio, video, track, embed, and iframe subresources.
	DisableMedia bool
	// DisableJavaScript skips script discovery, download, and execution while
	// retaining a queryable DOM for SSR extraction.
	DisableJavaScript bool
	// JavaScriptTimeout limits one top-level script or host callback. Zero uses
	// the remaining navigation context deadline.
	JavaScriptTimeout time.Duration
	// ResourceTimeout limits each subresource request independently. A timed-out
	// resource is recorded as failed without consuming the whole navigation
	// deadline. Zero uses the navigation context directly.
	ResourceTimeout time.Duration
	// WaitUntil controls which lifecycle milestone completes navigation. The
	// zero value is WaitUntilLoad for backward compatibility.
	WaitUntil NavigationWaitUntil
	// WaitForSelector returns as soon as the DOM matches this CSS selector.
	// If WaitForContent is also set, both conditions must match.
	WaitForSelector string
	// WaitForContent returns as soon as the DOM text contains this literal
	// string. Script, style, and template source text is ignored.
	WaitForContent string
	// CaptureHAR records request and response bodies for later HAR export.
	CaptureHAR bool
	// HAROmitBodies records request/response metadata without retaining body
	// text. Original sizes and a truncation marker remain in the HAR.
	HAROmitBodies bool
	// HARMaxBodyBytes caps the total raw request/response bytes retained in the
	// HAR. Zero is unlimited. Entries beyond the budget retain metadata only.
	HARMaxBodyBytes int64
	// RequestHeaderModifier may update final request headers after session
	// cookies are merged. It applies only to this navigation.
	RequestHeaderModifier func(*http.Request) error
	// RuntimeInitializer installs site-specific JavaScript compatibility hooks
	// before page scripts. The standard window and DOM already exist unless
	// UseCustomRuntime is enabled.
	RuntimeInitializer func(*goja.Runtime, *Page) error
	// UseCustomRuntime lets RuntimeInitializer provide the window and DOM used by
	// page scripts. Minib still installs timers, base64, Web Crypto, and
	// WebAssembly host primitives. This is intended for isolated challenge VMs.
	UseCustomRuntime bool
	// RuntimeFinalizer runs after page scripts and before navigation returns.
	RuntimeFinalizer func(*goja.Runtime, *Page) error
	// RuntimeCleanup releases resources allocated by RuntimeInitializer. It runs
	// on both successful and failed script execution.
	RuntimeCleanup func()
}

// NavigationWaitUntil identifies a non-visual navigation lifecycle milestone.
type NavigationWaitUntil string

const (
	WaitUntilLoad             NavigationWaitUntil = "load"
	WaitUntilDOMContentLoaded NavigationWaitUntil = "domcontentloaded"
)

// ResourceKind identifies a resource discovered in the page HTML.
type ResourceKind string

const (
	ScriptResource ResourceKind = "script"
	StyleResource  ResourceKind = "style"
	ImageResource  ResourceKind = "image"
	FontResource   ResourceKind = "font"
	MediaResource  ResourceKind = "media"
	OtherResource  ResourceKind = "other"
)

// Resource contains one downloaded static page resource.
type Resource struct {
	URL         string
	FinalURL    string
	Kind        ResourceKind
	StatusCode  int
	ContentType string
	Body        []byte
	// FromCache reports that Body came from a fresh or revalidated cache entry.
	FromCache       bool
	Err             error
	fetch_priority  int
	discovery_order int
	completed_at    time.Time
}

// ScriptFailure records a script that could not be downloaded or executed.
type ScriptFailure struct {
	URL string
	Err error
}

// Page is the result of a browser-like navigation.
type Page struct {
	URL                  string
	StatusCode           int
	Headers              http.Header
	ContentType          string
	HTML                 string
	RenderedHTML         string
	Document             *html.Node
	Resources            []Resource
	ScriptFailures       []ScriptFailure
	ConsoleMessages      []string
	XHRRequests          []string
	FetchRequests        []string
	ExecutedScripts      int
	disable_cache        bool
	disable_subresources bool
	disable_css          bool
	disable_images       bool
	disable_media        bool
	disable_javascript   bool
	javascript_timeout   time.Duration
	resource_timeout     time.Duration
	wait_until           NavigationWaitUntil
	wait_for_selector    string
	wait_for_content     string
	wait_for_matcher     cascadia.SelectorGroup
	runtime_initializer  func(*goja.Runtime, *Page) error
	runtime_finalizer    func(*goja.Runtime, *Page) error
	runtime_cleanup      func()
	use_custom_runtime   bool
	har_data             []byte
	navigation_url       string
}

type script_job struct {
	node           *html.Node
	resource_index int
	inline         string
	source_url     string
	defer_script   bool
	async_script   bool
	module_script  bool
	document_order int
}

type timer_job struct {
	id          int64
	callback    goja.Callable
	args        []goja.Value
	due_at_ms   int64
	interval_ms int64
	repeating   bool
	canceled    bool
}

type event_listener struct {
	callback goja.Value
	capture  bool
	once     bool
	passive  bool
	removed  bool
}

type xml_http_request struct {
	runtime          *page_runtime
	object           *goja.Object
	method           string
	raw_url          string
	request_headers  http.Header
	response_headers http.Header
	ready_state      int
	status           int
	status_text      string
	response_text    string
	response_url     string
	listeners        map[string][]goja.Callable
	async_request    bool
	request_cancel   context.CancelFunc
	request_version  int64
}

type xhr_network_request struct {
	ctx           context.Context
	method        string
	raw_url       string
	body          []byte
	headers       http.Header
	resource_type string
	timeout       time.Duration
}

type xhr_network_result struct {
	status           int
	status_text      string
	response_headers http.Header
	response_text    string
	response_url     string
	err              error
}

type page_runtime struct {
	browser               *MiniBrowser
	ctx                   context.Context
	lifecycle_ctx         context.Context
	network_ctx           context.Context
	page                  *Page
	page_url              *url.URL
	base_url              *url.URL
	vm                    *goja.Runtime
	nodes                 map[*html.Node]*goja.Object
	object_nodes          map[*goja.Object]*html.Node
	fragments             map[*html.Node]bool
	shadow_roots          map[*html.Node]*html.Node
	shadow_hosts          map[*html.Node]*html.Node
	shadow_modes          map[*html.Node]string
	adopted_style_sheets  map[*html.Node]goja.Value
	template_contents     map[*html.Node]*html.Node
	styles                map[*html.Node]*goja.Object
	style_blocks          map[*html.Node]*css_declaration_block
	style_sheets          []*css_style_sheet
	style_sheet_by_node   map[*html.Node]*css_style_sheet
	computed_styles       map[*html.Node]map[string]css_property
	styles_dirty          bool
	dirty_style_roots     map[*html.Node]bool
	listeners             map[*html.Node]map[string][]*event_listener
	window_listeners      map[string][]*event_listener
	dispatching_events    map[*goja.Object]bool
	dynamic_scripts       []*html.Node
	dynamic_styles        []*html.Node
	dynamic_resources     []*html.Node
	dynamic_seen          map[*html.Node]bool
	custom_elements       map[string]*custom_element_definition
	custom_waiters        map[string][]func(interface{}) error
	custom_constructed    map[*html.Node]bool
	custom_connected      map[*html.Node]bool
	pending_custom_nodes  []*html.Node
	custom_reactions      []custom_element_reaction
	running_reactions     bool
	timers                []*timer_job
	timer_by_id           map[int64]*timer_job
	timer_time_ms         int64
	host_jobs             []func()
	next_timer_id         int64
	current_script        *html.Node
	current_script_url    string
	ready_state           string
	user_agent            string
	request_headers       http.Header
	disable_css           bool
	javascript_timeout    time.Duration
	wait_until            NavigationWaitUntil
	modules               map[string]*module_record
	import_map            map[string]string
	external_jobs         chan func()
	pending_network_tasks atomic.Int32
	websockets            map[*browser_websocket]bool
	webassembly           *webassembly_host
	blob_urls             map[string]string
	next_blob_id          int64
	use_custom_runtime    bool
	wait_active           bool
	wait_matched          bool
}

// Navigate fetches an HTML document, builds its DOM, downloads HTML-discovered
// static resources, then executes JavaScript script tags in document order.
func (b *MiniBrowser) Navigate(ctx context.Context, raw_url string, headers http.Header, options ...NavigateOptions) (*Page, error) {
	if b == nil || b.http_client == nil {
		return nil, fmt.Errorf("minib: browser is closed")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, has_deadline := ctx.Deadline(); !has_deadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, b.timeout)
		defer cancel()
	}
	navigate_options := NavigateOptions{}
	if len(options) > 0 {
		navigate_options = options[len(options)-1]
	}
	navigate_options.WaitForSelector = strings.TrimSpace(navigate_options.WaitForSelector)
	if err := validate_navigate_options(navigate_options); err != nil {
		return nil, err
	}
	if navigate_options.WaitUntil == "" {
		navigate_options.WaitUntil = WaitUntilLoad
	}
	if navigate_options.RequestHeaderModifier != nil {
		ctx = with_request_header_modifier(ctx, navigate_options.RequestHeaderModifier)
	}
	var har_recorder *har_recorder
	if navigate_options.CaptureHAR {
		har_recorder = new_har_recorder(time.Now(), navigate_options.HAROmitBodies, navigate_options.HARMaxBodyBytes)
		ctx = with_har_recorder(ctx, har_recorder)
	}
	navigation_ctx, cancel_navigation := context.WithCancel(ctx)
	defer cancel_navigation()
	page, err := b.navigate(navigation_ctx, raw_url, headers, navigate_options, 0)
	if err != nil {
		return nil, err
	}
	if har_recorder != nil {
		page.har_data, err = har_recorder.marshal(text_content(find_element(page.Document, "title")))
		if err != nil {
			return nil, fmt.Errorf("minib: build HAR: %w", err)
		}
	}
	return page, nil
}

func validate_navigate_options(options NavigateOptions) error {
	if options.UseCustomRuntime && options.RuntimeInitializer == nil {
		return fmt.Errorf("minib: UseCustomRuntime requires RuntimeInitializer")
	}
	if options.JavaScriptTimeout < 0 {
		return fmt.Errorf("minib: JavaScriptTimeout cannot be negative")
	}
	if options.ResourceTimeout < 0 {
		return fmt.Errorf("minib: ResourceTimeout cannot be negative")
	}
	if options.HARMaxBodyBytes < 0 {
		return fmt.Errorf("minib: HARMaxBodyBytes cannot be negative")
	}
	if options.WaitForSelector != "" {
		if _, err := cascadia.ParseGroup(options.WaitForSelector); err != nil {
			return fmt.Errorf("minib: invalid WaitForSelector %q: %w", options.WaitForSelector, err)
		}
	}
	switch options.WaitUntil {
	case "", WaitUntilLoad, WaitUntilDOMContentLoaded:
		return nil
	default:
		return fmt.Errorf("minib: unsupported WaitUntil value %q", options.WaitUntil)
	}
}

func (page *Page) has_wait_condition() bool {
	return page != nil && (page.wait_for_selector != "" || page.wait_for_content != "")
}

func (page *Page) wait_condition_met() bool {
	if !page.has_wait_condition() || page.Document == nil {
		return false
	}
	if page.wait_for_selector != "" && cascadia.Query(page.Document, page.wait_for_matcher) == nil {
		return false
	}
	return page.wait_for_content == "" || strings.Contains(rendered_text_content(page.Document), page.wait_for_content)
}

func (b *MiniBrowser) navigate(ctx context.Context, raw_url string, headers http.Header, navigate_options NavigateOptions, navigation_count int) (*Page, error) {
	document_headers := clawreq.DefaultHeaders(clawreq.ProfileChrome)
	for name, values := range headers {
		document_headers[name] = append([]string(nil), values...)
	}
	if navigate_options.DisableCache {
		disable_cache_headers(document_headers)
	}
	response, final_url, err := b.fetch_redirects(with_har_resource_type(ctx, "document"), raw_url, document_headers)
	if err != nil {
		return nil, fmt.Errorf("minib: fetch document: %w", err)
	}
	html_text, err := response.Text()
	if err != nil {
		return nil, fmt.Errorf("minib: decode document: %w", err)
	}
	document, err := html.Parse(strings.NewReader(html_text))
	if err != nil {
		return nil, fmt.Errorf("minib: parse document: %w", err)
	}
	page_url, err := url.Parse(final_url)
	if err != nil {
		return nil, fmt.Errorf("minib: parse final URL: %w", err)
	}
	page := &Page{
		URL:                  final_url,
		StatusCode:           response.StatusCode,
		Headers:              response.Header.Clone(),
		ContentType:          response.ContentType(),
		HTML:                 html_text,
		Document:             document,
		disable_cache:        navigate_options.DisableCache,
		disable_subresources: navigate_options.DisableSubresources,
		disable_css:          navigate_options.DisableCSS,
		disable_images:       navigate_options.DisableImages,
		disable_media:        navigate_options.DisableMedia,
		disable_javascript:   navigate_options.DisableJavaScript,
		javascript_timeout:   navigate_options.JavaScriptTimeout,
		resource_timeout:     navigate_options.ResourceTimeout,
		wait_until:           navigate_options.WaitUntil,
		wait_for_selector:    navigate_options.WaitForSelector,
		wait_for_content:     navigate_options.WaitForContent,
		runtime_initializer:  navigate_options.RuntimeInitializer,
		runtime_finalizer:    navigate_options.RuntimeFinalizer,
		runtime_cleanup:      navigate_options.RuntimeCleanup,
		use_custom_runtime:   navigate_options.UseCustomRuntime,
	}
	if page.wait_for_selector != "" {
		page.wait_for_matcher, _ = cascadia.ParseGroup(page.wait_for_selector)
	}
	base_url := document_base_url(document, page_url)
	var jobs []script_job
	if !page.wait_condition_met() {
		jobs = discover_page_resources(page, base_url, navigate_options)
		b.download_resources(ctx, page, page_url, document_headers, navigate_options.DisableCache, navigate_options.ResourceTimeout)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := b.execute_page(ctx, page, page_url, jobs, document_headers); err != nil {
		return nil, err
	}
	page.RenderedHTML = render_node(document)
	if page.navigation_url != "" {
		if navigation_count == max_redirects {
			return nil, fmt.Errorf("minib: too many JavaScript navigations")
		}
		next_headers := headers.Clone()
		if next_headers == nil {
			next_headers = make(http.Header)
		}
		next_headers.Set("Referer", page.URL)
		return b.navigate(ctx, page.navigation_url, next_headers, navigate_options, navigation_count+1)
	}
	if page.has_wait_condition() && !page.wait_condition_met() {
		return nil, fmt.Errorf("minib: navigation completed before wait condition matched")
	}
	return page, nil
}

func (b *MiniBrowser) fetch_redirects(ctx context.Context, raw_url string, headers http.Header) (*clawreq.Response, string, error) {
	current_url := raw_url
	request_headers := headers.Clone()
	for redirect_count := 0; redirect_count <= max_redirects; redirect_count++ {
		response, err := b.Get(ctx, current_url, request_headers)
		if err != nil {
			return nil, "", err
		}
		if !is_redirect(response.StatusCode) || strings.TrimSpace(response.Header.Get("Location")) == "" {
			return response, current_url, nil
		}
		if redirect_count == max_redirects {
			return nil, "", fmt.Errorf("too many redirects")
		}
		current, err := url.Parse(current_url)
		if err != nil {
			return nil, "", err
		}
		next, err := current.Parse(response.Header.Get("Location"))
		if err != nil || (next.Scheme != "http" && next.Scheme != "https") {
			return nil, "", fmt.Errorf("invalid redirect location %q", response.Header.Get("Location"))
		}
		if !same_origin(current, next) {
			request_headers.Del("Authorization")
			request_headers.Del("Cookie")
		}
		request_headers.Set("Referer", current.String())
		current_url = next.String()
	}
	return nil, "", fmt.Errorf("too many redirects")
}

func is_redirect(status_code int) bool {
	switch status_code {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	default:
		return false
	}
}

func same_origin(first *url.URL, second *url.URL) bool {
	return first != nil && second != nil && strings.EqualFold(first.Scheme, second.Scheme) && strings.EqualFold(first.Host, second.Host)
}

func document_base_url(document *html.Node, page_url *url.URL) *url.URL {
	if page_url == nil {
		return &url.URL{}
	}
	base_url := *page_url
	for _, base_node := range find_by_tag(document, "base") {
		href := strings.TrimSpace(attribute(base_node, "href"))
		if href == "" {
			continue
		}
		resolved_url, err := page_url.Parse(href)
		if err == nil && (resolved_url.Scheme == "http" || resolved_url.Scheme == "https") {
			resolved_url.Fragment = ""
			return resolved_url
		}
		break
	}
	return &base_url
}

func discover_page_resources(page *Page, page_url *url.URL, navigate_options NavigateOptions) []script_job {
	jobs := make([]script_job, 0)
	resource_indexes := make(map[string]int)
	inline_index := 0
	document_order := 0
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			document_order++
			tag_name := strings.ToLower(node.Data)
			switch tag_name {
			case "script":
				if !navigate_options.DisableJavaScript && is_javascript_type(attribute(node, "type")) {
					module_script := strings.EqualFold(strings.TrimSpace(attribute(node, "type")), "module")
					if source := attribute(node, "src"); source != "" && !navigate_options.DisableSubresources {
						if resource_url, ok := resolve_resource_url(page_url, source); ok {
							_, resource_exists := resource_indexes[resource_url]
							resource_index := add_resource(page, resource_indexes, resource_url, ScriptResource)
							script_priority := script_resource_priority(node, module_script)
							if resource_exists {
								page.Resources[resource_index].fetch_priority = min_int(page.Resources[resource_index].fetch_priority, script_priority)
							} else {
								page.Resources[resource_index].fetch_priority = script_priority
							}
							jobs = append(jobs, script_job{
								node:           node,
								resource_index: resource_index,
								source_url:     resource_url,
								defer_script:   has_attribute(node, "defer") || module_script,
								async_script:   has_attribute(node, "async"),
								module_script:  module_script,
								document_order: document_order,
							})
						}
					} else if source := text_content(node); strings.TrimSpace(source) != "" {
						inline_index++
						jobs = append(jobs, script_job{
							node:           node,
							resource_index: -1,
							inline:         source,
							source_url:     fmt.Sprintf("%s#inline-%d", page.URL, inline_index),
							defer_script:   module_script,
							module_script:  module_script,
							document_order: document_order,
						})
					}
				}
			case "link":
				kind, load := link_resource_kind(node)
				if navigate_options.DisableSubresources ||
					(navigate_options.DisableCSS && (kind == StyleResource || kind == FontResource)) ||
					(navigate_options.DisableImages && kind == ImageResource) ||
					(navigate_options.DisableJavaScript && kind == ScriptResource) {
					load = false
				}
				if load {
					if resource_url, ok := resolve_resource_url(page_url, attribute(node, "href")); ok {
						resource_index := add_resource(page, resource_indexes, resource_url, kind)
						page.Resources[resource_index].fetch_priority = apply_fetch_priority(page.Resources[resource_index].fetch_priority, attribute(node, "fetchpriority"))
					}
				}
			case "img":
				if !navigate_options.DisableSubresources && !navigate_options.DisableImages {
					if resource_url, ok := resolve_resource_url(page_url, attribute(node, "src")); ok {
						resource_index := add_resource(page, resource_indexes, resource_url, ImageResource)
						page.Resources[resource_index].fetch_priority = apply_fetch_priority(page.Resources[resource_index].fetch_priority, attribute(node, "fetchpriority"))
					}
				}
			case "audio", "video", "source", "track", "embed", "iframe":
				if !navigate_options.DisableSubresources && !navigate_options.DisableMedia {
					if resource_url, ok := resolve_resource_url(page_url, attribute(node, "src")); ok {
						add_resource(page, resource_indexes, resource_url, MediaResource)
					}
				}
				if tag_name == "video" && !navigate_options.DisableSubresources && !navigate_options.DisableImages {
					if resource_url, ok := resolve_resource_url(page_url, attribute(node, "poster")); ok {
						add_resource(page, resource_indexes, resource_url, ImageResource)
					}
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(page.Document)
	return jobs
}

func add_resource(page *Page, indexes map[string]int, resource_url string, kind ResourceKind) int {
	if index, ok := indexes[resource_url]; ok {
		return index
	}
	index := len(page.Resources)
	indexes[resource_url] = index
	page.Resources = append(page.Resources, Resource{URL: resource_url, Kind: kind, fetch_priority: default_resource_priority(kind), discovery_order: index})
	return index
}

func default_resource_priority(kind ResourceKind) int {
	switch kind {
	case ScriptResource, StyleResource:
		return 1
	case FontResource:
		return 2
	case OtherResource:
		return 3
	case ImageResource:
		return 4
	case MediaResource:
		return 5
	default:
		return 3
	}
}

func script_resource_priority(node *html.Node, module_script bool) int {
	priority := 0
	if has_attribute(node, "async") {
		priority = 1
	} else if has_attribute(node, "defer") || module_script {
		priority = 2
	}
	return apply_fetch_priority(priority, attribute(node, "fetchpriority"))
}

func apply_fetch_priority(priority int, hint string) int {
	switch strings.ToLower(strings.TrimSpace(hint)) {
	case "high":
		if priority > 0 {
			priority--
		}
	case "low":
		priority += 2
	}
	return priority
}

func min_int(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func link_resource_kind(node *html.Node) (ResourceKind, bool) {
	rel := strings.Fields(strings.ToLower(attribute(node, "rel")))
	as_value := strings.ToLower(attribute(node, "as"))
	for _, value := range rel {
		switch value {
		case "stylesheet":
			return StyleResource, true
		case "modulepreload":
			return ScriptResource, true
		case "icon", "apple-touch-icon":
			return ImageResource, true
		case "preload":
			switch as_value {
			case "script":
				return ScriptResource, true
			case "style":
				return StyleResource, true
			case "image":
				return ImageResource, true
			case "font":
				return FontResource, true
			default:
				return OtherResource, true
			}
		}
	}
	return "", false
}

func is_javascript_type(value string) bool {
	value = strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
	return value == "" || value == "module" || strings.Contains(value, "javascript") || strings.Contains(value, "ecmascript")
}

func resolve_resource_url(base_url *url.URL, raw_url string) (string, bool) {
	raw_url = strings.TrimSpace(raw_url)
	if raw_url == "" {
		return "", false
	}
	parsed_url, err := url.Parse(raw_url)
	if err != nil {
		return "", false
	}
	resolved_url := base_url.ResolveReference(parsed_url)
	if resolved_url.Scheme != "http" && resolved_url.Scheme != "https" {
		return "", false
	}
	resolved_url.Fragment = ""
	return resolved_url.String(), true
}

func (b *MiniBrowser) download_resources(ctx context.Context, page *Page, page_url *url.URL, request_headers http.Header, disable_cache bool, resource_timeout time.Duration) {
	resource_indexes := make([]int, len(page.Resources))
	host_semaphores := make(map[string]chan struct{})
	for index := range page.Resources {
		resource_indexes[index] = index
		if resource_url, err := url.Parse(page.Resources[index].URL); err == nil {
			host_key := strings.ToLower(resource_url.Scheme + "://" + resource_url.Host)
			if host_semaphores[host_key] == nil {
				host_semaphores[host_key] = make(chan struct{}, per_host_resource_concurrency)
			}
		}
	}
	sort.SliceStable(resource_indexes, func(left_index int, right_index int) bool {
		left := page.Resources[resource_indexes[left_index]]
		right := page.Resources[resource_indexes[right_index]]
		if left.fetch_priority == right.fetch_priority {
			return left.discovery_order < right.discovery_order
		}
		return left.fetch_priority < right.fetch_priority
	})
	worker_count := resource_concurrency
	if worker_count > len(resource_indexes) {
		worker_count = len(resource_indexes)
	}
	resource_queue := make(chan int)
	var wait_group sync.WaitGroup
	wait_group.Add(worker_count)
	for worker_index := 0; worker_index < worker_count; worker_index++ {
		go func() {
			defer wait_group.Done()
			for resource_index := range resource_queue {
				resource := page.Resources[resource_index]
				resource_url, _ := url.Parse(resource.URL)
				host_key := strings.ToLower(resource_url.Scheme + "://" + resource_url.Host)
				host_semaphore := host_semaphores[host_key]
				if host_semaphore != nil {
					select {
					case host_semaphore <- struct{}{}:
					case <-ctx.Done():
						resource.Err = ctx.Err()
						resource.completed_at = time.Now()
						page.Resources[resource_index] = resource
						continue
					}
				}
				resource_ctx, cancel := context_with_optional_timeout(ctx, resource_timeout)
				resource = b.download_resource(resource_ctx, page_url, request_headers, resource, disable_cache)
				cancel()
				if host_semaphore != nil {
					<-host_semaphore
				}
				resource.completed_at = time.Now()
				page.Resources[resource_index] = resource
			}
		}()
	}
	for _, resource_index := range resource_indexes {
		select {
		case resource_queue <- resource_index:
		case <-ctx.Done():
			page.Resources[resource_index].Err = ctx.Err()
			page.Resources[resource_index].completed_at = time.Now()
		}
	}
	close(resource_queue)
	wait_group.Wait()
}

func (b *MiniBrowser) download_resource(ctx context.Context, page_url *url.URL, request_headers http.Header, resource Resource, disable_cache bool) Resource {
	ctx = with_har_resource_type(ctx, string(resource.Kind))
	headers := resource_headers(page_url, request_headers, resource.URL, resource.Kind)
	headers.Set("Priority", resource_priority_header(resource.fetch_priority))
	if disable_cache {
		disable_cache_headers(headers)
	}
	cache_request_headers, err := b.prepare_request_headers(resource.URL, headers)
	if err != nil {
		resource.Err = err
		return resource
	}
	cached, cache_found := resource_cache_entry{}, false
	if !disable_cache && b.resource_cache != nil {
		cached, cache_found = b.resource_cache.lookup(resource.URL, cache_request_headers)
		if cache_found && cached.fresh(time.Now()) {
			cached_resource := cached.cached_resource()
			har_recorder_from_context(ctx).record_cached(ctx, resource.URL, cache_request_headers, cached.response_headers, cached_resource)
			return cached_resource
		}
		if cache_found {
			if etag := cached.response_headers.Get("ETag"); etag != "" {
				headers.Set("If-None-Match", etag)
				cache_request_headers.Set("If-None-Match", etag)
			}
			if last_modified := cached.response_headers.Get("Last-Modified"); last_modified != "" {
				headers.Set("If-Modified-Since", last_modified)
				cache_request_headers.Set("If-Modified-Since", last_modified)
			}
		}
	}
	response, final_url, err := b.fetch_redirects(ctx, resource.URL, cache_request_headers)
	if err != nil {
		resource.Err = err
		return resource
	}
	response_time := time.Now()
	if response.StatusCode == http.StatusNotModified && cache_found {
		merged_headers := merge_response_headers(cached.response_headers, response.Header)
		resource = cached.cached_resource()
		resource.ContentType = merged_headers.Get("Content-Type")
		b.resource_cache.store(resource.URL, cache_request_headers, merged_headers, resource, response_time)
		return resource
	}
	resource.FinalURL = final_url
	resource.StatusCode = response.StatusCode
	resource.ContentType = response.ContentType()
	resource.Body = response.Body
	resource.FromCache = false
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		resource.Err = fmt.Errorf("HTTP %d", response.StatusCode)
	}
	if !disable_cache && b.resource_cache != nil {
		if response_cacheable(resource.StatusCode, response.Header) {
			b.resource_cache.store(resource.URL, cache_request_headers, response.Header, resource, response_time)
		} else if _, no_store := cache_control_directives(response.Header)["no-store"]; no_store {
			b.resource_cache.remove(resource.URL, cache_request_headers)
		}
	}
	return resource
}

func resource_priority_header(priority int) string {
	switch {
	case priority <= 0:
		return "u=0"
	case priority == 1:
		return "u=1"
	case priority == 2:
		return "u=2"
	case priority == 3:
		return "u=3"
	case priority == 4:
		return "u=5, i"
	default:
		return "u=7, i"
	}
}

func disable_cache_headers(headers http.Header) {
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Pragma", "no-cache")
}

func resource_headers(page_url *url.URL, request_headers http.Header, resource_url string, kind ResourceKind) http.Header {
	headers := request_headers.Clone()
	if headers == nil {
		headers = clawreq.DefaultHeaders(clawreq.ProfileChrome)
	}
	headers.Set("Referer", page_url.String())
	headers.Set("Sec-Fetch-Mode", "no-cors")
	headers.Del("Sec-Fetch-User")
	headers.Del("Upgrade-Insecure-Requests")
	resource_parsed, _ := url.Parse(resource_url)
	if same_origin(page_url, resource_parsed) {
		headers.Set("Sec-Fetch-Site", "same-origin")
	} else {
		headers.Set("Sec-Fetch-Site", "cross-site")
	}
	switch kind {
	case ScriptResource:
		headers.Set("Accept", "*/*")
		headers.Set("Sec-Fetch-Dest", "script")
	case StyleResource:
		headers.Set("Accept", "text/css,*/*;q=0.1")
		headers.Set("Sec-Fetch-Dest", "style")
	case ImageResource:
		headers.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
		headers.Set("Sec-Fetch-Dest", "image")
	case FontResource:
		headers.Set("Accept", "font/woff2,*/*;q=0.1")
		headers.Set("Sec-Fetch-Dest", "font")
	default:
		headers.Set("Accept", "*/*")
		headers.Set("Sec-Fetch-Dest", "empty")
	}
	return headers
}

func (b *MiniBrowser) execute_page(ctx context.Context, page *Page, page_url *url.URL, jobs []script_job, request_headers http.Header) error {
	b.js_mutex.Lock()
	defer b.js_mutex.Unlock()
	if b.page_runtime != nil {
		b.page_runtime.close_webassembly()
	}
	b.js_runtime = goja.New()
	if page.runtime_cleanup != nil {
		defer page.runtime_cleanup()
	}
	b.js_runtime.SetMaxCallStackSize(max_call_stack_depth)
	runtime := &page_runtime{
		browser:              b,
		ctx:                  ctx,
		lifecycle_ctx:        b.lifecycle_ctx,
		network_ctx:          ctx,
		page:                 page,
		page_url:             page_url,
		base_url:             document_base_url(page.Document, page_url),
		vm:                   b.js_runtime,
		nodes:                make(map[*html.Node]*goja.Object),
		object_nodes:         make(map[*goja.Object]*html.Node),
		fragments:            make(map[*html.Node]bool),
		shadow_roots:         make(map[*html.Node]*html.Node),
		shadow_hosts:         make(map[*html.Node]*html.Node),
		shadow_modes:         make(map[*html.Node]string),
		adopted_style_sheets: make(map[*html.Node]goja.Value),
		template_contents:    make(map[*html.Node]*html.Node),
		styles:               make(map[*html.Node]*goja.Object),
		style_blocks:         make(map[*html.Node]*css_declaration_block),
		style_sheet_by_node:  make(map[*html.Node]*css_style_sheet),
		computed_styles:      make(map[*html.Node]map[string]css_property),
		styles_dirty:         true,
		dirty_style_roots:    make(map[*html.Node]bool),
		listeners:            make(map[*html.Node]map[string][]*event_listener),
		window_listeners:     make(map[string][]*event_listener),
		dispatching_events:   make(map[*goja.Object]bool),
		timer_by_id:          make(map[int64]*timer_job),
		dynamic_seen:         make(map[*html.Node]bool),
		custom_elements:      make(map[string]*custom_element_definition),
		custom_waiters:       make(map[string][]func(interface{}) error),
		custom_constructed:   make(map[*html.Node]bool),
		custom_connected:     make(map[*html.Node]bool),
		modules:              make(map[string]*module_record),
		import_map:           parse_document_import_map(page.Document, document_base_url(page.Document, page_url)),
		external_jobs:        make(chan func(), max_host_callbacks),
		websockets:           make(map[*browser_websocket]bool),
		blob_urls:            make(map[string]string),
		ready_state:          "loading",
		user_agent:           request_headers.Get("User-Agent"),
		request_headers:      request_headers.Clone(),
		disable_css:          page.disable_css,
		javascript_timeout:   page.javascript_timeout,
		wait_until:           page.wait_until,
		use_custom_runtime:   page.use_custom_runtime,
		wait_active:          page.has_wait_condition(),
	}
	b.page_runtime = runtime
	defer func() {
		runtime.ctx = b.lifecycle_ctx
		runtime.network_ctx = b.lifecycle_ctx
		runtime.wait_active = false
	}()
	b.js_runtime.SetPromiseRejectionTracker(func(promise *goja.Promise, operation goja.PromiseRejectionOperation) {
		if operation == goja.PromiseRejectionReject {
			result := promise.Result()
			message := result.String()
			if object, ok := result.(*goja.Object); ok {
				if stack := object.Get("stack"); stack != nil && !goja.IsUndefined(stack) && !goja.IsNull(stack) && stack.String() != "" {
					message = stack.String()
				}
			}
			page.ConsoleMessages = append(page.ConsoleMessages, "unhandled rejection: "+message)
		}
	})
	if runtime.use_custom_runtime {
		if err := runtime.install_custom_host_runtime(); err != nil {
			return fmt.Errorf("minib: initialize custom host runtime: %w", err)
		}
	} else if err := runtime.install(); err != nil {
		return fmt.Errorf("minib: initialize page runtime: %w", err)
	}
	if runtime.wait_condition_met() {
		return nil
	}
	if page.runtime_initializer != nil {
		if err := page.runtime_initializer(runtime.vm, page); err != nil {
			return fmt.Errorf("minib: initialize site runtime: %w", err)
		}
	}
	if runtime.wait_condition_met() {
		return nil
	}
	if !runtime.use_custom_runtime {
		runtime.refresh_style_sheets()
	}
	blocking_jobs := make([]script_job, 0, len(jobs))
	async_jobs := make([]script_job, 0, len(jobs))
	deferred_jobs := make([]script_job, 0, len(jobs))
	for _, job := range jobs {
		switch {
		case job.async_script:
			async_jobs = append(async_jobs, job)
		case job.defer_script:
			deferred_jobs = append(deferred_jobs, job)
		default:
			blocking_jobs = append(blocking_jobs, job)
		}
	}
	sort.SliceStable(async_jobs, func(left_index int, right_index int) bool {
		left_job := async_jobs[left_index]
		right_job := async_jobs[right_index]
		if left_job.resource_index >= 0 && right_job.resource_index >= 0 {
			left_completed := page.Resources[left_job.resource_index].completed_at
			right_completed := page.Resources[right_job.resource_index].completed_at
			if !left_completed.Equal(right_completed) {
				return left_completed.Before(right_completed)
			}
		}
		return left_job.document_order < right_job.document_order
	})
	ordered_jobs := append(blocking_jobs, async_jobs...)
	ordered_jobs = append(ordered_jobs, deferred_jobs...)
	for _, job := range ordered_jobs {
		if runtime.wait_condition_met() {
			return nil
		}
		if err := ctx.Err(); err != nil {
			page.ScriptFailures = append(page.ScriptFailures, ScriptFailure{URL: job.source_url, Err: err})
			break
		}
		failure_count := len(page.ScriptFailures)
		runtime.execute_job(ctx, job)
		if runtime.wait_condition_met() {
			return nil
		}
		if job.resource_index >= 0 && !runtime.use_custom_runtime {
			if len(page.ScriptFailures) == failure_count {
				runtime.fire_node_event(job.node, "load")
			} else {
				runtime.fire_node_event(job.node, "error")
			}
		}
		if runtime.wait_condition_met() {
			return nil
		}
		runtime.run_host_jobs(ctx)
		if runtime.wait_condition_met() {
			return nil
		}
		if !runtime.use_custom_runtime {
			runtime.drain_dynamic_styles(ctx)
			runtime.drain_dynamic_scripts(ctx)
			runtime.drain_dynamic_resources(ctx)
		}
		runtime.run_host_jobs(ctx)
		if runtime.wait_condition_met() {
			return nil
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if page.runtime_finalizer != nil {
		if err := page.runtime_finalizer(runtime.vm, page); err != nil {
			return fmt.Errorf("minib: finalize site runtime: %w", err)
		}
	}
	if runtime.wait_condition_met() {
		return nil
	}
	if runtime.use_custom_runtime {
		runtime.pump_event_loop(ctx)
		return nil
	}
	runtime.ready_state = "interactive"
	runtime.fire_document_event("DOMContentLoaded")
	har_recorder_from_context(ctx).mark_content_loaded()
	if runtime.wait_condition_met() {
		return nil
	}
	if runtime.wait_until == WaitUntilDOMContentLoaded && !runtime.wait_active {
		return nil
	}
	runtime.pump_event_loop(ctx)
	if runtime.wait_condition_met() {
		return nil
	}
	runtime.ready_state = "complete"
	runtime.fire_document_event("readystatechange")
	runtime.fire_window_event("load")
	har_recorder_from_context(ctx).mark_loaded()
	runtime.pump_event_loop(ctx)
	return nil
}

func (runtime *page_runtime) wait_condition_met() bool {
	if runtime == nil || !runtime.wait_active {
		return false
	}
	if !runtime.wait_matched {
		runtime.wait_matched = runtime.page.wait_condition_met()
	}
	return runtime.wait_matched
}

func (runtime *page_runtime) execute_job(ctx context.Context, job script_job) {
	source := job.inline
	if job.resource_index >= 0 {
		resource := runtime.page.Resources[job.resource_index]
		if resource.Err != nil {
			runtime.fail_script(job.source_url, fmt.Errorf("download: %w", resource.Err))
			return
		}
		if len(resource.Body) > max_script_size {
			runtime.fail_script(job.source_url, fmt.Errorf("script exceeds %d bytes", max_script_size))
			return
		}
		decoded, err := clawreq.DecodeText(resource.Body, resource.ContentType)
		if err != nil {
			runtime.fail_script(job.source_url, fmt.Errorf("decode: %w", err))
			return
		}
		source = decoded
	}
	runtime.current_script = job.node
	runtime.current_script_url = job.source_url
	if job.module_script {
		_, _, err := runtime.evaluate_module(ctx, job.source_url, &source)
		runtime.current_script = nil
		runtime.current_script_url = ""
		if err != nil {
			runtime.fail_script(job.source_url, err)
			return
		}
		if !runtime.use_custom_runtime {
			runtime.sync_named_elements()
		}
		return
	}
	_, err := runtime.run_javascript(ctx, job.source_url, source)
	runtime.current_script = nil
	runtime.current_script_url = ""
	if err != nil {
		runtime.fail_script(job.source_url, err)
		return
	}
	runtime.page.ExecutedScripts++
	if !runtime.use_custom_runtime {
		runtime.sync_named_elements()
	}
}

func (runtime *page_runtime) sync_named_elements() {
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			name := attribute(node, "id")
			if name == "" {
				name = attribute(node, "name")
			}
			value := runtime.vm.Get(name)
			if name != "" && (value == nil || goja.IsUndefined(value)) {
				_ = runtime.vm.Set(name, runtime.node_object(node))
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(runtime.page.Document)
}

func (runtime *page_runtime) fail_script(source_url string, err error) {
	if detailed_error, ok := err.(fmt.Stringer); ok {
		err = fmt.Errorf("%s", strings.TrimSpace(detailed_error.String()))
	}
	runtime.page.ScriptFailures = append(runtime.page.ScriptFailures, ScriptFailure{URL: source_url, Err: err})
}

func run_javascript(ctx context.Context, vm *goja.Runtime, source_url string, source string) (value goja.Value, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	program, err := compile_javascript(source_url, source)
	if err != nil {
		return nil, err
	}
	finished := make(chan struct{})
	interrupt_done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			vm.Interrupt(ctx.Err())
		case <-finished:
		}
		close(interrupt_done)
	}()
	defer func() {
		close(finished)
		<-interrupt_done
		vm.ClearInterrupt()
		if recovered := recover(); recovered != nil {
			value = nil
			err = javascript_panic_error(recovered)
		}
	}()
	return vm.RunProgram(program)
}

func call_javascript(ctx context.Context, vm *goja.Runtime, callback goja.Callable, this goja.Value, args ...goja.Value) (value goja.Value, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	finished := make(chan struct{})
	interrupt_done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			vm.Interrupt(ctx.Err())
		case <-finished:
		}
		close(interrupt_done)
	}()
	defer func() {
		close(finished)
		<-interrupt_done
		vm.ClearInterrupt()
		if recovered := recover(); recovered != nil {
			value = nil
			err = javascript_panic_error(recovered)
		}
	}()
	return callback(this, args...)
}

func (runtime *page_runtime) javascript_context(parent context.Context) (context.Context, context.CancelFunc) {
	if runtime == nil || runtime.javascript_timeout <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, runtime.javascript_timeout)
}

func (runtime *page_runtime) run_javascript(ctx context.Context, source_url string, source string) (goja.Value, error) {
	javascript_ctx, cancel := runtime.javascript_context(ctx)
	defer cancel()
	previous_ctx := runtime.ctx
	runtime.ctx = javascript_ctx
	defer func() { runtime.ctx = previous_ctx }()
	value, err := run_javascript(javascript_ctx, runtime.vm, source_url, source)
	if context_err := javascript_ctx.Err(); context_err != nil {
		return value, context_err
	}
	return value, err
}

func (runtime *page_runtime) call_javascript(ctx context.Context, callback goja.Callable, this goja.Value, args ...goja.Value) (goja.Value, error) {
	javascript_ctx, cancel := runtime.javascript_context(ctx)
	defer cancel()
	previous_ctx := runtime.ctx
	runtime.ctx = javascript_ctx
	defer func() { runtime.ctx = previous_ctx }()
	value, err := call_javascript(javascript_ctx, runtime.vm, callback, this, args...)
	if context_err := javascript_ctx.Err(); context_err != nil {
		return value, context_err
	}
	return value, err
}

func javascript_panic_error(recovered any) error {
	stack := runtime_debug.Stack()
	const max_javascript_panic_stack = 8 << 10
	if len(stack) > max_javascript_panic_stack {
		stack = append(append([]byte(nil), stack[:max_javascript_panic_stack]...), []byte("\n... stack truncated")...)
	}
	return fmt.Errorf("minib: JavaScript engine panic: %v\n%s", recovered, stack)
}

func compile_javascript(source_url string, source string) (*goja.Program, error) {
	program, compile_err := goja.Compile(source_url, source, false)
	if compile_err == nil {
		return program, nil
	}
	transform_options := api.TransformOptions{
		Sourcefile: source_url,
		Loader:     api.LoaderJS,
		Target:     api.ES2017,
		Charset:    api.CharsetUTF8,
	}
	transformed := api.Transform(source, transform_options)
	if len(transformed.Errors) > 0 {
		return nil, compile_err
	}
	transformed_source := strings.ReplaceAll(string(transformed.Code), "import(", "__minib_import(")
	program, transformed_err := goja.Compile(source_url, transformed_source, false)
	if transformed_err == nil {
		return program, nil
	}
	transform_options.Format = api.FormatIIFE
	transformed = api.Transform(source, transform_options)
	if len(transformed.Errors) > 0 {
		return nil, transformed_err
	}
	transformed_source = strings.ReplaceAll(string(transformed.Code), "import(", "__minib_import(")
	return goja.Compile(source_url, transformed_source, false)
}

func (runtime *page_runtime) install() error {
	constructors := `
function DOMException(message, name) { this.message = message === undefined ? '' : String(message); this.name = name === undefined ? 'Error' : String(name); this.code = DOMException[this.name.replace(/Error$/, '').replace(/([a-z])([A-Z])/g, '$1_$2').toUpperCase() + '_ERR'] || 0; this.stack = (new Error(this.message)).stack; }
DOMException.prototype = Object.create(Error.prototype);
Object.defineProperty(DOMException.prototype, 'constructor', { configurable: true, writable: true, value: DOMException });
Object.defineProperty(DOMException.prototype, Symbol.toStringTag, { value: 'DOMException' });
Object.assign(DOMException, { INDEX_SIZE_ERR: 1, DOMSTRING_SIZE_ERR: 2, HIERARCHY_REQUEST_ERR: 3, WRONG_DOCUMENT_ERR: 4, INVALID_CHARACTER_ERR: 5, NO_DATA_ALLOWED_ERR: 6, NO_MODIFICATION_ALLOWED_ERR: 7, NOT_FOUND_ERR: 8, NOT_SUPPORTED_ERR: 9, INUSE_ATTRIBUTE_ERR: 10, INVALID_STATE_ERR: 11, SYNTAX_ERR: 12, INVALID_MODIFICATION_ERR: 13, NAMESPACE_ERR: 14, INVALID_ACCESS_ERR: 15, VALIDATION_ERR: 16, TYPE_MISMATCH_ERR: 17, SECURITY_ERR: 18, NETWORK_ERR: 19, ABORT_ERR: 20, URL_MISMATCH_ERR: 21, QUOTA_EXCEEDED_ERR: 22, TIMEOUT_ERR: 23, INVALID_NODE_TYPE_ERR: 24, DATA_CLONE_ERR: 25 });
var __minib_generic_event_targets = new WeakMap();
function EventTarget() { __minib_generic_event_targets.set(this, Object.create(null)); }
function __minib_generic_event_list(target, type, create) {
  var listeners = __minib_generic_event_targets.get(target);
  if (!listeners && create) { listeners = Object.create(null); __minib_generic_event_targets.set(target, listeners); }
  if (listeners && !listeners[type] && create) listeners[type] = [];
  return listeners && listeners[type];
}
EventTarget.prototype.addEventListener = function(type, callback, options) {
  if (this.__minib_addEventListener) return this.__minib_addEventListener.apply(this, arguments);
  if (this instanceof Node && typeof __minib_node_call === 'function') return __minib_node_call(this, 'addEventListener', Array.prototype.slice.call(arguments));
  if (callback == null || typeof callback !== 'function' && typeof callback.handleEvent !== 'function') return;
  type = String(type); var capture = typeof options === 'boolean' ? options : !!(options && options.capture);
  var listeners = __minib_generic_event_list(this, type, true);
  if (listeners.some(function(listener) { return listener.callback === callback && listener.capture === capture; })) return;
  listeners.push({ callback: callback, capture: capture, once: !!(options && options.once), passive: !!(options && options.passive) });
};
EventTarget.prototype.removeEventListener = function(type, callback, options) {
  if (this.__minib_removeEventListener) return this.__minib_removeEventListener.apply(this, arguments);
  if (this instanceof Node && typeof __minib_node_call === 'function') return __minib_node_call(this, 'removeEventListener', Array.prototype.slice.call(arguments));
  var listeners = __minib_generic_event_list(this, String(type), false); if (!listeners) return;
  var capture = typeof options === 'boolean' ? options : !!(options && options.capture);
  for (var index = 0; index < listeners.length; index++) if (listeners[index].callback === callback && listeners[index].capture === capture) { listeners.splice(index, 1); return; }
};
EventTarget.prototype.dispatchEvent = function(event) {
  if (this.__minib_dispatchEvent) return this.__minib_dispatchEvent.apply(this, arguments);
  if (this instanceof Node && typeof __minib_node_call === 'function') return __minib_node_call(this, 'dispatchEvent', Array.prototype.slice.call(arguments));
  if (!event || typeof event.type === 'undefined') throw new TypeError('dispatchEvent requires an Event');
  if (event.type === '') throw new DOMException('The event type is empty', 'InvalidStateError');
  event.target = this; event.currentTarget = this; event.eventPhase = Event.AT_TARGET;
  var listeners = (__minib_generic_event_list(this, String(event.type), false) || []).slice();
  for (var index = 0; index < listeners.length; index++) {
    var listener = listeners[index], current = __minib_generic_event_list(this, String(event.type), false) || [];
    if (current.indexOf(listener) < 0) continue;
    if (listener.once) this.removeEventListener(event.type, listener.callback, listener.capture);
    event.__minib_passive = listener.passive;
    if (typeof listener.callback === 'function') listener.callback.call(this, event); else listener.callback.handleEvent.call(listener.callback, event);
    event.__minib_passive = false;
    if (event.__minib_immediate_stopped) break;
  }
  var handler = this['on' + event.type];
  if (!event.__minib_immediate_stopped && typeof handler === 'function') handler.call(this, event);
  event.currentTarget = null; event.eventPhase = Event.NONE;
  return !event.defaultPrevented;
};
function Window() {}
Window.prototype = Object.create(EventTarget.prototype);
function Node() {}
Node.prototype = Object.create(EventTarget.prototype);
Object.assign(Node, {
  ELEMENT_NODE: 1, ATTRIBUTE_NODE: 2, TEXT_NODE: 3, CDATA_SECTION_NODE: 4,
  ENTITY_REFERENCE_NODE: 5, ENTITY_NODE: 6, PROCESSING_INSTRUCTION_NODE: 7,
  COMMENT_NODE: 8, DOCUMENT_NODE: 9, DOCUMENT_TYPE_NODE: 10,
  DOCUMENT_FRAGMENT_NODE: 11, NOTATION_NODE: 12,
  DOCUMENT_POSITION_DISCONNECTED: 1, DOCUMENT_POSITION_PRECEDING: 2,
  DOCUMENT_POSITION_FOLLOWING: 4, DOCUMENT_POSITION_CONTAINS: 8,
  DOCUMENT_POSITION_CONTAINED_BY: 16, DOCUMENT_POSITION_IMPLEMENTATION_SPECIFIC: 32
});
Object.keys(Node).forEach(function(name) { if (name.indexOf('_NODE') >= 0 || name.indexOf('DOCUMENT_POSITION_') === 0) Node.prototype[name] = Node[name]; });
function __minibOwnAccessor(name, writable) {
  var descriptor = { configurable: true, enumerable: true, get: function() { var own = Object.getOwnPropertyDescriptor(this, name); return own ? (own.get ? own.get.call(this) : own.value) : undefined; } };
  if (writable) descriptor.set = function(value) { var own = Object.getOwnPropertyDescriptor(this, name); if (own) { if (own.set) own.set.call(this, value); else if (own.writable) { own.value = value; Object.defineProperty(this, name, own); } } else Object.defineProperty(this, name, { configurable: true, enumerable: true, writable: true, value: value }); };
  return descriptor;
}
function __minibNodeAccessor(name, writable) {
  var descriptor = { configurable: true, enumerable: true, get: function() { return __minib_node_get(this, name); } };
  if (writable) descriptor.set = function(value) { __minib_node_set(this, name, value); };
  return descriptor;
}
['abort', 'auxclick', 'beforeinput', 'blur', 'change', 'click', 'close', 'contextmenu', 'dblclick', 'error', 'focus', 'focusin', 'focusout', 'input', 'keydown', 'keypress', 'keyup', 'load', 'mousedown', 'mouseenter', 'mouseleave', 'mousemove', 'mouseout', 'mouseover', 'mouseup', 'pointerdown', 'pointermove', 'pointerup', 'reset', 'resize', 'scroll', 'select', 'submit', 'touchcancel', 'touchend', 'touchmove', 'touchstart', 'wheel'].forEach(function(name) {
  Object.defineProperty(EventTarget.prototype, 'on' + name, __minibOwnAccessor('on' + name, true));
});
['nodeType', 'nodeName', 'parentNode', 'parentElement', 'firstChild', 'lastChild', 'nextSibling', 'previousSibling', 'childNodes', 'ownerDocument', 'isConnected', 'innerHTML', 'outerHTML'].forEach(function(name) { Object.defineProperty(Node.prototype, name, __minibNodeAccessor(name, false)); });
['textContent', 'nodeValue'].forEach(function(name) { Object.defineProperty(Node.prototype, name, __minibNodeAccessor(name, true)); });
var NodeFilter = { FILTER_ACCEPT: 1, FILTER_REJECT: 2, FILTER_SKIP: 3, SHOW_ALL: 4294967295, SHOW_ELEMENT: 1, SHOW_TEXT: 4, SHOW_COMMENT: 128 };
function TreeWalker(root, whatToShow, filter) { this.root = root; this.whatToShow = whatToShow == null ? NodeFilter.SHOW_ALL : whatToShow; this.filter = filter; this.currentNode = root; }
TreeWalker.prototype._accept = function(node) {
  if (this.whatToShow !== NodeFilter.SHOW_ALL && !(this.whatToShow & (1 << (node.nodeType - 1)))) return false;
  if (!this.filter) return true;
  var result = typeof this.filter === 'function' ? this.filter(node) : this.filter.acceptNode(node);
  return result === NodeFilter.FILTER_ACCEPT;
};
TreeWalker.prototype.nextNode = function() {
  var node = this.currentNode;
  while (node) {
    if (node.firstChild) node = node.firstChild;
    else {
      while (node && node !== this.root && !node.nextSibling) node = node.parentNode;
      if (!node || node === this.root) return null;
      node = node.nextSibling;
    }
    if (this._accept(node)) { this.currentNode = node; return node; }
  }
  return null;
};
function CharacterData() {}
CharacterData.prototype = Object.create(Node.prototype);
Object.defineProperty(CharacterData.prototype, 'data', __minibNodeAccessor('data', true));
function Text() {}
Text.prototype = Object.create(CharacterData.prototype);
function Comment() {}
Comment.prototype = Object.create(CharacterData.prototype);
function CDATASection() {}
CDATASection.prototype = Object.create(CharacterData.prototype);
function ProcessingInstruction() {}
ProcessingInstruction.prototype = Object.create(CharacterData.prototype);
function DocumentType() {}
DocumentType.prototype = Object.create(Node.prototype);
function Element() {}
Element.prototype = Object.create(Node.prototype);
['children', 'tagName', 'localName', 'style', 'sheet', 'classList', 'dataset', 'attributes', 'contentWindow', 'contentDocument', 'content', 'shadowRoot', 'protocol', 'host', 'hostname', 'port', 'pathname', 'search', 'hash', 'origin', 'clientWidth', 'clientHeight', 'offsetWidth', 'offsetHeight', 'scrollWidth', 'scrollHeight', 'scrollTop', 'scrollLeft'].forEach(function(name) { Object.defineProperty(Element.prototype, name, __minibNodeAccessor(name, false)); });
['id', 'className', 'src', 'href', 'value', 'name', 'type', 'rel', 'content', 'charset', 'innerHTML'].forEach(function(name) { Object.defineProperty(Element.prototype, name, __minibNodeAccessor(name, true)); });
function __minibMethod(name) { return function() { var own = this['__minib_' + name]; if (typeof own === 'function') return own.apply(this, arguments); return __minib_node_call(this, name, Array.prototype.slice.call(arguments)); }; }
['appendChild', 'removeChild', 'replaceChild', 'insertBefore', 'cloneNode', 'contains'].forEach(function(name) { Node.prototype[name] = __minibMethod(name); });
Node.prototype.hasChildNodes = function() { return this.firstChild !== null; };
Node.prototype.getRootNode = function() { var node = this; while (node.parentNode) node = node.parentNode; return node; };
Node.prototype.isSameNode = function(other) { return this === other; };
Node.prototype.isEqualNode = function(other) {
  if (other == null || this.nodeType !== other.nodeType || this.nodeName !== other.nodeName || this.nodeValue !== other.nodeValue) return false;
  if (this.nodeType === Node.ELEMENT_NODE) {
    var names = this.getAttributeNames(), otherNames = other.getAttributeNames();
    if (names.length !== otherNames.length) return false;
    for (var index = 0; index < names.length; index++) if (!other.hasAttribute(names[index]) || this.getAttribute(names[index]) !== other.getAttribute(names[index])) return false;
  }
  var child = this.firstChild, otherChild = other.firstChild;
  while (child && otherChild) { if (!child.isEqualNode(otherChild)) return false; child = child.nextSibling; otherChild = otherChild.nextSibling; }
  return child === null && otherChild === null;
};
Node.prototype.normalize = function() {
  var child = this.firstChild;
  while (child) {
    var next = child.nextSibling;
    if (child.nodeType === Node.TEXT_NODE) {
      while (next && next.nodeType === Node.TEXT_NODE) { child.data += next.data; this.removeChild(next); next = child.nextSibling; }
      if (child.data === '') this.removeChild(child);
    } else child.normalize();
    child = next;
  }
};
['insertAdjacentElement', 'getAttribute', 'setAttribute', 'getAttributeNS', 'setAttributeNS', 'removeAttribute', 'removeAttributeNS', 'hasAttribute', 'hasAttributeNS', 'hasAttributes', 'getAttributeNames', 'querySelector', 'querySelectorAll', 'getElementsByTagName', 'getElementsByClassName', 'matches', 'closest', 'attachShadow', 'getBoundingClientRect', 'getClientRects', 'focus', 'blur', 'click', 'getContext', 'toDataURL'].forEach(function(name) { Element.prototype[name] = __minibMethod(name); });
function HTMLElement() { if (typeof __minib_construct_html_element === 'function') return __minib_construct_html_element(this); }
HTMLElement.prototype = Object.create(Element.prototype);
HTMLElement.prototype.select = function() {};
function CustomElementRegistry() {}
function HTMLBodyElement() {}
HTMLBodyElement.prototype = Object.create(HTMLElement.prototype);
Object.defineProperty(HTMLBodyElement.prototype, Symbol.toStringTag, { value: 'HTMLBodyElement' });
function HTMLHtmlElement() {}
HTMLHtmlElement.prototype = Object.create(HTMLElement.prototype);
Object.defineProperty(HTMLHtmlElement.prototype, Symbol.toStringTag, { value: 'HTMLHtmlElement' });
function HTMLImageElement() {}
HTMLImageElement.prototype = Object.create(HTMLElement.prototype);
Object.defineProperty(HTMLImageElement.prototype, Symbol.toStringTag, { value: 'HTMLImageElement' });
function HTMLIFrameElement() {}
HTMLIFrameElement.prototype = Object.create(HTMLElement.prototype);
Object.defineProperty(HTMLIFrameElement.prototype, Symbol.toStringTag, { value: 'HTMLIFrameElement' });
function HTMLTemplateElement() {}
HTMLTemplateElement.prototype = Object.create(HTMLElement.prototype);
Object.defineProperty(HTMLTemplateElement.prototype, Symbol.toStringTag, { value: 'HTMLTemplateElement' });
var HTMLDivElement = HTMLElement, HTMLSpanElement = HTMLElement, HTMLButtonElement = HTMLElement, HTMLAnchorElement = HTMLElement, HTMLInputElement = HTMLElement, HTMLTextAreaElement = HTMLElement, HTMLSelectElement = HTMLElement, HTMLOptionElement = HTMLElement, HTMLFormElement = HTMLElement, HTMLLabelElement = HTMLElement, HTMLStyleElement = HTMLElement, HTMLScriptElement = HTMLElement, HTMLLinkElement = HTMLElement, HTMLCanvasElement = HTMLElement, HTMLParagraphElement = HTMLElement, HTMLHeadingElement = HTMLElement, HTMLUListElement = HTMLElement, HTMLLIElement = HTMLElement;
function HTMLMediaElement() {}
HTMLMediaElement.prototype = Object.create(HTMLElement.prototype);
HTMLMediaElement.prototype.canPlayType = function() { return 'probably'; };
HTMLMediaElement.prototype.load = function() {};
HTMLMediaElement.prototype.pause = function() { this.paused = true; };
HTMLMediaElement.prototype.play = function() { this.paused = false; return Promise.resolve(); };
function HTMLAudioElement() {}
HTMLAudioElement.prototype = Object.create(HTMLMediaElement.prototype);
Object.defineProperty(HTMLAudioElement.prototype, Symbol.toStringTag, { value: 'HTMLAudioElement' });
function HTMLVideoElement() {}
HTMLVideoElement.prototype = Object.create(HTMLMediaElement.prototype);
Object.defineProperty(HTMLVideoElement.prototype, Symbol.toStringTag, { value: 'HTMLVideoElement' });
function MediaSource() {}
MediaSource.isTypeSupported = function() { return false; };
function MessagePort() { EventTarget.call(this); this.onmessage = null; this.onmessageerror = null; this._target = null; }
MessagePort.prototype = Object.create(EventTarget.prototype);
MessagePort.prototype.postMessage = function(data) { if (typeof __minib_post_message_port === 'function') __minib_post_message_port(this._target, data); };
MessagePort.prototype.start = function() {};
MessagePort.prototype.close = function() { this._target = null; };
function MessageChannel() { this.port1 = new MessagePort(); this.port2 = new MessagePort(); this.port1._target = this.port2; this.port2._target = this.port1; }
function queueMicrotask(callback) { Promise.resolve().then(callback); }
function SVGElement() {}
SVGElement.prototype = Object.create(Element.prototype);
function DOMTokenList() {}
DOMTokenList.prototype.item = function(index) { return index >= 0 && index < this.length ? this[index] : null; };
DOMTokenList.prototype.contains = function(token) { return Array.prototype.indexOf.call(this, String(token)) >= 0; };
DOMTokenList.prototype.forEach = function(callback, thisArg) { for (var index = 0; index < this.length; index++) callback.call(thisArg, this[index], this[index], this); };
DOMTokenList.prototype.entries = function() { return Array.prototype.slice.call(this).map(function(value, index) { return [index, value]; })[Symbol.iterator](); };
DOMTokenList.prototype.keys = function() { return Array.prototype.keys.call(Array.prototype.slice.call(this)); };
DOMTokenList.prototype.values = function() { return Array.prototype.values.call(Array.prototype.slice.call(this)); };
DOMTokenList.prototype[Symbol.iterator] = DOMTokenList.prototype.values;
Object.defineProperty(DOMTokenList.prototype, Symbol.toStringTag, { value: 'DOMTokenList' });
function Document() {}
Document.prototype = Object.create(Node.prototype);
Object.defineProperty(Document.prototype, 'adoptedStyleSheets', __minibNodeAccessor('adoptedStyleSheets', true));
['createElement', 'createElementNS', 'createTextNode', 'createDocumentFragment', 'createComment', 'createEvent', 'createRange', 'importNode', 'getElementById', 'getElementsByName', 'getElementsByTagName', 'getElementsByClassName', 'querySelector', 'querySelectorAll'].forEach(function(name) { Document.prototype[name] = __minibMethod(name); });
Document.prototype.createTreeWalker = function(root, whatToShow, filter) { return new TreeWalker(root, whatToShow, filter); };
function DOMParser() {}
DOMParser.prototype.parseFromString = function(markup, mimeType) {
  mimeType = String(mimeType).toLowerCase();
  if (['text/html', 'text/xml', 'application/xml', 'application/xhtml+xml', 'image/svg+xml'].indexOf(mimeType) < 0) throw new TypeError('Unsupported DOMParser mime type');
  return __minib_parse_document(String(markup), mimeType);
};
function HTMLDocument() {}
HTMLDocument.prototype = Object.create(Document.prototype);
Object.defineProperty(HTMLDocument.prototype, Symbol.toStringTag, { value: 'HTMLDocument' });
function DocumentFragment() {}
DocumentFragment.prototype = Object.create(Node.prototype);
Object.defineProperty(DocumentFragment.prototype, 'children', __minibNodeAccessor('children', false));
['querySelector', 'querySelectorAll'].forEach(function(name) { DocumentFragment.prototype[name] = __minibMethod(name); });
function ShadowRoot() {}
ShadowRoot.prototype = Object.create(DocumentFragment.prototype);
Object.defineProperty(ShadowRoot.prototype, 'host', __minibNodeAccessor('host', false));
Object.defineProperty(ShadowRoot.prototype, 'mode', __minibNodeAccessor('mode', false));
Object.defineProperty(ShadowRoot.prototype, 'adoptedStyleSheets', __minibNodeAccessor('adoptedStyleSheets', true));
function __minibConvertNodes(owner, values) {
  var documentForNodes = owner.nodeType === Node.DOCUMENT_NODE ? owner : owner.ownerDocument;
  return Array.prototype.map.call(values, function(value) { return value instanceof Node ? value : documentForNodes.createTextNode(String(value)); });
}
function __minibInstallParentNode(prototype) {
  prototype.append = function() { __minibConvertNodes(this, arguments).forEach(function(node) { this.appendChild(node); }, this); };
  prototype.prepend = function() { var mark = this.firstChild; __minibConvertNodes(this, arguments).forEach(function(node) { this.insertBefore(node, mark); }, this); };
  prototype.replaceChildren = function() { var nodes = __minibConvertNodes(this, arguments); while (this.firstChild) this.removeChild(this.firstChild); nodes.forEach(function(node) { this.appendChild(node); }, this); };
}
[Element.prototype, Document.prototype, DocumentFragment.prototype].forEach(__minibInstallParentNode);
function __minibInstallChildNode(prototype) {
  prototype.before = function() { if (!this.parentNode) return; var parent = this.parentNode; __minibConvertNodes(this, arguments).forEach(function(node) { parent.insertBefore(node, this); }, this); };
  prototype.after = function() { if (!this.parentNode) return; var parent = this.parentNode, mark = this.nextSibling; __minibConvertNodes(this, arguments).forEach(function(node) { parent.insertBefore(node, mark); }); };
  prototype.replaceWith = function() { if (!this.parentNode) return; var parent = this.parentNode; __minibConvertNodes(this, arguments).forEach(function(node) { parent.insertBefore(node, this); }, this); if (this.parentNode === parent) parent.removeChild(this); };
  prototype.remove = function() { if (this.parentNode) this.parentNode.removeChild(this); };
}
[Element.prototype, CharacterData.prototype, DocumentType.prototype].forEach(__minibInstallChildNode);
Object.defineProperty(CharacterData.prototype, 'length', { configurable: true, enumerable: true, get: function() { return this.data.length; } });
CharacterData.prototype.substringData = function(offset, count) { offset = Number(offset); count = Number(count); if (offset < 0 || offset > this.length) throw new DOMException('Offset is outside the data', 'IndexSizeError'); return this.data.slice(offset, offset + Math.max(0, count)); };
CharacterData.prototype.appendData = function(data) { this.data += String(data); };
CharacterData.prototype.insertData = function(offset, data) { if (offset < 0 || offset > this.length) throw new DOMException('Offset is outside the data', 'IndexSizeError'); this.data = this.data.slice(0, offset) + String(data) + this.data.slice(offset); };
CharacterData.prototype.deleteData = function(offset, count) { if (offset < 0 || offset > this.length) throw new DOMException('Offset is outside the data', 'IndexSizeError'); this.data = this.data.slice(0, offset) + this.data.slice(offset + Math.max(0, Number(count))); };
CharacterData.prototype.replaceData = function(offset, count, data) { if (offset < 0 || offset > this.length) throw new DOMException('Offset is outside the data', 'IndexSizeError'); this.data = this.data.slice(0, offset) + String(data) + this.data.slice(offset + Math.max(0, Number(count))); };
Text.prototype.splitText = function(offset) { if (offset < 0 || offset > this.length) throw new DOMException('Offset is outside the data', 'IndexSizeError'); var sibling = this.ownerDocument.createTextNode(this.data.slice(offset)); this.data = this.data.slice(0, offset); if (this.parentNode) this.parentNode.insertBefore(sibling, this.nextSibling); return sibling; };
Object.defineProperty(Text.prototype, 'wholeText', { configurable: true, enumerable: true, get: function() { var text = this.data, node = this.previousSibling; while (node && node.nodeType === Node.TEXT_NODE) { text = node.data + text; node = node.previousSibling; } node = this.nextSibling; while (node && node.nodeType === Node.TEXT_NODE) { text += node.data; node = node.nextSibling; } return text; } });
Element.prototype.toggleAttribute = function(name, force) { var present = this.hasAttribute(name); if (arguments.length > 1) { if (force && !present) this.setAttribute(name, ''); else if (!force && present) this.removeAttribute(name); return !!force; } if (present) this.removeAttribute(name); else this.setAttribute(name, ''); return !present; };
Element.prototype.insertAdjacentText = function(position, data) { var text = this.ownerDocument.createTextNode(String(data)); return this.insertAdjacentElement(position, text); };
Element.prototype.insertAdjacentHTML = function(position, markup) { var range = this.ownerDocument.createRange(); range.selectNode(this); var fragment = range.createContextualFragment(String(markup)), normalized = String(position).toLowerCase(); if (normalized === 'beforebegin') { if (!this.parentNode) return; this.parentNode.insertBefore(fragment, this); } else if (normalized === 'afterbegin') this.insertBefore(fragment, this.firstChild); else if (normalized === 'beforeend') this.appendChild(fragment); else if (normalized === 'afterend') { if (!this.parentNode) return; this.parentNode.insertBefore(fragment, this.nextSibling); } else throw new DOMException('Invalid insertion position', 'SyntaxError'); };
function Range() {}
function NodeList() {}
NodeList.prototype = Object.create(Array.prototype);
function HTMLCollection() {}
HTMLCollection.prototype = Object.create(Array.prototype);
function Location() {}
Object.defineProperty(Location.prototype, Symbol.toStringTag, { value: 'Location' });
function FormData() { this._entries = []; }
FormData.prototype.append = function(name, value) { this._entries.push([String(name), value]); };
FormData.prototype.set = function(name, value) { this.delete(name); this.append(name, value); };
FormData.prototype.get = function(name) { var values = this.getAll(name); return values.length ? values[0] : null; };
FormData.prototype.getAll = function(name) { name = String(name); return this._entries.filter(function(entry) { return entry[0] === name; }).map(function(entry) { return entry[1]; }); };
FormData.prototype.has = function(name) { return this.getAll(name).length > 0; };
FormData.prototype.delete = function(name) { name = String(name); this._entries = this._entries.filter(function(entry) { return entry[0] !== name; }); };
FormData.prototype.forEach = function(callback, thisArg) { this._entries.forEach(function(entry) { callback.call(thisArg, entry[1], entry[0], this); }, this); };
FormData.prototype.entries = function() { return this._entries[Symbol.iterator](); };
FormData.prototype.keys = function() { return this._entries.map(function(entry) { return entry[0]; })[Symbol.iterator](); };
FormData.prototype.values = function() { return this._entries.map(function(entry) { return entry[1]; })[Symbol.iterator](); };
FormData.prototype[Symbol.iterator] = FormData.prototype.entries;
Object.defineProperty(FormData.prototype, Symbol.toStringTag, { value: 'FormData' });
function Event(type, init) { if (arguments.length < 1) throw new TypeError('Event requires a type'); init = init || {}; this.type = String(type); this.bubbles = !!init.bubbles; this.cancelable = !!init.cancelable; this.composed = !!init.composed; this.defaultPrevented = false; this.target = null; this.currentTarget = null; this.eventPhase = 0; this.isTrusted = false; this.timeStamp = Date.now(); this.__minib_stopped = false; this.__minib_immediate_stopped = false; this.__minib_passive = false; }
var __minib_event_constructor = Event;
Event.NONE = Event.prototype.NONE = 0; Event.CAPTURING_PHASE = Event.prototype.CAPTURING_PHASE = 1; Event.AT_TARGET = Event.prototype.AT_TARGET = 2; Event.BUBBLING_PHASE = Event.prototype.BUBBLING_PHASE = 3;
Event.prototype.preventDefault = function() { if (this.cancelable && !this.__minib_passive) this.defaultPrevented = true; };
Event.prototype.stopPropagation = function() { this.__minib_stopped = true; };
Event.prototype.stopImmediatePropagation = function() { this.__minib_stopped = true; this.__minib_immediate_stopped = true; };
Event.prototype.composedPath = function() { return []; };
Event.prototype.initEvent = function(type, bubbles, cancelable) { this.type = String(type); this.bubbles = !!bubbles; this.cancelable = !!cancelable; this.composed = false; this.defaultPrevented = false; this.__minib_stopped = false; this.__minib_immediate_stopped = false; };
['type', 'bubbles', 'cancelable', 'defaultPrevented', 'target', 'currentTarget', 'composed'].forEach(function(name) { Object.defineProperty(Event.prototype, name, __minibOwnAccessor(name, true)); });
function UIEvent(type, init) { init = init || {}; __minib_event_constructor.call(this, type, init); this.view = init.view || null; this.detail = Number(init.detail || 0); }
UIEvent.prototype = Object.create(Event.prototype);
function MouseEvent(type, init) { init = init || {}; UIEvent.call(this, type, init); this.screenX = Number(init.screenX || 0); this.screenY = Number(init.screenY || 0); this.clientX = Number(init.clientX || 0); this.clientY = Number(init.clientY || 0); this.ctrlKey = !!init.ctrlKey; this.shiftKey = !!init.shiftKey; this.altKey = !!init.altKey; this.metaKey = !!init.metaKey; this.button = Number(init.button || 0); this.buttons = Number(init.buttons || 0); this.relatedTarget = init.relatedTarget || null; }
MouseEvent.prototype = Object.create(UIEvent.prototype);
function KeyboardEvent(type, init) { init = init || {}; UIEvent.call(this, type, init); this.key = String(init.key || ''); this.code = String(init.code || ''); this.location = Number(init.location || 0); this.ctrlKey = !!init.ctrlKey; this.shiftKey = !!init.shiftKey; this.altKey = !!init.altKey; this.metaKey = !!init.metaKey; this.repeat = !!init.repeat; this.isComposing = !!init.isComposing; this.charCode = Number(init.charCode || 0); this.keyCode = Number(init.keyCode || (this.key === 'Enter' ? 13 : 0)); this.which = Number(init.which || this.keyCode); }
KeyboardEvent.prototype = Object.create(UIEvent.prototype);
function FocusEvent(type, init) { init = init || {}; UIEvent.call(this, type, init); this.relatedTarget = init.relatedTarget || null; }
FocusEvent.prototype = Object.create(UIEvent.prototype);
function InputEvent(type, init) { init = init || {}; UIEvent.call(this, type, init); this.data = init.data === undefined ? null : init.data; this.inputType = String(init.inputType || ''); this.isComposing = !!init.isComposing; }
InputEvent.prototype = Object.create(UIEvent.prototype);
function WheelEvent(type, init) { init = init || {}; MouseEvent.call(this, type, init); this.deltaX = Number(init.deltaX || 0); this.deltaY = Number(init.deltaY || 0); this.deltaZ = Number(init.deltaZ || 0); this.deltaMode = Number(init.deltaMode || 0); }
WheelEvent.prototype = Object.create(MouseEvent.prototype);
function PointerEvent(type, init) { init = init || {}; MouseEvent.call(this, type, init); this.pointerId = Number(init.pointerId || 0); this.width = Number(init.width || 1); this.height = Number(init.height || 1); this.pressure = Number(init.pressure || 0); this.tangentialPressure = Number(init.tangentialPressure || 0); this.tiltX = Number(init.tiltX || 0); this.tiltY = Number(init.tiltY || 0); this.twist = Number(init.twist || 0); this.pointerType = String(init.pointerType || ''); this.isPrimary = !!init.isPrimary; }
PointerEvent.prototype = Object.create(MouseEvent.prototype);
function CustomEvent(type, init) { __minib_event_constructor.call(this, type, init); this.detail = init && init.detail; }
CustomEvent.prototype = Object.create(Event.prototype);
CustomEvent.prototype.initCustomEvent = function(type, bubbles, cancelable, detail) { this.initEvent(type, bubbles, cancelable); this.detail = detail; };
function MessageEvent(type, init) { __minib_event_constructor.call(this, type, init); init = init || {}; this.data = init.data === undefined ? null : init.data; this.origin = String(init.origin || ''); this.lastEventId = String(init.lastEventId || ''); this.source = init.source || null; this.ports = init.ports || []; }
MessageEvent.prototype = Object.create(Event.prototype);
function PromiseRejectionEvent(type, init) { __minib_event_constructor.call(this, type, init); init = init || {}; this.promise = init.promise; this.reason = init.reason; }
PromiseRejectionEvent.prototype = Object.create(Event.prototype);
Object.defineProperty(PromiseRejectionEvent.prototype, 'constructor', { configurable: true, writable: true, value: PromiseRejectionEvent });
Object.defineProperty(PromiseRejectionEvent.prototype, Symbol.toStringTag, { value: 'PromiseRejectionEvent' });
function StyleSheet() {}
function CSSStyleSheet() { if (typeof __minib_construct_css_style_sheet === 'function') return __minib_construct_css_style_sheet(); }
CSSStyleSheet.prototype = Object.create(StyleSheet.prototype);
function MediaList() {}
function StyleSheetList() {}
function CSSRuleList() {}
function CSSRule() {}
CSSRule.STYLE_RULE = CSSRule.prototype.STYLE_RULE = 1; CSSRule.IMPORT_RULE = CSSRule.prototype.IMPORT_RULE = 3; CSSRule.MEDIA_RULE = CSSRule.prototype.MEDIA_RULE = 4; CSSRule.FONT_FACE_RULE = CSSRule.prototype.FONT_FACE_RULE = 5; CSSRule.KEYFRAMES_RULE = CSSRule.prototype.KEYFRAMES_RULE = 7; CSSRule.SUPPORTS_RULE = CSSRule.prototype.SUPPORTS_RULE = 12; CSSRule.LAYER_BLOCK_RULE = CSSRule.prototype.LAYER_BLOCK_RULE = 18;
function CSSStyleRule() {}
CSSStyleRule.prototype = Object.create(CSSRule.prototype);
function CSSMediaRule() {}
CSSMediaRule.prototype = Object.create(CSSRule.prototype);
function CSSStyleDeclaration() { throw new TypeError('Illegal constructor'); }
function __minib_css_property_name(name) {
  name = String(name);
  if (name === 'cssFloat') return 'float';
  if (name.slice(0, 2) === '--') return name;
  return name.replace(/[A-Z]/g, function(character) { return '-' + character.toLowerCase(); }).toLowerCase();
}
CSSStyleDeclaration.prototype.getPropertyValue = function(name) { return this.__bridge.get(__minib_css_property_name(name)); };
CSSStyleDeclaration.prototype.getPropertyPriority = function(name) { return this.__bridge.priority(__minib_css_property_name(name)); };
CSSStyleDeclaration.prototype.setProperty = function(name, value, priority) { return this.__bridge.set(__minib_css_property_name(name), value, priority || ''); };
CSSStyleDeclaration.prototype.removeProperty = function(name) { return this.__bridge.remove(__minib_css_property_name(name)); };
CSSStyleDeclaration.prototype.item = function(index) { return this.__bridge.item(Number(index) || 0); };
Object.defineProperty(CSSStyleDeclaration.prototype, 'cssText', { configurable: true, enumerable: true, get: function() { return this.__bridge.cssText(); }, set: function(value) { this.__bridge.setCssText(String(value)); } });
Object.defineProperty(CSSStyleDeclaration.prototype, 'length', { configurable: true, enumerable: true, get: function() { return this.__bridge.length(); } });
function __minib_create_style_declaration(bridge) {
  var target = Object.create(CSSStyleDeclaration.prototype);
  Object.defineProperty(target, '__bridge', { configurable: true, value: bridge });
  return new Proxy(target, {
    get: function(object, property, receiver) {
      if (typeof property !== 'string' || property in object) return Reflect.get(object, property, receiver);
      if (/^\d+$/.test(property)) return object.item(Number(property));
      return object.getPropertyValue(property);
    },
    set: function(object, property, value, receiver) {
      if (property === 'cssText') { object.cssText = value; return true; }
      if (typeof property !== 'string' || property in object) return Reflect.set(object, property, value, receiver);
      if (!/^\d+$/.test(property)) object.setProperty(property, value, '');
      return true;
    },
    has: function(object, property) { return typeof property === 'string' && !/^\d+$/.test(property) || Reflect.has(object, property); },
    ownKeys: function(object) { var keys = []; for (var index = 0; index < object.length; index++) keys.push(String(index)); return keys; },
    getOwnPropertyDescriptor: function(object, property) {
      if (typeof property === 'string' && /^\d+$/.test(property) && Number(property) < object.length) return { configurable: true, enumerable: true, value: object.item(Number(property)) };
      return Reflect.getOwnPropertyDescriptor(object, property);
    }
  });
}
function MutationObserver(callback) { this.callback = callback; }
MutationObserver.prototype.observe = function(target) { target.__minib_mutation_callback = this.callback; target.__minib_mutation_observer = this; this.target = target; };
MutationObserver.prototype.disconnect = function() { if (this.target) { delete this.target.__minib_mutation_callback; delete this.target.__minib_mutation_observer; } this.target = null; };
MutationObserver.prototype.takeRecords = function() { return []; };
function IntersectionObserver(callback) { this.callback = callback; this.targets = []; }
IntersectionObserver.prototype.observe = function(target) { var self = this; this.targets.push(target); setTimeout(function() { var rect = target.getBoundingClientRect(); self.callback([{ target: target, isIntersecting: true, intersectionRatio: 1, boundingClientRect: rect, intersectionRect: rect, rootBounds: rect, time: performance.now() }], self); }, 0); };
IntersectionObserver.prototype.unobserve = function(target) { this.targets = this.targets.filter(function(item) { return item !== target; }); };
IntersectionObserver.prototype.disconnect = function() { this.targets = []; };
IntersectionObserver.prototype.takeRecords = function() { return []; };
function ResizeObserverEntry(target) { var rect = target.getBoundingClientRect(); this.target = target; this.contentRect = rect; this.borderBoxSize = [{ inlineSize: rect.width, blockSize: rect.height }]; this.contentBoxSize = [{ inlineSize: rect.width, blockSize: rect.height }]; this.devicePixelContentBoxSize = [{ inlineSize: rect.width, blockSize: rect.height }]; }
function ResizeObserver(callback) { if (typeof callback !== 'function') throw new TypeError('ResizeObserver callback must be a function'); this.callback = callback; this.targets = []; }
ResizeObserver.prototype.observe = function(target) { if (!target || typeof target.getBoundingClientRect !== 'function') throw new TypeError('ResizeObserver target must be an Element'); if (this.targets.indexOf(target) < 0) this.targets.push(target); var self = this; setTimeout(function() { if (self.targets.indexOf(target) >= 0) self.callback([new ResizeObserverEntry(target)], self); }, 0); };
ResizeObserver.prototype.unobserve = function(target) { this.targets = this.targets.filter(function(item) { return item !== target; }); };
ResizeObserver.prototype.disconnect = function() { this.targets = []; };
function PerformanceObserver(callback) { if (typeof callback !== 'function') throw new TypeError('PerformanceObserver callback must be a function'); this.callback = callback; }
PerformanceObserver.supportedEntryTypes = [];
PerformanceObserver.prototype.observe = function() {};
PerformanceObserver.prototype.disconnect = function() {};
PerformanceObserver.prototype.takeRecords = function() { return []; };
function Blob(parts, options) {
  var chunks = Array.from(parts || [], function(part) {
    if (part instanceof Blob) return part._text;
    if (part instanceof ArrayBuffer || ArrayBuffer.isView(part)) {
      var bytes = part instanceof ArrayBuffer ? new Uint8Array(part) : new Uint8Array(part.buffer, part.byteOffset, part.byteLength), text = '';
      for (var index = 0; index < bytes.length; index++) text += String.fromCharCode(bytes[index]);
      return text;
    }
    return String(part);
  });
  this._text = chunks.join('');
  this.size = new TextEncoder().encode(this._text).byteLength;
  this.type = String(options && options.type || '').toLowerCase();
}
Blob.prototype.text = function() { return Promise.resolve(this._text); };
Blob.prototype.arrayBuffer = function() { return Promise.resolve(new TextEncoder().encode(this._text).buffer); };
Blob.prototype.slice = function(start, end, type) { return new Blob([this._text.slice(start || 0, end == null ? this._text.length : end)], { type: type || '' }); };
function File(parts, name, options) { Blob.call(this, parts, options); this.name = String(name || ''); this.lastModified = options && options.lastModified || Date.now(); }
File.prototype = Object.create(Blob.prototype);
function TextEncoder() { this.encoding = 'utf-8'; }
TextEncoder.prototype.encode = function(input) {
  var text = unescape(encodeURIComponent(String(input === undefined ? '' : input))), bytes = new Uint8Array(text.length);
  for (var index = 0; index < text.length; index++) bytes[index] = text.charCodeAt(index);
  return bytes;
};
TextEncoder.prototype.encodeInto = function(input, destination) {
  var bytes = this.encode(input), written = Math.min(bytes.length, destination.length);
  destination.set(bytes.subarray(0, written));
  return { read: String(input === undefined ? '' : input).length, written: written };
};
function ReadableStreamDefaultController(stream) { this._stream = stream; }
Object.defineProperty(ReadableStreamDefaultController.prototype, 'desiredSize', { configurable: true, get: function() { return this._stream._state === 'readable' ? 1 - this._stream._queue.length : null; } });
ReadableStreamDefaultController.prototype.enqueue = function(chunk) {
  var stream = this._stream;
  if (stream._state !== 'readable') throw new TypeError('ReadableStream is not readable');
  if (stream._pendingReads.length) stream._pendingReads.shift().resolve({ value: chunk, done: false });
  else stream._queue.push(chunk);
};
ReadableStreamDefaultController.prototype.close = function() {
  var stream = this._stream; if (stream._state !== 'readable') return; stream._state = 'closed';
  while (stream._pendingReads.length) stream._pendingReads.shift().resolve({ value: undefined, done: true });
  if (stream._closedResolve) stream._closedResolve();
};
ReadableStreamDefaultController.prototype.error = function(reason) {
  var stream = this._stream; if (stream._state !== 'readable') return; stream._state = 'errored'; stream._storedError = reason;
  while (stream._pendingReads.length) stream._pendingReads.shift().reject(reason);
  if (stream._closedReject) stream._closedReject(reason);
};
function ReadableStream(underlyingSource) {
  underlyingSource = underlyingSource || {}; this._queue = []; this._pendingReads = []; this._state = 'readable'; this._storedError = undefined; this._reader = null; this._source = underlyingSource;
  var controller = this._controller = new ReadableStreamDefaultController(this);
  if (typeof underlyingSource.start === 'function') try { Promise.resolve(underlyingSource.start(controller)).catch(function(reason) { controller.error(reason); }); } catch (reason) { controller.error(reason); }
}
Object.defineProperty(ReadableStream.prototype, 'locked', { configurable: true, get: function() { return this._reader !== null; } });
ReadableStream.prototype.cancel = function(reason) { if (this.locked) return Promise.reject(new TypeError('ReadableStream is locked')); this._queue.length = 0; this._state = 'closed'; return Promise.resolve(typeof this._source.cancel === 'function' ? this._source.cancel(reason) : undefined); };
ReadableStream.prototype.getReader = function() { if (this.locked) throw new TypeError('ReadableStream is locked'); return new ReadableStreamDefaultReader(this); };
ReadableStream.prototype.pipeThrough = function(transform, options) { this.pipeTo(transform.writable, options); return transform.readable; };
ReadableStream.prototype.pipeTo = function(destination) {
  var reader = this.getReader(), writer = destination.getWriter();
  function pump() { return reader.read().then(function(result) { if (result.done) return writer.close(); return Promise.resolve(writer.write(result.value)).then(pump); }); }
  return pump().finally(function() { reader.releaseLock(); writer.releaseLock(); });
};
ReadableStream.prototype.tee = function() {
  var reader = this.getReader(), controllers = [], branches = [0, 1].map(function() { return new ReadableStream({ start: function(controller) { controllers.push(controller); } }); });
  function pump() { reader.read().then(function(result) { if (result.done) { controllers.forEach(function(controller) { controller.close(); }); reader.releaseLock(); return; } controllers.forEach(function(controller) { controller.enqueue(result.value); }); pump(); }, function(reason) { controllers.forEach(function(controller) { controller.error(reason); }); }); }
  pump(); return branches;
};
function ReadableStreamDefaultReader(stream) { this._stream = stream; stream._reader = this; this.closed = stream._state === 'errored' ? Promise.reject(stream._storedError) : stream._state === 'closed' ? Promise.resolve() : new Promise(function(resolve, reject) { stream._closedResolve = resolve; stream._closedReject = reject; }); }
ReadableStreamDefaultReader.prototype.read = function() {
  var stream = this._stream; if (!stream) return Promise.reject(new TypeError('Reader has no stream'));
  if (stream._queue.length) return Promise.resolve({ value: stream._queue.shift(), done: false });
  if (stream._state === 'closed') return Promise.resolve({ value: undefined, done: true });
  if (stream._state === 'errored') return Promise.reject(stream._storedError);
  var pending, promise = new Promise(function(resolve, reject) { pending = { resolve: resolve, reject: reject }; }); stream._pendingReads.push(pending);
  if (typeof stream._source.pull === 'function') try { Promise.resolve(stream._source.pull(stream._controller)).catch(function(reason) { stream._controller.error(reason); }); } catch (reason) { stream._controller.error(reason); }
  return promise;
};
ReadableStreamDefaultReader.prototype.cancel = function(reason) { var stream = this._stream; if (!stream) return Promise.reject(new TypeError('Reader has no stream')); stream._queue.length = 0; stream._state = 'closed'; if (stream._closedResolve) stream._closedResolve(); return Promise.resolve(typeof stream._source.cancel === 'function' ? stream._source.cancel(reason) : undefined); };
ReadableStreamDefaultReader.prototype.releaseLock = function() { if (this._stream) { this._stream._reader = null; this._stream = null; } };
function URLSearchParams(init) {
  this._pairs = [];
  this.__minib_onchange = null;
  this.__minib_replace(init);
}
URLSearchParams.prototype.__minib_replace = function(init) {
  var pairs = this._pairs = [];
  if (init instanceof URLSearchParams) {
    init._pairs.forEach(function(pair) { pairs.push([pair[0], pair[1]]); });
  } else if (typeof init === 'string' || init == null) {
    var input = String(init == null ? '' : init).replace(/^\?/, '');
    if (input) input.split('&').forEach(function(part) {
      if (!part) return;
      var separator = part.indexOf('='), rawName = separator < 0 ? part : part.slice(0, separator), rawValue = separator < 0 ? '' : part.slice(separator + 1);
      function decode(value) { try { return decodeURIComponent(value.replace(/\+/g, ' ')); } catch (_) { return value; } }
      pairs.push([decode(rawName), decode(rawValue)]);
    });
  } else if (typeof init[Symbol.iterator] === 'function') {
    Array.from(init).forEach(function(pair) { if (!pair || pair.length !== 2) throw new TypeError('URLSearchParams pair must contain two items'); pairs.push([String(pair[0]), String(pair[1])]); });
  } else {
    Object.keys(init).forEach(function(name) { pairs.push([String(name), String(init[name])]); });
  }
};
URLSearchParams.prototype.__minib_changed = function() { if (typeof this.__minib_onchange === 'function') this.__minib_onchange(this.toString()); };
URLSearchParams.prototype.append = function(name, value) { this._pairs.push([String(name), String(value)]); this.__minib_changed(); };
URLSearchParams.prototype.delete = function(name, value) { name = String(name); var matchValue = arguments.length > 1, expected = String(value); this._pairs = this._pairs.filter(function(pair) { return pair[0] !== name || matchValue && pair[1] !== expected; }); this.__minib_changed(); };
URLSearchParams.prototype.get = function(name) { name = String(name); for (var index = 0; index < this._pairs.length; index++) if (this._pairs[index][0] === name) return this._pairs[index][1]; return null; };
URLSearchParams.prototype.getAll = function(name) { name = String(name); return this._pairs.filter(function(pair) { return pair[0] === name; }).map(function(pair) { return pair[1]; }); };
URLSearchParams.prototype.has = function(name, value) { name = String(name); var matchValue = arguments.length > 1, expected = String(value); return this._pairs.some(function(pair) { return pair[0] === name && (!matchValue || pair[1] === expected); }); };
URLSearchParams.prototype.set = function(name, value) { name = String(name); value = String(value); var found = false, next = []; this._pairs.forEach(function(pair) { if (pair[0] !== name) next.push(pair); else if (!found) { next.push([name, value]); found = true; } }); if (!found) next.push([name, value]); this._pairs = next; this.__minib_changed(); };
URLSearchParams.prototype.sort = function() { this._pairs = this._pairs.map(function(pair, index) { return [pair, index]; }).sort(function(left, right) { return left[0][0] < right[0][0] ? -1 : left[0][0] > right[0][0] ? 1 : left[1] - right[1]; }).map(function(item) { return item[0]; }); this.__minib_changed(); };
URLSearchParams.prototype.forEach = function(callback, thisArg) { var self = this; this._pairs.slice().forEach(function(pair) { callback.call(thisArg, pair[1], pair[0], self); }); };
URLSearchParams.prototype.entries = function() { return this._pairs.map(function(pair) { return [pair[0], pair[1]]; })[Symbol.iterator](); };
URLSearchParams.prototype.keys = function() { return this._pairs.map(function(pair) { return pair[0]; })[Symbol.iterator](); };
URLSearchParams.prototype.values = function() { return this._pairs.map(function(pair) { return pair[1]; })[Symbol.iterator](); };
URLSearchParams.prototype.toString = function() { function encode(value) { return encodeURIComponent(value).replace(/%20/g, '+').replace(/[!'()~]/g, function(character) { return '%' + character.charCodeAt(0).toString(16).toUpperCase(); }); } return this._pairs.map(function(pair) { return encode(pair[0]) + '=' + encode(pair[1]); }).join('&'); };
URLSearchParams.prototype[Symbol.iterator] = URLSearchParams.prototype.entries;
Object.defineProperty(URLSearchParams.prototype, 'size', { configurable: true, enumerable: true, get: function() { return this._pairs.length; } });
[Window, Node, CharacterData, Text, Comment, CDATASection, ProcessingInstruction, DocumentType, Element, HTMLElement, HTMLBodyElement, HTMLHtmlElement, HTMLImageElement, HTMLIFrameElement, HTMLTemplateElement, HTMLMediaElement, HTMLAudioElement, HTMLVideoElement, SVGElement, Document, DOMParser, HTMLDocument, DocumentFragment, ShadowRoot, UIEvent, MouseEvent, KeyboardEvent, FocusEvent, InputEvent, WheelEvent, PointerEvent, CustomEvent, MessageEvent, PromiseRejectionEvent, StyleSheet, CSSStyleSheet, MediaList, StyleSheetList, CSSRuleList, CSSRule, CSSStyleRule, CSSMediaRule, CSSStyleDeclaration, File, TextEncoder, ReadableStream, ReadableStreamDefaultController, ReadableStreamDefaultReader, ResizeObserver, ResizeObserverEntry, PerformanceObserver, AbortSignal, AbortController, URLSearchParams].forEach(function(constructor) {
  Object.defineProperty(constructor.prototype, 'constructor', { configurable: true, writable: true, value: constructor });
});
function Headers(init) {
  this._values = {};
  var self = this;
  if (typeof init === 'string') init.trim().split(/[\r\n]+/).forEach(function(line) { var index = line.indexOf(':'); if (index > 0) self.append(line.slice(0, index), line.slice(index + 1)); });
  else if (init instanceof Headers) init.forEach(function(value, name) { self.append(name, value); });
  else if (init && typeof init[Symbol.iterator] === 'function') Array.from(init).forEach(function(pair) { if (!pair || pair.length !== 2) throw new TypeError('Headers pair must contain two items'); self.append(pair[0], pair[1]); });
  else if (init && typeof init.forEach === 'function') init.forEach(function(value, name) { self.append(name, value); });
  else Object.keys(init || {}).forEach(function(name) { self.append(name, init[name]); });
}
Headers.prototype.append = function(name, value) { name = String(name).toLowerCase(); this._values[name] = this._values[name] ? this._values[name] + ', ' + value : String(value); };
Headers.prototype.set = function(name, value) { this._values[String(name).toLowerCase()] = String(value); };
Headers.prototype.get = function(name) { name = String(name).toLowerCase(); return Object.prototype.hasOwnProperty.call(this._values, name) ? this._values[name] : null; };
Headers.prototype.has = function(name) { return this.get(name) !== null; };
Headers.prototype.delete = function(name) { delete this._values[String(name).toLowerCase()]; };
Headers.prototype.forEach = function(callback, this_arg) { var self = this; Object.keys(this._values).forEach(function(name) { callback.call(this_arg, self._values[name], name, self); }); };
Headers.prototype.entries = function() { var self = this; return Object.keys(this._values).sort().map(function(name) { return [name, self._values[name]]; })[Symbol.iterator](); };
Headers.prototype.keys = function() { return Array.from(this.entries(), function(entry) { return entry[0]; })[Symbol.iterator](); };
Headers.prototype.values = function() { return Array.from(this.entries(), function(entry) { return entry[1]; })[Symbol.iterator](); };
Headers.prototype[Symbol.iterator] = Headers.prototype.entries;
function AbortSignal() { EventTarget.call(this); this.aborted = false; this.reason = undefined; }
AbortSignal.prototype = Object.create(EventTarget.prototype);
AbortSignal.prototype.throwIfAborted = function() { if (this.aborted) throw this.reason; };
AbortSignal.abort = function(reason) { var controller = new AbortController(); controller.abort(reason); return controller.signal; };
AbortSignal.timeout = function(milliseconds) { var controller = new AbortController(); setTimeout(function() { controller.abort(new DOMException('The operation timed out', 'TimeoutError')); }, Math.max(0, Number(milliseconds) || 0)); return controller.signal; };
AbortSignal.any = function(signals) {
  var controller = new AbortController();
  Array.from(signals || []).some(function(signal) {
    if (signal.aborted) { controller.abort(signal.reason); return true; }
    signal.addEventListener('abort', function() { controller.abort(signal.reason); }, { once: true });
    return false;
  });
  return controller.signal;
};
function AbortController() { this.signal = new AbortSignal(); }
AbortController.prototype.abort = function(reason) {
  if (this.signal.aborted) return;
  this.signal.aborted = true;
  this.signal.reason = reason === undefined ? new DOMException('This operation was aborted', 'AbortError') : reason;
  this.signal.dispatchEvent(new Event('abort'));
};
function Request(input, init) { init = init || {}; this.url = String(input && input.url || input); this.method = String(init.method || input && input.method || 'GET').toUpperCase(); this.headers = new Headers(init.headers || input && input.headers); this.body = init.body == null ? null : init.body; this.signal = init.signal || input && input.signal || null; }
function Response(body, init) { init = init || {}; this._body = body == null ? '' : String(body); this.body = body == null ? null : body instanceof ReadableStream ? body : new ReadableStream({ start: function(controller) { controller.enqueue(new TextEncoder().encode(String(body))); controller.close(); } }); this.status = init.status || 200; this.statusText = init.statusText || ''; this.headers = init.headers instanceof Headers ? init.headers : new Headers(init.headers); this.url = init.url || ''; this.ok = this.status >= 200 && this.status < 300; this.redirected = false; this.type = 'basic'; this.bodyUsed = false; }
Response.prototype.text = function() { this.bodyUsed = true; return Promise.resolve(this._body); };
Response.prototype.json = function() { this.bodyUsed = true; return Promise.resolve(JSON.parse(this._body)); };
Response.prototype.clone = function() { return new Response(this._body, { status: this.status, statusText: this.statusText, headers: this.headers, url: this.url }); };
function fetch(input, init) {
  var request = input instanceof Request ? input : new Request(input, init);
  return new Promise(function(resolve, reject) {
    if (request.signal && request.signal.aborted) { reject(request.signal.reason); return; }
    var xhr = new XMLHttpRequest();
    var settled = false;
    function cleanup() { if (request.signal) request.signal.removeEventListener('abort', abort); }
    function finish(callback, value) { if (settled) return; settled = true; cleanup(); callback(value); }
    function abort() { xhr.abort(); finish(reject, request.signal.reason); }
    xhr.__minib_resource_type = 'fetch';
    xhr.open(request.method, request.url, true);
    request.headers.forEach(function(value, name) { xhr.setRequestHeader(name, value); });
    xhr.onload = function() { finish(resolve, new Response(xhr.responseText, { status: xhr.status, statusText: xhr.statusText, headers: new Headers(xhr.getAllResponseHeaders()), url: xhr.responseURL })); };
    xhr.onerror = function() { finish(reject, new TypeError('Failed to fetch')); };
    xhr.onabort = function() { finish(reject, request.signal && request.signal.reason || new DOMException('This operation was aborted', 'AbortError')); };
    if (request.signal) request.signal.addEventListener('abort', abort, { once: true });
    xhr.send(request.body);
  });
}
var Intl = {
  getCanonicalLocales: function(locales) { return Array.isArray(locales) ? locales.map(String) : [String(locales || '')]; },
  DateTimeFormat: function() { this.format = function(value) { return new Date(value).toString(); }; this.resolvedOptions = function() { return { locale: 'zh-CN', timeZone: 'Asia/Shanghai' }; }; },
  NumberFormat: function() { this.format = function(value) { return String(value); }; },
  Collator: function() { this.compare = function(left, right) { left = String(left); right = String(right); return left < right ? -1 : left > right ? 1 : 0; }; },
  RelativeTimeFormat: function() { this.format = function(value, unit) { return String(value) + ' ' + unit; }; },
  PluralRules: function() { this.select = function() { return 'other'; }; }
};
[Intl.DateTimeFormat, Intl.NumberFormat, Intl.Collator, Intl.RelativeTimeFormat, Intl.PluralRules].forEach(function(constructor) {
  constructor.supportedLocalesOf = function(locales) { return Intl.getCanonicalLocales(locales); };
});
var __minib_native_array_join = Array.prototype.join, __minib_array_join_stack = [];
Array.prototype.join = function(separator) {
  if (__minib_array_join_stack.indexOf(this) >= 0) return '';
  __minib_array_join_stack.push(this);
  try { return __minib_native_array_join.call(this, separator); }
  finally { __minib_array_join_stack.pop(); }
};
if (!Date.prototype.toGMTString) Date.prototype.toGMTString = Date.prototype.toUTCString;
`
	if _, err := runtime.vm.RunString(constructors); err != nil {
		return err
	}
	window := runtime.vm.GlobalObject()
	_ = window.SetPrototype(window.Get("Window").ToObject(runtime.vm).Get("prototype").ToObject(runtime.vm))
	_ = window.Set("window", window)
	_ = window.Set("self", window)
	_ = window.Set("top", window)
	_ = window.Set("parent", window)
	runtime.install_shared_dom_bridge(window)
	_ = window.Set("__minib_construct_html_element", func(call goja.FunctionCall) goja.Value {
		if len(runtime.pending_custom_nodes) == 0 {
			return goja.Undefined()
		}
		node := runtime.pending_custom_nodes[len(runtime.pending_custom_nodes)-1]
		return runtime.node_object(node)
	})
	_ = window.Set("__minib_parse_document", func(markup string, _ string) goja.Value {
		document, err := html.Parse(strings.NewReader(markup))
		if err != nil {
			panic(runtime.vm.NewTypeError("DOMParser failed: %s", err.Error()))
		}
		return runtime.node_object(document)
	})
	_ = window.Set("__minib_post_message_port", func(call goja.FunctionCall) goja.Value {
		target_value := call.Argument(0)
		if target_value == nil || goja.IsUndefined(target_value) || goja.IsNull(target_value) {
			return goja.Undefined()
		}
		target := target_value.ToObject(runtime.vm)
		data := call.Argument(1)
		runtime.queue_host_job(func() {
			event := runtime.event_object("message")
			_ = event.SetPrototype(runtime.vm.Get("MessageEvent").ToObject(runtime.vm).Get("prototype").ToObject(runtime.vm))
			_ = event.Set("data", data)
			_ = event.Set("origin", "")
			_ = event.Set("lastEventId", "")
			_ = event.Set("source", nil)
			_ = event.Set("ports", runtime.vm.NewArray())
			_ = event.Set("bubbles", false)
			_ = event.Set("cancelable", false)
			_ = event.Set("composed", false)
			dispatch_event, ok := goja.AssertFunction(target.Get("dispatchEvent"))
			if !ok {
				return
			}
			if _, err := runtime.call_javascript(runtime.ctx, dispatch_event, target, event); err != nil {
				runtime.fail_script(runtime.page.URL+"#message-port", err)
			}
		})
		return goja.Undefined()
	})
	_ = window.Set("postMessage", func(call goja.FunctionCall) goja.Value {
		data := call.Argument(0)
		runtime.queue_host_job(func() {
			event := runtime.event_object("message")
			_ = event.SetPrototype(runtime.vm.Get("MessageEvent").ToObject(runtime.vm).Get("prototype").ToObject(runtime.vm))
			_ = event.Set("data", data)
			_ = event.Set("origin", runtime.page_url.Scheme+"://"+runtime.page_url.Host)
			_ = event.Set("lastEventId", "")
			_ = event.Set("source", window)
			_ = event.Set("ports", runtime.vm.NewArray())
			runtime.dispatch_window_event(event, "message")
		})
		return goja.Undefined()
	})
	_ = window.Set("__minib_import", func(specifier string) *goja.Promise {
		referrer_url := runtime.current_script_url
		if referrer_url == "" {
			referrer_url = runtime.page.URL
		}
		return runtime.import_module(referrer_url, specifier)
	})
	// Weibo's current bundle references this undeclared optional component.
	_ = window.Set("toolbar", runtime.vm.NewObject())
	_ = window.Set("document", runtime.node_object(runtime.page.Document))
	runtime.install_custom_elements(window)
	runtime.install_cssom(window)
	_ = window.Set("location", runtime.location_object(runtime.page_url))
	navigator := runtime.vm.NewObject()
	_ = navigator.Set("userAgent", runtime.user_agent)
	_ = navigator.Set("platform", "MacIntel")
	_ = navigator.Set("language", "zh-CN")
	_ = navigator.Set("languages", runtime.vm.NewArray("zh-CN", "zh", "en"))
	_ = navigator.Set("cookieEnabled", true)
	_ = navigator.Set("webdriver", false)
	_ = navigator.Set("vendor", "Google Inc.")
	_ = navigator.Set("plugins", runtime.vm.NewArray())
	_ = navigator.Set("mimeTypes", runtime.vm.NewArray())
	_ = navigator.Set("hardwareConcurrency", 8)
	_ = navigator.Set("maxTouchPoints", 0)
	_ = navigator.Set("javaEnabled", func() bool { return false })
	_ = navigator.Set("sendBeacon", func(string, ...any) bool { return true })
	_ = window.Set("navigator", navigator)
	_ = window.Set("screen", map[string]int{"width": 1440, "height": 900, "availWidth": 1440, "availHeight": 875, "colorDepth": 24, "pixelDepth": 24})
	_ = window.Set("innerWidth", 1440)
	_ = window.Set("innerHeight", 900)
	_ = window.Set("devicePixelRatio", 2)
	_ = window.Set("pageXOffset", 0)
	_ = window.Set("pageYOffset", 0)
	_ = window.Set("scrollTo", func(...any) {})
	_ = window.Set("scrollBy", func(...any) {})
	_ = window.Set("console", runtime.console_object())
	if err := runtime.install_web_crypto(window); err != nil {
		return err
	}
	if err := runtime.install_webassembly(window); err != nil {
		return err
	}
	performance_time := time.Now().UnixMilli()
	performance_entries := make([]map[string]any, 0)
	performance_array := func(entries []map[string]any) *goja.Object {
		values := make([]any, len(entries))
		for index := range entries {
			values[index] = entries[index]
		}
		return runtime.vm.NewArray(values...)
	}
	performance := runtime.vm.NewObject()
	_ = performance.Set("now", func() float64 {
		return float64(time.Now().UnixNano())/float64(time.Millisecond) - float64(performance_time)
	})
	_ = performance.Set("timeOrigin", performance_time)
	_ = performance.Set("timing", map[string]int64{"navigationStart": performance_time, "responseStart": performance_time})
	_ = performance.Set("navigation", map[string]int{"type": 0})
	_ = performance.Set("mark", func(name string) map[string]any {
		entry := map[string]any{"name": name, "entryType": "mark", "startTime": float64(time.Now().UnixMilli() - performance_time), "duration": 0}
		performance_entries = append(performance_entries, entry)
		return entry
	})
	_ = performance.Set("measure", func(name string, _ ...any) map[string]any {
		entry := map[string]any{"name": name, "entryType": "measure", "startTime": 0, "duration": 0}
		performance_entries = append(performance_entries, entry)
		return entry
	})
	_ = performance.Set("getEntries", func() *goja.Object { return performance_array(performance_entries) })
	_ = performance.Set("getEntriesByType", func(entry_type string) *goja.Object {
		entries := make([]map[string]any, 0)
		for _, entry := range performance_entries {
			if entry["entryType"] == entry_type {
				entries = append(entries, entry)
			}
		}
		return performance_array(entries)
	})
	_ = performance.Set("getEntriesByName", func(name string) *goja.Object {
		entries := make([]map[string]any, 0)
		for _, entry := range performance_entries {
			if entry["name"] == name {
				entries = append(entries, entry)
			}
		}
		return performance_array(entries)
	})
	_ = performance.Set("clearMarks", func() { performance_entries = nil })
	_ = performance.Set("clearMeasures", func() { performance_entries = nil })
	_ = performance.Set("toJSON", func() map[string]any { return map[string]any{"timeOrigin": performance_time} })
	_ = window.Set("performance", performance)
	_ = window.Set("history", map[string]any{"length": 1, "pushState": func(...any) {}, "replaceState": func(...any) {}, "back": func() {}, "forward": func() {}, "go": func(...any) {}})
	_ = window.Set("localStorage", runtime.storage_object())
	_ = window.Set("sessionStorage", runtime.storage_object())
	_ = window.Set("getComputedStyle", func(call goja.FunctionCall) goja.Value {
		return runtime.computed_style_object(runtime.object_node(call.Argument(0)))
	})
	_ = window.Set("atob", func(call goja.FunctionCall) goja.Value {
		encoded := strings.NewReplacer("\r", "", "\n", "", "\t", "", " ", "").Replace(call.Argument(0).String())
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			decoded, err = base64.RawStdEncoding.DecodeString(encoded)
		}
		if err != nil {
			panic(runtime.vm.NewGoError(fmt.Errorf("InvalidCharacterError: invalid base64 input")))
		}
		characters := make([]uint16, len(decoded))
		for index, value := range decoded {
			characters[index] = uint16(value)
		}
		return goja.StringFromUTF16(characters)
	})
	_ = window.Set("btoa", func(call goja.FunctionCall) goja.Value {
		value, ok := call.Argument(0).ToString().(goja.String)
		if !ok {
			panic(runtime.vm.NewGoError(fmt.Errorf("InvalidCharacterError: invalid binary string")))
		}
		data := make([]byte, value.Length())
		for index := range data {
			character := value.CharAt(index)
			if character > 255 {
				panic(runtime.vm.NewGoError(fmt.Errorf("InvalidCharacterError: btoa input is outside Latin-1")))
			}
			data[index] = byte(character)
		}
		return runtime.vm.ToValue(base64.StdEncoding.EncodeToString(data))
	})
	_ = window.Set("TextDecoder", runtime.text_decoder_constructor)
	_ = window.Set("Image", func(call goja.ConstructorCall) *goja.Object { return runtime.node_object(new_element("img")) })
	_ = window.Set("Audio", func(call goja.ConstructorCall) *goja.Object { return runtime.node_object(new_element("audio")) })
	_ = window.Set("URL", runtime.url_constructor)
	url_constructor := window.Get("URL").ToObject(runtime.vm)
	_ = url_constructor.Set("canParse", func(call goja.FunctionCall) bool {
		base_url := runtime.base_url
		if !goja.IsUndefined(call.Argument(1)) {
			parsed_base, err := url.Parse(call.Argument(1).String())
			if err != nil {
				return false
			}
			base_url = parsed_base
		}
		_, err := base_url.Parse(call.Argument(0).String())
		return err == nil
	})
	_ = url_constructor.Set("parse", func(call goja.FunctionCall) any {
		if can_parse, ok := goja.AssertFunction(url_constructor.Get("canParse")); ok {
			value, _ := can_parse(goja.Undefined(), call.Arguments...)
			if !value.ToBoolean() {
				return nil
			}
		}
		return runtime.url_constructor(goja.ConstructorCall{This: runtime.vm.NewObject(), Arguments: call.Arguments})
	})
	_ = url_constructor.Set("createObjectURL", runtime.create_object_url)
	_ = url_constructor.Set("revokeObjectURL", runtime.revoke_object_url)
	_ = window.Set("Worker", runtime.worker_constructor)
	runtime.install_worker_prototype(window.Get("Worker").ToObject(runtime.vm))
	_ = window.Set("XMLHttpRequest", runtime.xml_http_request_constructor)
	xhr_constructor := window.Get("XMLHttpRequest").ToObject(runtime.vm)
	xhr_prototype := xhr_constructor.Get("prototype").ToObject(runtime.vm)
	event_target_prototype := window.Get("EventTarget").ToObject(runtime.vm).Get("prototype").ToObject(runtime.vm)
	_ = xhr_prototype.SetPrototype(event_target_prototype)
	runtime.install_xml_http_request_prototype(xhr_constructor)
	runtime.install_web_socket(window)
	_ = window.Set("matchMedia", func(query string) map[string]any {
		return map[string]any{"matches": strings.Contains(query, "hover") || strings.Contains(query, "pointer"), "media": query, "addListener": func(...any) {}, "removeListener": func(...any) {}, "addEventListener": func(...any) {}, "removeEventListener": func(...any) {}}
	})
	runtime.install_window_events(window)
	runtime.install_timers(window)
	return nil
}

func (runtime *page_runtime) install_custom_host_runtime() error {
	window := runtime.vm.GlobalObject()
	_ = window.Set("window", window)
	_ = window.Set("self", window)
	_ = window.Set("globalThis", window)
	_ = window.Set("atob", func(call goja.FunctionCall) goja.Value {
		encoded := strings.NewReplacer("\r", "", "\n", "", "\t", "", " ", "").Replace(call.Argument(0).String())
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			decoded, err = base64.RawStdEncoding.DecodeString(encoded)
		}
		if err != nil {
			panic(runtime.vm.NewGoError(fmt.Errorf("InvalidCharacterError: invalid base64 input")))
		}
		characters := make([]uint16, len(decoded))
		for index, value := range decoded {
			characters[index] = uint16(value)
		}
		return goja.StringFromUTF16(characters)
	})
	_ = window.Set("btoa", func(call goja.FunctionCall) goja.Value {
		value, ok := call.Argument(0).ToString().(goja.String)
		if !ok {
			panic(runtime.vm.NewGoError(fmt.Errorf("InvalidCharacterError: invalid binary string")))
		}
		data := make([]byte, value.Length())
		for index := range data {
			character := value.CharAt(index)
			if character > 255 {
				panic(runtime.vm.NewGoError(fmt.Errorf("InvalidCharacterError: btoa input is outside Latin-1")))
			}
			data[index] = byte(character)
		}
		return runtime.vm.ToValue(base64.StdEncoding.EncodeToString(data))
	})
	if err := runtime.install_web_crypto(window); err != nil {
		return err
	}
	if err := runtime.install_webassembly(window); err != nil {
		return err
	}
	runtime.install_timers(window)
	return nil
}

func (runtime *page_runtime) xml_http_request_constructor(call goja.ConstructorCall) *goja.Object {
	request := &xml_http_request{
		runtime:          runtime,
		object:           call.This,
		request_headers:  make(http.Header),
		response_headers: make(http.Header),
		listeners:        make(map[string][]goja.Callable),
	}
	object := request.object
	define_getter(runtime.vm, object, "readyState", func() any { return request.ready_state })
	define_getter(runtime.vm, object, "status", func() any { return request.status })
	define_getter(runtime.vm, object, "statusText", func() any { return request.status_text })
	define_getter(runtime.vm, object, "responseText", func() any { return request.response_text })
	define_getter(runtime.vm, object, "responseURL", func() any { return request.response_url })
	define_getter(runtime.vm, object, "responseXML", func() any { return nil })
	define_getter(runtime.vm, object, "response", func() any { return request.response_value() })
	_ = object.Set("responseType", "")
	_ = object.Set("timeout", 0)
	_ = object.Set("withCredentials", false)
	_ = object.Set("upload", runtime.vm.NewObject())
	_ = object.Set("__minib_open", func(open_call goja.FunctionCall) goja.Value {
		if request.request_cancel != nil {
			request.request_cancel()
			request.request_cancel = nil
		}
		request.request_version++
		request.method = strings.ToUpper(open_call.Argument(0).String())
		request.raw_url = open_call.Argument(1).String()
		request.async_request = goja.IsUndefined(open_call.Argument(2)) || open_call.Argument(2).ToBoolean()
		request.status = 0
		request.status_text = ""
		request.response_text = ""
		request.response_url = ""
		request.ready_state = 1
		request.fire("readystatechange")
		return goja.Undefined()
	})
	_ = object.Set("__minib_setRequestHeader", func(name string, value string) { request.request_headers.Add(name, value) })
	_ = object.Set("__minib_getResponseHeader", func(name string) any {
		value := request.response_headers.Get(name)
		if value == "" {
			return nil
		}
		return value
	})
	_ = object.Set("__minib_getAllResponseHeaders", func() string {
		var builder strings.Builder
		for name, values := range request.response_headers {
			for _, value := range values {
				builder.WriteString(strings.ToLower(name))
				builder.WriteString(": ")
				builder.WriteString(value)
				builder.WriteString("\r\n")
			}
		}
		return builder.String()
	})
	_ = object.Set("__minib_overrideMimeType", func(string) {})
	_ = object.Set("__minib_abort", func() {
		request.abort()
	})
	_ = object.Set("__minib_addEventListener", func(event_call goja.FunctionCall) goja.Value {
		if callback, ok := goja.AssertFunction(event_call.Argument(1)); ok {
			event_name := event_call.Argument(0).String()
			request.listeners[event_name] = append(request.listeners[event_name], callback)
		}
		return goja.Undefined()
	})
	_ = object.Set("__minib_removeEventListener", func() {})
	_ = object.Set("__minib_send", func(send_call goja.FunctionCall) goja.Value {
		var body []byte
		if value := send_call.Argument(0); !goja.IsNull(value) && !goja.IsUndefined(value) {
			body = []byte(value.String())
		}
		request.send(body)
		return goja.Undefined()
	})
	return nil
}

func (runtime *page_runtime) install_xml_http_request_prototype(constructor *goja.Object) {
	prototype := constructor.Get("prototype").ToObject(runtime.vm)
	for _, name := range []string{"open", "setRequestHeader", "getResponseHeader", "getAllResponseHeaders", "overrideMimeType", "abort", "addEventListener", "removeEventListener", "send"} {
		method_name := name
		_ = prototype.Set(method_name, func(call goja.FunctionCall) goja.Value {
			method, ok := goja.AssertFunction(call.This.ToObject(runtime.vm).Get("__minib_" + method_name))
			if !ok {
				panic(runtime.vm.NewTypeError("XMLHttpRequest.%s called on incompatible receiver", method_name))
			}
			value, err := method(call.This, call.Arguments...)
			if err != nil {
				panic(err)
			}
			return value
		})
	}
	for _, name := range []string{"onreadystatechange", "onload", "onerror", "onabort", "ontimeout", "onloadend"} {
		_ = prototype.Set(name, nil)
	}
	for name, value := range map[string]int{"UNSENT": 0, "OPENED": 1, "HEADERS_RECEIVED": 2, "LOADING": 3, "DONE": 4} {
		_ = constructor.Set(name, value)
		_ = prototype.Set(name, value)
	}
}

func (request *xml_http_request) send(body []byte) {
	network_request, err := request.prepare_network_request(body)
	if err != nil {
		request.apply_failure(err, "error")
		return
	}
	request.request_version++
	request_version := request.request_version
	request_ctx, cancel := context_with_optional_timeout(network_request.ctx, network_request.timeout)
	network_request.ctx = request_ctx
	request.request_cancel = cancel
	if !request.async_request {
		result := request.perform_network_request(network_request)
		cancel()
		request.request_cancel = nil
		request.apply_result(result)
		return
	}
	request.runtime.begin_network_task()
	go func() {
		result := request.perform_network_request(network_request)
		cancel()
		request.runtime.complete_network_task(func() {
			if request.request_version != request_version {
				return
			}
			request.request_cancel = nil
			request.apply_result(result)
		})
	}()
}

func (request *xml_http_request) prepare_network_request(body []byte) (xhr_network_request, error) {
	if request.method == "" {
		request.method = http.MethodGet
	}
	request_url, err := request.runtime.base_url.Parse(request.raw_url)
	if err != nil {
		return xhr_network_request{}, err
	}
	headers := request.runtime.request_headers.Clone()
	if headers == nil {
		headers = clawreq.DefaultHeaders(clawreq.ProfileChrome)
	}
	if request.runtime.user_agent != "" {
		headers.Set("User-Agent", request.runtime.user_agent)
	}
	headers.Set("Accept", "*/*")
	headers.Set("Priority", "u=1, i")
	headers.Set("Referer", request.runtime.page.URL)
	headers.Set("Sec-Fetch-Dest", "empty")
	headers.Set("Sec-Fetch-Mode", "cors")
	headers.Del("Sec-Fetch-User")
	headers.Del("Upgrade-Insecure-Requests")
	if same_origin(request.runtime.page_url, request_url) {
		headers.Set("Sec-Fetch-Site", "same-origin")
	} else {
		headers.Set("Sec-Fetch-Site", "cross-site")
	}
	for name, values := range request.request_headers {
		headers[name] = append([]string(nil), values...)
	}
	if request.runtime.page.disable_cache {
		disable_cache_headers(headers)
	}
	resource_type := "xhr"
	if value := request.object.Get("__minib_resource_type"); value != nil && value.String() == "fetch" {
		resource_type = "fetch"
		request.runtime.page.FetchRequests = append(request.runtime.page.FetchRequests, request_url.String())
	} else {
		request.runtime.page.XHRRequests = append(request.runtime.page.XHRRequests, request_url.String())
	}
	request_timeout := request.runtime.page.resource_timeout
	if timeout_value := request.object.Get("timeout"); timeout_value != nil && !goja.IsUndefined(timeout_value) && !goja.IsNull(timeout_value) {
		if timeout_ms := timeout_value.ToInteger(); timeout_ms > 0 {
			request_timeout = time.Duration(timeout_ms) * time.Millisecond
		}
	}
	return xhr_network_request{
		ctx:           with_har_resource_type(request.runtime.network_ctx, resource_type),
		method:        request.method,
		raw_url:       request_url.String(),
		body:          append([]byte(nil), body...),
		headers:       headers,
		resource_type: resource_type,
		timeout:       request_timeout,
	}, nil
}

func (request *xml_http_request) perform_network_request(network_request xhr_network_request) xhr_network_result {
	var body_reader io.Reader
	if network_request.body != nil {
		body_reader = bytes.NewReader(network_request.body)
	}
	response, err := request.runtime.browser.Request(network_request.ctx, network_request.method, network_request.raw_url, body_reader, network_request.headers)
	if err != nil {
		return xhr_network_result{err: err}
	}
	response_text, err := response.Text()
	if err != nil {
		return xhr_network_result{err: err}
	}
	return xhr_network_result{
		status:           response.StatusCode,
		status_text:      response.Status,
		response_headers: response.Header,
		response_text:    response_text,
		response_url:     response.FinalURL,
	}
}

func (request *xml_http_request) apply_result(result xhr_network_result) {
	if result.err != nil {
		event_name := "error"
		if errors.Is(result.err, context.DeadlineExceeded) {
			event_name = "timeout"
		}
		request.apply_failure(result.err, event_name)
		return
	}
	request.status = result.status
	request.status_text = result.status_text
	request.response_headers = result.response_headers
	request.response_url = result.response_url
	request.ready_state = 2
	request.fire("readystatechange")
	request.response_text = result.response_text
	request.ready_state = 3
	request.fire("readystatechange")
	request.ready_state = 4
	request.fire("readystatechange")
	request.fire("load")
	request.fire("loadend")
}

func (request *xml_http_request) apply_failure(err error, event_name string) {
	request.status = 0
	request.status_text = err.Error()
	request.ready_state = 4
	request.fire("readystatechange")
	request.fire(event_name)
	request.fire("loadend")
}

func (request *xml_http_request) abort() {
	request.request_version++
	if request.request_cancel != nil {
		request.request_cancel()
		request.request_cancel = nil
	}
	if request.ready_state == 0 || request.ready_state == 4 {
		request.ready_state = 0
		return
	}
	request.status = 0
	request.status_text = ""
	request.response_text = ""
	request.response_url = ""
	request.ready_state = 4
	request.fire("readystatechange")
	request.fire("abort")
	request.fire("loadend")
	request.ready_state = 0
}

func (request *xml_http_request) response_value() any {
	if strings.EqualFold(request.object.Get("responseType").String(), "json") {
		json_object := request.runtime.vm.Get("JSON").ToObject(request.runtime.vm)
		if parse, ok := goja.AssertFunction(json_object.Get("parse")); ok {
			value, err := parse(json_object, request.runtime.vm.ToValue(request.response_text))
			if err == nil {
				return value
			}
		}
		return nil
	}
	return request.response_text
}

func (request *xml_http_request) fire(event_name string) {
	event := request.runtime.vm.NewObject()
	_ = event.Set("type", event_name)
	_ = event.Set("target", request.object)
	_ = event.Set("currentTarget", request.object)
	handler, has_handler := goja.AssertFunction(request.object.Get("on" + event_name))
	for _, callback := range request.listeners[event_name] {
		if _, err := request.runtime.call_javascript(request.runtime.ctx, callback, request.object, event); err != nil {
			request.runtime.fail_script(request.raw_url+"#"+event_name, err)
		}
	}
	if has_handler {
		if _, err := request.runtime.call_javascript(request.runtime.ctx, handler, request.object, event); err != nil {
			request.runtime.fail_script(request.raw_url+"#"+event_name, err)
		}
	}
}

func (runtime *page_runtime) console_object() *goja.Object {
	object := runtime.vm.NewObject()
	no_op := func(...any) {}
	for _, name := range []string{"log", "info", "debug", "trace", "group", "groupEnd"} {
		_ = object.Set(name, no_op)
	}
	for _, name := range []string{"warn", "error"} {
		level := name
		_ = object.Set(name, func(call goja.FunctionCall) goja.Value {
			parts := make([]string, 0, len(call.Arguments))
			for _, argument := range call.Arguments {
				part := argument.String()
				if argument_object, ok := argument.(*goja.Object); ok {
					if stack := argument_object.Get("stack"); stack != nil && !goja.IsUndefined(stack) && !goja.IsNull(stack) && !strings.Contains(part, stack.String()) {
						part += "\n" + stack.String()
					}
				}
				parts = append(parts, part)
			}
			runtime.page.ConsoleMessages = append(runtime.page.ConsoleMessages, level+": "+strings.Join(parts, " "))
			return goja.Undefined()
		})
	}
	return object
}

func (runtime *page_runtime) storage_object() *goja.Object {
	values := make(map[string]string)
	object := runtime.vm.NewObject()
	_ = object.Set("getItem", func(key string) any {
		value, ok := values[key]
		if !ok {
			return nil
		}
		return value
	})
	_ = object.Set("setItem", func(key string, value string) { values[key] = value })
	_ = object.Set("removeItem", func(key string) { delete(values, key) })
	_ = object.Set("clear", func() {
		for key := range values {
			delete(values, key)
		}
	})
	_ = object.Set("key", func(index int) any {
		for key := range values {
			if index == 0 {
				return key
			}
			index--
		}
		return nil
	})
	return object
}

func (runtime *page_runtime) location_object(parsed_url *url.URL) *goja.Object {
	object := runtime.vm.NewObject()
	_ = object.SetPrototype(runtime.vm.Get("Location").ToObject(runtime.vm).Get("prototype").ToObject(runtime.vm))
	set_location_fields(object, parsed_url)
	navigate := func(raw_url string) { runtime.request_navigation(raw_url) }
	_ = object.DefineAccessorProperty("href", runtime.vm.ToValue(func() string { return parsed_url.String() }), runtime.vm.ToValue(navigate), goja.FLAG_TRUE, goja.FLAG_TRUE)
	_ = object.Set("assign", navigate)
	_ = object.Set("replace", navigate)
	_ = object.Set("reload", func() {})
	_ = object.Set("toString", func() string { return parsed_url.String() })
	return object
}

func (runtime *page_runtime) request_navigation(raw_url string) {
	next_url, err := runtime.page_url.Parse(strings.TrimSpace(raw_url))
	if err == nil && (next_url.Scheme == "http" || next_url.Scheme == "https") {
		runtime.page.navigation_url = next_url.String()
	}
}

func (runtime *page_runtime) text_decoder_constructor(call goja.ConstructorCall) *goja.Object {
	object := call.This
	_ = object.Set("decode", func(value goja.Value) string {
		if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
			return ""
		}
		var bytes []byte
		if runtime.vm.ExportTo(value, &bytes) != nil {
			return ""
		}
		return string(bytes)
	})
	return nil
}

func set_location_fields(object *goja.Object, parsed_url *url.URL) {
	_ = object.Set("href", parsed_url.String())
	_ = object.Set("protocol", parsed_url.Scheme+":")
	_ = object.Set("host", parsed_url.Host)
	_ = object.Set("hostname", parsed_url.Hostname())
	_ = object.Set("port", parsed_url.Port())
	_ = object.Set("pathname", parsed_url.EscapedPath())
	_ = object.Set("search", func() string {
		if parsed_url.RawQuery == "" {
			return ""
		}
		return "?" + parsed_url.RawQuery
	}())
	_ = object.Set("hash", func() string {
		if parsed_url.Fragment == "" {
			return ""
		}
		return "#" + parsed_url.Fragment
	}())
	_ = object.Set("origin", parsed_url.Scheme+"://"+parsed_url.Host)
}

func (runtime *page_runtime) url_constructor(call goja.ConstructorCall) *goja.Object {
	raw_url := call.Argument(0).String()
	base_url := runtime.base_url
	if !goja.IsUndefined(call.Argument(1)) {
		if parsed_base, err := url.Parse(call.Argument(1).String()); err == nil {
			base_url = parsed_base
		}
	}
	parsed_url, err := base_url.Parse(raw_url)
	if err != nil {
		panic(runtime.vm.NewTypeError("invalid URL"))
	}
	object := call.This
	search_params_constructor, ok := goja.AssertConstructor(runtime.vm.Get("URLSearchParams"))
	if !ok {
		panic(runtime.vm.NewTypeError("URLSearchParams is not available"))
	}
	search_params, err := search_params_constructor(nil, runtime.vm.ToValue(parsed_url.RawQuery))
	if err != nil {
		panic(err)
	}
	search_params_object := search_params.ToObject(runtime.vm)
	_ = search_params_object.Set("__minib_onchange", func(query string) {
		parsed_url.RawQuery = query
		parsed_url.ForceQuery = false
	})
	replace_search_params := func() {
		replace, replace_ok := goja.AssertFunction(search_params_object.Get("__minib_replace"))
		if replace_ok {
			_, _ = replace(search_params_object, runtime.vm.ToValue(parsed_url.RawQuery))
		}
	}
	set_url := func(value string) {
		next_url, parse_err := base_url.Parse(value)
		if parse_err != nil {
			panic(runtime.vm.NewTypeError("invalid URL"))
		}
		*parsed_url = *next_url
		replace_search_params()
	}
	define_url_property := func(name string, getter func() string, setter func(string)) {
		var setter_value goja.Value
		if setter != nil {
			setter_value = runtime.vm.ToValue(setter)
		}
		_ = object.DefineAccessorProperty(name, runtime.vm.ToValue(getter), setter_value, goja.FLAG_TRUE, goja.FLAG_TRUE)
	}
	define_url_property("href", func() string { return parsed_url.String() }, set_url)
	define_url_property("origin", func() string { return parsed_url.Scheme + "://" + parsed_url.Host }, nil)
	define_url_property("protocol", func() string { return parsed_url.Scheme + ":" }, func(value string) { parsed_url.Scheme = strings.TrimSuffix(value, ":") })
	define_url_property("username", func() string {
		if parsed_url.User == nil {
			return ""
		}
		return parsed_url.User.Username()
	}, nil)
	define_url_property("password", func() string {
		if parsed_url.User == nil {
			return ""
		}
		password, _ := parsed_url.User.Password()
		return password
	}, nil)
	define_url_property("host", func() string { return parsed_url.Host }, func(value string) { parsed_url.Host = value })
	define_url_property("hostname", func() string { return parsed_url.Hostname() }, nil)
	define_url_property("port", func() string { return parsed_url.Port() }, nil)
	define_url_property("pathname", func() string { return parsed_url.EscapedPath() }, func(value string) {
		parsed_url.Path = value
		parsed_url.RawPath = ""
	})
	define_url_property("search", func() string {
		if parsed_url.RawQuery == "" {
			return ""
		}
		return "?" + parsed_url.RawQuery
	}, func(value string) {
		parsed_url.RawQuery = strings.TrimPrefix(value, "?")
		parsed_url.ForceQuery = false
		replace_search_params()
	})
	define_url_property("hash", func() string {
		if parsed_url.Fragment == "" {
			return ""
		}
		return "#" + parsed_url.Fragment
	}, func(value string) { parsed_url.Fragment = strings.TrimPrefix(value, "#") })
	define_getter(runtime.vm, object, "searchParams", func() any { return search_params_object })
	_ = object.Set("toString", func() string { return parsed_url.String() })
	_ = object.Set("toJSON", func() string { return parsed_url.String() })
	return object
}

func (runtime *page_runtime) install_timers(window *goja.Object) {
	add_timer := func(call goja.FunctionCall, repeating bool) goja.Value {
		callback, ok := goja.AssertFunction(call.Argument(0))
		if !ok {
			return runtime.vm.ToValue(0)
		}
		delay_ms := call.Argument(1).ToInteger()
		if delay_ms < 0 {
			delay_ms = 0
		}
		if repeating && delay_ms < 4 {
			delay_ms = 4
		}
		runtime.next_timer_id++
		args := make([]goja.Value, 0)
		if len(call.Arguments) > 2 {
			args = append(args, call.Arguments[2:]...)
		}
		timer := &timer_job{
			id:          runtime.next_timer_id,
			callback:    callback,
			args:        args,
			due_at_ms:   runtime.timer_time_ms + delay_ms,
			interval_ms: delay_ms,
			repeating:   repeating,
		}
		runtime.timers = append(runtime.timers, timer)
		runtime.timer_by_id[timer.id] = timer
		return runtime.vm.ToValue(runtime.next_timer_id)
	}
	clear_timer := func(id int64) {
		if timer := runtime.timer_by_id[id]; timer != nil {
			timer.canceled = true
			delete(runtime.timer_by_id, id)
		}
	}
	_ = window.Set("setTimeout", func(call goja.FunctionCall) goja.Value { return add_timer(call, false) })
	_ = window.Set("setInterval", func(call goja.FunctionCall) goja.Value { return add_timer(call, true) })
	_ = window.Set("clearTimeout", clear_timer)
	_ = window.Set("clearInterval", clear_timer)
	_ = window.Set("requestAnimationFrame", func(call goja.FunctionCall) goja.Value {
		return add_timer(goja.FunctionCall{Arguments: []goja.Value{call.Argument(0), runtime.vm.ToValue(16)}}, false)
	})
	_ = window.Set("cancelAnimationFrame", clear_timer)
}

func (runtime *page_runtime) run_timers(ctx context.Context, timer_deadline_ms int64) bool {
	ran_callback := false
	for callback_count := 0; callback_count < max_timer_callbacks && len(runtime.timers) > 0 && ctx.Err() == nil; callback_count++ {
		if runtime.wait_condition_met() {
			return ran_callback
		}
		sort.SliceStable(runtime.timers, func(left int, right int) bool {
			if runtime.timers[left].due_at_ms == runtime.timers[right].due_at_ms {
				return runtime.timers[left].id < runtime.timers[right].id
			}
			return runtime.timers[left].due_at_ms < runtime.timers[right].due_at_ms
		})
		timer := runtime.timers[0]
		if timer.due_at_ms > timer_deadline_ms {
			break
		}
		runtime.timers = runtime.timers[1:]
		if timer.canceled {
			continue
		}
		runtime.timer_time_ms = timer.due_at_ms
		if !timer.repeating {
			delete(runtime.timer_by_id, timer.id)
		}
		ran_callback = true
		if _, err := runtime.call_javascript(ctx, timer.callback, runtime.vm.GlobalObject(), timer.args...); err != nil {
			runtime.fail_script(runtime.page.URL+"#timer", err)
		}
		if runtime.wait_condition_met() {
			return ran_callback
		}
		if timer.repeating && !timer.canceled {
			timer.due_at_ms += timer.interval_ms
			runtime.timers = append(runtime.timers, timer)
		}
		runtime.run_host_jobs(ctx)
		runtime.drain_dynamic_styles(ctx)
		runtime.drain_dynamic_scripts(ctx)
		runtime.drain_dynamic_resources(ctx)
	}
	return ran_callback
}

func (runtime *page_runtime) queue_host_job(job func()) {
	if job != nil {
		runtime.host_jobs = append(runtime.host_jobs, job)
	}
}

func (runtime *page_runtime) run_host_jobs(ctx context.Context) {
	runtime.run_external_jobs(ctx)
	for callback_count := 0; len(runtime.host_jobs) > 0 && callback_count < max_host_callbacks && ctx.Err() == nil; callback_count++ {
		if runtime.wait_condition_met() {
			return
		}
		job := runtime.host_jobs[0]
		runtime.host_jobs = runtime.host_jobs[1:]
		job()
		if runtime.wait_condition_met() {
			return
		}
		runtime.run_external_jobs(ctx)
	}
}

func (runtime *page_runtime) pump_event_loop(ctx context.Context) {
	timer_deadline_ms := runtime.timer_time_ms + max_timer_time_ms
	for round := 0; round < max_event_loop_rounds && ctx.Err() == nil; round++ {
		if runtime.wait_condition_met() {
			return
		}
		runtime.run_external_jobs(ctx)
		if runtime.wait_condition_met() {
			return
		}
		if len(runtime.host_jobs) == 0 && len(runtime.timers) == 0 && len(runtime.dynamic_styles) == 0 && len(runtime.dynamic_scripts) == 0 && len(runtime.dynamic_resources) == 0 && len(runtime.external_jobs) == 0 {
			if runtime.pending_network_tasks.Load() > 0 && runtime.wait_for_external_job(ctx) {
				continue
			}
			return
		}
		had_immediate_work := len(runtime.host_jobs) > 0 || len(runtime.external_jobs) > 0 || len(runtime.dynamic_styles) > 0 || len(runtime.dynamic_scripts) > 0 || len(runtime.dynamic_resources) > 0
		runtime.run_host_jobs(ctx)
		ran_timer := runtime.run_timers(ctx, timer_deadline_ms)
		runtime.run_host_jobs(ctx)
		if runtime.wait_condition_met() {
			return
		}
		runtime.drain_dynamic_styles(ctx)
		runtime.drain_dynamic_scripts(ctx)
		runtime.drain_dynamic_resources(ctx)
		if runtime.wait_condition_met() {
			return
		}
		if !had_immediate_work && !ran_timer {
			if runtime.pending_network_tasks.Load() > 0 && runtime.wait_for_external_job(ctx) {
				continue
			}
			return
		}
	}
}

func (runtime *page_runtime) fire_document_event(event_name string) {
	runtime.fire_node_event(runtime.page.Document, event_name)
}

func (runtime *page_runtime) fire_window_event(event_name string) {
	runtime.dispatch_window_event(runtime.event_object(event_name), event_name)
}

func (runtime *page_runtime) fire_node_event(node *html.Node, event_name string) {
	runtime.dispatch_node_event(node, runtime.event_object(event_name), event_name)
}

func (runtime *page_runtime) event_object(event_name string) *goja.Object {
	object := runtime.vm.NewObject()
	_ = object.Set("type", event_name)
	_ = object.Set("initEvent", func(event_type string, bubbles bool, cancelable bool) {
		_ = object.Set("type", event_type)
		_ = object.Set("bubbles", bubbles)
		_ = object.Set("cancelable", cancelable)
	})
	_ = object.Set("initCustomEvent", func(call goja.FunctionCall) goja.Value {
		_ = object.Set("type", call.Argument(0).String())
		_ = object.Set("bubbles", call.Argument(1).ToBoolean())
		_ = object.Set("cancelable", call.Argument(2).ToBoolean())
		_ = object.Set("detail", call.Argument(3))
		return goja.Undefined()
	})
	_ = object.Set("target", runtime.node_object(runtime.page.Document))
	_ = object.Set("currentTarget", runtime.node_object(runtime.page.Document))
	_ = object.Set("defaultPrevented", false)
	_ = object.Set("preventDefault", func() { _ = object.Set("defaultPrevented", true) })
	_ = object.Set("stopPropagation", func() {})
	return object
}

func (runtime *page_runtime) range_object(document *html.Node) *goja.Object {
	// This currently covers contextual fragment creation used by application
	// renderers; offset-accurate selection remains part of the Range work.
	object := runtime.vm.NewObject()
	_ = object.SetPrototype(runtime.vm.Get("Range").ToObject(runtime.vm).Get("prototype").ToObject(runtime.vm))
	selected_node := find_element(document, "body")
	if selected_node == nil {
		selected_node = document
	}
	_ = object.Set("selectNode", func(value goja.Value) { selected_node = runtime.object_node(value) })
	_ = object.Set("selectNodeContents", func(value goja.Value) { selected_node = runtime.object_node(value) })
	_ = object.Set("setStart", func(value goja.Value, _ int) { selected_node = runtime.object_node(value) })
	_ = object.Set("setEnd", func(value goja.Value, _ int) { selected_node = runtime.object_node(value) })
	_ = object.Set("collapse", func(bool) {})
	_ = object.Set("detach", func() {})
	_ = object.Set("createContextualFragment", func(markup string) any {
		fragment := &html.Node{Type: html.DocumentNode, Data: "#document-fragment"}
		runtime.fragments[fragment] = true
		context_node := selected_node
		if context_node == nil || context_node.Type != html.ElementNode {
			context_node = find_element(document, "body")
		}
		if context_node != nil {
			for _, child := range append_html(context_node, markup) {
				context_node.RemoveChild(child)
				fragment.AppendChild(child)
			}
		}
		return runtime.node_object(fragment)
	})
	_ = object.Set("cloneContents", func() any {
		fragment := &html.Node{Type: html.DocumentNode, Data: "#document-fragment"}
		runtime.fragments[fragment] = true
		if selected_node != nil {
			for child := selected_node.FirstChild; child != nil; child = child.NextSibling {
				fragment.AppendChild(runtime.clone_node(child, true))
			}
		}
		return runtime.node_object(fragment)
	})
	_ = object.Set("insertNode", func(value goja.Value) {
		if node := runtime.object_node(value); node != nil && selected_node != nil {
			runtime.detach_node(node)
			selected_node.AppendChild(node)
			runtime.queue_dynamic_resource(node)
		}
	})
	return object
}

func (runtime *page_runtime) node_object(node *html.Node) *goja.Object {
	if node == nil {
		return nil
	}
	if object := runtime.nodes[node]; object != nil {
		return object
	}
	object := runtime.vm.NewObject()
	runtime.bind_node_object(node, object, true)
	return object
}

func (runtime *page_runtime) nullable_node_value(node *html.Node) goja.Value {
	if node == nil {
		return goja.Null()
	}
	return runtime.node_object(node)
}

func (runtime *page_runtime) bind_node_object(node *html.Node, object *goja.Object, set_prototype bool) {
	if node == nil || object == nil || runtime.nodes[node] == object {
		return
	}
	runtime.nodes[node] = object
	runtime.object_nodes[object] = node
	if set_prototype {
		runtime.set_node_prototype(object, node)
	}
	runtime.install_node_events(object, node)
	if node.Type == html.DocumentNode && !runtime.fragments[node] {
		runtime.install_document(object, node)
	}
}

func (runtime *page_runtime) install_custom_elements(window *goja.Object) {
	registry := runtime.vm.NewObject()
	_ = registry.SetPrototype(window.Get("CustomElementRegistry").ToObject(runtime.vm).Get("prototype").ToObject(runtime.vm))
	_ = registry.Set("define", func(call goja.FunctionCall) goja.Value {
		name := strings.ToLower(call.Argument(0).String())
		constructor := call.Argument(1)
		if name == "" || goja.IsUndefined(constructor) || goja.IsNull(constructor) {
			panic(runtime.vm.NewTypeError("invalid custom element definition"))
		}
		if _, exists := runtime.custom_elements[name]; exists {
			panic(runtime.vm.NewTypeError("custom element %s already defined", name))
		}
		definition, err := runtime.create_custom_element_definition(name, constructor)
		if err != nil {
			panic(err)
		}
		runtime.custom_elements[name] = definition
		for _, resolve := range runtime.custom_waiters[name] {
			_ = resolve(constructor)
		}
		delete(runtime.custom_waiters, name)
		for _, node := range find_by_tag(runtime.page.Document, name) {
			if _, err := runtime.construct_custom_element(node); err != nil {
				panic(err)
			}
			runtime.connect_custom_elements(node)
		}
		runtime.flush_custom_element_reactions()
		return goja.Undefined()
	})
	_ = registry.Set("get", func(name string) goja.Value {
		if definition := runtime.custom_elements[strings.ToLower(name)]; definition != nil {
			return definition.constructor
		}
		return goja.Undefined()
	})
	_ = registry.Set("whenDefined", func(name string) goja.Value {
		name = strings.ToLower(name)
		promise, resolve, _ := runtime.vm.NewPromise()
		if definition := runtime.custom_elements[name]; definition != nil {
			_ = resolve(definition.constructor)
		} else {
			runtime.custom_waiters[name] = append(runtime.custom_waiters[name], resolve)
		}
		return runtime.vm.ToValue(promise)
	})
	_ = registry.Set("upgrade", func(call goja.FunctionCall) {
		if node := runtime.object_node(call.Argument(0)); node != nil {
			runtime.connect_custom_elements(node)
		}
	})
	_ = window.Set("customElements", registry)
}

func (runtime *page_runtime) create_element_object(name string) *goja.Object {
	node := new_element(name)
	if runtime.custom_elements[strings.ToLower(name)] != nil {
		if object, err := runtime.construct_custom_element(node); err == nil {
			return object
		} else {
			runtime.fail_script(runtime.page.URL+"#custom-element", err)
		}
	}
	return runtime.node_object(node)
}

func (runtime *page_runtime) template_content(node *html.Node) *html.Node {
	if content := runtime.template_contents[node]; content != nil {
		return content
	}
	content := &html.Node{Type: html.DocumentNode, Data: "#document-fragment"}
	runtime.fragments[content] = true
	runtime.template_contents[node] = content
	for node.FirstChild != nil {
		child := node.FirstChild
		node.RemoveChild(child)
		content.AppendChild(child)
	}
	return content
}

func (runtime *page_runtime) construct_custom_element(node *html.Node) (*goja.Object, error) {
	if runtime.custom_constructed[node] {
		return runtime.node_object(node), nil
	}
	constructor := runtime.custom_elements[strings.ToLower(node.Data)]
	if constructor == nil {
		return runtime.node_object(node), nil
	}
	initial_attributes := append([]html.Attribute(nil), node.Attr...)
	previous_object := runtime.node_object(node)
	if constructor.prototype != nil {
		_ = previous_object.SetPrototype(constructor.prototype)
	}
	runtime.pending_custom_nodes = append(runtime.pending_custom_nodes, node)
	object, err := runtime.vm.New(constructor.constructor)
	runtime.pending_custom_nodes = runtime.pending_custom_nodes[:len(runtime.pending_custom_nodes)-1]
	if err != nil {
		if previous_object == nil {
			delete(runtime.nodes, node)
		} else {
			runtime.nodes[node] = previous_object
		}
		return nil, err
	}
	if runtime.nodes[node] != object {
		runtime.bind_node_object(node, object, false)
	}
	if previous_object != nil && previous_object != object {
		previous_names := make(map[string]bool)
		for _, name := range previous_object.GetOwnPropertyNames() {
			previous_names[name] = true
		}
		for _, name := range object.GetOwnPropertyNames() {
			if !previous_names[name] {
				_ = previous_object.Set(name, object.Get(name))
			}
		}
		_ = previous_object.SetPrototype(object.Prototype())
		runtime.nodes[node] = previous_object
		object = previous_object
	}
	runtime.custom_constructed[node] = true
	for _, attr := range node.Attr {
		runtime.attribute_changed(node, attr.Key, nil, attr.Val)
	}
	for _, attr := range initial_attributes {
		if !has_attribute(node, attr.Key) {
			runtime.attribute_changed(node, attr.Key, attr.Val, nil)
		}
	}
	return object, nil
}

func (runtime *page_runtime) connect_custom_elements(root *html.Node) {
	runtime.connect_custom_elements_walk(root)
	runtime.flush_custom_element_reactions()
}

func (runtime *page_runtime) connect_custom_elements_walk(root *html.Node) {
	if root == nil {
		return
	}
	if root.Type == html.ElementNode && runtime.custom_elements[strings.ToLower(root.Data)] != nil {
		_, err := runtime.construct_custom_element(root)
		if err != nil {
			runtime.fail_script(runtime.page.URL+"#custom-element", err)
			return
		}
		if !runtime.custom_connected[root] && contains_node(runtime.page.Document, root) {
			runtime.custom_connected[root] = true
			runtime.enqueue_custom_element_reaction(root, "connectedCallback")
		}
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		runtime.connect_custom_elements_walk(child)
	}
}

func (runtime *page_runtime) custom_observes_attribute(node *html.Node, name string) bool {
	definition := runtime.custom_elements[strings.ToLower(node.Data)]
	if definition == nil {
		return false
	}
	return definition.observed_attributes[strings.ToLower(name)]
}

func (runtime *page_runtime) attribute_changed(node *html.Node, name string, old_value any, new_value any) {
	if !runtime.custom_constructed[node] || !runtime.custom_observes_attribute(node, name) {
		return
	}
	runtime.enqueue_custom_element_reaction(node, "attributeChangedCallback", runtime.vm.ToValue(name), runtime.vm.ToValue(old_value), runtime.vm.ToValue(new_value))
	runtime.flush_custom_element_reactions()
}

func (runtime *page_runtime) set_element_attribute(node *html.Node, name string, value string) {
	old_value, exists := find_attribute(node, name)
	set_attribute(node, name, value)
	runtime.invalidate_node_styles(node)
	if exists {
		runtime.attribute_changed(node, name, old_value, value)
	} else {
		runtime.attribute_changed(node, name, nil, value)
	}
}

func (runtime *page_runtime) remove_element_attribute(node *html.Node, name string) {
	old_value, exists := find_attribute(node, name)
	if !exists {
		return
	}
	remove_attribute(node, name)
	runtime.invalidate_node_styles(node)
	runtime.attribute_changed(node, name, old_value, nil)
}

func (runtime *page_runtime) set_node_prototype(object *goja.Object, node *html.Node) {
	constructor_name := "Node"
	if node.Type == html.TextNode {
		constructor_name = "Text"
	} else if node.Type == html.CommentNode {
		constructor_name = "Comment"
	} else if node.Type == html.DoctypeNode {
		constructor_name = "DocumentType"
	} else if runtime.shadow_hosts[node] != nil {
		constructor_name = "ShadowRoot"
	} else if runtime.fragments[node] {
		constructor_name = "DocumentFragment"
	} else if node.Type == html.DocumentNode {
		constructor_name = "HTMLDocument"
	} else if node.Type == html.ElementNode {
		constructor_name = "HTMLElement"
		if strings.EqualFold(node.Data, "body") {
			constructor_name = "HTMLBodyElement"
		} else if strings.EqualFold(node.Data, "html") {
			constructor_name = "HTMLHtmlElement"
		} else if strings.EqualFold(node.Data, "img") {
			constructor_name = "HTMLImageElement"
		} else if strings.EqualFold(node.Data, "iframe") {
			constructor_name = "HTMLIFrameElement"
		} else if strings.EqualFold(node.Data, "template") {
			constructor_name = "HTMLTemplateElement"
		} else if strings.EqualFold(node.Data, "audio") {
			constructor_name = "HTMLAudioElement"
		} else if strings.EqualFold(node.Data, "video") {
			constructor_name = "HTMLVideoElement"
		} else if node.Namespace == "svg" {
			constructor_name = "SVGElement"
		}
	}
	constructor := runtime.vm.Get(constructor_name).ToObject(runtime.vm)
	_ = object.SetPrototype(constructor.Get("prototype").ToObject(runtime.vm))
}

func (runtime *page_runtime) install_node_properties(object *goja.Object, node *html.Node) {
	define_getter(runtime.vm, object, "nodeType", func() any { return dom_node_type(node) })
	define_getter(runtime.vm, object, "nodeName", func() any { return dom_node_name(node) })
	define_getter(runtime.vm, object, "parentNode", func() any { return runtime.node_object(node.Parent) })
	define_getter(runtime.vm, object, "isConnected", func() any { return contains_node(runtime.page.Document, node) })
	define_getter(runtime.vm, object, "parentElement", func() any {
		if node.Parent != nil && node.Parent.Type == html.ElementNode {
			return runtime.node_object(node.Parent)
		}
		return nil
	})
	define_getter(runtime.vm, object, "firstChild", func() any { return runtime.node_object(node.FirstChild) })
	define_getter(runtime.vm, object, "lastChild", func() any { return runtime.node_object(node.LastChild) })
	define_getter(runtime.vm, object, "nextSibling", func() any { return runtime.node_object(node.NextSibling) })
	define_getter(runtime.vm, object, "previousSibling", func() any { return runtime.node_object(node.PrevSibling) })
	define_getter(runtime.vm, object, "childNodes", func() any { return runtime.node_array(children(node, false)) })
	define_getter(runtime.vm, object, "children", func() any { return runtime.node_array(children(node, true)) })
	define_getter(runtime.vm, object, "ownerDocument", func() any {
		if node.Type == html.DocumentNode && !runtime.fragments[node] {
			return nil
		}
		return runtime.node_object(runtime.page.Document)
	})
	define_accessor(runtime.vm, object, "textContent", func() any { return text_content(node) }, func(value goja.Value) {
		if node.Type == html.TextNode || node.Type == html.CommentNode {
			node.Data = value.String()
			runtime.notify_mutation(node)
			return
		}
		runtime.remove_all_children(node)
		if text := value.String(); text != "" {
			node.AppendChild(&html.Node{Type: html.TextNode, Data: text})
		}
		runtime.invalidate_styles()
	})
	define_accessor(runtime.vm, object, "nodeValue", func() any {
		if node.Type == html.TextNode || node.Type == html.CommentNode {
			return node.Data
		}
		return nil
	}, func(value goja.Value) {
		if node.Type == html.TextNode || node.Type == html.CommentNode {
			node.Data = value.String()
			runtime.notify_mutation(node)
		}
	})
	if node.Type == html.TextNode || node.Type == html.CommentNode {
		define_accessor(runtime.vm, object, "data", func() any { return node.Data }, func(value goja.Value) {
			node.Data = value.String()
			runtime.notify_mutation(node)
		})
	}
	define_accessor(runtime.vm, object, "innerHTML", func() any { return render_children(node) }, func(value goja.Value) {
		runtime.remove_all_children(node)
		append_html(node, value.String())
		runtime.invalidate_styles()
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			runtime.queue_dynamic_resource(child)
		}
	})
	define_getter(runtime.vm, object, "outerHTML", func() any { return render_node(node) })
}

func (runtime *page_runtime) notify_mutation(node *html.Node) {
	runtime.invalidate_node_styles(node)
	object := runtime.node_object(node)
	callback, ok := goja.AssertFunction(object.Get("__minib_mutation_callback"))
	if !ok {
		return
	}
	observer := object.Get("__minib_mutation_observer")
	runtime.queue_host_job(func() {
		if _, err := runtime.call_javascript(runtime.ctx, callback, observer, runtime.vm.NewArray(), observer); err != nil {
			runtime.fail_script(runtime.page.URL+"#mutation-observer", err)
		}
	})
}

func (runtime *page_runtime) install_node_methods(object *goja.Object, node *html.Node) {
	_ = object.Set("appendChild", func(call goja.FunctionCall) goja.Value {
		child := runtime.object_node(call.Argument(0))
		if child == nil {
			return goja.Null()
		}
		if runtime.fragments[child] {
			for child.FirstChild != nil {
				fragment_child := child.FirstChild
				child.RemoveChild(fragment_child)
				node.AppendChild(fragment_child)
				runtime.queue_dynamic_resource(fragment_child)
			}
			return runtime.node_object(child)
		}
		runtime.detach_node(child)
		node.AppendChild(child)
		runtime.queue_dynamic_resource(child)
		return runtime.node_object(child)
	})
	_ = object.Set("removeChild", func(call goja.FunctionCall) goja.Value {
		child := runtime.object_node(call.Argument(0))
		if child == nil || child.Parent != node {
			return goja.Null()
		}
		runtime.detach_node(child)
		runtime.invalidate_styles()
		return runtime.node_object(child)
	})
	_ = object.Set("replaceChild", func(call goja.FunctionCall) goja.Value {
		child := runtime.object_node(call.Argument(0))
		old_child := runtime.object_node(call.Argument(1))
		if child == nil || old_child == nil || old_child.Parent != node {
			return goja.Null()
		}
		if child == old_child {
			return runtime.node_object(old_child)
		}
		if runtime.fragments[child] {
			inserted := make([]*html.Node, 0)
			for child.FirstChild != nil {
				fragment_child := child.FirstChild
				child.RemoveChild(fragment_child)
				node.InsertBefore(fragment_child, old_child)
				inserted = append(inserted, fragment_child)
			}
			runtime.detach_node(old_child)
			for _, fragment_child := range inserted {
				runtime.queue_dynamic_resource(fragment_child)
			}
			runtime.invalidate_styles()
			return runtime.node_object(old_child)
		}
		runtime.detach_node(child)
		node.InsertBefore(child, old_child)
		runtime.detach_node(old_child)
		runtime.invalidate_styles()
		runtime.queue_dynamic_resource(child)
		return runtime.node_object(old_child)
	})
	_ = object.Set("insertBefore", func(call goja.FunctionCall) goja.Value {
		child := runtime.object_node(call.Argument(0))
		mark := runtime.object_node(call.Argument(1))
		if child == nil {
			return goja.Null()
		}
		if child == mark {
			return runtime.node_object(child)
		}
		if runtime.fragments[child] {
			for child.FirstChild != nil {
				fragment_child := child.FirstChild
				child.RemoveChild(fragment_child)
				if mark != nil && mark.Parent == node {
					node.InsertBefore(fragment_child, mark)
				} else {
					node.AppendChild(fragment_child)
				}
				runtime.queue_dynamic_resource(fragment_child)
			}
			return runtime.node_object(child)
		}
		runtime.detach_node(child)
		if mark != nil && mark.Parent == node {
			node.InsertBefore(child, mark)
		} else {
			node.AppendChild(child)
		}
		runtime.queue_dynamic_resource(child)
		return runtime.node_object(child)
	})
	_ = object.Set("insertAdjacentElement", func(position string, value goja.Value) any {
		child := runtime.object_node(value)
		if child == nil {
			return nil
		}
		position = strings.ToLower(position)
		if (position == "beforebegin" || position == "afterend") && node.Parent == nil {
			return nil
		}
		if position != "beforebegin" && position != "afterbegin" && position != "beforeend" && position != "afterend" {
			return nil
		}
		runtime.detach_node(child)
		switch position {
		case "beforebegin":
			node.Parent.InsertBefore(child, node)
		case "afterbegin":
			node.InsertBefore(child, node.FirstChild)
		case "beforeend":
			node.AppendChild(child)
		case "afterend":
			if node.NextSibling == nil {
				node.Parent.AppendChild(child)
			} else {
				node.Parent.InsertBefore(child, node.NextSibling)
			}
		}
		runtime.queue_dynamic_resource(child)
		return runtime.node_object(child)
	})
	_ = object.Set("cloneNode", func(deep bool) any {
		clone := runtime.clone_node(node, deep)
		return runtime.node_object(clone)
	})
	_ = object.Set("contains", func(call goja.FunctionCall) goja.Value {
		return runtime.vm.ToValue(contains_node(node, runtime.object_node(call.Argument(0))))
	})
	runtime.install_node_events(object, node)
	for _, name := range []string{"appendChild", "removeChild", "replaceChild", "insertBefore", "cloneNode", "contains"} {
		_ = object.Set("__minib_"+name, object.Get(name))
	}
}

func (runtime *page_runtime) install_document(object *goja.Object, node *html.Node) {
	define_getter(runtime.vm, object, "documentElement", func() any { return runtime.node_object(find_element(node, "html")) })
	define_getter(runtime.vm, object, "head", func() any { return runtime.node_object(find_element(node, "head")) })
	define_getter(runtime.vm, object, "body", func() any { return runtime.node_object(find_element(node, "body")) })
	define_getter(runtime.vm, object, "currentScript", func() any { return runtime.node_object(runtime.current_script) })
	define_getter(runtime.vm, object, "readyState", func() any { return runtime.ready_state })
	define_getter(runtime.vm, object, "URL", func() any { return runtime.page.URL })
	define_getter(runtime.vm, object, "documentURI", func() any { return runtime.page.URL })
	define_getter(runtime.vm, object, "baseURI", func() any { return runtime.base_url.String() })
	define_getter(runtime.vm, object, "domain", func() any { return runtime.page_url.Hostname() })
	define_getter(runtime.vm, object, "referrer", func() any { return "" })
	define_getter(runtime.vm, object, "hidden", func() any { return false })
	define_getter(runtime.vm, object, "visibilityState", func() any { return "visible" })
	define_getter(runtime.vm, object, "activeElement", func() any { return runtime.node_object(find_element(node, "body")) })
	define_getter(runtime.vm, object, "scripts", func() any { return runtime.node_array(find_by_tag(node, "script")) })
	define_getter(runtime.vm, object, "styleSheets", func() any { return runtime.style_sheet_list_object() })
	define_getter(runtime.vm, object, "implementation", func() any { return runtime.document_implementation() })
	define_getter(runtime.vm, object, "defaultView", func() any { return runtime.vm.GlobalObject() })
	define_getter(runtime.vm, object, "location", func() any { return runtime.vm.GlobalObject().Get("location") })
	define_accessor(runtime.vm, object, "title", func() any { return text_content(find_element(node, "title")) }, func(value goja.Value) { set_document_title(node, value.String()) })
	define_accessor(runtime.vm, object, "cookie", func() any {
		persistent_header, _ := runtime.browser.persistent_cookie_header(runtime.page.URL)
		return runtime.browser.cookie_header(runtime.page_url, persistent_header, "")
	}, func(value goja.Value) { runtime.set_cookie(value.String()) })
	_ = object.Set("createElement", func(name string) any { return runtime.create_element_object(name) })
	_ = object.Set("createElementNS", func(_ string, name string) any {
		return runtime.node_object(new_element(name))
	})
	_ = object.Set("createTextNode", func(value string) any { return runtime.node_object(&html.Node{Type: html.TextNode, Data: value}) })
	_ = object.Set("createDocumentFragment", func() any {
		fragment := &html.Node{Type: html.DocumentNode, Data: "#document-fragment"}
		runtime.fragments[fragment] = true
		return runtime.node_object(fragment)
	})
	_ = object.Set("createComment", func(value string) any { return runtime.node_object(&html.Node{Type: html.CommentNode, Data: value}) })
	_ = object.Set("createEvent", func(name string) any { return runtime.event_object(name) })
	_ = object.Set("createRange", func() any { return runtime.range_object(node) })
	_ = object.Set("importNode", func(call goja.FunctionCall) goja.Value {
		node := runtime.object_node(call.Argument(0))
		if node == nil {
			return goja.Null()
		}
		clone := runtime.clone_node(node, call.Argument(1).ToBoolean())
		return runtime.node_object(clone)
	})
	_ = object.Set("getElementById", func(id string) any { return runtime.nullable_node_value(find_by_attribute(node, "id", id)) })
	_ = object.Set("getElementsByName", func(name string) any { return runtime.node_array(find_all_by_attribute(node, "name", name)) })
	_ = object.Set("getElementsByTagName", func(name string) any { return runtime.node_array(find_by_tag(node, name)) })
	_ = object.Set("getElementsByClassName", func(name string) any { return runtime.node_array(find_by_class(node, name)) })
	_ = object.Set("querySelector", func(selector string) any { return runtime.nullable_node_value(query_first(node, selector)) })
	_ = object.Set("querySelectorAll", func(selector string) any { return runtime.node_array(query_all(node, selector)) })
	_ = object.Set("elementFromPoint", func(float64, float64) any { return runtime.node_object(find_element(node, "body")) })
	_ = object.Set("write", func(markup string) {
		target := find_element(node, "body")
		if target != nil {
			for _, fragment := range append_html(target, markup) {
				runtime.queue_dynamic_resource(fragment)
			}
		}
	})
	_ = object.Set("writeln", func(markup string) {
		target := find_element(node, "body")
		if target != nil {
			for _, fragment := range append_html(target, markup+"\n") {
				runtime.queue_dynamic_resource(fragment)
			}
		}
	})
	for _, name := range []string{"createElement", "createElementNS", "createTextNode", "createDocumentFragment", "createComment", "createEvent", "createRange", "importNode", "getElementById", "getElementsByName", "getElementsByTagName", "getElementsByClassName", "querySelector", "querySelectorAll"} {
		_ = object.Set("__minib_"+name, object.Get(name))
	}
}

func (runtime *page_runtime) install_element(object *goja.Object, node *html.Node) {
	define_getter(runtime.vm, object, "tagName", func() any { return strings.ToUpper(node.Data) })
	define_getter(runtime.vm, object, "localName", func() any { return strings.ToLower(node.Data) })
	define_accessor(runtime.vm, object, "id", func() any { return attribute(node, "id") }, func(value goja.Value) { runtime.set_element_attribute(node, "id", value.String()) })
	define_accessor(runtime.vm, object, "className", func() any { return attribute(node, "class") }, func(value goja.Value) { runtime.set_element_attribute(node, "class", value.String()) })
	for _, name := range []string{"src", "href", "value", "name", "type", "rel", "content", "charset"} {
		attribute_name := name
		define_accessor(runtime.vm, object, name, func() any { return attribute(node, attribute_name) }, func(value goja.Value) {
			runtime.set_element_attribute(node, attribute_name, value.String())
			runtime.queue_dynamic_resource(node)
		})
	}
	define_getter(runtime.vm, object, "style", func() any { return runtime.style_object(node) })
	if strings.EqualFold(node.Data, "style") || strings.EqualFold(node.Data, "link") {
		define_getter(runtime.vm, object, "sheet", func() any {
			runtime.refresh_style_sheets()
			if sheet := runtime.style_sheet_by_node[node]; sheet != nil {
				return runtime.css_style_sheet_object(sheet)
			}
			return nil
		})
	}
	define_getter(runtime.vm, object, "classList", func() any { return runtime.class_list_object(node) })
	define_getter(runtime.vm, object, "dataset", func() any { return runtime.dataset_object(node) })
	define_getter(runtime.vm, object, "attributes", func() any { return runtime.attributes_object(node) })
	if strings.EqualFold(node.Data, "iframe") {
		define_getter(runtime.vm, object, "contentWindow", func() any { return runtime.vm.GlobalObject() })
		define_getter(runtime.vm, object, "contentDocument", func() any { return runtime.node_object(runtime.page.Document) })
	}
	if strings.EqualFold(node.Data, "template") {
		define_getter(runtime.vm, object, "content", func() any { return runtime.node_object(runtime.template_content(node)) })
		define_accessor(runtime.vm, object, "innerHTML", func() any { return render_children(runtime.template_content(node)) }, func(value goja.Value) {
			content := runtime.template_content(node)
			markup := value.String()
			set_inner_html(content, markup)
			for child := content.FirstChild; child != nil; child = child.NextSibling {
				runtime.queue_dynamic_resource(child)
			}
		})
	}
	if strings.EqualFold(node.Data, "audio") {
		_ = object.Set("canPlayType", func(string) string { return "probably" })
	}
	if strings.EqualFold(node.Data, "a") {
		define_accessor(runtime.vm, object, "href", func() any { return runtime.element_url(node).String() }, func(value goja.Value) { runtime.set_element_attribute(node, "href", value.String()) })
		for _, name := range []string{"protocol", "host", "hostname", "port", "pathname", "search", "hash", "origin"} {
			property_name := name
			define_getter(runtime.vm, object, property_name, func() any { return url_property(runtime.element_url(node), property_name) })
		}
	}
	for _, name := range []string{"clientWidth", "clientHeight", "offsetWidth", "offsetHeight", "scrollWidth", "scrollHeight", "scrollTop", "scrollLeft"} {
		define_getter(runtime.vm, object, name, func() any { return 0 })
	}
	_ = object.Set("getAttribute", func(name string) any {
		value, ok := find_attribute(node, name)
		if !ok {
			return nil
		}
		return value
	})
	_ = object.Set("setAttribute", func(name string, value string) {
		runtime.set_element_attribute(node, name, value)
		runtime.queue_dynamic_resource(node)
	})
	_ = object.Set("getAttributeNS", func(_ string, name string) any {
		value, ok := find_attribute(node, name)
		if !ok {
			return nil
		}
		return value
	})
	_ = object.Set("setAttributeNS", func(_ string, name string, value string) {
		runtime.set_element_attribute(node, name, value)
		runtime.queue_dynamic_resource(node)
	})
	_ = object.Set("removeAttribute", func(name string) { runtime.remove_element_attribute(node, name) })
	_ = object.Set("removeAttributeNS", func(_ string, name string) { runtime.remove_element_attribute(node, name) })
	_ = object.Set("hasAttribute", func(name string) bool { _, ok := find_attribute(node, name); return ok })
	_ = object.Set("hasAttributeNS", func(_ string, name string) bool { _, ok := find_attribute(node, name); return ok })
	_ = object.Set("hasAttributes", func() bool { return len(node.Attr) > 0 })
	_ = object.Set("getAttributeNames", func() []string { return element_attribute_names(node) })
	_ = object.Set("querySelector", func(selector string) any { return runtime.nullable_node_value(query_first(node, selector)) })
	_ = object.Set("querySelectorAll", func(selector string) any { return runtime.node_array(query_all(node, selector)) })
	_ = object.Set("getElementsByTagName", func(name string) any { return runtime.node_array(find_by_tag(node, name)) })
	_ = object.Set("getElementsByClassName", func(name string) any { return runtime.node_array(find_by_class(node, name)) })
	_ = object.Set("matches", func(selector string) bool {
		matcher, err := cascadia.ParseGroup(selector)
		return err == nil && matcher.Match(node)
	})
	_ = object.Set("closest", func(selector string) any {
		matcher, err := cascadia.ParseGroup(selector)
		if err != nil {
			return nil
		}
		for current := node; current != nil; current = current.Parent {
			if current.Type == html.ElementNode && matcher.Match(current) {
				return runtime.node_object(current)
			}
		}
		return nil
	})
	bounding_rect := func() map[string]float64 {
		if contains_node(runtime.page.Document, node) {
			// Geometry is synthetic because minib deliberately has no layout or
			// rendering backend yet.
			return map[string]float64{"x": 0, "y": 0, "top": 0, "right": 100, "bottom": 20, "left": 0, "width": 100, "height": 20}
		}
		return map[string]float64{"x": 0, "y": 0, "top": 0, "right": 0, "bottom": 0, "left": 0, "width": 0, "height": 0}
	}
	_ = object.Set("getBoundingClientRect", bounding_rect)
	_ = object.Set("getClientRects", func() []map[string]float64 {
		if !contains_node(runtime.page.Document, node) {
			return nil
		}
		return []map[string]float64{bounding_rect()}
	})
	_ = object.Set("focus", func() {})
	_ = object.Set("blur", func() {})
	_ = object.Set("click", func() { runtime.fire_node_event(node, "click") })
	_ = object.Set("getContext", func(context_type string) any {
		if strings.EqualFold(node.Data, "canvas") && strings.EqualFold(context_type, "2d") {
			return runtime.canvas_context_object()
		}
		return nil
	})
	_ = object.Set("toDataURL", func() string { return "data:image/png;base64," })
	for _, name := range []string{"insertAdjacentElement", "getAttribute", "setAttribute", "getAttributeNS", "setAttributeNS", "removeAttribute", "removeAttributeNS", "hasAttribute", "hasAttributeNS", "hasAttributes", "getAttributeNames", "querySelector", "querySelectorAll", "getElementsByTagName", "getElementsByClassName", "matches", "closest", "getBoundingClientRect", "getClientRects"} {
		_ = object.Set("__minib_"+name, object.Get(name))
	}
}

func element_attribute_names(node *html.Node) []string {
	attribute_names := make([]string, len(node.Attr))
	for attribute_index, attribute := range node.Attr {
		attribute_names[attribute_index] = attribute.Key
	}
	return attribute_names
}

func (runtime *page_runtime) document_implementation() *goja.Object {
	object := runtime.vm.NewObject()
	_ = object.Set("hasFeature", func() bool { return true })
	_ = object.Set("createHTMLDocument", func(title string) any {
		document := &html.Node{Type: html.DocumentNode}
		html_node := new_element("html")
		head := new_element("head")
		body := new_element("body")
		document.AppendChild(html_node)
		html_node.AppendChild(head)
		html_node.AppendChild(body)
		if title != "" {
			title_node := new_element("title")
			title_node.AppendChild(&html.Node{Type: html.TextNode, Data: title})
			head.AppendChild(title_node)
		}
		return runtime.node_object(document)
	})
	return object
}

func (runtime *page_runtime) canvas_context_object() *goja.Object {
	object := runtime.vm.NewObject()
	no_op := func(...any) {}
	for _, name := range []string{
		"arc", "beginPath", "bezierCurveTo", "clearRect", "clip", "closePath",
		"drawImage", "fill", "fillRect", "fillText", "lineTo", "moveTo",
		"quadraticCurveTo", "rect", "restore", "rotate", "save", "scale",
		"setTransform", "stroke", "strokeRect", "strokeText", "transform", "translate",
	} {
		_ = object.Set(name, no_op)
	}
	_ = object.Set("measureText", func(text string) map[string]float64 { return map[string]float64{"width": float64(len(text)) * 8} })
	_ = object.Set("getImageData", func() map[string]any { return map[string]any{"data": []byte{0, 0, 0, 0}, "width": 1, "height": 1} })
	_ = object.Set("createLinearGradient", func() *goja.Object { return runtime.canvas_gradient_object() })
	_ = object.Set("createRadialGradient", func() *goja.Object { return runtime.canvas_gradient_object() })
	_ = object.Set("createPattern", func() any { return nil })
	return object
}

func (runtime *page_runtime) canvas_gradient_object() *goja.Object {
	object := runtime.vm.NewObject()
	_ = object.Set("addColorStop", func(...any) {})
	return object
}

func (runtime *page_runtime) class_list_object(node *html.Node) *goja.Object {
	object := runtime.vm.NewObject()
	_ = object.SetPrototype(runtime.vm.Get("DOMTokenList").ToObject(runtime.vm).Get("prototype").ToObject(runtime.vm))
	classes := class_values(node)
	for index, name := range classes {
		_ = object.Set(fmt.Sprintf("%d", index), name)
	}
	_ = object.Set("length", len(classes))
	_ = object.Set("item", func(index int) any {
		if index < 0 || index >= len(classes) {
			return nil
		}
		return classes[index]
	})
	_ = object.Set("contains", func(name string) bool { return has_class(node, name) })
	_ = object.Set("add", func(names ...string) {
		classes := class_values(node)
		for _, name := range names {
			if !slice_contains(classes, name) {
				classes = append(classes, name)
			}
		}
		runtime.set_element_attribute(node, "class", strings.Join(classes, " "))
	})
	_ = object.Set("remove", func(names ...string) {
		classes := class_values(node)
		kept := classes[:0]
		for _, class_name := range classes {
			if !slice_contains(names, class_name) {
				kept = append(kept, class_name)
			}
		}
		runtime.set_element_attribute(node, "class", strings.Join(kept, " "))
	})
	_ = object.Set("toggle", func(name string) bool {
		if has_class(node, name) {
			classes := class_values(node)
			kept := classes[:0]
			for _, value := range classes {
				if value != name {
					kept = append(kept, value)
				}
			}
			runtime.set_element_attribute(node, "class", strings.Join(kept, " "))
			return false
		}
		classes := append(class_values(node), name)
		runtime.set_element_attribute(node, "class", strings.Join(classes, " "))
		return true
	})
	_ = object.Set("toString", func() string { return attribute(node, "class") })
	return object
}

func (runtime *page_runtime) dataset_object(node *html.Node) *goja.Object {
	object := runtime.vm.NewObject()
	for _, attr := range node.Attr {
		if strings.HasPrefix(attr.Key, "data-") {
			_ = object.Set(dataset_name(strings.TrimPrefix(attr.Key, "data-")), attr.Val)
		}
	}
	return object
}

func (runtime *page_runtime) attributes_object(node *html.Node) *goja.Object {
	object := runtime.vm.NewObject()
	for index, attr := range node.Attr {
		attribute_object := runtime.vm.NewObject()
		_ = attribute_object.Set("name", attr.Key)
		_ = attribute_object.Set("value", attr.Val)
		_ = attribute_object.Set("nodeName", attr.Key)
		_ = attribute_object.Set("nodeValue", attr.Val)
		_ = attribute_object.Set("specified", true)
		_ = attribute_object.Set("expando", false)
		_ = object.Set(attr.Key, attribute_object)
		_ = object.Set(fmt.Sprintf("%d", index), attribute_object)
	}
	_ = object.Set("length", len(node.Attr))
	_ = object.Set("item", func(index int) any {
		if index < 0 || index >= len(node.Attr) {
			return nil
		}
		return object.Get(fmt.Sprintf("%d", index))
	})
	_ = object.Set("getNamedItem", func(name string) any { return object.Get(name) })
	return object
}

func (runtime *page_runtime) element_url(node *html.Node) *url.URL {
	parsed_url, err := runtime.page_url.Parse(attribute(node, "href"))
	if err != nil {
		return &url.URL{}
	}
	return parsed_url
}

func url_property(parsed_url *url.URL, name string) string {
	switch name {
	case "protocol":
		if parsed_url.Scheme == "" {
			return ""
		}
		return parsed_url.Scheme + ":"
	case "host":
		return parsed_url.Host
	case "hostname":
		return parsed_url.Hostname()
	case "port":
		return parsed_url.Port()
	case "pathname":
		if parsed_url.EscapedPath() == "" {
			return "/"
		}
		return parsed_url.EscapedPath()
	case "search":
		if parsed_url.RawQuery != "" {
			return "?" + parsed_url.RawQuery
		}
	case "hash":
		if parsed_url.Fragment != "" {
			return "#" + parsed_url.Fragment
		}
	case "origin":
		if parsed_url.Scheme != "" && parsed_url.Host != "" {
			return parsed_url.Scheme + "://" + parsed_url.Host
		}
	}
	return ""
}

func dataset_name(value string) string {
	parts := strings.Split(value, "-")
	for index := 1; index < len(parts); index++ {
		if parts[index] != "" {
			parts[index] = strings.ToUpper(parts[index][:1]) + parts[index][1:]
		}
	}
	return strings.Join(parts, "")
}

func (runtime *page_runtime) node_array(nodes []*html.Node) *goja.Object {
	values := make([]any, 0, len(nodes))
	for _, node := range nodes {
		values = append(values, runtime.node_object(node))
	}
	array := runtime.vm.NewArray(values...)
	_ = array.Set("item", func(index int) any {
		if index < 0 || index >= len(nodes) {
			return nil
		}
		return runtime.node_object(nodes[index])
	})
	return array
}

func (runtime *page_runtime) object_node(value goja.Value) *html.Node {
	if goja.IsNull(value) || goja.IsUndefined(value) {
		return nil
	}
	object := value.ToObject(runtime.vm)
	if node := runtime.object_nodes[object]; node != nil {
		return node
	}
	wrapped := object.Get("node")
	if wrapped == nil || goja.IsNull(wrapped) || goja.IsUndefined(wrapped) {
		return nil
	}
	return runtime.object_node(wrapped)
}

func (runtime *page_runtime) queue_dynamic_resource(node *html.Node) {
	if node == nil || node.Type != html.ElementNode || !contains_node(runtime.page.Document, node) {
		return
	}
	// Custom-element reactions run after the current constructor. Connecting a
	// child synchronously while its parent is still being upgraded lets the child
	// fire events before the parent's connectedCallback can install listeners.
	// The outer connect_custom_elements walk will connect these descendants in
	// tree order once construction has completed.
	if len(runtime.pending_custom_nodes) == 0 {
		runtime.connect_custom_elements(node)
	}
	if runtime.dynamic_seen[node] {
		return
	}
	switch {
	case runtime.page.disable_subresources && strings.EqualFold(node.Data, "script") && attribute(node, "src") != "":
		runtime.dynamic_seen[node] = true
		runtime.queue_host_job(func() { runtime.fire_node_event(node, "load") })
	case runtime.page.disable_subresources && strings.EqualFold(node.Data, "link"):
		runtime.dynamic_seen[node] = true
		runtime.queue_host_job(func() { runtime.fire_node_event(node, "load") })
	case runtime.page.disable_subresources && strings.EqualFold(node.Data, "img"):
		runtime.dynamic_seen[node] = true
		runtime.queue_host_job(func() { runtime.fire_node_event(node, "load") })
	case runtime.disable_css && strings.EqualFold(node.Data, "style"):
	case runtime.disable_css && strings.EqualFold(node.Data, "link") && has_rel_value(node, "stylesheet"):
		runtime.dynamic_seen[node] = true
		runtime.queue_host_job(func() { runtime.fire_node_event(node, "load") })
	case runtime.page.disable_javascript && strings.EqualFold(node.Data, "script"):
		runtime.dynamic_seen[node] = true
		runtime.queue_host_job(func() { runtime.fire_node_event(node, "load") })
	case runtime.page.disable_images && strings.EqualFold(node.Data, "img"):
		runtime.dynamic_seen[node] = true
		runtime.queue_host_job(func() { runtime.fire_node_event(node, "load") })
	case strings.EqualFold(node.Data, "style"):
	case strings.EqualFold(node.Data, "script"):
		runtime.dynamic_seen[node] = true
		runtime.dynamic_scripts = append(runtime.dynamic_scripts, node)
	case strings.EqualFold(node.Data, "link") && has_rel_value(node, "stylesheet"):
		runtime.dynamic_seen[node] = true
		runtime.dynamic_styles = append(runtime.dynamic_styles, node)
	case strings.EqualFold(node.Data, "img") && attribute(node, "src") != "":
		runtime.dynamic_seen[node] = true
		runtime.dynamic_resources = append(runtime.dynamic_resources, node)
	}
}

func (runtime *page_runtime) drain_dynamic_styles(ctx context.Context) {
	if runtime.disable_css {
		runtime.dynamic_styles = nil
		return
	}
	for loaded := 0; len(runtime.dynamic_styles) > 0 && loaded < max_dynamic_scripts && ctx.Err() == nil; loaded++ {
		if runtime.wait_condition_met() {
			return
		}
		node := runtime.dynamic_styles[0]
		runtime.dynamic_styles = runtime.dynamic_styles[1:]
		resource_url, ok := resolve_resource_url(runtime.base_url, attribute(node, "href"))
		if !ok {
			runtime.fire_node_event(node, "error")
			continue
		}
		resource_index := runtime.find_or_download_resource(ctx, resource_url, StyleResource)
		if runtime.page.Resources[resource_index].Err != nil {
			runtime.fire_node_event(node, "error")
		} else {
			runtime.fire_node_event(node, "load")
		}
		if runtime.wait_condition_met() {
			return
		}
	}
}

func (runtime *page_runtime) drain_dynamic_scripts(ctx context.Context) {
	for loaded := 0; len(runtime.dynamic_scripts) > 0 && loaded < max_dynamic_scripts && ctx.Err() == nil; loaded++ {
		if runtime.wait_condition_met() {
			return
		}
		node := runtime.dynamic_scripts[0]
		runtime.dynamic_scripts = runtime.dynamic_scripts[1:]
		source_url := attribute(node, "src")
		job := script_job{
			node:           node,
			resource_index: -1,
			inline:         text_content(node),
			source_url:     runtime.page.URL + "#dynamic-script",
			module_script:  strings.EqualFold(strings.TrimSpace(attribute(node, "type")), "module"),
		}
		if source_url != "" {
			resolved_url, ok := resolve_resource_url(runtime.base_url, source_url)
			if !ok {
				runtime.fail_script(source_url, fmt.Errorf("invalid script URL"))
				continue
			}
			resource_index := runtime.find_or_download_resource(ctx, resolved_url, ScriptResource)
			job.resource_index = resource_index
			job.source_url = resolved_url
		}
		if strings.TrimSpace(job.inline) == "" && job.resource_index < 0 {
			continue
		}
		runtime.execute_job(ctx, job)
		if runtime.wait_condition_met() {
			return
		}
		if len(runtime.page.ScriptFailures) == 0 || runtime.page.ScriptFailures[len(runtime.page.ScriptFailures)-1].URL != job.source_url {
			runtime.fire_node_event(node, "load")
		} else {
			runtime.fire_node_event(node, "error")
		}
		if runtime.wait_condition_met() {
			return
		}
	}
}

func (runtime *page_runtime) drain_dynamic_resources(ctx context.Context) {
	for loaded := 0; len(runtime.dynamic_resources) > 0 && loaded < max_dynamic_resources && ctx.Err() == nil; loaded++ {
		if runtime.wait_condition_met() {
			return
		}
		node := runtime.dynamic_resources[0]
		runtime.dynamic_resources = runtime.dynamic_resources[1:]
		resource_url, ok := resolve_resource_url(runtime.base_url, attribute(node, "src"))
		if !ok {
			runtime.fire_node_event(node, "error")
			continue
		}
		resource_index := runtime.find_or_download_resource(ctx, resource_url, ImageResource)
		if runtime.page.Resources[resource_index].Err != nil {
			runtime.fire_node_event(node, "error")
		} else {
			runtime.fire_node_event(node, "load")
		}
		if runtime.wait_condition_met() {
			return
		}
	}
}

func (runtime *page_runtime) find_or_download_resource(ctx context.Context, resource_url string, kind ResourceKind) int {
	for index := range runtime.page.Resources {
		if runtime.page.Resources[index].URL == resource_url {
			return index
		}
	}
	resource_ctx, cancel := context_with_optional_timeout(ctx, runtime.page.resource_timeout)
	defer cancel()
	resource := runtime.browser.download_resource(resource_ctx, runtime.page_url, runtime.request_headers, Resource{URL: resource_url, Kind: kind, fetch_priority: default_resource_priority(kind)}, runtime.page.disable_cache)
	runtime.page.Resources = append(runtime.page.Resources, resource)
	return len(runtime.page.Resources) - 1
}

func context_with_optional_timeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, timeout)
}

func (runtime *page_runtime) set_cookie(raw_cookie string) {
	response := &http.Response{Header: http.Header{"Set-Cookie": []string{raw_cookie}}}
	for _, cookie := range response.Cookies() {
		_ = runtime.browser.SetCookie(runtime.page.URL, cookie)
	}
}

func define_getter(vm *goja.Runtime, object *goja.Object, name string, getter func() any) {
	_ = object.DefineAccessorProperty(name, vm.ToValue(getter), nil, goja.FLAG_TRUE, goja.FLAG_TRUE)
}

func event_type(vm *goja.Runtime, value goja.Value) (string, bool) {
	if goja.IsNull(value) || goja.IsUndefined(value) {
		return "", false
	}
	type_value := value.ToObject(vm).Get("type")
	if type_value == nil || goja.IsNull(type_value) || goja.IsUndefined(type_value) {
		return "", false
	}
	return type_value.String(), true
}

func define_accessor(vm *goja.Runtime, object *goja.Object, name string, getter func() any, setter func(goja.Value)) {
	_ = object.DefineAccessorProperty(name, vm.ToValue(getter), vm.ToValue(setter), goja.FLAG_TRUE, goja.FLAG_TRUE)
}

func attribute(node *html.Node, name string) string {
	value, _ := find_attribute(node, name)
	return value
}

func has_attribute(node *html.Node, name string) bool {
	_, ok := find_attribute(node, name)
	return ok
}

func find_attribute(node *html.Node, name string) (string, bool) {
	if node == nil {
		return "", false
	}
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, name) {
			return attr.Val, true
		}
	}
	return "", false
}

func set_attribute(node *html.Node, name string, value string) {
	for index := range node.Attr {
		if strings.EqualFold(node.Attr[index].Key, name) {
			node.Attr[index].Val = value
			return
		}
	}
	node.Attr = append(node.Attr, html.Attribute{Key: strings.ToLower(name), Val: value})
}

func remove_attribute(node *html.Node, name string) {
	for index := range node.Attr {
		if strings.EqualFold(node.Attr[index].Key, name) {
			node.Attr = append(node.Attr[:index], node.Attr[index+1:]...)
			return
		}
	}
}

func text_content(node *html.Node) string {
	if node == nil {
		return ""
	}
	if node.Type == html.TextNode {
		return node.Data
	}
	var builder strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		builder.WriteString(text_content(child))
	}
	return builder.String()
}

func rendered_text_content(node *html.Node) string {
	if node == nil {
		return ""
	}
	if node.Type == html.TextNode {
		return node.Data
	}
	if node.Type == html.ElementNode {
		switch strings.ToLower(node.Data) {
		case "script", "style", "template":
			return ""
		}
	}
	// ponytail: full DOM scans keep waits simple; add mutation versions if large-page profiling requires it.
	var builder strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		builder.WriteString(rendered_text_content(child))
	}
	return builder.String()
}

func set_text_content(node *html.Node, value string) {
	remove_children(node)
	if value != "" {
		node.AppendChild(&html.Node{Type: html.TextNode, Data: value})
	}
}

func remove_children(node *html.Node) {
	for node.FirstChild != nil {
		node.RemoveChild(node.FirstChild)
	}
}

func set_inner_html(node *html.Node, markup string) {
	remove_children(node)
	append_html(node, markup)
}

func append_html(node *html.Node, markup string) []*html.Node {
	context_node := node
	if context_node.Type != html.ElementNode {
		context_node = &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: "div"}
	}
	fragments, err := html.ParseFragment(strings.NewReader(markup), context_node)
	if err != nil {
		return nil
	}
	for _, fragment := range fragments {
		node.AppendChild(fragment)
	}
	return fragments
}

func render_node(node *html.Node) string {
	if node == nil {
		return ""
	}
	var buffer bytes.Buffer
	_ = html.Render(&buffer, node)
	return buffer.String()
}

func render_children(node *html.Node) string {
	var buffer bytes.Buffer
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		_ = html.Render(&buffer, child)
	}
	return buffer.String()
}

func children(node *html.Node, elements_only bool) []*html.Node {
	result := make([]*html.Node, 0)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if !elements_only || child.Type == html.ElementNode {
			result = append(result, child)
		}
	}
	return result
}

func dom_node_type(node *html.Node) int {
	if node.Type == html.DocumentNode && node.Data == "#document-fragment" {
		return 11
	}
	switch node.Type {
	case html.ElementNode:
		return 1
	case html.TextNode:
		return 3
	case html.CommentNode:
		return 8
	case html.DocumentNode:
		return 9
	case html.DoctypeNode:
		return 10
	default:
		return 0
	}
}

func dom_node_name(node *html.Node) string {
	if node.Type == html.DocumentNode && node.Data == "#document-fragment" {
		return "#document-fragment"
	}
	switch node.Type {
	case html.DocumentNode:
		return "#document"
	case html.TextNode:
		return "#text"
	case html.CommentNode:
		return "#comment"
	default:
		return strings.ToUpper(node.Data)
	}
}

func find_element(node *html.Node, name string) *html.Node {
	if node == nil {
		return nil
	}
	if node.Type == html.ElementNode && strings.EqualFold(node.Data, name) {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := find_element(child, name); found != nil {
			return found
		}
	}
	return nil
}

func find_by_attribute(node *html.Node, name string, value string) *html.Node {
	if node == nil {
		return nil
	}
	if found, ok := find_attribute(node, name); ok && found == value {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := find_by_attribute(child, name, value); found != nil {
			return found
		}
	}
	return nil
}

func find_all_by_attribute(node *html.Node, name string, value string) []*html.Node {
	result := make([]*html.Node, 0)
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.ElementNode {
			if found, ok := find_attribute(current, name); ok && found == value {
				result = append(result, current)
			}
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return result
}

func find_by_tag(node *html.Node, name string) []*html.Node {
	result := make([]*html.Node, 0)
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.ElementNode && (name == "*" || strings.EqualFold(current.Data, name)) {
			result = append(result, current)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return result
}

func find_by_class(node *html.Node, name string) []*html.Node {
	result := make([]*html.Node, 0)
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.ElementNode && has_class(current, name) {
			result = append(result, current)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return result
}

func class_values(node *html.Node) []string {
	return strings.Fields(attribute(node, "class"))
}

func has_class(node *html.Node, name string) bool {
	return slice_contains(class_values(node), name)
}

func slice_contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func query_first(node *html.Node, selector string) *html.Node {
	matcher, err := cascadia.ParseGroup(selector)
	if err != nil {
		return nil
	}
	return cascadia.Query(node, matcher)
}

func query_all(node *html.Node, selector string) []*html.Node {
	matcher, err := cascadia.ParseGroup(selector)
	if err != nil {
		return nil
	}
	return cascadia.QueryAll(node, matcher)
}

func set_document_title(document *html.Node, title string) {
	title_node := find_element(document, "title")
	if title_node == nil {
		head := find_element(document, "head")
		if head == nil {
			return
		}
		title_node = new_element("title")
		head.AppendChild(title_node)
	}
	set_text_content(title_node, title)
}

func new_element(name string) *html.Node {
	name = strings.ToLower(name)
	return &html.Node{Type: html.ElementNode, DataAtom: atom.Lookup([]byte(name)), Data: name}
}

func (runtime *page_runtime) clone_node(node *html.Node, deep bool) *html.Node {
	clone := &html.Node{Type: node.Type, DataAtom: node.DataAtom, Data: node.Data, Namespace: node.Namespace, Attr: append([]html.Attribute(nil), node.Attr...)}
	if runtime.fragments[node] {
		runtime.fragments[clone] = true
	}
	if deep {
		if content := runtime.template_contents[node]; content != nil {
			runtime.template_contents[clone] = runtime.clone_node(content, true)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			clone.AppendChild(runtime.clone_node(child, true))
		}
	}
	return clone
}

func contains_node(parent *html.Node, candidate *html.Node) bool {
	for current := candidate; current != nil; current = current.Parent {
		if current == parent {
			return true
		}
	}
	return false
}
