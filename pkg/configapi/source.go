package configapi

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

const (
	PrioritySystemFile  = 100
	PriorityUserFile    = 200
	PriorityDatabase    = 300
	PriorityRuntime     = 350
	PriorityEnvironment = 400
	PriorityValues      = 500
	PriorityCLI         = PriorityValues
)

// Source provides one configuration layer. Higher priority sources override
// lower priority sources recursively.
type Source interface {
	Name() string
	Priority() int
	Load(ctx context.Context) (map[string]any, error)
}

// MutableSource can atomically replace its complete layer after a candidate
// configuration has passed Manager validation.
type MutableSource interface {
	Source
	Store(ctx context.Context, values map[string]any) error
}

// WriteCapability lets a source that has a Store method expose a runtime
// read-only mode. Mutable sources without this optional interface are writable.
type WriteCapability interface {
	Writable() bool
}

type source_entry struct {
	source Source
	order  uint64
}

type SourceInfo struct {
	Name         string `json:"name"`
	Priority     int    `json:"priority"`
	Writable     bool   `json:"writable"`
	DefaultWrite bool   `json:"defaultWrite,omitempty"`
}

// MemorySource is a concurrency-safe mutable source suitable for runtime and
// GUI configuration overrides.
type MemorySource struct {
	name     string
	priority int
	mu       sync.RWMutex
	values   map[string]any
}

func NewMemorySource(name string, priority int, values map[string]any) (*MemorySource, error) {
	name = normalize_source_name(name)
	if name == "" {
		return nil, errors.New("configapi: source name is empty")
	}
	cloned, err := clone_values(values)
	if err != nil {
		return nil, err
	}
	return &MemorySource{name: name, priority: priority, values: cloned}, nil
}

func MustMemorySource(name string, priority int, values map[string]any) *MemorySource {
	source, err := NewMemorySource(name, priority, values)
	if err != nil {
		panic(err)
	}
	return source
}

func (s *MemorySource) Name() string { return s.name }

func (s *MemorySource) Priority() int { return s.priority }

func (s *MemorySource) Load(context.Context) (map[string]any, error) {
	if s == nil {
		return nil, errors.New("configapi: memory source is nil")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return clone_values(s.values)
}

func (s *MemorySource) Store(_ context.Context, values map[string]any) error {
	if s == nil {
		return errors.New("configapi: memory source is nil")
	}
	cloned, err := clone_values(values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.values = cloned
	s.mu.Unlock()
	return nil
}

// SetValue updates one dotted key in the source without publishing it. The
// owning Manager controls transactional validation and publication.
func (s *MemorySource) SetValue(key string, value any) error {
	if s == nil {
		return errors.New("configapi: memory source is nil")
	}
	s.mu.Lock()
	next, err := clone_values(s.values)
	if err == nil {
		set_path(next, key, value)
		s.values = next
	}
	s.mu.Unlock()
	return err
}

func (s *MemorySource) DeleteValue(key string) error {
	if s == nil {
		return errors.New("configapi: memory source is nil")
	}
	s.mu.Lock()
	next, err := clone_values(s.values)
	if err == nil {
		delete_path(next, key)
		s.values = next
	}
	s.mu.Unlock()
	return err
}

// StaticSource is an immutable configuration layer, typically used for CLI or
// environment overrides.
type StaticSource struct {
	name     string
	priority int
	values   map[string]any
}

func NewStaticSource(name string, priority int, values map[string]any) (*StaticSource, error) {
	name = normalize_source_name(name)
	if name == "" {
		return nil, errors.New("configapi: source name is empty")
	}
	cloned, err := clone_values(values)
	if err != nil {
		return nil, err
	}
	return &StaticSource{name: name, priority: priority, values: cloned}, nil
}

func (s *StaticSource) Name() string { return s.name }

func (s *StaticSource) Priority() int { return s.priority }

func (s *StaticSource) Load(context.Context) (map[string]any, error) {
	if s == nil {
		return nil, errors.New("configapi: static source is nil")
	}
	return clone_values(s.values)
}

// NewValuesSource expands dotted keys into a regular configuration tree.
// It represents explicit caller-provided overrides without coupling the
// source to a particular input mechanism.
func NewValuesSource(values map[string]any) (*StaticSource, error) {
	return new_dotted_static_source("values", PriorityValues, values)
}

// NewCLISource expands dotted flag keys into a regular configuration tree.
func NewCLISource(values map[string]any) (*StaticSource, error) {
	return new_dotted_static_source("cli", PriorityCLI, values)
}

func new_dotted_static_source(name string, priority int, values map[string]any) (*StaticSource, error) {
	nested := make(map[string]any)
	for key, value := range values {
		set_path(nested, key, value)
	}
	return NewStaticSource(name, priority, nested)
}

// Backend is the storage-neutral contract used by DatabaseSource. An internal
// application package can implement it with GORM, SQL, BoltDB, or a remote KV.
type Backend interface {
	LoadConfig(ctx context.Context) (map[string]any, error)
	SaveConfig(ctx context.Context, values map[string]any) error
}

type DatabaseSource struct {
	name     string
	priority int
	backend  Backend
}

func NewDatabaseSource(name string, priority int, backend Backend) (*DatabaseSource, error) {
	name = normalize_source_name(name)
	if name == "" {
		return nil, errors.New("configapi: source name is empty")
	}
	if backend == nil {
		return nil, errors.New("configapi: database backend is nil")
	}
	return &DatabaseSource{name: name, priority: priority, backend: backend}, nil
}

func (s *DatabaseSource) Name() string { return s.name }

func (s *DatabaseSource) Priority() int { return s.priority }

func (s *DatabaseSource) Load(ctx context.Context) (map[string]any, error) {
	if s == nil || s.backend == nil {
		return nil, errors.New("configapi: database source is nil")
	}
	values, err := s.backend.LoadConfig(ctx)
	if err != nil {
		return nil, err
	}
	return clone_values(values)
}

func (s *DatabaseSource) Store(ctx context.Context, values map[string]any) error {
	if s == nil || s.backend == nil {
		return errors.New("configapi: database source is nil")
	}
	cloned, err := clone_values(values)
	if err != nil {
		return err
	}
	return s.backend.SaveConfig(ctx, cloned)
}

func normalize_source_name(name string) string {
	return normalize_key(name)
}

func validate_source(source Source) error {
	if source == nil {
		return errors.New("configapi: source is nil")
	}
	if normalize_source_name(source.Name()) == "" {
		return errors.New("configapi: source name is empty")
	}
	if normalize_source_name(source.Name()) == default_source_name {
		return fmt.Errorf("configapi: source name %q is reserved", default_source_name)
	}
	return nil
}

func source_mutable(source Source) (MutableSource, bool) {
	mutable, ok := source.(MutableSource)
	if !ok {
		return nil, false
	}
	if capability, declared := source.(WriteCapability); declared && !capability.Writable() {
		return nil, false
	}
	return mutable, true
}
