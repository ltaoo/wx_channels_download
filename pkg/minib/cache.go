package minib

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type resource_cache_entry struct {
	resource         Resource
	response_headers http.Header
	vary_headers     []string
	vary_values      map[string]string
	stored_at        time.Time
}

type resource_cache struct {
	mutex   sync.RWMutex
	entries map[string][]resource_cache_entry
}

func new_resource_cache() *resource_cache {
	return &resource_cache{entries: make(map[string][]resource_cache_entry)}
}

func (cache *resource_cache) lookup(raw_url string, request_headers http.Header) (resource_cache_entry, bool) {
	if cache == nil {
		return resource_cache_entry{}, false
	}
	cache.mutex.RLock()
	defer cache.mutex.RUnlock()
	for _, entry := range cache.entries[raw_url] {
		if entry.matches(request_headers) {
			return entry.clone(), true
		}
	}
	return resource_cache_entry{}, false
}

func (cache *resource_cache) store(raw_url string, request_headers http.Header, response_headers http.Header, resource Resource, stored_at time.Time) {
	if cache == nil || !response_cacheable(resource.StatusCode, response_headers) {
		return
	}
	vary_headers := response_vary_headers(response_headers)
	entry := resource_cache_entry{
		resource:         clone_resource(resource),
		response_headers: response_headers.Clone(),
		vary_headers:     vary_headers,
		vary_values:      make(map[string]string, len(vary_headers)),
		stored_at:        stored_at,
	}
	entry.resource.FromCache = false
	for _, name := range vary_headers {
		entry.vary_values[name] = request_header_value(request_headers, name)
	}

	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	entries := cache.entries[raw_url]
	kept := entries[:0]
	for _, existing := range entries {
		if !existing.matches(request_headers) {
			kept = append(kept, existing)
		}
	}
	cache.entries[raw_url] = append(kept, entry)
}

func (cache *resource_cache) remove(raw_url string, request_headers http.Header) {
	if cache == nil {
		return
	}
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	entries := cache.entries[raw_url]
	kept := entries[:0]
	for _, entry := range entries {
		if !entry.matches(request_headers) {
			kept = append(kept, entry)
		}
	}
	if len(kept) == 0 {
		delete(cache.entries, raw_url)
	} else {
		cache.entries[raw_url] = kept
	}
}

func (entry resource_cache_entry) clone() resource_cache_entry {
	entry.resource = clone_resource(entry.resource)
	entry.response_headers = entry.response_headers.Clone()
	entry.vary_headers = append([]string(nil), entry.vary_headers...)
	entry.vary_values = clone_string_map(entry.vary_values)
	return entry
}

func (entry resource_cache_entry) matches(request_headers http.Header) bool {
	for _, name := range entry.vary_headers {
		if request_header_value(request_headers, name) != entry.vary_values[name] {
			return false
		}
	}
	return true
}

func (entry resource_cache_entry) fresh(now time.Time) bool {
	directives := cache_control_directives(entry.response_headers)
	if _, no_cache := directives["no-cache"]; no_cache {
		return false
	}
	if len(entry.response_headers.Values("Cache-Control")) == 0 && strings.EqualFold(strings.TrimSpace(entry.response_headers.Get("Pragma")), "no-cache") {
		return false
	}
	freshness_lifetime := response_freshness_lifetime(entry.response_headers, entry.stored_at, directives)
	if freshness_lifetime <= 0 {
		return false
	}
	return response_current_age(entry.response_headers, entry.stored_at, now) < freshness_lifetime
}

func (entry resource_cache_entry) cached_resource() Resource {
	resource := clone_resource(entry.resource)
	resource.FromCache = true
	resource.Err = nil
	return resource
}

func response_cacheable(status_code int, headers http.Header) bool {
	if status_code != http.StatusOK {
		return false
	}
	if _, no_store := cache_control_directives(headers)["no-store"]; no_store {
		return false
	}
	for _, name := range response_vary_headers(headers) {
		if name == "*" {
			return false
		}
	}
	return true
}

func response_vary_headers(headers http.Header) []string {
	seen := make(map[string]bool)
	names := make([]string, 0)
	for _, value := range headers.Values("Vary") {
		for _, name := range strings.Split(value, ",") {
			name = strings.TrimSpace(name)
			if name == "*" {
				return []string{"*"}
			}
			name = http.CanonicalHeaderKey(name)
			if name != "" && !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	sort.Strings(names)
	return names
}

func cache_control_directives(headers http.Header) map[string]string {
	directives := make(map[string]string)
	for _, value := range headers.Values("Cache-Control") {
		for _, part := range strings.Split(value, ",") {
			name, directive_value, _ := strings.Cut(strings.TrimSpace(part), "=")
			name = strings.ToLower(strings.TrimSpace(name))
			if name != "" {
				directives[name] = strings.Trim(strings.TrimSpace(directive_value), `"`)
			}
		}
	}
	return directives
}

func response_freshness_lifetime(headers http.Header, stored_at time.Time, directives map[string]string) time.Duration {
	if value, ok := directives["max-age"]; ok {
		seconds, err := strconv.ParseInt(value, 10, 64)
		if err != nil || seconds <= 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}
	date := response_date(headers, stored_at)
	if expires, err := http.ParseTime(headers.Get("Expires")); err == nil {
		if expires.After(date) {
			return expires.Sub(date)
		}
		return 0
	}
	if last_modified, err := http.ParseTime(headers.Get("Last-Modified")); err == nil && date.After(last_modified) {
		lifetime := date.Sub(last_modified) / 10
		if lifetime > 24*time.Hour {
			return 24 * time.Hour
		}
		return lifetime
	}
	return 0
}

func response_current_age(headers http.Header, stored_at time.Time, now time.Time) time.Duration {
	initial_age := time.Duration(0)
	if date := response_date(headers, stored_at); stored_at.After(date) {
		initial_age = stored_at.Sub(date)
	}
	if seconds, err := strconv.ParseInt(strings.TrimSpace(headers.Get("Age")), 10, 64); err == nil && seconds > 0 {
		age := time.Duration(seconds) * time.Second
		if age > initial_age {
			initial_age = age
		}
	}
	if now.After(stored_at) {
		initial_age += now.Sub(stored_at)
	}
	return initial_age
}

func response_date(headers http.Header, fallback time.Time) time.Time {
	if date, err := http.ParseTime(headers.Get("Date")); err == nil {
		return date
	}
	return fallback
}

func merge_response_headers(cached http.Header, revalidated http.Header) http.Header {
	merged := cached.Clone()
	for name, values := range revalidated {
		merged[name] = append([]string(nil), values...)
	}
	return merged
}

func request_header_value(headers http.Header, name string) string {
	return strings.Join(headers.Values(name), ", ")
}

func clone_resource(resource Resource) Resource {
	resource.Body = append([]byte(nil), resource.Body...)
	return resource
}

func clone_string_map(values map[string]string) map[string]string {
	cloned := make(map[string]string, len(values))
	for name, value := range values {
		cloned[name] = value
	}
	return cloned
}
