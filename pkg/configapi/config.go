// Package configapi defines the configuration contract shared by the host and
// runtime adapters. It intentionally does not depend on the application's
// concrete configuration loader.
package configapi

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
)

var ErrNilTarget = errors.New("configapi: decode target is nil")

// Snapshot is an immutable, versioned view of one configuration namespace.
// Values returns a clone so callers cannot mutate the published configuration.
type Snapshot struct {
	namespace string
	revision  uint64
	values    map[string]any
}

func (s Snapshot) Namespace() string {
	return s.namespace
}

func (s Snapshot) Revision() uint64 {
	return s.revision
}

func (s Snapshot) Values() map[string]any {
	values, err := clone_values(s.values)
	if err != nil {
		return map[string]any{}
	}
	return values
}

// Decode converts the snapshot into an adapter-owned typed configuration.
func (s Snapshot) Decode(target any) error {
	if target == nil {
		return ErrNilTarget
	}
	data, err := json.Marshal(s.values)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// Handler receives a complete replacement snapshot after a configuration
// publication. Handlers should return quickly and hand off expensive work.
type Handler func(Snapshot)

// Provider exposes namespace-scoped snapshots to adapters.
type Provider interface {
	Snapshot(namespace string) Snapshot
	Subscribe(namespace string, handler Handler) (unsubscribe func())
}

// Runtime contains immutable host metadata that does not come from a module's
// runtime configuration namespaces.
type Runtime struct {
	Version              string
	Mode                 string
	RootDir              string
	WorkDir              string
	GlobalScriptContent  string
	ContentScriptContent string
}

type subscription struct {
	namespace string
	handler   Handler
}

// Store is an in-memory Provider. Publish atomically replaces the complete
// effective configuration and then notifies subscribers outside the lock.
type Store struct {
	publish_mu    sync.Mutex
	mu            sync.RWMutex
	revision      uint64
	values        map[string]any
	next_sub_id   uint64
	subscriptions map[uint64]subscription
}

func NewStore() *Store {
	return &Store{
		values:        make(map[string]any),
		subscriptions: make(map[uint64]subscription),
	}
}

// Publish replaces the current effective configuration. The input is cloned
// before publication and can be safely reused or modified by the caller.
func (s *Store) Publish(values map[string]any) error {
	return s.publish(values, nil)
}

func (s *Store) publish(values map[string]any, before_notify func(revision uint64)) error {
	if s == nil {
		return errors.New("configapi: store is nil")
	}
	s.publish_mu.Lock()
	defer s.publish_mu.Unlock()
	cloned, err := clone_values(values)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.revision++
	s.values = cloned
	revision := s.revision
	subs := make([]subscription, 0, len(s.subscriptions))
	for _, sub := range s.subscriptions {
		subs = append(subs, sub)
	}
	s.mu.Unlock()
	if before_notify != nil {
		before_notify(revision)
	}

	for _, sub := range subs {
		sub.handler(snapshot_from_values(cloned, sub.namespace, revision))
	}
	return nil
}

func (s *Store) Snapshot(namespace string) Snapshot {
	if s == nil {
		return Snapshot{namespace: normalize_namespace(namespace), values: make(map[string]any)}
	}
	s.mu.RLock()
	snapshot := snapshot_from_values(s.values, namespace, s.revision)
	s.mu.RUnlock()
	return snapshot
}

func (s *Store) Subscribe(namespace string, handler Handler) func() {
	if s == nil || handler == nil {
		return func() {}
	}
	namespace = normalize_namespace(namespace)
	s.mu.Lock()
	s.next_sub_id++
	id := s.next_sub_id
	s.subscriptions[id] = subscription{namespace: namespace, handler: handler}
	s.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			s.mu.Lock()
			delete(s.subscriptions, id)
			s.mu.Unlock()
		})
	}
}

func snapshot_from_values(values map[string]any, namespace string, revision uint64) Snapshot {
	namespace = normalize_namespace(namespace)
	scoped := values
	if namespace != "" {
		value, ok := lookup_path(values, namespace)
		if !ok {
			scoped = map[string]any{}
		} else if value_map, ok := to_string_map(value); ok {
			scoped = value_map
		} else {
			scoped = map[string]any{"value": value}
		}
	}
	cloned, err := clone_values(scoped)
	if err != nil {
		cloned = map[string]any{}
	}
	return Snapshot{namespace: namespace, revision: revision, values: cloned}
}

func normalize_namespace(namespace string) string {
	return strings.Trim(strings.TrimSpace(namespace), ".")
}

func lookup_path(values map[string]any, path string) (any, bool) {
	var current any = values
	for _, part := range strings.Split(path, ".") {
		value_map, ok := to_string_map(current)
		if !ok {
			return nil, false
		}
		value, found := lookup_key(value_map, part)
		if !found {
			return nil, false
		}
		current = value
	}
	return current, true
}

func lookup_key(values map[string]any, key string) (any, bool) {
	if value, ok := values[key]; ok {
		return value, true
	}
	for candidate, value := range values {
		if strings.EqualFold(candidate, key) {
			return value, true
		}
	}
	return nil, false
}

func to_string_map(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case map[any]any:
		converted := make(map[string]any, len(typed))
		for key, item := range typed {
			key_string, ok := key.(string)
			if !ok {
				return nil, false
			}
			converted[key_string] = item
		}
		return converted, true
	default:
		return nil, false
	}
}

func clone_values(values map[string]any) (map[string]any, error) {
	if values == nil {
		return make(map[string]any), nil
	}
	data, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}
	cloned := make(map[string]any)
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil, err
	}
	return cloned, nil
}
