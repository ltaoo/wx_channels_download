package database

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func TestAutomationMigrationsRoundTrip(t *testing.T) {
	sql_db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer sql_db.Close()
	sql_db.SetMaxOpenConns(1)

	driver, err := sqlite3.WithInstance(sql_db, &sqlite3.Config{})
	if err != nil {
		t.Fatal(err)
	}
	source, err := iofs.New(Migrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	migrator, err := migrate.NewWithInstance("iofs", source, "sqlite3", driver)
	if err != nil {
		t.Fatal(err)
	}
	defer migrator.Close()

	if err := migrator.Migrate(2); err != nil {
		t.Fatal(err)
	}
	if _, err := sql_db.Exec(`INSERT INTO content (id, platform_id, type, external_id, tags)
		VALUES ('migration-test', 'wx_channels', 'video', 'migration-test', '["migration-tag"]')`); err != nil {
		t.Fatal(err)
	}

	for cycle := 0; cycle < 2; cycle++ {
		if err := migrator.Up(); err != nil {
			t.Fatalf("migrate up: %v", err)
		}
		version, dirty, err := migrator.Version()
		if err != nil || version != 4 || dirty {
			t.Fatalf("migration state: version=%d dirty=%v err=%v", version, dirty, err)
		}
		var table_count, platform_count int
		if err := sql_db.QueryRow(`SELECT COUNT(*) FROM sqlite_master
			WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name != 'schema_migrations'`).Scan(&table_count); err != nil {
			t.Fatal(err)
		}
		if table_count != 55 {
			t.Fatalf("got %d business tables, want 55", table_count)
		}
		if err := sql_db.QueryRow("SELECT COUNT(*) FROM platform").Scan(&platform_count); err != nil {
			t.Fatal(err)
		}
		if platform_count != 12 {
			t.Fatalf("got %d default platforms, want 12", platform_count)
		}
		var tag_count int
		if err := sql_db.QueryRow(`SELECT COUNT(*) FROM content_tag
			JOIN tag ON tag.id = content_tag.tag_id
			WHERE content_id = 'migration-test' AND tag.name = 'migration-tag'`).Scan(&tag_count); err != nil {
			t.Fatal(err)
		}
		if tag_count != 1 {
			t.Fatalf("got %d migrated content tags, want 1", tag_count)
		}
		if _, err := sql_db.Exec(`INSERT INTO flow_run_record
			(id, flow_id, status, definition, initial_data, checkpoint, node_logs, timeout_sec, deadline_at, retry_of)
			VALUES ('test-run', 'test-flow', 'completed', '{}', '{}', '{}', '[]', 60, 100, '')`); err != nil {
			t.Fatal(err)
		}
		if _, err := sql_db.Exec("INSERT INTO user_flow (id, name) VALUES ('test-flow', 'Test flow')"); err != nil {
			t.Fatal(err)
		}
		var node_configs string
		if err := sql_db.QueryRow("SELECT node_configs FROM user_flow WHERE id = 'test-flow'").Scan(&node_configs); err != nil {
			t.Fatal(err)
		}
		if node_configs != "{}" {
			t.Fatalf("got default node configs %q, want {}", node_configs)
		}
		var plan_id, parent_id, unused int
		var plan_detail string
		if err := sql_db.QueryRow(`EXPLAIN QUERY PLAN SELECT * FROM flow_run_record
			WHERE status = ? AND deadline_at > 0 AND deadline_at <= ? AND deleted_at IS NULL`,
			"WAITING", 100).Scan(&plan_id, &parent_id, &unused, &plan_detail); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(plan_detail, "USING INDEX idx_flow_run_record_status_deleted_deadline") ||
			!strings.Contains(plan_detail, "status=? AND deleted_at=? AND deadline_at>? AND deadline_at<?") {
			t.Fatalf("expiry query must use the composite index for filtering: %s", plan_detail)
		}
		if err := migrator.Up(); err != migrate.ErrNoChange {
			t.Fatalf("repeated migrate up: got %v, want ErrNoChange", err)
		}
		if err := migrator.Migrate(2); err != nil {
			t.Fatalf("rollback automation tables: %v", err)
		}
		version, dirty, err = migrator.Version()
		if err != nil || version != 2 || dirty {
			t.Fatalf("rollback state: version=%d dirty=%v err=%v", version, dirty, err)
		}
		if err := sql_db.QueryRow(`SELECT COUNT(*) FROM sqlite_master
			WHERE name IN ('flow_schedule', 'flow_run_record', 'tag', 'content_tag', 'account_tag', 'user_flow')`).Scan(&table_count); err != nil {
			t.Fatal(err)
		}
		if table_count != 0 {
			t.Fatalf("rollback left %d automation tables, want 0", table_count)
		}
		var index_count int
		if err := sql_db.QueryRow(`SELECT COUNT(*) FROM sqlite_master
			WHERE type = 'index' AND name = 'idx_flow_run_record_status_deleted_deadline'`).Scan(&index_count); err != nil {
			t.Fatal(err)
		}
		if index_count != 0 {
			t.Fatal("rollback left the expiry index")
		}
		if err := sql_db.QueryRow("SELECT COUNT(*) FROM platform").Scan(&platform_count); err != nil {
			t.Fatal(err)
		}
		if platform_count != 7 {
			t.Fatalf("rollback left %d default platforms, want 7", platform_count)
		}
		var content_count int
		if err := sql_db.QueryRow("SELECT COUNT(*) FROM content WHERE id = 'migration-test'").Scan(&content_count); err != nil {
			t.Fatal(err)
		}
		if content_count != 1 {
			t.Fatalf("rollback left %d original content rows, want 1", content_count)
		}
	}
}

func TestSubflowStateMigrationPreservesRunHistory(t *testing.T) {
	sql_db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer sql_db.Close()
	sql_db.SetMaxOpenConns(1)
	driver, err := sqlite3.WithInstance(sql_db, &sqlite3.Config{})
	if err != nil {
		t.Fatal(err)
	}
	source, err := iofs.New(Migrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	migrator, err := migrate.NewWithInstance("iofs", source, "sqlite3", driver)
	if err != nil {
		t.Fatal(err)
	}
	defer migrator.Close()
	if err := migrator.Migrate(3); err != nil {
		t.Fatal(err)
	}
	if _, err := sql_db.Exec(`INSERT INTO flow_run_record (id, flow_id, status, node_logs) VALUES ('existing', 'flow', 'COMPLETED', '[{"node_id":"first"}]')`); err != nil {
		t.Fatal(err)
	}
	if err := migrator.Up(); err != nil {
		t.Fatal(err)
	}
	var states, logs, status string
	if err := sql_db.QueryRow(`SELECT node_states, node_logs, status FROM flow_run_record WHERE id = 'existing'`).Scan(&states, &logs, &status); err != nil {
		t.Fatal(err)
	}
	if states != "{}" || logs != `[{"node_id":"first"}]` || status != "COMPLETED" {
		t.Fatalf("upgrade changed history: states=%s logs=%s status=%s", states, logs, status)
	}
	if err := migrator.Migrate(3); err != nil {
		t.Fatal(err)
	}
	if err := sql_db.QueryRow(`SELECT node_logs, status FROM flow_run_record WHERE id = 'existing'`).Scan(&logs, &status); err != nil {
		t.Fatal(err)
	}
	if logs != `[{"node_id":"first"}]` || status != "COMPLETED" {
		t.Fatal("rollback changed run history")
	}
}
