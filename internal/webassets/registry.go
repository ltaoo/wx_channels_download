// Package webassets provides a small, platform-neutral registry for embedded
// static files exposed by the local HTTP server.
package webassets

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"sync"
)

const defaultCacheControl = "no-cache"

var (
	ErrNilFS          = errors.New("static asset filesystem is nil")
	ErrInvalidPrefix  = errors.New("invalid static asset mount path")
	ErrDuplicateMount = errors.New("duplicate static asset mount path")
)

type mount struct {
	prefix   string
	assets   fs.FS
	fileName string
}

// Registry maps URL path prefixes to their owning filesystem. Registrations
// are normally completed during application setup, before requests are served.
type Registry struct {
	mu     sync.RWMutex
	mounts []mount
}

func NewRegistry() *Registry {
	return &Registry{}
}

// Register mounts assets at prefix. prefix must be an absolute, clean URL
// path, for example "/__assets/platform/wxchannels".
func (r *Registry) Register(prefix string, assets fs.FS) error {
	return r.register(prefix, assets, "")
}

// RegisterFile maps one URL path to one file within assets. It is intended for
// explicit compatibility aliases; new assets should use Register with a
// platform namespace instead.
func (r *Registry) RegisterFile(requestPath string, assets fs.FS, fileName string) error {
	if !fs.ValidPath(fileName) {
		return ErrInvalidPrefix
	}
	return r.register(requestPath, assets, fileName)
}

func (r *Registry) register(prefix string, assets fs.FS, fileName string) error {
	if assets == nil {
		return ErrNilFS
	}
	prefix = strings.TrimSuffix(prefix, "/")
	if prefix == "" || !strings.HasPrefix(prefix, "/") || path.Clean(prefix) != prefix {
		return ErrInvalidPrefix
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.mounts {
		if existing.prefix == prefix {
			return ErrDuplicateMount
		}
	}
	r.mounts = append(r.mounts, mount{prefix: prefix, assets: assets, fileName: fileName})
	return nil
}

// ServeHTTP serves a registered file, or returns 404 if no mount owns the
// request path. It supports GET and HEAD only.
func (r *Registry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		w.Header().Set("Allow", http.MethodGet+", "+http.MethodHead)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	assets, name, ok := r.lookup(req.URL.Path)
	if !ok {
		http.NotFound(w, req)
		return
	}
	data, err := fs.ReadFile(assets, name)
	if err != nil {
		http.NotFound(w, req)
		return
	}

	etag := assetETag(data)
	w.Header().Set("Content-Type", ContentType(name))
	w.Header().Set("Cache-Control", defaultCacheControl)
	w.Header().Set("ETag", etag)
	if strings.Contains(req.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if req.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (r *Registry) lookup(requestPath string) (fs.FS, string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var selected *mount
	for i := range r.mounts {
		candidate := &r.mounts[i]
		if candidate.fileName != "" {
			if requestPath != candidate.prefix {
				continue
			}
		} else if requestPath == candidate.prefix || !strings.HasPrefix(requestPath, candidate.prefix+"/") {
			continue
		}
		if selected == nil || len(candidate.prefix) > len(selected.prefix) {
			selected = candidate
		}
	}
	if selected == nil {
		return nil, "", false
	}
	name := selected.fileName
	if name == "" {
		name = strings.TrimPrefix(requestPath, selected.prefix+"/")
	}
	if !fs.ValidPath(name) {
		return nil, "", false
	}
	return selected.assets, name, true
}

func ContentType(name string) string {
	switch {
	case strings.HasSuffix(name, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(name, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(name, ".json"):
		return "application/json; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

func assetETag(data []byte) string {
	hash := sha256.Sum256(data)
	return `"` + hex.EncodeToString(hash[:]) + `"`
}
