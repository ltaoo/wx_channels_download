package minib

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/dop251/goja"
	"github.com/evanw/esbuild/pkg/api"
	"golang.org/x/net/html"

	"wx_channel/pkg/clawreq"
)

type module_state uint8

const (
	module_evaluating module_state = iota + 1
	module_evaluated
	module_failed
)

type module_record struct {
	module *goja.Object
	state  module_state
	err    error
}

type import_map_document struct {
	Imports map[string]string `json:"imports"`
}

type module_prefetch_result struct {
	resource      *Resource
	dependencies  []string
	context_error error
}

var module_require_pattern = regexp.MustCompile(`require\(\s*["']([^"']+)["']\s*\)`)

func parse_document_import_map(document *html.Node, document_url *url.URL) map[string]string {
	imports := make(map[string]string)
	for _, script := range find_by_tag(document, "script") {
		if !strings.EqualFold(strings.TrimSpace(attribute(script, "type")), "importmap") {
			continue
		}
		var import_map import_map_document
		if json.Unmarshal([]byte(text_content(script)), &import_map) != nil {
			continue
		}
		for raw_key, raw_target := range import_map.Imports {
			key := strings.TrimSpace(raw_key)
			target := strings.TrimSpace(raw_target)
			if key == "" || target == "" {
				continue
			}
			if is_url_like_module_specifier(key) {
				if resolved_key, ok := resolve_module_url(document_url.String(), key); ok {
					key = resolved_key
				}
			}
			if resolved_target, ok := resolve_module_url(document_url.String(), target); ok {
				imports[key] = resolved_target
			}
		}
	}
	return imports
}

func (runtime *page_runtime) resolve_module_specifier(referrer_url string, specifier string) (string, error) {
	specifier = strings.TrimSpace(specifier)
	if specifier == "" {
		return "", fmt.Errorf("minib: empty module specifier")
	}
	if mapped_target, ok := runtime.import_map[specifier]; ok {
		return mapped_target, nil
	}
	prefixes := make([]string, 0)
	for key := range runtime.import_map {
		if strings.HasSuffix(key, "/") && strings.HasPrefix(specifier, key) {
			prefixes = append(prefixes, key)
		}
	}
	sort.Slice(prefixes, func(left_index int, right_index int) bool {
		return len(prefixes[left_index]) > len(prefixes[right_index])
	})
	if len(prefixes) > 0 {
		prefix := prefixes[0]
		return runtime.import_map[prefix] + strings.TrimPrefix(specifier, prefix), nil
	}
	if !is_url_like_module_specifier(specifier) {
		return "", fmt.Errorf("minib: unresolved bare module specifier %q", specifier)
	}
	if resolved_url, ok := resolve_module_url(referrer_url, specifier); ok {
		return resolved_url, nil
	}
	return "", fmt.Errorf("minib: invalid module specifier %q from %q", specifier, referrer_url)
}

func is_url_like_module_specifier(specifier string) bool {
	return strings.HasPrefix(specifier, "/") || strings.HasPrefix(specifier, "./") || strings.HasPrefix(specifier, "../") || strings.HasPrefix(specifier, "//") || strings.Contains(specifier, ":")
}

func resolve_module_url(referrer_url string, specifier string) (string, bool) {
	base_url, err := url.Parse(referrer_url)
	if err != nil {
		return "", false
	}
	target_url, err := base_url.Parse(specifier)
	if err != nil || (target_url.Scheme != "http" && target_url.Scheme != "https") {
		return "", false
	}
	target_url.Fragment = ""
	return target_url.String(), true
}

func (runtime *page_runtime) evaluate_module(ctx context.Context, module_url string, source_override *string) (*goja.Object, bool, error) {
	if record := runtime.modules[module_url]; record != nil {
		switch record.state {
		case module_evaluating, module_evaluated:
			return module_record_namespace(runtime.vm, record), false, nil
		case module_failed:
			return nil, false, record.err
		}
	}
	module := runtime.vm.NewObject()
	_ = module.Set("exports", runtime.vm.NewObject())
	record := &module_record{module: module, state: module_evaluating}
	runtime.modules[module_url] = record
	fail := func(err error) (*goja.Object, bool, error) {
		record.state = module_failed
		record.err = err
		return nil, false, err
	}

	source, content_type, err := runtime.module_source(ctx, module_url, source_override)
	if err != nil {
		return fail(err)
	}
	transformed_source, err := transform_module_source(module_url, source, content_type)
	if err != nil {
		return fail(err)
	}
	wrapper_source := "(function(module,exports,require,__minib_import){\n\"use strict\";\n" + transformed_source + "\n;return module.exports;\n})"
	wrapper_value, err := runtime.run_javascript(ctx, module_url, wrapper_source)
	if err != nil {
		return fail(err)
	}
	wrapper, ok := goja.AssertFunction(wrapper_value)
	if !ok {
		return fail(fmt.Errorf("minib: module %q did not compile to a function", module_url))
	}
	require := func(specifier string) goja.Value {
		dependency_url, resolve_err := runtime.resolve_module_specifier(module_url, specifier)
		if resolve_err != nil {
			panic(runtime.vm.NewGoError(resolve_err))
		}
		namespace, _, load_err := runtime.evaluate_module(ctx, dependency_url, nil)
		if load_err != nil {
			panic(runtime.vm.NewGoError(load_err))
		}
		return namespace
	}
	dynamic_import := func(specifier string) *goja.Promise {
		return runtime.import_module(module_url, specifier)
	}
	exports := module.Get("exports")
	if _, err := runtime.call_javascript(ctx, wrapper, goja.Undefined(), module, exports, runtime.vm.ToValue(require), runtime.vm.ToValue(dynamic_import)); err != nil {
		return fail(err)
	}
	record.state = module_evaluated
	runtime.page.ExecutedScripts++
	return module_record_namespace(runtime.vm, record), true, nil
}

func (runtime *page_runtime) prefetch_module_graph(ctx context.Context, module_url string, source_override *string) error {
	visited := map[string]bool{module_url: true}
	wave := []string{module_url}
	worker_count := resource_concurrency

	for len(wave) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		results := make([]module_prefetch_result, len(wave))
		wait_group := &sync.WaitGroup{}
		worker_slots := make(chan struct{}, worker_count)
		for index, current_module_url := range wave {
			wait_group.Add(1)
			worker_slots <- struct{}{}
			go func(result_index int, current_url string, current_source_override *string) {
				defer wait_group.Done()
				defer func() { <-worker_slots }()
				results[result_index] = runtime.prefetch_module(ctx, current_url, current_source_override)
			}(index, current_module_url, source_override_for_module(module_url, current_module_url, source_override))
		}
		wait_group.Wait()

		next_wave_set := make(map[string]bool)
		for _, result := range results {
			if result.resource != nil {
				runtime.page.Resources = append(runtime.page.Resources, *result.resource)
			}
			if result.context_error != nil {
				return result.context_error
			}
			for _, dependency_url := range result.dependencies {
				if visited[dependency_url] || runtime.modules[dependency_url] != nil {
					continue
				}
				visited[dependency_url] = true
				next_wave_set[dependency_url] = true
			}
		}
		wave = make([]string, 0, len(next_wave_set))
		for dependency_url := range next_wave_set {
			wave = append(wave, dependency_url)
		}
		sort.Strings(wave)
	}
	return nil
}

func source_override_for_module(root_url string, current_url string, source_override *string) *string {
	if root_url == current_url {
		return source_override
	}
	return nil
}

func (runtime *page_runtime) prefetch_module(ctx context.Context, module_url string, source_override *string) module_prefetch_result {
	result := module_prefetch_result{}
	var load_error error
	if err := ctx.Err(); err != nil {
		result.context_error = err
		return result
	}

	source := ""
	content_type := ""
	resource_found := false
	if source_override != nil {
		source, content_type = *source_override, "application/javascript"
	} else {
		for index := range runtime.page.Resources {
			if runtime.page.Resources[index].URL != module_url {
				continue
			}
			if runtime.page.Resources[index].Err != nil {
				load_error = runtime.page.Resources[index].Err
				return result
			}
			source, content_type, load_error = runtime.decode_module_resource(module_url, runtime.page.Resources[index])
			resource_found = true
			break
		}
		if !resource_found && load_error == nil {
			resource_ctx, cancel := context_with_optional_timeout(ctx, runtime.page.resource_timeout)
			resource := runtime.browser.download_resource(resource_ctx, runtime.page_url, runtime.request_headers, Resource{
				URL:            module_url,
				Kind:           ScriptResource,
				fetch_priority: default_resource_priority(ScriptResource),
			}, runtime.page.disable_cache)
			cancel()
			resource_copy := resource
			result.resource = &resource_copy
			if resource.Err != nil {
				load_error = resource.Err
				return result
			}
			source, content_type, load_error = runtime.decode_module_resource(module_url, resource)
		}
	}
	if load_error != nil {
		return result
	}

	transformed_source, transform_err := transform_module_source(module_url, source, content_type)
	if transform_err != nil {
		return result
	}
	result.dependencies = runtime.module_dependency_urls(module_url, transformed_source)
	return result
}

func (runtime *page_runtime) decode_module_resource(module_url string, resource Resource) (string, string, error) {
	if len(resource.Body) > max_script_size {
		return "", "", fmt.Errorf("module %q exceeds %d bytes", module_url, max_script_size)
	}
	source, err := clawreq.DecodeText(resource.Body, resource.ContentType)
	if err != nil {
		return "", "", fmt.Errorf("decode module %q: %w", module_url, err)
	}
	return source, resource.ContentType, nil
}

func (runtime *page_runtime) module_dependency_urls(module_url string, transformed_source string) []string {
	matches := module_require_pattern.FindAllStringSubmatch(transformed_source, -1)
	dependencies := make([]string, 0, len(matches))
	seen := make(map[string]bool, len(matches))
	for _, match := range matches {
		specifier := match[1]
		if seen[specifier] {
			continue
		}
		seen[specifier] = true
		dependency_url, err := runtime.resolve_module_specifier(module_url, specifier)
		if err != nil {
			continue
		}
		dependencies = append(dependencies, dependency_url)
	}
	return dependencies
}

func module_record_namespace(vm *goja.Runtime, record *module_record) *goja.Object {
	if record == nil || record.module == nil {
		return nil
	}
	exports := record.module.Get("exports")
	if exports == nil || goja.IsNull(exports) || goja.IsUndefined(exports) {
		return vm.NewObject()
	}
	return exports.ToObject(vm)
}

func (runtime *page_runtime) module_source(ctx context.Context, module_url string, source_override *string) (string, string, error) {
	if source_override != nil {
		return *source_override, "application/javascript", nil
	}
	resource_index := runtime.find_or_download_resource(ctx, module_url, ScriptResource)
	resource := runtime.page.Resources[resource_index]
	if resource.Err != nil {
		return "", "", fmt.Errorf("module download %q: %w", module_url, resource.Err)
	}
	if len(resource.Body) > max_script_size {
		return "", "", fmt.Errorf("module %q exceeds %d bytes", module_url, max_script_size)
	}
	source, err := clawreq.DecodeText(resource.Body, resource.ContentType)
	if err != nil {
		return "", "", fmt.Errorf("decode module %q: %w", module_url, err)
	}
	return source, resource.ContentType, nil
}

func transform_module_source(module_url string, source string, content_type string) (string, error) {
	loader := api.LoaderJS
	parsed_url, _ := url.Parse(module_url)
	if strings.Contains(strings.ToLower(content_type), "json") || strings.EqualFold(path.Ext(parsed_url.Path), ".json") {
		loader = api.LoaderJSON
	}
	transformed := api.Transform(source, api.TransformOptions{
		Sourcefile: module_url,
		Loader:     loader,
		Target:     api.ES2020,
		Charset:    api.CharsetUTF8,
		Format:     api.FormatCommonJS,
		Define:     map[string]string{"import.meta.url": fmt.Sprintf("%q", module_url)},
	})
	if len(transformed.Errors) > 0 {
		return "", fmt.Errorf("module transform %q: %s", module_url, transformed.Errors[0].Text)
	}
	return strings.ReplaceAll(string(transformed.Code), "import(", "__minib_import("), nil
}

func (runtime *page_runtime) import_module(referrer_url string, specifier string) *goja.Promise {
	promise, resolve, reject := runtime.vm.NewPromise()
	module_url, err := runtime.resolve_module_specifier(referrer_url, specifier)
	if err != nil {
		_ = reject(runtime.vm.NewGoError(err))
		return promise
	}
	runtime.queue_host_job(func() {
		if err := runtime.prefetch_module_graph(runtime.ctx, module_url, nil); err != nil {
			_ = reject(runtime.vm.NewGoError(err))
			return
		}
		namespace, _, load_err := runtime.evaluate_module(runtime.ctx, module_url, nil)
		if load_err != nil {
			_ = reject(runtime.vm.NewGoError(load_err))
			return
		}
		_ = resolve(namespace)
	})
	return promise
}
