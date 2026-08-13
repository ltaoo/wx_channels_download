package zhihu

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

// HTMLCacheFile describes one persisted Zhihu HTML response.
type HTMLCacheFile struct {
	Path string
	Size int64
}

// SetPersistentCache configures the namespace-scoped persistent HTML cache
// supplied by the runtime composition root.
func (c *Client) SetPersistentCache(file_cache *cache.CacheProvider) {
	if c == nil {
		return
	}
	c.file_cache = file_cache
}

func canonical_cache_url(raw_url string) (string, error) {
	resolved_url := ResolveRealURL(strings.TrimSpace(raw_url))
	if article_url, ok := ParseArticleURL(resolved_url); ok {
		return article_url.Canonical, nil
	}
	if question_url, ok := ParseQuestionURL(resolved_url); ok {
		return question_url.Canonical, nil
	}
	if answer_url, ok := ParseAnswerURL(resolved_url); ok {
		return answer_url.Canonical, nil
	}
	return "", fmt.Errorf("invalid zhihu cache url %q", raw_url)
}

func html_cache_relative_path(raw_url string) (string, error) {
	canonical_url, err := canonical_cache_url(raw_url)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(canonical_url))
	request_key := hex.EncodeToString(digest[:])
	return filepath.ToSlash(filepath.Join("html", request_key+".html")), nil
}

func (c *Client) read_cached_html(raw_url string) ([]byte, bool, error) {
	if c == nil || c.file_cache == nil || !c.file_cache.Enabled() {
		return nil, false, nil
	}
	relative_path, err := html_cache_relative_path(raw_url)
	if err != nil {
		return nil, false, err
	}
	html_data, read_err := c.file_cache.Read(relative_path)
	if errors.Is(read_err, os.ErrNotExist) {
		return nil, false, nil
	}
	if read_err != nil {
		return nil, false, fmt.Errorf("read zhihu HTML cache: %w", read_err)
	}
	if len(html_data) == 0 {
		_ = c.file_cache.Remove(relative_path)
		return nil, false, nil
	}
	return html_data, true, nil
}

func (c *Client) write_cached_html(raw_url string, html_data []byte) error {
	if c == nil || c.file_cache == nil || !c.file_cache.Enabled() {
		return nil
	}
	relative_path, err := html_cache_relative_path(raw_url)
	if err != nil {
		return err
	}
	return c.file_cache.Write(relative_path, html_data)
}

func (c *Client) remove_cached_html(raw_url string) error {
	if c == nil || c.file_cache == nil || !c.file_cache.Enabled() {
		return nil
	}
	relative_path, err := html_cache_relative_path(raw_url)
	if err != nil {
		return err
	}
	return c.file_cache.Remove(relative_path)
}

// LookupHTMLCache locates the cached HTML for raw_url without performing a
// network request. A nil result means the response is not cached.
func LookupHTMLCache(file_cache *cache.CacheProvider, raw_url string) (*HTMLCacheFile, error) {
	if file_cache == nil || !file_cache.Enabled() {
		return nil, nil
	}
	relative_path, err := html_cache_relative_path(raw_url)
	if err != nil {
		return nil, err
	}
	cache_path, err := file_cache.Path(relative_path)
	if err != nil {
		return nil, err
	}
	file_info, stat_err := file_cache.Stat(relative_path)
	if errors.Is(stat_err, os.ErrNotExist) {
		return nil, nil
	}
	if stat_err != nil {
		return nil, fmt.Errorf("stat zhihu HTML cache: %w", stat_err)
	}
	if !file_info.Mode().IsRegular() || file_info.Size() <= 0 {
		return nil, nil
	}
	return &HTMLCacheFile{Path: cache_path, Size: file_info.Size()}, nil
}

// ClearHTMLCache removes the cached HTML response for raw_url.
func ClearHTMLCache(file_cache *cache.CacheProvider, raw_url string) (bool, error) {
	if file_cache == nil || !file_cache.Enabled() {
		return false, nil
	}
	relative_path, err := html_cache_relative_path(raw_url)
	if err != nil {
		return false, err
	}
	if _, err := file_cache.Stat(relative_path); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("stat zhihu HTML cache: %w", err)
	}
	if err := file_cache.Remove(relative_path); err != nil {
		return false, fmt.Errorf("clear zhihu HTML cache: %w", err)
	}
	return true, nil
}
