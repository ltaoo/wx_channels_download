package configapi

import (
	"context"
	"path/filepath"
	"testing"
)

func TestFileSourceYAMLRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	source, err := NewFileSource(FileSourceOptions{
		Name:     "user-file",
		Path:     path,
		Priority: PriorityUserFile,
		Optional: true,
		Writable: true,
	})
	if err != nil {
		t.Fatalf("new file source: %v", err)
	}
	if values, err := source.Load(context.Background()); err != nil || len(values) != 0 {
		t.Fatalf("optional load = %v, %v", values, err)
	}
	want := map[string]any{"proxy": map[string]any{"port": 3030, "system": true}}
	if err := source.Store(context.Background(), want); err != nil {
		t.Fatalf("store: %v", err)
	}
	got, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	port, _ := lookup_path(got, "proxy.port")
	if port != float64(3030) {
		t.Fatalf("port = %v", port)
	}
}

func TestReadonlyFileSourceIsNotAdvertisedAsWritable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	readonly_source, err := NewFileSource(FileSourceOptions{
		Name:     "system-file",
		Path:     path,
		Priority: PrioritySystemFile,
		Optional: true,
		Writable: false,
	})
	if err != nil {
		t.Fatalf("new file source: %v", err)
	}
	manager := NewManager()
	if err := manager.AddSource(readonly_source); err != nil {
		t.Fatalf("add source: %v", err)
	}
	if err := manager.SetDefaultWriteSource(readonly_source.Name()); err == nil {
		t.Fatal("read-only file source accepted as default write source")
	}
	for _, info := range manager.Sources() {
		if info.Name == readonly_source.Name() && info.Writable {
			t.Fatalf("read-only source advertised as writable: %+v", info)
		}
	}
}

func TestCLISourceExpandsDottedKeys(t *testing.T) {
	source, err := NewCLISource(map[string]any{"proxy.port": 3030, "proxy.tun": true})
	if err != nil {
		t.Fatalf("new CLI source: %v", err)
	}
	values, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	port, _ := lookup_path(values, "proxy.port")
	tun, _ := lookup_path(values, "proxy.tun")
	if port != float64(3030) || tun != true {
		t.Fatalf("CLI values = %v", values)
	}
}

func TestValuesSourceExpandsDottedKeys(t *testing.T) {
	source, err := NewValuesSource(map[string]any{"proxy.port": 4040})
	if err != nil {
		t.Fatalf("new values source: %v", err)
	}
	if source.Name() != "values" || source.Priority() != PriorityValues {
		t.Fatalf("values source metadata = %q, %d", source.Name(), source.Priority())
	}
	values, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	port, _ := lookup_path(values, "proxy.port")
	if port != float64(4040) {
		t.Fatalf("values = %v", values)
	}
}

type memory_backend struct {
	values map[string]any
}

func (b *memory_backend) LoadConfig(context.Context) (map[string]any, error) {
	return clone_values(b.values)
}

func (b *memory_backend) SaveConfig(_ context.Context, values map[string]any) error {
	cloned, err := clone_values(values)
	if err != nil {
		return err
	}
	b.values = cloned
	return nil
}

func TestDatabaseSourceUsesStorageNeutralBackend(t *testing.T) {
	backend := &memory_backend{values: map[string]any{"channels": map[string]any{"enabled": true}}}
	source, err := NewDatabaseSource("database", PriorityDatabase, backend)
	if err != nil {
		t.Fatalf("new database source: %v", err)
	}
	if err := source.Store(context.Background(), map[string]any{"channels": map[string]any{"enabled": false}}); err != nil {
		t.Fatalf("store: %v", err)
	}
	values, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	enabled, _ := lookup_path(values, "channels.enabled")
	if enabled != false {
		t.Fatalf("enabled = %v", enabled)
	}
}
