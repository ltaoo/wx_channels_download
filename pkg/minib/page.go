package minib

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/cascadia"
	"github.com/dop251/goja"
	"github.com/evanw/esbuild/pkg/api"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"wx_channel/pkg/clawreq"
)

const (
	max_redirects        = 10
	max_script_size      = 16 << 20
	resource_concurrency = 8
	// ponytail: a finite callback budget models navigation idle; make it configurable only if a real page needs more.
	max_timer_callbacks   = 8
	max_host_callbacks    = 256
	max_dynamic_scripts   = 64
	max_dynamic_resources = 256
	max_event_loop_rounds = 2
	max_call_stack_depth  = 1024
)

// NavigateOptions controls one navigation without changing browser-wide state.
type NavigateOptions struct {
	// DisableCache bypasses cache reads and writes and sends no-cache headers.
	DisableCache bool
	// CaptureHAR records request and response bodies for later HAR export.
	CaptureHAR bool
}

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
	FromCache bool
	Err       error
}

// ScriptFailure records a script that could not be downloaded or executed.
type ScriptFailure struct {
	URL string
	Err error
}

// Page is the result of a browser-like navigation.
type Page struct {
	URL             string
	StatusCode      int
	ContentType     string
	HTML            string
	RenderedHTML    string
	Document        *html.Node
	Resources       []Resource
	ScriptFailures  []ScriptFailure
	ConsoleMessages []string
	XHRRequests     []string
	FetchRequests   []string
	ExecutedScripts int
	disable_cache   bool
	har_data        []byte
	navigation_url  string
}

type script_job struct {
	node           *html.Node
	resource_index int
	inline         string
	source_url     string
	defer_script   bool
	async_script   bool
}

type timer_job struct {
	id       int64
	callback goja.Callable
	args     []goja.Value
	canceled bool
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
}

type page_runtime struct {
	browser              *MiniBrowser
	ctx                  context.Context
	page                 *Page
	page_url             *url.URL
	vm                   *goja.Runtime
	nodes                map[*html.Node]*goja.Object
	node_ids             map[int64]*html.Node
	next_node_id         int64
	fragments            map[*html.Node]bool
	template_contents    map[*html.Node]*html.Node
	styles               map[*html.Node]*goja.Object
	listeners            map[*html.Node]map[string][]goja.Callable
	window_listeners     map[string][]goja.Callable
	dynamic_scripts      []*html.Node
	dynamic_styles       []*html.Node
	dynamic_resources    []*html.Node
	dynamic_seen         map[*html.Node]bool
	custom_elements      map[string]goja.Value
	custom_waiters       map[string][]func(interface{}) error
	custom_constructed   map[*html.Node]bool
	custom_connected     map[*html.Node]bool
	pending_custom_nodes []*html.Node
	timers               []*timer_job
	host_jobs            []func()
	next_timer_id        int64
	current_script       *html.Node
	ready_state          string
	user_agent           string
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
	var har_recorder *har_recorder
	if navigate_options.CaptureHAR {
		har_recorder = new_har_recorder(time.Now())
		ctx = with_har_recorder(ctx, har_recorder)
	}
	page, err := b.navigate(ctx, raw_url, headers, navigate_options, 0)
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
		URL:           final_url,
		StatusCode:    response.StatusCode,
		ContentType:   response.ContentType(),
		HTML:          html_text,
		Document:      document,
		disable_cache: navigate_options.DisableCache,
	}
	jobs := discover_page_resources(page, page_url)
	b.download_resources(ctx, page, page_url, navigate_options.DisableCache)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := b.execute_page(ctx, page, page_url, jobs, document_headers.Get("User-Agent")); err != nil {
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

func discover_page_resources(page *Page, page_url *url.URL) []script_job {
	jobs := make([]script_job, 0)
	resource_indexes := make(map[string]int)
	inline_index := 0
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			tag_name := strings.ToLower(node.Data)
			switch tag_name {
			case "script":
				if is_javascript_type(attribute(node, "type")) {
					if source := attribute(node, "src"); source != "" {
						if resource_url, ok := resolve_resource_url(page_url, source); ok {
							resource_index := add_resource(page, resource_indexes, resource_url, ScriptResource)
							jobs = append(jobs, script_job{
								node:           node,
								resource_index: resource_index,
								source_url:     resource_url,
								defer_script:   has_attribute(node, "defer") || strings.EqualFold(strings.TrimSpace(attribute(node, "type")), "module"),
								async_script:   has_attribute(node, "async"),
							})
						}
					} else if source := text_content(node); strings.TrimSpace(source) != "" {
						inline_index++
						jobs = append(jobs, script_job{node: node, resource_index: -1, inline: source, source_url: fmt.Sprintf("%s#inline-%d", page.URL, inline_index)})
					}
				}
			case "link":
				kind, load := link_resource_kind(node)
				if load {
					if resource_url, ok := resolve_resource_url(page_url, attribute(node, "href")); ok {
						add_resource(page, resource_indexes, resource_url, kind)
					}
				}
			case "img":
				if resource_url, ok := resolve_resource_url(page_url, attribute(node, "src")); ok {
					add_resource(page, resource_indexes, resource_url, ImageResource)
				}
			case "audio", "video", "source", "track", "embed", "iframe":
				if resource_url, ok := resolve_resource_url(page_url, attribute(node, "src")); ok {
					add_resource(page, resource_indexes, resource_url, MediaResource)
				}
				if tag_name == "video" {
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
	page.Resources = append(page.Resources, Resource{URL: resource_url, Kind: kind})
	return index
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

func (b *MiniBrowser) download_resources(ctx context.Context, page *Page, page_url *url.URL, disable_cache bool) {
	var wait_group sync.WaitGroup
	semaphore := make(chan struct{}, resource_concurrency)
	for index := range page.Resources {
		wait_group.Add(1)
		go func(resource_index int) {
			defer wait_group.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				page.Resources[resource_index].Err = ctx.Err()
				return
			}
			page.Resources[resource_index] = b.download_resource(ctx, page_url, page.Resources[resource_index], disable_cache)
		}(index)
	}
	wait_group.Wait()
}

func (b *MiniBrowser) download_resource(ctx context.Context, page_url *url.URL, resource Resource, disable_cache bool) Resource {
	ctx = with_har_resource_type(ctx, string(resource.Kind))
	headers := resource_headers(page_url, resource.URL, resource.Kind)
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

func disable_cache_headers(headers http.Header) {
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Pragma", "no-cache")
}

func resource_headers(page_url *url.URL, resource_url string, kind ResourceKind) http.Header {
	headers := clawreq.DefaultHeaders(clawreq.ProfileChrome)
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

func (b *MiniBrowser) execute_page(ctx context.Context, page *Page, page_url *url.URL, jobs []script_job, user_agent string) error {
	b.js_mutex.Lock()
	defer b.js_mutex.Unlock()
	b.js_runtime = goja.New()
	b.js_runtime.SetMaxCallStackSize(max_call_stack_depth)
	runtime := &page_runtime{
		browser:            b,
		ctx:                ctx,
		page:               page,
		page_url:           page_url,
		vm:                 b.js_runtime,
		nodes:              make(map[*html.Node]*goja.Object),
		node_ids:           make(map[int64]*html.Node),
		fragments:          make(map[*html.Node]bool),
		template_contents:  make(map[*html.Node]*html.Node),
		styles:             make(map[*html.Node]*goja.Object),
		listeners:          make(map[*html.Node]map[string][]goja.Callable),
		window_listeners:   make(map[string][]goja.Callable),
		dynamic_seen:       make(map[*html.Node]bool),
		custom_elements:    make(map[string]goja.Value),
		custom_waiters:     make(map[string][]func(interface{}) error),
		custom_constructed: make(map[*html.Node]bool),
		custom_connected:   make(map[*html.Node]bool),
		ready_state:        "loading",
		user_agent:         user_agent,
	}
	b.js_runtime.SetPromiseRejectionTracker(func(promise *goja.Promise, operation goja.PromiseRejectionOperation) {
		if operation == goja.PromiseRejectionReject {
			page.ConsoleMessages = append(page.ConsoleMessages, "unhandled rejection: "+promise.Result().String())
		}
	})
	if err := runtime.install(); err != nil {
		return fmt.Errorf("minib: initialize page runtime: %w", err)
	}
	ordered_jobs := make([]script_job, 0, len(jobs))
	for stage := 0; stage < 3; stage++ {
		for _, job := range jobs {
			job_stage := 0
			if job.defer_script {
				job_stage = 1
			}
			if job.async_script {
				job_stage = 2
			}
			if job_stage == stage {
				ordered_jobs = append(ordered_jobs, job)
			}
		}
	}
	for _, job := range ordered_jobs {
		if err := ctx.Err(); err != nil {
			page.ScriptFailures = append(page.ScriptFailures, ScriptFailure{URL: job.source_url, Err: err})
			break
		}
		failure_count := len(page.ScriptFailures)
		runtime.execute_job(ctx, job)
		if job.resource_index >= 0 {
			if len(page.ScriptFailures) == failure_count {
				runtime.fire_node_event(job.node, "load")
			} else {
				runtime.fire_node_event(job.node, "error")
			}
		}
		runtime.run_host_jobs(ctx)
		runtime.drain_dynamic_styles(ctx)
		runtime.drain_dynamic_scripts(ctx)
		runtime.drain_dynamic_resources(ctx)
		runtime.run_host_jobs(ctx)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	runtime.ready_state = "interactive"
	runtime.fire_document_event("DOMContentLoaded")
	har_recorder_from_context(ctx).mark_content_loaded()
	runtime.pump_event_loop(ctx)
	runtime.ready_state = "complete"
	runtime.fire_document_event("readystatechange")
	runtime.fire_window_event("load")
	har_recorder_from_context(ctx).mark_loaded()
	runtime.pump_event_loop(ctx)
	return nil
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
	_, err := run_javascript(ctx, runtime.vm, job.source_url, source)
	runtime.current_script = nil
	if err != nil {
		runtime.fail_script(job.source_url, err)
		return
	}
	runtime.page.ExecutedScripts++
	runtime.sync_named_elements()
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
	runtime.page.ScriptFailures = append(runtime.page.ScriptFailures, ScriptFailure{URL: source_url, Err: err})
}

func run_javascript(ctx context.Context, vm *goja.Runtime, source_url string, source string) (goja.Value, error) {
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
	value, err := vm.RunProgram(program)
	close(finished)
	<-interrupt_done
	vm.ClearInterrupt()
	return value, err
}

func call_javascript(ctx context.Context, vm *goja.Runtime, callback goja.Callable, this goja.Value, args ...goja.Value) (goja.Value, error) {
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
	value, err := callback(this, args...)
	close(finished)
	<-interrupt_done
	if ctx.Err() == nil {
		vm.ClearInterrupt()
	}
	return value, err
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
function EventTarget() {}
EventTarget.prototype.addEventListener = function() { if (this.__minib_addEventListener) return this.__minib_addEventListener.apply(this, arguments); };
EventTarget.prototype.removeEventListener = function() { if (this.__minib_removeEventListener) return this.__minib_removeEventListener.apply(this, arguments); };
EventTarget.prototype.dispatchEvent = function() { if (this.__minib_dispatchEvent) return this.__minib_dispatchEvent.apply(this, arguments); return true; };
function Window() {}
Window.prototype = Object.create(EventTarget.prototype);
function Node() {}
Node.prototype = Object.create(EventTarget.prototype);
Node.ELEMENT_NODE = 1; Node.TEXT_NODE = 3; Node.COMMENT_NODE = 8; Node.DOCUMENT_NODE = 9; Node.DOCUMENT_FRAGMENT_NODE = 11;
function __minibOwnAccessor(name, writable) {
  var descriptor = { configurable: true, enumerable: true, get: function() { var own = Object.getOwnPropertyDescriptor(this, name); return own ? (own.get ? own.get.call(this) : own.value) : undefined; } };
  if (writable) descriptor.set = function(value) { var own = Object.getOwnPropertyDescriptor(this, name); if (own) { if (own.set) own.set.call(this, value); else if (own.writable) { own.value = value; Object.defineProperty(this, name, own); } } else Object.defineProperty(this, name, { configurable: true, enumerable: true, writable: true, value: value }); };
  return descriptor;
}
['nodeType', 'nodeName', 'parentNode', 'parentElement', 'firstChild', 'lastChild', 'nextSibling', 'previousSibling', 'childNodes', 'ownerDocument', 'isConnected'].forEach(function(name) { Object.defineProperty(Node.prototype, name, __minibOwnAccessor(name, false)); });
['textContent', 'nodeValue'].forEach(function(name) { Object.defineProperty(Node.prototype, name, __minibOwnAccessor(name, true)); });
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
['children', 'attributes'].forEach(function(name) { Object.defineProperty(Element.prototype, name, __minibOwnAccessor(name, false)); });
function __minibMethod(name) { return function() { return this['__minib_' + name].apply(this, arguments); }; }
['appendChild', 'removeChild', 'replaceChild', 'insertBefore', 'cloneNode', 'contains'].forEach(function(name) { Node.prototype[name] = __minibMethod(name); });
['insertAdjacentElement', 'getAttribute', 'setAttribute', 'getAttributeNS', 'setAttributeNS', 'removeAttribute', 'removeAttributeNS', 'hasAttribute', 'hasAttributeNS', 'hasAttributes', 'querySelector', 'querySelectorAll', 'getElementsByTagName', 'getElementsByClassName', 'matches', 'closest', 'getBoundingClientRect', 'getClientRects'].forEach(function(name) { Element.prototype[name] = __minibMethod(name); });
function HTMLElement() { if (typeof __minib_construct_html_element === 'function') __minib_construct_html_element(this); }
HTMLElement.prototype = Object.create(Element.prototype);
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
function MessagePort() { this.onmessage = null; this._target = null; }
MessagePort.prototype.postMessage = function(data) { var target = this._target; setTimeout(function() { if (target && typeof target.onmessage === 'function') target.onmessage({ data: data, type: 'message', target: target }); }, 0); };
MessagePort.prototype.start = function() {};
MessagePort.prototype.close = function() { this._target = null; };
function MessageChannel() { this.port1 = new MessagePort(); this.port2 = new MessagePort(); this.port1._target = this.port2; this.port2._target = this.port1; }
function queueMicrotask(callback) { Promise.resolve().then(callback); }
function SVGElement() {}
SVGElement.prototype = Object.create(Element.prototype);
function Document() {}
Document.prototype = Object.create(Node.prototype);
['createElement', 'createElementNS', 'createTextNode', 'createDocumentFragment', 'createComment', 'createEvent', 'createRange', 'importNode', 'getElementById', 'getElementsByTagName', 'getElementsByClassName', 'querySelector', 'querySelectorAll'].forEach(function(name) { Document.prototype[name] = __minibMethod(name); });
Document.prototype.createTreeWalker = function(root, whatToShow, filter) { return new TreeWalker(root, whatToShow, filter); };
function HTMLDocument() {}
HTMLDocument.prototype = Object.create(Document.prototype);
Object.defineProperty(HTMLDocument.prototype, Symbol.toStringTag, { value: 'HTMLDocument' });
function DocumentFragment() {}
DocumentFragment.prototype = Object.create(Node.prototype);
Object.defineProperty(DocumentFragment.prototype, 'children', __minibOwnAccessor('children', false));
['querySelector', 'querySelectorAll'].forEach(function(name) { DocumentFragment.prototype[name] = __minibMethod(name); });
function Range() {}
function NodeList() {}
NodeList.prototype = Array.prototype;
function HTMLCollection() {}
HTMLCollection.prototype = Array.prototype;
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
function Event(type, init) { this.type = String(type || ''); this.bubbles = !!(init && init.bubbles); this.cancelable = !!(init && init.cancelable); this.defaultPrevented = false; }
var __minib_event_constructor = Event;
Event.prototype.preventDefault = function() { if (this.cancelable) this.defaultPrevented = true; };
Event.prototype.stopPropagation = function() {};
Event.prototype.initEvent = function(type, bubbles, cancelable) { this.type = String(type || ''); this.bubbles = !!bubbles; this.cancelable = !!cancelable; };
['type', 'bubbles', 'cancelable', 'defaultPrevented', 'target', 'currentTarget', 'composed'].forEach(function(name) { Object.defineProperty(Event.prototype, name, __minibOwnAccessor(name, true)); });
function UIEvent(type, init) { __minib_event_constructor.call(this, type, init); }
UIEvent.prototype = Object.create(Event.prototype);
function MouseEvent(type, init) { __minib_event_constructor.call(this, type, init); }
MouseEvent.prototype = Object.create(UIEvent.prototype);
function KeyboardEvent(type, init) { __minib_event_constructor.call(this, type, init); this.key = init && init.key || ''; this.code = init && init.code || ''; }
KeyboardEvent.prototype = Object.create(UIEvent.prototype);
function FocusEvent(type, init) { __minib_event_constructor.call(this, type, init); this.relatedTarget = init && init.relatedTarget || null; }
FocusEvent.prototype = Object.create(UIEvent.prototype);
function InputEvent(type, init) { __minib_event_constructor.call(this, type, init); this.data = init && init.data || null; }
InputEvent.prototype = Object.create(UIEvent.prototype);
function WheelEvent(type, init) { __minib_event_constructor.call(this, type, init); }
WheelEvent.prototype = Object.create(MouseEvent.prototype);
function PointerEvent(type, init) { __minib_event_constructor.call(this, type, init); }
PointerEvent.prototype = Object.create(MouseEvent.prototype);
function CustomEvent(type, init) { __minib_event_constructor.call(this, type, init); this.detail = init && init.detail; }
CustomEvent.prototype = Object.create(Event.prototype);
CustomEvent.prototype.initCustomEvent = function(type, bubbles, cancelable, detail) { this.initEvent(type, bubbles, cancelable); this.detail = detail; };
function MutationObserver(callback) { this.callback = callback; }
MutationObserver.prototype.observe = function(target) { target.__minib_mutation_callback = this.callback; target.__minib_mutation_observer = this; this.target = target; };
MutationObserver.prototype.disconnect = function() { if (this.target) { delete this.target.__minib_mutation_callback; delete this.target.__minib_mutation_observer; } this.target = null; };
MutationObserver.prototype.takeRecords = function() { return []; };
function IntersectionObserver(callback) { this.callback = callback; this.targets = []; }
IntersectionObserver.prototype.observe = function(target) { var self = this; this.targets.push(target); setTimeout(function() { var rect = target.getBoundingClientRect(); self.callback([{ target: target, isIntersecting: true, intersectionRatio: 1, boundingClientRect: rect, intersectionRect: rect, rootBounds: rect, time: performance.now() }], self); }, 0); };
IntersectionObserver.prototype.unobserve = function(target) { this.targets = this.targets.filter(function(item) { return item !== target; }); };
IntersectionObserver.prototype.disconnect = function() { this.targets = []; };
IntersectionObserver.prototype.takeRecords = function() { return []; };
function Blob(parts, options) { this.size = (parts || []).reduce(function(size, part) { return size + String(part).length; }, 0); this.type = options && options.type || ''; }
function File(parts, name, options) { Blob.call(this, parts, options); this.name = String(name || ''); this.lastModified = options && options.lastModified || Date.now(); }
File.prototype = Object.create(Blob.prototype);
[Window, Node, CharacterData, Text, Comment, CDATASection, ProcessingInstruction, DocumentType, Element, HTMLElement, HTMLBodyElement, HTMLHtmlElement, HTMLImageElement, HTMLIFrameElement, HTMLTemplateElement, HTMLMediaElement, HTMLAudioElement, HTMLVideoElement, SVGElement, Document, HTMLDocument, DocumentFragment, UIEvent, MouseEvent, KeyboardEvent, FocusEvent, InputEvent, WheelEvent, PointerEvent, CustomEvent, File].forEach(function(constructor) {
  Object.defineProperty(constructor.prototype, 'constructor', { configurable: true, writable: true, value: constructor });
});
function Headers(init) {
  this._values = {};
  var self = this;
  if (typeof init === 'string') init.trim().split(/[\r\n]+/).forEach(function(line) { var index = line.indexOf(':'); if (index > 0) self.append(line.slice(0, index), line.slice(index + 1)); });
  else if (init && typeof init.forEach === 'function') init.forEach(function(value, name) { self.append(name, value); });
  else Object.keys(init || {}).forEach(function(name) { self.append(name, init[name]); });
}
Headers.prototype.append = function(name, value) { name = String(name).toLowerCase(); this._values[name] = this._values[name] ? this._values[name] + ', ' + value : String(value); };
Headers.prototype.set = function(name, value) { this._values[String(name).toLowerCase()] = String(value); };
Headers.prototype.get = function(name) { name = String(name).toLowerCase(); return Object.prototype.hasOwnProperty.call(this._values, name) ? this._values[name] : null; };
Headers.prototype.has = function(name) { return this.get(name) !== null; };
Headers.prototype.forEach = function(callback, this_arg) { var self = this; Object.keys(this._values).forEach(function(name) { callback.call(this_arg, self._values[name], name, self); }); };
function Request(input, init) { init = init || {}; this.url = String(input && input.url || input); this.method = String(init.method || input && input.method || 'GET').toUpperCase(); this.headers = new Headers(init.headers || input && input.headers); this.body = init.body == null ? null : init.body; }
function Response(body, init) { init = init || {}; this._body = body == null ? '' : String(body); this.status = init.status || 200; this.statusText = init.statusText || ''; this.headers = init.headers instanceof Headers ? init.headers : new Headers(init.headers); this.url = init.url || ''; this.ok = this.status >= 200 && this.status < 300; this.redirected = false; this.type = 'basic'; this.bodyUsed = false; }
Response.prototype.text = function() { this.bodyUsed = true; return Promise.resolve(this._body); };
Response.prototype.json = function() { this.bodyUsed = true; return Promise.resolve(JSON.parse(this._body)); };
Response.prototype.clone = function() { return new Response(this._body, { status: this.status, statusText: this.statusText, headers: this.headers, url: this.url }); };
function fetch(input, init) {
  var request = input instanceof Request ? input : new Request(input, init);
  return new Promise(function(resolve, reject) {
    var xhr = new XMLHttpRequest();
    xhr.__minib_resource_type = 'fetch';
    xhr.open(request.method, request.url, true);
    request.headers.forEach(function(value, name) { xhr.setRequestHeader(name, value); });
    xhr.onload = function() { resolve(new Response(xhr.responseText, { status: xhr.status, statusText: xhr.statusText, headers: new Headers(xhr.getAllResponseHeaders()), url: xhr.responseURL })); };
    xhr.onerror = function() { reject(new TypeError('Failed to fetch')); };
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
var __minib_native_apply = Function.prototype.apply;
Function.prototype.apply = function(this_arg, args) { return __minib_native_apply.call(this, this_arg, args == null ? [] : args); };
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
	_ = window.Set("__minib_construct_html_element", func(call goja.FunctionCall) goja.Value {
		if len(runtime.pending_custom_nodes) == 0 {
			return goja.Undefined()
		}
		node := runtime.pending_custom_nodes[len(runtime.pending_custom_nodes)-1]
		runtime.bind_node_object(node, call.Argument(0).ToObject(runtime.vm), false)
		return goja.Undefined()
	})
	_ = window.Set("__minib_import", func(string) *goja.Promise {
		// ponytail: lazy modules stay inert; fetch them when a target page actually invokes one.
		promise, resolve, _ := runtime.vm.NewPromise()
		_ = resolve(runtime.vm.NewObject())
		return promise
	})
	// ponytail: Weibo's current bundle references this undeclared optional component.
	_ = window.Set("toolbar", runtime.vm.NewObject())
	_ = window.Set("document", runtime.node_object(runtime.page.Document))
	runtime.install_custom_elements(window)
	_ = window.Set("location", runtime.location_object(runtime.page_url))
	_ = window.Set("navigator", map[string]any{
		"userAgent": runtime.user_agent, "platform": "MacIntel", "language": "zh-CN",
		"languages": []string{"zh-CN", "zh", "en"}, "cookieEnabled": true, "webdriver": false, "vendor": "Google Inc.",
		"plugins": []any{}, "mimeTypes": []any{}, "hardwareConcurrency": 8, "maxTouchPoints": 0,
		"javaEnabled": func() bool { return false },
		"sendBeacon":  func(string, ...any) bool { return true },
	})
	_ = window.Set("screen", map[string]int{"width": 1440, "height": 900, "availWidth": 1440, "availHeight": 875, "colorDepth": 24, "pixelDepth": 24})
	_ = window.Set("innerWidth", 1440)
	_ = window.Set("innerHeight", 900)
	_ = window.Set("devicePixelRatio", 2)
	_ = window.Set("pageXOffset", 0)
	_ = window.Set("pageYOffset", 0)
	_ = window.Set("scrollTo", func(...any) {})
	_ = window.Set("scrollBy", func(...any) {})
	_ = window.Set("console", runtime.console_object())
	performance_time := time.Now().UnixMilli()
	_ = window.Set("performance", map[string]any{
		"now":        func() float64 { return float64(time.Now().UnixNano()) / float64(time.Millisecond) },
		"timeOrigin": performance_time,
		"timing":     map[string]int64{"navigationStart": performance_time, "responseStart": performance_time},
		"navigation": map[string]int{"type": 0},
	})
	_ = window.Set("history", map[string]any{"length": 1, "pushState": func(...any) {}, "replaceState": func(...any) {}, "back": func() {}, "forward": func() {}, "go": func(...any) {}})
	_ = window.Set("localStorage", runtime.storage_object())
	_ = window.Set("sessionStorage", runtime.storage_object())
	_ = window.Set("getComputedStyle", func(call goja.FunctionCall) goja.Value {
		return runtime.computed_style_object(runtime.object_node(call.Argument(0)))
	})
	_ = window.Set("atob", func(value string) string {
		decoded, _ := base64.StdEncoding.DecodeString(value)
		return string(decoded)
	})
	_ = window.Set("btoa", func(value string) string { return base64.StdEncoding.EncodeToString([]byte(value)) })
	_ = window.Set("TextDecoder", runtime.text_decoder_constructor)
	_ = window.Set("Image", func(call goja.ConstructorCall) *goja.Object { return runtime.node_object(new_element("img")) })
	_ = window.Set("Audio", func(call goja.ConstructorCall) *goja.Object { return runtime.node_object(new_element("audio")) })
	_ = window.Set("URL", runtime.url_constructor)
	url_constructor := window.Get("URL").ToObject(runtime.vm)
	_ = url_constructor.Set("createObjectURL", func(any) string { return "blob:" + runtime.page_url.Scheme + "://" + runtime.page_url.Host + "/minib" })
	_ = url_constructor.Set("revokeObjectURL", func(string) {})
	_ = window.Set("XMLHttpRequest", runtime.xml_http_request_constructor)
	runtime.install_xml_http_request_prototype(window.Get("XMLHttpRequest").ToObject(runtime.vm))
	_ = window.Set("matchMedia", func(query string) map[string]any {
		return map[string]any{"matches": strings.Contains(query, "hover") || strings.Contains(query, "pointer"), "media": query, "addListener": func(...any) {}, "removeListener": func(...any) {}, "addEventListener": func(...any) {}, "removeEventListener": func(...any) {}}
	})
	runtime.install_window_events(window)
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
		request.method = strings.ToUpper(open_call.Argument(0).String())
		request.raw_url = open_call.Argument(1).String()
		request.ready_state = 1
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
		request.ready_state = 0
		request.status = 0
		request.fire("abort")
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
		var body io.Reader
		if value := send_call.Argument(0); !goja.IsNull(value) && !goja.IsUndefined(value) {
			body = strings.NewReader(value.String())
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

func (request *xml_http_request) send(body io.Reader) {
	if request.method == "" {
		request.method = http.MethodGet
	}
	request_url, err := request.runtime.page_url.Parse(request.raw_url)
	if err != nil {
		request.fail(err)
		return
	}
	headers := clawreq.DefaultHeaders(clawreq.ProfileChrome)
	headers.Set("Accept", "application/json, text/plain, */*")
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
	request_ctx := with_har_resource_type(request.runtime.ctx, resource_type)
	response, err := request.runtime.browser.Request(request_ctx, request.method, request_url.String(), body, headers)
	if err != nil {
		request.fail(err)
		return
	}
	response_text, err := response.Text()
	if err != nil {
		request.fail(err)
		return
	}
	request.runtime.queue_host_job(func() {
		request.status = response.StatusCode
		request.status_text = response.Status
		request.response_headers = response.Header
		request.response_text = response_text
		request.response_url = response.FinalURL
		request.ready_state = 4
		request.fire("readystatechange")
		request.fire("load")
		request.fire("loadend")
	})
}

func (request *xml_http_request) fail(err error) {
	request.runtime.queue_host_job(func() {
		request.status = 0
		request.status_text = err.Error()
		request.ready_state = 4
		request.fire("readystatechange")
		request.fire("error")
		request.fire("loadend")
	})
}

func (request *xml_http_request) response_value() any {
	if strings.EqualFold(request.object.Get("responseType").String(), "json") {
		var value any
		if json.Unmarshal([]byte(request.response_text), &value) == nil {
			return value
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
		if _, err := call_javascript(request.runtime.ctx, request.runtime.vm, callback, request.object, event); err != nil {
			request.runtime.fail_script(request.raw_url+"#"+event_name, err)
		}
	}
	if has_handler {
		if _, err := call_javascript(request.runtime.ctx, request.runtime.vm, handler, request.object, event); err != nil {
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
					if stack := argument_object.Get("stack"); !goja.IsUndefined(stack) && !goja.IsNull(stack) && !strings.Contains(part, stack.String()) {
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
	base_url := runtime.page_url
	if !goja.IsUndefined(call.Argument(1)) {
		if parsed_base, err := url.Parse(call.Argument(1).String()); err == nil {
			base_url = parsed_base
		}
	}
	parsed_url, err := base_url.Parse(raw_url)
	if err != nil {
		panic(runtime.vm.NewTypeError("invalid URL"))
	}
	object := runtime.location_object(parsed_url)
	_ = object.Set("toJSON", func() string { return parsed_url.String() })
	return object
}

func (runtime *page_runtime) install_window_events(window *goja.Object) {
	add_event_listener := func(call goja.FunctionCall) goja.Value {
		if callback, ok := goja.AssertFunction(call.Argument(1)); ok {
			event_name := call.Argument(0).String()
			runtime.window_listeners[event_name] = append(runtime.window_listeners[event_name], callback)
		}
		return goja.Undefined()
	}
	remove_event_listener := func() {}
	dispatch_event := func(call goja.FunctionCall) goja.Value {
		event_name, ok := event_type(runtime.vm, call.Argument(0))
		if !ok {
			panic(runtime.vm.NewTypeError("dispatchEvent requires an Event: %v", call.Argument(0).ToObject(runtime.vm).GetOwnPropertyNames()))
		}
		return runtime.vm.ToValue(runtime.dispatch_window_event(call.Argument(0).ToObject(runtime.vm), event_name))
	}
	_ = window.Set("__minib_addEventListener", add_event_listener)
	_ = window.Set("__minib_removeEventListener", remove_event_listener)
	_ = window.Set("__minib_dispatchEvent", dispatch_event)
	_ = window.Set("addEventListener", add_event_listener)
	_ = window.Set("removeEventListener", remove_event_listener)
	_ = window.Set("dispatchEvent", dispatch_event)
}

func (runtime *page_runtime) install_timers(window *goja.Object) {
	add_timer := func(call goja.FunctionCall) goja.Value {
		callback, ok := goja.AssertFunction(call.Argument(0))
		if !ok {
			return runtime.vm.ToValue(0)
		}
		runtime.next_timer_id++
		args := make([]goja.Value, 0)
		if len(call.Arguments) > 2 {
			args = append(args, call.Arguments[2:]...)
		}
		runtime.timers = append(runtime.timers, &timer_job{id: runtime.next_timer_id, callback: callback, args: args})
		return runtime.vm.ToValue(runtime.next_timer_id)
	}
	clear_timer := func(id int64) {
		for _, timer := range runtime.timers {
			if timer.id == id {
				timer.canceled = true
			}
		}
	}
	_ = window.Set("setTimeout", add_timer)
	_ = window.Set("setInterval", add_timer)
	_ = window.Set("clearTimeout", clear_timer)
	_ = window.Set("clearInterval", clear_timer)
	_ = window.Set("requestAnimationFrame", func(call goja.FunctionCall) goja.Value {
		return add_timer(goja.FunctionCall{Arguments: []goja.Value{call.Argument(0), runtime.vm.ToValue(0)}})
	})
	_ = window.Set("cancelAnimationFrame", clear_timer)
}

func (runtime *page_runtime) run_timers(ctx context.Context) {
	timer_count := len(runtime.timers)
	if timer_count > max_timer_callbacks {
		timer_count = max_timer_callbacks
	}
	for callback_count := 0; callback_count < timer_count && len(runtime.timers) > 0 && ctx.Err() == nil; callback_count++ {
		timer := runtime.timers[0]
		runtime.timers = runtime.timers[1:]
		if timer.canceled {
			continue
		}
		if _, err := call_javascript(ctx, runtime.vm, timer.callback, runtime.vm.GlobalObject(), timer.args...); err != nil {
			runtime.fail_script(runtime.page.URL+"#timer", err)
		}
		runtime.run_host_jobs(ctx)
		runtime.drain_dynamic_styles(ctx)
		runtime.drain_dynamic_scripts(ctx)
		runtime.drain_dynamic_resources(ctx)
	}
}

func (runtime *page_runtime) queue_host_job(job func()) {
	if job != nil {
		runtime.host_jobs = append(runtime.host_jobs, job)
	}
}

func (runtime *page_runtime) run_host_jobs(ctx context.Context) {
	for callback_count := 0; len(runtime.host_jobs) > 0 && callback_count < max_host_callbacks && ctx.Err() == nil; callback_count++ {
		job := runtime.host_jobs[0]
		runtime.host_jobs = runtime.host_jobs[1:]
		job()
	}
}

func (runtime *page_runtime) pump_event_loop(ctx context.Context) {
	for round := 0; round < max_event_loop_rounds && ctx.Err() == nil; round++ {
		if len(runtime.host_jobs) == 0 && len(runtime.timers) == 0 && len(runtime.dynamic_styles) == 0 && len(runtime.dynamic_scripts) == 0 && len(runtime.dynamic_resources) == 0 {
			return
		}
		runtime.run_host_jobs(ctx)
		runtime.run_timers(ctx)
		runtime.run_host_jobs(ctx)
		runtime.drain_dynamic_styles(ctx)
		runtime.drain_dynamic_scripts(ctx)
		runtime.drain_dynamic_resources(ctx)
	}
}

func (runtime *page_runtime) fire_document_event(event_name string) {
	runtime.fire_node_event(runtime.page.Document, event_name)
}

func (runtime *page_runtime) fire_window_event(event_name string) {
	runtime.dispatch_window_event(runtime.event_object(event_name), event_name)
}

func (runtime *page_runtime) dispatch_window_event(event *goja.Object, event_name string) bool {
	window := runtime.vm.GlobalObject()
	_ = event.Set("target", window)
	_ = event.Set("currentTarget", window)
	for _, callback := range runtime.window_listeners[event_name] {
		if _, err := call_javascript(runtime.ctx, runtime.vm, callback, window, event); err != nil {
			runtime.fail_script(runtime.page.URL+"#"+event_name, err)
		}
	}
	if callback, ok := goja.AssertFunction(window.Get("on" + event_name)); ok {
		if _, err := call_javascript(runtime.ctx, runtime.vm, callback, window, event); err != nil {
			runtime.fail_script(runtime.page.URL+"#"+event_name, err)
		}
	}
	default_prevented := event.Get("defaultPrevented")
	return default_prevented == nil || !default_prevented.ToBoolean()
}

func (runtime *page_runtime) fire_node_event(node *html.Node, event_name string) {
	runtime.dispatch_node_event(node, runtime.event_object(event_name), event_name)
}

func (runtime *page_runtime) dispatch_node_event(node *html.Node, event *goja.Object, event_name string) bool {
	stopped := false
	immediate_stopped := false
	_ = event.Set("target", runtime.node_object(node))
	_ = event.Set("stopPropagation", func() { stopped = true })
	_ = event.Set("stopImmediatePropagation", func() { stopped, immediate_stopped = true, true })
	for current := node; current != nil; current = current.Parent {
		current_object := runtime.node_object(current)
		_ = event.Set("currentTarget", current_object)
		for _, callback := range runtime.listeners[current][event_name] {
			if _, err := call_javascript(runtime.ctx, runtime.vm, callback, current_object, event); err != nil {
				runtime.fail_script(runtime.page.URL+"#"+event_name, err)
			}
			if immediate_stopped {
				break
			}
		}
		if !immediate_stopped {
			if callback, ok := goja.AssertFunction(current_object.Get("on" + event_name)); ok {
				if _, err := call_javascript(runtime.ctx, runtime.vm, callback, current_object, event); err != nil {
					runtime.fail_script(runtime.page.URL+"#"+event_name, err)
				}
			}
		}
		bubbles := event.Get("bubbles")
		if stopped || bubbles == nil || !bubbles.ToBoolean() {
			break
		}
	}
	default_prevented := event.Get("defaultPrevented")
	return default_prevented == nil || !default_prevented.ToBoolean()
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
	// ponytail: this covers renderer fragment creation; add offset-accurate selection only when an editor needs it.
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

func (runtime *page_runtime) bind_node_object(node *html.Node, object *goja.Object, set_prototype bool) {
	if node == nil || object == nil || runtime.nodes[node] == object {
		return
	}
	runtime.nodes[node] = object
	runtime.next_node_id++
	runtime.node_ids[runtime.next_node_id] = node
	_ = object.DefineDataProperty("__minib_node_id", runtime.vm.ToValue(runtime.next_node_id), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_FALSE)
	if set_prototype {
		runtime.set_node_prototype(object, node)
	}
	runtime.install_node_properties(object, node)
	runtime.install_node_methods(object, node)
	if runtime.fragments[node] {
		_ = object.Set("querySelector", func(selector string) any { return runtime.node_object(query_first(node, selector)) })
		_ = object.Set("querySelectorAll", func(selector string) any { return runtime.node_array(query_all(node, selector)) })
		_ = object.Set("__minib_querySelector", object.Get("querySelector"))
		_ = object.Set("__minib_querySelectorAll", object.Get("querySelectorAll"))
	}
	if node.Type == html.DocumentNode && !runtime.fragments[node] {
		runtime.install_document(object, node)
	}
	if node.Type == html.ElementNode {
		runtime.install_element(object, node)
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
		runtime.custom_elements[name] = constructor
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
		return goja.Undefined()
	})
	_ = registry.Set("get", func(name string) goja.Value {
		if constructor := runtime.custom_elements[strings.ToLower(name)]; constructor != nil {
			return constructor
		}
		return goja.Undefined()
	})
	_ = registry.Set("whenDefined", func(name string) goja.Value {
		name = strings.ToLower(name)
		promise, resolve, _ := runtime.vm.NewPromise()
		if constructor := runtime.custom_elements[name]; constructor != nil {
			_ = resolve(constructor)
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
	previous_object := runtime.nodes[node]
	runtime.pending_custom_nodes = append(runtime.pending_custom_nodes, node)
	object, err := runtime.vm.New(constructor)
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
			if name != "__minib_node_id" && !previous_names[name] {
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
	if root == nil {
		return
	}
	if root.Type == html.ElementNode && runtime.custom_elements[strings.ToLower(root.Data)] != nil {
		object, err := runtime.construct_custom_element(root)
		if err != nil {
			runtime.fail_script(runtime.page.URL+"#custom-element", err)
			return
		}
		if !runtime.custom_connected[root] && contains_node(runtime.page.Document, root) {
			runtime.custom_connected[root] = true
			if callback, ok := goja.AssertFunction(object.Get("connectedCallback")); ok {
				if _, err := call_javascript(runtime.ctx, runtime.vm, callback, object); err != nil {
					runtime.fail_script(runtime.page.URL+"#connected-callback", err)
				}
			}
		}
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		runtime.connect_custom_elements(child)
	}
}

func (runtime *page_runtime) custom_observes_attribute(node *html.Node, name string) bool {
	constructor := runtime.custom_elements[strings.ToLower(node.Data)]
	if constructor == nil {
		return false
	}
	observed := constructor.ToObject(runtime.vm).Get("observedAttributes")
	if observed == nil || goja.IsUndefined(observed) || goja.IsNull(observed) {
		return false
	}
	array := observed.ToObject(runtime.vm)
	for index, length := int64(0), array.Get("length").ToInteger(); index < length; index++ {
		if strings.EqualFold(array.Get(fmt.Sprintf("%d", index)).String(), name) {
			return true
		}
	}
	return false
}

func (runtime *page_runtime) attribute_changed(node *html.Node, name string, old_value any, new_value any) {
	if !runtime.custom_constructed[node] || !runtime.custom_observes_attribute(node, name) {
		return
	}
	object := runtime.node_object(node)
	callback, ok := goja.AssertFunction(object.Get("attributeChangedCallback"))
	if !ok {
		return
	}
	if _, err := call_javascript(runtime.ctx, runtime.vm, callback, object, runtime.vm.ToValue(name), runtime.vm.ToValue(old_value), runtime.vm.ToValue(new_value)); err != nil {
		runtime.fail_script(runtime.page.URL+"#attribute-changed-callback", err)
	}
}

func (runtime *page_runtime) set_element_attribute(node *html.Node, name string, value string) {
	old_value, exists := find_attribute(node, name)
	set_attribute(node, name, value)
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
		set_text_content(node, value.String())
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
		set_inner_html(node, value.String())
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			runtime.queue_dynamic_resource(child)
		}
	})
	define_getter(runtime.vm, object, "outerHTML", func() any { return render_node(node) })
}

func (runtime *page_runtime) notify_mutation(node *html.Node) {
	object := runtime.node_object(node)
	callback, ok := goja.AssertFunction(object.Get("__minib_mutation_callback"))
	if !ok {
		return
	}
	observer := object.Get("__minib_mutation_observer")
	runtime.queue_host_job(func() {
		if _, err := call_javascript(runtime.ctx, runtime.vm, callback, observer, runtime.vm.NewArray(), observer); err != nil {
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
		if child.Parent != nil {
			child.Parent.RemoveChild(child)
		}
		node.AppendChild(child)
		runtime.queue_dynamic_resource(child)
		return runtime.node_object(child)
	})
	_ = object.Set("removeChild", func(call goja.FunctionCall) goja.Value {
		child := runtime.object_node(call.Argument(0))
		if child == nil || child.Parent != node {
			return goja.Null()
		}
		node.RemoveChild(child)
		return runtime.node_object(child)
	})
	_ = object.Set("replaceChild", func(call goja.FunctionCall) goja.Value {
		child := runtime.object_node(call.Argument(0))
		old_child := runtime.object_node(call.Argument(1))
		if child == nil || old_child == nil || old_child.Parent != node {
			return goja.Null()
		}
		if runtime.fragments[child] {
			for child.FirstChild != nil {
				fragment_child := child.FirstChild
				child.RemoveChild(fragment_child)
				node.InsertBefore(fragment_child, old_child)
				runtime.queue_dynamic_resource(fragment_child)
			}
			node.RemoveChild(old_child)
			return runtime.node_object(old_child)
		}
		if child.Parent != nil {
			child.Parent.RemoveChild(child)
		}
		node.InsertBefore(child, old_child)
		node.RemoveChild(old_child)
		runtime.queue_dynamic_resource(child)
		return runtime.node_object(old_child)
	})
	_ = object.Set("insertBefore", func(call goja.FunctionCall) goja.Value {
		child := runtime.object_node(call.Argument(0))
		mark := runtime.object_node(call.Argument(1))
		if child == nil {
			return goja.Null()
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
		if child.Parent != nil {
			child.Parent.RemoveChild(child)
		}
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
		if child.Parent != nil {
			child.Parent.RemoveChild(child)
		}
		switch strings.ToLower(position) {
		case "beforebegin":
			if node.Parent == nil {
				return nil
			}
			node.Parent.InsertBefore(child, node)
		case "afterbegin":
			node.InsertBefore(child, node.FirstChild)
		case "beforeend":
			node.AppendChild(child)
		case "afterend":
			if node.Parent == nil {
				return nil
			}
			if node.NextSibling == nil {
				node.Parent.AppendChild(child)
			} else {
				node.Parent.InsertBefore(child, node.NextSibling)
			}
		default:
			return nil
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
	add_event_listener := func(call goja.FunctionCall) goja.Value {
		if callback, ok := goja.AssertFunction(call.Argument(1)); ok {
			event_name := call.Argument(0).String()
			if runtime.listeners[node] == nil {
				runtime.listeners[node] = make(map[string][]goja.Callable)
			}
			runtime.listeners[node][event_name] = append(runtime.listeners[node][event_name], callback)
		}
		return goja.Undefined()
	}
	remove_event_listener := func() {}
	dispatch_event := func(call goja.FunctionCall) goja.Value {
		event_name, ok := event_type(runtime.vm, call.Argument(0))
		if !ok {
			panic(runtime.vm.NewTypeError("dispatchEvent requires an Event: %v", call.Argument(0).ToObject(runtime.vm).GetOwnPropertyNames()))
		}
		return runtime.vm.ToValue(runtime.dispatch_node_event(node, call.Argument(0).ToObject(runtime.vm), event_name))
	}
	_ = object.Set("__minib_addEventListener", add_event_listener)
	_ = object.Set("__minib_removeEventListener", remove_event_listener)
	_ = object.Set("__minib_dispatchEvent", dispatch_event)
	_ = object.Set("addEventListener", add_event_listener)
	_ = object.Set("removeEventListener", remove_event_listener)
	_ = object.Set("dispatchEvent", dispatch_event)
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
	define_getter(runtime.vm, object, "domain", func() any { return runtime.page_url.Hostname() })
	define_getter(runtime.vm, object, "referrer", func() any { return "" })
	define_getter(runtime.vm, object, "hidden", func() any { return false })
	define_getter(runtime.vm, object, "visibilityState", func() any { return "visible" })
	define_getter(runtime.vm, object, "activeElement", func() any { return runtime.node_object(find_element(node, "body")) })
	define_getter(runtime.vm, object, "scripts", func() any { return runtime.node_array(find_by_tag(node, "script")) })
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
	_ = object.Set("getElementById", func(id string) any { return runtime.node_object(find_by_attribute(node, "id", id)) })
	_ = object.Set("getElementsByTagName", func(name string) any { return runtime.node_array(find_by_tag(node, name)) })
	_ = object.Set("getElementsByClassName", func(name string) any { return runtime.node_array(find_by_class(node, name)) })
	_ = object.Set("querySelector", func(selector string) any { return runtime.node_object(query_first(node, selector)) })
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
	for _, name := range []string{"createElement", "createElementNS", "createTextNode", "createDocumentFragment", "createComment", "createEvent", "createRange", "importNode", "getElementById", "getElementsByTagName", "getElementsByClassName", "querySelector", "querySelectorAll"} {
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
	_ = object.Set("querySelector", func(selector string) any { return runtime.node_object(query_first(node, selector)) })
	_ = object.Set("querySelectorAll", func(selector string) any { return runtime.node_array(query_all(node, selector)) })
	_ = object.Set("getElementsByTagName", func(name string) any { return runtime.node_array(find_by_tag(node, name)) })
	_ = object.Set("getElementsByClassName", func(name string) any { return runtime.node_array(find_by_class(node, name)) })
	_ = object.Set("matches", func(selector string) bool {
		matcher, err := cascadia.Parse(selector)
		return err == nil && matcher.Match(node)
	})
	_ = object.Set("closest", func(selector string) any {
		for current := node; current != nil; current = current.Parent {
			matcher, err := cascadia.Parse(selector)
			if err == nil && current.Type == html.ElementNode && matcher.Match(current) {
				return runtime.node_object(current)
			}
		}
		return nil
	})
	bounding_rect := func() map[string]float64 {
		if contains_node(runtime.page.Document, node) {
			// ponytail: synthetic layout is enough until callers need CSS-accurate geometry.
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
	for _, name := range []string{"insertAdjacentElement", "getAttribute", "setAttribute", "getAttributeNS", "setAttributeNS", "removeAttribute", "removeAttributeNS", "hasAttribute", "hasAttributeNS", "hasAttributes", "querySelector", "querySelectorAll", "getElementsByTagName", "getElementsByClassName", "matches", "closest", "getBoundingClientRect", "getClientRects"} {
		_ = object.Set("__minib_"+name, object.Get(name))
	}
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

func (runtime *page_runtime) style_object(node *html.Node) *goja.Object {
	if node != nil && runtime.styles[node] != nil {
		return runtime.styles[node]
	}
	object := runtime.vm.NewObject()
	values := make(map[string]string)
	if node != nil {
		for _, declaration := range strings.Split(attribute(node, "style"), ";") {
			parts := strings.SplitN(declaration, ":", 2)
			if len(parts) == 2 {
				values[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	}
	_ = object.Set("getPropertyValue", func(name string) string { return values[name] })
	_ = object.Set("setProperty", func(name string, value string) { values[name] = value; _ = object.Set(name, value) })
	_ = object.Set("removeProperty", func(name string) string {
		value := values[name]
		delete(values, name)
		_ = object.Delete(name)
		return value
	})
	for name, value := range values {
		_ = object.Set(name, value)
	}
	if node != nil {
		runtime.styles[node] = object
	}
	return object
}

func (runtime *page_runtime) computed_style_object(node *html.Node) *goja.Object {
	object := runtime.style_object(node)
	for name, value := range map[string]string{"fontSize": "16px", "display": "block", "visibility": "visible", "opacity": "1"} {
		if current := object.Get(name); current == nil || goja.IsUndefined(current) {
			_ = object.Set(name, value)
		}
	}
	return object
}

func (runtime *page_runtime) class_list_object(node *html.Node) *goja.Object {
	object := runtime.vm.NewObject()
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
	if node_id := object.Get("__minib_node_id"); node_id != nil {
		if node := runtime.node_ids[node_id.ToInteger()]; node != nil {
			return node
		}
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
	runtime.connect_custom_elements(node)
	if runtime.dynamic_seen[node] {
		return
	}
	switch {
	case strings.EqualFold(node.Data, "script"):
		runtime.dynamic_seen[node] = true
		runtime.dynamic_scripts = append(runtime.dynamic_scripts, node)
	case strings.EqualFold(node.Data, "link") && strings.EqualFold(attribute(node, "rel"), "stylesheet"):
		runtime.dynamic_seen[node] = true
		runtime.dynamic_styles = append(runtime.dynamic_styles, node)
	case strings.EqualFold(node.Data, "img") && attribute(node, "src") != "":
		runtime.dynamic_seen[node] = true
		runtime.dynamic_resources = append(runtime.dynamic_resources, node)
	}
}

func (runtime *page_runtime) drain_dynamic_styles(ctx context.Context) {
	for loaded := 0; len(runtime.dynamic_styles) > 0 && loaded < max_dynamic_scripts && ctx.Err() == nil; loaded++ {
		node := runtime.dynamic_styles[0]
		runtime.dynamic_styles = runtime.dynamic_styles[1:]
		resource_url, ok := resolve_resource_url(runtime.page_url, attribute(node, "href"))
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
	}
}

func (runtime *page_runtime) drain_dynamic_scripts(ctx context.Context) {
	for loaded := 0; len(runtime.dynamic_scripts) > 0 && loaded < max_dynamic_scripts && ctx.Err() == nil; loaded++ {
		node := runtime.dynamic_scripts[0]
		runtime.dynamic_scripts = runtime.dynamic_scripts[1:]
		source_url := attribute(node, "src")
		job := script_job{node: node, resource_index: -1, inline: text_content(node), source_url: runtime.page.URL + "#dynamic-script"}
		if source_url != "" {
			resolved_url, ok := resolve_resource_url(runtime.page_url, source_url)
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
		if len(runtime.page.ScriptFailures) == 0 || runtime.page.ScriptFailures[len(runtime.page.ScriptFailures)-1].URL != job.source_url {
			runtime.fire_node_event(node, "load")
		} else {
			runtime.fire_node_event(node, "error")
		}
	}
}

func (runtime *page_runtime) drain_dynamic_resources(ctx context.Context) {
	for loaded := 0; len(runtime.dynamic_resources) > 0 && loaded < max_dynamic_resources && ctx.Err() == nil; loaded++ {
		node := runtime.dynamic_resources[0]
		runtime.dynamic_resources = runtime.dynamic_resources[1:]
		resource_url, ok := resolve_resource_url(runtime.page_url, attribute(node, "src"))
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
	}
}

func (runtime *page_runtime) find_or_download_resource(ctx context.Context, resource_url string, kind ResourceKind) int {
	for index := range runtime.page.Resources {
		if runtime.page.Resources[index].URL == resource_url {
			return index
		}
	}
	resource := runtime.browser.download_resource(ctx, runtime.page_url, Resource{URL: resource_url, Kind: kind}, runtime.page.disable_cache)
	runtime.page.Resources = append(runtime.page.Resources, resource)
	return len(runtime.page.Resources) - 1
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
	matcher, err := cascadia.Parse(selector)
	if err != nil {
		return nil
	}
	return cascadia.Query(node, matcher)
}

func query_all(node *html.Node, selector string) []*html.Node {
	matcher, err := cascadia.Parse(selector)
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
