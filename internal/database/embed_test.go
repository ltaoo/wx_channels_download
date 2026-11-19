package database

import (
	"database/sql"
	"reflect"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
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
		if err != nil || version != 3 || dirty {
			t.Fatalf("migration state: version=%d dirty=%v err=%v", version, dirty, err)
		}
		var table_count, platform_count int
		if err := sql_db.QueryRow(`SELECT COUNT(*) FROM sqlite_master
			WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name != 'schema_migrations'`).Scan(&table_count); err != nil {
			t.Fatal(err)
		}
		if table_count != 57 {
			t.Fatalf("got %d business tables, want 57", table_count)
		}
		if err := sql_db.QueryRow("SELECT COUNT(*) FROM platform").Scan(&platform_count); err != nil {
			t.Fatal(err)
		}
		if platform_count != 12 {
			t.Fatalf("got %d default platforms, want 12", platform_count)
		}
		var tag_count int
		if err := sql_db.QueryRow(`SELECT (SELECT COUNT(*) FROM tag) + (SELECT COUNT(*) FROM content_tag)`).Scan(&tag_count); err != nil {
			t.Fatal(err)
		}
		if tag_count != 0 {
			t.Fatalf("got %d migrated tags and content tags, want 0", tag_count)
		}
		var original_tags string
		if err := sql_db.QueryRow(`SELECT tags FROM content WHERE id = 'migration-test'`).Scan(&original_tags); err != nil {
			t.Fatal(err)
		}
		if original_tags != `["migration-tag"]` {
			t.Fatalf("original content tags changed: %q", original_tags)
		}
		if _, err := sql_db.Exec(`INSERT INTO flow_run_record
			(id, flow_id, instance_id, status, definition, initial_data, checkpoint, timeout_sec, deadline_at, retry_of)
			VALUES ('test-run', 'test-flow', 'test-instance', 'completed', '{}', '{}', '{}', 60, 100, '')`); err != nil {
			t.Fatal(err)
		}
		if _, err := sql_db.Exec("INSERT INTO user_flow (id,name,definition) VALUES ('test-flow','Test flow','{}')"); err != nil {
			t.Fatal(err)
		}
		if _, err := sql_db.Exec("INSERT INTO user_flow_instance (id, name, flow_id) VALUES ('test-instance', 'Test instance', 'test-flow')"); err != nil {
			t.Fatal(err)
		}
		var node_configs, trigger_overrides, default_triggers, node_states string
		var inherit_triggers bool
		if err := sql_db.QueryRow(`SELECT instance.node_configs, instance.trigger_overrides, instance.inherit_triggers, flow.default_triggers, run.node_states
			FROM user_flow_instance AS instance JOIN user_flow AS flow ON flow.id = instance.flow_id
			JOIN flow_run_record AS run ON run.instance_id = instance.id WHERE instance.id = 'test-instance'`).Scan(&node_configs, &trigger_overrides, &inherit_triggers, &default_triggers, &node_states); err != nil {
			t.Fatal(err)
		}
		if node_configs != "{}" || trigger_overrides != "{}" || inherit_triggers || default_triggers != "[]" || node_states != "{}" {
			t.Fatalf("unexpected defaults: configs=%q overrides=%q inherit=%v triggers=%q states=%q", node_configs, trigger_overrides, inherit_triggers, default_triggers, node_states)
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
		if err := sql_db.QueryRow(`EXPLAIN QUERY PLAN SELECT id, created_at, status FROM flow_run_record
			WHERE instance_id = ? AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 20`, "test-instance").Scan(&plan_id, &parent_id, &unused, &plan_detail); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(plan_detail, "idx_flow_run_record_instance_history") {
			t.Fatalf("instance history must use its pagination index: %s", plan_detail)
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
			WHERE name IN ('flow_schedule', 'flow_run_record', 'tag', 'content_tag', 'account_tag', 'user_flow', 'user_flow_instance', 'flow_run_node_log')`).Scan(&table_count); err != nil {
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

func TestBrowseHistoryIDMigration(t *testing.T) {
	for _, id_type := range []string{"INTEGER", "TEXT"} {
		t.Run(id_type, func(t *testing.T) {
			sql_db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
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
			if id_type == "INTEGER" {
				var schema string
				if err := sql_db.QueryRow(`SELECT sql FROM sqlite_master WHERE name = 'browse_history'`).Scan(&schema); err != nil {
					t.Fatal(err)
				}
				schema = strings.Replace(schema, "`id` TEXT PRIMARY KEY", "`id` INTEGER PRIMARY KEY AUTOINCREMENT", 1)
				if _, err := sql_db.Exec("DROP TABLE browse_history;" + schema); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := sql_db.Exec(`INSERT INTO browse_history
				(id, platform_id, visited_times, type, external_id, title, url, source_url,
				cover_url, cover_width, cover_height, publish_time, extra_data, created_at, updated_at, deleted_at)
				VALUES (42, 'wx_channels', 5, 'video', 'legacy', '历史记录', 'https://example.com/video',
				'https://example.com/source', 'https://example.com/cover', '1920', '1080', 100, '{"key":"value"}', 101, 102, NULL),
				(9223372036854775807, 'wx_channels', 1, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 0, 0, 103);
				INSERT INTO browse_history_account (browse_history_id, account_id, role, created_at)
				VALUES ('42', 'wx_channels:author', 'author', 101)`); err != nil {
				t.Fatal(err)
			}
			if id_type == "TEXT" {
				if _, err := sql_db.Exec(`INSERT INTO browse_history (id, platform_id, external_id)
					VALUES ('webpage:existing:1', 'webpage', 'existing')`); err != nil {
					t.Fatal(err)
				}
			}
			db, err := gorm.Open(sqlite.New(sqlite.Config{Conn: sql_db}), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			var before, after []model.BrowseHistory
			if err := db.Order("CAST(id AS TEXT)").Find(&before).Error; err != nil {
				t.Fatal(err)
			}
			if err := migrator.Up(); err != nil {
				t.Fatal(err)
			}
			if err := db.Order("CAST(id AS TEXT)").Find(&after).Error; err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("browse history changed: before=%+v after=%+v", before, after)
			}
			var column_type string
			if err := sql_db.QueryRow(`SELECT type FROM pragma_table_info('browse_history') WHERE name = 'id' AND pk = 1`).Scan(&column_type); err != nil {
				t.Fatal(err)
			}
			if column_type != "TEXT" {
				t.Fatalf("primary key type = %q, want TEXT", column_type)
			}
			var count int
			if err := sql_db.QueryRow(`SELECT COUNT(*) FROM browse_history WHERE typeof(id) != 'text'`).Scan(&count); err != nil || count != 0 {
				t.Fatalf("non-text IDs: count=%d error=%v", count, err)
			}
			if err := sql_db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index'
				AND name IN ('idx_browse_history_time', 'idx_browse_history_external')`).Scan(&count); err != nil || count != 2 {
				t.Fatalf("history indexes: count=%d error=%v", count, err)
			}
			if err := sql_db.QueryRow(`SELECT COUNT(*) FROM browse_history_account AS link
				JOIN browse_history ON browse_history.id = link.browse_history_id
				WHERE link.account_id = 'wx_channels:author' AND link.role = 'author' AND link.created_at = 101`).Scan(&count); err != nil || count != 1 {
				t.Fatalf("account links: count=%d error=%v", count, err)
			}
			record := model.BrowseHistory{PlatformId: "wx_channels", ExternalId: "legacy", Timestamps: model.Timestamps{CreatedAt: 104}}
			if err := record.Upsert(db); err != nil {
				t.Fatalf("update migrated history: %v", err)
			}
			if err := sql_db.QueryRow(`SELECT visited_times FROM browse_history WHERE id = '42'`).Scan(&count); err != nil || count != 6 {
				t.Fatalf("updated visit count: count=%d error=%v", count, err)
			}
			record.ExternalId = "new"
			if err := record.Upsert(db); err != nil {
				t.Fatalf("insert string ID history: %v", err)
			}
			if record.Id != "wx_channels:new:104" {
				t.Fatalf("new history ID = %q", record.Id)
			}
			if err := migrator.Migrate(2); err != nil {
				t.Fatalf("rollback: %v", err)
			}
			if err := db.Where("id = ?", record.Id).First(&model.BrowseHistory{}).Error; err != nil {
				t.Fatalf("rollback lost string ID history: %v", err)
			}
			if err := migrator.Up(); err != nil {
				t.Fatalf("reapply: %v", err)
			}
		})
	}
}
