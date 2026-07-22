package database

import (
	"database/sql"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/golang-migrate/migrate/v4/database"
)

// sqliteMigrateDriver implements database.Driver using the pure-Go "sqlite"
// driver registered by github.com/glebarez/go-sqlite (via github.com/glebarez/sqlite).
type sqliteMigrateDriver struct {
	db *sql.DB

	mu     sync.Mutex
	isLock bool
}

func (d *sqliteMigrateDriver) Open(url string) (database.Driver, error) {
	db, err := sql.Open("sqlite", url)
	if err != nil {
		return nil, err
	}
	drv := &sqliteMigrateDriver{db: db}
	if err := drv.ensureVersionTable(); err != nil {
		db.Close()
		return nil, err
	}
	return drv, nil
}

func (d *sqliteMigrateDriver) ensureVersionTable() error {
	_, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT NOT NULL PRIMARY KEY,
			dirty   BOOLEAN NOT NULL
		)`)
	return err
}

func (d *sqliteMigrateDriver) Close() error {
	return d.db.Close()
}

func (d *sqliteMigrateDriver) Lock() error {
	d.mu.Lock()
	if d.isLock {
		d.mu.Unlock()
		return database.ErrLocked
	}
	d.isLock = true
	return nil
}

func (d *sqliteMigrateDriver) Unlock() error {
	if !d.isLock {
		return nil
	}
	d.isLock = false
	d.mu.Unlock()
	return nil
}

func (d *sqliteMigrateDriver) Run(migration io.Reader) error {
	data, err := io.ReadAll(migration)
	if err != nil {
		return fmt.Errorf("failed to read migration: %w", err)
	}

	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Split by semicolons and execute each statement.
	for _, stmt := range splitStatements(string(data)) {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("migration statement failed: %w\n%s", err, stmt)
		}
	}

	return tx.Commit()
}

func (d *sqliteMigrateDriver) SetVersion(version int, dirty bool) error {
	return d.ensureVersionTable()
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM schema_migrations"); err != nil {
		return err
	}
	if version >= 0 {
		if _, err := tx.Exec("INSERT INTO schema_migrations (version, dirty) VALUES (?, ?)", version, dirty); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (d *sqliteMigrateDriver) Version() (version int, dirty bool, err error) {
	err = d.db.QueryRow("SELECT version, dirty FROM schema_migrations LIMIT 1").Scan(&version, &dirty)
	if err == sql.ErrNoRows {
		return database.NilVersion, false, nil
	}
	if err != nil {
		return database.NilVersion, false, err
	}
	return version, dirty, nil
}

func (d *sqliteMigrateDriver) Drop() error {
	rows, err := d.db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, t := range tables {
		if _, err := d.db.Exec("DROP TABLE IF EXISTS " + t); err != nil {
			return err
		}
	}
	return nil
}

// splitStatements splits SQL text by semicolons, handling basic cases.
func splitStatements(sql string) []string {
	var parts []string
	for _, s := range strings.Split(sql, ";") {
		s = strings.TrimSpace(s)
		if s != "" {
			parts = append(parts, s)
		}
	}
	return parts
}
