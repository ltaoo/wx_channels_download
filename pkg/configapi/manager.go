package configapi

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

const default_source_name = "default"

var (
	ErrRevisionConflict = errors.New("configapi: revision conflict")
	ErrUnknownItem      = errors.New("configapi: unknown configuration item")
	ErrReadonlyItem     = errors.New("configapi: configuration item is read-only")
	ErrNoWritableSource = errors.New("configapi: no writable source")
)

type ValidationError struct {
	Key string `json:"key"`
	Err error  `json:"-"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("configapi: invalid value for %s: %v", e.Key, e.Err)
}

func (e *ValidationError) Unwrap() error { return e.Err }

type Change struct {
	Key          string       `json:"key"`
	OldValue     any          `json:"oldValue,omitempty"`
	NewValue     any          `json:"newValue,omitempty"`
	OldSource    string       `json:"oldSource,omitempty"`
	NewSource    string       `json:"newSource,omitempty"`
	ReloadPolicy ReloadPolicy `json:"reloadPolicy,omitempty"`
}

type UpdateRequest struct {
	Values           map[string]any `json:"values,omitempty"`
	Delete           []string       `json:"delete,omitempty"`
	TargetSource     string         `json:"targetSource,omitempty"`
	ExpectedRevision uint64         `json:"expectedRevision,omitempty"`
}

type UpdateResult struct {
	Revision uint64   `json:"revision"`
	Changes  []Change `json:"changes"`
}

type Entry struct {
	Item       Item   `json:"item"`
	Value      any    `json:"value,omitempty"`
	Source     string `json:"source,omitempty"`
	Writable   bool   `json:"writable"`
	Overridden bool   `json:"overridden,omitempty"`
}

type View struct {
	Revision uint64       `json:"revision"`
	Items    []Entry      `json:"items"`
	Sources  []SourceInfo `json:"sources"`
}

// Controller is the complete runtime configuration contract used by GUI and
// management layers.
type Controller interface {
	Provider
	Revision() uint64
	Schema() []Item
	View(redact_sensitive bool) View
	Apply(ctx context.Context, request UpdateRequest) (UpdateResult, error)
	Refresh(ctx context.Context) (UpdateResult, error)
}

// Manager combines ordered sources, module-owned schemas, validation,
// provenance, transactional updates, and immutable runtime publication.
type Manager struct {
	operation_mu sync.Mutex
	mu           sync.RWMutex
	store        *Store
	sources      map[string]source_entry
	next_order   uint64
	modules      map[string]map[string]struct{}
	item_owner   map[string]string
	items        map[string]Item
	effective    map[string]any
	provenance   map[string]string
	last_changes []Change
	write_source string
}

func NewManager() *Manager {
	return &Manager{
		store:      NewStore(),
		sources:    make(map[string]source_entry),
		modules:    make(map[string]map[string]struct{}),
		item_owner: make(map[string]string),
		items:      make(map[string]Item),
		effective:  make(map[string]any),
		provenance: make(map[string]string),
	}
}

func (m *Manager) AddSource(source Source) error {
	if m == nil {
		return errors.New("configapi: manager is nil")
	}
	if err := validate_source(source); err != nil {
		return err
	}
	name := normalize_source_name(source.Name())
	m.operation_mu.Lock()
	defer m.operation_mu.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.sources[name]; exists {
		return fmt.Errorf("configapi: duplicate source %q", name)
	}
	m.next_order++
	m.sources[name] = source_entry{source: source, order: m.next_order}
	return nil
}

func (m *Manager) RemoveSource(ctx context.Context, name string) (UpdateResult, error) {
	if m == nil {
		return UpdateResult{}, errors.New("configapi: manager is nil")
	}
	name = normalize_source_name(name)
	m.operation_mu.Lock()
	defer m.operation_mu.Unlock()
	m.mu.Lock()
	if _, exists := m.sources[name]; !exists {
		m.mu.Unlock()
		return UpdateResult{}, fmt.Errorf("configapi: source %q not found", name)
	}
	delete(m.sources, name)
	if m.write_source == name {
		m.write_source = ""
	}
	m.mu.Unlock()
	return m.refresh_locked(ctx, nil)
}

func (m *Manager) SetDefaultWriteSource(name string) error {
	if m == nil {
		return errors.New("configapi: manager is nil")
	}
	name = normalize_source_name(name)
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, exists := m.sources[name]
	if !exists {
		return fmt.Errorf("configapi: source %q not found", name)
	}
	if _, ok := source_mutable(entry.source); !ok {
		return fmt.Errorf("configapi: source %q is read-only", name)
	}
	m.write_source = name
	return nil
}

func (m *Manager) RegisterModule(declaration ModuleDeclaration) (*ModuleHandle, error) {
	if m == nil {
		return nil, errors.New("configapi: manager is nil")
	}
	module_name := strings.TrimSpace(declaration.Name)
	if module_name == "" {
		return nil, errors.New("configapi: module name is empty")
	}
	normalized_items := make([]Item, 0, len(declaration.Items))
	seen := make(map[string]struct{}, len(declaration.Items))
	for _, item := range declaration.Items {
		normalized, err := item.normalized()
		if err != nil {
			return nil, err
		}
		key_lower := strings.ToLower(normalized.Key)
		if _, exists := seen[key_lower]; exists {
			return nil, fmt.Errorf("configapi: duplicate schema item %q in module %s", normalized.Key, module_name)
		}
		seen[key_lower] = struct{}{}
		normalized_items = append(normalized_items, normalized)
	}

	m.operation_mu.Lock()
	defer m.operation_mu.Unlock()
	m.mu.Lock()
	if _, exists := m.modules[module_name]; exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("configapi: duplicate module %q", module_name)
	}
	owned := make(map[string]struct{}, len(normalized_items))
	for _, item := range normalized_items {
		key_lower := strings.ToLower(item.Key)
		if owner, exists := m.item_owner[key_lower]; exists {
			m.mu.Unlock()
			return nil, fmt.Errorf("configapi: item %q already owned by module %s", item.Key, owner)
		}
		owned[key_lower] = struct{}{}
	}
	for _, item := range normalized_items {
		key_lower := strings.ToLower(item.Key)
		m.items[key_lower] = item
		m.item_owner[key_lower] = module_name
	}
	m.modules[module_name] = owned
	m.mu.Unlock()

	handle := &ModuleHandle{}
	handle.unregister = func() {
		m.operation_mu.Lock()
		m.mu.Lock()
		owned_keys, exists := m.modules[module_name]
		if exists {
			for key := range owned_keys {
				delete(m.items, key)
				delete(m.item_owner, key)
			}
			delete(m.modules, module_name)
		}
		m.mu.Unlock()
		if exists {
			_, _ = m.refresh_locked(context.Background(), nil)
		}
		m.operation_mu.Unlock()
	}
	return handle, nil
}

func (m *Manager) Refresh(ctx context.Context) (UpdateResult, error) {
	if m == nil {
		return UpdateResult{}, errors.New("configapi: manager is nil")
	}
	m.operation_mu.Lock()
	defer m.operation_mu.Unlock()
	return m.refresh_locked(ctx, nil)
}

func (m *Manager) Apply(ctx context.Context, request UpdateRequest) (UpdateResult, error) {
	if m == nil {
		return UpdateResult{}, errors.New("configapi: manager is nil")
	}
	m.operation_mu.Lock()
	defer m.operation_mu.Unlock()

	m.mu.RLock()
	current_revision := m.store.Snapshot("").Revision()
	items := make(map[string]Item, len(m.items))
	for key, item := range m.items {
		items[key] = item
	}
	source_name := normalize_source_name(request.TargetSource)
	if source_name == "" {
		source_name = m.write_source
	}
	entry, exists := m.sources[source_name]
	m.mu.RUnlock()
	if request.ExpectedRevision != 0 && request.ExpectedRevision != current_revision {
		return UpdateResult{}, fmt.Errorf("%w: current=%d expected=%d", ErrRevisionConflict, current_revision, request.ExpectedRevision)
	}
	if source_name == "" {
		return UpdateResult{}, ErrNoWritableSource
	}
	if !exists {
		return UpdateResult{}, fmt.Errorf("configapi: source %q not found", source_name)
	}
	mutable, ok := source_mutable(entry.source)
	if !ok {
		return UpdateResult{}, fmt.Errorf("configapi: source %q is read-only", source_name)
	}
	for key := range request.Values {
		item, exists := items[strings.ToLower(normalize_key(key))]
		if !exists {
			return UpdateResult{}, fmt.Errorf("%w: %s", ErrUnknownItem, key)
		}
		if item.Readonly {
			return UpdateResult{}, fmt.Errorf("%w: %s", ErrReadonlyItem, key)
		}
	}
	for _, key := range request.Delete {
		item, exists := items[strings.ToLower(normalize_key(key))]
		if !exists {
			return UpdateResult{}, fmt.Errorf("%w: %s", ErrUnknownItem, key)
		}
		if item.Readonly {
			return UpdateResult{}, fmt.Errorf("%w: %s", ErrReadonlyItem, key)
		}
	}

	current_layer, err := mutable.Load(ctx)
	if err != nil {
		return UpdateResult{}, fmt.Errorf("configapi: load source %s: %w", source_name, err)
	}
	next_layer, err := clone_values(current_layer)
	if err != nil {
		return UpdateResult{}, err
	}
	for key, value := range request.Values {
		item := items[strings.ToLower(normalize_key(key))]
		if item.Sensitive && value == "********" {
			continue
		}
		set_path(next_layer, key, value)
	}
	for _, key := range request.Delete {
		delete_path(next_layer, key)
	}

	candidate, provenance, err := m.build_candidate(ctx, map[string]map[string]any{source_name: next_layer})
	if err != nil {
		return UpdateResult{}, err
	}
	if err := mutable.Store(ctx, next_layer); err != nil {
		return UpdateResult{}, fmt.Errorf("configapi: store source %s: %w", source_name, err)
	}
	return m.publish_candidate(candidate, provenance)
}

func (m *Manager) refresh_locked(ctx context.Context, overrides map[string]map[string]any) (UpdateResult, error) {
	candidate, provenance, err := m.build_candidate(ctx, overrides)
	if err != nil {
		return UpdateResult{}, err
	}
	return m.publish_candidate(candidate, provenance)
}

func (m *Manager) build_candidate(ctx context.Context, overrides map[string]map[string]any) (map[string]any, map[string]string, error) {
	m.mu.RLock()
	items := make(map[string]Item, len(m.items))
	for key, item := range m.items {
		items[key] = item
	}
	sources := make([]source_entry, 0, len(m.sources))
	for _, entry := range m.sources {
		sources = append(sources, entry)
	}
	m.mu.RUnlock()
	sort.SliceStable(sources, func(i, j int) bool {
		if sources[i].source.Priority() != sources[j].source.Priority() {
			return sources[i].source.Priority() < sources[j].source.Priority()
		}
		return sources[i].order < sources[j].order
	})

	candidate := make(map[string]any)
	provenance := make(map[string]string)
	item_keys := make([]string, 0, len(items))
	for key := range items {
		item_keys = append(item_keys, key)
	}
	sort.Strings(item_keys)
	for _, key := range item_keys {
		item := items[key]
		if item.Default == nil {
			continue
		}
		default_value := normalize_json_value(item.Default)
		set_path(candidate, item.Key, default_value)
		provenance[normalize_key(item.Key)] = default_source_name
	}
	for _, entry := range sources {
		name := normalize_source_name(entry.source.Name())
		values, overridden := overrides[name]
		if !overridden {
			var err error
			values, err = entry.source.Load(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("configapi: load source %s: %w", name, err)
			}
		}
		cloned, err := clone_values(values)
		if err != nil {
			return nil, nil, fmt.Errorf("configapi: clone source %s: %w", name, err)
		}
		merge_values(candidate, cloned, name, provenance, "")
	}
	for _, key := range item_keys {
		item := items[key]
		value, exists := lookup_path(candidate, item.Key)
		if !exists {
			if item.Required {
				return nil, nil, &ValidationError{Key: item.Key, Err: errors.New("value is required")}
			}
			continue
		}
		coerced, err := coerce_value(item.Type, value)
		if err != nil {
			return nil, nil, &ValidationError{Key: item.Key, Err: err}
		}
		if err := item.validate(coerced); err != nil {
			return nil, nil, &ValidationError{Key: item.Key, Err: err}
		}
		set_path(candidate, item.Key, coerced)
	}
	cloned, err := clone_values(candidate)
	if err != nil {
		return nil, nil, err
	}
	return cloned, provenance, nil
}

func (m *Manager) publish_candidate(candidate map[string]any, provenance map[string]string) (UpdateResult, error) {
	m.mu.RLock()
	current := m.effective
	current_provenance := m.provenance
	items := make(map[string]Item, len(m.items))
	for key, item := range m.items {
		items[key] = item
	}
	m.mu.RUnlock()
	changes := build_changes(current, candidate, current_provenance, provenance, items)
	if len(changes) == 0 {
		return UpdateResult{Revision: m.Revision()}, nil
	}
	effective_clone, err := clone_values(candidate)
	if err != nil {
		return UpdateResult{}, err
	}
	provenance_clone := clone_string_map(provenance)
	changes_clone := clone_changes(changes)
	err = m.store.publish(candidate, func(revision uint64) {
		m.mu.Lock()
		m.effective = effective_clone
		m.provenance = provenance_clone
		m.last_changes = changes_clone
		m.mu.Unlock()
	})
	if err != nil {
		return UpdateResult{}, err
	}
	return UpdateResult{Revision: m.Revision(), Changes: changes}, nil
}

func (m *Manager) Snapshot(namespace string) Snapshot {
	if m == nil {
		return Snapshot{namespace: normalize_namespace(namespace), values: make(map[string]any)}
	}
	return m.store.Snapshot(namespace)
}

func (m *Manager) Subscribe(namespace string, handler Handler) func() {
	if m == nil {
		return func() {}
	}
	return m.store.Subscribe(namespace, handler)
}

func (m *Manager) Revision() uint64 {
	if m == nil {
		return 0
	}
	return m.store.Snapshot("").Revision()
}

func (m *Manager) Schema() []Item {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return schema_items_sorted(m.items)
}

func (m *Manager) Sources() []SourceInfo {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	result := make([]SourceInfo, 0, len(m.sources)+1)
	result = append(result, SourceInfo{Name: default_source_name, Priority: 0, Writable: false})
	for _, entry := range m.sources {
		_, writable := source_mutable(entry.source)
		result = append(result, SourceInfo{
			Name:         entry.source.Name(),
			Priority:     entry.source.Priority(),
			Writable:     writable,
			DefaultWrite: normalize_source_name(entry.source.Name()) == m.write_source,
		})
	}
	m.mu.RUnlock()
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority < result[j].Priority
		}
		return result[i].Name < result[j].Name
	})
	return result
}

func (m *Manager) View(redact_sensitive bool) View {
	if m == nil {
		return View{}
	}
	m.mu.RLock()
	items := schema_items_sorted(m.items)
	effective := m.effective
	provenance := clone_string_map(m.provenance)
	sources := make(map[string]source_entry, len(m.sources))
	for name, entry := range m.sources {
		sources[name] = entry
	}
	write_source := m.write_source
	m.mu.RUnlock()
	entries := make([]Entry, 0, len(items))
	for _, item := range items {
		value, _ := lookup_path(effective, item.Key)
		if item.Sensitive && redact_sensitive {
			if value != nil && fmt.Sprint(value) != "" {
				value = "********"
			}
			if item.Default != nil && fmt.Sprint(item.Default) != "" {
				item.Default = "********"
			}
		} else {
			value = normalize_json_value(value)
		}
		source_name := lookup_string_map(provenance, item.Key)
		writable := !item.Readonly
		if source_name == "" || source_name == default_source_name {
			entry, exists := sources[write_source]
			if !exists {
				writable = false
			} else {
				_, source_writable := source_mutable(entry.source)
				writable = writable && source_writable
			}
		} else {
			if entry, exists := sources[source_name]; exists {
				_, source_writable := source_mutable(entry.source)
				writable = writable && source_writable
			} else {
				writable = false
			}
		}
		entries = append(entries, Entry{
			Item:       item,
			Value:      value,
			Source:     source_name,
			Writable:   writable,
			Overridden: source_name != "" && source_name != default_source_name,
		})
	}
	return View{Revision: m.Revision(), Items: entries, Sources: m.Sources()}
}

func (m *Manager) LastChanges() []Change {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return clone_changes(m.last_changes)
}

func (m *Manager) Value(key string) (any, bool) {
	if m == nil {
		return nil, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, exists := lookup_path(m.effective, key)
	if !exists {
		return nil, false
	}
	return normalize_json_value(value), true
}

func build_changes(old_values, new_values map[string]any, old_provenance, new_provenance map[string]string, items map[string]Item) []Change {
	old_flat := flatten_values(old_values)
	new_flat := flatten_values(new_values)
	keys := make(map[string]struct{}, len(old_flat)+len(new_flat))
	for key := range old_flat {
		keys[strings.ToLower(key)] = struct{}{}
	}
	for key := range new_flat {
		keys[strings.ToLower(key)] = struct{}{}
	}
	ordered_keys := make([]string, 0, len(keys))
	for key := range keys {
		ordered_keys = append(ordered_keys, key)
	}
	sort.Strings(ordered_keys)
	changes := make([]Change, 0)
	for _, key := range ordered_keys {
		old_value, old_exists := lookup_flat_value(old_flat, key)
		new_value, new_exists := lookup_flat_value(new_flat, key)
		old_source := lookup_string_map(old_provenance, key)
		new_source := lookup_string_map(new_provenance, key)
		if old_exists == new_exists && equal_values(old_value, new_value) && old_source == new_source {
			continue
		}
		policy := ReloadProcess
		item, declared := items[strings.ToLower(key)]
		if declared {
			policy = item.Reload
		}
		old_change_value := normalize_json_value(old_value)
		new_change_value := normalize_json_value(new_value)
		if declared && item.Sensitive {
			if old_exists && old_value != nil && fmt.Sprint(old_value) != "" {
				old_change_value = "********"
			}
			if new_exists && new_value != nil && fmt.Sprint(new_value) != "" {
				new_change_value = "********"
			}
		}
		changes = append(changes, Change{
			Key:          key,
			OldValue:     old_change_value,
			NewValue:     new_change_value,
			OldSource:    old_source,
			NewSource:    new_source,
			ReloadPolicy: policy,
		})
	}
	return changes
}

func lookup_flat_value(values map[string]any, key string) (any, bool) {
	for candidate, value := range values {
		if strings.EqualFold(candidate, key) {
			return value, true
		}
	}
	return nil, false
}

func lookup_string_map(values map[string]string, key string) string {
	for candidate, value := range values {
		if strings.EqualFold(candidate, key) {
			return value
		}
	}
	return ""
}

func clone_string_map(values map[string]string) map[string]string {
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func clone_changes(changes []Change) []Change {
	cloned := make([]Change, len(changes))
	copy(cloned, changes)
	for index := range cloned {
		cloned[index].OldValue = normalize_json_value(cloned[index].OldValue)
		cloned[index].NewValue = normalize_json_value(cloned[index].NewValue)
	}
	return cloned
}

var _ Controller = (*Manager)(nil)
