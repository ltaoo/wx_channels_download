package xdownloader

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
)

type Registry struct {
	mu          sync.RWMutex
	downloaders []Downloader
}

func NewRegistry(downloaders ...Downloader) *Registry {
	r := &Registry{}
	for _, downloader := range downloaders {
		r.Register(downloader)
	}
	return r
}

func NewDefaultRegistry(client *http.Client) *Registry {
	return NewRegistry(NewHTTPDownloader(client))
}

func (r *Registry) Register(downloader Downloader) {
	if downloader == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.downloaders = append(r.downloaders, downloader)
}

func (r *Registry) Select(req Request) (Downloader, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, downloader := range r.downloaders {
		if downloader.CanDownload(req) {
			return downloader, nil
		}
	}
	return nil, ErrNoDownloader
}

func (r *Registry) Downloaders() []Downloader {
	r.mu.RLock()
	defer r.mu.RUnlock()
	downloaders := make([]Downloader, len(r.downloaders))
	copy(downloaders, r.downloaders)
	return downloaders
}

func requestProtocol(req Request) Protocol {
	if req.Protocol != "" {
		return Protocol(strings.ToLower(string(req.Protocol)))
	}
	parsed, err := url.Parse(req.URL)
	if err != nil {
		return ""
	}
	return Protocol(strings.ToLower(parsed.Scheme))
}
