package d1

import (
	"fmt"
	"strings"
)

// Migration represents a database migration
type Migration struct {
	ID      int
	Name    string
	Content string
}

// EnsureMigrationsTable creates the migrations table if it doesn't exist
func (c *Client) EnsureMigrationsTable(databaseID string) error {
	sql := `CREATE TABLE IF NOT EXISTS _cf_migrations (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`
	_, err := c.Query(databaseID, sql, nil)
	return err
}

// GetAppliedMigrations returns a list of applied migration IDs
func (c *Client) GetAppliedMigrations(databaseID string) (map[int]bool, error) {
	sql := "SELECT id FROM _cf_migrations"
	resp, err := c.Query(databaseID, sql, nil)
	if err != nil {
		return nil, err
	}

	applied := make(map[int]bool)
	if len(resp.Result) > 0 && len(resp.Result[0].Results) > 0 {
		for _, row := range resp.Result[0].Results {
			if id, ok := row["id"].(float64); ok {
				applied[int(id)] = true
			}
		}
	}
	return applied, nil
}

// ApplyMigration applies a single migration
func (c *Client) ApplyMigration(databaseID string, migration Migration) error {
	// 1. Run the migration SQL
	// Split by semicolon to handle multiple statements
	// Note: This is a simple split and might break if semicolons are inside strings/comments
	// But for our schema files it should be fine.
	statements := strings.Split(migration.Content, ";")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		_, err := c.Query(databaseID, stmt, nil)
		if err != nil {
			return fmt.Errorf("failed to execute migration statement in %s: %v\nSQL: %s", migration.Name, err, stmt)
		}
	}

	// 2. Record the migration
	recordSQL := "INSERT INTO _cf_migrations (id, name) VALUES (?, ?)"
	_, err := c.Query(databaseID, recordSQL, []any{migration.ID, migration.Name})
	if err != nil {
		return fmt.Errorf("failed to record migration %s: %v", migration.Name, err)
	}

	return nil
}
