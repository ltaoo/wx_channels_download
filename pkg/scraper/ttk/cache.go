package ttk

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"wx_channel/pkg/cache"
)

const cache_directory_name = "ttk"

// HTMLCacheFile describes one HTML page already persisted in the fetch cache.
type HTMLCacheFile struct {
	Path string
	Size int64
}

// SetWorkDir enables persistent HTML caching beneath the runtime workdir.
func (c *Client) SetWorkDir(work_dir string) {
	if c == nil {
		return
	}
	cache_registry, err := cache.NewProviderRegistry(work_dir)
	if err != nil {
		c.file_cache = nil
		return
	}
	c.file_cache, _ = cache_registry.Namespace(cache_directory_name)
}

// SetPersistentCache configures the namespace-scoped persistent HTML cache.
func (c *Client) SetPersistentCache(file_cache *cache.CacheProvider) {
	if c == nil {
		return
	}
	c.file_cache = file_cache
}

func normalize_cache_url(raw_url string) (string, error) {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil || parsed_url.Scheme == "" || parsed_url.Host == "" {
		return "", fmt.Errorf("invalid cache url %q", raw_url)
	}
	parsed_url.Scheme = strings.ToLower(parsed_url.Scheme)
	parsed_url.Host = strings.ToLower(parsed_url.Host)
	parsed_url.Fragment = ""
	if parsed_url.Path == "" {
		parsed_url.Path = "/"
	}
	return parsed_url.String(), nil
}

func cache_url_key(raw_url string) (string, error) {
	normalized_url, err := normalize_cache_url(raw_url)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(normalized_url))
	return hex.EncodeToString(digest[:]), nil
}

func cache_namespace_path(source_url string) (string, error) {
	key, err := cache_url_key(source_url)
	if err != nil {
		return "", err
	}
	return key, nil
}

func html_cache_relative_path(source_url string, request_url string) (string, error) {
	namespace_path, err := cache_namespace_path(source_url)
	if err != nil {
		return "", err
	}
	request_key, err := cache_url_key(request_url)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(filepath.Join(namespace_path, request_key+".html")), nil
}

func (c *Client) cache_file_path(source_url string, request_url string) (string, error) {
	if c == nil || c.file_cache == nil || !c.file_cache.Enabled() || strings.TrimSpace(source_url) == "" {
		return "", nil
	}
	relative_path, err := html_cache_relative_path(source_url, request_url)
	if err != nil {
		return "", err
	}
	return c.file_cache.Path(relative_path)
}

// HTMLCacheFilePath returns the deterministic path for a cached request page.
// The file does not need to exist yet.
func HTMLCacheFilePath(work_dir string, source_url string, request_url string) (string, error) {
	cache_registry, err := cache.NewProviderRegistry(work_dir)
	if err != nil {
		return "", err
	}
	file_cache, err := cache_registry.Namespace(cache_directory_name)
	if err != nil {
		return "", err
	}
	return HTMLCacheFilePathWithCache(file_cache, source_url, request_url)
}

// HTMLCacheFilePathWithCache returns the deterministic cache path using a
// namespace-scoped persistent cache.
func HTMLCacheFilePathWithCache(file_cache *cache.CacheProvider, source_url string, request_url string) (string, error) {
	if _, err := validate_novel_url(strings.TrimSpace(source_url)); err != nil {
		return "", err
	}
	if file_cache == nil || !file_cache.Enabled() {
		return "", nil
	}
	relative_path, err := html_cache_relative_path(strings.TrimSpace(source_url), strings.TrimSpace(request_url))
	if err != nil {
		return "", err
	}
	return file_cache.Path(relative_path)
}

// LookupChapterHTMLCache locates the cached raw HTML for a chapter without
// performing a network request. A nil result means that chapter is not cached.
func LookupChapterHTMLCache(work_dir string, source_url string, chapter_url string) (*HTMLCacheFile, error) {
	cache_registry, err := cache.NewProviderRegistry(work_dir)
	if err != nil {
		return nil, err
	}
	file_cache, err := cache_registry.Namespace(cache_directory_name)
	if err != nil {
		return nil, err
	}
	return LookupChapterHTMLCacheWithCache(file_cache, source_url, chapter_url)
}

// LookupChapterHTMLCacheWithCache locates a chapter in a namespace-scoped
// persistent cache.
func LookupChapterHTMLCacheWithCache(file_cache *cache.CacheProvider, source_url string, chapter_url string) (*HTMLCacheFile, error) {
	if _, err := validate_novel_url(source_url); err != nil {
		return nil, err
	}
	if err := validate_ttk_url(chapter_url); err != nil {
		return nil, err
	}
	cache_path, err := HTMLCacheFilePathWithCache(file_cache, source_url, strings.TrimSpace(chapter_url))
	if err != nil || cache_path == "" {
		return nil, err
	}
	relative_path, err := html_cache_relative_path(source_url, strings.TrimSpace(chapter_url))
	if err != nil {
		return nil, err
	}
	file_info, stat_err := file_cache.Stat(relative_path)
	if errors.Is(stat_err, os.ErrNotExist) {
		return nil, nil
	}
	if stat_err != nil {
		return nil, fmt.Errorf("stat ttk chapter cache: %w", stat_err)
	}
	if !file_info.Mode().IsRegular() || file_info.Size() <= 0 {
		return nil, nil
	}
	return &HTMLCacheFile{Path: cache_path, Size: file_info.Size()}, nil
}

func (c *Client) read_cached_html(source_url string, request_url string) ([]byte, bool, error) {
	if c == nil || c.file_cache == nil || !c.file_cache.Enabled() || strings.TrimSpace(source_url) == "" {
		return nil, false, nil
	}
	relative_path, err := html_cache_relative_path(source_url, request_url)
	if err != nil {
		return nil, false, err
	}
	data, read_err := c.file_cache.Read(relative_path)
	if errors.Is(read_err, os.ErrNotExist) {
		return nil, false, nil
	}
	if read_err != nil {
		return nil, false, fmt.Errorf("read ttk cache: %w", read_err)
	}
	if len(data) == 0 {
		_ = c.remove_cached_html(source_url, request_url)
		return nil, false, nil
	}
	return data, true, nil
}

func (c *Client) write_cached_html(source_url string, request_url string, data []byte) error {
	if c == nil || c.file_cache == nil || !c.file_cache.Enabled() || strings.TrimSpace(source_url) == "" {
		return nil
	}
	relative_path, err := html_cache_relative_path(source_url, request_url)
	if err != nil {
		return err
	}
	return c.file_cache.Write(relative_path, data)
}

func (c *Client) remove_cached_html(source_url string, request_url string) error {
	if c == nil || c.file_cache == nil || !c.file_cache.Enabled() || strings.TrimSpace(source_url) == "" {
		return nil
	}
	relative_path, err := html_cache_relative_path(source_url, request_url)
	if err != nil {
		return err
	}
	return c.file_cache.Remove(relative_path)
}

// ClearHTMLCache removes all cached profile and chapter HTML associated with
// one source TTK novel URL.
func ClearHTMLCache(work_dir string, source_url string) (bool, error) {
	cache_registry, err := cache.NewProviderRegistry(work_dir)
	if err != nil {
		return false, err
	}
	file_cache, err := cache_registry.Namespace(cache_directory_name)
	if err != nil {
		return false, err
	}
	return ClearHTMLCacheWithCache(file_cache, source_url)
}

// ClearHTMLCacheWithCache removes cached pages for one novel from a
// namespace-scoped persistent cache.
func ClearHTMLCacheWithCache(file_cache *cache.CacheProvider, source_url string) (bool, error) {
	if _, err := validate_novel_url(source_url); err != nil {
		return false, err
	}
	if file_cache == nil || !file_cache.Enabled() {
		return false, nil
	}
	namespace_path, err := cache_namespace_path(source_url)
	if err != nil {
		return false, err
	}
	removed, err := file_cache.RemoveAll(namespace_path)
	if err != nil {
		return false, fmt.Errorf("clear ttk cache: %w", err)
	}
	return removed, nil
}
