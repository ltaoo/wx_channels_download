package database

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestAutomationMigrationPreservesLegacyRunsAndRollsBack(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	entries, err := Migrations.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".up.sql") && name < "000005" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	apply := func(name string) {
		t.Helper()
		raw, err := Migrations.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(raw)); err != nil {
			t.Fatalf("migration %s: %v", name, err)
		}
	}
	for _, name := range names {
		apply(name)
	}
	if _, err := db.Exec("INSERT INTO flow_run_record (id, flow_id, status) VALUES ('legacy', 'flow', 'COMPLETED')"); err != nil {
		t.Fatal(err)
	}
	apply("000005_automation_runtime.up.sql")
	var timeout, deadline int
	if err := db.QueryRow("SELECT timeout_sec, deadline_at FROM flow_run_record WHERE id = 'legacy'").Scan(&timeout, &deadline); err != nil {
		t.Fatal(err)
	}
	if timeout != 3600 || deadline != 0 {
		t.Fatal(fmt.Sprintf("invalid legacy defaults: %d %d", timeout, deadline))
	}
	apply("000005_automation_runtime.down.sql")
	var status string
	if err := db.QueryRow("SELECT status FROM flow_run_record WHERE id = 'legacy'").Scan(&status); err != nil || status != "COMPLETED" {
		t.Fatalf("rollback lost legacy record: %s %v", status, err)
	}
	apply("000005_automation_runtime.up.sql")
}
