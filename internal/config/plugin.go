package config

import (
	"fmt"
	"sync"

	"github.com/spf13/viper"
)

// Configurable is the interface each plugin/module implements to declare its own
// configuration schema and consume its config values from the unified config.yaml.
//
// Usage:
//  1. Implement Configurable on a plugin struct.
//  2. Call RegisterPlugin() in the plugin's init().
//  3. LoadPluginConfigs() is called after viper reads config.yaml — it registers
//     each plugin's schema into the global Registry and calls ApplyConfig() with
//     a SubViper scoped to the plugin's namespace.
type Configurable interface {
	// ConfigNamespace returns the top-level key in config.yaml for this plugin,
	// e.g. "channels", "69shuba", "bilibili".
	ConfigNamespace() string

	// ConfigSchema returns the config item definitions for this plugin.
	// Keys are relative to the namespace (e.g. "disableLocationToHome", not
	// "channels.disableLocationToHome"). The loader auto-prepends the namespace.
	ConfigSchema() []ConfigField

	// ApplyConfig is called after config.yaml is loaded, with a SubViper scoped
	// to the plugin's namespace. The plugin reads its config values from sub
	// and populates its own struct fields.
	ApplyConfig(sub *SubViper) error
}

var (
	pluginsMu sync.RWMutex
	plugins   []Configurable
)

// RegisterPlugin registers a plugin for config schema declaration and loading.
// Call this in the plugin's init() function. Duplicate namespaces are rejected.
func RegisterPlugin(p Configurable) {
	pluginsMu.Lock()
	defer pluginsMu.Unlock()

	ns := p.ConfigNamespace()
	for _, existing := range plugins {
		if existing.ConfigNamespace() == ns {
			panic(fmt.Sprintf("config: duplicate plugin namespace %q", ns))
		}
	}
	plugins = append(plugins, p)
}

// LoadPluginConfigs iterates over all registered plugins and:
//  1. Registers each plugin's ConfigSchema items into the global schema Registry
//     (with keys auto-prefixed by namespace).
//  2. Calls ApplyConfig() on each plugin, passing a SubViper scoped to its namespace.
//
// Must be called AFTER viper.ReadInConfig().
func LoadPluginConfigs() {
	pluginsMu.RLock()
	defer pluginsMu.RUnlock()

	for _, p := range plugins {
		ns := p.ConfigNamespace()
		// Register schema items with namespace prefix
		for _, item := range p.ConfigSchema() {
			prefixed := item
			prefixed.Key = ns + "." + item.Key
			Register(prefixed)
		}
		// Build scoped viper and apply config
		sub := viper.Sub(ns)
		if err := p.ApplyConfig(NewSubViper(sub)); err != nil {
			panic(fmt.Sprintf("config: plugin %q ApplyConfig failed: %v", ns, err))
		}
	}
}
