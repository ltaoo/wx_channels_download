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
	"sync"
)

const cache_directory_name = "fanqienovel"

var html_cache_mu sync.RWMutex

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

func cache_root_path(work_dir string) string {
	work_dir = strings.TrimSpace(work_dir)
	if work_dir == "" {
		return ""
	}
	return filepath.Join(work_dir, "cache", cache_directory_name)
}

func cache_namespace_path(work_dir string, source_url string) (string, error) {
	root_path := cache_root_path(work_dir)
	if root_path == "" {
		return "", nil
	}
	key, err := cache_url_key(source_url)
	if err != nil {
		return "", err
	}
	return filepath.Join(root_path, key), nil
}

func html_cache_file_path(work_dir string, source_url string, request_url string) (string, error) {
	namespace_path, err := cache_namespace_path(work_dir, source_url)
	if err != nil || namespace_path == "" {
		return "", err
	}
	request_key, err := cache_url_key(request_url)
	if err != nil {
		return "", err
	}
	cache_path := filepath.Join(namespace_path, request_key+".html")
	absolute_path, err := filepath.Abs(cache_path)
	if err != nil {
		return "", fmt.Errorf("resolve fanqienovel cache path: %w", err)
	}
	return absolute_path, nil
}

func (c *FanqieClient) cache_file_path(request_url string) (string, error) {
	if c == nil || strings.TrimSpace(c.work_dir) == "" || strings.TrimSpace(c.cache_source_url) == "" {
		return "", nil
	}
	return html_cache_file_path(c.work_dir, c.cache_source_url, request_url)
}

// HTMLCacheFilePath returns the deterministic path for a cached request page.
// The file does not need to exist yet.
func HTMLCacheFilePath(work_dir string, source_url string, request_url string) (string, error) {
	if _, err := parse_book_id(strings.TrimSpace(source_url)); err != nil {
		return "", err
	}
	return html_cache_file_path(work_dir, strings.TrimSpace(source_url), strings.TrimSpace(request_url))
}

// LookupChapterHTMLCache locates the cached raw HTML for a chapter without
// performing a network request. A nil result means that chapter is not cached.
func LookupChapterHTMLCache(work_dir string, source_url string, chapter_url string) (*HTMLCacheFile, error) {
	chapter_id, err := parse_chapter_id(chapter_url)
	if err != nil {
		return nil, err
	}
	parsed_url, err := url.Parse(strings.TrimSpace(chapter_url))
	if err != nil || parsed_url.Scheme == "" || parsed_url.Host == "" {
		return nil, fmt.Errorf("invalid chapter url %q", chapter_url)
	}
	parsed_url.Path = "/reader/" + chapter_id
	parsed_url.RawPath = ""
	parsed_url.RawQuery = "enter_from=page"
	parsed_url.Fragment = ""
	cache_path, err := HTMLCacheFilePath(work_dir, source_url, parsed_url.String())
	if err != nil || cache_path == "" {
		return nil, err
	}

	html_cache_mu.RLock()
	file_info, stat_err := os.Stat(cache_path)
	html_cache_mu.RUnlock()
	if errors.Is(stat_err, os.ErrNotExist) {
		return nil, nil
	}
	if stat_err != nil {
		return nil, fmt.Errorf("stat fanqienovel chapter cache: %w", stat_err)
	}
	if !file_info.Mode().IsRegular() || file_info.Size() <= 0 {
		return nil, nil
	}
	return &HTMLCacheFile{Path: cache_path, Size: file_info.Size()}, nil
}

func (c *FanqieClient) read_cached_html(request_url string) ([]byte, bool, error) {
	cache_path, err := c.cache_file_path(request_url)
	if err != nil || cache_path == "" {
		return nil, false, err
	}
	html_cache_mu.RLock()
	data, read_err := os.ReadFile(cache_path)
	html_cache_mu.RUnlock()
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
	cache_path, err := c.cache_file_path(request_url)
	if err != nil || cache_path == "" {
		return err
	}
	html_cache_mu.Lock()
	defer html_cache_mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(cache_path), 0755); err != nil {
		return fmt.Errorf("create fanqienovel cache directory: %w", err)
	}
	temporary_file, err := os.CreateTemp(filepath.Dir(cache_path), ".html-*.tmp")
	if err != nil {
		return fmt.Errorf("create fanqienovel cache file: %w", err)
	}
	temporary_path := temporary_file.Name()
	defer os.Remove(temporary_path)
	if _, err := temporary_file.Write(data); err != nil {
		_ = temporary_file.Close()
		return fmt.Errorf("write fanqienovel cache file: %w", err)
	}
	if err := temporary_file.Close(); err != nil {
		return fmt.Errorf("close fanqienovel cache file: %w", err)
	}
	if err := os.Remove(cache_path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("replace fanqienovel cache file: %w", err)
	}
	if err := os.Rename(temporary_path, cache_path); err != nil {
		return fmt.Errorf("commit fanqienovel cache file: %w", err)
	}
	return nil
}

func (c *FanqieClient) remove_cached_html(request_url string) error {
	cache_path, err := c.cache_file_path(request_url)
	if err != nil || cache_path == "" {
		return err
	}
	html_cache_mu.Lock()
	remove_err := os.Remove(cache_path)
	html_cache_mu.Unlock()
	if errors.Is(remove_err, os.ErrNotExist) {
		return nil
	}
	return remove_err
}

// ClearHTMLCache removes all cached profile and chapter HTML associated with
// one source book URL.
func ClearHTMLCache(work_dir string, source_url string) (bool, error) {
	if _, err := parse_book_id(source_url); err != nil {
		return false, err
	}
	namespace_path, err := cache_namespace_path(work_dir, source_url)
	if err != nil || namespace_path == "" {
		return false, err
	}
	html_cache_mu.Lock()
	defer html_cache_mu.Unlock()
	if _, err := os.Stat(namespace_path); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("stat fanqienovel cache: %w", err)
	}
	if err := os.RemoveAll(namespace_path); err != nil {
		return false, fmt.Errorf("clear fanqienovel cache: %w", err)
	}
	return true, nil
}
