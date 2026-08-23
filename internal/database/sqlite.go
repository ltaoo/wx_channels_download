package database

import (
	"database/sql"
	"fmt"

	"gorm.io/gorm"
)

const sqlite_busy_timeout_millis = 5000

// ConfigureSQLiteRuntime applies runtime settings that reduce lock contention
// under high-frequency download progress writes.
func ConfigureSQLiteRuntime(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}

	sql_db, err := db.DB()
	if err != nil {
		return fmt.Errorf("get database handle: %w", err)
	}
	configure_sqlite_pool(sql_db)

	if err := db.Exec(fmt.Sprintf("PRAGMA busy_timeout = %d", sqlite_busy_timeout_millis)).Error; err != nil {
		return fmt.Errorf("set sqlite busy_timeout: %w", err)
	}
	var journal_mode string
	if err := db.Raw("PRAGMA journal_mode = WAL").Scan(&journal_mode).Error; err != nil {
		return fmt.Errorf("set sqlite journal_mode WAL: %w", err)
	}
	if err := db.Exec("PRAGMA synchronous = NORMAL").Error; err != nil {
		return fmt.Errorf("set sqlite synchronous NORMAL: %w", err)
	}

	return nil
}
func configure_sqlite_pool(sql_db *sql.DB) {
	sql_db.SetMaxOpenConns(1)
	sql_db.SetMaxIdleConns(1)
}
