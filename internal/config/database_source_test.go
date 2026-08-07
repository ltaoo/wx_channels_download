package config

import (
	"context"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"wx_channel/pkg/configapi"
)

func TestDatabaseSourcePersistsRuntimeOverrides(t *testing.T) {
	schema := configapi.Item{
		Key:     "database_source_test.enabled",
		Type:    configapi.TypeBool,
		Default: false,
		Reload:  configapi.ReloadHot,
	}
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "config.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	first := &Config{FullPath: filepath.Join(t.TempDir(), "config.yaml")}
	register_config_test_module(t, first, "test.database_source", schema)
	if err := first.AttachDatabaseSource(context.Background(), db); err != nil {
		t.Fatalf("attach database source: %v", err)
	}
	if _, err := first.Apply(context.Background(), configapi.UpdateRequest{
		Values: map[string]any{"database_source_test.enabled": true},
	}); err != nil {
		t.Fatalf("apply database config: %v", err)
	}

	second := &Config{FullPath: filepath.Join(t.TempDir(), "config.yaml")}
	register_config_test_module(t, second, "test.database_source", schema)
	if err := second.AttachDatabaseSource(context.Background(), db); err != nil {
		t.Fatalf("attach database source to second config: %v", err)
	}
	if !second.GetBool("database_source_test.enabled") {
		t.Fatal("database override was not restored")
	}
	entry_found := false
	for _, entry := range second.View(false).Items {
		if entry.Item.Key == "database_source_test.enabled" {
			entry_found = true
			if entry.Source != "database" {
				t.Fatalf("source = %q, want database", entry.Source)
			}
		}
	}
	if !entry_found {
		t.Fatal("database source test item missing from GUI view")
	}
}
