package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

// testPlugin is a Configurable implementation for testing.
type testPlugin struct {
	ns       string
	schema   []ConfigItem
	applied  bool
	valueStr string
	valueInt int
}

func (p *testPlugin) ConfigNamespace() string          { return p.ns }
func (p *testPlugin) ConfigSchema() []ConfigItem        { return p.schema }
func (p *testPlugin) ApplyConfig(sub *SubViper) error {
	p.applied = true
	p.valueStr = sub.GetString("key")
	p.valueInt = sub.GetInt("count")
	return nil
}

func TestRegisterPlugin(t *testing.T) {
	// Reset the global plugin registry for this test
	pluginsMu.Lock()
	plugins = nil
	pluginsMu.Unlock()

	p := &testPlugin{ns: "test_ns"}
	RegisterPlugin(p)

	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
}

func TestRegisterPluginDuplicateNamespacePanics(t *testing.T) {
	pluginsMu.Lock()
	plugins = nil
	pluginsMu.Unlock()

	RegisterPlugin(&testPlugin{ns: "dup_test"})

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate namespace, but none occurred")
		}
	}()
	RegisterPlugin(&testPlugin{ns: "dup_test"})
}

func TestLoadPluginConfigs(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()

	pluginsMu.Lock()
	plugins = nil
	pluginsMu.Unlock()

	// Write a temp config file and load it
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	data := []byte("test_ns:\n  key: hello\n  count: 42\n")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	viper.SetConfigFile(configPath)
	if err := viper.ReadInConfig(); err != nil {
		t.Fatal(err)
	}

	// Register plugin with schema
	p := &testPlugin{
		ns: "test_ns",
		schema: []ConfigItem{
			{Key: "key", Type: ConfigTypeString, Default: "", Description: "test key"},
			{Key: "count", Type: ConfigTypeInt, Default: 0, Description: "test count"},
		},
	}
	RegisterPlugin(p)

	LoadPluginConfigs()

	if !p.applied {
		t.Fatal("ApplyConfig was not called")
	}
	if p.valueStr != "hello" {
		t.Fatalf("valueStr = %q, want %q", p.valueStr, "hello")
	}
	if p.valueInt != 42 {
		t.Fatalf("valueInt = %d, want %d", p.valueInt, 42)
	}

	// Verify schema items were registered with namespace prefix
	schema := GetSchema()
	foundKey := false
	foundCount := false
	for _, item := range schema {
		if item.Key == "test_ns.key" {
			foundKey = true
		}
		if item.Key == "test_ns.count" {
			foundCount = true
		}
	}
	if !foundKey {
		t.Fatal("test_ns.key not found in schema registry")
	}
	if !foundCount {
		t.Fatal("test_ns.count not found in schema registry")
	}
}

func TestLoadPluginConfigsDefaultValues(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()

	pluginsMu.Lock()
	plugins = nil
	pluginsMu.Unlock()

	// Load with no config file (defaults only)
	p := &testPlugin{
		ns: "default_test",
		schema: []ConfigItem{
			{Key: "key", Type: ConfigTypeString, Default: "default_val", Description: "test key"},
			{Key: "count", Type: ConfigTypeInt, Default: 100, Description: "test count"},
		},
	}
	RegisterPlugin(p)

	LoadPluginConfigs()

	if !p.applied {
		t.Fatal("ApplyConfig was not called")
	}
	if p.valueStr != "default_val" {
		t.Fatalf("valueStr = %q, want %q", p.valueStr, "default_val")
	}
	if p.valueInt != 100 {
		t.Fatalf("valueInt = %d, want %d", p.valueInt, 100)
	}
}

func TestSubViperMethods(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()

	v := viper.New()
	v.Set("str", "hello")
	v.Set("bool", true)
	v.Set("int", 42)
	v.Set("int64", int64(99))
	v.Set("float", 3.14)
	v.Set("slice", []string{"a", "b"})
	v.Set("map", map[string]interface{}{"k": "v"})

	sub := NewSubViper(v)

	if got := sub.GetString("str"); got != "hello" {
		t.Fatalf("GetString = %q, want %q", got, "hello")
	}
	if got := sub.GetBool("bool"); got != true {
		t.Fatal("GetBool = false, want true")
	}
	if got := sub.GetInt("int"); got != 42 {
		t.Fatalf("GetInt = %d, want %d", got, 42)
	}
	if got := sub.GetInt64("int64"); got != 99 {
		t.Fatalf("GetInt64 = %d, want %d", got, 99)
	}
	if got := sub.GetFloat64("float"); got != 3.14 {
		t.Fatalf("GetFloat64 = %f, want %f", got, 3.14)
	}
	if got := sub.GetStringSlice("slice"); len(got) != 2 {
		t.Fatalf("GetStringSlice len = %d, want %d", len(got), 2)
	}
	if got := sub.GetStringMap("map"); got["k"] != "v" {
		t.Fatalf("GetStringMap = %v", got)
	}
	if !sub.IsSet("str") {
		t.Fatal("IsSet('str') = false, want true")
	}
	if sub.IsSet("nonexistent") {
		t.Fatal("IsSet('nonexistent') = true, want false")
	}
}

func TestSubViperNil(t *testing.T) {
	sub := NewSubViper(nil)
	if sub == nil {
		t.Fatal("NewSubViper(nil) returned nil")
	}
	// Should not panic
	_ = sub.GetString("any")
	_ = sub.GetBool("any")
}
