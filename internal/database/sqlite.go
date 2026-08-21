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
	if err := remove_velo_timestamp_callbacks(db); err != nil {
		return err
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

// remove_velo_timestamp_callbacks removes Velo's string timestamp callbacks.
// Application models use Unix-millisecond integer timestamps and GORM already
// handles slices correctly. Velo's create callback assumes a single struct and
// panics when CreateInBatches supplies a slice.
func remove_velo_timestamp_callbacks(db *gorm.DB) error {
	if err := db.Callback().Create().Remove("set_created_at"); err != nil {
		return fmt.Errorf("remove incompatible create timestamp callback: %w", err)
	}
	if err := db.Callback().Update().Remove("set_updated_at"); err != nil {
		return fmt.Errorf("remove incompatible update timestamp callback: %w", err)
	}
	return nil
}

func configure_sqlite_pool(sql_db *sql.DB) {
	sql_db.SetMaxOpenConns(1)
	sql_db.SetMaxIdleConns(1)
}
