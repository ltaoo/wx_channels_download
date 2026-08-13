package fanqienovel

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

// HTMLCacheFile describes one HTML page already persisted in the fetch cache.
type HTMLCacheFile struct {
	Path string
	Size int64
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

func (c *FanqieClient) cache_file_path(request_url string) (string, error) {
	if c == nil || c.file_cache == nil || !c.file_cache.Enabled() || strings.TrimSpace(c.cache_source_url) == "" {
		return "", nil
	}
	relative_path, err := html_cache_relative_path(c.cache_source_url, request_url)
	if err != nil {
		return "", err
	}
	return c.file_cache.Path(relative_path)
}

// HTMLCacheFilePath returns the deterministic cache path using a
// namespace-scoped persistent cache. The file does not need to exist yet.
func HTMLCacheFilePath(file_cache *cache.CacheProvider, source_url string, request_url string) (string, error) {
	if _, err := parse_book_id(strings.TrimSpace(source_url)); err != nil {
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

// LookupHTMLCache locates one cached raw HTML response without performing a
// network request. A nil result means the response is not cached.
func LookupHTMLCache(file_cache *cache.CacheProvider, source_url string, request_url string) (*HTMLCacheFile, error) {
	cache_path, err := HTMLCacheFilePath(file_cache, source_url, request_url)
	if err != nil || cache_path == "" {
		return nil, err
	}
	relative_path, err := html_cache_relative_path(source_url, request_url)
	if err != nil {
		return nil, err
	}
	file_info, stat_err := file_cache.Stat(relative_path)
	if errors.Is(stat_err, os.ErrNotExist) {
		return nil, nil
	}
	if stat_err != nil {
		return nil, fmt.Errorf("stat fanqienovel HTML cache: %w", stat_err)
	}
	if !file_info.Mode().IsRegular() || file_info.Size() <= 0 {
		return nil, nil
	}
	return &HTMLCacheFile{Path: cache_path, Size: file_info.Size()}, nil
}

// LookupProfileHTMLCache locates the canonical profile page cached for one
// source book URL.
func LookupProfileHTMLCache(file_cache *cache.CacheProvider, source_url string) (*HTMLCacheFile, error) {
	book_id, err := parse_book_id(strings.TrimSpace(source_url))
	if err != nil {
		return nil, err
	}
	request_url := strings.TrimRight(fanqie_base_url, "/") + "/page/" + book_id
	return LookupHTMLCache(file_cache, source_url, request_url)
}

// LookupChapterHTMLCache locates a chapter in a namespace-scoped persistent
// cache without performing a network request. A nil result means that chapter
// is not cached.
func LookupChapterHTMLCache(file_cache *cache.CacheProvider, source_url string, chapter_url string) (*HTMLCacheFile, error) {
	chapter_id, err := parse_chapter_id(chapter_url)
	if err != nil {
		return nil, err
	}
	request_url := strings.TrimRight(fanqie_base_url, "/") + "/reader/" + chapter_id + "?enter_from=page"
	return LookupHTMLCache(file_cache, source_url, request_url)
}

func (c *FanqieClient) read_cached_html(request_url string) ([]byte, bool, error) {
	if c == nil || c.file_cache == nil || !c.file_cache.Enabled() || strings.TrimSpace(c.cache_source_url) == "" {
		return nil, false, nil
	}
	relative_path, err := html_cache_relative_path(c.cache_source_url, request_url)
	if err != nil {
		return nil, false, err
	}
	data, read_err := c.file_cache.Read(relative_path)
	if errors.Is(read_err, os.ErrNotExist) {
		return nil, false, nil
	}
	if read_err != nil {
		return nil, false, fmt.Errorf("read fanqienovel cache: %w", read_err)
	}
	if len(data) == 0 {
		_ = c.remove_cached_html(request_url)
		return nil, false, nil
	}
	return data, true, nil
}

func (c *FanqieClient) write_cached_html(request_url string, data []byte) error {
	if c == nil || c.file_cache == nil || !c.file_cache.Enabled() || strings.TrimSpace(c.cache_source_url) == "" {
		return nil
	}
	relative_path, err := html_cache_relative_path(c.cache_source_url, request_url)
	if err != nil {
		return err
	}
	return c.file_cache.Write(relative_path, data)
}

func (c *FanqieClient) remove_cached_html(request_url string) error {
	if c == nil || c.file_cache == nil || !c.file_cache.Enabled() || strings.TrimSpace(c.cache_source_url) == "" {
		return nil
	}
	relative_path, err := html_cache_relative_path(c.cache_source_url, request_url)
	if err != nil {
		return err
	}
	return c.file_cache.Remove(relative_path)
}

// ClearHTMLCache removes cached pages for one book from a namespace-scoped
// persistent cache.
func ClearHTMLCache(file_cache *cache.CacheProvider, source_url string) (bool, error) {
	if _, err := parse_book_id(source_url); err != nil {
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
		return false, fmt.Errorf("clear fanqienovel cache: %w", err)
	}
	return removed, nil
}
