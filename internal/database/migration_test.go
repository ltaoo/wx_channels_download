package database

import (
	"path/filepath"
	"testing"

	velo_database "github.com/ltaoo/velo/database"
)

func TestMigrationsSupportVersion3(t *testing.T) {
	cfg := &velo_database.DBConfig{
		Type:                      velo_database.DBTypeSQLite,
		Path:                      SQLiteDSN(filepath.Join(t.TempDir(), "data.db")),
		DisableTimestampCallbacks: true,
	}
	db, err := velo_database.NewDatabase(cfg)
	if err != nil {
		t.Fatal(err)
	}
	sql_db, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sql_db.Close() })
	migrator := velo_database.NewMigrator(cfg, &Migrations)
	assert_version_3 := func() {
		t.Helper()
		var version int
		var dirty bool
		if err := sql_db.QueryRow("SELECT version, dirty FROM schema_migrations").Scan(&version, &dirty); err != nil {
			t.Fatal(err)
		}
		if version != 3 || dirty {
			t.Fatalf("expected clean version 3, got version=%d dirty=%t", version, dirty)
		}
	}

	if err := migrator.MigrateUp(db); err != nil {
		t.Fatalf("initialize fresh database: %v", err)
	}
	assert_version_3()
	if err := migrator.MigrateUp(db); err != nil {
		t.Fatalf("initialize existing version 3 database: %v", err)
	}
	assert_version_3()
	if err := migrator.MigrateTo(db, 2); err != nil {
		t.Fatalf("roll back version 3: %v", err)
	}
	for _, table := range []string{"flow_schedule", "flow_run_record", "tag", "content_tag", "account_tag"} {
		if db.Migrator().HasTable(table) {
			t.Fatalf("table %s remains after rollback", table)
		}
	}
	if err := migrator.MigrateUp(db); err != nil {
		t.Fatalf("upgrade version 2 database: %v", err)
	}
	assert_version_3()
	for _, table := range []string{"flow_schedule", "flow_run_record", "tag", "content_tag", "account_tag"} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("table %s missing after upgrade", table)
		}
	}
}
