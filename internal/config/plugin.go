package config

import (
	"fmt"
	"strings"
	"sync"

	"wx_channel/pkg/configapi"
)

// Configurable is the interface each plugin/module implements to declare its own
// configuration schema and consume its config values from the unified config.yaml.
//
// Usage:
//  1. Implement Configurable on a plugin struct.
//  2. Call RegisterPlugin() in the plugin's init().
//  3. The host registers every plugin schema before Manager refresh, then calls
//     ApplyConfig with a ScopedConfig limited to that plugin's namespace.
type Configurable interface {
	// ConfigNamespace returns the top-level key in config.yaml for this plugin,
	// e.g. "channels", "69shuba", "bilibili".
	ConfigNamespace() string

	// ConfigSchema returns configapi item definitions for this plugin.
	// Keys are relative to the namespace (e.g. "disableLocationToHome", not
	// "channels.disableLocationToHome"). The loader auto-prepends the namespace.
	ConfigSchema() []configapi.Item

	// ApplyConfig is called after all sources are loaded, with a ScopedConfig scoped
	// to the plugin's namespace. The plugin reads its config values from sub
	// and populates its own struct fields.
	ApplyConfig(config *ScopedConfig) error
}

type configurable_aliases interface {
	// ConfigAliases returns compatibility keys that are already fully qualified.
	ConfigAliases() []configapi.Item
}

var (
	plugins_mu sync.RWMutex
	plugins    []Configurable
)

// RegisterPlugin registers a plugin for config schema declaration and loading.
// Call this in the plugin's init() function. Duplicate namespaces are rejected.
func RegisterPlugin(p Configurable) {
	plugins_mu.Lock()
	defer plugins_mu.Unlock()

	ns := p.ConfigNamespace()
	for _, existing := range plugins {
		if existing.ConfigNamespace() == ns {
			panic(fmt.Sprintf("config: duplicate plugin namespace %q", ns))
		}
	}
	plugins = append(plugins, p)
}

// register_plugin_schemas attaches every adapter-owned schema directly to the
// manager. Prefixing prevents one adapter from owning another's namespace.
func register_plugin_schemas(manager *configapi.Manager) ([]*configapi.ModuleHandle, error) {
	plugins_mu.RLock()
	defer plugins_mu.RUnlock()

	handles := make([]*configapi.ModuleHandle, 0, len(plugins))
	for _, p := range plugins {
		ns := p.ConfigNamespace()
		schema := p.ConfigSchema()
		items := make([]configapi.Item, 0, len(schema))
		for _, item := range schema {
			prefixed := item
			prefixed.Key = ns + "." + item.Key
			prefixed.Sensitive = prefixed.Sensitive || sensitive_config_key(prefixed.Key)
			items = append(items, prefixed)
		}
		if aliases, ok := p.(configurable_aliases); ok {
			for _, item := range aliases.ConfigAliases() {
				item.Sensitive = item.Sensitive || sensitive_config_key(item.Key)
				items = append(items, item)
			}
		}
		handle, err := manager.RegisterModule(
			configapi.DeclareModule("adapter."+ns, items...),
		)
		if err != nil {
			for index := len(handles) - 1; index >= 0; index-- {
				handles[index].Unregister()
			}
			return nil, fmt.Errorf("config: register plugin %q schema: %w", ns, err)
		}
		handles = append(handles, handle)
	}
	return handles, nil
}

// ApplyPluginConfigs initializes legacy adapter configuration structs from
// namespace-scoped immutable snapshots. Runtime adapters should subscribe to
// configapi.Provider directly for subsequent hot updates.
func ApplyPluginConfigs(provider configapi.Provider) error {
	plugins_mu.RLock()
	defer plugins_mu.RUnlock()

	for _, plugin := range plugins {
		namespace := plugin.ConfigNamespace()
		if err := plugin.ApplyConfig(NewScopedConfig(provider.Snapshot(namespace))); err != nil {
			return fmt.Errorf("config: plugin %q ApplyConfig failed: %w", namespace, err)
		}
	}
	return nil
}

func sensitive_config_key(key string) bool {
	key = strings.ToLower(key)
	for _, marker := range []string{"password", "token", "cookie", "secret", "privatekey", "apikey"} {
		if strings.Contains(key, marker) {
			return true
		}
	}
	return false
}
