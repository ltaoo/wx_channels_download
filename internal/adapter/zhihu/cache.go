package zhihuadapter

import (
	"strings"

	"wx_channel/internal/adapter"
	"wx_channel/pkg/cache"
	"wx_channel/pkg/scraper/zhihu"
)

// FetchCacheEntries returns the HTML cache produced for one Zhihu page fetch.
func (h *handler) FetchCacheEntries(raw_url string, _ any) ([]adapter.FetchCacheEntry, error) {
	cache_file, err := zhihu.LookupHTMLCache(h.runtime_file_cache(), strings.TrimSpace(raw_url))
	if err != nil || cache_file == nil {
		return nil, err
	}
	return []adapter.FetchCacheEntry{{
		Key:  "page-html",
		Name: "页面 HTML",
		URL:  strings.TrimSpace(raw_url),
		Path: cache_file.Path,
		Size: cache_file.Size,
	}}, nil
}

// ClearFetchCache removes the HTML cache produced for one Zhihu page URL.
func (h *handler) ClearFetchCache(raw_url string) (bool, error) {
	return zhihu.ClearHTMLCache(h.runtime_file_cache(), strings.TrimSpace(raw_url))
}

func (h *handler) runtime_file_cache() *cache.CacheProvider {
	if h == nil {
		return nil
	}
	h.runtime_mu.RLock()
	file_cache := h.file_cache
	h.runtime_mu.RUnlock()
	return file_cache
}
