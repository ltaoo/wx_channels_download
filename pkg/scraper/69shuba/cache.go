package shuba69

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"wx_channel/pkg/cache"
)

const cache_directory_name = "69shuba"

// HTMLCacheEntry describes one persisted HTML page used for diagnostics.
type HTMLCacheEntry struct {
	Directory string `json:"directory"`
	HTMLPath  string `json:"html_path"`
	Size      int64  `json:"size"`
}

// SetWorkDir enables persistent response caching beneath the runtime workdir.
func (c *Client) SetWorkDir(work_dir string) {
	if c == nil {
		return
	}
	c.work_dir = strings.TrimSpace(work_dir)
	cache_registry, err := cache.NewProviderRegistry(c.work_dir)
	if err != nil {
		c.file_cache = nil
		return
	}
	c.file_cache, _ = cache_registry.Namespace(cache_directory_name)
}

// SetPersistentCache configures the namespace-scoped persistent response
// cache supplied by the runtime composition root.
func (c *Client) SetPersistentCache(file_cache *cache.CacheProvider) {
	if c == nil {
		return
	}
	c.file_cache = file_cache
}

func (c *Client) cache_response(page_kind string, raw_url string, html_text string) (*HTMLCacheEntry, error) {
	if c == nil || c.file_cache == nil || !c.file_cache.Enabled() {
		return nil, nil
	}
	cache_directory, err := response_cache_directory_path(raw_url)
	if err != nil {
		return nil, err
	}
	file_name, err := cache_file_name(page_kind)
	if err != nil {
		return nil, err
	}
	cache_path := filepath.ToSlash(filepath.Join(cache_directory, file_name))
	html_path, err := c.file_cache.Path(cache_path)
	if err != nil {
		return nil, err
	}
	directory_path, err := c.file_cache.Path(filepath.ToSlash(cache_directory))
	if err != nil {
		return nil, err
	}
	entry := &HTMLCacheEntry{Directory: directory_path, HTMLPath: html_path, Size: int64(len(html_text))}
	if err := c.file_cache.Write(cache_path, []byte(html_text)); err != nil {
		return nil, err
	}
	return entry, nil
}

func cache_file_name(page_kind string) (string, error) {
	switch strings.TrimSpace(page_kind) {
	case "profile":
		return "profile.html", nil
	case "directory":
		return "chapters.html", nil
	default:
		return "", fmt.Errorf("unsupported 69shuba cache page kind %q", page_kind)
	}
}

func response_cache_directory(work_dir string, raw_url string) (string, error) {
	work_dir = strings.TrimSpace(work_dir)
	if work_dir == "" {
		return "", nil
	}
	cache_registry, err := cache.NewProviderRegistry(work_dir)
	if err != nil {
		return "", err
	}
	file_cache, err := cache_registry.Namespace(cache_directory_name)
	if err != nil {
		return "", err
	}
	relative_path, err := response_cache_directory_path(raw_url)
	if err != nil {
		return "", err
	}
	return file_cache.Path(filepath.ToSlash(relative_path))
}

func response_cache_directory_path(raw_url string) (string, error) {
	request_hash := sha256.Sum256([]byte(strings.TrimSpace(raw_url)))
	request_key := hex.EncodeToString(request_hash[:])
	book_namespace := "url-" + request_key[:16]
	if matches := book_id_pattern.FindStringSubmatch(raw_url); len(matches) > 1 {
		book_namespace = "book-" + matches[1]
	}
	return book_namespace, nil
}

// LookupHTMLCaches returns the persisted profile and directory HTML for one
// novel URL without performing a network request.
func LookupHTMLCaches(file_cache *cache.CacheProvider, raw_url string) ([]HTMLCacheEntry, error) {
	if file_cache == nil || !file_cache.Enabled() {
		return nil, nil
	}
	cache_directory, err := response_cache_directory_path(raw_url)
	if err != nil {
		return nil, err
	}
	directory_path, err := file_cache.Path(filepath.ToSlash(cache_directory))
	if err != nil {
		return nil, err
	}
	entries := make([]HTMLCacheEntry, 0, 2)
	for _, file_name := range []string{"profile.html", "chapters.html"} {
		relative_path := filepath.ToSlash(filepath.Join(cache_directory, file_name))
		file_info, stat_err := file_cache.Stat(relative_path)
		if errors.Is(stat_err, os.ErrNotExist) {
			continue
		}
		if stat_err != nil {
			return nil, fmt.Errorf("stat 69shuba HTML cache: %w", stat_err)
		}
		if !file_info.Mode().IsRegular() || file_info.Size() <= 0 {
			continue
		}
		html_path, path_err := file_cache.Path(relative_path)
		if path_err != nil {
			return nil, path_err
		}
		entries = append(entries, HTMLCacheEntry{
			Directory: directory_path,
			HTMLPath:  html_path,
			Size:      file_info.Size(),
		})
	}
	return entries, nil
}

// ClearHTMLCache removes the persisted profile and directory HTML for one
// novel URL.
func ClearHTMLCache(file_cache *cache.CacheProvider, raw_url string) (bool, error) {
	if file_cache == nil || !file_cache.Enabled() {
		return false, nil
	}
	cache_directory, err := response_cache_directory_path(raw_url)
	if err != nil {
		return false, err
	}
	removed, err := file_cache.RemoveAll(filepath.ToSlash(cache_directory))
	if err != nil {
		return false, fmt.Errorf("clear 69shuba HTML cache: %w", err)
	}
	return removed, nil
}
