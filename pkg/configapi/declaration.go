package configapi

import (
	"errors"
	"strings"
	"sync"
)

var ErrNilProvider = errors.New("configapi: provider is nil")

// Declaration lists the configuration namespaces owned or consumed by one
// module. It is immutable after construction.
type Declaration struct {
	namespaces []string
}

// Declare creates a module configuration declaration. Empty and duplicate
// namespaces are ignored while preserving the declaration order.
func Declare(namespaces ...string) Declaration {
	declared := make([]string, 0, len(namespaces))
	seen := make(map[string]struct{}, len(namespaces))
	for _, namespace := range namespaces {
		namespace = normalize_namespace(namespace)
		if namespace == "" {
			continue
		}
		if _, ok := seen[namespace]; ok {
			continue
		}
		seen[namespace] = struct{}{}
		declared = append(declared, namespace)
	}
	return Declaration{namespaces: declared}
}

// Namespaces returns a copy of the namespaces declared by the module.
func (d Declaration) Namespaces() []string {
	return append([]string(nil), d.namespaces...)
}

// Snapshot returns the latest snapshot for one declared namespace. Access to
// undeclared namespaces is rejected so a module's configuration dependency is
// explicit and reviewable.
func (d Declaration) Snapshot(provider Provider, namespace string) (Snapshot, error) {
	if provider == nil {
		return Snapshot{}, ErrNilProvider
	}
	namespace = normalize_namespace(namespace)
	if !d.includes(namespace) {
		return Snapshot{}, errors.New("configapi: namespace " + namespace + " is not declared")
	}
	return provider.Snapshot(namespace), nil
}

// Decode decodes one declared namespace into a module-owned typed config.
func (d Declaration) Decode(provider Provider, namespace string, target any) error {
	snapshot, err := d.Snapshot(provider, namespace)
	if err != nil {
		return err
	}
	return snapshot.Decode(target)
}

// Subscribe watches all declared namespaces. A Store publication has one
// revision but emits one callback per namespace; callbacks with the same
// revision are coalesced so the module applies each publication once.
func (d Declaration) Subscribe(provider Provider, handler func(uint64)) (func(), error) {
	if provider == nil {
		return nil, ErrNilProvider
	}
	if handler == nil || len(d.namespaces) == 0 {
		return func() {}, nil
	}

	var callback_mu sync.Mutex
	var applied_revision uint64
	unsubscribes := make([]func(), 0, len(d.namespaces))
	for _, namespace := range d.namespaces {
		unsubscribe := provider.Subscribe(namespace, func(snapshot Snapshot) {
			revision := snapshot.Revision()
			callback_mu.Lock()
			defer callback_mu.Unlock()
			if revision != 0 && revision <= applied_revision {
				return
			}
			if revision != 0 {
				applied_revision = revision
			}
			handler(revision)
		})
		unsubscribes = append(unsubscribes, unsubscribe)
	}

	var unsubscribe_once sync.Once
	return func() {
		unsubscribe_once.Do(func() {
			for _, unsubscribe := range unsubscribes {
				unsubscribe()
			}
		})
	}, nil
}

func (d Declaration) includes(namespace string) bool {
	for _, declared := range d.namespaces {
		if strings.EqualFold(declared, namespace) {
			return true
		}
	}
	return false
}
