package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/ltaoo/velo"
	"gorm.io/gorm"
)

func TestVeloSQLiteMigratesAndPreservesData(t *testing.T) {
	db_path := filepath.Join(t.TempDir(), "application.db")
	open_database := func() *gorm.DB {
		t.Helper()
		app := velo.NewApp(&velo.VeloAppOpt{Mode: velo.ModeHttp})
		if err := app.Migrate(&velo.VeloDatabaseOpt{
			DBType:                    velo.DBTypeSQLite,
			DBPath:                    SQLiteDSN(db_path),
			Migrations:                &Migrations,
			DisableTimestampCallbacks: true,
		}); err != nil {
			t.Fatal(err)
		}
		if err := ConfigureSQLiteRuntime(app.DB); err != nil {
			t.Fatal(err)
		}
		return app.DB
	}
	legacy, err := gorm.Open(sqlite.Open(SQLiteDSN(db_path)), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	legacy_db, err := legacy.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = legacy_db.Close() })
	if err := legacy.Exec("CREATE TABLE driver_probe (value INTEGER NOT NULL)").Error; err != nil {
		t.Fatal(err)
	}
	if err := legacy.Exec("INSERT INTO driver_probe VALUES (0)").Error; err != nil {
		t.Fatal(err)
	}
	if err := legacy_db.Close(); err != nil {
		t.Fatal(err)
	}
	db := open_database()
	sql_db, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sql_db.Close() })
	var version int
	var dirty bool
	if err := sql_db.QueryRow("SELECT version, dirty FROM schema_migrations").Scan(&version, &dirty); err != nil {
		t.Fatal(err)
	}
	if version != 3 || dirty {
		t.Fatalf("migration version=%d dirty=%t", version, dirty)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var connections []*sql.Conn
	for i := 0; i < 2; i++ {
		connection, err := sql_db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		connections = append(connections, connection)
		t.Cleanup(func() { _ = connection.Close() })
		for pragma, expected := range map[string]string{
			"journal_mode": "wal",
			"synchronous":  "1",
			"busy_timeout": "5000",
		} {
			var actual string
			if err := connection.QueryRowContext(ctx, "PRAGMA "+pragma).Scan(&actual); err != nil {
				t.Fatal(err)
			}
			if actual != expected {
				t.Fatalf("connection %d %s=%s, want %s", i, pragma, actual, expected)
			}
		}
	}
	for _, connection := range connections {
		_ = connection.Close()
	}
	var workers sync.WaitGroup
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for j := 0; j < 8; j++ {
				if err := db.WithContext(ctx).Exec("UPDATE driver_probe SET value = value + 1").Error; err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	workers.Wait()
	if err := sql_db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := open_database()
	reopened_db, err := reopened.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened_db.Close() })
	var value int
	if err := reopened_db.QueryRow("SELECT value FROM driver_probe").Scan(&value); err != nil {
		t.Fatal(err)
	}
	if value != 32 {
		t.Fatalf("persisted value=%d, want 32", value)
	}
}
