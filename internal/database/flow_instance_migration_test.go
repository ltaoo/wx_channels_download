package database

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestFlowInstanceDisabledMigration(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sql_db, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sql_db.Close() })
	for _, name := range []string{"000001_init", "000002_content_media_asset", "000003_flow_schedule"} {
		raw, err := Migrations.ReadFile("migrations/" + name + ".up.sql")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := sql_db.Exec(string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := sql_db.Exec("INSERT INTO user_flow (id,name,definition) VALUES ('flow','Flow','{}'); INSERT INTO user_flow_instance (id,flow_id,name) VALUES ('instance','flow','Instance')"); err != nil {
			t.Fatal(err)
		}
		var disabled bool
		if err := sql_db.QueryRow("SELECT disabled FROM user_flow_instance WHERE id='instance'").Scan(&disabled); err != nil || disabled {
			t.Fatalf("new instance must default to enabled: %v, %v", disabled, err)
		}
		if _, err := sql_db.Exec("UPDATE user_flow_instance SET disabled=1 WHERE id='instance'"); err != nil {
			t.Fatal(err)
		}
		if err := sql_db.QueryRow("SELECT disabled FROM user_flow_instance WHERE id='instance'").Scan(&disabled); err != nil || !disabled {
			t.Fatalf("disabled state must be writable: %v, %v", disabled, err)
		}
		if attempt == 1 {
			break
		}
		down, err := Migrations.ReadFile("migrations/000003_flow_schedule.down.sql")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := sql_db.Exec(string(down)); err != nil {
			t.Fatal(err)
		}
		up, err := Migrations.ReadFile("migrations/000003_flow_schedule.up.sql")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := sql_db.Exec(string(up)); err != nil {
			t.Fatal(err)
		}
	}
}
