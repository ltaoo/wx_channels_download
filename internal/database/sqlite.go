package database

import (
	"database/sql"
	"fmt"

	"gorm.io/gorm"
)

const sqliteBusyTimeoutMillis = 5000

// ConfigureSQLiteRuntime applies runtime settings that reduce lock contention
// under high-frequency download progress writes.
func ConfigureSQLiteRuntime(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get database handle: %w", err)
	}
	configureSQLitePool(sqlDB)

	if err := db.Exec(fmt.Sprintf("PRAGMA busy_timeout = %d", sqliteBusyTimeoutMillis)).Error; err != nil {
		return fmt.Errorf("set sqlite busy_timeout: %w", err)
	}
	var journalMode string
	if err := db.Raw("PRAGMA journal_mode = WAL").Scan(&journalMode).Error; err != nil {
		return fmt.Errorf("set sqlite journal_mode WAL: %w", err)
	}
	if err := db.Exec("PRAGMA synchronous = NORMAL").Error; err != nil {
		return fmt.Errorf("set sqlite synchronous NORMAL: %w", err)
	}

	return nil
}

func configureSQLitePool(sqlDB *sql.DB) {
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
}
