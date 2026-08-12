package cache

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ProviderRegistry owns the application's persistent cache root and issues
// namespace-scoped CacheProvider references.
type ProviderRegistry struct {
	root_dir     string
	namespace_mu sync.Mutex
	namespaces   map[string]*CacheProvider
}

// CacheProvider is a namespace-scoped persistent file cache. It cannot address
// files outside the namespace assigned by ProviderRegistry.
type CacheProvider struct {
	root_dir string
	file_mu  sync.RWMutex
}

// NewProviderRegistry creates a persistent cache registry rooted at
// workdir/cache.
func NewProviderRegistry(work_dir string) (*ProviderRegistry, error) {
	work_dir = strings.TrimSpace(work_dir)
	if work_dir == "" {
		return &ProviderRegistry{namespaces: make(map[string]*CacheProvider)}, nil
	}
	root_dir, err := filepath.Abs(filepath.Join(work_dir, "cache"))
	if err != nil {
		return nil, fmt.Errorf("resolve persistent cache root: %w", err)
	}
	return &ProviderRegistry{
		root_dir:   filepath.Clean(root_dir),
		namespaces: make(map[string]*CacheProvider),
	}, nil
}

// Namespace returns the isolated persistent file cache for name.
func (r *ProviderRegistry) Namespace(name string) (*CacheProvider, error) {
	if r == nil {
		return nil, errors.New("cache provider registry is nil")
	}
	name = strings.TrimSpace(name)
	if !valid_namespace(name) {
		return nil, fmt.Errorf("invalid cache namespace %q", name)
	}

	r.namespace_mu.Lock()
	defer r.namespace_mu.Unlock()
	if cache_provider := r.namespaces[name]; cache_provider != nil {
		return cache_provider, nil
	}
	root_dir := ""
	if r.root_dir != "" {
		root_dir = filepath.Join(r.root_dir, name)
	}
	cache_provider := &CacheProvider{root_dir: root_dir}
	r.namespaces[name] = cache_provider
	return cache_provider, nil
}

// Enabled reports whether this namespace has a persistent workdir.
func (c *CacheProvider) Enabled() bool {
	return c != nil && c.root_dir != ""
}

// Path returns the absolute path for a namespace-relative cache file. Paths
// use slash-separated components and may not escape the namespace.
func (c *CacheProvider) Path(relative_path string) (string, error) {
	if c == nil {
		return "", errors.New("file cache is nil")
	}
	if c.root_dir == "" {
		return "", nil
	}
	clean_path, err := clean_relative_path(relative_path)
	if err != nil {
		return "", err
	}
	return filepath.Join(c.root_dir, filepath.FromSlash(clean_path)), nil
}

// Read reads one namespace-relative cache file.
func (c *CacheProvider) Read(relative_path string) ([]byte, error) {
	file_path, err := c.Path(relative_path)
	if err != nil || file_path == "" {
		return nil, err
	}
	c.file_mu.RLock()
	data, read_err := os.ReadFile(file_path)
	c.file_mu.RUnlock()
	return data, read_err
}

// Stat returns metadata for one namespace-relative cache path.
func (c *CacheProvider) Stat(relative_path string) (os.FileInfo, error) {
	file_path, err := c.Path(relative_path)
	if err != nil || file_path == "" {
		if err == nil {
			err = os.ErrNotExist
		}
		return nil, err
	}
	c.file_mu.RLock()
	file_info, stat_err := os.Stat(file_path)
	c.file_mu.RUnlock()
	return file_info, stat_err
}

// Write atomically replaces one namespace-relative cache file.
func (c *CacheProvider) Write(relative_path string, data []byte) error {
	file_path, err := c.Path(relative_path)
	if err != nil || file_path == "" {
		return err
	}

	c.file_mu.Lock()
	defer c.file_mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(file_path), 0755); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}
	temporary_file, err := os.CreateTemp(filepath.Dir(file_path), ".cache-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary cache file: %w", err)
	}
	temporary_path := temporary_file.Name()
	defer os.Remove(temporary_path)
	if _, err := temporary_file.Write(data); err != nil {
		_ = temporary_file.Close()
		return fmt.Errorf("write temporary cache file: %w", err)
	}
	if err := temporary_file.Close(); err != nil {
		return fmt.Errorf("close temporary cache file: %w", err)
	}
	if err := os.Rename(temporary_path, file_path); err != nil {
		if remove_err := os.Remove(file_path); remove_err != nil && !errors.Is(remove_err, os.ErrNotExist) {
			return fmt.Errorf("replace cache file: %w", remove_err)
		}
		if err := os.Rename(temporary_path, file_path); err != nil {
			return fmt.Errorf("commit cache file: %w", err)
		}
	}
	return nil
}

// Remove removes one namespace-relative cache file. Missing files are ignored.
func (c *CacheProvider) Remove(relative_path string) error {
	file_path, err := c.Path(relative_path)
	if err != nil || file_path == "" {
		return err
	}
	c.file_mu.Lock()
	remove_err := os.Remove(file_path)
	c.file_mu.Unlock()
	if errors.Is(remove_err, os.ErrNotExist) {
		return nil
	}
	return remove_err
}

// RemoveAll removes a namespace-relative cache directory or file. It returns
// false when the target did not exist.
func (c *CacheProvider) RemoveAll(relative_path string) (bool, error) {
	file_path, err := c.Path(relative_path)
	if err != nil || file_path == "" {
		return false, err
	}
	c.file_mu.Lock()
	defer c.file_mu.Unlock()
	if _, err := os.Stat(file_path); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if err := os.RemoveAll(file_path); err != nil {
		return false, err
	}
	return true, nil
}

func valid_namespace(name string) bool {
	if name == "" || name == "." || !fs.ValidPath(name) {
		return false
	}
	for _, character := range name {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func clean_relative_path(relative_path string) (string, error) {
	relative_path = strings.TrimSpace(strings.ReplaceAll(relative_path, "\\", "/"))
	if relative_path == "" || relative_path == "." || !fs.ValidPath(relative_path) || strings.Contains(relative_path, ":") {
		return "", fmt.Errorf("invalid cache path %q", relative_path)
	}
	return relative_path, nil
}
